package httpcore

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"volt/internal/model"
)

// MaxResponseBodyBytes is the hard cap on how many response-body bytes are
// retained and displayed. A response body larger than this is truncated to
// exactly the first MaxResponseBodyBytes bytes and flagged as truncated
// (Req 4.11). The value is 5 MB (5 * 1024 * 1024).
const MaxResponseBodyBytes = 5_242_880

// Doer is the minimal HTTP-execution dependency Execute relies on. It is
// satisfied by *http.Client, and being an interface it can be replaced by a
// mock in tests so the engine's execution logic is exercised without real
// network I/O. The network call is the only side effect in httpcore, and it is
// isolated entirely behind this interface.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// NewDoer builds the default Doer used in production from the user settings. It
// wires the TLS-verification setting into the transport: when s.TLSVerify is
// false the transport is configured to skip TLS certificate verification
// (Req 9.5). The per-request timeout is applied by Execute through a context
// deadline derived from s.TimeoutSeconds rather than on the client, so a single
// configured client can serve many requests.
func NewDoer(s model.Settings) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		// InsecureSkipVerify is the inverse of the user's TLS-verify setting:
		// disabling verification skips certificate checks (Req 9.5).
		InsecureSkipVerify: !s.TLSVerify,
	}
	return &http.Client{Transport: transport}
}

// Execute sends a PreparedRequest using the injected Doer and returns the
// resulting HTTPResponse. It performs the only network I/O in the package.
//
// Behavior:
//   - A per-request deadline is derived from s.TimeoutSeconds via
//     context.WithTimeout; exceeding it yields a distinct timeout error
//     indication (Req 9.3, 9.5).
//   - The TLS-verification setting is honored through the Doer's transport
//     configuration. When doer is nil a default client is built with NewDoer(s)
//     so the TLS-skip setting still applies (Req 9.5).
//   - Network and timeout errors are captured into HTTPResponse.Error with
//     Status, DurationMs, and SizeBytes left at zero so the viewer renders only
//     the error (Req 4.9).
//   - A response body larger than MaxResponseBodyBytes is truncated to exactly
//     the first MaxResponseBodyBytes bytes and Truncated is set (Req 4.11).
func Execute(ctx context.Context, doer Doer, pr PreparedRequest, s model.Settings) model.HTTPResponse {
	if ctx == nil {
		ctx = context.Background()
	}
	if doer == nil {
		doer = NewDoer(s)
	}

	// Apply the configured timeout as a context deadline (Req 9.3).
	timeout := time.Duration(s.TimeoutSeconds) * time.Second
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Build the *http.Request from the PreparedRequest. A nil body is used when
	// there are no body bytes so methods without a body send none.
	var bodyReader io.Reader
	if len(pr.Body) > 0 {
		bodyReader = bytes.NewReader(pr.Body)
	}
	req, err := http.NewRequestWithContext(ctx, pr.Method, pr.URL, bodyReader)
	if err != nil {
		// A malformed request that the validated PreparedRequest still could not
		// turn into an *http.Request is reported as an error with zeroed fields.
		return model.HTTPResponse{Error: err.Error()}
	}

	// Apply the assembled headers and the resolved Content-Type.
	if pr.Header != nil {
		req.Header = pr.Header.Clone()
	}
	if pr.ContentType != "" {
		req.Header.Set(HeaderContentType, pr.ContentType)
	}

	start := time.Now()
	resp, err := doer.Do(req)
	if err != nil {
		// Network/transport/timeout failures: zero status, duration, and size so
		// the viewer shows only the error (Req 4.9). Timeouts get a distinct
		// indication (Req 9.5).
		return model.HTTPResponse{Error: executionErrorMessage(ctx, err, s.TimeoutSeconds)}
	}
	defer resp.Body.Close()

	durationMs := time.Since(start).Milliseconds()

	body, truncated, readErr := readBody(resp.Body)
	if readErr != nil {
		// A read failure after a response was received keeps the status already
		// received and reports the read error.
		return model.HTTPResponse{
			Status:     resp.StatusCode,
			StatusText: statusText(resp),
			Headers:    convertHeaders(resp.Header),
			DurationMs: durationMs,
			Error:      readErr.Error(),
		}
	}

	return model.HTTPResponse{
		Status:     resp.StatusCode,
		StatusText: statusText(resp),
		Headers:    convertHeaders(resp.Header),
		Body:       string(body),
		DurationMs: durationMs,
		SizeBytes:  len(body),
		Truncated:  truncated,
	}
}

// readBody reads up to MaxResponseBodyBytes+1 bytes from r so truncation can be
// detected without buffering an unbounded body. When the body exceeds
// MaxResponseBodyBytes the returned bytes are exactly the first
// MaxResponseBodyBytes and truncated is true; otherwise the full body is
// returned with truncated false (Req 4.11).
func readBody(r io.Reader) (body []byte, truncated bool, err error) {
	limited := io.LimitReader(r, MaxResponseBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if len(data) > MaxResponseBodyBytes {
		return data[:MaxResponseBodyBytes], true, nil
	}
	return data, false, nil
}

// executionErrorMessage produces the error message for a failed Do, giving a
// distinct timeout indication when the context deadline was exceeded (Req 9.5).
func executionErrorMessage(ctx context.Context, err error, timeoutSeconds int) string {
	if errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
		if timeoutSeconds > 0 {
			return "request timed out after " + strconv.Itoa(timeoutSeconds) + "s"
		}
		return "request timed out"
	}
	return err.Error()
}

// statusText returns the human-readable status text for resp, e.g. "OK" for a
// "200 OK" status line, falling back to the standard text for the status code.
func statusText(resp *http.Response) string {
	if resp.Status != "" {
		// resp.Status is typically "200 OK"; drop the leading numeric code.
		if idx := strings.IndexByte(resp.Status, ' '); idx >= 0 {
			return resp.Status[idx+1:]
		}
		return resp.Status
	}
	return http.StatusText(resp.StatusCode)
}

// convertHeaders flattens an http.Header into a slice of enabled KeyValue
// pairs, emitting one entry per header value so multi-valued headers are
// preserved.
func convertHeaders(h http.Header) []model.KeyValue {
	if len(h) == 0 {
		return nil
	}
	out := make([]model.KeyValue, 0, len(h))
	for name, values := range h {
		for _, value := range values {
			out = append(out, model.KeyValue{Key: name, Value: value, Enabled: true})
		}
	}
	return out
}
