package store

import (
	"errors"
	"testing"

	"volt/internal/model"
)

// newTestStore opens a fresh in-memory store for a test, failing the test on
// any open error and closing it on cleanup.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// findEnv returns the Environment with the given ID from a slice, or false.
func findEnv(envs []model.Environment, id string) (model.Environment, bool) {
	for _, e := range envs {
		if e.ID == id {
			return e, true
		}
	}
	return model.Environment{}, false
}

// TestSaveEnvironmentRoundTrip verifies a saved Environment and its Variables
// round-trip through ListEnvironments preserving name, variable order, values,
// and the active flag (Req 6.7).
func TestSaveEnvironmentRoundTrip(t *testing.T) {
	s := newTestStore(t)

	in := model.Environment{
		Name:   "Dev",
		Active: true,
		Variables: []model.Variable{
			{Name: "host", Value: "localhost"},
			{Name: "port", Value: "8080"},
			{Name: "token", Value: ""},
		},
	}
	saved, err := s.SaveEnvironment(in)
	if err != nil {
		t.Fatalf("SaveEnvironment: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("SaveEnvironment did not assign an ID")
	}

	envs, err := s.ListEnvironments()
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}
	got, ok := findEnv(envs, saved.ID)
	if !ok {
		t.Fatalf("saved environment %q not found in list", saved.ID)
	}
	if got.Name != "Dev" {
		t.Fatalf("name = %q, want Dev", got.Name)
	}
	if !got.Active {
		t.Fatal("active flag not persisted")
	}
	if len(got.Variables) != 3 {
		t.Fatalf("variable count = %d, want 3", len(got.Variables))
	}
	wantOrder := []model.Variable{
		{Name: "host", Value: "localhost"},
		{Name: "port", Value: "8080"},
		{Name: "token", Value: ""},
	}
	for i, w := range wantOrder {
		if got.Variables[i] != w {
			t.Fatalf("variable[%d] = %+v, want %+v", i, got.Variables[i], w)
		}
	}
}

// TestSaveEnvironmentUpsertRebuildsVariables verifies saving an existing
// Environment replaces its variable set rather than accumulating it.
func TestSaveEnvironmentUpsertRebuildsVariables(t *testing.T) {
	s := newTestStore(t)

	saved, err := s.SaveEnvironment(model.Environment{
		Name: "Env",
		Variables: []model.Variable{
			{Name: "a", Value: "1"},
			{Name: "b", Value: "2"},
		},
	})
	if err != nil {
		t.Fatalf("SaveEnvironment: %v", err)
	}

	saved.Variables = []model.Variable{{Name: "c", Value: "3"}}
	if _, err := s.SaveEnvironment(saved); err != nil {
		t.Fatalf("SaveEnvironment (update): %v", err)
	}

	envs, err := s.ListEnvironments()
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}
	got, _ := findEnv(envs, saved.ID)
	if len(got.Variables) != 1 || got.Variables[0].Name != "c" {
		t.Fatalf("variables = %+v, want only {c:3}", got.Variables)
	}
}

// TestSaveEnvironmentDuplicateNameRejected verifies cross-Environment name
// uniqueness is enforced and the rejected save leaves prior data unchanged
// (Req 6.1, 6.9).
func TestSaveEnvironmentDuplicateNameRejected(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.SaveEnvironment(model.Environment{Name: "Prod"}); err != nil {
		t.Fatalf("first SaveEnvironment: %v", err)
	}
	_, err := s.SaveEnvironment(model.Environment{Name: "Prod"})
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("duplicate name error = %v, want ErrDuplicateName", err)
	}

	envs, err := s.ListEnvironments()
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("environment count = %d, want 1 (duplicate must not be added)", len(envs))
	}
}

// TestSaveEnvironmentRenameAllowsSameName verifies that re-saving the same
// Environment with its own name is not treated as a duplicate.
func TestSaveEnvironmentRenameAllowsSameName(t *testing.T) {
	s := newTestStore(t)

	saved, err := s.SaveEnvironment(model.Environment{Name: "Staging"})
	if err != nil {
		t.Fatalf("SaveEnvironment: %v", err)
	}
	if _, err := s.SaveEnvironment(saved); err != nil {
		t.Fatalf("re-save with same name should be allowed, got: %v", err)
	}
}

