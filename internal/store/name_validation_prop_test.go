package store

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 15: Name validation by length and uniqueness
//
// Validates: Requirements 5.8, 6.1, 6.2, 6.9
//
// These property-based tests cover Property 15: a Collection/Request/Folder name
// is accepted iff its rune length is in 1..255, an Environment name iff its rune
// length is in 1..64 and unique among Environments, and a Variable name iff its
// rune length is in 1..128 and unique within its Environment, with Variable
// values accepted for rune lengths 0..4096. Every rejection leaves prior stored
// data unchanged (Req 6.9).
//
// All length-bound generators below span deliberately chosen rune lengths that
// straddle each bound (0, 1, just below, exactly at, and just above the
// maximum). Lengths are measured with utf8.RuneCountInString so multi-byte
// runes count the same way the validators count them; the generators mix
// multi-byte runes so byte length differs from rune length. Each iteration uses
// a fresh Open(":memory:") store so iterations stay independent.

// nvRunes is a mix of single- and multi-byte runes used to build names/values
// of an exact rune length whose byte length differs from its rune length.
var nvRunes = []rune{'a', 'Z', '9', '_', 'é', 'ß', '世', '本', 'Ω', '🚀'}

// nvStringOfRuneLen returns a string of exactly n runes drawn from nvRunes. For
// n == 0 it returns the empty string. The resulting byte length generally
// exceeds n, so a validator that mistakenly counted bytes would disagree with
// these tests.
func nvStringOfRuneLen(rng *rand.Rand, n int) string {
	if n <= 0 {
		return ""
	}
	rs := make([]rune, n)
	for i := range rs {
		rs[i] = nvRunes[rng.Intn(len(nvRunes))]
	}
	return string(rs)
}

// nvPickNameLen returns a rune length straddling the 1..max name bound: it
// includes 0 (empty, invalid), 1 (min valid), an interior length, max-1, max
// (max valid), and lengths just/well above max (invalid).
func nvPickNameLen(rng *rand.Rand, max int) int {
	candidates := []int{0, 1, 2, max / 2, max - 1, max, max + 1, max + 7}
	return candidates[rng.Intn(len(candidates))]
}

// nvPickValueLen returns a rune length straddling the 0..max value bound. Here 0
// is valid (an empty value is allowed); only lengths above max are invalid.
func nvPickValueLen(rng *rand.Rand, max int) int {
	candidates := []int{0, 1, max / 2, max - 1, max, max + 1, max + 9}
	return candidates[rng.Intn(len(candidates))]
}

// ---------------------------------------------------------------------------
// Collection / Request / Folder name length (1..255)
// ---------------------------------------------------------------------------

