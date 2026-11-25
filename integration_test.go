//go:build integration

package mediastack_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	mediastack "github.com/tigusigalpa/mediastack-go"
)

// Integration tests hit the real api.mediastack.com. Run them with:
//
//	MEDIASTACK_ACCESS_KEY=... go test -tags=integration ./...
//
// A 429 response is treated as a skip, not a failure: quota and rate
// limits are an account condition, not a code defect.

func integrationClient(t *testing.T) *mediastack.Client {
	t.Helper()
	key := os.Getenv(mediastack.EnvAccessKey)
	if key == "" {
		t.Skip(mediastack.EnvAccessKey + " not set")
	}
	c, err := mediastack.NewClient(key)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func skipOnRateLimit(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	var apiErr *mediastack.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus == 429 {
		t.Skipf("skipping: rate/usage limit reached: %v", err)
	}
}

func TestIntegrationNewsList(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.News.List(ctx, &mediastack.NewsParams{Limit: 3})
	skipOnRateLimit(t, err)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("no articles returned")
	}
	if resp.Meta.StatusCode != 200 {
		t.Fatalf("status = %d", resp.Meta.StatusCode)
	}
	t.Logf("got %d articles, first: %s", len(resp.Data), resp.Data[0].Title)
}

func TestIntegrationSourcesSearch(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.Sources.Search(ctx, "abc", &mediastack.SourceParams{Limit: 5})
	skipOnRateLimit(t, err)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("no sources returned")
	}
	if resp.Data[0].ID == "" {
		t.Fatal("source ID must be populated")
	}
	t.Logf("got %d sources, first: %s (%s)", len(resp.Data), resp.Data[0].Name, resp.Data[0].ID)
}

func TestIntegrationInvalidKey(t *testing.T) {
	c, err := mediastack.NewClient("definitely-invalid-key")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = c.News.List(ctx, &mediastack.NewsParams{Limit: 1})
	if err == nil {
		t.Fatal("expected authentication error")
	}
	if !errors.Is(err, mediastack.ErrInvalidAccessKey) {
		t.Fatalf("want ErrInvalidAccessKey, got %v", err)
	}
}
