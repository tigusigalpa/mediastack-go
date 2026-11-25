package mediastack

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestNewClientMissingKey(t *testing.T) {
	for _, key := range []string{"", "   ", "\t\n"} {
		if _, err := NewClient(key); !errors.Is(err, ErrMissingAccessKey) {
			t.Fatalf("key %q: want ErrMissingAccessKey, got %v", key, err)
		}
	}
}

func TestNewClientTrimsKey(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.URL.Query().Get("access_key")
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()

	c, err := NewClient("  padded-key  ", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.News.List(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if gotKey != "padded-key" {
		t.Fatalf("access_key = %q, want padded-key", gotKey)
	}
}

func TestKeySentAsQueryNotHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("key must never be sent in Authorization header")
		}
		if r.URL.Query().Get("access_key") != "k1" {
			t.Error("access_key query param missing")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()
	c, _ := NewClient("k1", WithBaseURL(srv.URL))
	if _, err := c.News.List(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestUserAgentHeader(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	if _, err := c.News.List(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ua, "mediastack-go/"+Version) {
		t.Fatalf("User-Agent %q missing version", ua)
	}
	if !strings.Contains(ua, "github.com/tigusigalpa/mediastack-go") {
		t.Fatalf("User-Agent %q missing repo URL", ua)
	}
}

func TestWithBaseURLTrailingSlash(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()
	c, err := NewClient("k", WithBaseURL(srv.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.News.List(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(path, "//") {
		t.Fatalf("path %q contains double slash", path)
	}
	if path != "/v1/news" {
		t.Fatalf("path = %q, want /v1/news", path)
	}
}

func TestWithHTTPClientClose(t *testing.T) {
	hc := &http.Client{}
	c, err := NewClient("k", WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	// The custom client must still work after Close.
	srv := httptest.NewServer(jsonHandler(t, "news_empty.json"))
	defer srv.Close()
	c2, _ := NewClient("k", WithHTTPClient(hc), WithBaseURL(srv.URL))
	if _, err := c2.News.List(context.Background(), nil); err != nil {
		t.Fatalf("custom client broken after Close: %v", err)
	}
}

func TestResponseMeta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("cf-ray", "8f2c-test")
		w.Header().Set("X-RateLimit-Remaining-Month", "42")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	resp, err := c.News.List(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Meta.StatusCode != 200 {
		t.Fatalf("StatusCode = %d", resp.Meta.StatusCode)
	}
	if resp.Meta.RequestID != "8f2c-test" {
		t.Fatalf("RequestID = %q", resp.Meta.RequestID)
	}
	if resp.Meta.Attempts != 1 {
		t.Fatalf("Attempts = %d", resp.Meta.Attempts)
	}
	if resp.Meta.FetchedAt.IsZero() {
		t.Fatal("FetchedAt not set")
	}
	if !resp.Meta.RateLimit.Present || resp.Meta.RateLimit.RemainingMonth != 42 {
		t.Fatalf("RateLimit = %+v", resp.Meta.RateLimit)
	}
	if got := c.RateLimit(); got.RemainingMonth != 42 {
		t.Fatalf("Client.RateLimit = %+v", got)
	}
}

func TestEmptySuccessBody(t *testing.T) {
	c := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
	}))
	if _, err := c.News.List(context.Background(), nil); !errors.Is(err, ErrEmptyResponse) {
		t.Fatalf("want ErrEmptyResponse, got %v", err)
	}
}

func TestResponseTooLarge(t *testing.T) {
	c := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"pagination":{},"data":[` + strings.Repeat(" ", 5000) + `]}`))
	}), WithMaxResponseBytes(64))
	if _, err := c.News.List(context.Background(), nil); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("want ErrResponseTooLarge, got %v", err)
	}
}

func TestNetworkErrorRedactsKey(t *testing.T) {
	// Unreachable server: a transport error wraps *url.Error which
	// contains the full URL — the key must be scrubbed.
	c, err := NewClient("super-secret-key", WithBaseURL("http://127.0.0.1:1"), WithNoRetry())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.News.List(context.Background(), nil)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), "super-secret-key") {
		t.Fatalf("key leaked in error: %s", err.Error())
	}
}

func TestRateLimitConcurrent(t *testing.T) {
	c := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Remaining-Month", "7")
		w.Write(fixture(t, "news_empty.json"))
	}))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.News.List(context.Background(), nil)
			_ = c.RateLimit()
		}()
	}
	wg.Wait()
	if got := c.RateLimit(); !got.Present {
		t.Fatal("rate limit not recorded")
	}
}

func TestBuildURLLiteralCommas(t *testing.T) {
	c, _ := NewClient("k")
	u := c.buildURL("news", map[string][]string{
		"access_key": {"KEY"},
		"countries":  {"us,gb,-de"},
	})
	if !strings.Contains(u, "countries=us,gb,-de") {
		t.Fatalf("commas must stay literal: %s", u)
	}
	if !strings.HasPrefix(u, DefaultBaseURL+"/v1/news?") {
		t.Fatalf("unexpected URL %s", u)
	}
}

func BenchmarkDecodeNewsResponse(b *testing.B) {
	body, err := os.ReadFile("testdata/news_ok.json")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var out NewsResponse
		if err := json.Unmarshal(body, &out); err != nil {
			b.Fatal(err)
		}
	}
}
