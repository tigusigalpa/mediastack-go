package mediastack

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// config collects everything NewClient needs. Options mutate it; it is
// copied into the Client and never touched again.
type config struct {
	httpClient       *http.Client
	timeout          time.Duration
	baseURL          string
	userAgent        string
	retry            RetryPolicy
	noRetry          bool
	ratePerSecond    float64
	rateBurst        int
	logger           *slog.Logger
	maxResponseBytes int64
	strictEnums      bool
	clock            func() time.Time
	sleep            func(ctx context.Context, d time.Duration) error
	rateObserver     func(RateLimit)
}

func defaultConfig() config {
	return config{
		timeout:          30 * time.Second,
		baseURL:          DefaultBaseURL,
		userAgent:        UserAgent(),
		retry:            DefaultRetryPolicy(),
		logger:           slog.New(discardHandler{}),
		maxResponseBytes: 10 * 1024 * 1024,
		clock:            time.Now,
		sleep:            sleepWithContext,
	}
}

// sleepWithContext sleeps for d, aborting early when ctx is done. It is
// the default sleeper; tests substitute their own via WithClock-adjacent
// hooks.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// discardHandler is a slog.Handler that drops every record. It exists
// because slog.DiscardHandler requires Go 1.24 while this module
// supports Go 1.23.
type discardHandler struct{}

// Enabled reports that nothing is ever logged.
func (discardHandler) Enabled(context.Context, slog.Level) bool { return false }

// Handle discards the record.
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }

// WithAttrs returns the same handler.
func (h discardHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

// WithGroup returns the same handler.
func (h discardHandler) WithGroup(string) slog.Handler { return h }

// Option customizes a Client at construction time. Options are applied
// in the order they are passed; the last one wins. Every option
// validates its argument and returns an error instead of panicking.
type Option func(*config) error

// WithHTTPClient takes a shallow copy of hc at construction time and
// retains its transport, cookie jar, timeout and same-origin redirect
// callback. WithTimeout does not override hc.Timeout. The caller owns
// the transport and is responsible for closing its idle connections.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *config) error {
		if hc == nil {
			return errors.New("mediastack: http client must not be nil")
		}
		c.httpClient = hc
		return nil
	}
}

// WithTimeout sets the timeout applied to each request, including
// retries. The default is 30s. It is applied through
// http.Client.Timeout and is ignored when WithHTTPClient supplies a
// custom client.
func WithTimeout(d time.Duration) Option {
	return func(c *config) error {
		if d <= 0 {
			return fmt.Errorf("mediastack: timeout must be positive, got %s", d)
		}
		c.timeout = d
		return nil
	}
}

// WithBaseURL overrides the API base URL. The value must carry an http
// or https scheme and a host; a trailing slash is stripped. Intended for
// tests and proxies.
//
// Warning: passing an http:// URL sends the access key unencrypted.
func WithBaseURL(rawURL string) Option {
	return func(c *config) error {
		u, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil {
			return fmt.Errorf("mediastack: invalid base URL: %w", err)
		}
		if u.Scheme != "https" && u.Scheme != "http" {
			return fmt.Errorf("mediastack: base URL must use http or https, got %q", u.Scheme)
		}
		if u.Hostname() == "" {
			return errors.New("mediastack: base URL must include a host")
		}
		if u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
			return errors.New("mediastack: base URL must omit credentials, query and fragment")
		}
		c.baseURL = strings.TrimRight(u.Scheme+"://"+u.Host+u.EscapedPath(), "/")
		return nil
	}
}

// WithUserAgent overrides the User-Agent header. The default is
// UserAgent(), for example
// "mediastack-go/1.0.0 (+https://github.com/tigusigalpa/mediastack-go)".
func WithUserAgent(ua string) Option {
	return func(c *config) error {
		if strings.TrimSpace(ua) == "" {
			return errors.New("mediastack: user agent must not be empty")
		}
		c.userAgent = ua
		return nil
	}
}

