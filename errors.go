package mediastack

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Client-side and API sentinel errors. APIError.Unwrap returns one of
// the API sentinels, so errors.Is works against them; client-side
// failures (before any network call) are returned directly or wrapped in
// *ValidationError.
var (
	// ErrMissingAccessKey is returned by NewClient when the access key
	// is empty or contains only whitespace.
	ErrMissingAccessKey = errors.New("mediastack: missing API access key")

	// ErrInvalidParams is matched by every *ValidationError through
	// errors.Is.
	ErrInvalidParams = errors.New("mediastack: invalid parameters")

	// ErrMissingSearch is returned when GET /v1/sources is called
	// without the required search term.
	ErrMissingSearch = errors.New("mediastack: sources search term is required")

	// ErrMaxOffset is returned when offset+limit would exceed the
	// MaxPaginationWindow (10,000) enforced by the API.
	ErrMaxOffset = errors.New("mediastack: pagination window exceeded")

	// ErrInvalidPagination is returned when a response claims more results
	// are available without advancing the page offset.
	ErrInvalidPagination = errors.New("mediastack: pagination did not advance")

	// ErrResponseTooLarge is returned when a response body exceeds the
	// configured size limit (WithMaxResponseBytes).
	ErrResponseTooLarge = errors.New("mediastack: response body exceeds size limit")

	// ErrEmptyResponse is returned when the API answers success with an
	// empty body.
	ErrEmptyResponse = errors.New("mediastack: empty response body")

	// ErrInvalidAccessKey matches missing_access_key and
	// invalid_access_key (HTTP 401).
	ErrInvalidAccessKey = errors.New("mediastack: invalid API access key")

	// ErrInactiveUser matches inactive_user (HTTP 401).
	ErrInactiveUser = errors.New("mediastack: account is inactive")

	// ErrAccountOnHold matches account_on_hold (HTTP 401).
	ErrAccountOnHold = errors.New("mediastack: account is on hold")

	// ErrAccessRestricted matches api_access_blocked,
	// https_access_restricted and function_access_restricted (HTTP 403).
	ErrAccessRestricted = errors.New("mediastack: function access restricted by plan")

	// ErrInvalidFunction matches invalid_api_function (HTTP 404).
	ErrInvalidFunction = errors.New("mediastack: invalid API function")

	// ErrNotFound matches 404_not_found and any other HTTP 404.
	ErrNotFound = errors.New("mediastack: resource not found")

	// ErrValidation matches validation_error (HTTP 422).
	ErrValidation = errors.New("mediastack: validation error")

	// ErrUsageLimitReached matches usage_limit_reached,
	// daily_usage_limit_reached and fair_use_limit_reached (HTTP 429).
	// These errors are permanent for the billing period and are never
	// retried.
	ErrUsageLimitReached = errors.New("mediastack: usage limit reached")

	// ErrRateLimitReached matches rate_limit_reached (HTTP 429). This
	// is the only retryable 429 variant.
	ErrRateLimitReached = errors.New("mediastack: rate limit reached")

	// ErrInternal matches internal_error and other HTTP 5xx failures.
	ErrInternal = errors.New("mediastack: internal server error")

	// ErrMaintenance matches maintenance_mode (HTTP 503).
	ErrMaintenance = errors.New("mediastack: service unavailable")
)

// APIError is a normalized representation of every error format the API
// produces. The live API, the OpenAPI contract and the validation
// endpoint emit three different envelopes; decodeAPIError folds all of
// them into this single type.
type APIError struct {
	// HTTPStatus is the HTTP status code of the response.
	HTTPStatus int

	// Code is error.type, a string error.code, or a known numeric code's
	// name. It can be empty when only the HTTP status is available.
	Code string

	// NumericCode is the apilayer numeric error code when the response
	// carried one (format B). Numbers are ambiguous — 104 alone covers
	// three different 429 conditions — so they are informational only.
	NumericCode int

	// Message is the human-readable error detail.
	Message string

	// Context carries per-field validation details for 422 responses,
	// for example {"date": ["NO_SUCH_CHOICE_ERROR"]}.
	Context map[string][]string

	// RequestID is the cf-ray (or X-Request-Id) value of the failed
	// request. Quote it when contacting APILayer support.
	RequestID string

	// RetryAfter is the parsed Retry-After header, when present.
	RetryAfter time.Duration

	// Body is the raw response body, truncated to a reasonable size and
	// with any access_key value redacted.
	Body []byte
}

// apiErrorBodyLimit caps how much of an error body is retained in
// APIError.Body.
const apiErrorBodyLimit = 4096

