package httpcore

import (
	"errors"
	"math/rand"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

// Feature: volt-api-client, Property 3: Invalid URLs are rejected without a network call

// ValidateURL is the gate that every request must pass before any network call
// is made (Req 1.3): it parses and validates the raw URL and returns a parsed
// *url.URL only on success. On any failure it returns a nil URL together with a
// *ValidationError naming the failed rule. Because ValidateURL performs no I/O
// itself, a nil URL paired with a *ValidationError is exactly the condition
// under which the caller must abort — proving no network call could proceed.
//
// This property generates URLs that are guaranteed invalid across each of the
// rejection categories (empty, whitespace-only, missing scheme, unsupported
// scheme, missing host) and asserts that ValidateURL always rejects them with a
// nil URL and a *ValidationError whose rule matches the category.

// invalidKind enumerates the categories of invalid URL the generator produces.
type invalidKind int

const (
	kindEmpty invalidKind = iota
	kindWhitespace
	kindMissingScheme
	kindUnsupportedScheme
	kindMissingHost
)

// invalidURLCase is a generated invalid-URL scenario carrying the raw input and
// the validation rule it is expected to trigger.
type invalidURLCase struct {
	raw      string
	wantRule string
	kind     invalidKind
}

// schemeRunes is the alphabet for generated URL schemes: letters only, so every
// generated scheme is a syntactically valid scheme that url.Parse accepts.
var schemeRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// hostRunes is the alphabet for host/path-like text. It deliberately excludes
// ':' so a generated missing-scheme string can never be parsed as "scheme:..."
// and excludes whitespace and braces.
var hostRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789.-/")

// whitespaceRunes is the alphabet for whitespace-only inputs, all of which
// TrimSpace collapses to the empty string.
var whitespaceRunes = []rune(" \t\n\r\v\f")

func randFrom(r *rand.Rand, runes []rune, minLen, maxLen int) string {
	n := minLen + r.Intn(maxLen-minLen+1)
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteRune(runes[r.Intn(len(runes))])
	}
	return b.String()
}

// Generate implements quick.Generator, producing a raw URL guaranteed to be
// invalid for one randomly chosen rejection category along with the rule that
// category must yield.
func (invalidURLCase) Generate(r *rand.Rand, size int) reflect.Value {
	switch invalidKind(r.Intn(5)) {
	case kindEmpty:
		// The empty string trims to empty -> RuleEmptyURL.
		return reflect.ValueOf(invalidURLCase{raw: "", wantRule: RuleEmptyURL, kind: kindEmpty})

	case kindWhitespace:
		// Whitespace-only input trims to empty -> RuleEmptyURL.
		raw := randFrom(r, whitespaceRunes, 1, 6)
		return reflect.ValueOf(invalidURLCase{raw: raw, wantRule: RuleEmptyURL, kind: kindWhitespace})

	case kindMissingScheme:
		// Host/path text with no scheme and no ':' parses with an empty scheme
		// -> RuleMissingScheme. Force a leading letter so the value is non-empty
		// and never whitespace-only.
		raw := string(hostRunes[r.Intn(26)]) + randFrom(r, hostRunes, 0, 12)
		return reflect.ValueOf(invalidURLCase{raw: raw, wantRule: RuleMissingScheme, kind: kindMissingScheme})

	case kindUnsupportedScheme:
		// A scheme other than http/https with "://" -> RuleUnsupportedScheme.
		// The scheme check precedes the host check, so a host is optional here.
		var scheme string
		for {
			scheme = randFrom(r, schemeRunes, 1, 8)
			lower := strings.ToLower(scheme)
			if lower != "http" && lower != "https" {
				break
			}
		}
		raw := scheme + "://" + randFrom(r, hostRunes, 0, 10)
		return reflect.ValueOf(invalidURLCase{raw: raw, wantRule: RuleUnsupportedScheme, kind: kindUnsupportedScheme})

	default:
		// http/https with an empty host -> RuleMissingHost. Nothing appears
		// between "//" and the first '/', so the host is always empty.
		scheme := "http"
		if r.Intn(2) == 0 {
			scheme = "https"
		}
		raw := scheme + "://"
		if r.Intn(2) == 0 {
			raw += "/" + randFrom(r, hostRunes, 0, 8) // path only, still no host
		}
		return reflect.ValueOf(invalidURLCase{raw: raw, wantRule: RuleMissingHost, kind: kindMissingHost})
	}
}

func TestProp3_InvalidURLsRejectedWithoutNetworkCall(t *testing.T) {
	// The set of rules that all denote a rejected URL (no network call).
	rejectionRules := map[string]struct{}{
		RuleEmptyURL:          {},
		RuleMissingScheme:     {},
		RuleUnsupportedScheme: {},
		RuleMissingHost:       {},
		RuleMalformedURL:      {},
	}

	property := func(c invalidURLCase) bool {
		got, err := ValidateURL(c.raw)

		// A rejected URL must yield a nil parsed URL: with no URL to send, the
		// caller cannot proceed to a network call.
		if got != nil {
			t.Logf("ValidateURL(%q) returned non-nil URL %v on a rejected input", c.raw, got)
			return false
		}

		// The failure must be a *ValidationError naming a rejection rule.
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Logf("ValidateURL(%q) error = %v (%T), want *ValidationError", c.raw, err, err)
			return false
		}
		if _, ok := rejectionRules[ve.Rule]; !ok {
			t.Logf("ValidateURL(%q) rule = %q, not a recognized rejection rule", c.raw, ve.Rule)
			return false
		}

		// The rule must match the category the input was constructed for.
		if ve.Rule != c.wantRule {
			t.Logf("ValidateURL(%q) rule = %q, want %q", c.raw, ve.Rule, c.wantRule)
			return false
		}
		return true
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// TestProp3_NoSchemeNeverParsesAsScheme is a generator sanity guard: it confirms
// the missing-scheme generator alphabet truly produces empty-scheme URLs, so the
// property above exercises the intended rejection path rather than accidentally
// landing on a different rule.
func TestProp3_MissingSchemeGeneratorSanity(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		raw := string(hostRunes[r.Intn(26)]) + randFrom(r, hostRunes, 0, 12)
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			t.Fatalf("missing-scheme generator produced unparseable %q: %v", raw, err)
		}
		if parsed.Scheme != "" {
			t.Fatalf("missing-scheme generator produced %q with non-empty scheme %q", raw, parsed.Scheme)
		}
	}
}
