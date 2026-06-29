package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"volt/internal/model"
)

// This file implements the one-time migration of pre-existing localStorage
// History into the Go-managed store, guarded by the legacy_history_migrated
// meta flag so the import happens exactly once (Req 7.8). On failure the legacy
// data is left untouched and the flag stays unset so a later launch can retry
// (Req 7.9).

// MigrateLegacyHistory imports the supplied legacy History entries into the
// store exactly once. The first successful run records each entry and sets the
// legacy_history_migrated meta flag; any subsequent run observes the flag and
// returns immediately without re-importing, leaving the migrated History
// unchanged (idempotent, Req 7.8).
//
// The import and the flag write run inside a single transaction: if any insert
// fails the whole transaction rolls back, so the import is never partial and
// the flag remains unset, allowing the migration to be retried on a later
// launch (Req 7.9). The returned migrated flag is true only when this call
// performed the import; it is false when the migration had already completed.
func (s *Store) MigrateLegacyHistory(entries []model.HistoryEntry) (migrated bool, err error) {
	// Fast path: if the migration already ran, do not re-import (Req 7.8).
	done, err := s.isLegacyHistoryMigrated()
	if err != nil {
		return false, err
	}
	if done {
		return false, nil
	}

	err = s.withTx(func(tx *sql.Tx) error {
		// Re-check inside the transaction to guard against a concurrent
		// migration having completed between the read above and here.
		done, err := isLegacyHistoryMigratedTx(tx)
		if err != nil {
			return err
		}
		if done {
			migrated = false
			return nil
		}

		for _, h := range entries {
			if err := insertHistoryTx(tx, h); err != nil {
				return err
			}
		}

		// Enforce the 1000-entry cap the same way AddHistory does, in case the
		// legacy log exceeded it (Req 7.6): keep the most recent entries by
		// timestamp and discard the oldest.
		if _, err := tx.Exec(
			`DELETE FROM history WHERE id NOT IN (
			    SELECT id FROM history ORDER BY at DESC, id DESC LIMIT ?
			 )`,
			historyCap,
		); err != nil {
			return fmt.Errorf("store: prune migrated history: %w", err)
		}

		if err := setLegacyHistoryMigratedTx(tx); err != nil {
			return err
		}
		migrated = true
		return nil
	})
	if err != nil {
		// On failure the transaction rolled back, so the flag is unset and the
		// legacy data is preserved for a retry (Req 7.9).
		return false, err
	}
	return migrated, nil
}

// insertHistoryTx records a single legacy History entry within tx, mirroring
// AddHistory's insert: a missing ID is assigned a fresh one, a zero timestamp is
// stamped with the current time, and the full request configuration is
// serialized to JSON so it round-trips on read (Req 7.5).
func insertHistoryTx(tx *sql.Tx, h model.HistoryEntry) error {
	if h.ID == "" {
		h.ID = newID()
	}
	if h.At == 0 {
		h.At = time.Now().UnixMilli()
	}
	request, err := json.Marshal(h.Request)
	if err != nil {
		return fmt.Errorf("store: encode legacy history request: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO history(id, method, url, status, duration_ms, at, error, request)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		h.ID, h.Method, h.URL, h.Status, h.DurationMs, h.At, h.Error, string(request),
	); err != nil {
		return fmt.Errorf("store: migrate history entry: %w", err)
	}
	return nil
}

// isLegacyHistoryMigrated reports whether the one-time localStorage→store
// History migration has already completed (Req 7.8). It is true only when the
// meta flag is present and set to "1".
func (s *Store) isLegacyHistoryMigrated() (bool, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM meta WHERE key = ?`, metaKeyLegacyHistoryMigrated,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store: read legacy history migrated flag: %w", err)
	}
	return value == "1", nil
}

// isLegacyHistoryMigratedTx is the transactional variant of
// isLegacyHistoryMigrated, used to re-check the flag inside the migration
// transaction.
func isLegacyHistoryMigratedTx(tx *sql.Tx) (bool, error) {
	var value string
	err := tx.QueryRow(
		`SELECT value FROM meta WHERE key = ?`, metaKeyLegacyHistoryMigrated,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store: read legacy history migrated flag: %w", err)
	}
	return value == "1", nil
}

// setLegacyHistoryMigratedTx marks the one-time History migration complete by
// writing "1" to the legacy_history_migrated meta flag within tx. It is only
// called after every legacy entry has been inserted successfully, so the flag
// is set if and only if the import committed (Req 7.8, 7.9).
func setLegacyHistoryMigratedTx(tx *sql.Tx) error {
	if _, err := tx.Exec(
		`INSERT INTO meta(key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		metaKeyLegacyHistoryMigrated, "1",
	); err != nil {
		return fmt.Errorf("store: set legacy history migrated flag: %w", err)
	}
	return nil
}
