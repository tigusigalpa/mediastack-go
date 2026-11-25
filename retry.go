package mediastack

import (
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// RetryPolicy controls automatic request retries.
//
// Every Mediastack endpoint is an idempotent GET, which is the only
// reason automatic retries are safe here: repeating a failed request
// never mutates server state.
//
// The default policy retries eligible transport errors and HTTP 5xx.
// For HTTP 429 it retries rate_limit_reached and unclassified 429 responses.
// Recognized quota errors and other 4xx responses return immediately.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts including the first
	// one. The default is 3.
	MaxAttempts int

	// BaseDelay is the delay before the first retry. The default is
	// 500ms.
	BaseDelay time.Duration

	// MaxDelay caps every computed delay and every Retry-After hint.
	// The default is 10s.
	MaxDelay time.Duration

	// Jitter is the randomization factor applied to computed delays, in
	// the range [0,1]. A delay d is multiplied by a random value from
	// [1-Jitter, 1+Jitter]. The default is 0.2.
	Jitter float64

	// RetryOn, when set, replaces the retry decision for transport errors
	// and non-2xx responses. The response is nil on transport failure.
	// For non-2xx responses, err is an *APIError.
	RetryOn func(resp *http.Response, err error) bool
}

// DefaultRetryPolicy returns the policy applied when neither WithRetry
// nor WithNoRetry is used.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    10 * time.Second,
		Jitter:      0.2,
	}
}

// withDefaults fills zero fields from DefaultRetryPolicy.
func (p RetryPolicy) withDefaults() RetryPolicy {
	d := DefaultRetryPolicy()
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = d.MaxAttempts
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = d.BaseDelay
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = d.MaxDelay
	}
	if p.Jitter <= 0 {
		p.Jitter = d.Jitter
	}
	if p.Jitter > 1 {
		p.Jitter = 1
	}
	return p
}

// backoff computes the delay before retry number n (1-based): base
// doubled each attempt, capped at MaxDelay, then randomized by Jitter.
func (p RetryPolicy) backoff(n int) time.Duration {
	d := min(p.BaseDelay, p.MaxDelay)
	for i := 1; i < n && d < p.MaxDelay; i++ {
		if d > p.MaxDelay/2 {
			d = p.MaxDelay
			break
		}
		d *= 2
	}
	if p.Jitter > 0 && d > 0 {
		//nolint:gosec // Retry jitter only needs to spread requests over time.
		factor := 1 + (rand.Float64()*2-1)*p.Jitter
		jittered := float64(d) * factor
		if jittered >= float64(p.MaxDelay) {
			return p.MaxDelay
		}
		d = time.Duration(jittered)
	}
	return d
}

// parseRetryAfter parses a Retry-After header value. Both the delta
// seconds form ("3") and the HTTP-date form are supported. now supplies
// the clock so results are deterministic under WithClock. Negative or
// unparseable values yield zero.
func parseRetryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
		if secs < 0 {
			return 0
		}
		if secs > math.MaxInt64/int64(time.Second) {
			return time.Duration(math.MaxInt64)
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}
