// Package mobile is the gomobile-bindable facade over Volt's shared Go core
// (internal/httpcore + internal/store). It exists so the Android build can call
// the exact same request-preparation, execution, and persistence logic as the
// desktop (Wails) build, guaranteeing behavioral parity across platforms
// (Req 15.1, 15.2, 15.5).
//
// gomobile bind supports only a restricted set of parameter and return types
// (string, the numeric types, bool, and error) and cannot bind arbitrary
// structs, slices, or maps. Every method here therefore takes and returns JSON
// strings for structured data: the Capacitor bridge plugin marshals JavaScript
// objects to JSON, calls a Bridge method, and unmarshals the JSON result. The
// JSON shapes are exactly the model types shared with the desktop path, so a
// request and response serialize identically on both platforms.
//
// Error handling convention:
//   - Execute always returns a JSON model.HTTPResponse. Any internal failure
//     (bad input JSON, store read error, URL validation error) is surfaced in
//     that response's "error" field rather than as a separate error channel, so
//     the caller renders it the same way it renders a network failure.
//   - Data-returning CRUD methods return the result as JSON on success, or a
//     {"error":"..."} envelope on failure.
//   - Action-only CRUD methods (rename/delete/move/etc.) return {"ok":true} on
//     success, or a {"error":"..."} envelope on failure.
package mobile

import (
	"context"
	"encoding/json"
	"time"

	"volt/internal/httpcore"
	"volt/internal/model"
	"volt/internal/store"
)

// Bridge is the gomobile-bindable entry point. It owns the SQLite-backed store
// and exposes string-in/string-out methods mirroring the Wails bindings. A
// single Bridge is constructed per app session with NewBridge and released with
// Close. It is an exported type so gomobile binds its constructor and methods.
type Bridge struct {
	store *store.Store
}

// NewBridge opens (or creates) the on-device SQLite database at dbPath and
// returns a ready Bridge. The pure-Go SQLite driver links cleanly under
// gomobile, so the same persistence layer used on desktop runs on Android,
// retaining Collections, Environments, and History across restarts (Req 15.5).
// The returned error is the only non-string value crossing the binding and is
// permitted by gomobile.
func NewBridge(dbPath string) (*Bridge, error) {
	s, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return &Bridge{store: s}, nil
}

// Close releases the underlying database connections. It is safe to call on a
// nil Bridge or one whose store failed to open.
func (b *Bridge) Close() error {
	if b == nil || b.store == nil {
		return nil
	}
	return b.store.Close()
}

// ---------------------------------------------------------------------------
// Request execution
// ---------------------------------------------------------------------------

// Execute prepares and sends the request described by reqJSON (a JSON-encoded
// model.RawRequest) through the shared httpcore engine and returns a
// JSON-encoded model.HTTPResponse. It mirrors the desktop ExecuteRequest path:
// the active Environment and Settings are loaded from the store, the request is
// prepared (variable interpolation, validation, param merge, auth, body
// encoding) and executed with the configured timeout and TLS setting, and the
// outcome is recorded in History (Req 15.1, 15.2).
//
// Because Android requests run through native Go rather than the WebView's
// fetch, they are not subject to browser CORS (Req 15.2).
//
// Any internal failure is returned as an HTTPResponse with its Error field set
// rather than failing the call, so the caller always receives a well-formed
// response JSON.
func (b *Bridge) Execute(reqJSON string) string {
	var raw model.RawRequest
	if err := json.Unmarshal([]byte(reqJSON), &raw); err != nil {
		return responseErrorJSON("invalid request JSON: " + err.Error())
	}

	settings, err := b.store.GetSettings()
	if err != nil {
		return responseErrorJSON("load settings: " + err.Error())
	}

	env, err := b.activeEnvironment()
	if err != nil {
		return responseErrorJSON("load environment: " + err.Error())
	}

	// Prepare resolves variables, validates the URL, merges params, derives
	// auth, and encodes the body. A validation error means no network call is
	// made (Req 1.3); it is surfaced as a failed response and still recorded in
	// history so the configuration is preserved (Req 7.2).
	prepared, err := httpcore.PrepareRequest(raw, env, settings)
	if err != nil {
		resp := model.HTTPResponse{Error: err.Error()}
		b.recordHistory(raw, resp)
		return marshalJSON(resp)
	}

	resp := httpcore.Execute(context.Background(), nil, prepared, settings)
	b.recordHistory(raw, resp)
	return marshalJSON(resp)
}

// activeEnvironment returns the single active Environment, or a zero
// Environment when none is active (in which case {{tokens}} resolve to nothing
// and are sent literally per Req 6.8).
func (b *Bridge) activeEnvironment() (model.Environment, error) {
	envs, err := b.store.ListEnvironments()
	if err != nil {
		return model.Environment{}, err
	}
	for i := range envs {
		if envs[i].Active {
			return envs[i], nil
		}
	}
	return model.Environment{}, nil
}

