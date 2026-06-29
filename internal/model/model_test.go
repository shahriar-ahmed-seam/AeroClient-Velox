package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// assertFieldNames marshals v and fails if any of the wanted JSON keys is
// missing from the top-level object.
func assertFieldNames(t *testing.T, v any, want ...string) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal %T into map: %v (json=%s)", v, err, data)
	}
	for _, key := range want {
		if _, ok := obj[key]; !ok {
			t.Errorf("%T JSON %s missing expected field %q", v, data, key)
		}
	}
}

// roundTrip marshals src, unmarshals into a fresh value of the same type, and
// asserts deep equality.
func roundTrip[T any](t *testing.T, src T) {
	t.Helper()
	data, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal %T: %v", src, err)
	}
	var dst T
	if err := json.Unmarshal(data, &dst); err != nil {
		t.Fatalf("unmarshal %T: %v (json=%s)", src, err, data)
	}
	if !reflect.DeepEqual(src, dst) {
		t.Errorf("round-trip mismatch for %T:\n  src=%+v\n  dst=%+v\n  json=%s", src, src, dst, data)
	}
}

func sampleKeyValue() KeyValue {
	return KeyValue{Key: "Authorization", Value: "Bearer abc", Enabled: true}
}

func sampleBodySpec() BodySpec {
	return BodySpec{
		Type: BodyURLEncoded,
		Raw:  `{"hello":"world"}`,
		FormFields: []KeyValue{
			{Key: "a", Value: "1", Enabled: true},
			{Key: "b", Value: "2", Enabled: false},
		},
	}
}

func sampleAuthSpec() AuthSpec {
	return AuthSpec{
		Type:           AuthAPIKey,
		BearerToken:    "tok",
		BasicUser:      "user",
		BasicPass:      "pass",
		APIKeyName:     "X-Api-Key",
		APIKeyValue:    "secret",
		APIKeyLocation: APIKeyInHeader,
	}
}

func sampleRawRequest() RawRequest {
	return RawRequest{
		Method: MethodPost,
		URL:    "https://example.com/api?x=1",
		Params: []KeyValue{{Key: "x", Value: "1", Enabled: true}},
		Headers: []KeyValue{
			{Key: "Accept", Value: "application/json", Enabled: true},
			{Key: "Accept", Value: "text/plain", Enabled: true},
		},
		Body: sampleBodySpec(),
		Auth: sampleAuthSpec(),
	}
}

func sampleSavedRequest() SavedRequest {
	return SavedRequest{
		RawRequest: sampleRawRequest(),
		ID:         "req-1",
		Name:       "Create user",
	}
}

func sampleFolder() Folder {
	return Folder{
		ID:       "folder-1",
		Name:     "Users",
		Folders:  []Folder{{ID: "folder-2", Name: "Admin", Folders: []Folder{}, Requests: []SavedRequest{}}},
		Requests: []SavedRequest{sampleSavedRequest()},
	}
}

func sampleCollection() Collection {
	return Collection{
		ID:       "col-1",
		Name:     "My API",
		Folders:  []Folder{sampleFolder()},
		Requests: []SavedRequest{sampleSavedRequest()},
		Order:    3,
	}
}

func sampleEnvironment() Environment {
	return Environment{
		ID:        "env-1",
		Name:      "Production",
		Variables: []Variable{{Name: "base", Value: "https://api.example.com"}},
		Active:    true,
	}
}

func sampleHistoryEntry() HistoryEntry {
	return HistoryEntry{
		ID:         "h-1",
		Method:     MethodGet,
		URL:        "https://example.com",
		Status:     200,
		DurationMs: 123,
		At:         1700000000,
		Error:      "",
		Request:    sampleRawRequest(),
	}
}

func sampleSettings() Settings {
	return Settings{
		Theme:          "dark",
		TLSVerify:      true,
		TimeoutSeconds: 30,
		ProxyURL:       "http://localhost:8080",
	}
}

