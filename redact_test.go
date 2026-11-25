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

func TestRedirectStaysOnOrigin(t *testing.T) {
	for _, location := range []string{"/redirected", "https://other.example/redirected", "http://api.mediastack.com/redirected"} {
		t.Run(location, func(t *testing.T) {
			var calls int
			hc := handlerTransport(func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.URL.Path == "/v1/news" {
					w.Header().Set("Location", location)
					w.WriteHeader(http.StatusFound)
					return
				}
				if ref := req.Header.Get("Referer"); ref != "" {
					t.Errorf("redirect Referer = %q, want no Referer", ref)
				}
				w.Write(fixture(t, "news_empty.json"))
			})
			var callbackCalls int
			hc.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
				callbackCalls++
				return nil
			}
			client, err := NewClient("redirect-secret", WithHTTPClient(hc), WithNoRetry())
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.News.List(context.Background(), nil)
			if location == "/redirected" {
				if err != nil || calls != 2 || callbackCalls != 1 {
					t.Fatalf("same-origin redirect: trips=%d callback=%d err=%v", calls, callbackCalls, err)
				}
				return
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.HTTPStatus != http.StatusFound || calls != 1 || callbackCalls != 0 {
				t.Fatalf("cross-origin redirect: trips=%d callback=%d err=%v", calls, callbackCalls, err)
			}
		})
	}
}

func TestErrorEnvelopeWithSuccessStatus(t *testing.T) {
	key := "reflected-secret"
	hc := handlerTransport(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "request "+key)
		w.Write([]byte(`{"success":false,"error":{"code":"invalid_access_key","message":"bad key: ` + key + `","context":{"` + key + `":["received ` + key + `"]}}}`))
	})
	client, err := NewClient(key, WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []func() error{
		func() error { _, err := client.News.List(context.Background(), nil); return err },
		func() error { _, err := client.Sources.Search(context.Background(), "bbc", nil); return err },
	} {
		err := call()
		var apiErr *APIError
		if !errors.Is(err, ErrInvalidAccessKey) || !errors.As(err, &apiErr) || apiErr.HTTPStatus != http.StatusOK {
			t.Fatalf("want typed error with HTTP 200, got %v", err)
		}
		for _, value := range []string{err.Error(), apiErr.Message, apiErr.RequestID, apiErr.ValidationContext(), string(apiErr.Body), fmt.Sprintf("%#v", client)} {
			if strings.Contains(value, key) {
				t.Fatal("credential leaked")
			}
		}
	}
}

func TestTransportErrorRedactsNestedMessage(t *testing.T) {
	key := "secret!&value"
	cause := fmt.Errorf("transport rejected %s: %w", url.QueryEscape(key), context.Canceled)
	hc := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, cause
	})}
	client, err := NewClient(key, WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.News.List(context.Background(), nil)
	var transportErr *url.Error
	if !errors.As(err, &transportErr) || !errors.Is(err, context.Canceled) {
		t.Fatalf("want wrapped cancellation, got %v", err)
	}
	if strings.Contains(err.Error(), key) || strings.Contains(err.Error(), url.QueryEscape(key)) {
		t.Fatal("credential leaked from transport error")
	}
}

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
