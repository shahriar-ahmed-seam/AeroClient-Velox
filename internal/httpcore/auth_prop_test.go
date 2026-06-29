package httpcore

import (
	"encoding/base64"
	"math/rand"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 7: Authorization derivation across all types with header precedence

// Authorization derivation (Req 3.2-3.9) is a single pure decision over an
// AuthSpec. This property generates random AuthSpecs spanning every type
// (None, Bearer, Basic, API Key in Header, API Key in Query, plus an
// unrecognized type) with fields that mix empty, whitespace-only, and
// meaningful values, then asserts DeriveAuthHeader obeys, for each type:
//
//   - Bearer: a token with a non-whitespace character yields
//     "Authorization: Bearer <token>"; empty/whitespace-only yields nothing
//     (Req 3.2, 3.3).
//   - Basic: a username with a non-whitespace character yields
//     "Authorization: Basic <base64(user:pass)>" decoding back to "user:pass"
//     with a missing password treated as empty; whitespace-only username yields
//     nothing (Req 3.4).
//   - API Key (Header): a name with a non-whitespace character yields that
//     request header (Req 3.5).
//   - API Key (Query): a name with a non-whitespace character yields exactly one
//     query name-value pair (Req 3.6).
//   - API Key with empty/whitespace-only name yields nothing (Req 3.7).
//   - None (and any unrecognized type) yields nothing (Req 3.8).
//
// It then checks the precedence rule (Req 3.9): whenever the spec produces an
// Authorization header, ApplyAuth must replace any pre-existing Authorization
// row in the headers table so exactly the derived value survives.

// authTypePool covers every recognized type plus one unrecognized value, which
// must behave like None (Req 3.8).
var authTypePool = []string{
	model.AuthNone, model.AuthBearer, model.AuthBasic, model.AuthAPIKey, "unrecognized",
}

// apiKeyLocationPool covers explicit header/query locations plus an empty
// location, which the derivation treats as header (Req 3.5).
var apiKeyLocationPool = []string{model.APIKeyInHeader, model.APIKeyInQuery, ""}

// authStringPool mixes empty, whitespace-only, and meaningful values (some
// padded with surrounding whitespace) so the whitespace-only guards in
// Req 3.3/3.7 are exercised alongside valid inputs.
var authStringPool = []string{
	"", "   ", "\t", "\n", " \t\n ",
	"token", "alice", "s3cret", "X-API-Key", "api_key",
	"  padded  ", "value:with:colons", "é-unicode",
}

// genAuthString draws a field value from the mixed pool.
func genAuthString(r *rand.Rand) string {
	return authStringPool[r.Intn(len(authStringPool))]
}

// authCase is a generated authorization scenario.
type authCase struct {
	spec model.AuthSpec
}

// Generate implements quick.Generator for authCase, producing an AuthSpec with
// a random type and randomly varied (often empty/whitespace) fields.
func (authCase) Generate(r *rand.Rand, size int) reflect.Value {
	spec := model.AuthSpec{
		Type:           authTypePool[r.Intn(len(authTypePool))],
		BearerToken:    genAuthString(r),
		BasicUser:      genAuthString(r),
		BasicPass:      genAuthString(r),
		APIKeyName:     genAuthString(r),
		APIKeyValue:    genAuthString(r),
		APIKeyLocation: apiKeyLocationPool[r.Intn(len(apiKeyLocationPool))],
	}
	return reflect.ValueOf(authCase{spec: spec})
}

// hasNonWhitespace reports whether s contains at least one non-whitespace
// character, matching the guard the derivation uses (strings.TrimSpace != "").
func hasNonWhitespace(s string) bool {
	return strings.TrimSpace(s) != ""
}

func TestProp7_AuthorizationDerivationAndPrecedence(t *testing.T) {
	property := func(c authCase) bool {
		a := c.spec
		headerName, headerValue, queryKey, queryValue := DeriveAuthHeader(a)

		switch a.Type {
		case model.AuthBearer:
			if hasNonWhitespace(a.BearerToken) {
				// Req 3.2: Authorization: Bearer <token>, no query pair.
				if headerName != HeaderAuthorization || headerValue != "Bearer "+a.BearerToken {
					t.Logf("bearer: got (%q, %q), want (Authorization, Bearer %q)", headerName, headerValue, a.BearerToken)
					return false
				}
				if queryKey != "" || queryValue != "" {
					t.Logf("bearer: unexpected query pair (%q, %q)", queryKey, queryValue)
					return false
				}
			} else if headerName != "" || headerValue != "" || queryKey != "" || queryValue != "" {
				// Req 3.3: empty/whitespace token adds nothing.
				t.Logf("bearer whitespace guard failed: header (%q, %q) query (%q, %q)", headerName, headerValue, queryKey, queryValue)
				return false
			}

		case model.AuthBasic:
			if hasNonWhitespace(a.BasicUser) {
				// Req 3.4: Authorization: Basic base64(user:pass).
				if headerName != HeaderAuthorization {
					t.Logf("basic: header name = %q, want Authorization", headerName)
					return false
				}
				const prefix = "Basic "
				if !strings.HasPrefix(headerValue, prefix) {
					t.Logf("basic: value %q missing %q prefix", headerValue, prefix)
					return false
				}
				decoded, err := base64.StdEncoding.DecodeString(headerValue[len(prefix):])
				if err != nil {
					t.Logf("basic: base64 decode failed: %v", err)
					return false
				}
				if string(decoded) != a.BasicUser+":"+a.BasicPass {
					t.Logf("basic: decoded = %q, want %q", decoded, a.BasicUser+":"+a.BasicPass)
					return false
				}
				if queryKey != "" || queryValue != "" {
					t.Logf("basic: unexpected query pair (%q, %q)", queryKey, queryValue)
					return false
				}
			} else if headerName != "" || headerValue != "" || queryKey != "" || queryValue != "" {
				// Req 3.3-style guard for Basic: whitespace-only user adds nothing.
				t.Logf("basic whitespace guard failed: header (%q, %q) query (%q, %q)", headerName, headerValue, queryKey, queryValue)
				return false
			}

		case model.AuthAPIKey:
			if !hasNonWhitespace(a.APIKeyName) {
				// Req 3.7: empty/whitespace name adds nothing.
				if headerName != "" || headerValue != "" || queryKey != "" || queryValue != "" {
					t.Logf("apikey whitespace guard failed: header (%q, %q) query (%q, %q)", headerName, headerValue, queryKey, queryValue)
					return false
				}
			} else if a.APIKeyLocation == model.APIKeyInQuery {
				// Req 3.6: exactly one query name-value pair, no header.
				if headerName != "" || headerValue != "" {
					t.Logf("apikey query: unexpected header (%q, %q)", headerName, headerValue)
					return false
				}
				if queryKey != a.APIKeyName || queryValue != a.APIKeyValue {
					t.Logf("apikey query: got (%q, %q), want (%q, %q)", queryKey, queryValue, a.APIKeyName, a.APIKeyValue)
					return false
				}
			} else {
				// Req 3.5: header location (explicit or defaulted) yields a header.
				if headerName != a.APIKeyName || headerValue != a.APIKeyValue {
					t.Logf("apikey header: got (%q, %q), want (%q, %q)", headerName, headerValue, a.APIKeyName, a.APIKeyValue)
					return false
				}
				if queryKey != "" || queryValue != "" {
					t.Logf("apikey header: unexpected query pair (%q, %q)", queryKey, queryValue)
					return false
				}
			}

		default:
			// Req 3.8: None and any unrecognized type add nothing.
			if headerName != "" || headerValue != "" || queryKey != "" || queryValue != "" {
				t.Logf("none/unknown should add nothing: header (%q, %q) query (%q, %q)", headerName, headerValue, queryKey, queryValue)
				return false
			}
		}

		// Req 3.9: whenever derivation produces an Authorization header, ApplyAuth
		// must override any pre-existing headers-table Authorization row so only
		// the derived value remains.
		if headerName == HeaderAuthorization {
			h := http.Header{}
			h.Add(HeaderAuthorization, "Bearer stale-from-headers-table")
			ApplyAuth(h, nil, a)
			got := h.Values(HeaderAuthorization)
			if len(got) != 1 || got[0] != headerValue {
				t.Logf("precedence failed: Authorization = %v, want [%q]", got, headerValue)
				return false
			}
		}

		return true
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
