package mediastack

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func serveError(t *testing.T, status int, fixtureName string) *Client {
	hc := handlerTransport(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(fixture(t, fixtureName))
	})
	c, err := NewClient("k", WithHTTPClient(hc), WithNoRetry())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func apiErrOf(t *testing.T, err error) *APIError {
	t.Helper()
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T (%v)", err, err)
	}
	return apiErr
}

func TestErrorFormatA(t *testing.T) {
	c := serveError(t, 401, "error_401_format_a.json")
	_, err := c.News.List(context.Background(), nil)
	if !errors.Is(err, ErrInvalidAccessKey) {
		t.Fatalf("want ErrInvalidAccessKey, got %v", err)
	}
	ae := apiErrOf(t, err)
	if ae.Code != "invalid_access_key" {
		t.Fatalf("Code = %q", ae.Code)
	}
	if ae.NumericCode != 0 {
		t.Fatalf("NumericCode = %d", ae.NumericCode)
	}
	if ae.HTTPStatus != 401 {
		t.Fatalf("HTTPStatus = %d", ae.HTTPStatus)
	}
	if !strings.Contains(ae.Message, "Access Key") {
		t.Fatalf("Message = %q", ae.Message)
	}
	if ae.Temporary() {
		t.Fatal("401 must not be temporary")
	}
}

func TestErrorFormatB(t *testing.T) {
	c := serveError(t, 401, "error_401_format_b.json")
	_, err := c.News.List(context.Background(), nil)
	if !errors.Is(err, ErrInvalidAccessKey) {
		t.Fatalf("want ErrInvalidAccessKey, got %v", err)
	}
	ae := apiErrOf(t, err)
	if ae.Code != "invalid_access_key" || ae.NumericCode != 101 {
		t.Fatalf("Code=%q NumericCode=%d", ae.Code, ae.NumericCode)
	}
}

func TestErrorFormatCValidation(t *testing.T) {
	c := serveError(t, 422, "error_422_validation.json")
	_, err := c.News.List(context.Background(), nil)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
	ae := apiErrOf(t, err)
	if ae.Context["date"][0] != "NO_SUCH_CHOICE_ERROR" {
		t.Fatalf("Context = %v", ae.Context)
	}
	if got := ae.ValidationContext(); got != "date=[NO_SUCH_CHOICE_ERROR]" {
		t.Fatalf("ValidationContext = %q", got)
	}
	if !strings.Contains(err.Error(), "validation failed: date=[NO_SUCH_CHOICE_ERROR]") {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestError403Restricted(t *testing.T) {
	c := serveError(t, 403, "error_403_restricted.json")
	_, err := c.News.List(context.Background(), nil)
	if !errors.Is(err, ErrAccessRestricted) {
		t.Fatalf("want ErrAccessRestricted, got %v", err)
	}
	if apiErrOf(t, err).Code != "function_access_restricted" {
		t.Fatalf("Code = %q", apiErrOf(t, err).Code)
	}
}

func TestError404(t *testing.T) {
	c := serveError(t, 404, "error_404.json")
	if _, err := c.News.List(context.Background(), nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestError429Variants(t *testing.T) {
	c := serveError(t, 429, "error_429_rate_limit.json")
	_, err := c.News.List(context.Background(), nil)
	if !errors.Is(err, ErrRateLimitReached) {
		t.Fatalf("want ErrRateLimitReached, got %v", err)
	}
	if !apiErrOf(t, err).Temporary() {
		t.Fatal("rate_limit_reached must be Temporary")
	}

	c = serveError(t, 429, "error_429_usage_limit.json")
	_, err = c.News.List(context.Background(), nil)
	if !errors.Is(err, ErrUsageLimitReached) {
		t.Fatalf("want ErrUsageLimitReached, got %v", err)
	}
	if apiErrOf(t, err).Temporary() {
		t.Fatal("usage_limit_reached must NOT be Temporary")
	}
}

func TestError500And503(t *testing.T) {
	c := serveError(t, 500, "error_500.json")
	_, err := c.News.List(context.Background(), nil)
	if !errors.Is(err, ErrInternal) {
		t.Fatalf("want ErrInternal, got %v", err)
	}
	if !apiErrOf(t, err).Temporary() {
		t.Fatal("500 must be Temporary")
	}

	c = serveError(t, 503, "error_503_maintenance.json")
	_, err = c.News.List(context.Background(), nil)
	if !errors.Is(err, ErrMaintenance) {
		t.Fatalf("want ErrMaintenance, got %v", err)
	}
}

func TestErrorHTMLBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(403)
		w.Write([]byte("<html><body>Cloudflare block</body></html>"))
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL), WithNoRetry())
	_, err := c.News.List(context.Background(), nil)
	if !errors.Is(err, ErrAccessRestricted) {
		t.Fatalf("want ErrAccessRestricted, got %v", err)
	}
	ae := apiErrOf(t, err)
	if ae.Code != "" {
		t.Fatalf("Code = %q, want empty fallback to status", ae.Code)
	}
	if len(ae.Body) == 0 {
		t.Fatal("Body must retain the (truncated) response")
	}
}

func TestErrorStringFormat(t *testing.T) {
	e := &APIError{
		HTTPStatus: 429,
		Code:       "rate_limit_reached",
		Message:    "You have exceeded the maximum rate limitation",
		RequestID:  "8f2c-abc",
	}
	got := e.Error()
	for _, want := range []string{"mediastack:", "429", "rate_limit_reached", "exceeded", "request id: 8f2c-abc"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Error() = %q, missing %q", got, want)
		}
	}
	e2 := &APIError{HTTPStatus: 500}
	if got := e2.Error(); !strings.Contains(got, "500") {
		t.Fatalf("Error() = %q", got)
	}
}

