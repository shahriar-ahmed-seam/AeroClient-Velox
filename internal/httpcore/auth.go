package httpcore

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"volt/internal/model"
)

// HeaderAuthorization is the canonical name of the HTTP Authorization header.
// Bearer and Basic auth both produce this header, and a produced value takes
// precedence over any Authorization row configured in the headers table
// (Req 3.9).
const HeaderAuthorization = "Authorization"

// DeriveAuthHeader resolves an AuthSpec into the pieces the engine applies to a
// request, per Requirement 3. It performs no I/O and returns, in order:
//
//   - headerName, headerValue: a request header to set. For Bearer and Basic
//     this is the "Authorization" header (Req 3.2, 3.4); for API Key in Header
//     it is the configured key name (Req 3.5). Empty headerName means no header
//     is contributed.
//   - queryKey, queryValue: a single query-string name-value pair to append,
//     produced only for API Key in Query (Req 3.6). Empty queryKey means no
//     query parameter is contributed.
//
// Whitespace-only guards (Req 3.3, 3.7) and the None type (Req 3.8) cause the
// corresponding outputs to be empty so the request is sent without that auth:
//
//   - Bearer: a token with at least one non-whitespace character yields
//     "Authorization: Bearer <token>"; an empty or whitespace-only token yields
//     no header (Req 3.2, 3.3).
//   - Basic: a username with at least one non-whitespace character yields
//     "Authorization: Basic <base64(user:pass)>", with a missing password
//     treated as the empty string (Req 3.4).
//   - API Key (Header): a key name with at least one non-whitespace character
//     yields that name and value as a request header (Req 3.5).
//   - API Key (Query): a key name with at least one non-whitespace character
//     yields that name and value as a query parameter (Req 3.6).
//   - API Key with an empty or whitespace-only name adds nothing (Req 3.7).
//   - None (or any unrecognized type) adds nothing (Req 3.8).
//
// The actual override of a headers-table Authorization row (Req 3.9) is applied
// by ApplyAuth, which uses Header.Set for the Authorization header.
func DeriveAuthHeader(a model.AuthSpec) (headerName, headerValue, queryKey, queryValue string) {
	switch a.Type {
	case model.AuthBearer:
		if strings.TrimSpace(a.BearerToken) == "" {
			return "", "", "", ""
		}
		return HeaderAuthorization, "Bearer " + a.BearerToken, "", ""

	case model.AuthBasic:
		if strings.TrimSpace(a.BasicUser) == "" {
			return "", "", "", ""
		}
		// A missing password is treated as an empty string (Req 3.4).
		credentials := a.BasicUser + ":" + a.BasicPass
		encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
		return HeaderAuthorization, "Basic " + encoded, "", ""

	case model.AuthAPIKey:
		if strings.TrimSpace(a.APIKeyName) == "" {
			return "", "", "", ""
		}
		if a.APIKeyLocation == model.APIKeyInQuery {
			return "", "", a.APIKeyName, a.APIKeyValue
		}
		// Default and explicit "header" location both place the key in a header.
		return a.APIKeyName, a.APIKeyValue, "", ""

	default:
		// model.AuthNone and any unrecognized type contribute nothing.
		return "", "", "", ""
	}
}

// ApplyAuth derives the authorization from a and applies it to the request
// header h and URL u. A produced Authorization header is written with
// Header.Set so it replaces (overrides) any Authorization row already present
// in the headers table, satisfying Requirement 3.9. An API-Key-in-header value
// is also applied with Header.Set so the configured key is authoritative. An
// API-Key-in-query pair is appended as exactly one name-value pair to u's query
// string (Req 3.6).
//
// h must be non-nil. u may be nil when no query mutation is possible; in that
// case any derived query pair is silently skipped.
func ApplyAuth(h http.Header, u *url.URL, a model.AuthSpec) {
	headerName, headerValue, queryKey, queryValue := DeriveAuthHeader(a)

	if headerName != "" {
		h.Set(headerName, headerValue)
	}

	if queryKey != "" && u != nil {
		q := u.Query()
		q.Add(queryKey, queryValue)
		u.RawQuery = q.Encode()
	}
}
