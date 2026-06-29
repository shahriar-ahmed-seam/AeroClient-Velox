package store

import (
	"fmt"
	"math/rand"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 18: History is capped at 1000 entries discarding the oldest
//
// Validates: Requirements 7.6
//
// TestHistoryCappedAtThousandDiscardingOldest is the property-based test for
// Property 18. For any number N of recorded executions — chosen to straddle the
// 1000-entry cap (below, at, and above it) — recording each via AddHistory with
// strictly increasing timestamps and then reading the log back with ListHistory
// must satisfy:
//
//	(a) The retained count is exactly min(N, historyCap). Below the cap the full
//	    set survives; at or above the cap exactly historyCap entries remain.
//	(b) When N exceeds the cap, the survivors are precisely the newest
//	    historyCap entries (the most recent by timestamp) in newest-first order,
//	    and the oldest N-historyCap entries are absent (Req 7.6).
//
// Each iteration uses a fresh Open(":memory:") store so iterations are
// independent. Timestamps increase strictly with insertion index, so the newest
// historyCap entries are deterministically the last historyCap inserted, making
// the expected survivor set and ordering unambiguous.
func TestHistoryCappedAtThousandDiscardingOldest(t *testing.T) {
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		// N straddles the cap: [historyCap-5, historyCap+10] covers below
		// (995..999), at (1000), and above (1001..1010) the 1000-entry cap.
		n := historyCap - 5 + rng.Intn(16)

		entries := genCappedHistory(rng, n)

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

		// (a) Retained count is exactly min(N, historyCap).
		wantCount := n
		if wantCount > historyCap {
			wantCount = historyCap
		}
		if len(got) != wantCount {
			failMsg = fmt.Sprintf("count: got %d, want %d (N=%d, cap=%d)",
				len(got), wantCount, n, historyCap)
			return false
		}

		// Expected survivors are the newest wantCount entries (the tail of the
		// strictly-increasing-timestamp sequence), in newest-first order.
		wantIDs := make([]string, wantCount)
		for i := 0; i < wantCount; i++ {
			wantIDs[i] = entries[n-1-i].ID
		}

		// (b) Survivors are exactly the newest historyCap entries, in order, so
		// the oldest N-historyCap entries are absent.
		gotIDs := make(map[string]bool, len(got))
		for i := range got {
			gotIDs[got[i].ID] = true
			if got[i].ID != wantIDs[i] {
				failMsg = fmt.Sprintf(
					"survivor mismatch at index %d: got id %q (at %d), want id %q",
					i, got[i].ID, got[i].At, wantIDs[i])
				return false
			}
		}

		// The discarded oldest entries (indices [0, n-wantCount)) must be gone.
		for i := 0; i < n-wantCount; i++ {
			if gotIDs[entries[i].ID] {
				failMsg = fmt.Sprintf(
					"oldest discarded entry %q (at %d) still present",
					entries[i].ID, entries[i].At)
				return false
			}
		}

		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 25}); err != nil { // reduced from 100: each run inserts ~1000 rows; 25 runs still straddle the 1000 cap (sped up per request)
		t.Fatalf("Property 18 failed: %v\n%s", err, failMsg)
	}
}

// genCappedHistory builds a sequence of n history entries with process-unique
// IDs and strictly increasing, non-zero timestamps (At = index+1 within a
// per-iteration base), so insertion order, timestamp order, and newest-first
// order all coincide. This makes the newest historyCap entries the unambiguous
// tail of the sequence, which the cap test relies on to identify survivors.
func genCappedHistory(rng *rand.Rand, n int) []model.HistoryEntry {
	methods := []string{
		model.MethodGet, model.MethodPost, model.MethodPut, model.MethodPatch,
		model.MethodDelete, model.MethodHead, model.MethodOptions,
	}
	out := make([]model.HistoryEntry, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, model.HistoryEntry{
			ID:         nextID(&historyCapSeqCounter),
			Method:     methods[rng.Intn(len(methods))],
			URL:        "https://example.com/" + randStr(rng, 1, 8),
			Status:     rng.Intn(500) + 100,
			DurationMs: int64(rng.Intn(5000)),
			At:         int64(i) + 1, // strictly increasing, distinct, non-zero
			Request:    genRaw(rng),
		})
	}
	return out
}

// historyCapSeqCounter feeds nextID so every generated entry across all
// iterations receives a process-unique ID, keeping primary keys collision-free
// and timestamp ties impossible to confuse with ID ordering.
var historyCapSeqCounter int
