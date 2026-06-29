package httpcore

import (
	"io"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 5: Form body round-trip excludes disabled and empty-key rows

// The two form body encodings are pure mappings from a slice of form fields to
// a request body plus the corresponding form media type (Req 2.6, 2.7):
//   - urlencoded → an application/x-www-form-urlencoded body.
//   - form-data  → a multipart/form-data body carrying an engine-generated
//     boundary in the Content-Type.
//
// In both cases the encoding includes exactly the enabled, non-empty-key rows
// (preserving rows that share a key) and excludes every disabled row and every
// empty-key row (Req 2.8). This property generates form fields that mix
// enabled/disabled rows, empty/non-empty keys, and duplicate keys, encodes them
// with a POST method (so the body is never suppressed by the GET/HEAD rule),
// then decodes the produced body back into a key→values map and asserts it
// equals exactly the enabled, non-empty-key rows.

// formKeyPool is a small pool of field keys so duplicates arise naturally. It
// includes keys that require percent-encoding under urlencoded (spaces, '&',
// '=', non-ASCII) while remaining safe for multipart field names (no quotes or
// newlines, which the multipart writer would need to escape).
var formKeyPool = []string{"a", "b", "dup", "q r", "k&v", "x=y", "é"}

// formValueRunes is the alphabet for generated field values; values may also be
// empty. It mixes characters that exercise percent-encoding for urlencoded.
var formValueRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789 -_&=%é")

// formRoundTripCase is a generated scenario: the chosen form body type plus a
// slice of fields mixing enabled/disabled, empty/non-empty keys, and
// duplicates.
type formRoundTripCase struct {
	bodyType string
	fields   []model.KeyValue
}

// genFormRows builds a random slice of form fields. With probability ~1/4 a row
// gets an empty key (which encoding must exclude); otherwise the key is drawn
// from formKeyPool so duplicates arise naturally. Enabled and value are random,
// and values may be empty.
func genFormRows(r *rand.Rand) []model.KeyValue {
	n := r.Intn(9) // 0..8 rows
	rows := make([]model.KeyValue, 0, n)
	for i := 0; i < n; i++ {
		key := ""
		if r.Intn(4) != 0 { // ~75% non-empty key
			key = formKeyPool[r.Intn(len(formKeyPool))]
		}
		value := ""
		if r.Intn(4) != 0 { // ~75% non-empty value
			value = randFrom(r, formValueRunes, 0, 10)
		}
		rows = append(rows, model.KeyValue{
			Key:     key,
			Value:   value,
			Enabled: r.Intn(2) == 0,
		})
	}
	return rows
}

// Generate implements quick.Generator for formRoundTripCase.
func (formRoundTripCase) Generate(r *rand.Rand, size int) reflect.Value {
	bodyType := model.BodyURLEncoded
	if r.Intn(2) == 0 {
		bodyType = model.BodyFormData
	}
	return reflect.ValueOf(formRoundTripCase{
		bodyType: bodyType,
		fields:   genFormRows(r),
	})
}

// expectedForm computes the key→values map a correct form encoding must
// round-trip to: the multiset of enabled, non-empty-key rows, with duplicates
// preserved and disabled/empty-key rows excluded (Req 2.6, 2.7, 2.8).
func expectedForm(fields []model.KeyValue) map[string][]string {
	want := map[string][]string{}
	for _, f := range fields {
		if f.Enabled && f.Key != "" {
			want[f.Key] = append(want[f.Key], f.Value)
		}
	}
	return want
}

// decodeMultipart reads a multipart/form-data body using the boundary carried
// in contentType and returns the decoded field key→values map.
func decodeMultipart(t *testing.T, body []byte, contentType string) (map[string][]string, bool) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Logf("ParseMediaType(%q) failed: %v", contentType, err)
		return nil, false
	}
	if mediaType != "multipart/form-data" {
		t.Logf("media type = %q, want multipart/form-data", mediaType)
		return nil, false
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Logf("multipart Content-Type %q carries no boundary", contentType)
		return nil, false
	}

	got := map[string][]string{}
	mr := multipart.NewReader(strings.NewReader(string(body)), boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Logf("NextPart failed: %v", err)
			return nil, false
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Logf("reading part %q failed: %v", part.FormName(), err)
			return nil, false
		}
		name := part.FormName()
		got[name] = append(got[name], string(data))
	}
	return got, true
}

func TestProp5_FormBodyRoundTrip(t *testing.T) {
	property := func(c formRoundTripCase) bool {
		b := model.BodySpec{Type: c.bodyType, FormFields: c.fields}
		body, contentType := EncodeBody(b, model.MethodPost)

		want := expectedForm(c.fields)

		switch c.bodyType {
		case model.BodyURLEncoded:
			// Req 2.6: the urlencoded media type is fixed.
			if contentType != ContentTypeURLEncoded {
				t.Logf("urlencoded Content-Type = %q, want %q", contentType, ContentTypeURLEncoded)
				return false
			}
			vals, err := url.ParseQuery(string(body))
			if err != nil {
				t.Logf("ParseQuery(%q) failed: %v", body, err)
				return false
			}
			if !reflect.DeepEqual(map[string][]string(vals), want) {
				t.Logf("urlencoded round-trip = %#v, want %#v (fields=%#v)", map[string][]string(vals), want, c.fields)
				return false
			}

		case model.BodyFormData:
			// Req 2.7: the multipart media type carries the generated boundary.
			got, ok := decodeMultipart(t, body, contentType)
			if !ok {
				return false
			}
			if !reflect.DeepEqual(got, want) {
				t.Logf("form-data round-trip = %#v, want %#v (fields=%#v)", got, want, c.fields)
				return false
			}

		default:
			t.Logf("unexpected body type %q", c.bodyType)
			return false
		}

		return true
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
