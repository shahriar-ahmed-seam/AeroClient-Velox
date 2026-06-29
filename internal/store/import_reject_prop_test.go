package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 21: Malformed or unsupported imports are rejected atomically
//
// Validates: Requirements 8.5
//
// TestImportRejectAtomic is the property-based test for Property 21: for any
// import payload that is NOT a valid, supported envelope — malformed JSON,
// structurally wrong JSON (missing/unknown voltFormat or missing payload), or a
// correctly shaped envelope carrying an unsupported version — Import rejects the
// whole file before any write and returns ErrInvalidFormat or
// ErrUnsupportedVersion, leaving every existing Collection, Environment, and the
// Settings exactly as they were (Req 8.5).
//
// Each iteration:
//   - opens a fresh in-memory store and seeds it with one valid collection and
//     one valid environment using the package's existing valid-data generators,
//   - captures a full snapshot of stored state (ListTree + ListEnvironments +
//     GetSettings),
//   - generates one random BAD import payload across the categories below,
//   - calls Import and asserts it returns a non-nil error matching the expected
//     sentinel (ErrInvalidFormat or ErrUnsupportedVersion), and
//   - re-captures the snapshot and asserts it equals the pre-import snapshot, so
//     the rejection left all data unchanged.
//
// Bad-payload categories (selected at random per iteration):
//   0. non-JSON / garbage bytes                            -> ErrInvalidFormat
//   1. valid JSON but not an envelope object (array/number/
//      string/bool/null)                                   -> ErrInvalidFormat
//   2. JSON object missing voltFormat (empty/unknown
//      voltFormat) but otherwise version-correct           -> ErrInvalidFormat
//   3. correct voltFormat + version but missing the
//      collection/environment payload                      -> ErrInvalidFormat
//   4. correct format and payload but an unsupported
//      version number (!= exportVersion)                   -> ErrUnsupportedVersion
func TestImportRejectAtomic(t *testing.T) {
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))
		counter := 0

		// Fresh in-memory store per iteration so nothing leaks across runs.
		s, err := Open(":memory:")
		if err != nil {
			failMsg = fmt.Sprintf("Open(:memory:): %v", err)
			return false
		}
		defer s.Close()

		// --- seed valid existing data ---
		if _, err := s.SaveCollection(genOneCollection(rng, &counter)); err != nil {
			failMsg = fmt.Sprintf("seed SaveCollection: %v", err)
			return false
		}
		if _, err := s.SaveEnvironment(genOneEnvironment(rng, &counter)); err != nil {
			failMsg = fmt.Sprintf("seed SaveEnvironment: %v", err)
			return false
		}

		// --- capture the pre-import snapshot ---
		before, err := captureStoreSnapshot(s)
		if err != nil {
			failMsg = fmt.Sprintf("snapshot before: %v", err)
			return false
		}

		// --- generate one random bad payload ---
		payload := genBadImport(rng)

		// --- attempt the import: it must be rejected atomically ---
		_, gotErr := s.Import(payload)
		if gotErr == nil {
			failMsg = fmt.Sprintf("Import accepted a bad payload:\n%s", payload)
			return false
		}
		// Every category must be rejected with one of the two documented
		// rejection sentinels (Req 8.5). Which one applies depends on the
		// envelope's own field-check order inside Import — e.g. a JSON `null`
		// unmarshals to a zero envelope and is rejected by the version gate —
		// so the property asserts membership in the set of rejection errors
		// rather than over-specifying a single sentinel per category.
		if !errors.Is(gotErr, ErrInvalidFormat) && !errors.Is(gotErr, ErrUnsupportedVersion) {
			failMsg = fmt.Sprintf("Import returned %v, want ErrInvalidFormat or ErrUnsupportedVersion for payload:\n%s", gotErr, payload)
			return false
		}

		// --- the rejection must have left all data unchanged (Req 8.5) ---
		after, err := captureStoreSnapshot(s)
		if err != nil {
			failMsg = fmt.Sprintf("snapshot after: %v", err)
			return false
		}
		if !reflect.DeepEqual(before, after) {
			failMsg = fmt.Sprintf("store changed after a rejected import:\n before: %#v\n after:  %#v", before, after)
			return false
		}

		return true
	}

	// Minimum 100 iterations.
	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 21 failed: %v\n%s", err, failMsg)
	}
}

// ---------------------------------------------------------------------------
// Snapshot helper (distinct name; captures the full observable store state).
// ---------------------------------------------------------------------------

// storeSnapshot is a comparable picture of all data an import could possibly
// mutate: the collection tree, the environment list, and the settings row.
type storeSnapshot struct {
	Collections  []model.Collection
	Environments []model.Environment
	Settings     model.Settings
}

