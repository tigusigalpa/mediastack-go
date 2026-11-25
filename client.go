package mediastack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// ModulePath is the Go import path of this module.
	ModulePath = "github.com/tigusigalpa/mediastack-go"

	// Namespace is the public identity of this SDK.
	Namespace = "Tigusigalpa/Mediastack"

	// DefaultBaseURL is the production endpoint of the Mediastack API.
	DefaultBaseURL = "https://api.mediastack.com"

	// APIVersion is the API version segment used in request paths.
	APIVersion = "v1"

	// EnvAccessKey is the environment variable read by NewClientFromEnv.
	EnvAccessKey = "MEDIASTACK_ACCESS_KEY"

	// MaxLimit is the largest page size the API accepts.
	MaxLimit = 100

	// MaxPaginationWindow is the largest offset+limit the API accepts.
	MaxPaginationWindow = 10000

	// DefaultLimit is the page size the API applies when limit is omitted.
	DefaultLimit = 25
)

// Client is a Mediastack API client. It is safe for concurrent use by
// multiple goroutines; create it once and reuse it for the lifetime of
// the application.
//
// The client never exposes the access key in logs, error strings or
// %-format output.
type Client struct {
	// News provides access to GET /v1/news: live and historical
	// articles with filters.
	News *NewsService

	// Sources provides access to GET /v1/sources: source discovery.
	Sources *SourcesService

	key          *string // Keep the credential out of fmt's struct dumps.
	hc           *http.Client
	ownTransport bool
	cfg          config
	limiter      *limiter

	mu   sync.RWMutex
	last RateLimit
}

// NewClient creates a client with the given API access key. The key is
// trimmed; an empty or whitespace-only key fails with
// ErrMissingAccessKey. NewClient performs no network call.
//
// Obtain a key in the dashboard at https://mediastack.com/dashboard.
func NewClient(accessKey string, opts ...Option) (*Client, error) {
	key := strings.TrimSpace(accessKey)
	if key == "" {
		return nil, ErrMissingAccessKey
	}
	cfg := defaultConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	hc := &http.Client{Timeout: cfg.timeout}
	own := false
	if cfg.httpClient != nil {
		*hc = *cfg.httpClient
	} else if transport, ok := http.DefaultTransport.(*http.Transport); ok {
		hc.Transport = transport.Clone()
		own = true
	}
	hc.CheckRedirect = checkRedirect(hc.CheckRedirect)

	c := &Client{
		key:          &key,
		hc:           hc,
		ownTransport: own,
		cfg:          cfg,
	}
	if cfg.ratePerSecond > 0 {
		c.limiter = newLimiter(cfg.ratePerSecond, cfg.rateBurst, cfg.clock, cfg.sleep)
	}
	c.News = &NewsService{c: c}
	c.Sources = &SourcesService{c: c}
	return c, nil
}

// NewClientFromEnv creates a client using the MEDIASTACK_ACCESS_KEY
// environment variable. It is the only place the SDK reads the
// environment.
func NewClientFromEnv(opts ...Option) (*Client, error) {
	return NewClient(os.Getenv(EnvAccessKey), opts...)
}

// RateLimit reports the last quota snapshot received from the API.
// It returns a zero value until a response supplies quota headers.
func (c *Client) RateLimit() RateLimit {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.last
}

// Close releases idle connections held by the underlying transport.
// When the client was built on a custom *http.Client (see
// WithHTTPClient), Close does nothing and returns nil: ownership of
// that client stays with the caller.
func (c *Client) Close() error {
	if c.ownTransport {
		c.hc.CloseIdleConnections()
	}
	return nil
}