// recordHistory records the executed request and its outcome, capturing the
// full configuration for later restore (Req 7.1, 7.2). It is best-effort: a
// persistence failure must not break execution, so the error is ignored.
func (b *Bridge) recordHistory(raw model.RawRequest, resp model.HTTPResponse) {
	_ = b.store.AddHistory(model.HistoryEntry{
		Method:     raw.Method,
		URL:        raw.URL,
		Status:     resp.Status,
		DurationMs: resp.DurationMs,
		At:         time.Now().UnixMilli(),
		Error:      resp.Error,
		Request:    raw,
	})
}

// ---------------------------------------------------------------------------
// Collections
// ---------------------------------------------------------------------------

// SaveCollection persists the collection described by collectionJSON (a
// model.Collection) and returns the saved collection (with assigned IDs) as
// JSON, or an error envelope on failure.
func (b *Bridge) SaveCollection(collectionJSON string) string {
	var c model.Collection
	if err := json.Unmarshal([]byte(collectionJSON), &c); err != nil {
		return errorJSONMsg("invalid collection JSON: " + err.Error())
	}
	saved, err := b.store.SaveCollection(c)
	if err != nil {
		return errorJSON(err)
	}
	return marshalJSON(saved)
}

// RenameCollection renames the collection with the given id.
func (b *Bridge) RenameCollection(id, name string) string {
	return actionResult(b.store.RenameCollection(id, name))
}

// DeleteCollection removes the collection with the given id and its subtree.
func (b *Bridge) DeleteCollection(id string) string {
	return actionResult(b.store.DeleteCollection(id))
}

// ---------------------------------------------------------------------------
// Folders
// ---------------------------------------------------------------------------

// SaveFolder persists the folder described by folderJSON (a model.Folder) under
// parentID (a collection or folder ID) and returns the saved folder as JSON.
func (b *Bridge) SaveFolder(folderJSON, parentID string) string {
	var f model.Folder
	if err := json.Unmarshal([]byte(folderJSON), &f); err != nil {
		return errorJSONMsg("invalid folder JSON: " + err.Error())
	}
	saved, err := b.store.SaveFolder(f, parentID)
	if err != nil {
		return errorJSON(err)
	}
	return marshalJSON(saved)
}

// DeleteFolder removes the folder with the given id and its subtree.
func (b *Bridge) DeleteFolder(id string) string {
	return actionResult(b.store.DeleteFolder(id))
}

// ---------------------------------------------------------------------------
// Requests
// ---------------------------------------------------------------------------

// SaveRequest persists the request described by requestJSON (a
// model.SavedRequest) under parentID (a collection or folder ID) and returns
// the saved request as JSON.
func (b *Bridge) SaveRequest(requestJSON, parentID string) string {
	var r model.SavedRequest
	if err := json.Unmarshal([]byte(requestJSON), &r); err != nil {
		return errorJSONMsg("invalid request JSON: " + err.Error())
	}
	saved, err := b.store.SaveRequest(r, parentID)
	if err != nil {
		return errorJSON(err)
	}
	return marshalJSON(saved)
}

// DeleteRequest removes the saved request with the given id.
func (b *Bridge) DeleteRequest(id string) string {
	return actionResult(b.store.DeleteRequest(id))
}

// MoveRequest relocates the request with the given id into targetParentID.
func (b *Bridge) MoveRequest(requestID, targetParentID string) string {
	return actionResult(b.store.MoveRequest(requestID, targetParentID))
}

// ListTree returns the full collection tree (folders and requests in stored
// order) as a JSON array of model.Collection.
func (b *Bridge) ListTree() string {
	tree, err := b.store.ListTree()
	if err != nil {
		return errorJSON(err)
	}
	return marshalJSON(tree)
}

// ---------------------------------------------------------------------------
// Environments
// ---------------------------------------------------------------------------

// SaveEnvironment persists the environment described by environmentJSON (a
// model.Environment) and returns the saved environment as JSON.
func (b *Bridge) SaveEnvironment(environmentJSON string) string {
	var e model.Environment
	if err := json.Unmarshal([]byte(environmentJSON), &e); err != nil {
		return errorJSONMsg("invalid environment JSON: " + err.Error())
	}
	saved, err := b.store.SaveEnvironment(e)
	if err != nil {
		return errorJSON(err)
	}
	return marshalJSON(saved)
}

// DeleteEnvironment removes the environment with the given id.
func (b *Bridge) DeleteEnvironment(id string) string {
	return actionResult(b.store.DeleteEnvironment(id))
}

// SetActiveEnvironment makes the environment with the given id the single
// active environment; an empty id clears the active selection.
func (b *Bridge) SetActiveEnvironment(id string) string {
	return actionResult(b.store.SetActiveEnvironment(id))
}

// ListEnvironments returns every environment with its variables as a JSON array
// of model.Environment.
func (b *Bridge) ListEnvironments() string {
	envs, err := b.store.ListEnvironments()
	if err != nil {
		return errorJSON(err)
	}
	return marshalJSON(envs)
}

