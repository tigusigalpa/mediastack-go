package mediastack

import (
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
// The default policy retries network failures, HTTP 500/502/503/504 and
// the rate_limit_reached variant of HTTP 429. The quota variants of 429
// (usage_limit_reached, daily_usage_limit_reached, fair_use_limit_reached)
// and every other 4xx are never retried: waiting cannot fix them.
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

	// RetryOn, when set, replaces the built-in retry decision. It
	// receives the response (nil on transport failure) and the error
	// (nil when the response status is 2xx; *APIError for other
	// statuses; the transport error otherwise).
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
	d := p.BaseDelay
	for i := 1; i < n; i++ {
		d *= 2
		if d >= p.MaxDelay {
			d = p.MaxDelay
			break
		}
	}
	if d > p.MaxDelay {
		d = p.MaxDelay
	}
	if p.Jitter > 0 && d > 0 {
		factor := 1 + (rand.Float64()*2-1)*p.Jitter //nolint:gosec // backoff jitter is not security-sensitive
		d = time.Duration(float64(d) * factor)
	}
	if d < 0 {
		d = 0
	}
	if d > p.MaxDelay {
		d = p.MaxDelay
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
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
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
