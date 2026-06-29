package store

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 16: Exactly one (or zero) active environment
//
// Validates: Requirements 6.3, 6.10
//
// TestActiveEnvironmentInvariant is the property-based test for Property 16:
// across any sequence of environment mutations — saving environments (some
// marked active), activating an existing environment or clearing the selection
// via SetActiveEnvironment(""), and deleting environments — the store keeps at
// most one environment active. After every single operation the count of
// environments whose Active flag is true is always 0 or 1, never more (Req 6.3).
//
// The test additionally asserts the deletion rule from Req 6.10: whenever the
// operation deletes the environment that was active, the resulting active count
// is exactly 0, since the active flag lived on the now-removed row.
//
// Each iteration drives a fresh in-memory store through a random op sequence.
// Generated environment names are unique (a per-iteration counter feeds the
// name) and valid (1..64 runes), so every save succeeds and the invariant is
// exercised by the store's own bookkeeping rather than by rejected writes.
func TestActiveEnvironmentInvariant(t *testing.T) {
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		s, err := Open(":memory:")
		if err != nil {
			failMsg = fmt.Sprintf("Open: %v", err)
			return false
		}
		defer s.Close()

		// liveIDs tracks the IDs currently persisted so SetActive/Delete can
		// target real environments; nameCounter keeps generated names unique.
		liveIDs := []string{}
		nameCounter := 0

		// A random sequence of 1..40 operations exercises interleavings of
		// save/activate/clear/delete against an evolving environment set.
		ops := rng.Intn(40) + 1
		for i := 0; i < ops; i++ {
			switch rng.Intn(4) {
			case 0:
				// Save a brand-new environment, sometimes marked active.
				nameCounter++
				active := rng.Intn(2) == 0
				saved, err := s.SaveEnvironment(model.Environment{
					Name:      fmt.Sprintf("env_%d", nameCounter),
					Active:    active,
					Variables: genActiveEnvVars(rng),
				})
				if err != nil {
					failMsg = fmt.Sprintf("SaveEnvironment: %v", err)
					return false
				}
				liveIDs = append(liveIDs, saved.ID)

			case 1:
				// Re-save (upsert) an existing environment, toggling its active
				// flag, so the at-most-one-active path runs on updates too.
				if len(liveIDs) == 0 {
					continue
				}
				id := liveIDs[rng.Intn(len(liveIDs))]
				name, ok := envName(s, t, id)
				if !ok {
					continue
				}
				if _, err := s.SaveEnvironment(model.Environment{
					ID:        id,
					Name:      name,
					Active:    rng.Intn(2) == 0,
					Variables: genActiveEnvVars(rng),
				}); err != nil {
					failMsg = fmt.Sprintf("SaveEnvironment (upsert): %v", err)
					return false
				}

			case 2:
				// SetActiveEnvironment to an existing ID or "" (clear). Clearing
				// is always valid; activating a live ID is always valid.
				if len(liveIDs) == 0 || rng.Intn(3) == 0 {
					if err := s.SetActiveEnvironment(""); err != nil {
						failMsg = fmt.Sprintf("SetActiveEnvironment(\"\"): %v", err)
						return false
					}
				} else {
					id := liveIDs[rng.Intn(len(liveIDs))]
					if err := s.SetActiveEnvironment(id); err != nil {
						failMsg = fmt.Sprintf("SetActiveEnvironment(%q): %v", id, err)
						return false
					}
				}

			case 3:
				// Delete an existing environment. Capture whether the target was
				// active so the Req 6.10 "no active after deleting active" rule
				// can be checked against the post-delete state.
				if len(liveIDs) == 0 {
					continue
				}
				idx := rng.Intn(len(liveIDs))
				id := liveIDs[idx]
				deletedWasActive, err := envIsActive(s, t, id)
				if err != nil {
					failMsg = fmt.Sprintf("read active before delete: %v", err)
					return false
				}
				if err := s.DeleteEnvironment(id); err != nil {
					failMsg = fmt.Sprintf("DeleteEnvironment(%q): %v", id, err)
					return false
				}
				liveIDs = append(liveIDs[:idx], liveIDs[idx+1:]...)

				if deletedWasActive {
					envs, err := s.ListEnvironments()
					if err != nil {
						failMsg = fmt.Sprintf("ListEnvironments after delete: %v", err)
						return false
					}
					if c := activeCount(envs); c != 0 {
						failMsg = fmt.Sprintf(
							"deleted the active environment but active count = %d, want 0 (Req 6.10)", c)
						return false
					}
				}
			}

			// Core invariant (Req 6.3): after every operation, at most one
			// environment is active.
			envs, err := s.ListEnvironments()
			if err != nil {
				failMsg = fmt.Sprintf("ListEnvironments: %v", err)
				return false
			}
			if c := activeCount(envs); c > 1 {
				failMsg = fmt.Sprintf("active count = %d after op %d, want 0 or 1 (Req 6.3)", c, i)
				return false
			}
		}
		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 16 failed: %v\n%s", err, failMsg)
	}
}

// genActiveEnvVars returns a non-nil slice (length 0..3) of variables with
// names unique within the environment and valid value lengths, so every save in
// the property test succeeds.
func genActiveEnvVars(rng *rand.Rand) []model.Variable {
	n := rng.Intn(4)
	out := make([]model.Variable, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, model.Variable{
			Name:  fmt.Sprintf("k%d", i),
			Value: randStr(rng, 0, 12),
		})
	}
	return out
}

// envName returns the current name of the environment with the given ID by
// consulting ListEnvironments, so an upsert can preserve the (unique) name.
func envName(s *Store, t *testing.T, id string) (string, bool) {
	t.Helper()
	envs, err := s.ListEnvironments()
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}
	e, ok := findEnv(envs, id)
	return e.Name, ok
}

// envIsActive reports whether the environment with the given ID is currently
// flagged active. A missing ID is reported as not active without error.
func envIsActive(s *Store, t *testing.T, id string) (bool, error) {
	t.Helper()
	envs, err := s.ListEnvironments()
	if err != nil {
		return false, errors.New("ListEnvironments failed")
	}
	e, ok := findEnv(envs, id)
	if !ok {
		return false, nil
	}
	return e.Active, nil
}
