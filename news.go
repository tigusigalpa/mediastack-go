package mediastack

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"time"
)

// NewsService provides access to the GET /v1/news endpoint: live and
// historical news articles with filtering.
//
// Endpoint documentation:
// https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/API%20Endpoints/getNews
// Live news: https://docs.apilayer.com/mediastack/docs/api-features#live-news
// Historical news: https://docs.apilayer.com/mediastack/docs/api-features#historical-news
type NewsService struct {
	c *Client
}

// List returns one page of news articles matching params. A nil params
// value is valid and means "no filters, API defaults".
//
// The response Data slice is never nil: an empty result set decodes to
// an empty slice.
func (s *NewsService) List(ctx context.Context, params *NewsParams) (*NewsResponse, error) {
	p := NewsParams{}
	if params != nil {
		p = *params
	}
	values, err := p.encode(s.c.cfg.strictEnums)
	if err != nil {
		return nil, err
	}
	body, meta, err := s.c.do(ctx, "news", values)
	if err != nil {
		return nil, err
	}
	var out NewsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("mediastack: decode news response: %w", err)
	}
	if out.Data == nil {
		out.Data = []Article{}
	}
	out.Meta = meta
	return &out, nil
}

// Historical returns one page of articles published between from and to
// (inclusive, UTC days). It is a convenience wrapper around List: params
// is cloned, never mutated, and its Date field is replaced by
// Between(from, to). A nil params value is valid.
//
// Historical data requires a paid plan; on the Free plan the API
// answers with a plan restriction error — that decision belongs to the
// server, not to this SDK.
func (s *NewsService) Historical(ctx context.Context, from, to time.Time, params *NewsParams) (*NewsResponse, error) {
	p := NewsParams{}
	if params != nil {
		p = *params
	}
	p.Date = Between(from, to)
	return s.List(ctx, &p)
}

// Iter lazily paginates through every article matching params, fetching
// the next page only when the previous one is exhausted. It performs no
// network call until the first element is requested.
//
// The iterator stops after yielding an error; a second call to next
// returns false. When the pagination window (MaxPaginationWindow) would
// be exceeded, it yields an error wrapping ErrMaxOffset once and ends —
// records beyond the window are unreachable through offset pagination.
// A nil params value iterates the first page of unfiltered results.
//
//	for a, err := range client.News.Iter(ctx, params) {
//		if err != nil {
//			return err
//		}
//		fmt.Println(a.Title)
//	}
func (s *NewsService) Iter(ctx context.Context, params *NewsParams) iter.Seq2[Article, error] {
	return func(yield func(Article, error) bool) {
		p := NewsParams{}
		if params != nil {
			p = *params
		}
		for {
			resp, err := s.List(ctx, &p)
			if err != nil {
				yield(Article{}, err)
				return
			}
			for _, a := range resp.Data {
				if !yield(a, nil) {
					return
				}
			}
			if !resp.Pagination.HasMore() {
				return
			}
			p.Offset = resp.Pagination.NextOffset()
		}
	}
}

// Collect fetches articles until maxItems are gathered, the result set
// ends, or the pagination window is exhausted.
//
// A maxItems of zero fetches exactly one page. A maxItems above
// MaxPaginationWindow fails with ErrMaxOffset before any request: that
// many records cannot be reached through offset pagination. Collect
// picks page sizes of min(MaxLimit, remaining) so it never fetches
// more than asked for.
func (s *NewsService) Collect(ctx context.Context, params *NewsParams, maxItems int) ([]Article, error) {
	if maxItems < 0 {
		return nil, &ValidationError{Fields: map[string][]string{
			"max": {fmt.Sprintf("must be >= 0, got %d", maxItems)},
		}}
	}
	if maxItems > MaxPaginationWindow {
		return nil, fmt.Errorf("%w: cannot collect %d articles, max is %d",
			ErrMaxOffset, maxItems, MaxPaginationWindow)
	}
	p := NewsParams{}
	if params != nil {
		p = *params
	}
	if maxItems > 0 {
		p.Limit = min(MaxLimit, maxItems)
	}
	out := []Article{}
	for {
		resp, err := s.List(ctx, &p)
		if err != nil {
			return nil, err
		}
		out = append(out, resp.Data...)
		if maxItems == 0 || len(out) >= maxItems || !resp.Pagination.HasMore() {
			break
		}
		p.Offset = resp.Pagination.NextOffset()
		p.Limit = min(MaxLimit, maxItems-len(out))
	}
	return out, nil
}
