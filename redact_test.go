package mediastack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRedactKey(t *testing.T) {
	s := "GET https://api.mediastack.com/v1/news?access_key=abc123&limit=5 failed for key abc123"
	got := redactKey(s, "abc123")
	if strings.Contains(got, "abc123") {
		t.Fatalf("key not redacted: %s", got)
	}
	if !strings.Contains(got, "access_key=REDACTED") {
		t.Fatalf("param form not redacted: %s", got)
	}
}

func TestRedactURL(t *testing.T) {
	u, _ := url.Parse("https://api.mediastack.com/v1/news?access_key=sekrit&limit=3&categories=business")
	got := redactURL(u)
	if strings.Contains(got, "sekrit") {
		t.Fatalf("key leaked: %s", got)
	}
	if !strings.Contains(got, "access_key=REDACTED") || !strings.Contains(got, "limit=3") {
		t.Fatalf("other params must survive: %s", got)
	}
	if got := redactURL(nil); got != "" {
		t.Fatalf("nil URL must give empty, got %q", got)
	}
}

func TestRedactHeader(t *testing.T) {
	h := http.Header{"X-Url": {"https://x/?access_key=zzz"}, "Other": {"ok"}}
	out := redactHeader(h)
	if strings.Contains(out.Get("X-Url"), "zzz") {
		t.Fatalf("key leaked: %v", out)
	}
	if out.Get("Other") != "ok" {
		t.Fatal("unrelated header must be preserved")
	}
	// Original untouched.
	if !strings.Contains(h.Get("X-Url"), "zzz") {
		t.Fatal("must return a copy, not mutate input")
	}
}

func TestLogsNeverContainKey(t *testing.T) {
	key := "log-secret-99"
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write(fixture(t, "error_500.json"))
	}))
	defer srv.Close()
	c, _ := NewClient(key, WithBaseURL(srv.URL), WithLogger(logger),
		WithRetry(RetryPolicy{MaxAttempts: 2, BaseDelay: 0, Jitter: 0}),
		withSleep(instantSleep(nil)))
	_, err := c.News.List(context.Background(), nil)
	if err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(buf.String(), key) {
		t.Fatalf("key in logs:\n%s", buf.String())
	}
	if strings.Contains(err.Error(), key) {
		t.Fatalf("key in error: %s", err.Error())
	}
	if strings.Contains(fmt.Sprintf("%+v", err), key) {
		t.Fatalf("key in %%+v: %+v", err)
	}
}

func TestURLErrorKeyRedacted(t *testing.T) {
	key := "url-secret-1"
	c, _ := NewClient(key, WithBaseURL("http://127.0.0.1:1"), WithNoRetry())
	_, err := c.News.List(context.Background(), nil)
	if err == nil {
		t.Fatal("want transport error")
	}
	if strings.Contains(err.Error(), key) {
		t.Fatalf("key leaked through url.Error: %s", err.Error())
	}
	var ue *url.Error
	if !errors.As(err, &ue) {
		t.Fatal("must still unwrap to *url.Error")
	}
	if strings.Contains(ue.URL, key) {
		t.Fatalf("url.Error.URL contains key: %s", ue.URL)
	}
}
