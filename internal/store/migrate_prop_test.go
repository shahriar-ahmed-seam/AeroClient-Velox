package store

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 19: Legacy history migration is idempotent
//
// Validates: Requirements 7.8
//
// TestMigrateLegacyHistoryIsIdempotentProp is the property-based test for
// Property 19. For any fresh in-memory store, calling MigrateLegacyHistory a
// random number of times K (1..5) — each call with its own independently
// generated set of legacy entries — must satisfy:
//
//	(a) Exactly the first call performs the import (returns migrated=true once);
//	    every subsequent call observes the legacy_history_migrated flag and
//	    returns migrated=false without importing anything (Req 7.8).
//	(b) The resulting ListHistory equals exactly the entries supplied to the
//	    first successful migration, in newest-first order, regardless of K. Later
//	    calls — even with entirely different entry sets — neither add to nor
//	    change the migrated History (idempotent, Req 7.8).
//
// Each iteration uses a fresh Open(":memory:") store so iterations stay
// independent. Generated entry sets carry process-unique IDs and distinct,
// non-zero timestamps, and each set stays far below the 1000-entry cap so no
// pruning occurs and the first set is expected back verbatim.
func TestMigrateLegacyHistoryIsIdempotentProp(t *testing.T) {
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		s, err := Open(":memory:")
		if err != nil {
			failMsg = fmt.Sprintf("Open: %v", err)
			return false
		}
		defer s.Close()

		// K calls (1..5), each with its own independently generated entry set.
		k := rng.Intn(5) + 1

		var firstSet []model.HistoryEntry
		migratedCount := 0

		for call := 0; call < k; call++ {
			entries := genLegacyEntries(rng)

			migrated, err := s.MigrateLegacyHistory(entries)
			if err != nil {
				failMsg = fmt.Sprintf("MigrateLegacyHistory call %d: %v", call, err)
				return false
			}

			if call == 0 {
				// (a) The first call must perform the import.
				if !migrated {
					failMsg = "first call returned migrated=false, want true"
					return false
				}
				firstSet = entries
				migratedCount++
			} else {
				// (a) Every subsequent call must be a no-op import.
				if migrated {
					failMsg = fmt.Sprintf(
						"call %d returned migrated=true, want false (already migrated)", call)
					return false
				}
			}
		}

		// (a) Exactly one call performed the import across all K calls.
		if migratedCount != 1 {
			failMsg = fmt.Sprintf("migrated=true count = %d, want exactly 1 (K=%d)", migratedCount, k)
			return false
		}

		// (b) History equals exactly the first set, newest-first, regardless of K.
		got, err := s.ListHistory()
		if err != nil {
			failMsg = fmt.Sprintf("ListHistory: %v", err)
			return false
		}
		want := sortedHistory(firstSet)
		if !reflect.DeepEqual(got, want) {
			failMsg = fmt.Sprintf("history mismatch after K=%d calls:\n got: %#v\nwant: %#v", k, got, want)
			return false
		}

		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 19 failed: %v\n%s", err, failMsg)
	}
}

// genLegacyEntries builds a non-nil set (length 1..10) of legacy History entries
// with process-unique IDs and distinct, non-zero timestamps so the migrated
// History has an unambiguous newest-first order. Each entry carries a full
// RawRequest (via genRaw) so the migration round-trip exercises configuration
// fidelity. The count stays far below the 1000-entry cap so no pruning occurs
// and the supplied set is expected back verbatim.
func genLegacyEntries(rng *rand.Rand) []model.HistoryEntry {
	methods := []string{
		model.MethodGet, model.MethodPost, model.MethodPut, model.MethodPatch,
		model.MethodDelete, model.MethodHead, model.MethodOptions,
	}
	n := rng.Intn(10) + 1
	out := make([]model.HistoryEntry, 0, n)
	for i := 0; i < n; i++ {
		var status int
		errMsg := ""
		if rng.Intn(2) == 0 {
			// Success: real status code, no error.
			status = rng.Intn(500) + 100
		} else {
			// Failure: error indication, status 0.
			errMsg = randStr(rng, 1, 20)
		}
		out = append(out, model.HistoryEntry{
			ID:         nextID(&legacyMigrateSeqCounter),
			Method:     methods[rng.Intn(len(methods))],
			URL:        "https://example.com/" + randStr(rng, 1, 8),
			Status:     status,
			DurationMs: int64(rng.Intn(5000)),
			// Distinct, non-zero timestamps via per-index buckets of 1,000,000.
			At:      int64(i)*1_000_000 + int64(rng.Intn(1_000_000)) + 1,
			Error:   errMsg,
			Request: genRaw(rng),
		})
	}
	return out
}

// legacyMigrateSeqCounter feeds nextID so every generated entry across all
// iterations receives a process-unique ID, keeping the history primary key
// collision-free across calls and iterations.
var legacyMigrateSeqCounter int
