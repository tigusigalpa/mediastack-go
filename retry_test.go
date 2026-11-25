package mediastack

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// flakyServer fails with status failStatus n times then serves
// news_empty.json.
func flakyServer(t *testing.T, failStatus, n int) (*httptest.Server, *atomic.Int64) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= int64(n) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(failStatus)
			w.Write(fixture(t, "error_500.json"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	return srv, &calls
}

func TestRetrySucceedsAfter500(t *testing.T) {
	srv, calls := flakyServer(t, 500, 1)
	defer srv.Close()
	var delays []time.Duration
	c, _ := NewClient("k", WithBaseURL(srv.URL),
		WithRetry(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Second, Jitter: 0}),
		withSleep(instantSleep(&delays)))
	resp, err := c.News.List(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d", calls.Load())
	}
	if resp.Meta.Attempts != 2 {
		t.Fatalf("Attempts = %d", resp.Meta.Attempts)
	}
	if len(delays) != 1 {
		t.Fatalf("delays = %v", delays)
	}
}

func TestRetryExhaustsAttempts(t *testing.T) {
	srv, calls := flakyServer(t, 500, 10)
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL),
		WithRetry(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, Jitter: 0}),
		withSleep(instantSleep(nil)))
	_, err := c.News.List(context.Background(), nil)
	if err == nil {
		t.Fatal("want error after exhausting retries")
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want exactly 3", calls.Load())
	}
	if !errors.Is(err, ErrInternal) {
		t.Fatalf("want ErrInternal, got %v", err)
	}
}

func TestRetryAfterHeaderPreferred(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(429)
			w.Write(fixture(t, "error_429_rate_limit.json"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()
	var delays []time.Duration
	c, _ := NewClient("k", WithBaseURL(srv.URL),
		WithRetry(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 30 * time.Second, Jitter: 0}),
		withSleep(instantSleep(&delays)))
	if _, err := c.News.List(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(delays) != 1 || delays[0] != 7*time.Second {
		t.Fatalf("Retry-After must drive the delay, got %v", delays)
	}
}

func TestRetryAfterCappedByMaxDelay(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "3600")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(503)
			w.Write(fixture(t, "error_503_maintenance.json"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()
	var delays []time.Duration
	c, _ := NewClient("k", WithBaseURL(srv.URL),
		WithRetry(RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Second, Jitter: 0}),
		withSleep(instantSleep(&delays)))
	if _, err := c.News.List(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(delays) != 1 || delays[0] != 2*time.Second {
		t.Fatalf("delay must be capped at MaxDelay, got %v", delays)
	}
}

func TestRetryAfterHTTPDate(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	got := parseRetryAfter(now.Add(3*time.Second).UTC().Format(http.TimeFormat), now)
	if got != 3*time.Second {
		t.Fatalf("HTTP-date Retry-After = %s", got)
	}
	if d := parseRetryAfter(now.Add(-time.Second).Format(http.TimeFormat), now); d != 0 {
		t.Fatalf("past date must yield 0, got %s", d)
	}
	if d := parseRetryAfter("-5", now); d != 0 {
		t.Fatalf("negative must yield 0, got %s", d)
	}
	if d := parseRetryAfter("junk", now); d != 0 {
		t.Fatalf("junk must yield 0, got %s", d)
	}
	if d := parseRetryAfter("", now); d != 0 {
		t.Fatalf("empty must yield 0, got %s", d)
	}
}

func TestNoRetryOn422(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(422)
		w.Write(fixture(t, "error_422_validation.json"))
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL),
		WithRetry(RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond}),
		withSleep(instantSleep(nil)))
	_, err := c.News.List(context.Background(), nil)
	if err == nil {
		t.Fatal("want error")
	}
	if calls.Load() != 1 {
		t.Fatalf("422 must not be retried, calls = %d", calls.Load())
	}
}

func TestNoRetryOnUsageLimit(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(429)
		w.Write(fixture(t, "error_429_usage_limit.json"))
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL),
		WithRetry(RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond}),
		withSleep(instantSleep(nil)))
	_, err := c.News.List(context.Background(), nil)
	if !errors.Is(err, ErrUsageLimitReached) {
		t.Fatalf("want ErrUsageLimitReached, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("usage limit must not be retried, calls = %d", calls.Load())
	}
}