// WithRetry replaces the default retry policy. Zero fields of p inherit
// the defaults; see DefaultRetryPolicy.
func WithRetry(p RetryPolicy) Option {
	return func(c *config) error {
		if math.IsNaN(p.Jitter) || p.Jitter < 0 || p.Jitter > 1 {
			return fmt.Errorf("mediastack: retry jitter must be in [0,1], got %g", p.Jitter)
		}
		if p.BaseDelay < 0 || p.MaxDelay < 0 {
			return errors.New("mediastack: retry delays must not be negative")
		}
		c.retry = p.withDefaults()
		c.noRetry = false
		return nil
	}
}

// WithNoRetry disables automatic retries entirely: every request is
// attempted exactly once.
func WithNoRetry() Option {
	return func(c *config) error {
		c.retry.MaxAttempts = 1
		c.noRetry = true
		return nil
	}
}

// WithRateLimit enables a client-side token-bucket limiter of perSecond
// requests per second with a burst allowance of burst requests. APILayer
// publishes monthly quotas but no per-second limit, so the limiter is
// off by default; enable it to protect your own quota or to throttle a
// fleet of workers.
//
// A zero rate disables the limiter. Rates must be finite and
// non-negative. A burst below 1 is treated as 1.
func WithRateLimit(perSecond float64, burst int) Option {
	return func(c *config) error {
		if perSecond < 0 || math.IsNaN(perSecond) || math.IsInf(perSecond, 0) {
			return fmt.Errorf("mediastack: rate must be finite and non-negative, got %g", perSecond)
		}
		c.ratePerSecond = perSecond
		c.rateBurst = burst
		return nil
	}
}

// WithLogger installs a slog logger. A nil logger restores the default
// silence. The client logs at Debug for responses and at Warn for
// retries and unexpected content types; it never logs at
// Error (errors are returned, not logged) and never logs the access key.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) error {
		if l == nil {
			c.logger = slog.New(discardHandler{})
			return nil
		}
		c.logger = l
		return nil
	}
}

// WithMaxResponseBytes caps the size of a decoded response body. Larger
// responses fail with ErrResponseTooLarge. The default is 10 MiB.
func WithMaxResponseBytes(n int64) Option {
	return func(c *config) error {
		if n <= 0 || n == math.MaxInt64 {
			return fmt.Errorf("mediastack: max response bytes must be between 1 and %d, got %d", int64(math.MaxInt64-1), n)
		}
		c.maxResponseBytes = n
		return nil
	}
}

// WithStrictEnums enables client-side checks of category, language and
// country values against the documented sets. It is off by default
// because the API may support more values than this SDK knows; without
// it, unknown values reach the server and surface as validation_error.
func WithStrictEnums() Option {
	return func(c *config) error {
		c.strictEnums = true
		return nil
	}
}

// WithClock substitutes the clock used for response timestamps and for
// evaluating HTTP-date Retry-After headers. Intended for tests.
func WithClock(fn func() time.Time) Option {
	return func(c *config) error {
		if fn == nil {
			return errors.New("mediastack: clock must not be nil")
		}
		c.clock = fn
		return nil
	}
}

// WithRateLimitObserver registers a callback invoked after every
// response that carries quota headers. It runs synchronously, outside
// the client's lock. Concurrent requests may invoke it concurrently.
func WithRateLimitObserver(fn func(RateLimit)) Option {
	return func(c *config) error {
		c.rateObserver = fn
		return nil
	}
}

// withSleep substitutes the sleeper used for retry backoff and limiter
// waits. It is unexported and exists for tests inside this package.
func withSleep(fn func(ctx context.Context, d time.Duration) error) Option {
	return func(c *config) error {
		if fn == nil {
			return errors.New("mediastack: sleeper must not be nil")
		}
		c.sleep = fn
		return nil
	}
}