// checkRedirect only allows redirects within the configured request origin.
// Go's default Referer includes the previous request's query string.
func checkRedirect(custom func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		first := via[0].URL
		if req.URL.Scheme != first.Scheme || !strings.EqualFold(req.URL.Host, first.Host) || req.URL.User != nil {
			return http.ErrUseLastResponse
		}
		req.Header.Del("Referer")
		if custom != nil {
			return custom(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
}

// buildURL assembles the request URL: base + /v1/ + path + query. The
// query keeps literal commas (the reference format used in the API
// documentation) instead of the %2C produced by url.Values.Encode.
func (c *Client) buildURL(path string, values url.Values) string {
	u := c.cfg.baseURL + "/" + APIVersion + "/" + path
	if len(values) == 0 {
		return u
	}
	q := values.Encode()
	q = strings.ReplaceAll(q, "%2C", ",")
	return u + "?" + q
}

// do executes one logical GET request: it applies the client-side
// limiter, retries according to the configured policy, enforces the
// response size cap and normalizes API errors. It returns the response
// body on success.
func (c *Client) do(ctx context.Context, path string, q url.Values) ([]byte, ResponseMeta, error) {
	if ctx == nil {
		return nil, ResponseMeta{}, errors.New("mediastack: context must not be nil")
	}
	values := make(url.Values, len(q)+1)
	for k, vs := range q {
		cp := make([]string, len(vs))
		copy(cp, vs)
		values[k] = cp
	}
	key := *c.key
	values.Set("access_key", key)
	rawURL := c.buildURL(path, values)

	policy := c.cfg.retry
	if c.cfg.noRetry {
		policy.MaxAttempts = 1
	}
	if policy.MaxAttempts < 1 {
		policy.MaxAttempts = 1
	}

	var meta ResponseMeta
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if c.limiter != nil {
			if err := c.limiter.wait(ctx); err != nil {
				return nil, meta, err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, meta, fmt.Errorf("mediastack: build request: %w", sanitizeError(err, key))
		}
		req.Header.Set("User-Agent", c.cfg.userAgent)
		req.Header.Set("Accept", "application/json")

		start := c.cfg.clock()
		resp, err := c.hc.Do(req)
		meta.Attempts = attempt

		if err != nil {
			meta.Duration += c.cfg.clock().Sub(start)
			err = sanitizeError(err, key)
			if !c.shouldRetry(policy, nil, err) || attempt == policy.MaxAttempts {
				return nil, meta, fmt.Errorf("mediastack: request failed: %w", err)
			}
			delay := policy.backoff(attempt)
			c.logRetry(attempt, delay, err)
			if werr := c.cfg.sleep(ctx, delay); werr != nil {
				return nil, meta, werr
			}
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, c.cfg.maxResponseBytes+1))
		_ = resp.Body.Close()
		meta.StatusCode = resp.StatusCode
		meta.RequestID = redactKey(requestID(resp.Header), key)
		meta.FetchedAt = c.cfg.clock()
		meta.Duration += meta.FetchedAt.Sub(start)
		meta.RateLimit = parseRateLimit(resp.Header)
		c.storeRateLimit(meta.RateLimit)
		c.logResponse(req.URL, resp.StatusCode, meta, attempt)

		if readErr != nil {
			return nil, meta, fmt.Errorf("mediastack: read response body: %w", sanitizeError(readErr, key))
		}
		if int64(len(body)) > c.cfg.maxResponseBytes {
			return nil, meta, fmt.Errorf("%w: %d byte limit", ErrResponseTooLarge, c.cfg.maxResponseBytes)
		}

		if mt := mediaType(resp.Header); mt != "" && mt != "application/json" {
			c.cfg.logger.Warn("mediastack: unexpected content type",
				"content_type", redactKey(mt, key),
				"status", resp.StatusCode,
				"request_id", meta.RequestID,
			)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if len(bytes.TrimSpace(body)) == 0 {
				return nil, meta, ErrEmptyResponse
			}
			return body, meta, nil
		}

		apiErr := decodeAPIError(resp.StatusCode, meta.RequestID, body, key)
		apiErr.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), c.cfg.clock())

		if !c.shouldRetry(policy, resp, apiErr) || attempt == policy.MaxAttempts {
			return nil, meta, apiErr
		}
		delay := policy.backoff(attempt)
		if apiErr.RetryAfter > 0 {
			delay = apiErr.RetryAfter
			if delay > policy.MaxDelay {
				delay = policy.MaxDelay
			}
		}
		c.logRetry(attempt, delay, apiErr)
		if werr := c.cfg.sleep(ctx, delay); werr != nil {
			return nil, meta, werr
		}
	}
	return nil, meta, errors.New("mediastack: unreachable retry state")
}

// shouldRetry decides whether a failed attempt is worth repeating.
// A caller-supplied RetryOn replaces the built-in table.
func (c *Client) shouldRetry(policy RetryPolicy, resp *http.Response, err error) bool {
	if policy.RetryOn != nil {
		return policy.RetryOn(resp, err)
	}
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Temporary()
	}
	// Transport error: retry everything except caller cancellation.
	return IsTemporary(err)
}

// storeRateLimit records the latest quota snapshot, ignoring empty ones.
func (c *Client) storeRateLimit(rl RateLimit) {
	if !rl.Present {
		return
	}
	c.mu.Lock()
	c.last = rl
	c.mu.Unlock()
	if fn := c.cfg.rateObserver; fn != nil {
		safeObserve(fn, rl)
	}
}

// safeObserve invokes the observer, shielding the request path from a
// panicking callback.
func safeObserve(fn func(RateLimit), rl RateLimit) {
	defer func() { _ = recover() }()
	fn(rl)
}

// logResponse emits a Debug record for a completed HTTP attempt.
func (c *Client) logResponse(u *url.URL, status int, meta ResponseMeta, attempt int) {
	attrs := []any{
		"method", http.MethodGet,
		"url", redactKey(redactURL(u), *c.key),
		"status", status,
		"attempt", attempt,
		"duration_ms", meta.Duration.Milliseconds(),
		"request_id", meta.RequestID,
	}
	if meta.RateLimit.Present {
		attrs = append(attrs,
			"rate_limit_remaining_day", meta.RateLimit.RemainingDay,
			"rate_limit_remaining_month", meta.RateLimit.RemainingMonth,
		)
	}
	c.cfg.logger.Debug("mediastack: response", attrs...)
}

// logRetry emits a Warn record before a retry.
func (c *Client) logRetry(attempt int, delay time.Duration, cause error) {
	c.cfg.logger.Warn("mediastack: retrying request",
		"attempt", attempt+1,
		"delay_ms", delay.Milliseconds(),
		"reason", cause.Error(),
	)
}

// mediaType parses the Content-Type header, tolerating the API's
// non-standard "application/json; Charset=UTF-8" casing.
func mediaType(h http.Header) string {
	ct := h.Get("Content-Type")
	if ct == "" {
		return ""
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return ct
	}
	return mt
}

// sanitizeError redacts request and transport error messages while retaining
// their error chain for classification.
func sanitizeError(err error, key string) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		cp := *ue
		cp.URL = redactKey(ue.URL, key)
		cp.Err = sanitizeError(ue.Err, key)
		return &cp
	}
	message := err.Error()
	if safe := redactKey(message, key); safe != message {
		return &redactedError{message: safe, cause: err}
	}
	return err
}

type redactedError struct {
	message string
	cause   error
}

func (e *redactedError) Error() string { return e.message }
func (e *redactedError) Unwrap() error { return e.cause }

// Timeout preserves the result of url.Error.Timeout after sanitizing its cause.
func (e *redactedError) Timeout() bool { return IsTimeout(e.cause) }
