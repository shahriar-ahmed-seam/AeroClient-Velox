package httpcore

import (
	"bytes"
	"mime/multipart"
	"net/url"
	"strings"

	"volt/internal/model"
)

// Default Content-Type values applied by EncodeBody when the body type carries
// raw content (Req 2.3, 2.4). The form body types use their own media types
// (Req 2.6, 2.7) which are produced during encoding.
const (
	// ContentTypeJSON is the default Content-Type for a Raw JSON body.
	ContentTypeJSON = "application/json"
	// ContentTypeText is the default Content-Type for a Plain Text body.
	ContentTypeText = "text/plain"
	// ContentTypeURLEncoded is the Content-Type for an x-www-form-urlencoded body.
	ContentTypeURLEncoded = "application/x-www-form-urlencoded"
)

// EncodeBody encodes a BodySpec into the request body bytes and the
// Content-Type the engine should apply, per Requirement 2.
//
// Behavior by body type:
//   - None: no body and no Content-Type (Req 2.2) — returns (nil, "").
//   - Raw JSON: the raw content with a default Content-Type of
//     "application/json" (Req 2.3).
//   - Plain Text: the raw content with a default Content-Type of "text/plain"
//     (Req 2.4).
//   - x-www-form-urlencoded: the enabled, non-empty-key form fields encoded as
//     a URL-encoded form body with Content-Type
//     "application/x-www-form-urlencoded" (Req 2.6).
//   - form-data: the enabled, non-empty-key form fields encoded as a multipart
//     body with Content-Type "multipart/form-data" carrying the
//     engine-generated boundary (Req 2.7).
//
// Disabled rows and rows with an empty key are excluded from both form
// encodings (Req 2.8). When method is GET or HEAD the request is always
// bodyless regardless of the selected body type, so EncodeBody returns
// (nil, "") (Req 2.9).
//
// The returned Content-Type is the engine default for the body type. The
// user-set Content-Type override for Raw JSON / Plain Text (Req 2.5) is applied
// by the caller (PrepareRequest): when the headers table already supplies a
// Content-Type, that value takes precedence over this default. The form body
// types always carry their own media type because the multipart boundary is
// engine-generated and the urlencoded media type is fixed.
func EncodeBody(b model.BodySpec, method string) (body []byte, contentType string) {
	// GET and HEAD are always bodyless regardless of the configured body type.
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case model.MethodGet, model.MethodHead:
		return nil, ""
	}

	switch b.Type {
	case model.BodyJSON:
		return []byte(b.Raw), ContentTypeJSON
	case model.BodyText:
		return []byte(b.Raw), ContentTypeText
	case model.BodyURLEncoded:
		return encodeURLEncoded(b.FormFields), ContentTypeURLEncoded
	case model.BodyFormData:
		return encodeMultipart(b.FormFields)
	case model.BodyNone:
		return nil, ""
	default:
		// Unknown/unset body type is treated as None: no body, no Content-Type.
		return nil, ""
	}
}

// encodeURLEncoded encodes the enabled, non-empty-key form fields as an
// application/x-www-form-urlencoded body. Disabled rows and empty-key rows are
// excluded; rows sharing a key are all preserved (Req 2.6, 2.8).
func encodeURLEncoded(fields []model.KeyValue) []byte {
	values := url.Values{}
	for _, f := range fields {
		if f.Enabled && f.Key != "" {
			values.Add(f.Key, f.Value)
		}
	}
	return []byte(values.Encode())
}

// encodeMultipart encodes the enabled, non-empty-key form fields as a
// multipart/form-data body and returns the body together with the Content-Type
// carrying the generated boundary. Disabled rows and empty-key rows are
// excluded; rows sharing a key are all preserved (Req 2.7, 2.8).
func encodeMultipart(fields []model.KeyValue) (body []byte, contentType string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, f := range fields {
		if f.Enabled && f.Key != "" {
			// WriteField writes to an in-memory buffer; the only error path is
			// the underlying writer failing, which cannot happen for a
			// bytes.Buffer, so the error is safe to ignore here.
			_ = w.WriteField(f.Key, f.Value)
		}
	}
	// Close writes the trailing boundary; ignore its error for the same reason.
	_ = w.Close()
	return buf.Bytes(), w.FormDataContentType()
}
