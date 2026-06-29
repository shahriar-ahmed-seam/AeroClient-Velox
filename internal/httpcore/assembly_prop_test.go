package httpcore

import (
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 1: Request assembly includes enabled rows, excludes disabled rows, and preserves method and duplicate header names

// Request assembly is the composition of three pure steps (Req 1.2, 1.4, 1.5,
// 1.6): the selected method is carried onto the executed request unchanged,
// MergeParams percent-encodes every enabled non-empty-key query parameter onto
// the URL (excluding disabled and empty-key rows, preserving rows that share a
// key), and BuildHeaders sets every enabled non-empty-key header (excluding
// disabled and empty-key rows, preserving rows that share a name via
// Header.Add). This property generates random methods together with parameter
// and header slices that mix enabled/disabled rows, empty/non-empty keys, and
// duplicate keys, assembles an *http.Request the way PrepareRequest will, and
// asserts the assembly is exactly the multiset of enabled non-empty rows with
// the method preserved.

// assemblyMethods is the set of methods the editor can select (Req 1.1). The
// property only relies on the method passing through assembly unchanged.
var assemblyMethods = []string{
	model.MethodGet, model.MethodPost, model.MethodPut, model.MethodPatch,
	model.MethodDelete, model.MethodHead, model.MethodOptions,
}

// paramKeyPool is a small pool of parameter keys so that random generation
// naturally produces duplicate keys. It deliberately includes values that
// require percent-encoding (spaces, '&', '=', non-ASCII) so the encoding rule
// in Req 1.4 is exercised.
var paramKeyPool = []string{"a", "b", "dup", "q r", "k&v", "x=y", "é"}

// headerKeyPool is a small pool of header names. It mixes case ("X-Custom" vs
// "x-custom") so canonicalization-driven merging is exercised, and repeats names
// so duplicate-name preservation (Req 1.6) is exercised.
var headerKeyPool = []string{"X-Custom", "x-custom", "Accept", "Authorization", "X-Token"}

// kvValueRunes is the alphabet for generated values; values may also be empty.
var kvValueRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789 -_")

// assemblyCase is a generated assembly scenario: a selected method plus
// parameter and header rows that mix enabled/disabled, empty/non-empty keys,
// and duplicates.
type assemblyCase struct {
	method  string
	params  []model.KeyValue
	headers []model.KeyValue
}

// genRows builds a random slice of KeyValue rows. With probability ~1/4 a row
// gets an empty key (which assembly must exclude); otherwise the key is drawn
// from keyPool so duplicates arise naturally. Enabled and value are random, and
// values may be empty.
func genRows(r *rand.Rand, keyPool []string) []model.KeyValue {
	n := r.Intn(9) // 0..8 rows
	rows := make([]model.KeyValue, 0, n)
	for i := 0; i < n; i++ {
		key := ""
		if r.Intn(4) != 0 { // ~75% non-empty key
			key = keyPool[r.Intn(len(keyPool))]
		}
		value := ""
		if r.Intn(4) != 0 { // ~75% non-empty value
			value = randFrom(r, kvValueRunes, 0, 8)
		}
		rows = append(rows, model.KeyValue{
			Key:     key,
			Value:   value,
			Enabled: r.Intn(2) == 0,
		})
	}
	return rows
}

// Generate implements quick.Generator for assemblyCase.
func (assemblyCase) Generate(r *rand.Rand, size int) reflect.Value {
	return reflect.ValueOf(assemblyCase{
		method:  assemblyMethods[r.Intn(len(assemblyMethods))],
		params:  genRows(r, paramKeyPool),
		headers: genRows(r, headerKeyPool),
	})
}

// expectedParams computes the query a correct MergeParams must produce: the
// multiset of enabled, non-empty-key rows, with duplicates and order preserved.
func expectedParams(params []model.KeyValue) url.Values {
	want := url.Values{}
	for _, p := range params {
		if p.Enabled && p.Key != "" {
			want.Add(p.Key, p.Value)
		}
	}
	return want
}

// expectedHeaders computes the header map a correct BuildHeaders must produce:
// the multiset of enabled, non-empty-key rows keyed canonically (matching
// Header.Add), with duplicates and order preserved.
func expectedHeaders(headers []model.KeyValue) map[string][]string {
	want := map[string][]string{}
	for _, row := range headers {
		if row.Enabled && row.Key != "" {
			ck := http.CanonicalHeaderKey(row.Key)
			want[ck] = append(want[ck], row.Value)
		}
	}
	return want
}

func TestProp1_RequestAssembly(t *testing.T) {
	property := func(c assemblyCase) bool {
		// Assemble the request the way PrepareRequest will: start from a valid
		// base URL, merge params, build headers, and attach the method.
		u, err := url.Parse("https://example.com/path")
		if err != nil {
			t.Fatalf("base URL failed to parse: %v", err)
		}
		MergeParams(u, c.params)
		header := BuildHeaders(c.headers)

		req, err := http.NewRequest(c.method, u.String(), nil)
		if err != nil {
			t.Logf("NewRequest(%q) failed: %v", c.method, err)
			return false
		}
		req.Header = header

		// Method passes through unchanged (Req 1.2).
		if req.Method != c.method {
			t.Logf("method = %q, want %q", req.Method, c.method)
			return false
		}

		// Query holds exactly the enabled non-empty params, duplicates and all,
		// and nothing from disabled or empty-key rows (Req 1.4, 1.5).
		gotParams := req.URL.Query()
		wantParams := expectedParams(c.params)
		if !reflect.DeepEqual(map[string][]string(gotParams), map[string][]string(wantParams)) {
			t.Logf("query = %#v, want %#v (params=%#v)", gotParams, wantParams, c.params)
			return false
		}

		// An empty key must never surface in the query.
		if _, ok := gotParams[""]; ok {
			t.Logf("query contains an empty key: %#v", gotParams)
			return false
		}

		// Percent-encoding (Req 1.4): the encoded query must never contain a
		// literal space; url.Values.Encode emits spaces as '+' or %20.
		for i := 0; i < len(req.URL.RawQuery); i++ {
			if req.URL.RawQuery[i] == ' ' {
				t.Logf("RawQuery contains a literal space: %q", req.URL.RawQuery)
				return false
			}
		}

		// Headers hold exactly the enabled non-empty rows, with duplicate names
		// preserved and disabled/empty-key rows excluded (Req 1.5, 1.6).
		gotHeaders := map[string][]string(req.Header)
		wantHeaders := expectedHeaders(c.headers)
		if !reflect.DeepEqual(gotHeaders, wantHeaders) {
			t.Logf("headers = %#v, want %#v (headers=%#v)", gotHeaders, wantHeaders, c.headers)
			return false
		}

		return true
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