func TestAPIErrorNeverLeaksKey(t *testing.T) {
	key := "key-12345-secret"
	body := []byte(`{"error":{"code":"invalid_access_key","message":"bad key access_key=` + key + `"}}`)
	ae := decodeAPIError(401, "", body, key)
	if strings.Contains(ae.Error(), key) {
		t.Fatalf("key in Error(): %s", ae.Error())
	}
	if strings.Contains(fmt.Sprintf("%+v", ae), key) {
		t.Fatalf("key in %%+v dump: %+v", ae)
	}
	if strings.Contains(string(ae.Body), key) {
		t.Fatalf("key in Body: %s", ae.Body)
	}
}

func TestSentinelFor(t *testing.T) {
	cases := []struct {
		code   string
		status int
		want   error
	}{
		{"missing_access_key", 401, ErrInvalidAccessKey},
		{"invalid_access_key", 401, ErrInvalidAccessKey},
		{"inactive_user", 401, ErrInactiveUser},
		{"account_on_hold", 401, ErrAccountOnHold},
		{"api_access_blocked", 403, ErrAccessRestricted},
		{"https_access_restricted", 403, ErrAccessRestricted},
		{"function_access_restricted", 403, ErrAccessRestricted},
		{"invalid_api_function", 404, ErrInvalidFunction},
		{"404_not_found", 404, ErrNotFound},
		{"validation_error", 422, ErrValidation},
		{"usage_limit_reached", 429, ErrUsageLimitReached},
		{"daily_usage_limit_reached", 429, ErrUsageLimitReached},
		{"fair_use_limit_reached", 429, ErrUsageLimitReached},
		{"rate_limit_reached", 429, ErrRateLimitReached},
		{"internal_error", 500, ErrInternal},
		{"maintenance_mode", 503, ErrMaintenance},
		{"unknown_code", 401, ErrInvalidAccessKey},
		{"unknown_code", 403, ErrAccessRestricted},
		{"unknown_code", 404, ErrNotFound},
		{"unknown_code", 422, ErrValidation},
		{"unknown_code", 429, ErrRateLimitReached},
		{"unknown_code", 500, ErrInternal},
		{"unknown_code", 503, ErrInternal},
		{"unknown_code", 200, nil},
	}
	for _, tc := range cases {
		got := sentinelFor(tc.code, tc.status)
		if tc.want == nil {
			if got != nil {
				t.Errorf("sentinelFor(%q,%d) = %v, want nil", tc.code, tc.status, got)
			}
			continue
		}
		if !errors.Is(got, tc.want) {
			t.Errorf("sentinelFor(%q,%d) = %v, want %v", tc.code, tc.status, got, tc.want)
		}
	}
}

