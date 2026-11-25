package mediastack

import (
	"encoding/json"
	"testing"
	"time"
)

func TestArticleFull(t *testing.T) {
	var a Article
	if err := json.Unmarshal(fixture(t, "article_full.json"), &a); err != nil {
		t.Fatal(err)
	}
	if a.Author != "Jeremy Hutson" {
		t.Fatalf("author = %q", a.Author)
	}
	if a.Source != "TMZ.com" {
		t.Fatalf("source = %q", a.Source)
	}
	if !a.HasImage() {
		t.Fatal("HasImage must be true")
	}
	want := time.Date(2020, 8, 5, 5, 47, 24, 0, time.UTC)
	if !a.PublishedAt.Equal(want) {
		t.Fatalf("PublishedAt = %v, want %v", a.PublishedAt, want)
	}
	if a.Category != CategoryEntertainment || a.Language != LanguageEnglish || a.Country != CountryUS {
		t.Fatalf("enums = %s %s %s", a.Category, a.Language, a.Country)
	}
	if a.IsZero() {
		t.Fatal("decoded article must not be zero")
	}
	if got := a.String(); got == "" {
		t.Fatal("String must not be empty")
	}
}

func TestArticleNulls(t *testing.T) {
	var a Article
	if err := json.Unmarshal(fixture(t, "article_nulls.json"), &a); err != nil {
		t.Fatal(err)
	}
	if a.Author != "" || a.Description != "" || a.Image != "" {
		t.Fatalf("nulls must decode to empty strings: %+v", a)
	}
	if a.HasImage() {
		t.Fatal("HasImage must be false for null image")
	}
	if u, err := a.ImageURL(); u != nil || err != nil {
		t.Fatal("ImageURL must be nil,nil for empty image")
	}
}

func TestArticleBadDate(t *testing.T) {
	var resp NewsResponse
	if err := json.Unmarshal(fixture(t, "news_bad_date.json"), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("data len = %d", len(resp.Data))
	}
	if !resp.Data[0].PublishedAt.IsZero() {
		t.Fatal("bad date must produce zero time, not an error")
	}
	want := time.Date(2021, 3, 4, 10, 20, 30, 0, time.UTC)
	if !resp.Data[1].PublishedAt.Equal(want) {
		t.Fatalf("PublishedAt = %v, want %v", resp.Data[1].PublishedAt, want)
	}
}

func TestPublishedAtLayouts(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Time
	}{
		{`"2020-08-05T05:47:24+00:00"`, time.Date(2020, 8, 5, 5, 47, 24, 0, time.UTC)},
		{`"2020-08-05T05:47:24Z"`, time.Date(2020, 8, 5, 5, 47, 24, 0, time.UTC)},
		{`"2020-08-05T05:47:24.999Z"`, time.Date(2020, 8, 5, 5, 47, 24, 999000000, time.UTC)},
		{`"2020-08-05T05:47:24"`, time.Date(2020, 8, 5, 5, 47, 24, 0, time.UTC)},
		{`"2020-08-05 05:47:24"`, time.Date(2020, 8, 5, 5, 47, 24, 0, time.UTC)},
		{`"2020-08-05"`, time.Date(2020, 8, 5, 0, 0, 0, 0, time.UTC)},
		{`"garbage"`, time.Time{}},
		{`""`, time.Time{}},
		{`null`, time.Time{}},
		{`123`, time.Time{}},
	}
	for _, tc := range cases {
		got := parsePublishedAt(json.RawMessage(tc.raw))
		if !got.Equal(tc.want) {
			t.Fatalf("parsePublishedAt(%s) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestArticleHelpers(t *testing.T) {
	a := Article{Title: "T", Source: "S", URL: "https://example.com/x", Image: "https://example.com/i.png"}
	if a.String() != "T — S" {
		t.Fatalf("String = %q", a.String())
	}
	u, err := a.LinkURL()
	if err != nil || u.Host != "example.com" {
		t.Fatalf("LinkURL = %v %v", u, err)
	}
	iu, err := a.ImageURL()
	if err != nil || iu == nil {
		t.Fatalf("ImageURL = %v %v", iu, err)
	}
	if (Article{}).IsZero() == false {
		t.Fatal("zero article must be IsZero")
	}
}

func TestSourceIDAndCode(t *testing.T) {
	var resp SourceResponse
	if err := json.Unmarshal(fixture(t, "sources_id.json"), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data[0].ID != "abc-au" {
		t.Fatalf("id: ID = %q", resp.Data[0].ID)
	}
	if err := json.Unmarshal(fixture(t, "sources_code.json"), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data[0].ID != "abc-au" {
		t.Fatalf("code: ID = %q", resp.Data[0].ID)
	}
	s := Source{ID: "x", Name: "N"}
	if got := s.String(); got != "N (x)" {
		t.Fatalf("String = %q", got)
	}
}

func TestNewsResponseEmptyDataNotNil(t *testing.T) {
	var resp NewsResponse
	if err := json.Unmarshal(fixture(t, "news_empty.json"), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data == nil {
		t.Fatal("empty data must decode to empty slice, not nil")
	}
	if len(resp.Data) != 0 {
		t.Fatalf("data len = %d", len(resp.Data))
	}
}

func TestArticleMarshalRoundTrip(t *testing.T) {
	var a Article
	if err := json.Unmarshal(fixture(t, "article_full.json"), &a); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var a2 Article
	if err := json.Unmarshal(b, &a2); err != nil {
		t.Fatal(err)
	}
	if a2.Title != a.Title {
		t.Fatal("round trip lost title")
	}
}
