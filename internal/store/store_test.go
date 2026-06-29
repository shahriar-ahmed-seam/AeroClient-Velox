package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// TestOpenCreatesSchemaAndVersion verifies that Open creates the database,
// installs every expected table, and records the current schema version.
func TestOpenCreatesSchemaAndVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "volt.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	v, err := s.schemaVersion()
	if err != nil {
		t.Fatalf("schemaVersion: %v", err)
	}
	if v != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", v, SchemaVersion)
	}

	wantTables := []string{
		"collections", "folders", "requests",
		"environments", "variables", "history", "settings", "meta",
	}
	for _, name := range wantTables {
		var got string
		err := s.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name,
		).Scan(&got)
		if err != nil {
			t.Fatalf("expected table %q to exist: %v", name, err)
		}
	}
}

// TestReopenRetainsData confirms the schema and recorded version persist across
// reopening the same file (the foundation of the persistence round-trip).
func TestReopenRetainsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "volt.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	v, err := s2.schemaVersion()
	if err != nil {
		t.Fatalf("schemaVersion after reopen: %v", err)
	}
	if v != SchemaVersion {
		t.Fatalf("schema version after reopen = %d, want %d", v, SchemaVersion)
	}
}

// TestWithTxRollsBackOnError verifies the transaction helper rolls back when the
// callback returns an error, leaving no committed data behind (Req 5.9).
func TestWithTxRollsBackOnError(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	sentinel := errors.New("boom")
	err = s.withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`INSERT INTO collections(id, name, ord) VALUES (?, ?, ?)`,
			"c1", "Temp", 0,
		); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("withTx error = %v, want %v", err, sentinel)
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM collections`).Scan(&count); err != nil {
		t.Fatalf("count collections: %v", err)
	}
	if count != 0 {
		t.Fatalf("collections count = %d, want 0 (rollback should discard the insert)", count)
	}
}

// TestWithTxCommitsOnSuccess verifies the transaction helper commits when the
// callback succeeds.
func TestWithTxCommitsOnSuccess(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if err := s.withTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO collections(id, name, ord) VALUES (?, ?, ?)`,
			"c1", "Kept", 0,
		)
		return err
	}); err != nil {
		t.Fatalf("withTx: %v", err)
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM collections`).Scan(&count); err != nil {
		t.Fatalf("count collections: %v", err)
	}
	if count != 1 {
		t.Fatalf("collections count = %d, want 1", count)
	}
}
