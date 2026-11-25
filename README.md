# Mediastack Golang Client/SDK/Library

![Mediastack Go](https://i.postimg.cc/qBX9P6sn/mediastack-go-hero-banner.jpg)

[![CI](https://github.com/tigusigalpa/mediastack-go/actions/workflows/ci.yml/badge.svg)](https://github.com/tigusigalpa/mediastack-go/actions/workflows/ci.yml)
[![Tests](https://github.com/tigusigalpa/mediastack-go/actions/workflows/test.yml/badge.svg)](https://github.com/tigusigalpa/mediastack-go/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tigusigalpa/mediastack-go.svg)](https://pkg.go.dev/github.com/tigusigalpa/mediastack-go)
[![Coverage](https://codecov.io/gh/tigusigalpa/mediastack-go/branch/main/graph/badge.svg)](https://codecov.io/gh/tigusigalpa/mediastack-go)
[![Release](https://img.shields.io/github/v/release/tigusigalpa/mediastack-go)](https://github.com/tigusigalpa/mediastack-go/releases)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Mediastack Go is an unofficial Go client for the Mediastack news API. Use it to fetch a page of recent headlines, run a historical search, or look up the source IDs behind your news feed. It uses Go's standard library and requires Go 1.23 or later.

[Quick start](#quick-start) · [Filters and historical news](#filters-and-historical-news) · [Pagination](#pagination) · [Errors and quotas](#errors-and-quotas) · [Configuration](#configuration) · [Testing](#testing)

## Quick start

Install the package in your Go module and get a Mediastack access key from your [account dashboard](https://mediastack.com/dashboard).

```sh
go get github.com/tigusigalpa/mediastack-go
```

Set `MEDIASTACK_ACCESS_KEY` before running your program.

```sh
export MEDIASTACK_ACCESS_KEY='your-key'
```

In PowerShell, use `$env:MEDIASTACK_ACCESS_KEY = 'your-key'`.

Save this as `main.go` and run `go run .`.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tigusigalpa/mediastack-go"
)

func main() {
	client, err := mediastack.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	page, err := client.News.List(ctx, &mediastack.NewsParams{Limit: 5})
	if err != nil {
		log.Fatal(err)
	}
	for _, article := range page.Data {
		fmt.Printf("%s | %s\n%s\n", article.Source, article.Title, article.URL)
	}
}
```

Use `NewClient(key, ...)` if your application already loads credentials from a secret store. Both constructors validate configuration locally. The API checks the key on the first request.

## Choose a method

| What you need | Method | HTTP endpoint |
| --- | --- | --- |
| One page of recent or filtered articles | `News.List(ctx, params)` | `GET /v1/news` |
| One page from an inclusive historical date range | `News.Historical(ctx, from, to, params)` | `GET /v1/news` |
| A stream of articles across pages | `News.Iter(ctx, params)` | `GET /v1/news` |
| A slice capped at a given number of articles | `News.Collect(ctx, params, maxItems)` | `GET /v1/news` |
| Find source IDs by name or keyword | `Sources.Search(ctx, term, params)` | `GET /v1/sources` |
| List or iterate matching sources with a search term in params | `Sources.List(ctx, params)`, `Sources.Iter(ctx, params)` | `GET /v1/sources` |

`News.List(ctx, nil)` uses the API defaults. `Sources.Search(ctx, "bbc", nil)` uses the given search term with default filters. When using `Sources.List` or `Sources.Iter`, set `SourceParams.Search`.

Every successful page has `Data`, `Pagination` and `Meta`. An empty result has an empty `Data` slice. `Meta` includes the HTTP status, request ID, attempt count, HTTP time, fetch time and any quota headers returned by the server. Use the request ID when reporting an upstream issue.

`Article.Author`, `Description` and `Image` are empty strings when the API returns null. Check `article.PublishedAt.IsZero()` before using the timestamp as a checkpoint. `article.URL` points to the original publication.

## Filters and historical news

Use `Filter[T]` to include and exclude values. This example requests English-language business headlines about a topic while excluding a related term.

```go
params := &mediastack.NewsParams{
	Keywords: mediastack.Filter[string]{
		Include: []string{"interest rates"},
		Exclude: []string{"mortgage"},
	},
	Languages: mediastack.Filter[mediastack.Language]{
		Include: []mediastack.Language{mediastack.LanguageEnglish},
	},
	Categories: mediastack.Filter[mediastack.Category]{
		Include: []mediastack.Category{mediastack.CategoryBusiness},
	},
	Sort:  mediastack.SortPublishedDesc,
	Limit: 20,
}
page, err := client.News.List(ctx, params)
```

Pass each term as its own list element. Phrases can contain spaces. The client joins the lists and prefixes exclusions with `-`. To request articles with images, set `OnlyWithImage` to a pointer to `true`. A nil pointer leaves the API default in effect.

To search by publisher, look up the ID and use it in `NewsParams.Sources.Include`. Source names displayed in articles can differ from their IDs.

```go
page, err := client.Sources.Search(ctx, "bbc", nil)
if err != nil {
	return err
}
for _, source := range page.Data {
	fmt.Printf("%s: %s\n", source.ID, source.Name)
}
```

For historical news, pass a date range with `Historical` or set `NewsParams.Date` using `OnDay`, `Between` or a named `Preset`. The SDK formats explicit dates as UTC calendar days, with both ends included.

```go
from := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
to := time.Date(2025, time.January, 7, 0, 0, 0, 0, time.UTC)
page, err := client.News.Historical(ctx, from, to, &mediastack.NewsParams{
	Sort:  mediastack.SortPublishedAsc,
	Limit: 50,
})
```

For a date entered by a user, `ParseDateFilter` accepts a day such as `2025-01-01`, a range such as `2025-01-01,2025-01-07`, or a preset such as `last7days`.

APILayer documents a 30-minute news delay on Free and historical access starting at Standard. Check the current [API features](https://docs.apilayer.com/mediastack/docs/api-features) and [pricing](https://mediastack.com/pricing/) for your account.

### Query parameters

| API parameter | News | Sources | Go field or behavior |
| --- | --- | --- | --- |
| `access_key` | Required | Required | Supplied by the client |
| `search` | | Required | `SourceParams.Search` |
| `keywords` | Optional | | `NewsParams.Keywords` |
| `sources` | Optional | | `NewsParams.Sources` with IDs from the source lookup |
| `categories`, `countries`, `languages` | Optional | Optional | `Categories`, `Countries`, `Languages` |
| `date` | Optional | | `NewsParams.Date` |
| `sort` | Optional | | `SortPublishedDesc` (default), `SortPublishedAsc` or `SortPopularity` |
| `only_with_image` | Optional | | `NewsParams.OnlyWithImage` |
| `limit`, `offset` | Optional | Optional | `Limit`, `Offset` |

The client validates typed filters, date ranges and pagination locally. Enable `WithStrictEnums()` to validate category and language values and check the shape of country codes. Use `Extra` for raw query parameters. The client always sets its own `access_key` and validates paging overrides before sending them.

## Pagination

Start with `List` for a single page. Use `Iter` to process results one at a time. Breaking out of the loop prevents the SDK from requesting additional pages.

```go
processed := 0
for article, err := range client.News.Iter(ctx, params) {
	if err != nil {
		return err
	}
	fmt.Println(article.Title)
	processed++
	if processed == 250 {
		break
	}
}
```

- The page size defaults to 25 and can be set from 1 to 100. The API accepts up to 10,000 records per query through offset pagination.
- The iterators use the returned count and total to decide when to continue. They reduce the next page size when needed to finish the 10,000-record window. If the API reports more records at that boundary, the iterator yields `ErrMaxOffset`. A stalled offset yields `ErrInvalidPagination`.
- `Collect(ctx, params, 250)` returns up to 250 articles and chooses its own page sizes. `Collect(ctx, params, 0)` fetches one page with the supplied settings. Use `Iter` if you need to keep articles processed before a later request fails.
- For a manual loop, call `page.Pagination.HasMore()` and use `NextOffset()` for the next request. Check that the offset advances and that the next request fits within the window.

For a backfill, query smaller historical intervals and persist your progress after storing each batch. A busy interval can fill the 10,000-record window by itself. If that happens on a single day, split further by source or another filter and deduplicate overlapping results.

For a live feed, new articles can shift results between requests. Poll recent pages on a schedule and deduplicate by article URL in storage. Size your polling interval, page count and retries against your request budget. One page fetch uses one API request regardless of the page size. [Mediastack FAQ](https://mediastack.com/faq)

## Errors and quotas

Use `errors.Is` for decisions and `errors.As` for details. An upstream error can include a machine code, field validation details, a request ID and a `Retry-After` duration.

```go
switch {
case errors.Is(err, mediastack.ErrUsageLimitReached):
	log.Print("Mediastack quota exhausted")
case errors.Is(err, mediastack.ErrRateLimitReached):
	log.Print("Mediastack request throttled")
case errors.Is(err, mediastack.ErrAccessRestricted):
	log.Print("Mediastack denied access to this feature")
case err != nil:
	var apiErr *mediastack.APIError
	if errors.As(err, &apiErr) {
		log.Printf("Mediastack status=%d code=%s request_id=%s",
			apiErr.HTTPStatus, apiErr.Code, apiErr.RequestID)
	}
	return err
}
```

| Error | What to check |
| --- | --- |
| `ErrMissingAccessKey` / `ErrInvalidAccessKey` | Check the environment variable and the key in your dashboard |
| `ErrMissingSearch` | Supply the query for `/v1/sources` |
| `ErrInvalidParams` | Inspect `*ValidationError.Fields` for checks performed locally |
| `ErrValidation` | Inspect `*APIError.Context` for fields rejected upstream |
| `ErrUsageLimitReached` | Check the quota for your plan before scheduling another run |
| `ErrRateLimitReached` | Review request pacing and the upstream `Retry-After` value |
| `ErrMaxOffset` | Narrow the date interval or the filters |
| `ErrInvalidPagination` | Inspect the returned offset and count before retrying a paginated job |
| `ErrResponseTooLarge` | Reduce the page size or set a response cap appropriate for your service |

By default, each SDK call makes up to three attempts. It retries eligible transport failures, HTTP 5xx and rate-limit 429s. Quota exhaustion and client-side validation failures return immediately. The default backoff starts at 500 ms, uses jitter and caps delays at 10 seconds. A usable `Retry-After` value takes precedence up to the configured cap.

The optional token bucket limits outgoing attempts per client, including retries. When the API sends quota headers, `page.Meta.RateLimit` describes that response and `client.RateLimit()` returns the latest available snapshot. Check `Present` before using the counters. `WithRateLimitObserver` runs synchronously and can be called from several goroutines at once.

## Configuration

Reuse one client across goroutines to share the connection pool and request pacing. Give each HTTP operation a context. A context deadline bounds pagination, rate-limit waits and retry backoff across the whole operation. The HTTP client timeout applies to each attempt.

```go
client, err := mediastack.NewClientFromEnv(
	mediastack.WithTimeout(10*time.Second),
	mediastack.WithRateLimit(2, 2),
	mediastack.WithRetry(mediastack.RetryPolicy{
		MaxAttempts: 3,
		MaxDelay:    5 * time.Second,
	}),
)
```

| Option | Default and use |
| --- | --- |
| `WithTimeout(d)` | 30 seconds per HTTP attempt with the SDK's client |
| `WithHTTPClient(hc)` | Supplies a transport, cookie jar and timeout through a shallow copy. The caller manages the transport's idle connections |
| `WithBaseURL(u)` | `https://api.mediastack.com`. Use a trusted HTTPS proxy or a local test server. A path prefix is supported |
| `WithRetry(policy)`, `WithNoRetry()` | Set retry limits or disable SDK retries. Zero policy fields inherit defaults |
| `WithRateLimit(rps, burst)` | Off. Enable per-client pacing, including fractional requests per second |
| `WithLogger(logger)` | Silent. A `slog.Logger` can record responses at Debug and retries or unexpected content types at Warn |
| `WithMaxResponseBytes(n)` | 10 MiB for the body read by the SDK |
| `WithStrictEnums()` | Off. Validate category, language and country values locally |
| `WithRateLimitObserver(fn)` | Off. Receive snapshots when quota headers are present |
| `WithUserAgent(s)`, `WithClock(fn)` | Override the user agent or the clock used for timing and `Retry-After` |

Mediastack authenticates with an `access_key` query parameter. Keep the key in server-side configuration and use HTTPS. The SDK redacts the key from diagnostic logs and API error details, strips the `Referer` on redirects, and follows redirects within the original scheme and host. Configure any custom transport, proxy or access logger to redact query strings too. See [SECURITY.md](SECURITY.md) and the [endpoint reference](https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0).

## Testing

```sh
go test ./...
go vet ./...
go build ./examples/...
```

Most HTTP tests use local test servers. They run without a Mediastack key. To run the optional live tests, set `MEDIASTACK_ACCESS_KEY` and run `go test -tags=integration -run '^TestIntegration' ./...`. The live requests can count toward your quota.

Run `go run ./examples/basic` with your key to try the first request. The [examples directory](examples/README.md) also covers filters, manual pagination, historical queries, source lookup and error handling.

## Reference

The [Go reference](https://pkg.go.dev/github.com/tigusigalpa/mediastack-go) lists all methods, types and constants. The [OpenAPI snapshot](api/openapi.json) documents parameters such as named date presets and `only_with_image`. The SDK accepts both `id` and `code` in source records and exposes them through `Source.ID`. It also reads string and numeric API error envelopes and preserves field-level validation details.

The import path is `github.com/tigusigalpa/mediastack-go`, the package is `mediastack` and the SDK exposes `Tigusigalpa/Mediastack` through `mediastack.Namespace`.

See [CONTRIBUTING.md](CONTRIBUTING.md) for development workflow and the [changelog](CHANGELOG.md) for release history. Mediastack Go is independently maintained and published under the [MIT license](LICENSE).
