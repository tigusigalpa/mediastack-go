package mediastack

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSourcesListRequiresSearch(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		jsonHandler(t, "sources_id.json")(w, nil)
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	for _, p := range []*SourceParams{nil, {}, {Search: "   "}} {
		if _, err := c.Sources.List(context.Background(), p); !errors.Is(err, ErrMissingSearch) {
			t.Fatalf("want ErrMissingSearch, got %v", err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("no network call allowed, got %d", calls.Load())
	}
}

func TestSourcesSearch(t *testing.T) {
	var gotSearch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSearch = r.URL.Query().Get("search")
		jsonHandler(t, "sources_id.json")(w, r)
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	resp, err := c.Sources.Search(context.Background(), "abc", &SourceParams{
		Countries: Filter[Country]{Include: []Country{CountryAU}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotSearch != "abc" {
		t.Fatalf("search = %q", gotSearch)
	}
	if len(resp.Data) != 2 || resp.Data[0].ID != "abc-au" {
		t.Fatalf("data = %+v", resp.Data)
	}
	// Params must not be mutated.
	p := &SourceParams{}
	if _, err := c.Sources.Search(context.Background(), "x", p); err != nil {
		t.Fatal(err)
	}
	if p.Search != "" {
		t.Fatal("Search must not mutate params")
	}
}

func TestSourcesIter(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			w.Write([]byte(`{"pagination":{"limit":2,"offset":0,"count":2,"total":3},"data":[{"id":"a","name":"A","category":"general","country":"us","language":"en","url":"https://a.com"},{"id":"b","name":"B","category":"general","country":"us","language":"en","url":"https://b.com"}]}`))
			return
		}
		w.Write([]byte(`{"pagination":{"limit":2,"offset":2,"count":1,"total":3},"data":[{"id":"c","name":"C","category":"general","country":"us","language":"en","url":"https://c.com"}]}`))
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	var ids []string
	for src, err := range c.Sources.Iter(context.Background(), &SourceParams{Search: "n", Limit: 2}) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, src.ID)
	}
	if len(ids) != 3 || ids[0] != "a" || ids[2] != "c" {
		t.Fatalf("ids = %v", ids)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d", calls.Load())
	}
}

func TestSourcesIterMissingSearch(t *testing.T) {
	c, _ := NewClient("k", WithBaseURL("http://127.0.0.1:1"))
	var errs int
	for _, err := range c.Sources.Iter(context.Background(), nil) {
		if err != nil {
			errs++
			if !errors.Is(err, ErrMissingSearch) {
				t.Fatalf("want ErrMissingSearch, got %v", err)
			}
		}
	}
	if errs != 1 {
		t.Fatalf("errs = %d", errs)
	}
}
