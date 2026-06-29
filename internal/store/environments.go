package store

import (
	"database/sql"
	"fmt"

	"volt/internal/model"
)

// This file implements Environment persistence: creating/updating Environments
// and their Variables, deleting them, listing them, and managing which single
// Environment is active. All writes run through withTx so a rejected save or a
// failed write leaves prior data unchanged (Req 6.9).
//
// Invariants enforced here:
//   - Environment names are unique among Environments (Req 6.1, validated as a
//     defense-in-depth backstop, returning ErrDuplicateName on collision).
//   - Variable names are unique within an Environment and length-bounded, with
//     values length-bounded (Req 6.2, via validateEnvironment).
//   - At most one Environment is active at a time, and deleting the active
//     Environment leaves no Environment active (Req 6.3, 6.10).
//   - Environments and their Variables round-trip across reopen, preserving the
//     active flag and variable order (Req 6.7).

// ---------------------------------------------------------------------------
// SaveEnvironment
// ---------------------------------------------------------------------------

// SaveEnvironment persists an Environment and rebuilds its Variables in a single
// transaction (Req 6.1, 6.2, 6.7). Save uses upsert semantics: saving an
// Environment whose ID already exists replaces it, and its Variables are cleared
// and rebuilt from the supplied model so removed Variables do not linger.
// Variable slice position determines the stored order.
//
// Validation runs before any write: the Environment name length, every Variable
// name/value bound, and in-Environment Variable-name uniqueness are checked by
// validateEnvironment, and cross-Environment name uniqueness is checked inside
// the transaction. Any violation is returned (ErrInvalidName, ErrInvalidValue,
// or ErrDuplicateName) before data is touched, so a rejected save leaves prior
// data unchanged (Req 6.9).
//
// The Environment's active flag is persisted as given; to preserve the
// at-most-one-active invariant (Req 6.3), saving an Environment as active clears
// the active flag on all other Environments within the same transaction. An
// Environment that arrives without an ID is assigned a fresh one, and the
// fully-populated Environment (with assigned ID) is returned.
func (s *Store) SaveEnvironment(e model.Environment) (model.Environment, error) {
	// Defense-in-depth validation (Req 6.1, 6.2, 6.9): reject the whole save
	// before any write if the Environment name is out of bounds, any Variable is
	// invalid, or two Variables share a name.
	if err := validateEnvironment(e); err != nil {
		return model.Environment{}, err
	}
	if e.ID == "" {
		e.ID = newID()
	}
	err := s.withTx(func(tx *sql.Tx) error {
		// Enforce cross-Environment name uniqueness (Req 6.1). A collision with
		// any other Environment rejects the save; the withTx rollback leaves
		// prior data unchanged.
		var conflicts int
		if err := tx.QueryRow(
			`SELECT COUNT(1) FROM environments WHERE name = ? AND id <> ?`,
			e.Name, e.ID,
		).Scan(&conflicts); err != nil {
			return fmt.Errorf("store: check environment name: %w", err)
		}
		if conflicts > 0 {
			return fmt.Errorf("%w: environment %q", ErrDuplicateName, e.Name)
		}

		// Upsert the environment row, persisting the active flag (Req 6.7).
		if _, err := tx.Exec(
			`INSERT INTO environments(id, name, active) VALUES (?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET name = excluded.name, active = excluded.active`,
			e.ID, e.Name, boolToInt(e.Active),
		); err != nil {
			return fmt.Errorf("store: save environment: %w", err)
		}

		// Maintain the at-most-one-active invariant (Req 6.3): if this
		// Environment is active, every other Environment must be inactive.
		if e.Active {
			if _, err := tx.Exec(
				`UPDATE environments SET active = 0 WHERE id <> ?`, e.ID,
			); err != nil {
				return fmt.Errorf("store: clear other active environments: %w", err)
			}
		}

		// Rebuild the Variable set from scratch so the stored Variables mirror
		// the model exactly, including removals. Slice position becomes "ord" so
		// ListEnvironments reproduces display order (Req 6.7).
		if _, err := tx.Exec(
			`DELETE FROM variables WHERE environment_id = ?`, e.ID,
		); err != nil {
			return fmt.Errorf("store: clear environment variables: %w", err)
		}
		for i := range e.Variables {
			v := e.Variables[i]
			if _, err := tx.Exec(
				`INSERT INTO variables(environment_id, name, value, ord) VALUES (?, ?, ?, ?)`,
				e.ID, v.Name, v.Value, i,
			); err != nil {
				return fmt.Errorf("store: save variable %q: %w", v.Name, err)
			}
		}
		return nil
	})
	if err != nil {
		return model.Environment{}, err
	}
	return e, nil
}

