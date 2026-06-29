package store

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 20: Import/export round-trip with collision-safe naming
//
// Validates: Requirements 8.1, 8.2, 8.3, 8.4, 8.6
//
// TestImportExportRoundTrip is the property-based test for Property 20: for any
// valid Collection or Environment, exporting it and then importing the exported
// bytes back into the SAME store recreates an entry whose field values and
// nesting structure are identical to those present at export time (modulo newly
// assigned identifiers), and because the imported name matches the still-present
// original, the original is left untouched while the import becomes a separate
// new entry (collision-safe — Req 8.6).
//
// Each iteration:
//   - builds one random valid collection (nested folders/requests, depth <= 10)
//     and one random valid environment using the package's existing valid-data
//     generators (genRequests/genFolders/genVariables/randStr/nextID),
//   - saves both into a fresh in-memory store,
//   - exports each, then imports the exported bytes back into the SAME store,
//   - asserts the imported copy has a NEW id, is structurally equal to the
//     original ignoring IDs (and the import-assigned Order), the environment's
//     name is disambiguated, its variables are identical, and both the original
//     and the imported copy now coexist.
func TestImportExportRoundTrip(t *testing.T) {
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))
		counter := 0

		srcCol := genOneCollection(rng, &counter)
		srcEnv := genOneEnvironment(rng, &counter)

		// Fresh in-memory store per iteration. The deferred Close runs on every
		// return path so the shared in-memory database is released between
		// iterations (no leakage across runs).
		s, err := Open(":memory:")
		if err != nil {
			failMsg = fmt.Sprintf("Open(:memory:): %v", err)
			return false
		}
		defer s.Close()

		// --- save the originals ---
		savedCol, err := s.SaveCollection(srcCol)
		if err != nil {
			failMsg = fmt.Sprintf("SaveCollection: %v", err)
			return false
		}
		savedEnv, err := s.SaveEnvironment(srcEnv)
		if err != nil {
			failMsg = fmt.Sprintf("SaveEnvironment: %v", err)
			return false
		}

		// --- export each entry ---
		colBytes, err := s.ExportCollection(savedCol.ID)
		if err != nil {
			failMsg = fmt.Sprintf("ExportCollection: %v", err)
			return false
		}
		envBytes, err := s.ExportEnvironment(savedEnv.ID)
		if err != nil {
			failMsg = fmt.Sprintf("ExportEnvironment: %v", err)
			return false
		}

		// --- import the exported bytes back into the SAME store ---
		colResult, err := s.Import(colBytes)
		if err != nil {
			failMsg = fmt.Sprintf("Import(collection): %v", err)
			return false
		}
		envResult, err := s.Import(envBytes)
		if err != nil {
			failMsg = fmt.Sprintf("Import(environment): %v", err)
			return false
		}

		// === Collection assertions (Req 8.1, 8.3, 8.4, 8.6) ===

		// The imported collection must have a NEW id (Req 8.3).
		if colResult.CollectionID == savedCol.ID {
			failMsg = fmt.Sprintf("imported collection reused source id %q", savedCol.ID)
			return false
		}

		tree, err := s.ListTree()
		if err != nil {
			failMsg = fmt.Sprintf("ListTree: %v", err)
			return false
		}
		// Both the original and the imported copy must coexist (Req 8.6).
		if len(tree) != 2 {
			failMsg = fmt.Sprintf("expected 2 collections after import, got %d", len(tree))
			return false
		}
		original := findCollection(tree, savedCol.ID)
		imported := findCollection(tree, colResult.CollectionID)
		if original == nil {
			failMsg = fmt.Sprintf("original collection %q missing after import (overwritten?)", savedCol.ID)
			return false
		}
		if imported == nil {
			failMsg = fmt.Sprintf("imported collection %q not found", colResult.CollectionID)
			return false
		}
		// Structure/content identical, ignoring IDs and the import-assigned Order
		// (Req 8.4): names, nesting, ordering of children, and request configs.
		normOriginal := normalizeCollection(*original)
		normImported := normalizeCollection(*imported)
		if !reflect.DeepEqual(normOriginal, normImported) {
			failMsg = fmt.Sprintf("imported collection differs from original (ignoring IDs):\n orig: %#v\n imp:  %#v", normOriginal, normImported)
			return false
		}

		// === Environment assertions (Req 8.2, 8.3, 8.4, 8.6) ===

		// The imported environment must have a NEW id (Req 8.3).
		if envResult.EnvironmentID == savedEnv.ID {
			failMsg = fmt.Sprintf("imported environment reused source id %q", savedEnv.ID)
			return false
		}
		// Its name must be disambiguated because the original name still exists
		// (Req 8.6): environment names are unique, so the import cannot keep the
		// colliding name.
		if envResult.Name == savedEnv.Name {
			failMsg = fmt.Sprintf("imported environment kept colliding name %q (not disambiguated)", savedEnv.Name)
			return false
		}

		envs, err := s.ListEnvironments()
		if err != nil {
			failMsg = fmt.Sprintf("ListEnvironments: %v", err)
			return false
		}
		// Both the original and the imported copy must coexist (Req 8.6).
		if len(envs) != 2 {
			failMsg = fmt.Sprintf("expected 2 environments after import, got %d", len(envs))
			return false
		}
		origEnv := findEnvironment(envs, savedEnv.ID)
		impEnv := findEnvironment(envs, envResult.EnvironmentID)
		if origEnv == nil {
			failMsg = fmt.Sprintf("original environment %q missing after import (overwritten?)", savedEnv.ID)
			return false
		}
		if impEnv == nil {
			failMsg = fmt.Sprintf("imported environment %q not found", envResult.EnvironmentID)
			return false
		}
		// The original's name and variables must be untouched (Req 8.6).
		if origEnv.Name != savedEnv.Name {
			failMsg = fmt.Sprintf("original environment name changed: got %q want %q", origEnv.Name, savedEnv.Name)
			return false
		}
		// Variables identical between original and import (Req 8.2, 8.4): the
		// disambiguated name is the only difference.
		if !reflect.DeepEqual(impEnv.Variables, origEnv.Variables) {
			failMsg = fmt.Sprintf("imported environment variables differ:\n orig: %#v\n imp:  %#v", origEnv.Variables, impEnv.Variables)
			return false
		}

		return true
	}

	// Minimum 100 iterations.
	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 20 failed: %v\n%s", err, failMsg)
	}
}

