package httpcore

import (
	"context"
	"io"
	"math/rand"
	"net/http"
	"reflect"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// patternByte is the deterministic byte expected at position i of a generated
// response body. Using a position-dependent pattern (rather than a constant
// fill) means a truncation that retained the wrong window of bytes — not the
// leading prefix — would be detected by the body comparison below.
func patternByte(i int) byte { return byte(i % 251) }

// patternReader streams exactly n bytes following patternByte without ever
// materializing the full body in memory at the source. Only Execute's internal
// LimitReader buffer (capped at MaxResponseBodyBytes+1) is allocated, keeping
// per-iteration memory bounded even for sizes well above the 5 MB boundary.
type patternReader struct {
	remaining int
	pos       int
}

func newPatternReader(n int) *patternReader { return &patternReader{remaining: n} }

func (p *patternReader) Read(buf []byte) (int, error) {
	if p.remaining == 0 {
		return 0, io.EOF
	}
	n := len(buf)
	if n > p.remaining {
		n = p.remaining
	}
	for i := 0; i < n; i++ {
		buf[i] = patternByte(p.pos)
		p.pos++
	}
	p.remaining -= n
	return n, nil
}

// bodySize is a testing/quick generator whose distribution is biased around the
// 5 MB truncation boundary so the property exercises sizes below, exactly at,
// and above MaxResponseBodyBytes far more often than uniform sampling would.
// Concrete sizes are capped to a small delta around the boundary (plus some
// small sizes) so memory stays reasonable while still hitting the real limit.
type bodySize struct{ n int }

func (bodySize) Generate(rng *rand.Rand, _ int) reflect.Value {
	const boundary = MaxResponseBodyBytes
	const delta = 4096
	var n int
	switch rng.Intn(5) {
	case 0: // small bodies well under the limit
		n = rng.Intn(delta)
	case 1: // just below the boundary
		n = boundary - 1 - rng.Intn(delta)
	case 2: // exactly at the boundary (no truncation)
		n = boundary
	case 3: // just above the boundary (truncation)
		n = boundary + 1 + rng.Intn(delta)
	default: // straddle the boundary on either side
		n = boundary - delta/2 + rng.Intn(delta)
	}
	if n < 0 {
		n = 0
	}
	return reflect.ValueOf(bodySize{n})
}

// Feature: volt-api-client, Property 11: Large response bodies are truncated at the 5 MB boundary
//
// For a generated body size N, Execute returns Body length == min(N,
// MaxResponseBodyBytes), Truncated == (N > MaxResponseBodyBytes), and when
// truncated the retained bytes are exactly the first MaxResponseBodyBytes
// bytes of the original body.
//
// Validates: Requirements 4.11
func TestExecute_LargeBodyTruncationBoundary(t *testing.T) {
	s := model.Settings{TLSVerify: true, TimeoutSeconds: 30}

	property := func(bs bodySize) bool {
		n := bs.n
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
			Body:       io.NopCloser(newPatternReader(n)),
		}
		doer := &mockDoer{resp: resp}

		got := Execute(context.Background(), doer, basePrepared(), s)
		if got.Error != "" {
			return false
		}

		wantTruncated := n > MaxResponseBodyBytes
		wantLen := n
		if wantTruncated {
			wantLen = MaxResponseBodyBytes
		}

		if got.Truncated != wantTruncated {
			return false
		}
		if len(got.Body) != wantLen {
			return false
		}
		if got.SizeBytes != wantLen {
			return false
		}
		// The retained bytes must be exactly the leading prefix of the body.
		for i := 0; i < wantLen; i++ {
			if got.Body[i] != patternByte(i) {
				return false
			}
		}
		return true
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("truncation boundary property failed: %v", err)
	}
}