// TestNameValidationCollectionTreeLength exercises the 1..255 rune-length bound
// on Collection, saved-Request, and Folder names across SaveCollection,
// RenameCollection, SaveRequest, and SaveFolder. A name within bounds is
// accepted and persisted; a name of length 0 or above 255 is rejected with
// ErrInvalidName and leaves prior data unchanged.
func TestNameValidationCollectionTreeLength(t *testing.T) {
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		s, err := Open(":memory:")
		if err != nil {
			failMsg = fmt.Sprintf("Open: %v", err)
			return false
		}
		defer s.Close()

		// Baseline valid collection used as a parent and as the "prior data"
		// that rejections must leave untouched.
		base := model.Collection{
			ID:       "base-col",
			Name:     "baseline",
			Order:    0,
			Requests: []model.SavedRequest{},
			Folders:  []model.Folder{},
		}
		if _, err := s.SaveCollection(base); err != nil {
			failMsg = fmt.Sprintf("SaveCollection(baseline): %v", err)
			return false
		}

		// --- SaveCollection name bound ---
		cn := nvPickNameLen(rng, maxCollectionNameLen)
		colName := nvStringOfRuneLen(rng, cn)
		colValid := cn >= 1 && cn <= maxCollectionNameLen

		before := nvSnapshotTree(t, s)
		_, cErr := s.SaveCollection(model.Collection{
			ID:       "cand-col",
			Name:     colName,
			Order:    1,
			Requests: []model.SavedRequest{},
			Folders:  []model.Folder{},
		})
		if ok, msg := nvCheckTree(t, s, before, colValid, cErr, ErrInvalidName,
			fmt.Sprintf("SaveCollection rune-len %d", cn)); !ok {
			failMsg = msg
			return false
		}

		// --- RenameCollection name bound (rename the baseline) ---
		rn := nvPickNameLen(rng, maxCollectionNameLen)
		renameName := nvStringOfRuneLen(rng, rn)
		renameValid := rn >= 1 && rn <= maxCollectionNameLen

		before = nvSnapshotTree(t, s)
		rErr := s.RenameCollection(base.ID, renameName)
		if renameValid {
			if rErr != nil {
				failMsg = fmt.Sprintf("RenameCollection rune-len %d rejected: %v", rn, rErr)
				return false
			}
		} else {
			if !errors.Is(rErr, ErrInvalidName) {
				failMsg = fmt.Sprintf("RenameCollection rune-len %d returned %v, want ErrInvalidName", rn, rErr)
				return false
			}
			if after := nvSnapshotTree(t, s); !reflect.DeepEqual(before, after) {
				failMsg = fmt.Sprintf("RenameCollection rune-len %d rejected but tree changed", rn)
				return false
			}
		}

		// --- SaveRequest name bound (under the baseline collection) ---
		qn := nvPickNameLen(rng, maxRequestNameLen)
		reqName := nvStringOfRuneLen(rng, qn)
		reqValid := qn >= 1 && qn <= maxRequestNameLen

		before = nvSnapshotTree(t, s)
		_, qErr := s.SaveRequest(model.SavedRequest{
			ID:         "cand-req",
			Name:       reqName,
			RawRequest: nvSimpleRaw(),
		}, base.ID)
		if ok, msg := nvCheckTree(t, s, before, reqValid, qErr, ErrInvalidName,
			fmt.Sprintf("SaveRequest rune-len %d", qn)); !ok {
			failMsg = msg
			return false
		}

		// --- SaveFolder name bound (under the baseline collection) ---
		fn := nvPickNameLen(rng, maxFolderNameLen)
		folderName := nvStringOfRuneLen(rng, fn)
		folderValid := fn >= 1 && fn <= maxFolderNameLen

		before = nvSnapshotTree(t, s)
		_, fErr := s.SaveFolder(model.Folder{
			ID:       "cand-folder",
			Name:     folderName,
			Requests: []model.SavedRequest{},
			Folders:  []model.Folder{},
		}, base.ID)
		if ok, msg := nvCheckTree(t, s, before, folderValid, fErr, ErrInvalidName,
			fmt.Sprintf("SaveFolder rune-len %d", fn)); !ok {
			failMsg = msg
			return false
		}

		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 15 (collection-tree name length) failed: %v\n%s", err, failMsg)
	}
}

// ---------------------------------------------------------------------------
// Environment name length (1..64) and cross-environment uniqueness
// ---------------------------------------------------------------------------

