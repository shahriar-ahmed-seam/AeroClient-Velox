package httpcore

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 4: Body encoding and Content-Type rules

// EncodeBody is a pure mapping from a BodySpec and a method to the request body
// bytes plus the engine-default Content-Type (Req 2.2, 2.3, 2.4, 2.9). This
// property exercises the raw-content and bodyless rules:
//   - None (and any unknown/unset type) → (nil, "") (Req 2.2).
//   - Raw JSON → the raw content with the "application/json" default (Req 2.3).
//   - Plain Text → the raw content with the "text/plain" default (Req 2.4).
//   - GET or HEAD → (nil, "") regardless of the selected body type (Req 2.9).
//
// The user-set Content-Type override (Req 2.5) is applied by the caller
// (PrepareRequest), not by EncodeBody; EncodeBody always returns the engine
// default for the body type, which is the value the override replaces. This
// property therefore pins down the default that Req 2.5 builds upon. The
// form-body encodings (urlencoded, form-data) are validated by Property 5.

// bodyEncMethods spans every selectable method so the GET/HEAD bodyless rule is
// exercised against the body-bearing verbs as well (Req 2.9).
var bodyEncMethods = []string{
	model.MethodGet, model.MethodPost, model.MethodPut, model.MethodPatch,
	model.MethodDelete, model.MethodHead, model.MethodOptions,
}

// bodyEncTypes spans the raw/bodyless types covered by this property plus an
// unknown type (treated as None) and the form types (which GET/HEAD must still
// reduce to a bodyless request).
var bodyEncTypes = []string{
	model.BodyNone, model.BodyJSON, model.BodyText,
	model.BodyURLEncoded, model.BodyFormData, "unknown",
}

// rawRunes is the alphabet for generated raw body content; raw content may also
// be empty.
var rawRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789 {}\":,\n")

// bodyEncCase is a generated scenario: a method plus a BodySpec whose Type and
// Raw vary across the input space.
type bodyEncCase struct {
	method string
	body   model.BodySpec
}

// Generate implements quick.Generator for bodyEncCase.
func (bodyEncCase) Generate(r *rand.Rand, size int) reflect.Value {
	raw := ""
	if r.Intn(4) != 0 { // ~75% non-empty raw content
		raw = randFrom(r, rawRunes, 0, 24)
	}
	return reflect.ValueOf(bodyEncCase{
		method: bodyEncMethods[r.Intn(len(bodyEncMethods))],
		body: model.BodySpec{
			Type: bodyEncTypes[r.Intn(len(bodyEncTypes))],
			Raw:  raw,
		},
	})
}

// isGetOrHead reports whether the method is GET or HEAD, which are always
// bodyless (Req 2.9).
func isGetOrHead(method string) bool {
	return method == model.MethodGet || method == model.MethodHead
}

func TestProp4_BodyEncodingAndContentType(t *testing.T) {
	property := func(c bodyEncCase) bool {
		body, contentType := EncodeBody(c.body, c.method)

		// Req 2.9: GET and HEAD are always bodyless regardless of body type.
		if isGetOrHead(c.method) {
			if body != nil || contentType != "" {
				t.Logf("GET/HEAD method=%q type=%q produced body=%q ct=%q, want (nil, \"\")",
					c.method, c.body.Type, body, contentType)
				return false
			}
			return true
		}

		switch c.body.Type {
		case model.BodyNone, "unknown":
			// Req 2.2: None (and any unknown/unset type) yields no body and no
			// Content-Type.
			if body != nil || contentType != "" {
				t.Logf("type=%q produced body=%q ct=%q, want (nil, \"\")",
					c.body.Type, body, contentType)
				return false
			}
		case model.BodyJSON:
			// Req 2.3: Raw JSON carries the raw content with the JSON default.
			if string(body) != c.body.Raw || contentType != ContentTypeJSON {
				t.Logf("JSON raw=%q produced body=%q ct=%q, want body=%q ct=%q",
					c.body.Raw, body, contentType, c.body.Raw, ContentTypeJSON)
				return false
			}
		case model.BodyText:
			// Req 2.4: Plain Text carries the raw content with the text default.
			if string(body) != c.body.Raw || contentType != ContentTypeText {
				t.Logf("Text raw=%q produced body=%q ct=%q, want body=%q ct=%q",
					c.body.Raw, body, contentType, c.body.Raw, ContentTypeText)
				return false
			}
		default:
			// Form types are validated by Property 5; this property only
			// requires that they are not treated as bodyless on a body-bearing
			// method, i.e. they carry a Content-Type.
			if contentType == "" {
				t.Logf("form type=%q on method=%q produced empty Content-Type", c.body.Type, c.method)
				return false
			}
		}
		return true
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
