package store

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 12: Persistence round-trip across reopen
//
// Validates: Requirements 5.1, 5.5, 5.6, 6.7, 7.5, 9.7, 15.5
//
// TestPersistenceRoundTripAcrossReopen is the property-based test for Property
// 12: for any persisted entity set (a collection tree with folders and
// requests, environments with variables, history, and settings), saving it,
// closing the store, and reopening it yields data structurally equal to the
// original — including names, nesting structure, and order (accounting for the
// store's documented ordering rules: collections by Order, environments by
// name, history newest-first).
//
// The generators below produce only VALID data (valid name/value lengths, a
// valid 1..600 timeout, names unique where the store requires it, and folder
// nesting well within the 10-level limit) so every Save succeeds. Each
// iteration uses a fresh on-disk SQLite file under t.TempDir() so the round-trip
// genuinely crosses a Close/Open boundary rather than reading from a warm cache.
func TestPersistenceRoundTripAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	fileNo := 0
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))
		counter := 0

		cols := genCollections(rng, &counter)
		envs := genEnvironments(rng, &counter)
		hist := genHistory(rng, &counter)
		settings := genSettings(rng)

		fileNo++
		dbPath := filepath.Join(dir, fmt.Sprintf("volt-%d.db", fileNo))

		// --- write everything, then close ---
		s, err := Open(dbPath)
		if err != nil {
			failMsg = fmt.Sprintf("Open(write): %v", err)
			return false
		}
		for i := range cols {
			if _, err := s.SaveCollection(cols[i]); err != nil {
				failMsg = fmt.Sprintf("SaveCollection %q: %v", cols[i].ID, err)
				_ = s.Close()
				return false
			}
		}
		for i := range envs {
			if _, err := s.SaveEnvironment(envs[i]); err != nil {
				failMsg = fmt.Sprintf("SaveEnvironment %q: %v", envs[i].ID, err)
				_ = s.Close()
				return false
			}
		}
		for i := range hist {
			if err := s.AddHistory(hist[i]); err != nil {
				failMsg = fmt.Sprintf("AddHistory %q: %v", hist[i].ID, err)
				_ = s.Close()
				return false
			}
		}
		if err := s.SaveSettings(settings); err != nil {
			failMsg = fmt.Sprintf("SaveSettings: %v", err)
			_ = s.Close()
			return false
		}
		if err := s.Close(); err != nil {
			failMsg = fmt.Sprintf("Close: %v", err)
			return false
		}

		// --- reopen and read everything back ---
		s2, err := Open(dbPath)
		if err != nil {
			failMsg = fmt.Sprintf("Open(reopen): %v", err)
			return false
		}
		defer s2.Close()

		// Collections come back ordered by Order; generated Order is 0,1,2,...
		// so the expected order is the generation order.
		gotTree, err := s2.ListTree()
		if err != nil {
			failMsg = fmt.Sprintf("ListTree: %v", err)
			return false
		}
		if !reflect.DeepEqual(gotTree, cols) {
			failMsg = fmt.Sprintf("collection tree mismatch:\n got: %#v\nwant: %#v", gotTree, cols)
			return false
		}

		// Environments come back ordered by (name, id).
		wantEnvs := sortedEnvs(envs)
		gotEnvs, err := s2.ListEnvironments()
		if err != nil {
			failMsg = fmt.Sprintf("ListEnvironments: %v", err)
			return false
		}
		if !reflect.DeepEqual(gotEnvs, wantEnvs) {
			failMsg = fmt.Sprintf("environments mismatch:\n got: %#v\nwant: %#v", gotEnvs, wantEnvs)
			return false
		}

		// History comes back newest-first (at DESC, id DESC).
		wantHist := sortedHistory(hist)
		gotHist, err := s2.ListHistory()
		if err != nil {
			failMsg = fmt.Sprintf("ListHistory: %v", err)
			return false
		}
		if !reflect.DeepEqual(gotHist, wantHist) {
			failMsg = fmt.Sprintf("history mismatch:\n got: %#v\nwant: %#v", gotHist, wantHist)
			return false
		}

		// Settings round-trip verbatim (valid timeout is stored as given).
		gotSettings, err := s2.GetSettings()
		if err != nil {
			failMsg = fmt.Sprintf("GetSettings: %v", err)
			return false
		}
		if !reflect.DeepEqual(gotSettings, settings) {
			failMsg = fmt.Sprintf("settings mismatch:\n got: %#v\nwant: %#v", gotSettings, settings)
			return false
		}

		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 12 failed: %v\n%s", err, failMsg)
	}
}

