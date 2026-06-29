package store

import (
	"fmt"
	"math/rand"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 13: Moving a request leaves it in exactly one location
//
// Validates: Requirements 5.4
//
// TestMoveRequestLeavesExactlyOneLocation is the property-based test for
// Property 13: for any valid collection tree (collections, folders, and
// requests nested within the 10-level limit), moving an existing request to any
// existing parent (a collection or a folder) leaves that request present in
// exactly one location across the whole tree, and that location is directly
// under the chosen target parent.
//
// It reuses the valid-data generators from persistence_prop_test.go so every
// setup Save succeeds, and a fresh in-memory store per iteration keeps the
// iterations independent. Iterations whose generated tree has no request or no
// parent carry no move to exercise and pass trivially.
func TestMoveRequestLeavesExactlyOneLocation(t *testing.T) {
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))
		counter := 0

		cols := genCollections(rng, &counter)

		// Candidate requests to move and candidate target parents (collection
		// IDs plus every folder ID) are read straight from the generated model,
		// whose IDs are already assigned by the generators.
		requestIDs := collectRequestIDs(cols)
		parentIDs := collectParentIDs(cols)
		if len(requestIDs) == 0 || len(parentIDs) == 0 {
			// Nothing to move; the property is vacuously satisfied.
			return true
		}

		s, err := Open(":memory:")
		if err != nil {
			failMsg = fmt.Sprintf("Open: %v", err)
			return false
		}
		defer s.Close()

		for i := range cols {
			if _, err := s.SaveCollection(cols[i]); err != nil {
				failMsg = fmt.Sprintf("SaveCollection %q: %v", cols[i].ID, err)
				return false
			}
		}

		reqID := requestIDs[rng.Intn(len(requestIDs))]
		targetParentID := parentIDs[rng.Intn(len(parentIDs))]

		if err := s.MoveRequest(reqID, targetParentID); err != nil {
			failMsg = fmt.Sprintf("MoveRequest(%q -> %q): %v", reqID, targetParentID, err)
			return false
		}

		tree, err := s.ListTree()
		if err != nil {
			failMsg = fmt.Sprintf("ListTree: %v", err)
			return false
		}

		// Walk the whole reconstructed tree, counting occurrences of the moved
		// request and recording the parent that directly contains it.
		count, foundParent := locateRequest(tree, reqID)
		if count != 1 {
			failMsg = fmt.Sprintf(
				"moved request %q appears %d times in the tree, want exactly 1",
				reqID, count)
			return false
		}
		if foundParent != targetParentID {
			failMsg = fmt.Sprintf(
				"moved request %q is under parent %q, want target parent %q",
				reqID, foundParent, targetParentID)
			return false
		}
		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 13 failed: %v\n%s", err, failMsg)
	}
}

// collectRequestIDs returns the IDs of every saved request anywhere in the
// generated collection tree (collection-level and folder-nested).
func collectRequestIDs(cols []model.Collection) []string {
	var ids []string
	for _, c := range cols {
		for _, r := range c.Requests {
			ids = append(ids, r.ID)
		}
		for _, f := range c.Folders {
			ids = append(ids, folderRequestIDs(f)...)
		}
	}
	return ids
}

// folderRequestIDs returns the request IDs in f and all of its descendant
// folders.
func folderRequestIDs(f model.Folder) []string {
	var ids []string
	for _, r := range f.Requests {
		ids = append(ids, r.ID)
	}
	for _, child := range f.Folders {
		ids = append(ids, folderRequestIDs(child)...)
	}
	return ids
}

// collectParentIDs returns every valid target-parent ID in the tree: each
// collection ID plus every folder ID at any depth.
func collectParentIDs(cols []model.Collection) []string {
	var ids []string
	for _, c := range cols {
		ids = append(ids, c.ID)
		for _, f := range c.Folders {
			ids = append(ids, folderIDs(f)...)
		}
	}
	return ids
}

// folderIDs returns the ID of f and all of its descendant folders.
func folderIDs(f model.Folder) []string {
	ids := []string{f.ID}
	for _, child := range f.Folders {
		ids = append(ids, folderIDs(child)...)
	}
	return ids
}

// locateRequest walks the reconstructed tree and returns how many times a
// request with id appears as a direct child of any collection or folder, along
// with the ID of the parent that directly contains the last-seen occurrence.
func locateRequest(cols []model.Collection, id string) (count int, parentID string) {
	for _, c := range cols {
		for _, r := range c.Requests {
			if r.ID == id {
				count++
				parentID = c.ID
			}
		}
		for _, f := range c.Folders {
			fc, fp := locateRequestInFolder(f, id)
			count += fc
			if fc > 0 {
				parentID = fp
			}
		}
	}
	return count, parentID
}

// locateRequestInFolder counts occurrences of id within f and its descendant
// folders, returning the directly-containing folder ID for the last-seen one.
func locateRequestInFolder(f model.Folder, id string) (count int, parentID string) {
	for _, r := range f.Requests {
		if r.ID == id {
			count++
			parentID = f.ID
		}
	}
	for _, child := range f.Folders {
		cc, cp := locateRequestInFolder(child, id)
		count += cc
		if cc > 0 {
			parentID = cp
		}
	}
	return count, parentID
}
