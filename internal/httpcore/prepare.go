package httpcore

import (
	"net/http"

	"volt/internal/model"
)

// HeaderContentType is the canonical name of the HTTP Content-Type header. A
// Content-Type supplied in the headers table takes precedence over the engine
// default for Raw JSON / Plain Text bodies (Req 2.5).
const HeaderContentType = "Content-Type"

// PreparedRequest is the fully-resolved, ready-to-send request produced by
// PrepareRequest. Every variable token has been interpolated, the URL has been
// validated and had its query parameters and any API-key-in-query pair merged
// in, headers have been assembled with authorization applied, and the body has
// been encoded. It carries no unresolved configuration: a caller (Execute) can
// turn it directly into an *http.Request.
type PreparedRequest struct {
	// Method is the HTTP verb, unchanged from the input request.
	Method string
	// URL is the interpolated, validated URL with query params and any
	// API-key-in-query pair merged in.
	URL string
	// Header is the assembled request header with authorization applied and the
	// Content-Type removed (the Content-Type is carried in ContentType).
	Header http.Header
	// Body is the encoded request body bytes; nil/empty when there is no body.
	Body []byte
	// ContentType is the Content-Type to set on the request. An empty string
	// means no Content-Type should be set.
	ContentType string
}

// PrepareRequest resolves, validates, and encodes a raw request into a
// PreparedRequest. It performs NO network I/O.
//
// The steps, in order:
//
//  1. Interpolate {{name}} tokens against the active environment env across the
//     URL, every query parameter (key and value), every header (key and
//     value), and the body (raw content and form-field keys/values), per
//     Requirements 6.4 and 6.5. Tokens with no match are left literal so the
//     request can still be prepared (Req 6.8).
//  2. Validate the interpolated URL. On failure a *ValidationError naming the
//     failed rule is returned with a zero PreparedRequest and no further work
//     (Req 1.3); the caller must not perform a network call.
//  3. Merge enabled, non-empty query parameters into the URL query string,
//     excluding disabled and empty-key rows (Req 1.2, 1.4, 1.5).
//  4. Build the request headers from enabled, non-empty header rows, preserving
//     duplicate header names (Req 1.6).
//  5. Encode the body per its BodySpec and method (GET/HEAD are bodyless), and
//     resolve the Content-Type: a Content-Type present in the headers table
//     wins over the engine default for raw JSON / plain-text bodies (Req 2.5),
//     while form bodies always use their engine-generated media type (which
//     carries the multipart boundary). The resolved Content-Type is moved out
//     of the header map into PreparedRequest.ContentType so there is a single
//     authoritative value.
//  6. Apply authorization, deriving the header and/or query pair from the auth
//     spec. A produced Authorization header overrides any Authorization row
//     from the headers table (Req 3.9), and an API-key-in-query pair is
//     appended to the URL query.
func PrepareRequest(r model.Request, env model.Environment, s model.Settings) (PreparedRequest, error) {
	// 1. Interpolate every field that supports {{name}} tokens (Req 6.4, 6.5).
	interpolatedURL, _ := InterpolateString(r.URL, env)
	params := interpolateKeyValues(r.Params, env)
	headers := interpolateKeyValues(r.Headers, env)
	body := interpolateBody(r.Body, env)

	// 2. Validate the resolved URL before doing anything else (Req 1.3). On
	// failure return the ValidationError and a zero PreparedRequest so the
	// caller performs no network call.
	parsedURL, err := ValidateURL(interpolatedURL)
	if err != nil {
		return PreparedRequest{}, err
	}

	// 3. Merge enabled, non-empty query parameters into the URL (Req 1.2, 1.4, 1.5).
	MergeParams(parsedURL, params)

	// 4. Assemble headers, preserving duplicate names (Req 1.6).
	header := BuildHeaders(headers)

	// 5. Encode the body and resolve the Content-Type with headers-table
	// override precedence for raw bodies (Req 2.2-2.9, 2.5).
	encodedBody, defaultContentType := EncodeBody(body, r.Method)
	contentType := resolveContentType(header, body.Type, defaultContentType)
	// The Content-Type lives in PreparedRequest.ContentType, so remove any copy
	// from the header map to keep a single authoritative value.
	header.Del(HeaderContentType)

	// 6. Apply authorization to the header and URL. A produced Authorization
	// header overrides any headers-table Authorization row (Req 3.9); an
	// API-key-in-query pair is appended to the query (Req 3.6).
	ApplyAuth(header, parsedURL, r.Auth)

	return PreparedRequest{
		Method:      r.Method,
		URL:         parsedURL.String(),
		Header:      header,
		Body:        encodedBody,
		ContentType: contentType,
	}, nil
}

// resolveContentType determines the Content-Type to apply, honoring the
// user-set headers-table override for raw JSON / plain-text bodies (Req 2.5)
// while always using the engine-generated media type for form bodies (whose
// multipart boundary must match the encoded body). When no headers-table
// Content-Type is present, the engine default is used. An empty result means no
// Content-Type should be set.
func resolveContentType(header http.Header, bodyType model.BodyType, defaultContentType string) string {
	existing := header.Get(HeaderContentType)

	switch bodyType {
	case model.BodyFormData:
		// Form-data carries an engine-generated boundary, so the default media
		// type is authoritative.
		return defaultContentType
	default:
		if existing != "" {
			return existing
		}
		return defaultContentType
	}
}

// interpolateKeyValues returns a copy of rows with {{name}} tokens resolved in
// both the key and value of every row (Req 6.5). Enabled flags are preserved so
// downstream assembly still excludes disabled rows.
func interpolateKeyValues(rows []model.KeyValue, env model.Environment) []model.KeyValue {
	if len(rows) == 0 {
		return nil
	}
	out := make([]model.KeyValue, len(rows))
	for i, row := range rows {
		key, _ := InterpolateString(row.Key, env)
		value, _ := InterpolateString(row.Value, env)
		out[i] = model.KeyValue{Key: key, Value: value, Enabled: row.Enabled}
	}
	return out
}

// interpolateBody returns a copy of b with {{name}} tokens resolved in the raw
// content and in every form field's key and value (Req 6.5). The body type is
// preserved so encoding behaves identically to the unresolved spec.
func interpolateBody(b model.BodySpec, env model.Environment) model.BodySpec {
	raw, _ := InterpolateString(b.Raw, env)
	return model.BodySpec{
		Type:       b.Type,
		Raw:        raw,
		FormFields: interpolateKeyValues(b.FormFields, env),
	}
}
