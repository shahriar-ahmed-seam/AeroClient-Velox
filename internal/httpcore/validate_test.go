package httpcore

import (
	"errors"
	"net/url"
	"reflect"
	"sort"
	"testing"

	"volt/internal/model"
)

func TestValidateURL_Rejects(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		wantRule string
	}{
		{"empty", "", RuleEmptyURL},
		{"whitespace only", "   ", RuleEmptyURL},
		{"missing scheme", "api.example.com/path", RuleMissingScheme},
		{"non-http(s) scheme ftp", "ftp://files.example.com", RuleUnsupportedScheme},
		{"non-http(s) scheme file", "file:///etc/passwd", RuleUnsupportedScheme},
		{"missing host", "http://", RuleMissingHost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateURL(tt.rawURL)
			if err == nil {
				t.Fatalf("ValidateURL(%q) = %v, want error", tt.rawURL, got)
			}
			if got != nil {
				t.Errorf("ValidateURL(%q) returned non-nil URL %v on failure", tt.rawURL, got)
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("ValidateURL(%q) error %T, want *ValidationError", tt.rawURL, err)
			}
			if ve.Rule != tt.wantRule {
				t.Errorf("ValidateURL(%q) rule = %q, want %q", tt.rawURL, ve.Rule, tt.wantRule)
			}
		})
	}
}

func TestValidateURL_Accepts(t *testing.T) {
	tests := []string{
		"http://example.com",
		"https://api.example.com/v1/users?x=1",
		"HTTPS://Example.com", // scheme case-insensitive
		"http://localhost:8080/path",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			got, err := ValidateURL(raw)
			if err != nil {
				t.Fatalf("ValidateURL(%q) unexpected error: %v", raw, err)
			}
			if got == nil {
				t.Fatalf("ValidateURL(%q) returned nil URL", raw)
			}
		})
	}
}

func TestMergeParams(t *testing.T) {
	u, err := ValidateURL("https://example.com/path?existing=1")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	params := []model.KeyValue{
		{Key: "a", Value: "1", Enabled: true},
		{Key: "b", Value: "two words", Enabled: true}, // needs percent-encoding
		{Key: "disabled", Value: "x", Enabled: false}, // excluded
		{Key: "", Value: "noKey", Enabled: true},      // excluded (empty key)
		{Key: "empty", Value: "", Enabled: true},      // kept, value empty
	}
	MergeParams(u, params)

	q := u.Query()
	if got := q.Get("a"); got != "1" {
		t.Errorf("param a = %q, want 1", got)
	}
	if got := q.Get("b"); got != "two words" {
		t.Errorf("param b = %q, want 'two words'", got)
	}
	if _, ok := q["disabled"]; ok {
		t.Errorf("disabled param should be excluded")
	}
	if _, ok := q[""]; ok {
		t.Errorf("empty-key param should be excluded")
	}
	if vals, ok := q["empty"]; !ok || len(vals) != 1 || vals[0] != "" {
		t.Errorf("empty-value param should be present with empty value, got %v", vals)
	}
	if got := q.Get("existing"); got != "1" {
		t.Errorf("existing query param should be preserved, got %q", got)
	}

	// RawQuery must be percent-encoded.
	parsedBack, perr := url.ParseQuery(u.RawQuery)
	if perr != nil {
		t.Fatalf("RawQuery not valid percent-encoding: %v", perr)
	}
	if parsedBack.Get("b") != "two words" {
		t.Errorf("percent-encoded round-trip for b failed: %q", parsedBack.Get("b"))
	}
}

func TestMergeParams_DuplicateKeys(t *testing.T) {
	u, _ := ValidateURL("https://example.com")
	params := []model.KeyValue{
		{Key: "tag", Value: "go", Enabled: true},
		{Key: "tag", Value: "test", Enabled: true},
	}
	MergeParams(u, params)
	vals := u.Query()["tag"]
	sort.Strings(vals)
	want := []string{"go", "test"}
	if !reflect.DeepEqual(vals, want) {
		t.Errorf("duplicate query keys = %v, want %v", vals, want)
	}
}

func TestBuildHeaders(t *testing.T) {
	headers := []model.KeyValue{
		{Key: "Accept", Value: "application/json", Enabled: true},
		{Key: "X-Disabled", Value: "no", Enabled: false}, // excluded
		{Key: "", Value: "noKey", Enabled: true},         // excluded
		{Key: "X-Multi", Value: "first", Enabled: true},
		{Key: "X-Multi", Value: "second", Enabled: true}, // duplicate preserved
	}
	h := BuildHeaders(headers)

	if got := h.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q, want application/json", got)
	}
	if _, ok := h["X-Disabled"]; ok {
		t.Errorf("disabled header should be excluded")
	}
	if _, ok := h[""]; ok {
		t.Errorf("empty-key header should be excluded")
	}
	multi := h.Values("X-Multi")
	want := []string{"first", "second"}
	if !reflect.DeepEqual(multi, want) {
		t.Errorf("duplicate header X-Multi = %v, want %v", multi, want)
	}
}