func sampleHTTPResponse() HTTPResponse {
	return HTTPResponse{
		Status:     200,
		StatusText: "OK",
		Headers:    []KeyValue{{Key: "Content-Type", Value: "application/json", Enabled: true}},
		Body:       `{"ok":true}`,
		DurationMs: 42,
		SizeBytes:  11,
		Error:      "",
		Truncated:  false,
	}
}

func TestKeyValueJSON(t *testing.T) {
	v := sampleKeyValue()
	assertFieldNames(t, v, "key", "value", "enabled")
	roundTrip(t, v)
}

func TestBodySpecJSON(t *testing.T) {
	v := sampleBodySpec()
	assertFieldNames(t, v, "type", "raw", "formFields")
	roundTrip(t, v)
}

func TestAuthSpecJSON(t *testing.T) {
	v := sampleAuthSpec()
	assertFieldNames(t, v, "type", "bearerToken", "basicUser", "basicPass", "apiKeyName", "apiKeyValue", "apiKeyLocation")
	roundTrip(t, v)
}

func TestRawRequestJSON(t *testing.T) {
	v := sampleRawRequest()
	assertFieldNames(t, v, "method", "url", "params", "headers", "body", "auth")
	roundTrip(t, v)
}

func TestRequestAliasJSON(t *testing.T) {
	// Request is a type alias for RawRequest and must serialize identically.
	var v Request = sampleRawRequest()
	assertFieldNames(t, v, "method", "url", "params", "headers", "body", "auth")
	roundTrip(t, v)
}

func TestSavedRequestJSON(t *testing.T) {
	v := sampleSavedRequest()
	// Embedded RawRequest fields are promoted to the top level alongside id/name.
	assertFieldNames(t, v, "id", "name", "method", "url", "params", "headers", "body", "auth")
	roundTrip(t, v)
}

func TestFolderJSON(t *testing.T) {
	v := sampleFolder()
	assertFieldNames(t, v, "id", "name", "folders", "requests")
	roundTrip(t, v)
}

func TestCollectionJSON(t *testing.T) {
	v := sampleCollection()
	assertFieldNames(t, v, "id", "name", "folders", "requests", "order")
	roundTrip(t, v)
}

func TestVariableJSON(t *testing.T) {
	v := Variable{Name: "token", Value: "xyz"}
	assertFieldNames(t, v, "name", "value")
	roundTrip(t, v)
}

func TestEnvironmentJSON(t *testing.T) {
	v := sampleEnvironment()
	assertFieldNames(t, v, "id", "name", "variables", "active")
	roundTrip(t, v)
}

func TestHistoryEntryJSON(t *testing.T) {
	v := sampleHistoryEntry()
	assertFieldNames(t, v, "id", "method", "url", "status", "durationMs", "at", "error", "request")
	roundTrip(t, v)
}

func TestSettingsJSON(t *testing.T) {
	v := sampleSettings()
	assertFieldNames(t, v, "theme", "tlsVerify", "timeoutSeconds", "proxyUrl")
	roundTrip(t, v)
}

func TestHTTPResponseJSON(t *testing.T) {
	v := sampleHTTPResponse()
	assertFieldNames(t, v, "status", "statusText", "headers", "body", "durationMs", "sizeBytes", "error", "truncated")
	roundTrip(t, v)
}

// TestExpectedFieldNamesMatchFrontend pins the exact JSON field-name spelling so
// drift away from the TypeScript data models (camelCase) is caught.
func TestExpectedFieldNamesMatchFrontend(t *testing.T) {
	data, err := json.Marshal(sampleRawRequest())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	js := string(data)
	for _, camel := range []string{`"method"`, `"url"`, `"params"`, `"headers"`, `"body"`, `"auth"`} {
		if !strings.Contains(js, camel) {
			t.Errorf("expected %s in RawRequest JSON, got %s", camel, js)
		}
	}
}
