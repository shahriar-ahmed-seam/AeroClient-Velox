package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"

	"volt/internal/model"
)

// This file implements Volt's import/export (Req 8.1–8.7). Export serializes a
// single Collection or Environment into a documented, versioned JSON envelope.
// Import parses such an envelope, validates its format/version, and recreates
// the contained entries as brand-new entries (fresh IDs) so an import never
// overwrites or merges with existing data (Req 8.3, 8.6). A malformed, invalid,
// or unsupported-version file is rejected in full, leaving existing data
// unchanged (Req 8.5).

// voltFormat values identify what a JSON envelope carries. They are written to
// the "voltFormat" field on export and matched on import (Req 8.1, 8.2).
const (
	voltFormatCollection  = "collection"
	voltFormatEnvironment = "environment"
)

// exportVersion is the current export schema version written to every envelope
// and the only version accepted on import (Req 8.1, 8.2, 8.5).
const exportVersion = 1

// ErrInvalidFormat is returned when an imported file is not valid JSON, is
// structurally malformed, or carries an unrecognized "voltFormat" (Req 8.5). It
// is returned before any write so existing data is left unchanged.
var ErrInvalidFormat = errors.New("store: invalid import format")

// ErrUnsupportedVersion is returned when an imported envelope carries a
// "version" that does not match a supported schema (Req 8.5). It is returned
// before any write so existing data is left unchanged.
var ErrUnsupportedVersion = errors.New("store: unsupported import version")

// exportEnvelope is the documented JSON shape produced by Export and consumed by
// Import. Exactly one of Collection or Environment is populated, selected by
// VoltFormat. Version gates compatibility and ExportedAt is an RFC3339 stamp of
// when the file was produced (Req 8.1, 8.2).
type exportEnvelope struct {
	VoltFormat  string             `json:"voltFormat"`
	Version     int                `json:"version"`
	ExportedAt  string             `json:"exportedAt"`
	Collection  *model.Collection  `json:"collection,omitempty"`
	Environment *model.Environment `json:"environment,omitempty"`
}

// ImportResult reports what an Import created so callers can surface or select
// the new entry. Format is "collection" or "environment"; the matching ID is the
// freshly assigned identifier of the recreated entry and Name is its stored name
// (which may have been disambiguated for an Environment, see Import).
type ImportResult struct {
	Format        string `json:"format"`
	CollectionID  string `json:"collectionId,omitempty"`
	EnvironmentID string `json:"environmentId,omitempty"`
	Name          string `json:"name"`
}

// ---------------------------------------------------------------------------
// Export
// ---------------------------------------------------------------------------

// ExportCollection serializes the Collection with the given ID — including every
// nested Folder and Request with its full configuration — into the documented
// versioned JSON envelope (Req 8.1). It returns ErrNotFound when no Collection
// has the given ID. The returned bytes are indented for readability and can be
// written to disk atomically with WriteFileAtomic (Req 8.7).
func (s *Store) ExportCollection(id string) ([]byte, error) {
	collections, err := s.ListTree()
	if err != nil {
		return nil, err
	}
	for i := range collections {
		if collections[i].ID == id {
			c := collections[i]
			return marshalEnvelope(exportEnvelope{
				VoltFormat: voltFormatCollection,
				Version:    exportVersion,
				ExportedAt: time.Now().UTC().Format(time.RFC3339),
				Collection: &c,
			})
		}
	}
	return nil, ErrNotFound
}

// ExportEnvironment serializes the Environment with the given ID — including
// every Variable name and value — into the documented versioned JSON envelope
// (Req 8.2). It returns ErrNotFound when no Environment has the given ID.
func (s *Store) ExportEnvironment(id string) ([]byte, error) {
	envs, err := s.ListEnvironments()
	if err != nil {
		return nil, err
	}
	for i := range envs {
		if envs[i].ID == id {
			e := envs[i]
			return marshalEnvelope(exportEnvelope{
				VoltFormat:  voltFormatEnvironment,
				Version:     exportVersion,
				ExportedAt:  time.Now().UTC().Format(time.RFC3339),
				Environment: &e,
			})
		}
	}
	return nil, ErrNotFound
}

