package store

import (
	"testing"

	"volt/internal/model"
)

// legacyEntry builds a minimal legacy History entry for migration tests.
func legacyEntry(id, url string) model.HistoryEntry {
	return model.HistoryEntry{
		ID:     id,
		Method: model.MethodGet,
		URL:    url,
		Status: 200,
		At:     1, // non-zero so ordering is deterministic
		Request: model.RawRequest{
			Method: model.MethodGet,
			URL:    url,
		},
	}
}

// TestMigrateLegacyHistoryImportsOnce verifies the first migration imports every
// legacy entry and marks the flag complete (Req 7.8).
func TestMigrateLegacyHistoryImportsOnce(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	entries := []model.HistoryEntry{
		legacyEntry("h1", "https://a.example"),
		legacyEntry("h2", "https://b.example"),
	}

	migrated, err := s.MigrateLegacyHistory(entries)
	if err != nil {
		t.Fatalf("MigrateLegacyHistory: %v", err)
	}
	if !migrated {
		t.Fatalf("migrated = false, want true on first run")
	}

	got, err := s.ListHistory()
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("history count = %d, want 2", len(got))
	}

	done, err := s.isLegacyHistoryMigrated()
	if err != nil {
		t.Fatalf("isLegacyHistoryMigrated: %v", err)
	}
	if !done {
		t.Fatalf("flag not set after successful migration")
	}
}

// TestMigrateLegacyHistoryIsIdempotent verifies a second migration does not
// re-import and leaves the migrated History unchanged (Req 7.8 — exactly once).
func TestMigrateLegacyHistoryIsIdempotent(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	entries := []model.HistoryEntry{legacyEntry("h1", "https://a.example")}

	if _, err := s.MigrateLegacyHistory(entries); err != nil {
		t.Fatalf("first MigrateLegacyHistory: %v", err)
	}

	// A second run, even with different entries, must not import anything.
	migrated, err := s.MigrateLegacyHistory([]model.HistoryEntry{
		legacyEntry("h2", "https://b.example"),
		legacyEntry("h3", "https://c.example"),
	})
	if err != nil {
		t.Fatalf("second MigrateLegacyHistory: %v", err)
	}
	if migrated {
		t.Fatalf("migrated = true on second run, want false (already migrated)")
	}

	got, err := s.ListHistory()
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("history count = %d, want 1 (no re-import)", len(got))
	}
	if got[0].ID != "h1" {
		t.Fatalf("retained entry = %q, want h1", got[0].ID)
	}
}

// TestMigrateLegacyHistoryFailureLeavesFlagUnset verifies that when an insert
// fails the transaction rolls back: no entries are imported and the flag stays
// unset so a later launch can retry (Req 7.9).
func TestMigrateLegacyHistoryFailureLeavesFlagUnset(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// Two entries sharing the same ID violate the history primary key, forcing
	// the second insert to fail and the whole migration to roll back.
	entries := []model.HistoryEntry{
		legacyEntry("dup", "https://a.example"),
		legacyEntry("dup", "https://b.example"),
	}

	migrated, err := s.MigrateLegacyHistory(entries)
	if err == nil {
		t.Fatalf("MigrateLegacyHistory succeeded, want error on duplicate ID")
	}
	if migrated {
		t.Fatalf("migrated = true on failure, want false")
	}

	got, err := s.ListHistory()
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("history count = %d, want 0 (rollback should discard inserts)", len(got))
	}

	done, err := s.isLegacyHistoryMigrated()
	if err != nil {
		t.Fatalf("isLegacyHistoryMigrated: %v", err)
	}
	if done {
		t.Fatalf("flag set after failed migration, want unset for retry")
	}

	// A retry with valid data should now succeed.
	migrated, err = s.MigrateLegacyHistory([]model.HistoryEntry{
		legacyEntry("h1", "https://a.example"),
	})
	if err != nil {
		t.Fatalf("retry MigrateLegacyHistory: %v", err)
	}
	if !migrated {
		t.Fatalf("retry migrated = false, want true")
	}
}