// TestNameValidationEnvironmentNameLengthAndUniqueness exercises the 1..64
// rune-length bound on Environment names and cross-Environment name uniqueness.
// An in-bounds, unique name is accepted; length 0 or above 64 is rejected with
// ErrInvalidName; a name duplicating an existing Environment is rejected with
// ErrDuplicateName. Every rejection leaves prior environments unchanged.
func TestNameValidationEnvironmentNameLengthAndUniqueness(t *testing.T) {
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		s, err := Open(":memory:")
		if err != nil {
			failMsg = fmt.Sprintf("Open: %v", err)
			return false
		}
		defer s.Close()

		// Baseline existing environment whose name candidates may duplicate.
		base := model.Environment{
			ID:        "base-env",
			Name:      "existing-env",
			Variables: []model.Variable{},
			Active:    false,
		}
		if _, err := s.SaveEnvironment(base); err != nil {
			failMsg = fmt.Sprintf("SaveEnvironment(baseline): %v", err)
			return false
		}

		// Decide whether to test a length case or a duplicate-name case.
		duplicate := rng.Intn(2) == 0

		before := nvSnapshotEnvs(t, s)

		var (
			candName string
			candErr  error
			valid    bool
			wantErr  error
			label    string
		)
		if duplicate {
			// Exact duplicate of the baseline name (which is within bounds), so
			// the only reason to reject is the name collision.
			candName = base.Name
			valid = false
			wantErr = ErrDuplicateName
			label = "duplicate environment name"
		} else {
			en := nvPickNameLen(rng, maxEnvironmentNameLen)
			candName = nvStringOfRuneLen(rng, en)
			valid = en >= 1 && en <= maxEnvironmentNameLen
			wantErr = ErrInvalidName
			label = fmt.Sprintf("environment name rune-len %d", en)
		}

		_, candErr = s.SaveEnvironment(model.Environment{
			ID:        "cand-env",
			Name:      candName,
			Variables: []model.Variable{},
			Active:    false,
		})

		if valid {
			if candErr != nil {
				failMsg = fmt.Sprintf("%s accepted-case rejected: %v", label, candErr)
				return false
			}
			// Accepted: the candidate must now be present alongside the baseline.
			after := nvSnapshotEnvs(t, s)
			if len(after) != len(before)+1 {
				failMsg = fmt.Sprintf("%s accepted but env count %d, want %d", label, len(after), len(before)+1)
				return false
			}
			return true
		}

		// Rejected: the specific sentinel must surface and prior data must be
		// byte-for-byte unchanged.
		if !errors.Is(candErr, wantErr) {
			failMsg = fmt.Sprintf("%s returned %v, want %v", label, candErr, wantErr)
			return false
		}
		if after := nvSnapshotEnvs(t, s); !reflect.DeepEqual(before, after) {
			failMsg = fmt.Sprintf("%s rejected but environments changed:\n before: %#v\n after:  %#v", label, before, after)
			return false
		}
		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 15 (environment name) failed: %v\n%s", err, failMsg)
	}
}

// ---------------------------------------------------------------------------
// Variable name length (1..128), value length (0..4096), in-env uniqueness
// ---------------------------------------------------------------------------

// TestNameValidationVariableLengthAndUniqueness exercises the 1..128 rune-length
// bound on Variable names, the 0..4096 rune-length bound on Variable values, and
// uniqueness of Variable names within an Environment. An in-bounds, unique
// variable is accepted; a name of length 0 or above 128 is rejected with
// ErrInvalidName; a value above 4096 is rejected with ErrInvalidValue; a name
// duplicating another variable in the same Environment is rejected with
// ErrDuplicateName. Every rejection leaves the prior environment unchanged.
func TestNameValidationVariableLengthAndUniqueness(t *testing.T) {
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		s, err := Open(":memory:")
		if err != nil {
			failMsg = fmt.Sprintf("Open: %v", err)
			return false
		}
		defer s.Close()

		// Baseline environment holding one known-good variable. Rejections of a
		// re-save of this environment must leave this stored state untouched.
		base := model.Environment{
			ID:   "var-env",
			Name: "var-env",
			Variables: []model.Variable{
				{Name: "kept", Value: "kept-value"},
			},
			Active: false,
		}
		if _, err := s.SaveEnvironment(base); err != nil {
			failMsg = fmt.Sprintf("SaveEnvironment(baseline): %v", err)
			return false
		}

		before := nvSnapshotEnvs(t, s)

		// Choose one of three scenarios: variable-name length, value length, or
		// a duplicate variable name within the environment.
		switch rng.Intn(3) {
		case 0: // variable name length bound
			vn := nvPickNameLen(rng, maxVariableNameLen)
			name := nvStringOfRuneLen(rng, vn)
			valid := vn >= 1 && vn <= maxVariableNameLen
			_, e := s.SaveEnvironment(model.Environment{
				ID:        base.ID,
				Name:      base.Name,
				Variables: []model.Variable{{Name: name, Value: "v"}},
			})
			return nvJudgeVarSave(t, s, before, valid, e, ErrInvalidName,
				fmt.Sprintf("variable name rune-len %d", vn), &failMsg)

		case 1: // variable value length bound
			vl := nvPickValueLen(rng, maxVariableValueLen)
			value := nvStringOfRuneLen(rng, vl)
			valid := vl <= maxVariableValueLen // 0 is allowed
			_, e := s.SaveEnvironment(model.Environment{
				ID:        base.ID,
				Name:      base.Name,
				Variables: []model.Variable{{Name: "name", Value: value}},
			})
			return nvJudgeVarSave(t, s, before, valid, e, ErrInvalidValue,
				fmt.Sprintf("variable value rune-len %d", vl), &failMsg)

		default: // duplicate variable name within the environment
			_, e := s.SaveEnvironment(model.Environment{
				ID:   base.ID,
				Name: base.Name,
				Variables: []model.Variable{
					{Name: "dup", Value: "a"},
					{Name: "dup", Value: "b"},
				},
			})
			return nvJudgeVarSave(t, s, before, false, e, ErrDuplicateName,
				"duplicate variable name", &failMsg)
		}
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 15 (variable validation) failed: %v\n%s", err, failMsg)
	}
}