func TestRetryOnContextCancelDuringSleep(t *testing.T) {
	srv, calls := flakyServer(t, 500, 10)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	c, _ := NewClient("k", WithBaseURL(srv.URL),
		WithRetry(RetryPolicy{MaxAttempts: 5, BaseDelay: time.Hour}),
		withSleep(func(ctx context.Context, _ time.Duration) error {
			cancel()
			return ctx.Err()
		}))
	_, err := c.News.List(ctx, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("no second request must be made, calls = %d", calls.Load())
	}
}

func TestRetryOnOverride(t *testing.T) {
	srv, calls := flakyServer(t, 422, 10)
	defer srv.Close()
	// Force retries even on 422 via custom RetryOn.
	c, _ := NewClient("k", WithBaseURL(srv.URL),
		WithRetry(RetryPolicy{
			MaxAttempts: 2, BaseDelay: time.Millisecond,
			RetryOn: func(*http.Response, error) bool { return true },
		}),
		withSleep(instantSleep(nil)))
	// flakyServer writes the 500 body for failing calls; status is 422.
	_, err := c.News.List(context.Background(), nil)
	if err == nil {
		t.Fatal("want error")
	}
	if calls.Load() != 2 {
		t.Fatalf("custom RetryOn must retry, calls = %d", calls.Load())
	}
}

func TestBackoffProgression(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 5, BaseDelay: 100 * time.Millisecond, MaxDelay: 400 * time.Millisecond, Jitter: 0}
	want := []time.Duration{100, 200, 400, 400, 400}
	for i, w := range want {
		if got := p.backoff(i + 1); got != w*time.Millisecond {
			t.Fatalf("backoff(%d) = %s, want %s", i+1, got, w*time.Millisecond)
		}
	}
}

func TestBackoffJitterBounds(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 2, BaseDelay: time.Second, MaxDelay: 10 * time.Second, Jitter: 0.2}
	for i := 0; i < 100; i++ {
		d := p.backoff(1)
		if d < 800*time.Millisecond || d > 1200*time.Millisecond {
			t.Fatalf("jitter out of bounds: %s", d)
		}
	}
}

func TestRetryPolicyDefaults(t *testing.T) {
	p := (RetryPolicy{}).withDefaults()
	if p.MaxAttempts != 3 || p.BaseDelay != 500*time.Millisecond || p.MaxDelay != 10*time.Second || p.Jitter != 0.2 {
		t.Fatalf("defaults = %+v", p)
	}
	p2 := (RetryPolicy{Jitter: 5}).withDefaults()
	if p2.Jitter != 1 {
		t.Fatalf("jitter must clamp to 1, got %g", p2.Jitter)
	}
}

func TestRetriesOnNetworkError(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	srv.Close() // server refuses every connection
	c, _ := NewClient("k", WithBaseURL(srv.URL),
		WithRetry(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, Jitter: 0}),
		withSleep(instantSleep(nil)))
	_, err := c.News.List(context.Background(), nil)
	if err == nil {
		t.Fatal("want transport error")
	}
	if calls.Load() != 0 {
		t.Fatalf("handler must not be reached, calls = %d", calls.Load())
	}
}

func TestDeadlineExceededNotRetried(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()
	slept := 0
	c, _ := NewClient("k", WithBaseURL(srv.URL), WithHTTPClient(&http.Client{Timeout: time.Millisecond}),
		WithRetry(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond}),
		withSleep(func(context.Context, time.Duration) error {
			slept++
			return nil
		}))
	_, err := c.News.List(context.Background(), nil)
	if err == nil {
		t.Fatal("want timeout error")
	}
	if slept != 0 {
		t.Fatalf("deadline must not be retried, retries happened: %d", slept)
	}
	if !IsTimeout(err) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want timeout-ish error, got %v", err)
	}
}

func TestAttemptsRecordedOnFailure(t *testing.T) {
	srv, calls := flakyServer(t, 500, 10)
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL),
		WithRetry(RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond}),
		withSleep(instantSleep(nil)))
	_, err := c.News.List(context.Background(), nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
}