// ---------------------------------------------------------------------------
// History
// ---------------------------------------------------------------------------

// AddHistory records the history entry described by entryJSON (a
// model.HistoryEntry). Execute records history automatically; this method
// exists for parity with the desktop bindings.
func (b *Bridge) AddHistory(entryJSON string) string {
	var h model.HistoryEntry
	if err := json.Unmarshal([]byte(entryJSON), &h); err != nil {
		return errorJSONMsg("invalid history JSON: " + err.Error())
	}
	return actionResult(b.store.AddHistory(h))
}

// ListHistory returns recorded history entries newest-first as a JSON array of
// model.HistoryEntry.
func (b *Bridge) ListHistory() string {
	entries, err := b.store.ListHistory()
	if err != nil {
		return errorJSON(err)
	}
	return marshalJSON(entries)
}

// ClearHistory deletes all recorded history entries.
func (b *Bridge) ClearHistory() string {
	return actionResult(b.store.ClearHistory())
}

// MigrateLegacyHistory imports legacy localStorage history entries
// (entriesJSON, a JSON array of model.HistoryEntry) exactly once. It returns a
// JSON object {"migrated": <bool>} indicating whether this call performed the
// import, or an error envelope on failure.
func (b *Bridge) MigrateLegacyHistory(entriesJSON string) string {
	var entries []model.HistoryEntry
	if err := json.Unmarshal([]byte(entriesJSON), &entries); err != nil {
		return errorJSONMsg("invalid history JSON: " + err.Error())
	}
	migrated, err := b.store.MigrateLegacyHistory(entries)
	if err != nil {
		return errorJSON(err)
	}
	return marshalJSON(struct {
		Migrated bool `json:"migrated"`
	}{Migrated: migrated})
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

// GetSettings returns the effective settings (defaults on first launch) as a
// JSON model.Settings.
func (b *Bridge) GetSettings() string {
	s, err := b.store.GetSettings()
	if err != nil {
		return errorJSON(err)
	}
	return marshalJSON(s)
}

// SaveSettings persists the settings described by settingsJSON (a
// model.Settings), applying the timeout-range validation of the store layer.
func (b *Bridge) SaveSettings(settingsJSON string) string {
	var s model.Settings
	if err := json.Unmarshal([]byte(settingsJSON), &s); err != nil {
		return errorJSONMsg("invalid settings JSON: " + err.Error())
	}
	return actionResult(b.store.SaveSettings(s))
}

// ---------------------------------------------------------------------------
// Import / Export
// ---------------------------------------------------------------------------

// ExportCollection returns the documented versioned JSON envelope for the
// collection with the given id. On success the returned string is the export
// JSON itself; on failure it is a {"error":"..."} envelope.
func (b *Bridge) ExportCollection(id string) string {
	data, err := b.store.ExportCollection(id)
	if err != nil {
		return errorJSON(err)
	}
	return string(data)
}

// ExportEnvironment returns the documented versioned JSON envelope for the
// environment with the given id.
func (b *Bridge) ExportEnvironment(id string) string {
	data, err := b.store.ExportEnvironment(id)
	if err != nil {
		return errorJSON(err)
	}
	return string(data)
}

// Import parses a previously exported envelope (dataJSON) and recreates its
// collection or environment as a new entry, returning the store.ImportResult as
// JSON, or an error envelope when the file is malformed or unsupported.
func (b *Bridge) Import(dataJSON string) string {
	result, err := b.store.Import([]byte(dataJSON))
	if err != nil {
		return errorJSON(err)
	}
	return marshalJSON(result)
}

// ---------------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------------

// marshalJSON encodes v to a JSON string. A marshal failure (which should not
// occur for the model types) is itself surfaced as an error envelope so the
// caller always receives valid JSON.
func marshalJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return errorJSONMsg("encode result: " + err.Error())
	}
	return string(data)
}

// actionResult maps a plain error return into the action-method JSON
// convention: {"ok":true} on success or {"error":"..."} on failure.
func actionResult(err error) string {
	if err != nil {
		return errorJSON(err)
	}
	return `{"ok":true}`
}

// errorJSON renders err as a {"error":"..."} envelope.
func errorJSON(err error) string {
	return errorJSONMsg(err.Error())
}

// errorJSONMsg renders msg as a {"error":"..."} envelope, marshalling the
// message so any quotes or control characters are correctly escaped.
func errorJSONMsg(msg string) string {
	data, err := json.Marshal(struct {
		Error string `json:"error"`
	}{Error: msg})
	if err != nil {
		// Marshalling a string field cannot realistically fail; fall back to a
		// constant valid-JSON envelope.
		return `{"error":"unknown error"}`
	}
	return string(data)
}

// responseErrorJSON builds a JSON model.HTTPResponse carrying only the given
// error message, used by Execute so an internal failure is reported in the same
// shape as a network failure (Req 15.4).
func responseErrorJSON(msg string) string {
	return marshalJSON(model.HTTPResponse{Error: msg})
}
