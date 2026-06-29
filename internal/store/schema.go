// Package store provides durable, Go-managed persistence for Volt's
// Collections, Folders, Requests, Environments, Variables, History, and
// Settings. It is backed by SQLite through the pure-Go driver
// modernc.org/sqlite (no cgo) so it links cleanly under gomobile for the
// Android build. All write operations run inside transactions that roll back
// on failure, so a failed save or delete leaves prior data intact
// (Req 5.9, 6.9).
package store

// SchemaVersion is the current version of the on-disk schema. It is written to
// the meta table on first initialization and used by future migrations to
// detect and upgrade older databases.
const SchemaVersion = 1

// Meta table keys. The meta table is a simple key/value store holding
// database-wide bookkeeping values.
const (
	// metaKeySchemaVersion records the integer SchemaVersion the database was
	// created or last migrated to.
	metaKeySchemaVersion = "schema_version"
	// metaKeyLegacyHistoryMigrated records whether the one-time localStorage→store
	// History migration has completed (Req 7.8). Stored as "1" once migration
	// succeeds; absent or "0" otherwise.
	metaKeyLegacyHistoryMigrated = "legacy_history_migrated"
)

// schemaStatements is the ordered list of DDL statements that create the full
// schema. They are executed inside a single transaction during initialization
// so the database is either fully created or not created at all.
//
// Design notes:
//   - Collections, Folders, and Requests form a tree. Ordering within a parent
//     is captured by the integer "ord" column so ListTree can reproduce the
//     saved order (Req 5.6). Nesting is expressed by parent IDs: a Folder
//     belongs to a Collection and optionally to a parent Folder; a Request
//     belongs to a Collection and optionally to a Folder.
//   - The full request configuration (params, headers, body, auth) is stored as
//     a JSON document in requests.data, mirroring model.RawRequest, while method
//     and url are duplicated into their own columns for cheap listing/indexing.
//   - Environments hold an "active" flag; at most one row is active at a time
//     (enforced by the environment CRUD layer, Req 6.3/6.10).
//   - Variables are keyed by (environment_id, name) so a name is unique within
//     its environment (Req 6.2), with "ord" preserving display order.
//   - Settings is a single-row table (id pinned to 1).
//   - Foreign keys use ON DELETE CASCADE so deleting a Collection or Folder
//     removes its descendants, and deleting an Environment removes its
//     Variables.
var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS collections (
		id   TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		ord  INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS folders (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
		parent_id     TEXT REFERENCES folders(id) ON DELETE CASCADE,
		ord           INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS requests (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
		folder_id     TEXT REFERENCES folders(id) ON DELETE CASCADE,
		ord           INTEGER NOT NULL DEFAULT 0,
		method        TEXT NOT NULL,
		url           TEXT NOT NULL,
		data          TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS environments (
		id     TEXT PRIMARY KEY,
		name   TEXT NOT NULL,
		active INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS variables (
		environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
		name           TEXT NOT NULL,
		value          TEXT NOT NULL,
		ord            INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (environment_id, name)
	)`,

	`CREATE TABLE IF NOT EXISTS history (
		id          TEXT PRIMARY KEY,
		method      TEXT NOT NULL,
		url         TEXT NOT NULL,
		status      INTEGER NOT NULL DEFAULT 0,
		duration_ms INTEGER NOT NULL DEFAULT 0,
		at          INTEGER NOT NULL DEFAULT 0,
		error       TEXT NOT NULL DEFAULT '',
		request     TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS settings (
		id              INTEGER PRIMARY KEY CHECK (id = 1),
		theme           TEXT NOT NULL,
		tls_verify      INTEGER NOT NULL,
		timeout_seconds INTEGER NOT NULL,
		proxy_url       TEXT NOT NULL DEFAULT ''
	)`,

	// Indexes that the tree, environment, and history queries rely on.
	`CREATE INDEX IF NOT EXISTS idx_folders_collection ON folders(collection_id)`,
	`CREATE INDEX IF NOT EXISTS idx_folders_parent ON folders(parent_id)`,
	`CREATE INDEX IF NOT EXISTS idx_requests_collection ON requests(collection_id)`,
	`CREATE INDEX IF NOT EXISTS idx_requests_folder ON requests(folder_id)`,
	`CREATE INDEX IF NOT EXISTS idx_variables_environment ON variables(environment_id)`,
	`CREATE INDEX IF NOT EXISTS idx_history_at ON history(at)`,
}
