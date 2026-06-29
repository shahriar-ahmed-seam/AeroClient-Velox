package store

import (
	"errors"
	"reflect"
	"testing"

	"volt/internal/model"
)

// sampleRequest builds a SavedRequest with a representative, fully-populated
// configuration so round-trip tests exercise the data-JSON column.
func sampleRequest(id, name string) model.SavedRequest {
	return model.SavedRequest{
		ID:   id,
		Name: name,
		RawRequest: model.RawRequest{
			Method: model.MethodPost,
			URL:    "https://example.com/" + name,
			Params: []model.KeyValue{{Key: "q", Value: "1", Enabled: true}},
			Headers: []model.KeyValue{
				{Key: "X-Test", Value: "v", Enabled: true},
			},
			Body: model.BodySpec{Type: model.BodyJSON, Raw: `{"a":1}`},
			Auth: model.AuthSpec{Type: model.AuthBearer, BearerToken: "tok"},
		},
	}
}

// TestSaveCollectionListTreeRoundTrip verifies a full nested tree survives a
// save/list round-trip with names, nesting, and order intact (Req 5.1, 5.6).
func TestSaveCollectionListTreeRoundTrip(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	c := model.Collection{
		ID:    "c1",
		Name:  "My Collection",
		Order: 0,
		Requests: []model.SavedRequest{
			sampleRequest("r-top", "top"),
		},
		Folders: []model.Folder{
			{
				ID:   "f1",
				Name: "Folder One",
				Requests: []model.SavedRequest{
					sampleRequest("r-f1-a", "f1a"),
					sampleRequest("r-f1-b", "f1b"),
				},
				Folders: []model.Folder{
					{
						ID:   "f1-1",
						Name: "Nested",
						Requests: []model.SavedRequest{
							sampleRequest("r-nested", "nested"),
						},
						Folders: []model.Folder{},
					},
				},
			},
			{
				ID:       "f2",
				Name:     "Folder Two",
				Folders:  []model.Folder{},
				Requests: []model.SavedRequest{},
			},
		},
	}

	if _, err := s.SaveCollection(c); err != nil {
		t.Fatalf("SaveCollection: %v", err)
	}

	tree, err := s.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if len(tree) != 1 {
		t.Fatalf("got %d collections, want 1", len(tree))
	}
	if !reflect.DeepEqual(tree[0], c) {
		t.Fatalf("round-trip mismatch:\n got: %#v\nwant: %#v", tree[0], c)
	}
}