// ---------------------------------------------------------------------------
// DeleteEnvironment
// ---------------------------------------------------------------------------

// DeleteEnvironment removes an Environment and, by ON DELETE CASCADE, all of its
// Variables. If the deleted Environment was the active one, no Environment is
// active afterwards (Req 6.10): the active flag lives on the deleted row, so
// removing the row simply leaves no active Environment. It returns ErrNotFound
// when no Environment has the given ID, leaving existing data untouched.
func (s *Store) DeleteEnvironment(id string) error {
	return s.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM environments WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("store: delete environment: %w", err)
		}
		return requireAffected(res)
	})
}

// ---------------------------------------------------------------------------
// SetActiveEnvironment
// ---------------------------------------------------------------------------

// SetActiveEnvironment makes the Environment with the given ID the single active
// Environment, clearing the active flag on all others so at most one is active
// (Req 6.3). Passing an empty id clears the active selection entirely, leaving
// no Environment active. It returns ErrNotFound when a non-empty id matches no
// Environment, leaving existing active state unchanged.
func (s *Store) SetActiveEnvironment(id string) error {
	return s.withTx(func(tx *sql.Tx) error {
		if id == "" {
			// Clear active on every Environment (Req 6.3): no Environment active.
			if _, err := tx.Exec(`UPDATE environments SET active = 0`); err != nil {
				return fmt.Errorf("store: clear active environment: %w", err)
			}
			return nil
		}

		// Activate the target. A zero-rows result means the ID does not exist;
		// report ErrNotFound and roll back without changing any active state.
		res, err := tx.Exec(`UPDATE environments SET active = 1 WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("store: set active environment: %w", err)
		}
		if err := requireAffected(res); err != nil {
			return err
		}

		// Clear active on all other Environments to keep at most one active.
		if _, err := tx.Exec(
			`UPDATE environments SET active = 0 WHERE id <> ?`, id,
		); err != nil {
			return fmt.Errorf("store: clear other active environments: %w", err)
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// ListEnvironments
// ---------------------------------------------------------------------------

// ListEnvironments returns every Environment with its Variables in stored order
// (Req 6.7). Environments are ordered by name (then ID for stability) and each
// Environment's Variables are ordered by their stored "ord". The persisted
// active flag is reported so callers can see which Environment, if any, is
// active.
func (s *Store) ListEnvironments() ([]model.Environment, error) {
	// Load Variables first, grouped by environment, so each Environment can be
	// populated in a single pass.
	vars, err := s.loadVariables()
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`SELECT id, name, active FROM environments ORDER BY name, id`)
	if err != nil {
		return nil, fmt.Errorf("store: list environments: %w", err)
	}
	defer rows.Close()

	out := []model.Environment{}
	for rows.Next() {
		var (
			id, name string
			active   int
		)
		if err := rows.Scan(&id, &name, &active); err != nil {
			return nil, fmt.Errorf("store: scan environment: %w", err)
		}
		variables := vars[id]
		if variables == nil {
			variables = []model.Variable{}
		}
		out = append(out, model.Environment{
			ID:        id,
			Name:      name,
			Variables: variables,
			Active:    active != 0,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate environments: %w", err)
	}
	return out, nil
}

// loadVariables returns the Variables grouped by environment_id in stored
// ("ord") order, ready to attach to each Environment in ListEnvironments.
func (s *Store) loadVariables() (map[string][]model.Variable, error) {
	rows, err := s.db.Query(
		`SELECT environment_id, name, value FROM variables ORDER BY environment_id, ord`)
	if err != nil {
		return nil, fmt.Errorf("store: list variables: %w", err)
	}
	defer rows.Close()

	byEnv := map[string][]model.Variable{}
	for rows.Next() {
		var envID string
		var v model.Variable
		if err := rows.Scan(&envID, &v.Name, &v.Value); err != nil {
			return nil, fmt.Errorf("store: scan variable: %w", err)
		}
		byEnv[envID] = append(byEnv[envID], v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate variables: %w", err)
	}
	return byEnv, nil
}

// boolToInt maps a Go bool to the 0/1 integer SQLite stores for boolean
// columns such as environments.active.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
