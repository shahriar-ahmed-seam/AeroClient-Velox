package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"volt/internal/httpcore"
	"volt/internal/model"
	"volt/internal/store"
)

// errStoreUnavailable is returned by the store-backed bindings when the
// persistence layer failed to open at startup. Surfacing a typed error lets the
// frontend show a problem instead of silently losing data.
var errStoreUnavailable = errors.New("volt: storage is unavailable")

// App is the Wails binding surface exposed to the Svelte frontend. It owns the
// application context and the SQLite-backed store, and delegates request
// execution to the shared pure core (internal/httpcore) and all persistence to
// internal/store, so desktop behavior matches the Android build that shares the
// same packages.
type App struct {
	ctx   context.Context
	store *store.Store
}

// NewApp creates a new App. The store is opened in startup once the Wails
// runtime context is available.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved so runtime
// methods and request cancellation can use it, and the persistent store is
// opened at the per-user data directory (created if necessary). A store that
// fails to open is logged and left nil; the store-backed bindings then report
// errStoreUnavailable rather than panicking.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	dbPath, err := defaultDBPath()
	if err != nil {
		log.Printf("volt: resolve data directory: %v", err)
		return
	}
	st, err := store.Open(dbPath)
	if err != nil {
		log.Printf("volt: open store at %s: %v", dbPath, err)
		return
	}
	a.store = st
}

// shutdown closes the store, releasing the database connection. It is safe to
// call when the store never opened.
func (a *App) shutdown(context.Context) {
	if a.store != nil {
		_ = a.store.Close()
	}
}

// defaultDBPath returns the path to the Volt SQLite database inside a per-user
// data directory (e.g. %AppData%/Volt on Windows, ~/.config/Volt on Linux,
// ~/Library/Application Support/Volt on macOS), creating the directory if it
// does not yet exist.
func defaultDBPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	voltDir := filepath.Join(configDir, "Volt")
	if err := os.MkdirAll(voltDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(voltDir, "volt.db"), nil
}

// fallbackSettings are the request settings used when the store is unavailable,
// mirroring the store's first-launch defaults (System theme, TLS verification
// on, 30-second timeout).
func fallbackSettings() model.Settings {
	return model.Settings{Theme: "system", TLSVerify: true, TimeoutSeconds: 30}
}

// AppVersion returns the application's semantic version embedded at build time
// (Req 13.1). It is "dev" for local builds.
func (a *App) AppVersion() string {
	return version
}

// ---------------------------------------------------------------------------
// Request execution
// ---------------------------------------------------------------------------

// ExecuteRequest prepares and sends a raw request through the shared core and
// records the outcome in History (Req 1.2, 7.1).
//
// It loads the active environment and the current settings, calls
// httpcore.PrepareRequest to interpolate, validate, and encode the request, and
// — only when preparation succeeds — calls httpcore.Execute to perform the
// network call. A preparation failure (e.g. an invalid URL) returns an
// HTTPResponse carrying the error with no network call. Either way a History
// entry is recorded for both successful and failed executions before the
// response is returned.
func (a *App) ExecuteRequest(raw model.RawRequest) model.HTTPResponse {
	activeEnv, settings := a.activeContext()

	prepared, err := httpcore.PrepareRequest(raw, activeEnv, settings)
	if err != nil {
		// Validation failed before any network I/O (Req 1.3). Surface the error
		// and still record the attempt in History (Req 7.2).
		resp := model.HTTPResponse{Error: err.Error()}
		a.recordHistory(raw, resp)
		return resp
	}

	resp := httpcore.Execute(a.ctx, nil, prepared, settings)
	a.recordHistory(raw, resp)
	return resp
}

// activeContext resolves the environment and settings used to prepare and
// execute a request: the single active Environment (empty when none is active
// or the store is unavailable) and the current Settings (fallback defaults when
// the store is unavailable).
func (a *App) activeContext() (model.Environment, model.Settings) {
	settings := fallbackSettings()
	var activeEnv model.Environment
	if a.store == nil {
		return activeEnv, settings
	}

	if envs, err := a.store.ListEnvironments(); err == nil {
		for _, e := range envs {
			if e.Active {
				activeEnv = e
				break
			}
		}
	}
	if s, err := a.store.GetSettings(); err == nil {
		settings = s
	}
	return activeEnv, settings
}

// recordHistory appends a History entry for an executed (or attempted) request,
// capturing the method, URL, status, elapsed time, timestamp, error, and the
// full request configuration for later restore (Req 7.1, 7.2). A nil store or a
// write failure is ignored so a recording problem never breaks the response
// returned to the user.
func (a *App) recordHistory(raw model.RawRequest, resp model.HTTPResponse) {
	if a.store == nil {
		return
	}
	entry := model.HistoryEntry{
		Method:     raw.Method,
		URL:        raw.URL,
		Status:     resp.Status,
		DurationMs: resp.DurationMs,
		At:         time.Now().UnixMilli(),
		Error:      resp.Error,
		Request:    raw,
	}
	_ = a.store.AddHistory(entry)
}

// ---------------------------------------------------------------------------
// Collection / Folder / Request tree
// ---------------------------------------------------------------------------

// ListTree returns the full collection tree with nested folders and requests.
func (a *App) ListTree() ([]model.Collection, error) {
	if a.store == nil {
		return nil, errStoreUnavailable
	}
	return a.store.ListTree()
}