// marshalEnvelope renders an envelope as indented JSON.
func marshalEnvelope(env exportEnvelope) ([]byte, error) {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("store: encode export: %w", err)
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// Import
// ---------------------------------------------------------------------------

// Import parses a previously exported envelope and recreates its Collection or
// Environment as a brand-new entry (Req 8.3, 8.4). Every recreated entity is
// assigned a fresh ID so an import never collides with or mutates existing data,
// and a name that matches an existing entry is imported as a separate new entry
// rather than overwriting it (Req 8.6) — for Environments, whose names must be
// unique, the imported name is disambiguated with a numeric suffix so the
// existing Environment is left untouched.
//
// The recreation runs through SaveCollection / SaveEnvironment, each of which is
// a single all-or-nothing transaction, so a recreated entry is written atomically
// (Req 8.5). If the input is not valid JSON, is structurally malformed, names an
// unknown format, or carries an unsupported version, Import rejects the whole
// file before any write and returns ErrInvalidFormat or ErrUnsupportedVersion,
// leaving all existing Collections, Environments, and Variables unchanged
// (Req 8.5).
func (s *Store) Import(data []byte) (ImportResult, error) {
	var env exportEnvelope
	// Reject anything that is not a well-formed envelope object before touching
	// stored data (Req 8.5): malformed JSON or a non-object shape fails here.
	if err := json.Unmarshal(data, &env); err != nil {
		return ImportResult{}, fmt.Errorf("%w: %v", ErrInvalidFormat, err)
	}

	// The version must match a supported schema regardless of format (Req 8.5).
	if env.Version != exportVersion {
		return ImportResult{}, fmt.Errorf("%w: version %d", ErrUnsupportedVersion, env.Version)
	}

	switch env.VoltFormat {
	case voltFormatCollection:
		return s.importCollection(env)
	case voltFormatEnvironment:
		return s.importEnvironment(env)
	default:
		return ImportResult{}, fmt.Errorf("%w: unknown voltFormat %q", ErrInvalidFormat, env.VoltFormat)
	}
}

// importCollection recreates the envelope's Collection as a new entry with fresh
// IDs throughout its tree (Req 8.3, 8.4, 8.6). Collections carry no uniqueness
// constraint, so a name matching an existing Collection naturally becomes a
// separate new entry.
func (s *Store) importCollection(env exportEnvelope) (ImportResult, error) {
	if env.Collection == nil {
		return ImportResult{}, fmt.Errorf("%w: missing collection payload", ErrInvalidFormat)
	}
	c := *env.Collection
	// Assign fresh IDs so the import never collides with existing entries; blank
	// IDs are filled in by SaveCollection's insert helpers (Req 8.3).
	resetCollectionIDs(&c)

	// Append after existing collections so the new entry has a stable position.
	count, err := s.countCollections()
	if err != nil {
		return ImportResult{}, err
	}
	c.Order = count

	saved, err := s.SaveCollection(c)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{
		Format:       voltFormatCollection,
		CollectionID: saved.ID,
		Name:         saved.Name,
	}, nil
}

// importEnvironment recreates the envelope's Environment as a new entry with a
// fresh ID (Req 8.3, 8.4). Because Environment names must be unique (Req 6.1), a
// name that collides with an existing Environment is disambiguated so the import
// becomes a separate new entry and the existing Environment is left unchanged
// (Req 8.6). The imported Environment is never made active, so importing cannot
// alter which Environment is currently active (Req 6.3).
func (s *Store) importEnvironment(env exportEnvelope) (ImportResult, error) {
	if env.Environment == nil {
		return ImportResult{}, fmt.Errorf("%w: missing environment payload", ErrInvalidFormat)
	}
	e := *env.Environment
	e.ID = ""        // fresh ID assigned by SaveEnvironment (Req 8.3)
	e.Active = false // never steal active state from existing data (Req 6.3, 8.6)

	uniqueName, err := s.uniqueEnvironmentName(e.Name)
	if err != nil {
		return ImportResult{}, err
	}
	e.Name = uniqueName

	saved, err := s.SaveEnvironment(e)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{
		Format:        voltFormatEnvironment,
		EnvironmentID: saved.ID,
		Name:          saved.Name,
	}, nil
}

// resetCollectionIDs clears the IDs of a Collection and its entire subtree so
// SaveCollection assigns brand-new identifiers, guaranteeing an import recreates
// independent entries (Req 8.3).
func resetCollectionIDs(c *model.Collection) {
	c.ID = ""
	for i := range c.Requests {
		c.Requests[i].ID = ""
	}
	for i := range c.Folders {
		resetFolderIDs(&c.Folders[i])
	}
}

// resetFolderIDs clears the IDs of a Folder and its nested folders/requests.
func resetFolderIDs(f *model.Folder) {
	f.ID = ""
	for i := range f.Requests {
		f.Requests[i].ID = ""
	}
	for i := range f.Folders {
		resetFolderIDs(&f.Folders[i])
	}
}

// countCollections returns the number of stored collections, used to position an
// imported collection after the existing ones.
func (s *Store) countCollections() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM collections`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count collections: %w", err)
	}
	return n, nil
}

// uniqueEnvironmentName returns name unchanged when no Environment already uses
// it, otherwise a disambiguated variant ("name (2)", "name (3)", ...) that does
// not collide, keeping within the Environment name length bound (Req 6.1). This
// lets an imported Environment be stored as a separate new entry even when its
// name matches an existing one (Req 8.6).
func (s *Store) uniqueEnvironmentName(name string) (string, error) {
	envs, err := s.ListEnvironments()
	if err != nil {
		return "", err
	}
	existing := make(map[string]struct{}, len(envs))
	for i := range envs {
		existing[envs[i].Name] = struct{}{}
	}
	if _, clash := existing[name]; !clash {
		return name, nil
	}
	for i := 2; ; i++ {
		candidate := fitEnvironmentName(name, fmt.Sprintf(" (%d)", i))
		if _, clash := existing[candidate]; !clash {
			return candidate, nil
		}
	}
}

// fitEnvironmentName joins base and suffix so the result stays within the
// Environment name length bound (maxEnvironmentNameLen runes), trimming the base
// as needed to make room for the suffix.
func fitEnvironmentName(base, suffix string) string {
	room := maxEnvironmentNameLen - utf8.RuneCountInString(suffix)
	if room < 0 {
		room = 0
	}
	if utf8.RuneCountInString(base) > room {
		base = string([]rune(base)[:room])
	}
	return base + suffix
}

// ---------------------------------------------------------------------------
// Atomic file write (Req 8.7)
// ---------------------------------------------------------------------------

// WriteFileAtomic writes data to path by first writing a temporary file in the
// same directory and then renaming it into place, so a reader never observes a
// partially written export (Req 8.1, 8.2). If the destination directory is not
// writable the write fails before any file appears at path, aborting the export
// without leaving a partial file (Req 8.7). It is the helper the bindings use to
// persist the bytes returned by ExportCollection / ExportEnvironment.
func WriteFileAtomic(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".volt-export-*.tmp")
	if err != nil {
		return fmt.Errorf("store: create export temp file: %w", err)
	}
	tmpName := tmp.Name()
	// On any failure after creation, remove the temp file so no partial artifact
	// is left behind (Req 8.7).
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, werr := tmp.Write(data); werr != nil {
		_ = tmp.Close()
		return fmt.Errorf("store: write export temp file: %w", werr)
	}
	if cerr := tmp.Close(); cerr != nil {
		return fmt.Errorf("store: close export temp file: %w", cerr)
	}
	if rerr := os.Rename(tmpName, path); rerr != nil {
		return fmt.Errorf("store: finalize export file: %w", rerr)
	}
	return nil
}
