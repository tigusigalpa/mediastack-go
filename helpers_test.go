package mediastack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// fixture reads a JSON fixture from testdata.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// jsonHandler replies 200 with the fixture body.
func jsonHandler(t *testing.T, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; Charset=UTF-8")
		w.Write(fixture(t, name))
	}
}

// newServerClient starts an httptest server running handler and returns
// a client pointed at it. The server is closed by t.Cleanup. Additional
// options are applied after WithBaseURL so tests can override them.
func newServerClient(t *testing.T, handler http.Handler, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	base := []Option{WithBaseURL(srv.URL), WithNoRetry()}
	c, err := NewClient("test-key", append(base, opts...)...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// instantSleep returns a sleeper that records delays without waiting.
func instantSleep(recorded *[]time.Duration) func(context.Context, time.Duration) error {
	return func(_ context.Context, d time.Duration) error {
		if recorded != nil {
			*recorded = append(*recorded, d)
		}
		return nil
	}
}
