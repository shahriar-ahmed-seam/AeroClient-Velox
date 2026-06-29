package httpcore

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"volt/internal/model"
)

// mockDoer is a configurable Doer used to exercise Execute's error, timeout,
// and truncation paths without performing real network I/O. Exactly one of the
// behavior fields is used per test.
type mockDoer struct {
	// resp/err are returned directly when respectContext is false.
	resp *http.Response
	err  error
	// respectContext makes Do block until the request context is cancelled
	// (e.g. by the timeout deadline) and then return the context error,
	// emulating a slow upstream that exceeds the configured timeout.
	respectContext bool
	// called records whether Do was invoked.
	called bool
}

func (m *mockDoer) Do(req *http.Request) (*http.Response, error) {
	m.called = true
	if m.respectContext {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}
	return m.resp, m.err
}

// newResponse builds a minimal 200 OK *http.Response whose body is the given
// bytes, suitable for feeding to Execute's body reader.
func newResponse(body []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
}

func basePrepared() PreparedRequest {
	return PreparedRequest{
		Method: http.MethodGet,
		URL:    "http://example.com",
		Header: http.Header{},
	}
}

// Req 4.9: a network/transport error must surface in HTTPResponse.Error with
// Status, DurationMs, and SizeBytes left at zero so the viewer shows only the
// error.
func TestExecute_NetworkError(t *testing.T) {
	wantErr := errors.New("dial tcp: connection refused")
	doer := &mockDoer{err: wantErr}

	s := model.Settings{TLSVerify: true, TimeoutSeconds: 30}
	got := Execute(context.Background(), doer, basePrepared(), s)

	if !doer.called {
		t.Fatalf("expected Doer.Do to be called")
	}
	if got.Error != wantErr.Error() {
		t.Errorf("Error = %q, want %q", got.Error, wantErr.Error())
	}
	if got.Status != 0 {
		t.Errorf("Status = %d, want 0 on network error", got.Status)
	}
	if got.DurationMs != 0 {
		t.Errorf("DurationMs = %d, want 0 on network error", got.DurationMs)
	}
	if got.SizeBytes != 0 {
		t.Errorf("SizeBytes = %d, want 0 on network error", got.SizeBytes)
	}
	if got.Truncated {
		t.Errorf("Truncated = true, want false on network error")
	}
	if got.Body != "" {
		t.Errorf("Body = %q, want empty on network error", got.Body)
	}
}

// Req 9.5: when a Request exceeds the configured timeout, Execute must abort and
// return a distinct timeout error indication with a zeroed status. The mock
// Doer blocks until the context deadline (derived from a 1s Settings timeout)
// fires.
func TestExecute_Timeout(t *testing.T) {
	doer := &mockDoer{respectContext: true}

	s := model.Settings{TLSVerify: true, TimeoutSeconds: 1}
	start := time.Now()
	got := Execute(context.Background(), doer, basePrepared(), s)
	elapsed := time.Since(start)

	if !doer.called {
		t.Fatalf("expected Doer.Do to be called")
	}
	// The error must indicate a timeout distinctly (not a generic message).
	if !strings.Contains(got.Error, "timed out") {
		t.Errorf("Error = %q, want a timeout indication containing 'timed out'", got.Error)
	}
	if !strings.Contains(got.Error, "1s") {
		t.Errorf("Error = %q, want the configured 1s timeout reflected", got.Error)
	}
	if got.Status != 0 {
		t.Errorf("Status = %d, want 0 on timeout", got.Status)
	}
	if got.DurationMs != 0 {
		t.Errorf("DurationMs = %d, want 0 on timeout", got.DurationMs)
	}
	if got.SizeBytes != 0 {
		t.Errorf("SizeBytes = %d, want 0 on timeout", got.SizeBytes)
	}
	// Sanity: the call should have blocked roughly until the deadline and not
	// returned essentially instantly, confirming the context wired the timeout.
	if elapsed < 500*time.Millisecond {
		t.Errorf("Execute returned after %v, expected it to block until the ~1s deadline", elapsed)
	}
}

// Req 4.11: a response body larger than MaxResponseBodyBytes (5,242,880) must be
// truncated to exactly the first MaxResponseBodyBytes bytes with Truncated set;
// bodies at or under the limit must not be truncated.
func TestExecute_BodyTruncation(t *testing.T) {
	tests := []struct {
		name          string
		size          int
		wantTruncated bool
		wantLen       int
	}{
		{"over limit", MaxResponseBodyBytes + 1024, true, MaxResponseBodyBytes},
		{"way over limit", MaxResponseBodyBytes * 2, true, MaxResponseBodyBytes},
		{"exactly at limit", MaxResponseBodyBytes, false, MaxResponseBodyBytes},
		{"under limit", MaxResponseBodyBytes - 1, false, MaxResponseBodyBytes - 1},
	}

	s := model.Settings{TLSVerify: true, TimeoutSeconds: 30}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := make([]byte, tt.size)
			for i := range body {
				body[i] = 'a'
			}
			doer := &mockDoer{resp: newResponse(body)}

			got := Execute(context.Background(), doer, basePrepared(), s)

			if got.Error != "" {
				t.Fatalf("unexpected Error: %q", got.Error)
			}
			if got.Status != http.StatusOK {
				t.Errorf("Status = %d, want %d", got.Status, http.StatusOK)
			}
			if got.Truncated != tt.wantTruncated {
				t.Errorf("Truncated = %v, want %v", got.Truncated, tt.wantTruncated)
			}
			if len(got.Body) != tt.wantLen {
				t.Errorf("len(Body) = %d, want %d", len(got.Body), tt.wantLen)
			}
			if got.SizeBytes != tt.wantLen {
				t.Errorf("SizeBytes = %d, want %d", got.SizeBytes, tt.wantLen)
			}
		})
	}
}
