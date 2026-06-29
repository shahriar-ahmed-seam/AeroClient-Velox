package httpcore

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"

	"volt/internal/model"
)

func TestDeriveAuthHeader_Bearer(t *testing.T) {
	t.Run("non-whitespace token yields Authorization Bearer", func(t *testing.T) {
		name, value, qk, qv := DeriveAuthHeader(model.AuthSpec{
			Type:        model.AuthBearer,
			BearerToken: "abc123",
		})
		if name != HeaderAuthorization || value != "Bearer abc123" {
			t.Fatalf("got (%q, %q), want (Authorization, Bearer abc123)", name, value)
		}
		if qk != "" || qv != "" {
			t.Fatalf("expected no query pair, got (%q, %q)", qk, qv)
		}
	})

	t.Run("empty token yields no header", func(t *testing.T) {
		name, value, _, _ := DeriveAuthHeader(model.AuthSpec{Type: model.AuthBearer, BearerToken: ""})
		if name != "" || value != "" {
			t.Fatalf("expected no header, got (%q, %q)", name, value)
		}
	})

	t.Run("whitespace-only token yields no header", func(t *testing.T) {
		name, value, _, _ := DeriveAuthHeader(model.AuthSpec{Type: model.AuthBearer, BearerToken: "   \t\n"})
		if name != "" || value != "" {
			t.Fatalf("expected no header, got (%q, %q)", name, value)
		}
	})
}

func TestDeriveAuthHeader_Basic(t *testing.T) {
	t.Run("user and pass base64 of user:pass", func(t *testing.T) {
		name, value, _, _ := DeriveAuthHeader(model.AuthSpec{
			Type:      model.AuthBasic,
			BasicUser: "alice",
			BasicPass: "s3cret",
		})
		if name != HeaderAuthorization {
			t.Fatalf("header name = %q, want Authorization", name)
		}
		const prefix = "Basic "
		if len(value) <= len(prefix) || value[:len(prefix)] != prefix {
			t.Fatalf("value %q missing Basic prefix", value)
		}
		decoded, err := base64.StdEncoding.DecodeString(value[len(prefix):])
		if err != nil {
			t.Fatalf("base64 decode failed: %v", err)
		}
		if string(decoded) != "alice:s3cret" {
			t.Fatalf("decoded = %q, want alice:s3cret", decoded)
		}
	})

	t.Run("missing password treated as empty", func(t *testing.T) {
		_, value, _, _ := DeriveAuthHeader(model.AuthSpec{
			Type:      model.AuthBasic,
			BasicUser: "bob",
		})
		decoded, err := base64.StdEncoding.DecodeString(value[len("Basic "):])
		if err != nil {
			t.Fatalf("base64 decode failed: %v", err)
		}
		if string(decoded) != "bob:" {
			t.Fatalf("decoded = %q, want bob:", decoded)
		}
	})

	t.Run("whitespace-only username yields no header", func(t *testing.T) {
		name, value, _, _ := DeriveAuthHeader(model.AuthSpec{Type: model.AuthBasic, BasicUser: "  ", BasicPass: "x"})
		if name != "" || value != "" {
			t.Fatalf("expected no header, got (%q, %q)", name, value)
		}
	})
}

func TestDeriveAuthHeader_APIKey(t *testing.T) {
	t.Run("header location yields request header", func(t *testing.T) {
		name, value, qk, qv := DeriveAuthHeader(model.AuthSpec{
			Type:           model.AuthAPIKey,
			APIKeyName:     "X-API-Key",
			APIKeyValue:    "tok",
			APIKeyLocation: model.APIKeyInHeader,
		})
		if name != "X-API-Key" || value != "tok" {
			t.Fatalf("got header (%q, %q), want (X-API-Key, tok)", name, value)
		}
		if qk != "" || qv != "" {
			t.Fatalf("expected no query pair, got (%q, %q)", qk, qv)
		}
	})

	t.Run("missing location defaults to header", func(t *testing.T) {
		name, value, _, _ := DeriveAuthHeader(model.AuthSpec{
			Type:        model.AuthAPIKey,
			APIKeyName:  "X-Key",
			APIKeyValue: "v",
		})
		if name != "X-Key" || value != "v" {
			t.Fatalf("got header (%q, %q), want (X-Key, v)", name, value)
		}
	})

	t.Run("query location yields query pair", func(t *testing.T) {
		name, value, qk, qv := DeriveAuthHeader(model.AuthSpec{
			Type:           model.AuthAPIKey,
			APIKeyName:     "api_key",
			APIKeyValue:    "tok",
			APIKeyLocation: model.APIKeyInQuery,
		})
		if name != "" || value != "" {
			t.Fatalf("expected no header, got (%q, %q)", name, value)
		}
		if qk != "api_key" || qv != "tok" {
			t.Fatalf("got query (%q, %q), want (api_key, tok)", qk, qv)
		}
	})

	t.Run("whitespace-only name adds nothing", func(t *testing.T) {
		name, value, qk, qv := DeriveAuthHeader(model.AuthSpec{
			Type:           model.AuthAPIKey,
			APIKeyName:     "   ",
			APIKeyValue:    "tok",
			APIKeyLocation: model.APIKeyInQuery,
		})
		if name != "" || value != "" || qk != "" || qv != "" {
			t.Fatalf("expected nothing, got header (%q, %q) query (%q, %q)", name, value, qk, qv)
		}
	})
}

func TestDeriveAuthHeader_None(t *testing.T) {
	name, value, qk, qv := DeriveAuthHeader(model.AuthSpec{Type: model.AuthNone})
	if name != "" || value != "" || qk != "" || qv != "" {
		t.Fatalf("expected nothing for None, got header (%q, %q) query (%q, %q)", name, value, qk, qv)
	}
}

func TestApplyAuth_OverridesHeadersTableAuthorization(t *testing.T) {
	h := http.Header{}
	h.Add(HeaderAuthorization, "Bearer stale-from-headers-table")

	ApplyAuth(h, nil, model.AuthSpec{Type: model.AuthBearer, BearerToken: "fresh"})

	got := h.Values(HeaderAuthorization)
	if len(got) != 1 {
		t.Fatalf("expected exactly one Authorization header, got %v", got)
	}
	if got[0] != "Bearer fresh" {
		t.Fatalf("Authorization = %q, want Bearer fresh", got[0])
	}
}

func TestApplyAuth_AppendsQueryPair(t *testing.T) {
	u, _ := url.Parse("https://example.com/path?existing=1")
	h := http.Header{}

	ApplyAuth(h, u, model.AuthSpec{
		Type:           model.AuthAPIKey,
		APIKeyName:     "api_key",
		APIKeyValue:    "tok",
		APIKeyLocation: model.APIKeyInQuery,
	})

	q := u.Query()
	if q.Get("existing") != "1" {
		t.Fatalf("existing query param lost: %q", u.RawQuery)
	}
	if q.Get("api_key") != "tok" {
		t.Fatalf("api_key not appended: %q", u.RawQuery)
	}
}

func TestApplyAuth_NoneLeavesRequestUntouched(t *testing.T) {
	u, _ := url.Parse("https://example.com/?a=1")
	h := http.Header{}
	h.Add(HeaderAuthorization, "Bearer keep-me")

	ApplyAuth(h, u, model.AuthSpec{Type: model.AuthNone})

	if got := h.Get(HeaderAuthorization); got != "Bearer keep-me" {
		t.Fatalf("None should not touch headers, Authorization = %q", got)
	}
	if u.RawQuery != "a=1" {
		t.Fatalf("None should not touch query, got %q", u.RawQuery)
	}
}