// TestListTreePersistsAcrossReopen confirms the tree is durable across closing
// and reopening the database file (Req 5.6).
func TestListTreePersistsAcrossReopen(t *testing.T) {
	path := t.TempDir() + "/volt.db"

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	c := model.Collection{
		ID:       "c1",
		Name:     "Persisted",
		Folders:  []model.Folder{{ID: "f1", Name: "F", Folders: []model.Folder{}, Requests: []model.SavedRequest{sampleRequest("r1", "r1")}}},
		Requests: []model.SavedRequest{},
	}
	if _, err := s1.SaveCollection(c); err != nil {
		t.Fatalf("SaveCollection: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	tree, err := s2.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if len(tree) != 1 || !reflect.DeepEqual(tree[0], c) {
		t.Fatalf("after reopen mismatch:\n got: %#v\nwant: %#v", tree, c)
	}
}

// TestCollectionOrderPreserved verifies collections come back in their stored
// Order, not insertion-by-ID order.
func TestCollectionOrderPreserved(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// Saved out of order; Order field dictates the listing order.
	if _, err := s.SaveCollection(model.Collection{ID: "b", Name: "B", Order: 2}); err != nil {
		t.Fatalf("save B: %v", err)
	}
	if _, err := s.SaveCollection(model.Collection{ID: "a", Name: "A", Order: 0}); err != nil {
		t.Fatalf("save A: %v", err)
	}
	if _, err := s.SaveCollection(model.Collection{ID: "c", Name: "C", Order: 1}); err != nil {
		t.Fatalf("save C: %v", err)
	}

	tree, err := s.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	gotNames := []string{tree[0].Name, tree[1].Name, tree[2].Name}
	wantNames := []string{"A", "C", "B"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("collection order = %v, want %v", gotNames, wantNames)
	}
}

// TestSaveRequestAppendsInOrder verifies incremental SaveRequest appends new
// requests after existing siblings and keeps that order on listing.
func TestSaveRequestAppendsInOrder(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := s.SaveCollection(model.Collection{ID: "c1", Name: "C"}); err != nil {
		t.Fatalf("SaveCollection: %v", err)
	}
	for _, name := range []string{"first", "second", "third"} {
		if _, err := s.SaveRequest(sampleRequest("", name), "c1"); err != nil {
			t.Fatalf("SaveRequest %s: %v", name, err)
		}
	}

	tree, err := s.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	got := []string{}
	for _, r := range tree[0].Requests {
		got = append(got, r.Name)
	}
	want := []string{"first", "second", "third"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request order = %v, want %v", got, want)
	}
}

// TestSaveRequestIntoFolder verifies a request saved with a folder parent lands
// inside that folder and inherits the folder's collection.
func TestSaveRequestIntoFolder(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := s.SaveCollection(model.Collection{
		ID:      "c1",
		Name:    "C",
		Folders: []model.Folder{{ID: "f1", Name: "F", Folders: []model.Folder{}, Requests: []model.SavedRequest{}}},
	}); err != nil {
		t.Fatalf("SaveCollection: %v", err)
	}

	if _, err := s.SaveRequest(sampleRequest("r1", "in-folder"), "f1"); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	tree, err := s.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if len(tree[0].Requests) != 0 {
		t.Fatalf("collection should have no top-level requests, got %d", len(tree[0].Requests))
	}
	if len(tree[0].Folders) != 1 || len(tree[0].Folders[0].Requests) != 1 {
		t.Fatalf("folder should contain exactly one request, got %#v", tree[0].Folders)
	}
	if tree[0].Folders[0].Requests[0].Name != "in-folder" {
		t.Fatalf("request name = %q, want in-folder", tree[0].Folders[0].Requests[0].Name)
	}
}

// TestSaveRequestUpdatesInPlace verifies re-saving an existing request ID updates
// it (upsert) and keeps its position rather than duplicating.
func TestSaveRequestUpdatesInPlace(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := s.SaveCollection(model.Collection{ID: "c1", Name: "C"}); err != nil {
		t.Fatalf("SaveCollection: %v", err)
	}
	if _, err := s.SaveRequest(sampleRequest("r1", "alpha"), "c1"); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}
	if _, err := s.SaveRequest(sampleRequest("r2", "beta"), "c1"); err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}

	// Update r1's name; it should stay first.
	updated := sampleRequest("r1", "alpha-renamed")
	if _, err := s.SaveRequest(updated, "c1"); err != nil {
		t.Fatalf("SaveRequest update: %v", err)
	}

	tree, err := s.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	reqs := tree[0].Requests
	if len(reqs) != 2 {
		t.Fatalf("got %d requests, want 2 (no duplication)", len(reqs))
	}
	if reqs[0].ID != "r1" || reqs[0].Name != "alpha-renamed" {
		t.Fatalf("first request = %q/%q, want r1/alpha-renamed", reqs[0].ID, reqs[0].Name)
	}
	if reqs[1].ID != "r2" {
		t.Fatalf("second request ID = %q, want r2", reqs[1].ID)
	}
}

// TestSaveAssignsIDsWhenBlank verifies blank IDs are populated on save.
func TestSaveAssignsIDsWhenBlank(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	saved, err := s.SaveCollection(model.Collection{Name: "No ID"})
	if err != nil {
		t.Fatalf("SaveCollection: %v", err)
	}
	if saved.ID == "" {
		t.Fatalf("collection ID was not assigned")
	}

	r, err := s.SaveRequest(sampleRequest("", "no-id"), saved.ID)
	if err != nil {
		t.Fatalf("SaveRequest: %v", err)
	}
	if r.ID == "" {
		t.Fatalf("request ID was not assigned")
	}
}