func TestNumericCodeFallback(t *testing.T) {
	// Numeric code with no type: unambiguous numbers map, ambiguous do not.
	body := []byte(`{"success":false,"error":{"code":103,"info":"bad function"}}`)
	ae := decodeAPIError(404, "", body, "")
	if ae.Code != "invalid_api_function" || ae.NumericCode != 103 {
		t.Fatalf("Code=%q NumericCode=%d", ae.Code, ae.NumericCode)
	}
	body = []byte(`{"success":false,"error":{"code":101,"info":"bad key"}}`)
	ae = decodeAPIError(401, "", body, "")
	if ae.Code != "" {
		t.Fatalf("ambiguous 101 must leave Code empty, got %q", ae.Code)
	}
	if !errors.Is(ae, ErrInvalidAccessKey) {
		t.Fatal("empty Code on 401 must still map via status")
	}
	body = []byte(`{"success":false,"error":{"code":104,"type":"usage_limit_reached","info":"quota"}}`)
	ae = decodeAPIError(429, "", body, "")
	if ae.Code != "usage_limit_reached" || ae.NumericCode != 104 {
		t.Fatalf("Code=%q", ae.Code)
	}
}

func TestAmbiguousNumericQuotaError(t *testing.T) {
	body := []byte(`{"error":{"code":104,"info":"usage limit exceeded"}}`)
	err := decodeAPIError(http.StatusTooManyRequests, "", body, "")
	if !errors.Is(err, ErrUsageLimitReached) || IsTemporary(err) {
		t.Fatalf("want a permanent usage limit error, got %v", err)
	}
}

func TestValidationErrorOutput(t *testing.T) {
	ve := &ValidationError{Fields: map[string][]string{
		"offset": {"must be >= 0, got -3"},
		"limit":  {"must be between 1 and 100, got 500"},
	}}
	got := ve.Error()
	if !strings.HasPrefix(got, "mediastack: invalid parameters: ") {
		t.Fatalf("Error() = %q", got)
	}
	if strings.Index(got, "limit") > strings.Index(got, "offset") {
		t.Fatalf("fields not sorted: %q", got)
	}
	if !errors.Is(ve, ErrInvalidParams) {
		t.Fatal("must match ErrInvalidParams")
	}
}

func TestIsTimeoutAndTemporary(t *testing.T) {
	if IsTemporary(nil) || IsTimeout(nil) {
		t.Fatal("nil must not be temporary/timeout")
	}
	if IsTemporary(context.Canceled) || IsTemporary(context.DeadlineExceeded) {
		t.Fatal("ctx errors must not be temporary")
	}
	if IsTimeout(context.DeadlineExceeded) == false {
		// DeadlineExceeded implements net.Error with Timeout()==true.
		t.Fatal("deadline must be a timeout")
	}
	if !IsTemporary(&APIError{HTTPStatus: 500}) {
		t.Fatal("500 APIError must be temporary")
	}
	if IsTemporary(&APIError{HTTPStatus: 422}) {
		t.Fatal("422 must not be temporary")
	}
	var netErr net.Error = &net.DNSError{IsTimeout: true}
	if !IsTemporary(netErr) || !IsTimeout(netErr) {
		t.Fatal("net timeout must be temporary and a timeout")
	}
}

func TestRequestIDFallback(t *testing.T) {
	h := http.Header{"X-Request-Id": {"rid-1"}}
	if got := requestID(h); got != "rid-1" {
		t.Fatalf("requestID = %q", got)
	}
	h.Set("cf-ray", "cf-9")
	if got := requestID(h); got != "cf-9" {
		t.Fatalf("cf-ray must win, got %q", got)
	}
}

func TestRetryAfterCaptured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(429)
		w.Write(fixture(t, "error_429_usage_limit.json"))
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL), WithNoRetry())
	_, err := c.News.List(context.Background(), nil)
	ae := apiErrOf(t, err)
	if ae.RetryAfter != 5*time.Second {
		t.Fatalf("RetryAfter = %s", ae.RetryAfter)
	}
}
