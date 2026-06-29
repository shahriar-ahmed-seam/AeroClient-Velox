package httpcore

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"volt/internal/model"
)

// Validation rule identifiers carried by a ValidationError. They name the
// specific URL rule that failed so the frontend can render a rule-specific
// message and tests can assert on the exact cause (Req 1.3).
const (
	// RuleEmptyURL indicates the URL was empty (or whitespace-only).
	RuleEmptyURL = "empty_url"
	// RuleMissingScheme indicates the URL omitted a scheme (e.g. "api.example.com").
	RuleMissingScheme = "missing_scheme"
	// RuleUnsupportedScheme indicates the URL used a scheme other than http or https.
	RuleUnsupportedScheme = "unsupported_scheme"
	// RuleMissingHost indicates the URL omitted a host.
	RuleMissingHost = "missing_host"
	// RuleMalformedURL indicates the URL could not be parsed at all.
	RuleMalformedURL = "malformed_url"
)

// ValidationError is returned by request-preparation logic when a request is
// rejected before any network call is made. Rule names the failed validation
// rule (one of the Rule* constants) and Message is a human-readable
// explanation. It is defined here and reused by later preparation steps
// (e.g. PrepareRequest), per Requirement 1.3.
type ValidationError struct {
	// Rule is the identifier of the validation rule that failed.
	Rule string
	// Message is a human-readable description of the failure.
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Rule, e.Message)
}

// newValidationError constructs a *ValidationError for the given rule.
func newValidationError(rule, message string) *ValidationError {
	return &ValidationError{Rule: rule, Message: message}
}

// ValidateURL validates rawURL against Requirement 1.3 and returns the parsed
// URL on success. It rejects, in order, an empty URL, a malformed URL, a URL
// with no scheme, a URL whose scheme is neither http nor https, and a URL with
// no host. On any failure it returns a *ValidationError naming the failed rule
// and a nil *url.URL, and the caller must NOT perform a network call.
func ValidateURL(rawURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, newValidationError(RuleEmptyURL, "URL must not be empty")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, newValidationError(RuleMalformedURL, "URL could not be parsed: "+err.Error())
	}

	if parsed.Scheme == "" {
		return nil, newValidationError(RuleMissingScheme, "URL must include a scheme, e.g. https://")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, newValidationError(RuleUnsupportedScheme, "URL scheme must be http or https, got "+parsed.Scheme)
	}

	if parsed.Host == "" {
		return nil, newValidationError(RuleMissingHost, "URL must include a host")
	}

	return parsed, nil
}

// MergeParams appends every enabled, non-empty-key query parameter from params
// onto u's existing query string as a percent-encoded name-value pair, and
// excludes disabled rows and rows with an empty key (Req 1.4, 1.5). Parameter
// values may be empty. Rows sharing a key are all preserved. The merged query
// is written back to u.RawQuery in percent-encoded form.
func MergeParams(u *url.URL, params []model.KeyValue) {
	if u == nil {
		return
	}
	q := u.Query()
	for _, p := range params {
		if p.Enabled && p.Key != "" {
			q.Add(p.Key, p.Value)
		}
	}
	u.RawQuery = q.Encode()
}

// BuildHeaders constructs an http.Header from the enabled, non-empty-key header
// rows, excluding disabled rows and rows with an empty key (Req 1.5, 1.6).
// Multiple rows that share a header name are all preserved via Header.Add, so
// duplicate header names survive assembly.
func BuildHeaders(headers []model.KeyValue) http.Header {
	h := make(http.Header)
	for _, row := range headers {
		if row.Enabled && row.Key != "" {
			h.Add(row.Key, row.Value)
		}
	}
	return h
}
