package store

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 17: History records complete entries in reverse chronological order and restores configuration
//
// Validates: Requirements 7.1, 7.2, 7.3, 7.4
//
// TestHistoryRecordsCompleteEntriesInReverseChronologicalOrder is the
// property-based test for Property 17. For any random sequence of executed
// requests — a mix of successful outcomes (a real status code with no error)
// and failed outcomes (an error indication with status 0) — recording each via
// AddHistory and then reading them back with ListHistory must satisfy:
//
//	(a) Every recorded entry is present with all of its fields intact, including
//	    its full Request configuration, so a selected entry can restore the
//	    stored method, URL, params, headers, body, and auth (Req 7.1, 7.2, 7.4).
//	(b) Entries are returned in reverse-chronological order, newest first, i.e.
//	    timestamps are non-increasing down the list (Req 7.3).
//
// Each iteration uses a fresh Open(":memory:") store so iterations stay
// independent. The generated count stays well under the 1000-entry cap so no
// pruning occurs and the full set is expected back.
func TestHistoryRecordsCompleteEntriesInReverseChronologicalOrder(t *testing.T) {
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		entries := genHistorySeq(rng)

		s, err := Open(":memory:")
		if err != nil {
			failMsg = fmt.Sprintf("Open: %v", err)
			return false
		}
		defer s.Close()

		for i := range entries {
			if err := s.AddHistory(entries[i]); err != nil {
				failMsg = fmt.Sprintf("AddHistory %q: %v", entries[i].ID, err)
				return false
			}
		}

		got, err := s.ListHistory()
		if err != nil {
			failMsg = fmt.Sprintf("ListHistory: %v", err)
			return false
		}

		// (a) Every added entry is present with its full fields and request
		// configuration intact. Compare against the expected newest-first order
		// so a single DeepEqual covers both presence and field fidelity.
		want := sortedHistory(entries)
		if !reflect.DeepEqual(got, want) {
			failMsg = fmt.Sprintf("history mismatch:\n got: %#v\nwant: %#v", got, want)
			return false
		}

		// (b) Timestamps are non-increasing down the list (reverse chronological).
		for i := 1; i < len(got); i++ {
			if got[i-1].At < got[i].At {
				failMsg = fmt.Sprintf(
					"order violated at index %d: at %d precedes newer at %d",
					i, got[i-1].At, got[i].At)
				return false
			}
		}

		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 17 failed: %v\n%s", err, failMsg)
	}
}

// genHistorySeq builds a non-nil sequence (length 1..12) of history entries with
// unique IDs and distinct, non-zero timestamps so the newest-first ordering is
// unambiguous. Each entry is randomly either a SUCCESS (a real status code in
// 100..599 with an empty Error) or a FAILURE (status 0 with a non-empty Error),
// mirroring how httpcore records completed vs. failed executions (Req 7.1, 7.2).
// Every entry carries a full RawRequest (via genRaw) so the round-trip exercises
// configuration fidelity (Req 7.4). The count stays far below the 1000-entry cap
// so no pruning occurs.
func genHistorySeq(rng *rand.Rand) []model.HistoryEntry {
	methods := []string{
		model.MethodGet, model.MethodPost, model.MethodPut, model.MethodPatch,
		model.MethodDelete, model.MethodHead, model.MethodOptions,
	}
	n := rng.Intn(12) + 1
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
			ID:         nextID(&historySeqCounter),
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

// historySeqCounter feeds nextID so every generated entry across all iterations
// receives a process-unique ID, keeping primary keys collision-free.
var historySeqCounter int
