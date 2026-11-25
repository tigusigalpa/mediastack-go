# Mediastack Golang Client/SDK/Library

Production-ready Go SDK for the [Mediastack](https://mediastack.com) news API — live and historical articles from 7,500+ sources, with typed filters, pagination, retries and rate limiting. **Zero dependencies.**

Namespace: `Tigusigalpa/Mediastack` · Module: `github.com/tigusigalpa/mediastack-go` · Package: `mediastack`

[![CI](https://github.com/tigusigalpa/mediastack-go/actions/workflows/ci.yml/badge.svg)](https://github.com/tigusigalpa/mediastack-go/actions/workflows/ci.yml)
[![Tests](https://github.com/tigusigalpa/mediastack-go/actions/workflows/test.yml/badge.svg)](https://github.com/tigusigalpa/mediastack-go/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tigusigalpa/mediastack-go.svg)](https://pkg.go.dev/github.com/tigusigalpa/mediastack-go)
[![CodeQL](https://github.com/tigusigalpa/mediastack-go/actions/workflows/codeql.yml/badge.svg)](https://github.com/tigusigalpa/mediastack-go/actions/workflows/codeql.yml)
[![codecov](https://codecov.io/gh/tigusigalpa/mediastack-go/branch/main/graph/badge.svg)](https://codecov.io/gh/tigusigalpa/mediastack-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-%E2%89%A5%201.23-007d9c)](go.mod)
[![Latest Release](https://img.shields.io/github/v/release/tigusigalpa/mediastack-go)](https://github.com/tigusigalpa/mediastack-go/releases)

Mediastack aggregates worldwide news, headlines and blog articles — live and historical — and returns them in a lightweight JSON format through two GET endpoints. This SDK wraps the entire documented surface of the API in an idiomatic, fully typed, single-package library. It is unofficial and maintained independently of APILayer.

## Features

- **Complete API coverage** — both documented endpoints, `/v1/news` and `/v1/sources`, with every documented parameter.
- **Typed filters** — `Filter[T]` models include/exclude lists (`cnn,-bbc`) so callers never touch wire encoding.
- **Lazy pagination** — `iter.Seq2` iterators that fetch pages only as you consume them, plus `Collect` and manual offset helpers.
- **Dual-format error decoding** — the live API, the OpenAPI contract and the validation endpoint emit three different error envelopes; all normalize into one `*APIError`.
- **Safe retries** — exponential backoff with jitter and `Retry-After` support; quota-exhausted 429s are correctly *not* retried.
- **Optional rate limiter** — a dependency-free token bucket plus parsing of APILayer `X-RateLimit-*` quota headers.
- **Key safety** — the access key is redacted from logs, error strings and `%v` output; it travels only in the `access_key` query parameter.
- **Zero dependencies** — standard library only; `go.mod` has no `require` block.
- **Fully tested** — unit tests run without network; integration tests live behind a build tag.

## Table of Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Choosing a Service](#choosing-a-service)
- [Recipes](#recipes)
- [Configuration](#configuration)
- [Service Map](#service-map)
- [Method Usage](#method-usage)
- [Reading Responses](#reading-responses)
- [Params & Validation](#params--validation)
- [Pagination](#pagination)
- [Error Handling](#error-handling)
- [Retries & Rate Limits](#retries--rate-limits)
- [Plans & Limits](#plans--limits)
- [Concurrency](#concurrency)
- [Using the SDK in a Service](#using-the-sdk-in-a-service)
- [Testing](#testing)
- [Security](#security)
- [Troubleshooting](#troubleshooting)
- [Examples](#examples)
- [Endpoint Documentation](#endpoint-documentation)
- [Namespace & Identity](#namespace--identity)
- [Documentation Discrepancies](#documentation-discrepancies)
- [Contributing](#contributing)
- [License](#license)
- [Disclaimer](#disclaimer)

## Requirements

- **Go 1.23+** (uses `iter.Seq2`, `slog`, `math/rand/v2`).
- A **Mediastack access key** — register at [mediastack.com](https://mediastack.com) (free plan available), then find the key in your [dashboard](https://mediastack.com/dashboard).

## Installation

```bash
go get github.com/tigusigalpa/mediastack-go
```

```go
import mediastack "github.com/tigusigalpa/mediastack-go"
```

API documentation on pkg.go.dev: [pkg.go.dev/github.com/tigusigalpa/mediastack-go](https://pkg.go.dev/github.com/tigusigalpa/mediastack-go)

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	mediastack "github.com/tigusigalpa/mediastack-go"
)

func main() {
	client, err := mediastack.NewClient(os.Getenv(mediastack.EnvAccessKey))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	resp, err := client.News.List(context.Background(), &mediastack.NewsParams{
		Limit: 3, // stay frugal: the free plan allows 100 requests/month
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range resp.Data {
		fmt.Println(a.PublishedAt.Format("2006-01-02"), a.Source, "—", a.Title)
	}
}
```

```bash
export MEDIASTACK_ACCESS_KEY=your_key
go run .
```

## Choosing a Service

| You want | Service |
| --- | --- |
| Live or historical news articles, filtered by keyword/source/category/country/language/date | `client.News` (`GET /v1/news`) |
| Discover which sources exist, their IDs, categories and sites | `client.Sources` (`GET /v1/sources`) |

Typical workflow: use `client.Sources.Search(ctx, "bbc", nil)` to learn that the identifier is `bbc`, then filter articles with `NewsParams.Sources = Filter[string]{Include: []string{"bbc"}}`.

## Recipes

### News on a topic

```go
resp, err := client.News.List(ctx, &mediastack.NewsParams{
	Keywords: mediastack.Filter[string]{Include: []string{"artificial intelligence"}},
	Limit:    10,
})
```

### Only articles with images

```go
withImage := true
resp, err := client.News.List(ctx, &mediastack.NewsParams{
	OnlyWithImage: &withImage, // nil would omit the parameter entirely
})
```

### Exclude a source

```go
resp, err := client.News.List(ctx, &mediastack.NewsParams{
	Sources: mediastack.Filter[string]{
		Include: []string{"cnn"},
		Exclude: []string{"bbc"}, // encodes as sources=cnn,-bbc
	},
})
```

### Historical range (paid plans)

```go
from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
to := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
resp, err := client.News.Historical(ctx, from, to, &mediastack.NewsParams{Limit: 50})
```

Or with a named preset: `Date: mediastack.Preset(mediastack.DateLast7Days)`.

### Every matching article

```go
for article, err := range client.News.Iter(ctx, params) {
	if err != nil {
		return err
	}
	fmt.Println(article.Title)
}
```

### Find a source

```go
resp, err := client.Sources.Search(ctx, "bbc", &mediastack.SourceParams{
	Countries: mediastack.Filter[mediastack.Country]{Include: []mediastack.Country{mediastack.CountryGB}},
})
```

## Configuration

```go
client, err := mediastack.NewClient(key,
	mediastack.WithTimeout(15*time.Second),
	mediastack.WithRetry(mediastack.RetryPolicy{MaxAttempts: 5}),
	mediastack.WithRateLimit(5, 5),
)
```

| Option | Default | Effect |
| --- | --- | --- |
| `WithHTTPClient(hc)` | internal `*http.Client` with 30s timeout | supply your own transport/proxy; `Close` then does nothing |
| `WithTimeout(d)` | `30s` | per-request timeout via `http.Client.Timeout` |
| `WithBaseURL(u)` | `https://api.mediastack.com` | custom endpoint (tests, proxies); must be http/https with a host |
| `WithUserAgent(s)` | `mediastack-go/<ver> (+repo)` | custom `User-Agent` |
| `WithRetry(p)` | 3 attempts, 500ms base, ×2, 10s cap, 0.2 jitter | replace retry policy; zero fields inherit defaults |
| `WithNoRetry()` | — | exactly one attempt per call |
| `WithRateLimit(rps, burst)` | off | client-side token bucket (rps may be fractional, e.g. `0.5`) |
| `WithLogger(l)` | silent | `slog` logging: Debug per response, Warn per retry/limiter wait |
| `WithMaxResponseBytes(n)` | `10 MiB` | cap on decoded body size → `ErrResponseTooLarge` |
| `WithStrictEnums()` | off | validate category/language/country values client-side |
| `WithClock(fn)` | `time.Now` | test hook for timestamps and HTTP-date `Retry-After` |
| `WithRateLimitObserver(fn)` | — | callback on every response carrying quota headers |

## Service Map

| Service | Method | Endpoint | Documentation |
| --- | --- | --- | --- |
| `client.News` | `List`, `Historical`, `Iter`, `Collect` | `GET /v1/news` | [getNews](https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/API%20Endpoints/getNews) |
| `client.Sources` | `List`, `Search`, `Iter` | `GET /v1/sources` | [getSources](https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/API%20Endpoints/getSources) |

## Method Usage

```go
// NewsService
func (s *NewsService) List(ctx context.Context, params *NewsParams) (*NewsResponse, error)
func (s *NewsService) Historical(ctx context.Context, from, to time.Time, params *NewsParams) (*NewsResponse, error)
func (s *NewsService) Iter(ctx context.Context, params *NewsParams) iter.Seq2[Article, error]
func (s *NewsService) Collect(ctx context.Context, params *NewsParams, maxItems int) ([]Article, error)

// SourcesService
func (s *SourcesService) List(ctx context.Context, params *SourceParams) (*SourceResponse, error)
func (s *SourcesService) Search(ctx context.Context, term string, params *SourceParams) (*SourceResponse, error)
func (s *SourcesService) Iter(ctx context.Context, params *SourceParams) iter.Seq2[Source, error]

// Client
func NewClient(accessKey string, opts ...Option) (*Client, error)
func NewClientFromEnv(opts ...Option) (*Client, error)
func (c *Client) RateLimit() RateLimit
func (c *Client) Close() error
```

Notes:

- `nil` params are accepted everywhere except `Sources.List`/`Sources.Search`, where the API-mandated `search` term is required (`ErrMissingSearch`).
- Methods never mutate the params you pass.
- `ctx` is required: pass `context.Background()` or a real deadline. The SDK never substitutes one for you.

## Reading Responses

```go
resp, err := client.News.List(ctx, params)
resp.Pagination            // {Limit, Offset, Count, Total} + Page/TotalPages/HasMore/NextOffset/IsLast/Remaining
resp.Data                  // []Article — never nil after success
resp.Meta.StatusCode       // 200
resp.Meta.RequestID        // cf-ray — quote it to APILayer support
resp.Meta.Attempts         // HTTP attempts used (retries included)
resp.Meta.Duration         // total round-trip time
resp.Meta.RateLimit        // quota headers, see Retries & Rate Limits
```

`Article` fields map one-to-one to the [documented schema](https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/Article). Nullable fields (`author`, `description`, `image`) decode JSON `null` to `""` — no pointers to check. `PublishedAt` parses tolerantly: RFC 3339 with or without timezone/fraction, `YYYY-MM-DD HH:MM:SS`, `YYYY-MM-DDTHH:MM:SS`, and bare dates (all treated as UTC); an unparseable value yields `time.Time{}` rather than failing the whole page.

Helpers: `a.HasImage()`, `a.ImageURL()`, `a.LinkURL()`, `a.IsZero()`, `a.String()`.

`Source.ID` is populated from either `id` or `code` — the API uses both spellings across documentation and responses; the SDK folds them into one field. That ID is the value to pass to `NewsParams.Sources`.

## Params & Validation

```go
type NewsParams struct {
	Keywords      Filter[string]   // full-text, phrases with spaces OK
	Sources       Filter[string]   // source IDs (from Sources endpoint)
	Categories    Filter[Category]
	Countries     Filter[Country]
	Languages     Filter[Language]
	Date          DateFilter       // OnDay / Between / Preset / ParseDateFilter
	Sort          SortOrder        // published_desc (default) | published_asc | popularity
	Limit         int              // 1..100; 0 = omit → API applies 25
	Offset        int              // >= 0; offset+limit <= 10000
	OnlyWithImage *bool            // nil = omit; &true / &false send explicitly
	Extra         url.Values       // escape hatch, applied last, overrides everything
}
```

`SourceParams` is the same minus `Keywords`, `Sources`, `Date`, `Sort`, `OnlyWithImage`, and plus a required `Search` term.

**Validated client-side:** `limit`/`offset` ranges, `offset+limit ≤ 10000`, the three `sort` values, empty/comma-containing filter elements, malformed `date` filters, missing `search` for `/v1/sources`, and a valid `access_key`/`baseURL`.

**Deliberately not validated** (unless `WithStrictEnums`): whether a category/language/country is in the documented set. The API may know more than this SDK; unknown values travel to the server and come back as a clear `validation_error`. See [Documentation Discrepancies](#documentation-discrepancies).

`Filter[T]`:

```go
type Filter[T ~string] struct {
	Include []T // all values must be non-empty and contain no commas
	Exclude []T // encoded after includes, each prefixed with "-"
}
```

`DateFilter`:

```go
mediastack.OnDay(t)          // "2024-05-01"
mediastack.Between(from, to) // "2024-05-01,2024-05-31" (inclusive, UTC)
mediastack.Preset(mediastack.DateLast7Days) // "last7days"
mediastack.ParseDateFilter("2024-05-01,2024-05-31")
```

Presets: `today`, `thisweek`, `thismonth`, `last7days`, `last30days`, `last3months`, `yeartodate`.

## Pagination

The API paginates with `limit` (≤ 100) and `offset`. **Only the first 10,000 matching records are reachable**: `offset + limit > 10000` is rejected, and `total` may exceed the window.

Three styles:

```go
// 1. Lazy iterator — fetches on demand, stops on first error,
//    yields ErrMaxOffset once if the window would be exceeded.
for a, err := range client.News.Iter(ctx, params) { ... }

// 2. Collect — bounded gather with efficient page sizes (min(100, remaining)).
articles, err := client.News.Collect(ctx, params, 250) // → requests of 100, 100, 50

// 3. Manual — you own the offset.
for {
	resp, err := client.News.List(ctx, params)
	if err != nil { return err }
	for _, a := range resp.Data { fmt.Println(a.Title) }
	if !resp.Pagination.HasMore() { break }
	params.Offset = resp.Pagination.NextOffset()
}
```

`Pagination` helpers: `Page()`, `TotalPages()`, `HasMore()`, `NextOffset()`, `IsLast()`, `Remaining()`. `HasMore` is computed as `offset+count < total`, so a short last page never triggers a wasted, quota-consuming request.

## Error Handling

Two mechanisms:

```go
var apiErr *mediastack.APIError
switch {
case errors.As(err, &apiErr):
	fmt.Println(apiErr.HTTPStatus, apiErr.Code, apiErr.Message, apiErr.RequestID)
case errors.Is(err, mediastack.ErrRateLimitReached):
	// slow down
}
```

`APIError.Unwrap` returns the matching sentinel, so `errors.Is` works on every API failure.

| API `type` | Numeric | HTTP | Sentinel | Retried |
| --- | --- | --- | --- | --- |
| `missing_access_key`, `invalid_access_key` | 101 | 401 | `ErrInvalidAccessKey` | no |
| `inactive_user` | 102 | 401 | `ErrInactiveUser` | no |
| `account_on_hold` | 107 | 401 | `ErrAccountOnHold` | no |
| `api_access_blocked`, `https_access_restricted`, `function_access_restricted` | 104/105 | 403 | `ErrAccessRestricted` | no |
| `invalid_api_function` | 103 | 404 | `ErrInvalidFunction` | no |
| `404_not_found` | 404 | 404 | `ErrNotFound` | no |
| `validation_error` | — | 422 | `ErrValidation` | no |
| `usage_limit_reached`, `daily_usage_limit_reached`, `fair_use_limit_reached` | 104 | 429 | `ErrUsageLimitReached` | **no** |
| `rate_limit_reached` | 106 | 429 | `ErrRateLimitReached` | yes, honoring `Retry-After` |
| `internal_error` | 0 | 500 | `ErrInternal` | yes |
| `maintenance_mode` | 503 | 503 | `ErrMaintenance` | yes |

Client-side sentinels: `ErrMissingAccessKey`, `ErrInvalidParams` (matched by `*ValidationError`), `ErrMissingSearch`, `ErrMaxOffset`, `ErrResponseTooLarge`, `ErrEmptyResponse`.

Helpers: `mediastack.IsTimeout(err)`, `mediastack.IsTemporary(err)`, `apiErr.Temporary()`, `apiErr.ValidationContext()` (e.g. `date=[NO_SUCH_CHOICE_ERROR]`).

**Important:** the API emits *three* error envelopes (see [Documentation Discrepancies](#documentation-discrepancies)). Always switch on `apiErr.Code` / sentinels, never on `apiErr.NumericCode` — `104` alone covers three different 429 conditions.

## Retries & Rate Limits

Default policy: 3 attempts, exponential backoff `500ms × 2ⁿ` capped at 10s, ±20% jitter, `Retry-After` (seconds or HTTP-date) takes precedence but is still capped. Retried: transport errors, 5xx, and 429 `rate_limit_reached`. Never retried: 4xx (incl. quota-429s), decode failures, `context.Canceled`/`DeadlineExceeded`. All endpoints are idempotent GETs, which is what makes automatic retries safe.

```go
mediastack.WithRetry(mediastack.RetryPolicy{
	MaxAttempts: 5, BaseDelay: time.Second, MaxDelay: 30 * time.Second, Jitter: 0.3,
	RetryOn: func(resp *http.Response, err error) bool { /* custom */ },
})
mediastack.WithNoRetry()
```

APILayer reports quota through `X-RateLimit-Limit-Day`, `X-RateLimit-Limit-Month`, `X-RateLimit-Remaining-Day`, `X-RateLimit-Remaining-Month` ([reference](https://marketplace.apilayer.com/docs/article/rate-limits)). They are not present on every response; check `RateLimit.Present`. Latest snapshot: `client.RateLimit()` or `ResponseMeta.RateLimit`; callback: `WithRateLimitObserver`.

Client-side pacing is **off by default** (APILayer publishes no per-second limit for Mediastack): `WithRateLimit(2, 2)` enables a standard-library token bucket.

## Plans & Limits

| Plan | Requests/month | Live data | History | Commercial use |
| --- | --- | --- | --- | --- |
| Free | 100 | delayed ~30 min | no | no |
| Standard ($24.99/mo) | 10,000 | yes | yes | yes |
| Professional ($99.99/mo) | 50,000 | yes | yes | yes |
| Business ($249.99/mo) | 250,000 | yes | yes | yes |
| Enterprise | custom | yes | yes | yes |

See [pricing](https://mediastack.com/pricing), [FAQ](https://mediastack.com/faq) and [billing overages](https://mediastack.com/billing-overages-documentation). Caveats that affect SDK behavior:

- **Free plan:** news arrive with a ~30-minute delay and historical `date` queries are rejected by the server.
- **One query may consume more than one quota unit** ([FAQ](https://mediastack.com/faq)) — the SDK never promises "one page = one call".
- **Overage billing** on paid plans means `usage_limit_reached` is permanent, not transient — it is never retried.
- Quota notifications fire at 75%, 90%, 100% — watch `client.RateLimit()` or use `WithRateLimitObserver`.

## Concurrency

`Client` is safe for concurrent use by multiple goroutines. Create one per application and share it; the internal limiter (when enabled), the quota snapshot, and the retry policy are all coordinated internally. Every method takes an explicit `context.Context` — the client stores none and starts no background goroutine.

## Using the SDK in a Service

```go
func run(ctx context.Context) error {
	client, err := mediastack.NewClientFromEnv(
		mediastack.WithTimeout(10*time.Second),
		mediastack.WithRateLimit(1, 1), // be gentle with a monthly quota
	)
	if err != nil {
		return err
	}
	defer client.Close()

	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			resp, err := client.News.List(reqCtx, &mediastack.NewsParams{Limit: 5})
			cancel()
			if err != nil {
				if mediastack.IsTemporary(err) {
					continue // transient — next tick will retry
				}
				return err
			}
			// process resp.Data...
		}
	}
}
```

## Testing

```bash
go test ./...                          # unit tests, no network needed
go test -race -count=1 ./...           # with the race detector
go test -cover ./...                   # coverage (target: >= 90%)
MEDIASTACK_ACCESS_KEY=... \
  go test -tags=integration -count=1 ./...   # live API smoke tests
make check                             # fmt + vet + build + race + lint
```

Integration tests skip when the key is missing and treat 429 as a skip, not a failure.

## Security

- Keep the access key **server-side only** — never in browsers, mobile apps, or public repositories. This is an explicit warning from the vendor documentation as well.
- The key travels only as the `access_key` query parameter — never in `Authorization` or any other header.
- The SDK redacts `access_key` from error strings, `%+v` dumps, `APIError.Body`, `APIError.Message` and every log line.
- If a key leaks, rotate it in the [dashboard](https://mediastack.com/dashboard).
- `WithBaseURL("http://...")` sends the key unencrypted — use it only for local test servers.
- Report SDK vulnerabilities via `SECURITY.md`, not public issues.

## Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `401 invalid_access_key` | bad/missing key; or any request with a bad key — auth is checked **before** routing, so even wrong paths return 401 | verify key in [dashboard](https://mediastack.com/dashboard); don't infer "endpoint exists" from status codes |
| `429 usage_limit_reached` | monthly quota exhausted (permanent for the period) | upgrade plan or wait; not retried |
| `429 rate_limit_reached` | too many requests per second | retried automatically; or enable `WithRateLimit` |
| `422 validation_error` on `date` | bad preset/range, or `date` on a plan without history | check `apiErr.Context` for the failing field |
| `data` comes back empty | filters too narrow; Free plan 30-min delay | relax filters; note live delay on Free |
| Historical data never arrives | Free plan has no history | paid plan required; server decides, SDK relays `ErrAccessRestricted` |
| `mediastack: pagination window exceeded` | `offset+limit > 10000` | narrow the query; only first 10k records are reachable |
| `403 function_access_restricted` | feature not in your plan | check plan features |

## Examples

Self-contained programs under [`examples/`](examples/README.md): `basic`, `filters`, `pagination`, `historical`, `sources`, `errors`, `ratelimit`. Run with `MEDIASTACK_ACCESS_KEY=... go run ./examples/<name>`.

## Endpoint Documentation

### GET /v1/news — Live News

Returns live and historical news articles from thousands of sources worldwide.

- **Documentation:** https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/API%20Endpoints/getNews
- **Semantics:** https://docs.apilayer.com/mediastack/docs/api-features#live-news · https://docs.apilayer.com/mediastack/docs/api-features#historical-news
- **Response schema:** https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/NewsResponse
- **SDK methods:** `client.News.List`, `client.News.Historical`, `client.News.Iter`, `client.News.Collect`
- **Response codes:** 200, 401, 403, 404, 422, 429, 500, 503

| Parameter | Type | Required | Default | SDK field |
| --- | --- | --- | --- | --- |
| `access_key` | string | yes | — | client constructor arg |
| `sources` | string (csv, `-` excludes) | no | — | `Sources Filter[string]` |
| `categories` | string (csv, `-` excludes) | no | — | `Categories Filter[Category]` |
| `countries` | string (csv, `-` excludes) | no | — | `Countries Filter[Country]` |
| `languages` | string (csv, `-` excludes) | no | — | `Languages Filter[Language]` |
| `keywords` | string (csv, `-` excludes) | no | — | `Keywords Filter[string]` |
| `date` | string | no | — | `Date DateFilter` |
| `sort` | string | no | `published_desc` | `Sort SortOrder` |
| `only_with_image` | boolean | no | `false` | `OnlyWithImage *bool` |
| `limit` | integer 1–100 | no | `25` | `Limit int` |
| `offset` | integer ≥ 0 | no | `0` | `Offset int` |

### GET /v1/sources — News Sources

Searches and filters news sources and their metadata.

- **Documentation:** https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/API%20Endpoints/getSources
- **Semantics:** https://docs.apilayer.com/mediastack/docs/api-features#news-sources
- **Response schema:** https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/SourceResponse
- **SDK methods:** `client.Sources.List`, `client.Sources.Search`, `client.Sources.Iter`
- **Response codes:** 200, 401, 403, 404, 422, 429, 500, 503

| Parameter | Type | Required | Default | SDK field |
| --- | --- | --- | --- | --- |
| `access_key` | string | yes | — | client constructor arg |
| `search` | string | **yes** | — | `Search string` |
| `categories` | string (csv) | no | — | `Categories Filter[Category]` |
| `countries` | string (csv) | no | — | `Countries Filter[Country]` |
| `languages` | string (csv) | no | — | `Languages Filter[Language]` |
| `limit` | integer 1–100 | no | `25` | `Limit int` |
| `offset` | integer ≥ 0 | no | `0` | `Offset int` |

### Response Schemas

- `Article`: https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/Article
- `Source`: https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/Source
- `Pagination`: https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/Pagination
- `ApiError` / `ApiError_error`: https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/ApiError · https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/ApiError_error

### More documentation

- API overview: https://docs.apilayer.com/mediastack/docs/api-documentation
- Quickstart: https://docs.apilayer.com/mediastack/docs/quickstart-guide
- Getting started (auth, HTTPS, errors): https://docs.apilayer.com/mediastack/docs/getting-started
- Options (filters): https://docs.apilayer.com/mediastack/docs/options
- Code examples: https://docs.apilayer.com/mediastack/docs/code-examples
- Playground: https://docs.apilayer.com/mediastack/docs/playground
- Sources & countries list: https://mediastack.com/sources
- Status page: https://status.mediastack.com
- APILayer product page: https://apilayer.com/marketplace/mediastack-api
- Official OpenAPI contract (snapshot): [`api/openapi.json`](api/openapi.json) · live: `docs.apilayer.com/services/api/published-definition?...`

## Namespace & Identity

Go has no `namespace` keyword like PHP or C#. The public identity **Tigusigalpa/Mediastack** is expressed in three consistent ways:

1. The `mediastack.Namespace` constant (`"Tigusigalpa/Mediastack"`), next to `ModulePath`, `DefaultBaseURL`, `APIVersion`.
2. Documentation — this README, `doc.go`, and the license header.
3. The module path `github.com/tigusigalpa/mediastack-go` with package name `mediastack` (lowercase, single word — Go convention), used as `mediastack.NewClient(...)`.

## Documentation Discrepancies

The vendor documentation is internally inconsistent in several places, verified against the live API and the official OpenAPI contract. The SDK resolves each as follows (and there is a test for every one):

1. **Three error formats.** Live responses use `{"error":{"code":"type","message":...}}` (string code), the OpenAPI contract documents `{"success":false,"error":{"code":101,"type":"...","info":...}}` (numeric code), and 422s add a `context` map. `decodeAPIError` accepts all three; `APIError.Code` prefers `type`, falls back to string `code`, then to unambiguous numeric mapping, then to the HTTP status.
2. **Ambiguous numeric codes.** `104` covers `api_access_blocked`, `usage_limit_reached`, `daily_usage_limit_reached`, `fair_use_limit_reached`; `101` covers missing *and* invalid key; `105` covers two restrictions. The SDK never maps ambiguous numbers to a `Code`.
3. **`politics` category.** Present on the API Features page, absent from OpenAPI. Exposed as `CategoryPolitics` and included in `Categories()`; the client does not block it.
4. **Swedish: `se` vs `sv`.** The Options page lists `se` (the country code for Sweden); the correct language code is `sv` per API Features. `LanguageSwedish = "sv"`, plus `LanguageSwedishLegacy = "se"` for compatibility.
5. **Hindi `hi`.** Listed on API Features but missing from Options, which documents 13 languages and `hi` would make 14. `LanguageHindi` exists but is kept out of the canonical `Languages()` list.
6. **Source id: `id` vs `code`.** OpenAPI says `code`, live responses and examples use `id`. `Source.UnmarshalJSON` reads both into a single `ID` field — there is no separate `Code` field to confuse callers.
7. **`search` on `/v1/sources`.** Required per OpenAPI and API Features ("the request is rejected without it"). The SDK enforces it client-side (`ErrMissingSearch`) to save the wasted quota.
8. **`only_with_image`.** In OpenAPI but absent from API Features/Options pages. Implemented as a full typed parameter.
9. **`HEAD` unsupported.** `HEAD /v1/news` returns a Cloudflare 403 HTML page — the SDK only issues `GET`.
10. **Brotli responses.** The server may answer `br`-encoded if asked; the SDK never sends `Accept-Encoding: br` (Go's transport handles gzip automatically, not brotli).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). In short: `make check` must pass, new behavior needs a failing test first, exported symbols need godoc, commits follow Conventional Commits.

## License

[MIT](LICENSE) — Copyright (c) 2026 Igor Sazonov ([tigusigalpa](https://github.com/tigusigalpa), sovletig@gmail.com).

## Disclaimer

This is an **unofficial** SDK maintained independently of [APILayer](https://apilayer.com) / Mediastack. "Mediastack" is a product name of APILayer; this repository provides a Go client for its public API and is not endorsed by or affiliated with the vendor.