// TestRenameCollection verifies rename updates the name and reports ErrNotFound
// for a missing ID.
func TestRenameCollection(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := s.SaveCollection(model.Collection{ID: "c1", Name: "Old"}); err != nil {
		t.Fatalf("SaveCollection: %v", err)
	}
	if err := s.RenameCollection("c1", "New"); err != nil {
		t.Fatalf("RenameCollection: %v", err)
	}
	tree, err := s.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if tree[0].Name != "New" {
		t.Fatalf("name = %q, want New", tree[0].Name)
	}

	if err := s.RenameCollection("missing", "X"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RenameCollection(missing) error = %v, want ErrNotFound", err)
	}
}

// TestDeleteCollectionCascades verifies deleting a collection removes its
// folders and requests, and that deleting a missing ID reports ErrNotFound.
func TestDeleteCollectionCascades(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	c := model.Collection{
		ID:   "c1",
		Name: "C",
		Folders: []model.Folder{
			{ID: "f1", Name: "F", Folders: []model.Folder{}, Requests: []model.SavedRequest{sampleRequest("r1", "r1")}},
		},
		Requests: []model.SavedRequest{sampleRequest("r2", "r2")},
	}
	if _, err := s.SaveCollection(c); err != nil {
		t.Fatalf("SaveCollection: %v", err)
	}

	if err := s.DeleteCollection("c1"); err != nil {
		t.Fatalf("DeleteCollection: %v", err)
	}

	tree, err := s.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if len(tree) != 0 {
		t.Fatalf("got %d collections after delete, want 0", len(tree))
	}
	// Cascade should have removed the folder and request rows.
	for _, table := range []string{"folders", "requests"} {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Fatalf("%s rows = %d after cascade delete, want 0", table, n)
		}
	}

	if err := s.DeleteCollection("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteCollection(missing) error = %v, want ErrNotFound", err)
	}
}

// TestSaveRequestUnknownParent verifies an unknown parent ID is rejected and
// leaves no data behind.
func TestSaveRequestUnknownParent(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := s.SaveRequest(sampleRequest("r1", "orphan"), "nope"); !errors.Is(err, ErrUnknownParent) {
		t.Fatalf("SaveRequest(unknown parent) error = %v, want ErrUnknownParent", err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM requests`).Scan(&n); err != nil {
		t.Fatalf("count requests: %v", err)
	}
	if n != 0 {
		t.Fatalf("requests rows = %d, want 0 (rejected save must not persist)", n)
	}
}

// TestSaveFolderRoundTrip verifies incremental folder save under a collection
// and under another folder, preserving nesting.
func TestSaveFolderRoundTrip(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := s.SaveCollection(model.Collection{ID: "c1", Name: "C"}); err != nil {
		t.Fatalf("SaveCollection: %v", err)
	}
	parent, err := s.SaveFolder(model.Folder{ID: "f1", Name: "Parent", Folders: []model.Folder{}, Requests: []model.SavedRequest{}}, "c1")
	if err != nil {
		t.Fatalf("SaveFolder parent: %v", err)
	}
	if _, err := s.SaveFolder(model.Folder{ID: "f2", Name: "Child", Folders: []model.Folder{}, Requests: []model.SavedRequest{}}, parent.ID); err != nil {
		t.Fatalf("SaveFolder child: %v", err)
	}

	tree, err := s.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if len(tree[0].Folders) != 1 || tree[0].Folders[0].Name != "Parent" {
		t.Fatalf("expected one top folder Parent, got %#v", tree[0].Folders)
	}
	if len(tree[0].Folders[0].Folders) != 1 || tree[0].Folders[0].Folders[0].Name != "Child" {
		t.Fatalf("expected nested Child folder, got %#v", tree[0].Folders[0].Folders)
	}
}
