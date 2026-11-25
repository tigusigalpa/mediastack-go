package mediastack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// fetchPage decodes the common envelope for both endpoints. Some API
// gateways return an error envelope with a successful HTTP status.
func fetchPage[T any](ctx context.Context, c *Client, path string, query url.Values) (Pagination, []T, ResponseMeta, error) {
	body, meta, err := c.do(ctx, path, query)
	if err != nil {
		return Pagination{}, nil, meta, err
	}
	var page struct {
		Pagination *Pagination     `json:"pagination"`
		Data       []T             `json:"data"`
		Error      json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return Pagination{}, nil, meta, fmt.Errorf("mediastack: decode %s response: %w", path, sanitizeError(err, *c.key))
	}
	if len(page.Error) > 0 && string(page.Error) != "null" {
		return Pagination{}, nil, meta, decodeAPIError(meta.StatusCode, meta.RequestID, body, *c.key)
	}
	if page.Pagination == nil && page.Data == nil {
		return Pagination{}, nil, meta, fmt.Errorf("mediastack: decode %s response: missing pagination and data", path)
	}
	if page.Data == nil {
		page.Data = []T{}
	}
	var pagination Pagination
	if page.Pagination != nil {
		pagination = *page.Pagination
	}
	return pagination, page.Data, meta, nil
}

// Pagination mirrors the pagination object the API attaches to every
// response.
//
// The API allows at most MaxPaginationWindow (10,000) records per query
// through offset+limit; Total may be larger than that, in which case
// only the first window is reachable.
//
// Schema: https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/Pagination
type Pagination struct {
	// Limit is the requested page size.
	Limit int `json:"limit"`

	// Offset is the requested offset into the result set.
	Offset int `json:"offset"`

	// Count is the number of elements actually present on this page.
	Count int `json:"count"`

	// Total is the total number of elements matching the query.
	Total int `json:"total"`
}

// Page returns the 1-based page number of this window.
func (p Pagination) Page() int {
	if p.Limit <= 0 {
		return 1
	}
	return p.Offset/p.Limit + 1
}

// TotalPages returns the total number of pages of size Limit.
func (p Pagination) TotalPages() int {
	if p.Limit <= 0 {
		if p.Total > 0 {
			return 1
		}
		return 0
	}
	return (p.Total + p.Limit - 1) / p.Limit
}

// HasMore reports whether another page follows this one. It is computed
// as Offset+Count < Total rather than Count == Limit, so a short last
// page cannot trigger an extra, quota-consuming request.
func (p Pagination) HasMore() bool {
	return p.Offset >= 0 && p.Offset < p.Total && p.Count < p.Total-p.Offset
}

// NextOffset returns the offset of the page following this one.
func (p Pagination) NextOffset() int {
	return p.Offset + p.Count
}

// advancePage rejects a stalled cursor and fits the next request into the
// remaining pagination window, including a partial final page.
func advancePage(p Pagination, requestedOffset, limit int) (int, int, error) {
	next := p.NextOffset()
	if p.Offset < requestedOffset || next <= p.Offset {
		return 0, 0, fmt.Errorf("%w: requested offset %d, received offset %d and count %d",
			ErrInvalidPagination, requestedOffset, p.Offset, p.Count)
	}
	if next >= MaxPaginationWindow {
		return 0, 0, fmt.Errorf("%w: next offset %d reaches the %d-record window",
			ErrMaxOffset, next, MaxPaginationWindow)
	}
	remaining := MaxPaginationWindow - next
	if limit > remaining || (limit == 0 && DefaultLimit > remaining) {
		limit = remaining
	}
	return next, limit, nil
}

// IsLast reports whether this is the final page of the result set.
func (p Pagination) IsLast() bool {
	return !p.HasMore()
}

// Remaining returns how many elements follow this page. It is never
// negative.
func (p Pagination) Remaining() int {
	if r := p.Total - (p.Offset + p.Count); r > 0 {
		return r
	}
	return 0
}
