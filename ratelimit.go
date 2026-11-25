package mediastack

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimit carries the quota information APILayer reports through
// response headers. Not every response contains these headers (they are
// absent on unauthenticated responses, for example); check Present
// before reading the counters.
//
// Header names follow
// https://marketplace.apilayer.com/docs/article/rate-limits.
type RateLimit struct {
	// LimitDay is the daily request quota.
	LimitDay int

	// LimitMonth is the monthly request quota.
	LimitMonth int

	// RemainingDay is the number of requests left today.
	RemainingDay int

	// RemainingMonth is the number of requests left this month.
	RemainingMonth int

	// Present reports whether at least one quota header was found.
	Present bool
}

// parseRateLimit reads the APILayer quota headers. Non-numeric values
// are ignored rather than treated as errors.
func parseRateLimit(h http.Header) RateLimit {
	var rl RateLimit
	get := func(name string, dst *int) {
		if v, err := strconv.Atoi(h.Get(name)); err == nil {
			*dst = v
			rl.Present = true
		}
	}
	get("X-RateLimit-Limit-Day", &rl.LimitDay)
	get("X-RateLimit-Limit-Month", &rl.LimitMonth)
	get("X-RateLimit-Remaining-Day", &rl.RemainingDay)
	get("X-RateLimit-Remaining-Month", &rl.RemainingMonth)
	return rl
}

// limiter is a token-bucket rate limiter built on the standard library.
// It launches no goroutine: tokens are refilled lazily when wait is
// called, based on elapsed wall time. now and sleep are injected so
// tests can run without real delays.
type limiter struct {
	mu     sync.Mutex
	rate   float64 // tokens per second
	burst  float64
	tokens float64
	last   time.Time
	now    func() time.Time
	sleep  func(ctx context.Context, d time.Duration) error
}

// newLimiter returns a limiter that starts with a full bucket of burst
// tokens and refills at rate tokens per second.
func newLimiter(rate float64, burst int, now func() time.Time, sleep func(context.Context, time.Duration) error) *limiter {
	if burst < 1 {
		burst = 1
	}
	return &limiter{
		rate:   rate,
		burst:  float64(burst),
		tokens: float64(burst),
		last:   now(),
		now:    now,
		sleep:  sleep,
	}
}

// wait blocks until a token is available or ctx is done.
func (l *limiter) wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		l.mu.Lock()
		now := l.now()
		if elapsed := now.Sub(l.last).Seconds(); elapsed > 0 {
			l.tokens += elapsed * l.rate
			if l.tokens > l.burst {
				l.tokens = l.burst
			}
			l.last = now
		}
		if l.tokens >= 1 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}
		deficit := 1 - l.tokens
		l.mu.Unlock()

		nanos := math.Ceil(deficit / l.rate * float64(time.Second))
		d := time.Duration(math.MaxInt64)
		if nanos < float64(d) {
			d = time.Duration(nanos)
		}
		if err := l.sleep(ctx, d); err != nil {
			return err
		}
	}
}
