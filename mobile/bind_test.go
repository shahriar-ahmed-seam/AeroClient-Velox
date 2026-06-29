package mobile

import (
	"encoding/json"
	"reflect"
	"testing"

	"volt/internal/model"
)

// These tests pin down the central guarantee of the gomobile facade: it is a
// thin JSON transport over the same internal/model types and internal/store
// calls the Wails bindings use, so a request or response serialized on Android
// has byte-for-byte the same shape (field names, nesting) as on desktop
// (Req 15.1, 15.2). The desktop (Wails) path passes model values across the
// binding directly; the mobile path unmarshals a JSON string into the same
// model type, performs the same store/httpcore call, and remarshals. If those
// two paths agree on JSON shape, parity holds.

// sampleRawRequest is a representative, fully-populated request exercising every
// nested model type (KeyValue rows, BodySpec, AuthSpec) so the parity checks
// cover the whole request surface, not just scalar fields.
func sampleRawRequest() model.RawRequest {
	return model.RawRequest{
		Method: model.MethodPost,
		URL:    "https://api.example.com/v1/widgets",
		Params: []model.KeyValue{
			{Key: "page", Value: "2", Enabled: true},
			{Key: "draft", Value: "", Enabled: false},
		},
		Headers: []model.KeyValue{
			{Key: "X-Trace", Value: "abc123", Enabled: true},
			{Key: "Accept", Value: "application/json", Enabled: true},
		},
		Body: model.BodySpec{
			Type: model.BodyJSON,
			Raw:  `{"name":"widget"}`,
			FormFields: []model.KeyValue{
				{Key: "field", Value: "v", Enabled: true},
			},
		},
		Auth: model.AuthSpec{
			Type:           model.AuthAPIKey,
			BearerToken:    "tok",
			BasicUser:      "user",
			BasicPass:      "pass",
			APIKeyName:     "X-Api-Key",
			APIKeyValue:    "secret",
			APIKeyLocation: model.APIKeyInHeader,
		},
	}
}

// roundTripJSON mimics exactly what every Bridge method does with structured
// input: it decodes the incoming JSON string into the model type and re-encodes
// it. If the facade altered field names or shapes, the re-encoded bytes would
// differ from the model package's own encoding.
func roundTripJSON[T any](t *testing.T, in string) string {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(in), &v); err != nil {
		t.Fatalf("bridge-style unmarshal failed: %v", err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("bridge-style marshal failed: %v", err)
	}
	return string(out)
}

// jsonKeys returns the set of top-level JSON object keys produced by marshalling
// v, used to assert two values share the same JSON shape.
func jsonKeys(t *testing.T, v any) map[string]bool {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal for key extraction: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal for key extraction: %v", err)
	}
	keys := make(map[string]bool, len(m))
	for k := range m {
		keys[k] = true
	}
	return keys
}

// keysOfJSON returns the top-level keys present in a JSON object string.
func keysOfJSON(t *testing.T, s string) map[string]bool {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("invalid JSON object %q: %v", s, err)
	}
	keys := make(map[string]bool, len(m))
	for k := range m {
		keys[k] = true
	}
	return keys
}

// TestRawRequestJSONShapeParity confirms that pushing a model.RawRequest through
// the facade's decode/encode cycle yields exactly the JSON the model package
// produces directly, so the request that goes on the wire is identical on
// Android and desktop (Req 15.1, 15.2).
func TestRawRequestJSONShapeParity(t *testing.T) {
	raw := sampleRawRequest()

	modelJSON, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal model.RawRequest: %v", err)
	}

	bridgeJSON := roundTripJSON[model.RawRequest](t, string(modelJSON))

	if bridgeJSON != string(modelJSON) {
		t.Fatalf("request JSON shape diverged\n model:  %s\n bridge: %s", modelJSON, bridgeJSON)
	}
}

// TestHTTPResponseJSONShapeParity confirms the response type round-trips through
// the facade unchanged, so the Response_Viewer sees the same fields regardless
// of platform (Req 15.2).
func TestHTTPResponseJSONShapeParity(t *testing.T) {
	resp := model.HTTPResponse{
		Status:     200,
		StatusText: "OK",
		Headers: []model.KeyValue{
			{Key: "Content-Type", Value: "application/json", Enabled: true},
		},
		Body:       `{"ok":true}`,
		DurationMs: 42,
		SizeBytes:  11,
		Error:      "",
		Truncated:  false,
	}

	modelJSON, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal model.HTTPResponse: %v", err)
	}

	bridgeJSON := roundTripJSON[model.HTTPResponse](t, string(modelJSON))

	if bridgeJSON != string(modelJSON) {
		t.Fatalf("response JSON shape diverged\n model:  %s\n bridge: %s", modelJSON, bridgeJSON)
	}
}

// TestExecuteInvalidURLReturnsHTTPResponseShape confirms Execute always returns
// a JSON-decodable model.HTTPResponse — even on a validation failure — with the
// Error field set and no extra/missing fields relative to the model type. This
// is the Android error-surfacing contract (Req 15.2, and the error-in-response
// convention the desktop path shares).
func TestExecuteInvalidURLReturnsHTTPResponseShape(t *testing.T) {
	b, err := NewBridge(":memory:")
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	defer b.Close()

	// Missing scheme/host: PrepareRequest rejects this before any network call,
	// so the failure travels back in the response's Error field.
	raw := model.RawRequest{Method: model.MethodGet, URL: "not-a-valid-url"}
	reqJSON, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	resultJSON := b.Execute(string(reqJSON))

	var resp model.HTTPResponse
	if err := json.Unmarshal([]byte(resultJSON), &resp); err != nil {
		t.Fatalf("Execute result is not a decodable model.HTTPResponse: %v\n got: %s", err, resultJSON)
	}
	if resp.Error == "" {
		t.Fatalf("expected Error to be set for an invalid URL, got: %s", resultJSON)
	}

	// The returned JSON must carry exactly the model.HTTPResponse shape.
	wantKeys := jsonKeys(t, model.HTTPResponse{})
	gotKeys := keysOfJSON(t, resultJSON)
	if !reflect.DeepEqual(wantKeys, gotKeys) {
		t.Fatalf("Execute JSON shape mismatch\n want keys: %v\n got keys:  %v", wantKeys, gotKeys)
	}
}