// rawErrorEnvelope accepts every documented error shape at once:
//
//	{"error": {"code": "invalid_access_key", "message": "..."}}      // format A
//	{"success": false, "error": {"code": 101, "type": "...", "info": "..."}}  // format B
//	{"error": {"code": "validation_error", "message": "...", "context": {...}}}  // format C
type rawErrorEnvelope struct {
	Error *struct {
		Code    json.RawMessage     `json:"code"`
		Type    string              `json:"type"`
		Info    string              `json:"info"`
		Message string              `json:"message"`
		Context map[string][]string `json:"context"`
	} `json:"error"`
}

// decodeAPIError normalizes an error response body into *APIError.
// Non-JSON bodies (for example Cloudflare HTML on 403) still produce a
// usable error built from the status line.
func decodeAPIError(status int, id string, body []byte, key string) *APIError {
	e := &APIError{
		HTTPStatus: status,
		RequestID:  id,
		Body:       []byte(truncate(redactKey(string(body), key), apiErrorBodyLimit)),
	}
	var env rawErrorEnvelope
	if json.Unmarshal(body, &env) != nil || env.Error == nil {
		e.Message = http.StatusText(status)
		if e.Message == "" {
			e.Message = "request failed"
		}
		return e
	}
	re := env.Error

	// Resolve the machine code. Priority: error.type > string error.code
	// > unambiguous numeric code mapping.
	code, numeric := parseRawCode(re.Code)
	switch {
	case re.Type != "":
		e.Code = re.Type
	case code != "":
		e.Code = code
	case numeric != nil:
		e.Code = codeForNumeric(*numeric)
	}
	if numeric != nil {
		e.NumericCode = *numeric
	}
	if sentinelFor(e.Code, 0) == nil {
		e.Code = redactKey(e.Code, key)
	}

	e.Message = re.Message
	if e.Message == "" {
		e.Message = re.Info
	}
	if e.Message == "" {
		e.Message = http.StatusText(status)
	}
	// A misbehaving upstream can echo the access key inside the message;
	// scrub it so error strings never leak credentials.
	e.Message = redactKey(e.Message, key)
	if len(re.Context) > 0 {
		e.Context = make(map[string][]string, len(re.Context))
		for field, values := range re.Context {
			for i, value := range values {
				values[i] = redactKey(value, key)
			}
			e.Context[redactKey(field, key)] = values
		}
	}
	return e
}

// parseRawCode reads error.code which may be a JSON string or number.
func parseRawCode(raw json.RawMessage) (code string, numeric *int) {
	if len(raw) > 0 && raw[0] == '"' {
		_ = json.Unmarshal(raw, &code)
		return code, nil
	}
	if json.Unmarshal(raw, &numeric) != nil {
		return "", nil
	}
	return "", numeric
}

// codeForNumeric maps unambiguous numeric codes to their documented
// type. Ambiguous numbers (101, 104, 105 cover several types) return
// the empty string; callers then fall back to the HTTP status.
func codeForNumeric(n int) string {
	switch n {
	case 0:
		return "internal_error"
	case 102:
		return "inactive_user"
	case 103:
		return "invalid_api_function"
	case 106:
		return "rate_limit_reached"
	case 107:
		return "account_on_hold"
	case 404:
		return "404_not_found"
	case 503:
		return "maintenance_mode"
	}
	return ""
}