// ---------------------------------------------------------------------------
// Generators (valid data only)
// ---------------------------------------------------------------------------

const genAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// nextID returns a process-unique identifier so primary keys never collide
// across the generated collections, folders, requests, environments, and
// history entries.
func nextID(counter *int) string {
	*counter++
	return fmt.Sprintf("ent-%d", *counter)
}

// randStr returns an alphanumeric string whose rune length is in [min, max]
// inclusive. With min=0 it may return the empty string.
func randStr(rng *rand.Rand, min, max int) string {
	n := min
	if max > min {
		n += rng.Intn(max - min + 1)
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = genAlphabet[rng.Intn(len(genAlphabet))]
	}
	return string(b)
}

// genKVs returns a non-nil slice (length 0..3) of key/value rows. Empty slices
// are non-nil so they survive the JSON round-trip identically (a nil slice
// would marshal to null and decode back to nil, breaking DeepEqual).
func genKVs(rng *rand.Rand) []model.KeyValue {
	n := rng.Intn(4)
	out := make([]model.KeyValue, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, model.KeyValue{
			Key:     randStr(rng, 0, 8),
			Value:   randStr(rng, 0, 8),
			Enabled: rng.Intn(2) == 0,
		})
	}
	return out
}

// genRaw builds a fully-populated RawRequest with non-nil slices throughout.
func genRaw(rng *rand.Rand) model.RawRequest {
	methods := []string{
		model.MethodGet, model.MethodPost, model.MethodPut, model.MethodPatch,
		model.MethodDelete, model.MethodHead, model.MethodOptions,
	}
	bodyTypes := []string{
		model.BodyNone, model.BodyJSON, model.BodyText,
		model.BodyFormData, model.BodyURLEncoded,
	}
	authTypes := []string{
		model.AuthNone, model.AuthBearer, model.AuthBasic, model.AuthAPIKey,
	}
	locs := []string{model.APIKeyInHeader, model.APIKeyInQuery}

	return model.RawRequest{
		Method:  methods[rng.Intn(len(methods))],
		URL:     "https://example.com/" + randStr(rng, 1, 8),
		Params:  genKVs(rng),
		Headers: genKVs(rng),
		Body: model.BodySpec{
			Type:       bodyTypes[rng.Intn(len(bodyTypes))],
			Raw:        randStr(rng, 0, 16),
			FormFields: genKVs(rng),
		},
		Auth: model.AuthSpec{
			Type:           authTypes[rng.Intn(len(authTypes))],
			BearerToken:    randStr(rng, 0, 10),
			BasicUser:      randStr(rng, 0, 8),
			BasicPass:      randStr(rng, 0, 8),
			APIKeyName:     randStr(rng, 0, 8),
			APIKeyValue:    randStr(rng, 0, 8),
			APIKeyLocation: locs[rng.Intn(len(locs))],
		},
	}
}

// genRequests returns a non-nil slice (length 0..2) of saved requests with
// valid (1..255-char) names and unique IDs.
func genRequests(rng *rand.Rand, counter *int) []model.SavedRequest {
	n := rng.Intn(3)
	out := make([]model.SavedRequest, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, model.SavedRequest{
			ID:         nextID(counter),
			Name:       randStr(rng, 1, 12),
			RawRequest: genRaw(rng),
		})
	}
	return out
}

// genFolders returns a non-nil slice (length 0..2) of folders, recursing while
// depthLeft > 0 so the produced subtree never exceeds the 10-level limit.
func genFolders(rng *rand.Rand, counter *int, depthLeft int) []model.Folder {
	if depthLeft <= 0 {
		return []model.Folder{}
	}
	n := rng.Intn(3)
	out := make([]model.Folder, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, model.Folder{
			ID:       nextID(counter),
			Name:     randStr(rng, 1, 12),
			Requests: genRequests(rng, counter),
			Folders:  genFolders(rng, counter, depthLeft-1),
		})
	}
	return out
}

