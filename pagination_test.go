package mediastack

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestPaginationMethods(t *testing.T) {
	p := Pagination{Limit: 25, Offset: 50, Count: 25, Total: 120}
	if p.Page() != 3 {
		t.Fatalf("Page = %d", p.Page())
	}
	if p.TotalPages() != 5 {
		t.Fatalf("TotalPages = %d", p.TotalPages())
	}
	if !p.HasMore() {
		t.Fatal("HasMore must be true")
	}
	if p.IsLast() {
		t.Fatal("IsLast must be false")
	}
	if p.NextOffset() != 75 {
		t.Fatalf("NextOffset = %d", p.NextOffset())
	}
	if p.Remaining() != 45 {
		t.Fatalf("Remaining = %d", p.Remaining())
	}
}

func TestPaginationEdgeCases(t *testing.T) {
	last := Pagination{Limit: 25, Offset: 100, Count: 20, Total: 120}
	if last.HasMore() || !last.IsLast() {
		t.Fatal("short last page: HasMore must be false")
	}
	zero := Pagination{}
	if zero.Page() != 1 || zero.TotalPages() != 0 || zero.Remaining() != 0 {
		t.Fatalf("zero pagination: %v %v %v", zero.Page(), zero.TotalPages(), zero.Remaining())
	}
	// count < limit but still more pages by total — the important case.
	mid := Pagination{Limit: 100, Offset: 0, Count: 42, Total: 100}
	if !mid.HasMore() {
		t.Fatal("HasMore must follow offset+count < total, not count==limit")
	}
}

// pagingServer serves page-sized slices of n synthetic articles.
func pagingServer(n int) *httptest.Server {
	mk := func(offset, limit int) []byte {
		end := offset + limit
		if end > n {
			end = n
		}
		data := "["
		for i := offset; i < end; i++ {
			if i > offset {
				data += ","
			}
			data += fmt.Sprintf(`{"title":"a%d","url":"https://e.com/%d","source":"s","category":"general","language":"en","country":"us","published_at":"2021-01-01"}`, i, i)
		}
		data += "]"
		return []byte(fmt.Sprintf(`{"pagination":{"limit":%d,"offset":%d,"count":%d,"total":%d},"data":%s}`,
			limit, offset, end-offset, n, data))
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		offset, limit := 0, 25
		fmt.Sscanf(q.Get("offset"), "%d", &offset)
		fmt.Sscanf(q.Get("limit"), "%d", &limit)
		w.Header().Set("Content-Type", "application/json")
		w.Write(mk(offset, limit))
	}))
}

func TestIterPaginates(t *testing.T) {
	srv := pagingServer(7)
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	var got []string
	for a, err := range c.News.Iter(context.Background(), &NewsParams{Limit: 3}) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, a.Title)
	}
	if len(got) != 7 {
		t.Fatalf("got %d articles: %v", len(got), got)
	}
	for i, title := range got {
		if title != fmt.Sprintf("a%d", i) {
			t.Fatalf("order broken at %d: %s", i, title)
		}
	}
}

func TestIterLazy(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		jsonHandler(t, "news_empty.json")(w, r)
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	it := c.News.Iter(context.Background(), nil)
	if calls.Load() != 0 {
		t.Fatal("Iter must not hit the network before first read")
	}
	for range it {
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d", calls.Load())
	}
}

func TestIterErrorStops(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"pagination":{"limit":1,"offset":0,"count":1,"total":2},"data":[{"title":"x","url":"u","source":"s","category":"g","language":"en","country":"us","published_at":"2021-01-01"}]}`))
			return
		}
		w.WriteHeader(422)
		w.Write(fixture(t, "error_422_validation.json"))
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	var errs int
	var items int
	for _, err := range c.News.Iter(context.Background(), &NewsParams{Limit: 1}) {
		if err != nil {
			errs++
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("want ErrValidation, got %v", err)
			}
			continue
		}
		items++
	}
	if items != 1 || errs != 1 || calls.Load() != 2 {
		t.Fatalf("items=%d errs=%d calls=%d", items, errs, calls.Load())
	}
}

func TestIterMaxWindow(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		offset := 0
		fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"pagination":{"limit":100,"offset":%d,"count":100,"total":20000},"data":[]}`, offset)
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	// Two full pages fit; the third would cross the 10,000 window.
	var errs int
	for _, err := range c.News.Iter(context.Background(), &NewsParams{Limit: 100, Offset: 9800}) {
		if err != nil {
			errs++
			if !errors.Is(err, ErrMaxOffset) {
				t.Fatalf("want ErrMaxOffset, got %v", err)
			}
		}
	}
	if errs != 1 {
		t.Fatalf("ErrMaxOffset must be yielded exactly once, got %d", errs)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
}

func TestCollect(t *testing.T) {
	srv := pagingServer(250)
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	arts, err := c.News.Collect(context.Background(), nil, 250)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 250 {
		t.Fatalf("got %d articles", len(arts))
	}
}

func TestCollectPageSizes(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		offset, limit := 0, 25
		fmt.Sscanf(q.Get("offset"), "%d", &offset)
		fmt.Sscanf(q.Get("limit"), "%d", &limit)
		seen = append(seen, fmt.Sprintf("%d:%d", limit, offset))
		end := offset + limit
		if end > 250 {
			end = 250
		}
		data := "["
		for i := offset; i < end; i++ {
			if i > offset {
				data += ","
			}
			data += fmt.Sprintf(`{"title":"a%d","url":"u","source":"s","category":"g","language":"en","country":"us","published_at":"2021-01-01"}`, i)
		}
		data += "]"
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"pagination":{"limit":%d,"offset":%d,"count":%d,"total":250},"data":%s}`, limit, offset, end-offset, data)
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	arts, err := c.News.Collect(context.Background(), nil, 250)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 250 {
		t.Fatalf("got %d", len(arts))
	}
	want := []string{"100:0", "100:100", "50:200"}
	if len(seen) != len(want) {
		t.Fatalf("requests = %v", seen)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("request %d = %s, want %s", i, seen[i], want[i])
		}
	}
}

func TestCollectZeroMeansOnePage(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		jsonHandler(t, "news_ok.json")(w, r)
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	arts, err := c.News.Collect(context.Background(), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d", calls.Load())
	}
	if len(arts) != 2 {
		t.Fatalf("got %d articles", len(arts))
	}
}

func TestCollectTooLarge(t *testing.T) {
	c, _ := NewClient("k")
	if _, err := c.News.Collect(context.Background(), nil, MaxPaginationWindow+1); !errors.Is(err, ErrMaxOffset) {
		t.Fatalf("want ErrMaxOffset, got %v", err)
	}
	if _, err := c.News.Collect(context.Background(), nil, -5); err == nil {
		t.Fatal("negative max must error")
	}
}
