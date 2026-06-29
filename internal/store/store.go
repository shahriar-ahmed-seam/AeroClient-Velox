package store

import (
	"database/sql"
	"fmt"

	// modernc.org/sqlite registers the pure-Go "sqlite" database/sql driver.
	// It uses no cgo, which keeps the package linkable under gomobile for the
	// Android build.
	_ "modernc.org/sqlite"
)

// driverName is the database/sql driver name registered by modernc.org/sqlite.
const driverName = "sqlite"

// Store is the SQLite-backed persistence layer. It owns a *sql.DB connection
// pool and exposes the CRUD operations used by the bindings. Construct one with
// Open and release it with Close.
//
// All mutating operations are expected to run through withTx so that a failure
// rolls back and leaves prior data intact (Req 5.9, 6.9). CRUD methods are
// implemented by later tasks; this type provides the database handle, schema
// initialization, and the transaction helper they build on.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path, enabling foreign-key
// enforcement, ensures the schema exists, and records the current schema
// version. The special path ":memory:" opens a private in-memory database,
// which is convenient for tests.
//
// On any failure the partially opened handle is closed and the error is
// returned, so a caller never receives a half-initialized Store.
func Open(path string) (*Store, error) {
	db, err := sql.Open(driverName, dsn(path))
	if err != nil {
		return nil, fmt.Errorf("store: open database: %w", err)
	}

	// SQLite permits a single writer at a time. Serializing access to one
	// connection avoids "database is locked" errors from concurrent writers
	// and keeps transaction semantics simple across the CRUD layer.
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.init(); err != nil {
		// Best-effort close; the init error is the meaningful one.
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// dsn builds the modernc.org/sqlite DSN for path. Foreign-key enforcement is
// turned on per connection via the _pragma query parameter so ON DELETE CASCADE
// relationships are honored. The driver treats everything before the first "?"
// as the filename, so a bare OS path (including a Windows drive path) is passed
// through unescaped while the pragma travels in the query string.
func dsn(path string) string {
	if path == ":memory:" {
		// A shared in-memory database so the single pooled connection sees a
		// consistent database for the lifetime of the Store.
		return "file::memory:?cache=shared&_pragma=foreign_keys(1)"
	}
	return path + "?_pragma=foreign_keys(1)"
}

// init creates the schema if absent and records the schema version. The DDL and
// the version write run inside a single transaction so initialization is all or
// nothing.
func (s *Store) init() error {
	return s.withTx(func(tx *sql.Tx) error {
		for _, stmt := range schemaStatements {
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("store: create schema: %w", err)
			}
		}
		// Record the schema version once, on first creation. INSERT OR IGNORE
		// leaves an existing (possibly future-migrated) value untouched.
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO meta(key, value) VALUES (?, ?)`,
			metaKeySchemaVersion, fmt.Sprintf("%d", SchemaVersion),
		); err != nil {
			return fmt.Errorf("store: set schema version: %w", err)
		}
		return nil
	})
}

// Close releases the underlying database connections. It is safe to call on a
// nil *Store.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// withTx runs fn inside a single transaction. If fn returns an error (or the
// commit fails) the transaction is rolled back and the error is returned, so a
// failed write leaves prior data intact (Req 5.9, 6.9). This is the foundation
// the CRUD operations in later tasks build on for atomic, all-or-nothing
// writes.
func (s *Store) withTx(fn func(*sql.Tx) error) (err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin transaction: %w", err)
	}

	// Guard against a panic in fn leaving an open transaction.
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%w (rollback failed: %v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit transaction: %w", err)
	}
	return nil
}

// schemaVersion reads the recorded schema version from the meta table. It is
// used by tests and future migrations to detect the on-disk version.
func (s *Store) schemaVersion() (int, error) {
	var v int
	err := s.db.QueryRow(
		`SELECT value FROM meta WHERE key = ?`, metaKeySchemaVersion,
	).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("store: read schema version: %w", err)
	}
	return v, nil
}