// genCollections returns a non-nil slice (length 0..3) of collections. Each
// collection's Order is its index so ListTree returns them in generation order.
func genCollections(rng *rand.Rand, counter *int) []model.Collection {
	n := rng.Intn(4)
	out := []model.Collection{}
	for i := 0; i < n; i++ {
		out = append(out, model.Collection{
			ID:       nextID(counter),
			Name:     randStr(rng, 1, 12),
			Order:    i,
			Requests: genRequests(rng, counter),
			// Top-level folders sit at depth 1; budget of 3 keeps nesting <= 4.
			Folders: genFolders(rng, counter, 3),
		})
	}
	return out
}

// genVariables returns a non-nil slice (length 0..4) of variables with names
// unique within the environment (index-prefixed) and valid value lengths.
func genVariables(rng *rand.Rand) []model.Variable {
	n := rng.Intn(5)
	out := make([]model.Variable, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, model.Variable{
			Name:  fmt.Sprintf("v%d_%s", i, randStr(rng, 1, 5)),
			Value: randStr(rng, 0, 20),
		})
	}
	return out
}

// genEnvironments returns a non-nil slice (length 0..3) of environments with
// names unique among them (index-prefixed) and at most one marked active, so
// the store's at-most-one-active invariant leaves the generated set unchanged.
func genEnvironments(rng *rand.Rand, counter *int) []model.Environment {
	n := rng.Intn(4)
	out := []model.Environment{}
	activeIdx := -1
	if n > 0 && rng.Intn(2) == 0 {
		activeIdx = rng.Intn(n)
	}
	for i := 0; i < n; i++ {
		out = append(out, model.Environment{
			ID:        nextID(counter),
			Name:      fmt.Sprintf("env%d_%s", i, randStr(rng, 1, 5)),
			Variables: genVariables(rng),
			Active:    i == activeIdx,
		})
	}
	return out
}

// genHistory returns a non-nil slice (length 0..7) of history entries with
// unique IDs and distinct, non-zero timestamps (so newest-first ordering is
// unambiguous). The count stays well under the 1000-entry cap so no pruning
// occurs.
func genHistory(rng *rand.Rand, counter *int) []model.HistoryEntry {
	methods := []string{
		model.MethodGet, model.MethodPost, model.MethodPut, model.MethodPatch,
		model.MethodDelete, model.MethodHead, model.MethodOptions,
	}
	n := rng.Intn(8)
	out := []model.HistoryEntry{}
	for i := 0; i < n; i++ {
		errMsg := ""
		if rng.Intn(2) == 0 {
			errMsg = randStr(rng, 1, 20)
		}
		out = append(out, model.HistoryEntry{
			ID:         nextID(counter),
			Method:     methods[rng.Intn(len(methods))],
			URL:        "https://example.com/" + randStr(rng, 1, 8),
			Status:     rng.Intn(500) + 100,
			DurationMs: int64(rng.Intn(5000)),
			// Distinct timestamps: per-index buckets of 1,000,000.
			At:      int64(i)*1_000_000 + int64(rng.Intn(1_000_000)),
			Error:   errMsg,
			Request: genRaw(rng),
		})
	}
	return out
}

// genSettings returns valid Settings: a known theme, a random TLS flag, a valid
// 1..600 timeout (stored verbatim), and an arbitrary proxy string.
func genSettings(rng *rand.Rand) model.Settings {
	themes := []string{"light", "dark", "system"}
	return model.Settings{
		Theme:          themes[rng.Intn(len(themes))],
		TLSVerify:      rng.Intn(2) == 0,
		TimeoutSeconds: rng.Intn(MaxTimeoutSeconds-MinTimeoutSeconds+1) + MinTimeoutSeconds,
		ProxyURL:       randStr(rng, 0, 12),
	}
}

// ---------------------------------------------------------------------------
// Expected-ordering helpers (mirror the store's documented ordering rules)
// ---------------------------------------------------------------------------

// sortedEnvs returns a copy of in ordered by (name, id), matching the order
// ListEnvironments produces.
func sortedEnvs(in []model.Environment) []model.Environment {
	out := append([]model.Environment{}, in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// sortedHistory returns a copy of in ordered newest-first (at DESC, id DESC),
// matching the order ListHistory produces.
func sortedHistory(in []model.HistoryEntry) []model.HistoryEntry {
	out := append([]model.HistoryEntry{}, in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At > out[j].At
		}
		return out[i].ID > out[j].ID
	})
	return out
}
