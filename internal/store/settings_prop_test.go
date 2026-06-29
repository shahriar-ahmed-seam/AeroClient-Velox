package store

import (
	"fmt"
	"math/rand"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 22: Timeout value validation
//
// Validates: Requirements 9.6
//
// TestTimeoutValueValidation is the property-based test for Property 22. It
// drives a fresh in-memory store through a random sequence of SaveSettings
// calls whose TimeoutSeconds span both the accepted range (1..600) and values
// outside it (-100..1000), with random theme/tls/proxy on every save. After
// each save it reads the settings back and asserts the timeout rules from
// Req 9.6:
//
//	(a) When the saved timeout was in-range (1..600), the stored timeout equals
//	    exactly what was saved.
//	(b) When the saved timeout was out-of-range, the stored timeout retains the
//	    previous effective timeout — the value last accepted, or the 30-second
//	    default before any valid save has occurred.
//	(c) Regardless of input, the stored timeout always stays within 1..600.
//
// It also asserts the non-timeout fields (theme, tlsVerify, proxy) always
// reflect the latest save, since an out-of-range timeout retains the prior
// timeout but still persists the rest of the settings.
func TestTimeoutValueValidation(t *testing.T) {
	var failMsg string

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))

		s, err := Open(":memory:")
		if err != nil {
			failMsg = fmt.Sprintf("Open: %v", err)
			return false
		}
		defer s.Close()

		// effective tracks the timeout we expect the store to hold. Before any
		// valid save it is the 30-second default (Req 9.8); each accepted
		// in-range save updates it, each rejected save leaves it unchanged.
		effective := DefaultTimeoutSeconds

		ops := rng.Intn(40) + 1
		for i := 0; i < ops; i++ {
			in := genTimeoutSettings(rng)

			if err := s.SaveSettings(in); err != nil {
				failMsg = fmt.Sprintf("SaveSettings(timeout=%d): %v", in.TimeoutSeconds, err)
				return false
			}

			// Update the expected effective timeout following the Req 9.6 rule.
			inRange := in.TimeoutSeconds >= MinTimeoutSeconds && in.TimeoutSeconds <= MaxTimeoutSeconds
			if inRange {
				effective = in.TimeoutSeconds
			}

			got, err := s.GetSettings()
			if err != nil {
				failMsg = fmt.Sprintf("GetSettings: %v", err)
				return false
			}

			// (a)/(b) Stored timeout matches the expected effective value.
			if got.TimeoutSeconds != effective {
				failMsg = fmt.Sprintf(
					"op %d: saved timeout %d (inRange=%v) -> stored %d, want %d (Req 9.6)",
					i, in.TimeoutSeconds, inRange, got.TimeoutSeconds, effective)
				return false
			}

			// (c) Stored timeout is always within the accepted bounds.
			if got.TimeoutSeconds < MinTimeoutSeconds || got.TimeoutSeconds > MaxTimeoutSeconds {
				failMsg = fmt.Sprintf(
					"op %d: stored timeout %d outside 1..600 (Req 9.6)", i, got.TimeoutSeconds)
				return false
			}

			// Non-timeout fields always reflect the latest save.
			if got.Theme != in.Theme {
				failMsg = fmt.Sprintf("op %d: theme got %q, want %q", i, got.Theme, in.Theme)
				return false
			}
			if got.TLSVerify != in.TLSVerify {
				failMsg = fmt.Sprintf("op %d: tlsVerify got %v, want %v", i, got.TLSVerify, in.TLSVerify)
				return false
			}
			if got.ProxyURL != in.ProxyURL {
				failMsg = fmt.Sprintf("op %d: proxyUrl got %q, want %q", i, got.ProxyURL, in.ProxyURL)
				return false
			}
		}
		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 22 failed: %v\n%s", err, failMsg)
	}
}

// genTimeoutSettings builds a Settings value whose TimeoutSeconds is drawn from
// a span that straddles the accepted 1..600 range — sometimes in-range,
// sometimes below the minimum or above the maximum — so the property exercises
// both the accept and the retain-previous branches of SaveSettings. The theme,
// tlsVerify, and proxy fields are randomized to confirm they always persist
// regardless of the timeout outcome.
func genTimeoutSettings(rng *rand.Rand) model.Settings {
	themes := []string{"light", "dark", "system"}
	// Timeout in [-100, 1000): roughly 40% in-range, the rest out-of-range on
	// either side, so both branches are hit frequently across iterations.
	timeout := rng.Intn(1100) - 100
	return model.Settings{
		Theme:          themes[rng.Intn(len(themes))],
		TLSVerify:      rng.Intn(2) == 0,
		TimeoutSeconds: timeout,
		ProxyURL:       randStr(rng, 0, 16),
	}
}