// SaveCollection creates or updates a Collection.
func (a *App) SaveCollection(c model.Collection) (model.Collection, error) {
	if a.store == nil {
		return model.Collection{}, errStoreUnavailable
	}
	return a.store.SaveCollection(c)
}

// RenameCollection renames the Collection with the given ID.
func (a *App) RenameCollection(id, name string) error {
	if a.store == nil {
		return errStoreUnavailable
	}
	return a.store.RenameCollection(id, name)
}

// DeleteCollection removes the Collection with the given ID and its contents.
func (a *App) DeleteCollection(id string) error {
	if a.store == nil {
		return errStoreUnavailable
	}
	return a.store.DeleteCollection(id)
}

// SaveFolder creates or updates a Folder under the given parent (a Collection or
// Folder ID).
func (a *App) SaveFolder(f model.Folder, parentID string) (model.Folder, error) {
	if a.store == nil {
		return model.Folder{}, errStoreUnavailable
	}
	return a.store.SaveFolder(f, parentID)
}

// DeleteFolder removes the Folder with the given ID and its contents.
func (a *App) DeleteFolder(id string) error {
	if a.store == nil {
		return errStoreUnavailable
	}
	return a.store.DeleteFolder(id)
}

// SaveRequest creates or updates a SavedRequest under the given parent (a
// Collection or Folder ID).
func (a *App) SaveRequest(r model.SavedRequest, parentID string) (model.SavedRequest, error) {
	if a.store == nil {
		return model.SavedRequest{}, errStoreUnavailable
	}
	return a.store.SaveRequest(r, parentID)
}

// DeleteRequest removes the SavedRequest with the given ID.
func (a *App) DeleteRequest(id string) error {
	if a.store == nil {
		return errStoreUnavailable
	}
	return a.store.DeleteRequest(id)
}

// MoveRequest relocates a SavedRequest into a target Collection or Folder.
func (a *App) MoveRequest(requestID, targetParentID string) error {
	if a.store == nil {
		return errStoreUnavailable
	}
	return a.store.MoveRequest(requestID, targetParentID)
}

// ---------------------------------------------------------------------------
// Environments
// ---------------------------------------------------------------------------

// SaveEnvironment creates or updates an Environment and its Variables.
func (a *App) SaveEnvironment(e model.Environment) (model.Environment, error) {
	if a.store == nil {
		return model.Environment{}, errStoreUnavailable
	}
	return a.store.SaveEnvironment(e)
}

// DeleteEnvironment removes the Environment with the given ID.
func (a *App) DeleteEnvironment(id string) error {
	if a.store == nil {
		return errStoreUnavailable
	}
	return a.store.DeleteEnvironment(id)
}

// ListEnvironments returns every Environment with its Variables.
func (a *App) ListEnvironments() ([]model.Environment, error) {
	if a.store == nil {
		return nil, errStoreUnavailable
	}
	return a.store.ListEnvironments()
}

// SetActiveEnvironment makes the given Environment active, or clears the active
// selection when id is empty.
func (a *App) SetActiveEnvironment(id string) error {
	if a.store == nil {
		return errStoreUnavailable
	}
	return a.store.SetActiveEnvironment(id)
}

// ---------------------------------------------------------------------------
// History
// ---------------------------------------------------------------------------

// ListHistory returns recorded History entries newest-first.
func (a *App) ListHistory() ([]model.HistoryEntry, error) {
	if a.store == nil {
		return nil, errStoreUnavailable
	}
	return a.store.ListHistory()
}

// ClearHistory deletes all recorded History entries.
func (a *App) ClearHistory() error {
	if a.store == nil {
		return errStoreUnavailable
	}
	return a.store.ClearHistory()
}

// MigrateLegacyHistory imports legacy localStorage History entries exactly once,
// returning whether this call performed the import.
func (a *App) MigrateLegacyHistory(entries []model.HistoryEntry) (bool, error) {
	if a.store == nil {
		return false, errStoreUnavailable
	}
	return a.store.MigrateLegacyHistory(entries)
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

// GetSettings returns the effective Settings, synthesizing first-launch
// defaults when none have been saved.
func (a *App) GetSettings() (model.Settings, error) {
	if a.store == nil {
		return fallbackSettings(), errStoreUnavailable
	}
	return a.store.GetSettings()
}

// SaveSettings persists the supplied Settings.
func (a *App) SaveSettings(s model.Settings) error {
	if a.store == nil {
		return errStoreUnavailable
	}
	return a.store.SaveSettings(s)
}

// ---------------------------------------------------------------------------
// Import / Export
// ---------------------------------------------------------------------------

// ExportCollection serializes a Collection into the versioned JSON envelope.
func (a *App) ExportCollection(id string) ([]byte, error) {
	if a.store == nil {
		return nil, errStoreUnavailable
	}
	return a.store.ExportCollection(id)
}

// ExportEnvironment serializes an Environment into the versioned JSON envelope.
func (a *App) ExportEnvironment(id string) ([]byte, error) {
	if a.store == nil {
		return nil, errStoreUnavailable
	}
	return a.store.ExportEnvironment(id)
}

// Import recreates the Collection or Environment described by a versioned JSON
// envelope as a brand-new entry.
func (a *App) Import(data []byte) (store.ImportResult, error) {
	if a.store == nil {
		return store.ImportResult{}, errStoreUnavailable
	}
	return a.store.Import(data)
}
