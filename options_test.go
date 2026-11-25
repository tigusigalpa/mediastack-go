package mediastack

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWithBaseURLInvalid(t *testing.T) {
	for _, u := range []string{"://", "ftp://x.com", "notaurl", "https://"} {
		if _, err := NewClient("k", WithBaseURL(u)); err == nil {
			t.Fatalf("WithBaseURL(%q): want error", u)
		}
	}
}

func TestWithBaseURLPreservesPath(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()
	c, err := NewClient("k", WithBaseURL(srv.URL+"/proxy"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.News.List(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if path != "/proxy/v1/news" {
		t.Fatalf("path = %q, want /proxy/v1/news", path)
	}
}

func TestOptionValidation(t *testing.T) {
	cases := []struct {
		name string
		opt  Option
	}{
		{"nil http client", WithHTTPClient(nil)},
		{"zero timeout", WithTimeout(0)},
		{"negative timeout", WithTimeout(-time.Second)},
		{"empty user agent", WithUserAgent("  ")},
		{"bad jitter low", WithRetry(RetryPolicy{Jitter: -0.1})},
		{"bad jitter high", WithRetry(RetryPolicy{Jitter: 1.5})},
		{"negative delay", WithRetry(RetryPolicy{BaseDelay: -time.Second})},
		{"negative rate", WithRateLimit(-1, 1)},
		{"zero max bytes", WithMaxResponseBytes(0)},
		{"nil clock", WithClock(nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewClient("k", tc.opt); err == nil {
				t.Fatal("want option error")
			}
		})
	}
}

func TestWithTimeoutApplied(t *testing.T) {
	c, err := NewClient("k", WithTimeout(123*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if c.hc.Timeout != 123*time.Second {
		t.Fatalf("timeout = %s", c.hc.Timeout)
	}
}

func TestWithTimeoutIgnoredForCustomClient(t *testing.T) {
	hc := &http.Client{Timeout: 7 * time.Second}
	_, err := NewClient("k", WithHTTPClient(hc), WithTimeout(99*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if hc.Timeout != 7*time.Second {
		t.Fatalf("custom client timeout mutated to %s", hc.Timeout)
	}
}

func TestWithNoRetrySingleAttempt(t *testing.T) {
	var calls int
	c := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(500)
		w.Write(fixture(t, "error_500.json"))
	}))
	_, err := c.News.List(context.Background(), nil)
	if err == nil {
		t.Fatal("want error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestWithLoggerNilIsSilent(t *testing.T) {
	c, err := NewClient("k", WithLogger(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.cfg.logger == nil {
		t.Fatal("logger must default to discard, not nil")
	}
	if c.cfg.logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("default logger must discard everything")
	}
}

func TestDefaultConfig(t *testing.T) {
	c, err := NewClient("k")
	if err != nil {
		t.Fatal(err)
	}
	if c.cfg.baseURL != DefaultBaseURL {
		t.Fatalf("baseURL = %s", c.cfg.baseURL)
	}
	if c.cfg.timeout != 30*time.Second {
		t.Fatalf("timeout = %s", c.cfg.timeout)
	}
	if c.cfg.maxResponseBytes != 10*1024*1024 {
		t.Fatalf("maxResponseBytes = %d", c.cfg.maxResponseBytes)
	}
	if c.limiter != nil {
		t.Fatal("limiter must be off by default")
	}
	if c.cfg.retry.MaxAttempts != 3 {
		t.Fatalf("MaxAttempts = %d", c.cfg.retry.MaxAttempts)
	}
}

func TestNilOptionSkipped(t *testing.T) {
	if _, err := NewClient("k", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestNewClientFromEnv(t *testing.T) {
	t.Setenv(EnvAccessKey, "")
	if _, err := NewClientFromEnv(); !errors.Is(err, ErrMissingAccessKey) {
		t.Fatalf("want ErrMissingAccessKey, got %v", err)
	}
	t.Setenv(EnvAccessKey, "env-key")
	c, err := NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if *c.key != "env-key" {
		t.Fatalf("key = %q", *c.key)
	}
}
