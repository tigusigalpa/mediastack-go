package mediastack

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
)

// SourcesService provides access to the GET /v1/sources endpoint:
// discovery and metadata of news sources.
//
// Endpoint documentation:
// https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/API%20Endpoints/getSources
// Semantics: https://docs.apilayer.com/mediastack/docs/api-features#news-sources
type SourcesService struct {
	c *Client
}

// List returns one page of news sources matching params.
// params.Search is required by the API; when it is empty List fails
// with ErrMissingSearch before any network call, saving the request
// quota a rejected call would consume.
//
// This endpoint does not accept sort, date, keywords or
// only_with_image; those filters belong to NewsService.
func (s *SourcesService) List(ctx context.Context, params *SourceParams) (*SourceResponse, error) {
	p := SourceParams{}
	if params != nil {
		p = *params
	}
	values, err := p.encode(s.c.cfg.strictEnums)
	if err != nil {
		return nil, err
	}
	body, meta, err := s.c.do(ctx, "sources", values)
	if err != nil {
		return nil, err
	}
	var out SourceResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("mediastack: decode sources response: %w", err)
	}
	if out.Data == nil {
		out.Data = []Source{}
	}
	out.Meta = meta
	return &out, nil
}

// Search is a convenience wrapper that sets the required search term on
// a copy of params (which may be nil) and calls List.
func (s *SourcesService) Search(ctx context.Context, term string, params *SourceParams) (*SourceResponse, error) {
	p := SourceParams{}
	if params != nil {
		p = *params
	}
	p.Search = term
	return s.List(ctx, &p)
}

// Iter lazily paginates through the sources matching params, fetching
// the next page only when the previous one is exhausted. It performs no
// network call until the first element is requested.
//
// Like List, Iter requires a non-empty search term; a missing term
// yields ErrMissingSearch as the first and only element.
func (s *SourcesService) Iter(ctx context.Context, params *SourceParams) iter.Seq2[Source, error] {
	return func(yield func(Source, error) bool) {
		p := SourceParams{}
		if params != nil {
			p = *params
		}
		for {
			resp, err := s.List(ctx, &p)
			if err != nil {
				yield(Source{}, err)
				return
			}
			for _, src := range resp.Data {
				if !yield(src, nil) {
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
