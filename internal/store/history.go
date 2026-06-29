package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"volt/internal/model"
)

// This file implements History persistence: recording an executed request,
// listing recorded entries newest-first, and clearing the log.
//
// Behavioral rules enforced here:
//   - A History entry records the full request configuration plus the outcome
//     (method, URL, status, elapsed time, timestamp, and error) for both
//     successful and failed executions (Req 7.1, 7.2). Error is "" on success.
//   - Entries are presented in reverse-chronological order, newest first
//     (Req 7.3), and round-trip across reopen with their configuration intact
//     (Req 7.5).
//   - The log is capped at 1000 entries; once the cap is exceeded the oldest
//     entries are discarded so only the most recent 1000 are retained (Req 7.6).

// historyCap is the maximum number of History entries retained. Once recording
// pushes the count past this limit, the oldest entries (by timestamp) are
// pruned so only the most recent historyCap entries remain (Req 7.6).
const historyCap = 1000

// AddHistory records a single executed request, capturing its full
// configuration and outcome (Req 7.1, 7.2). The entry's structured request
// configuration is serialized to JSON in the request column so it can be
// restored later (Req 7.5). The insert and the cap-enforcing prune run in one
// transaction so the log never observes a state that exceeds the cap (Req 7.6).
//
// An entry that arrives without an ID is assigned a fresh one, and an entry with
// a zero timestamp is stamped with the current time (Unix milliseconds); a
// caller-provided timestamp is preserved. After insertion the log is pruned to
// the most recent historyCap entries, discarding the oldest by timestamp.
func (s *Store) AddHistory(h model.HistoryEntry) error {
	if h.ID == "" {
		h.ID = newID()
	}
	if h.At == 0 {
		h.At = time.Now().UnixMilli()
	}
	request, err := json.Marshal(h.Request)
	if err != nil {
		return fmt.Errorf("store: encode history request: %w", err)
	}
	return s.withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`INSERT INTO history(id, method, url, status, duration_ms, at, error, request)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			h.ID, h.Method, h.URL, h.Status, h.DurationMs, h.At, h.Error, string(request),
		); err != nil {
			return fmt.Errorf("store: add history: %w", err)
		}

		// Enforce the 1000-entry cap (Req 7.6): keep the most recent entries by
		// timestamp (id breaks ties to match ListHistory's ordering) and delete
		// everything beyond the cap, which is the oldest.
		if _, err := tx.Exec(
			`DELETE FROM history WHERE id NOT IN (
			    SELECT id FROM history ORDER BY at DESC, id DESC LIMIT ?
			 )`,
			historyCap,
		); err != nil {
			return fmt.Errorf("store: prune history: %w", err)
		}
		return nil
	})
}

// ListHistory returns every recorded History entry in reverse-chronological
// order, newest first (Req 7.3). Entries with equal timestamps are ordered by
// ID descending for a stable result. Each entry's request configuration is
// rehydrated from the JSON stored in the request column so a selected entry can
// restore its full configuration (Req 7.5).
func (s *Store) ListHistory() ([]model.HistoryEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, method, url, status, duration_ms, at, error, request
		 FROM history ORDER BY at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: list history: %w", err)
	}
	defer rows.Close()

	out := []model.HistoryEntry{}
	for rows.Next() {
		var (
			h       model.HistoryEntry
			request string
		)
		if err := rows.Scan(
			&h.ID, &h.Method, &h.URL, &h.Status, &h.DurationMs, &h.At, &h.Error, &request,
		); err != nil {
			return nil, fmt.Errorf("store: scan history: %w", err)
		}
		if err := json.Unmarshal([]byte(request), &h.Request); err != nil {
			return nil, fmt.Errorf("store: decode history request %q: %w", h.ID, err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate history: %w", err)
	}
	return out, nil
}

// ClearHistory deletes all recorded History entries (Req 7.7 backstop), leaving
// the rest of the stored data untouched. Clearing an already-empty log is a
// no-op and reports no error.
func (s *Store) ClearHistory() error {
	return s.withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM history`); err != nil {
			return fmt.Errorf("store: clear history: %w", err)
		}
		return nil
	})
}