// ---------------------------------------------------------------------------
// Shared assertion helpers (distinct nv-prefixed names)
// ---------------------------------------------------------------------------

// nvSimpleRaw returns a minimal valid RawRequest for use in name-only tests.
func nvSimpleRaw() model.RawRequest {
	return model.RawRequest{
		Method:  model.MethodGet,
		URL:     "https://example.com/x",
		Params:  []model.KeyValue{},
		Headers: []model.KeyValue{},
		Body:    model.BodySpec{Type: model.BodyNone, FormFields: []model.KeyValue{}},
		Auth:    model.AuthSpec{Type: model.AuthNone},
	}
}

// nvSnapshotTree returns the full collection tree, failing the test on a read
// error (a read failure is a test-harness problem, not a property violation).
func nvSnapshotTree(t *testing.T, s *Store) []model.Collection {
	t.Helper()
	tree, err := s.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	return tree
}

// nvSnapshotEnvs returns all environments, failing the test on a read error.
func nvSnapshotEnvs(t *testing.T, s *Store) []model.Environment {
	t.Helper()
	envs, err := s.ListEnvironments()
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}
	return envs
}

// nvCheckTree judges a Save against the collection tree: when valid the save
// must succeed; when invalid it must fail with wantErr and leave the tree
// (captured in before) unchanged. It returns (true, "") on success or
// (false, message) describing the first violation.
func nvCheckTree(
	t *testing.T,
	s *Store,
	before []model.Collection,
	valid bool,
	gotErr, wantErr error,
	label string,
) (bool, string) {
	t.Helper()
	if valid {
		if gotErr != nil {
			return false, fmt.Sprintf("%s accepted-case rejected: %v", label, gotErr)
		}
		return true, ""
	}
	if !errors.Is(gotErr, wantErr) {
		return false, fmt.Sprintf("%s returned %v, want %v", label, gotErr, wantErr)
	}
	if after := nvSnapshotTree(t, s); !reflect.DeepEqual(before, after) {
		return false, fmt.Sprintf("%s rejected but tree changed", label)
	}
	return true, ""
}

// nvJudgeVarSave judges a SaveEnvironment re-save in the variable tests: when
// valid the save must succeed; when invalid it must fail with wantErr and leave
// the stored environments (captured in before) unchanged. It records the first
// violation in *failMsg and reports whether the iteration passed.
func nvJudgeVarSave(
	t *testing.T,
	s *Store,
	before []model.Environment,
	valid bool,
	gotErr, wantErr error,
	label string,
	failMsg *string,
) bool {
	t.Helper()
	if valid {
		if gotErr != nil {
			*failMsg = fmt.Sprintf("%s accepted-case rejected: %v", label, gotErr)
			return false
		}
		return true
	}
	if !errors.Is(gotErr, wantErr) {
		*failMsg = fmt.Sprintf("%s returned %v, want %v", label, gotErr, wantErr)
		return false
	}
	if after := nvSnapshotEnvs(t, s); !reflect.DeepEqual(before, after) {
		*failMsg = fmt.Sprintf("%s rejected but environments changed:\n before: %#v\n after:  %#v", label, before, after)
		return false
	}
	return true
}