// Error returns a single-line description of the failure, for example
// "mediastack: 429 rate_limit_reached: You have exceeded ... (request id: 8f2c...)".
// It never contains the API access key.
func (e *APIError) Error() string {
	var b strings.Builder
	b.WriteString("mediastack: ")
	b.WriteString(strconv.Itoa(e.HTTPStatus))
	code := e.Code
	if code == "" {
		code = http.StatusText(e.HTTPStatus)
	}
	if code != "" {
		b.WriteByte(' ')
		b.WriteString(code)
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	if vc := e.ValidationContext(); vc != "" {
		b.WriteString(" (validation failed: ")
		b.WriteString(vc)
		b.WriteByte(')')
	}
	if e.RequestID != "" {
		b.WriteString(" (request id: ")
		b.WriteString(e.RequestID)
		b.WriteByte(')')
	}
	return b.String()
}

// Unwrap returns the sentinel error matching Code (falling back to the
// HTTP status), enabling errors.Is checks like
// errors.Is(err, ErrRateLimitReached).
func (e *APIError) Unwrap() error {
	if e.Code == "" && e.NumericCode == 104 && e.HTTPStatus == http.StatusTooManyRequests {
		return ErrUsageLimitReached
	}
	return sentinelFor(e.Code, e.HTTPStatus)
}

// Is lets errors.Is match sentinels directly even when Unwrap is
// shadowed by wrapping.
func (e *APIError) Is(target error) bool {
	return e.Unwrap() == target
}

// Temporary reports whether the failure is worth retrying: true for
// rate_limit_reached and 5xx statuses, false for every other 4xx and for
// the quota-exhausted 429 variants. Network errors are not APIError
// values; use IsTemporary for a check that covers both.
func (e *APIError) Temporary() bool {
	if e.HTTPStatus == http.StatusTooManyRequests {
		return e.Code == "rate_limit_reached" || (e.Code == "" && e.NumericCode != 104)
	}
	return e.HTTPStatus >= 500
}

// ValidationContext renders Context deterministically, for example
// "date=[NO_SUCH_CHOICE_ERROR]; limit=[TOO_LARGE]".
func (e *APIError) ValidationContext() string {
	if len(e.Context) == 0 {
		return ""
	}
	keys := make([]string, 0, len(e.Context))
	for k := range e.Context {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(k)
		b.WriteString("=[")
		b.WriteString(strings.Join(e.Context[k], ", "))
		b.WriteByte(']')
	}
	return b.String()
}

// sentinelFor maps a machine code, then an HTTP status, to a sentinel
// error. Unknown combinations return nil.
func sentinelFor(code string, status int) error {
	switch code {
	case "missing_access_key", "invalid_access_key":
		return ErrInvalidAccessKey
	case "inactive_user":
		return ErrInactiveUser
	case "account_on_hold":
		return ErrAccountOnHold
	case "api_access_blocked", "https_access_restricted", "function_access_restricted":
		return ErrAccessRestricted
	case "invalid_api_function":
		return ErrInvalidFunction
	case "404_not_found":
		return ErrNotFound
	case "validation_error":
		return ErrValidation
	case "usage_limit_reached", "daily_usage_limit_reached", "fair_use_limit_reached":
		return ErrUsageLimitReached
	case "rate_limit_reached":
		return ErrRateLimitReached
	case "internal_error":
		return ErrInternal
	case "maintenance_mode":
		return ErrMaintenance
	}
	switch {
	case status == http.StatusUnauthorized:
		return ErrInvalidAccessKey
	case status == http.StatusForbidden:
		return ErrAccessRestricted
	case status == http.StatusNotFound:
		return ErrNotFound
	case status == http.StatusUnprocessableEntity:
		return ErrValidation
	case status == http.StatusTooManyRequests:
		return ErrRateLimitReached
	case status >= 500:
		return ErrInternal
	}
	return nil
}

// ValidationError aggregates every client-side parameter problem found
// during request encoding, keyed by parameter name. It matches
// ErrInvalidParams through errors.Is.
type ValidationError struct {
	// Fields maps a parameter name to its validation failures.
	Fields map[string][]string
}

// add appends a failure message for a field.
func (e *ValidationError) add(field, msg string) {
	if e.Fields == nil {
		e.Fields = map[string][]string{}
	}
	e.Fields[field] = append(e.Fields[field], msg)
}

// err returns e when it holds at least one failure, else nil.
func (e *ValidationError) err() error {
	if len(e.Fields) == 0 {
		return nil
	}
	return e
}

// Error renders all failures deterministically (fields sorted), for
// example "mediastack: invalid parameters: limit must be between 1 and
// 100, got 500; offset must be >= 0, got -3".
func (e *ValidationError) Error() string {
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("mediastack: invalid parameters: ")
	for i, k := range keys {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(k)
		b.WriteByte(' ')
		b.WriteString(strings.Join(e.Fields[k], "; "))
	}
	return b.String()
}

// Is reports a match with ErrInvalidParams so callers can write
// errors.Is(err, mediastack.ErrInvalidParams).
func (e *ValidationError) Is(target error) bool {
	return target == ErrInvalidParams
}

// IsTimeout reports whether err is or wraps a network timeout.
func IsTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// IsTemporary reports whether err is worth retrying. It returns false
// for context cancellation and deadlines, true for the retryable API
// failures (see APIError.Temporary) and for other transport errors.
func IsTemporary(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Temporary()
	}
	var ne net.Error
	return errors.As(err, &ne)
}

// requestID extracts a request identifier from response headers. The API
// serves traffic through Cloudflare, so cf-ray is the most reliable
// identifier; X-Request-Id is accepted as a fallback.
func requestID(h http.Header) string {
	if v := h.Get("cf-ray"); v != "" {
		return v
	}
	return h.Get("x-request-id")
}
