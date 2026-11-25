package mediastack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewsListDecodesFixture(t *testing.T) {
	c := newServerClient(t, jsonHandler(t, "news_ok.json"))
	resp, err := c.News.List(context.Background(), &NewsParams{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("data len = %d", len(resp.Data))
	}
	a := resp.Data[0]
	if a.Source != "TMZ.com" || a.Category != CategoryEntertainment {
		t.Fatalf("article = %+v", a)
	}
	if a.PublishedAt.IsZero() {
		t.Fatal("PublishedAt must be parsed")
	}
	if resp.Pagination.Total != 10000 {
		t.Fatalf("total = %d", resp.Pagination.Total)
	}
}

func TestNewsListQueryParams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()
	tr := true
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	p := &NewsParams{
		Keywords:      Filter[string]{Include: []string{"tennis"}, Exclude: []string{"football"}},
		Sources:       Filter[string]{Include: []string{"cnn"}, Exclude: []string{"bbc"}},
		Categories:    Filter[Category]{Include: []Category{CategoryBusiness}},
		Countries:     Filter[Country]{Include: []Country{CountryUS, CountryGB}, Exclude: []Country{CountryDE}},
		Languages:     Filter[Language]{Include: []Language{LanguageEnglish}},
		Date:          Preset(DateLast7Days),
		Sort:          SortPopularity,
		Limit:         50,
		Offset:        100,
		OnlyWithImage: &tr,
	}
	if _, err := c.News.List(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"keywords":        "tennis,-football",
		"sources":         "cnn,-bbc",
		"categories":      "business",
		"countries":       "us,gb,-de",
		"languages":       "en",
		"date":            "last7days",
		"sort":            "popularity",
		"limit":           "50",
		"offset":          "100",
		"only_with_image": "true",
		"access_key":      "k",
	}
	for k, want := range checks {
		if got := gotQuery.Get(k); got != want {
			t.Fatalf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestNewsParamsNotMutated(t *testing.T) {
	c := newServerClient(t, jsonHandler(t, "news_empty.json"))
	p := &NewsParams{Limit: 5, Offset: 7}
	if _, err := c.News.List(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if p.Limit != 5 || p.Offset != 7 || !p.Date.IsZero() {
		t.Fatalf("params mutated: %+v", p)
	}
	if _, err := c.News.Historical(context.Background(), time.Now(), time.Now(), p); err != nil {
		t.Fatal(err)
	}
	if !p.Date.IsZero() {
		t.Fatal("Historical must not mutate params.Date")
	}
}

func TestHistoricalSetsDate(t *testing.T) {
	var gotDate string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDate = r.URL.Query().Get("date")
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	from := time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC)
	to := time.Date(2020, 1, 31, 23, 0, 0, 0, time.UTC)
	if _, err := c.News.Historical(context.Background(), from, to, nil); err != nil {
		t.Fatal(err)
	}
	if gotDate != "2020-01-01,2020-01-31" {
		t.Fatalf("date = %q", gotDate)
	}
}

func TestNewsNilParams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	if _, err := c.News.List(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(gotQuery) != 1 || gotQuery.Get("access_key") != "k" {
		t.Fatalf("only access_key expected, got %v", gotQuery)
	}
}

func TestNewsKeywordSpacesEncodeAsPlus(t *testing.T) {
	var raw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "news_empty.json"))
	}))
	defer srv.Close()
	c, _ := NewClient("k", WithBaseURL(srv.URL))
	p := &NewsParams{Keywords: Filter[string]{Include: []string{"machine learning"}}}
	if _, err := c.News.List(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "keywords=machine+learning") {
		t.Fatalf("spaces must encode as +: %s", raw)
	}
}