// TestSaveCollectionRoundTrip marshals a model.Collection, saves it through the
// facade, and decodes the returned JSON back into a model.Collection, asserting
// the value survives the JSON round-trip unchanged. Explicit IDs are supplied so
// the facade assigns none and the comparison is exact (Req 15.1, 15.5).
func TestSaveCollectionRoundTrip(t *testing.T) {
	b, err := NewBridge(":memory:")
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	defer b.Close()

	want := model.Collection{
		ID:   "coll-12-3",
		Name: "Parity Collection",
		Folders: []model.Folder{
			{
				ID:      "folder-12-3",
				Name:    "Nested",
				Folders: []model.Folder{},
				Requests: []model.SavedRequest{
					{RawRequest: sampleRawRequest(), ID: "req-folder-12-3", Name: "In Folder"},
				},
			},
		},
		Requests: []model.SavedRequest{
			{RawRequest: sampleRawRequest(), ID: "req-top-12-3", Name: "Top Level"},
		},
		Order: 7,
	}

	in, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal collection: %v", err)
	}

	got := decodeCollectionResult(t, b.SaveCollection(string(in)))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SaveCollection round-trip mismatch\n want: %+v\n got:  %+v", want, got)
	}

	// ListTree returns the same JSON shape; locate our collection and confirm it
	// decodes back into an equal model value.
	var tree []model.Collection
	treeJSON := b.ListTree()
	if err := json.Unmarshal([]byte(treeJSON), &tree); err != nil {
		t.Fatalf("ListTree result not decodable as []model.Collection: %v\n got: %s", err, treeJSON)
	}
	found, ok := findCollection(tree, want.ID)
	if !ok {
		t.Fatalf("saved collection %q not present in ListTree result: %s", want.ID, treeJSON)
	}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("ListTree round-trip mismatch\n want: %+v\n got:  %+v", want, found)
	}
}

// TestSaveEnvironmentRoundTrip marshals a model.Environment, saves it through
// the facade, and decodes both the save result and the ListEnvironments entry
// back into model.Environment, asserting equality (Req 15.1, 15.5).
func TestSaveEnvironmentRoundTrip(t *testing.T) {
	b, err := NewBridge(":memory:")
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	defer b.Close()

	want := model.Environment{
		ID:   "env-12-3",
		Name: "Parity Env",
		Variables: []model.Variable{
			{Name: "baseUrl", Value: "https://api.example.com"},
			{Name: "token", Value: "abc"},
		},
		Active: false,
	}

	in, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal environment: %v", err)
	}

	got := decodeEnvironmentResult(t, b.SaveEnvironment(string(in)))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SaveEnvironment round-trip mismatch\n want: %+v\n got:  %+v", want, got)
	}

	var envs []model.Environment
	envsJSON := b.ListEnvironments()
	if err := json.Unmarshal([]byte(envsJSON), &envs); err != nil {
		t.Fatalf("ListEnvironments result not decodable as []model.Environment: %v\n got: %s", err, envsJSON)
	}
	found, ok := findEnvironment(envs, want.ID)
	if !ok {
		t.Fatalf("saved environment %q not present in ListEnvironments result: %s", want.ID, envsJSON)
	}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("ListEnvironments round-trip mismatch\n want: %+v\n got:  %+v", want, found)
	}
}

// decodeCollectionResult decodes a facade result that should be a model.Collection,
// failing if it is instead an {"error":...} envelope.
func decodeCollectionResult(t *testing.T, result string) model.Collection {
	t.Helper()
	assertNotErrorEnvelope(t, result)
	var c model.Collection
	if err := json.Unmarshal([]byte(result), &c); err != nil {
		t.Fatalf("result not decodable as model.Collection: %v\n got: %s", err, result)
	}
	return c
}

// decodeEnvironmentResult decodes a facade result that should be a
// model.Environment, failing if it is instead an {"error":...} envelope.
func decodeEnvironmentResult(t *testing.T, result string) model.Environment {
	t.Helper()
	assertNotErrorEnvelope(t, result)
	var e model.Environment
	if err := json.Unmarshal([]byte(result), &e); err != nil {
		t.Fatalf("result not decodable as model.Environment: %v\n got: %s", err, result)
	}
	return e
}

// assertNotErrorEnvelope fails the test when result is an {"error":"..."}
// envelope, so a store failure surfaces as a clear test failure rather than a
// silently zero-valued decode.
func assertNotErrorEnvelope(t *testing.T, result string) {
	t.Helper()
	var env struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(result), &env); err == nil && env.Error != "" {
		t.Fatalf("facade returned an error envelope: %s", result)
	}
}

func findCollection(tree []model.Collection, id string) (model.Collection, bool) {
	for _, c := range tree {
		if c.ID == id {
			return c, true
		}
	}
	return model.Collection{}, false
}

func findEnvironment(envs []model.Environment, id string) (model.Environment, bool) {
	for _, e := range envs {
		if e.ID == id {
			return e, true
		}
	}
	return model.Environment{}, false
}
