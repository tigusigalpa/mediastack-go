package mediastack

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestParseRateLimit(t *testing.T) {
	h := http.Header{
		"X-Ratelimit-Limit-Day":       {"100"},
		"X-Ratelimit-Limit-Month":     {"10000"},
		"X-Ratelimit-Remaining-Day":   {"99"},
		"X-Ratelimit-Remaining-Month": {"9000"},
	}
	rl := parseRateLimit(h)
	if !rl.Present {
		t.Fatal("Present must be true")
	}
	if rl.LimitDay != 100 || rl.LimitMonth != 10000 || rl.RemainingDay != 99 || rl.RemainingMonth != 9000 {
		t.Fatalf("rl = %+v", rl)
	}
}

func TestParseRateLimitMissingAndJunk(t *testing.T) {
	if rl := parseRateLimit(http.Header{}); rl.Present {
		t.Fatal("empty headers must not be Present")
	}
	h := http.Header{"X-Ratelimit-Limit-Day": {"soon"}}
	if rl := parseRateLimit(h); rl.Present {
		t.Fatal("non-numeric value must be ignored")
	}
}

func TestRateLimitObserver(t *testing.T) {
	var seen []RateLimit
	c := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Remaining-Day", "5")
		w.Write(fixture(t, "news_empty.json"))
	}), WithRateLimitObserver(func(rl RateLimit) { seen = append(seen, rl) }))
	if _, err := c.News.List(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0].RemainingDay != 5 {
		t.Fatalf("observer = %v", seen)
	}
}

func TestRateLimitObserverPanicSafe(t *testing.T) {
	c := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Remaining-Day", "5")
		w.Write(fixture(t, "news_empty.json"))
	}), WithRateLimitObserver(func(RateLimit) { panic("boom") }))
	if _, err := c.News.List(context.Background(), nil); err != nil {
		t.Fatalf("panicking observer must not break requests: %v", err)
	}
}

func TestLimiterPaces(t *testing.T) {
	c := newServerClient(t, jsonHandler(t, "news_empty.json"), WithRateLimit(20, 1))
	start := time.Now()
	for i := 0; i < 5; i++ {
		if _, err := c.News.List(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	// 5 requests at 20 rps with burst 1: ~4*50ms = 0.2s minimum.
	if elapsed < 150*time.Millisecond {
		t.Fatalf("limiter did not pace: %s", elapsed)
	}
}

func TestLimiterContextCancel(t *testing.T) {
	c := newServerClient(t, jsonHandler(t, "news_empty.json"), WithRateLimit(0.001, 1))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	// First call drains the only token; second waits for a refill that
	// would take 1000s — the context must abort it.
	_, _ = c.News.List(context.Background(), nil)
	_, err := c.News.List(ctx, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
}

func TestLimiterWaitRefill(t *testing.T) {
	// Unit-level: token bucket math with an injected clock and sleeper.
	now := time.Now()
	cur := now
	var slept []time.Duration
	l := newLimiter(10, 2, func() time.Time { return cur },
		func(_ context.Context, d time.Duration) error {
			slept = append(slept, d)
			cur = cur.Add(d)
			return nil
		})
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if err := l.wait(ctx); err != nil {
			t.Fatal(err)
		}
	}
	// burst=2 free, then two ~100ms waits.
	if len(slept) != 2 {
		t.Fatalf("slept = %v", slept)
	}
}
