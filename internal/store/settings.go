package store

import (
	"database/sql"
	"errors"
	"fmt"

	"volt/internal/model"
)

// This file implements Settings persistence: reading the single Settings row
// (synthesizing first-launch defaults when none exists) and saving it with
// timeout validation.
//
// Behavioral rules enforced here:
//   - On first launch, before any Settings have been saved, GetSettings reports
//     the defaults: System theme, TLS verification enabled, and a 30-second
//     timeout (Req 9.8).
//   - SaveSettings accepts a request timeout only within 1..600 seconds
//     inclusive; a value outside that range is rejected and the previously
//     stored (or default) timeout is retained instead, while the remaining
//     settings are still persisted (Req 9.6).
//   - Settings round-trip across reopen of the store (Req 9.7); persistence is
//     a single pinned row (id = 1) upserted on every save.

// Settings defaults applied on first launch, before any Settings row exists
// (Req 9.8).
const (
	// DefaultTheme is the theme selected before the user chooses one. "system"
	// follows the operating-system light/dark preference.
	DefaultTheme = "system"
	// DefaultTLSVerify is the first-launch TLS verification state: enabled, so
	// certificates are verified until the user explicitly opts out.
	DefaultTLSVerify = true
	// DefaultTimeoutSeconds is the first-launch request timeout in seconds.
	DefaultTimeoutSeconds = 30
)

// Allowed request-timeout bounds in seconds, inclusive (Req 9.6).
const (
	// MinTimeoutSeconds is the smallest accepted request timeout.
	MinTimeoutSeconds = 1
	// MaxTimeoutSeconds is the largest accepted request timeout.
	MaxTimeoutSeconds = 600
)

// settingsRowID is the fixed primary key of the single Settings row. The
// settings table pins id to 1 (see schema), so all reads and writes target this
// one row.
const settingsRowID = 1

// defaultSettings returns a fresh copy of the first-launch Settings (Req 9.8):
// System theme, TLS verification enabled, a 30-second timeout, and no proxy.
func defaultSettings() model.Settings {
	return model.Settings{
		Theme:          DefaultTheme,
		TLSVerify:      DefaultTLSVerify,
		TimeoutSeconds: DefaultTimeoutSeconds,
		ProxyURL:       "",
	}
}

// validTimeout reports whether t is an accepted request timeout, i.e. within
// 1..600 seconds inclusive (Req 9.6).
func validTimeout(t int) bool {
	return t >= MinTimeoutSeconds && t <= MaxTimeoutSeconds
}

// GetSettings returns the effective Settings. When no Settings row exists yet
// (first launch), it returns the defaults — System theme, TLS verification
// enabled, and a 30-second timeout (Req 9.8) — without persisting them; the
// defaults only become a stored row once SaveSettings is called. When a row
// exists it is returned verbatim, so Settings survive a restart (Req 9.7).
func (s *Store) GetSettings() (model.Settings, error) {
	var out model.Settings
	err := s.db.QueryRow(
		`SELECT theme, tls_verify, timeout_seconds, proxy_url
		 FROM settings WHERE id = ?`, settingsRowID,
	).Scan(&out.Theme, &out.TLSVerify, &out.TimeoutSeconds, &out.ProxyURL)
	if errors.Is(err, sql.ErrNoRows) {
		// First launch: no row yet, report the defaults (Req 9.8).
		return defaultSettings(), nil
	}
	if err != nil {
		return model.Settings{}, fmt.Errorf("store: get settings: %w", err)
	}
	return out, nil
}

// SaveSettings persists the supplied Settings into the single pinned row,
// upserting so the values survive a restart (Req 9.7).
//
// Timeout validation (Req 9.6): if s.TimeoutSeconds falls outside 1..600
// seconds, the out-of-range value is not rejected wholesale; instead the
// previously stored timeout (or the 30-second default when none has been saved)
// is retained while the rest of the Settings are persisted. A timeout within
// range is stored as given.
func (s *Store) SaveSettings(in model.Settings) error {
	return s.withTx(func(tx *sql.Tx) error {
		// Resolve the timeout to store. An in-range value is taken as-is; an
		// out-of-range value retains the previously stored timeout, falling back
		// to the default when no row exists yet (Req 9.6).
		timeout := in.TimeoutSeconds
		if !validTimeout(timeout) {
			var prev int
			err := tx.QueryRow(
				`SELECT timeout_seconds FROM settings WHERE id = ?`, settingsRowID,
			).Scan(&prev)
			switch {
			case errors.Is(err, sql.ErrNoRows):
				prev = DefaultTimeoutSeconds
			case err != nil:
				return fmt.Errorf("store: read previous timeout: %w", err)
			}
			timeout = prev
		}

		if _, err := tx.Exec(
			`INSERT INTO settings(id, theme, tls_verify, timeout_seconds, proxy_url)
			 VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET
			    theme = excluded.theme,
			    tls_verify = excluded.tls_verify,
			    timeout_seconds = excluded.timeout_seconds,
			    proxy_url = excluded.proxy_url`,
			settingsRowID, in.Theme, boolToInt(in.TLSVerify), timeout, in.ProxyURL,
		); err != nil {
			return fmt.Errorf("store: save settings: %w", err)
		}
		return nil
	})
}