// ---------------------------------------------------------------------------
// Single-entity generators (build one valid entity from the package's existing
// element generators; distinct names so they do not redeclare genCollections /
// genEnvironments).
// ---------------------------------------------------------------------------

// genOneCollection builds exactly one valid Collection with nested folders and
// requests. Top-level folders sit at depth 1 and the folder budget of 4 keeps
// total nesting at most 5 levels, well within the 10-level limit.
func genOneCollection(rng *rand.Rand, counter *int) model.Collection {
	return model.Collection{
		ID:       nextID(counter),
		Name:     randStr(rng, 1, 12),
		Order:    0,
		Requests: genRequests(rng, counter),
		Folders:  genFolders(rng, counter, 4),
	}
}

// genOneEnvironment builds exactly one valid Environment with a unique-style
// name and valid variables. It is never marked active, mirroring how an import
// stores an environment.
func genOneEnvironment(rng *rand.Rand, counter *int) model.Environment {
	return model.Environment{
		ID:        nextID(counter),
		Name:      fmt.Sprintf("env_%s", randStr(rng, 1, 8)),
		Variables: genVariables(rng),
		Active:    false,
	}
}

// ---------------------------------------------------------------------------
// Structural-equality helpers (compare collections/folders/requests ignoring
// IDs, plus the import-assigned Collection.Order).
// ---------------------------------------------------------------------------

// normalizeCollection returns a deep copy of c with every ID cleared and the
// collection Order zeroed, so two collections that differ only by their
// (freshly assigned) identifiers and stored position compare equal under
// reflect.DeepEqual. The copy is made via a JSON round-trip so the original is
// never mutated and non-nil empty slices are preserved.
func normalizeCollection(c model.Collection) model.Collection {
	clone := cloneCollection(c)
	clone.Order = 0
	stripCollectionIDs(&clone)
	return clone
}

// cloneCollection deep-copies a Collection via JSON so normalization does not
// mutate stored or generated data.
func cloneCollection(c model.Collection) model.Collection {
	data, err := json.Marshal(c)
	if err != nil {
		panic(fmt.Sprintf("clone collection marshal: %v", err))
	}
	var out model.Collection
	if err := json.Unmarshal(data, &out); err != nil {
		panic(fmt.Sprintf("clone collection unmarshal: %v", err))
	}
	return out
}

// stripCollectionIDs clears the ID of a Collection and every nested folder and
// request so structural comparison ignores identifiers.
func stripCollectionIDs(c *model.Collection) {
	c.ID = ""
	for i := range c.Requests {
		c.Requests[i].ID = ""
	}
	for i := range c.Folders {
		stripFolderIDs(&c.Folders[i])
	}
}

// stripFolderIDs clears the ID of a Folder and its nested folders/requests.
func stripFolderIDs(f *model.Folder) {
	f.ID = ""
	for i := range f.Requests {
		f.Requests[i].ID = ""
	}
	for i := range f.Folders {
		stripFolderIDs(&f.Folders[i])
	}
}

// findCollection returns a pointer to the collection with the given ID, or nil.
func findCollection(cols []model.Collection, id string) *model.Collection {
	for i := range cols {
		if cols[i].ID == id {
			return &cols[i]
		}
	}
	return nil
}

// findEnvironment returns a pointer to the environment with the given ID, or nil.
func findEnvironment(envs []model.Environment, id string) *model.Environment {
	for i := range envs {
		if envs[i].ID == id {
			return &envs[i]
		}
	}
	return nil
}