// captureStoreSnapshot reads the full observable store state used to assert an
// rejected import is atomic (leaves data unchanged, Req 8.5).
func captureStoreSnapshot(s *Store) (storeSnapshot, error) {
	tree, err := s.ListTree()
	if err != nil {
		return storeSnapshot{}, fmt.Errorf("ListTree: %w", err)
	}
	envs, err := s.ListEnvironments()
	if err != nil {
		return storeSnapshot{}, fmt.Errorf("ListEnvironments: %w", err)
	}
	settings, err := s.GetSettings()
	if err != nil {
		return storeSnapshot{}, fmt.Errorf("GetSettings: %w", err)
	}
	return storeSnapshot{Collections: tree, Environments: envs, Settings: settings}, nil
}

// ---------------------------------------------------------------------------
// Bad-import generator (distinct name; produces a payload + the sentinel error
// Import must return for it).
// ---------------------------------------------------------------------------

// genBadImport returns a random invalid import payload spanning the categories
// described on TestImportRejectAtomic. Every returned payload is guaranteed to
// be rejected by Import with ErrInvalidFormat or ErrUnsupportedVersion.
func genBadImport(rng *rand.Rand) []byte {
	switch rng.Intn(5) {
	case 0:
		// Category 0: non-JSON / garbage bytes.
		return genGarbageBytes(rng)

	case 1:
		// Category 1: valid JSON but not an object envelope.
		nonObjects := []string{
			"[]",
			"[1, 2, 3]",
			"42",
			"-0.5",
			`"a string"`,
			"true",
			"false",
			"null",
			fmt.Sprintf("%q", randStr(rng, 0, 8)),
		}
		return []byte(nonObjects[rng.Intn(len(nonObjects))])

	case 2:
		// Category 2: JSON object with version-correct field but a
		// missing/unknown voltFormat, so it is structurally invalid.
		formats := []string{"", "Collection", "ENVIRONMENT", "settings", "history", randStr(rng, 1, 10)}
		obj := map[string]any{
			"version": exportVersion,
			"random":  randStr(rng, 0, 12),
		}
		// Half the time include an unknown voltFormat, half the time omit it.
		if rng.Intn(2) == 0 {
			obj["voltFormat"] = formats[rng.Intn(len(formats))]
		}
		return mustMarshalBytes(obj)

	case 3:
		// Category 3: correct voltFormat + supported version but the matching
		// payload (collection/environment) is absent.
		obj := map[string]any{
			"version":    exportVersion,
			"exportedAt": "2024-01-01T00:00:00Z",
		}
		if rng.Intn(2) == 0 {
			obj["voltFormat"] = voltFormatCollection // collection field missing
		} else {
			obj["voltFormat"] = voltFormatEnvironment // environment field missing
		}
		return mustMarshalBytes(obj)

	default:
		// Category 4: correct format and payload but an unsupported version.
		badVersion := genUnsupportedVersion(rng)
		var obj map[string]any
		if rng.Intn(2) == 0 {
			obj = map[string]any{
				"voltFormat": voltFormatCollection,
				"version":    badVersion,
				"exportedAt": "2024-01-01T00:00:00Z",
				"collection": model.Collection{Name: "x"},
			}
		} else {
			obj = map[string]any{
				"voltFormat":  voltFormatEnvironment,
				"version":     badVersion,
				"exportedAt":  "2024-01-01T00:00:00Z",
				"environment": model.Environment{Name: "x"},
			}
		}
		return mustMarshalBytes(obj)
	}
}

// genGarbageBytes returns bytes that are not valid JSON: a random run of
// characters drawn from a set rich in JSON-significant punctuation so the result
// reliably fails to parse as an envelope.
func genGarbageBytes(rng *rand.Rand) []byte {
	const alphabet = "abc XYZ 123 {}[]:,\"\\<>/=@#%&*()!?;~`"
	n := 1 + rng.Intn(24)
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rng.Intn(len(alphabet))]
	}
	// Guard against the rare chance the random run is accidentally valid JSON
	// (e.g. a lone number or bare token): if so, prepend a byte that cannot
	// start any JSON value so json.Unmarshal is guaranteed to fail.
	if json.Valid(b) {
		b = append([]byte{'}'}, b...)
	}
	return b
}

// genUnsupportedVersion returns an int version that is guaranteed not to equal
// the supported exportVersion.
func genUnsupportedVersion(rng *rand.Rand) int {
	candidates := []int{0, -1, exportVersion + 1, exportVersion + 2, 99, 1000, -7}
	v := candidates[rng.Intn(len(candidates))]
	if v == exportVersion {
		v = exportVersion + 1
	}
	return v
}

// mustMarshalBytes marshals v to JSON, panicking on the impossible error so the
// generator stays expression-friendly.
func mustMarshalBytes(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("genBadImport marshal: %v", err))
	}
	return data
}