// TestSaveEnvironmentInvalidRejected verifies validation runs before any write
// for an out-of-bounds name and a duplicate variable name (Req 6.1, 6.2, 6.9).
func TestSaveEnvironmentInvalidRejected(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.SaveEnvironment(model.Environment{Name: ""}); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("empty name error = %v, want ErrInvalidName", err)
	}

	_, err := s.SaveEnvironment(model.Environment{
		Name: "Env",
		Variables: []model.Variable{
			{Name: "dup", Value: "1"},
			{Name: "dup", Value: "2"},
		},
	})
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("duplicate variable error = %v, want ErrDuplicateName", err)
	}

	envs, err := s.ListEnvironments()
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}
	if len(envs) != 0 {
		t.Fatalf("environment count = %d, want 0 (invalid saves must not persist)", len(envs))
	}
}

// TestSetActiveEnvironmentSingleActive verifies that activating an Environment
// makes it the only active one (Req 6.3).
func TestSetActiveEnvironmentSingleActive(t *testing.T) {
	s := newTestStore(t)

	a, _ := s.SaveEnvironment(model.Environment{Name: "A", Active: true})
	b, _ := s.SaveEnvironment(model.Environment{Name: "B"})

	if err := s.SetActiveEnvironment(b.ID); err != nil {
		t.Fatalf("SetActiveEnvironment: %v", err)
	}

	envs, _ := s.ListEnvironments()
	active := activeCount(envs)
	if active != 1 {
		t.Fatalf("active count = %d, want 1", active)
	}
	got, _ := findEnv(envs, b.ID)
	if !got.Active {
		t.Fatal("B should be active")
	}
	gotA, _ := findEnv(envs, a.ID)
	if gotA.Active {
		t.Fatal("A should no longer be active")
	}
}

// TestSetActiveEnvironmentClear verifies passing an empty id clears the active
// selection (Req 6.3).
func TestSetActiveEnvironmentClear(t *testing.T) {
	s := newTestStore(t)

	s.SaveEnvironment(model.Environment{Name: "A", Active: true})

	if err := s.SetActiveEnvironment(""); err != nil {
		t.Fatalf("SetActiveEnvironment(\"\"): %v", err)
	}
	envs, _ := s.ListEnvironments()
	if c := activeCount(envs); c != 0 {
		t.Fatalf("active count = %d, want 0 after clearing", c)
	}
}

// TestSetActiveEnvironmentNotFound verifies activating a missing Environment
// reports ErrNotFound and changes no active state.
func TestSetActiveEnvironmentNotFound(t *testing.T) {
	s := newTestStore(t)

	a, _ := s.SaveEnvironment(model.Environment{Name: "A", Active: true})

	if err := s.SetActiveEnvironment("does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	envs, _ := s.ListEnvironments()
	got, _ := findEnv(envs, a.ID)
	if !got.Active {
		t.Fatal("A should still be active after a failed activation")
	}
}

// TestDeleteActiveEnvironmentClearsActive verifies that deleting the active
// Environment leaves no Environment active (Req 6.10) and cascades variables.
func TestDeleteActiveEnvironmentClearsActive(t *testing.T) {
	s := newTestStore(t)

	a, _ := s.SaveEnvironment(model.Environment{
		Name:      "A",
		Active:    true,
		Variables: []model.Variable{{Name: "k", Value: "v"}},
	})
	s.SaveEnvironment(model.Environment{Name: "B"})

	if err := s.DeleteEnvironment(a.ID); err != nil {
		t.Fatalf("DeleteEnvironment: %v", err)
	}

	envs, _ := s.ListEnvironments()
	if c := activeCount(envs); c != 0 {
		t.Fatalf("active count = %d, want 0 after deleting active env", c)
	}
	if _, ok := findEnv(envs, a.ID); ok {
		t.Fatal("deleted environment should not be listed")
	}

	// Variables of the deleted environment must be gone (cascade).
	var varCount int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM variables WHERE environment_id = ?`, a.ID,
	).Scan(&varCount); err != nil {
		t.Fatalf("count variables: %v", err)
	}
	if varCount != 0 {
		t.Fatalf("variable count = %d, want 0 (cascade delete)", varCount)
	}
}

// TestDeleteEnvironmentNotFound verifies deleting a missing Environment reports
// ErrNotFound.
func TestDeleteEnvironmentNotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.DeleteEnvironment("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

// TestListEnvironmentsEmpty verifies an empty store returns an empty,
// non-nil slice.
func TestListEnvironmentsEmpty(t *testing.T) {
	s := newTestStore(t)
	envs, err := s.ListEnvironments()
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}
	if envs == nil {
		t.Fatal("ListEnvironments returned nil, want empty slice")
	}
	if len(envs) != 0 {
		t.Fatalf("len = %d, want 0", len(envs))
	}
}

// activeCount returns how many Environments are flagged active.
func activeCount(envs []model.Environment) int {
	n := 0
	for _, e := range envs {
		if e.Active {
			n++
		}
	}
	return n
}
