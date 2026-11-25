package mediastack

import (
	"errors"
	"net/url"
	"testing"
	"time"
)

func mustEncodeNews(t *testing.T, p *NewsParams) url.Values {
	t.Helper()
	v, err := p.encode(false)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return v
}

func TestFilterEncode(t *testing.T) {
	p := &NewsParams{
		Categories: Filter[Category]{
			Include: []Category{CategoryBusiness},
			Exclude: []Category{CategorySports},
		},
		Countries: Filter[Country]{
			Include: []Country{CountryUS, CountryGB},
			Exclude: []Country{CountryDE},
		},
	}
	v := mustEncodeNews(t, p)
	if got := v.Get("categories"); got != "business,-sports" {
		t.Fatalf("categories = %q", got)
	}
	if got := v.Get("countries"); got != "us,gb,-de" {
		t.Fatalf("countries = %q", got)
	}
}

func TestEmptyFilterOmitted(t *testing.T) {
	v := mustEncodeNews(t, &NewsParams{})
	for _, k := range []string{"keywords", "sources", "categories", "countries", "languages", "date", "sort", "only_with_image", "limit", "offset"} {
		if _, ok := v[k]; ok {
			t.Fatalf("unexpected param %s", k)
		}
	}
}

func TestLimitZeroNotSent(t *testing.T) {
	v := mustEncodeNews(t, &NewsParams{Limit: 0})
	if _, ok := v["limit"]; ok {
		t.Fatal("limit=0 must not be sent")
	}
	v = mustEncodeNews(t, &NewsParams{Limit: 100})
	if v.Get("limit") != "100" {
		t.Fatalf("limit = %q", v.Get("limit"))
	}
}

func TestOnlyWithImageTriState(t *testing.T) {
	tr, fl := true, false
	v := mustEncodeNews(t, &NewsParams{})
	if _, ok := v["only_with_image"]; ok {
		t.Fatal("nil OnlyWithImage must be omitted")
	}
	v = mustEncodeNews(t, &NewsParams{OnlyWithImage: &tr})
	if v.Get("only_with_image") != "true" {
		t.Fatalf("only_with_image = %q", v.Get("only_with_image"))
	}
	v = mustEncodeNews(t, &NewsParams{OnlyWithImage: &fl})
	if v.Get("only_with_image") != "false" {
		t.Fatalf("only_with_image = %q", v.Get("only_with_image"))
	}
}

func TestDateFilterEncodings(t *testing.T) {
	day := time.Date(2020, 1, 15, 12, 30, 0, 0, time.FixedZone("X", 3600))
	cases := []struct {
		name string
		d    DateFilter
		want string
	}{
		{"day", OnDay(day), "2020-01-15"},
		{"range", Between(day, day.Add(48*time.Hour)), "2020-01-15,2020-01-17"},
		{"preset", Preset(DateLast7Days), "last7days"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := mustEncodeNews(t, &NewsParams{Date: tc.d})
			if v.Get("date") != tc.want {
				t.Fatalf("date = %q, want %q", v.Get("date"), tc.want)
			}
		})
	}
}

func TestDateFilterInvalidRange(t *testing.T) {
	from := time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := (&NewsParams{Date: Between(from, to)}).encode(false)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	if len(ve.Fields["date"]) == 0 {
		t.Fatalf("date field errors = %v", ve.Fields)
	}
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatal("ValidationError must match ErrInvalidParams")
	}
}

func TestParseDateFilter(t *testing.T) {
	cases := []struct {
		in    string
		want  string
		isErr bool
	}{
		{"2020-01-01", "2020-01-01", false},
		{"2020-01-01,2020-01-31", "2020-01-01,2020-01-31", false},
		{"last7days", "last7days", false},
		{"today", "today", false},
		{"", "", false},
		{"bogus", "", true},
		{"2020-13-40", "", true},
		{"2020-02-01,2020-01-01", "", true},
		{"a,b,c", "", true},
	}
	for _, tc := range cases {
		d, err := ParseDateFilter(tc.in)
		if tc.isErr {
			if err == nil {
				t.Fatalf("ParseDateFilter(%q): want error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseDateFilter(%q): %v", tc.in, err)
		}
		if got := d.String(); got != tc.want {
			t.Fatalf("ParseDateFilter(%q).String() = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDateFilterIsZero(t *testing.T) {
	if !(DateFilter{}).IsZero() {
		t.Fatal("zero DateFilter must be IsZero")
	}
	if OnDay(time.Now()).IsZero() {
		t.Fatal("OnDay must not be IsZero")
	}
	if (DateFilter{invalid: true}).String() != "" {
		t.Fatal("invalid filter String must be empty")
	}
}

func TestExtraOverrides(t *testing.T) {
	p := &NewsParams{
		Limit: 10,
		Extra: url.Values{"limit": {"77"}, "future_param": {"x"}},
	}
	v := mustEncodeNews(t, p)
	if v.Get("limit") != "77" {
		t.Fatalf("Extra must override, limit = %q", v.Get("limit"))
	}
	if v.Get("future_param") != "x" {
		t.Fatal("Extra param missing")
	}
}

func TestKeywordWithCommaRejected(t *testing.T) {
	p := &NewsParams{Keywords: Filter[string]{Include: []string{"food, industry"}}}
	_, err := p.encode(false)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	if len(ve.Fields["keywords"]) == 0 {
		t.Fatalf("keywords errors = %v", ve.Fields)
	}
}

func TestKeywordEmptyRejected(t *testing.T) {
	p := &NewsParams{Keywords: Filter[string]{Exclude: []string{""}}}
	_, err := p.encode(false)
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("want ErrInvalidParams, got %v", err)
	}
}

func TestSortValidation(t *testing.T) {
	if _, err := (&NewsParams{Sort: "bogus"}).encode(false); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("want ErrInvalidParams, got %v", err)
	}
	v := mustEncodeNews(t, &NewsParams{Sort: SortPopularity})
	if v.Get("sort") != "popularity" {
		t.Fatalf("sort = %q", v.Get("sort"))
	}
}

func TestLimitOffsetValidation(t *testing.T) {
	cases := []struct {
		name   string
		p      NewsParams
		sentin error
	}{
		{"limit 101", NewsParams{Limit: 101}, ErrInvalidParams},
		{"limit -1", NewsParams{Limit: -1}, ErrInvalidParams},
		{"offset -1", NewsParams{Offset: -1}, ErrInvalidParams},
		{"window", NewsParams{Limit: 100, Offset: 9901}, ErrMaxOffset},
		{"window default limit", NewsParams{Offset: 9976}, ErrMaxOffset},
		{"window ok", NewsParams{Limit: 100, Offset: 9900}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.p.encode(false)
			if tc.sentin == nil {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			if !errors.Is(err, tc.sentin) {
				t.Fatalf("want %v, got %v", tc.sentin, err)
			}
		})
	}
}

func TestSourceParamsEncode(t *testing.T) {
	p := &SourceParams{
		Search: "abc",
		Countries: Filter[Country]{
			Include: []Country{CountryAU},
		},
		Limit: 100,
	}
	v, err := p.encode(false)
	if err != nil {
		t.Fatal(err)
	}
	if v.Get("search") != "abc" || v.Get("countries") != "au" || v.Get("limit") != "100" {
		t.Fatalf("values = %v", v)
	}
}

func TestSourceParamsMissingSearch(t *testing.T) {
	for _, p := range []*SourceParams{nil, {}, {Search: "  "}} {
		if _, err := p.encode(false); !errors.Is(err, ErrMissingSearch) {
			t.Fatalf("want ErrMissingSearch, got %v", err)
		}
	}
}

func TestStrictEnums(t *testing.T) {
	p := &NewsParams{Categories: Filter[Category]{Include: []Category{"madeup"}}}
	if _, err := p.encode(false); err != nil {
		t.Fatalf("non-strict must pass unknown category, got %v", err)
	}
	if _, err := p.encode(true); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("strict must reject unknown category, got %v", err)
	}
	// politics is documented on the features page and must pass strict.
	p2 := &NewsParams{Categories: Filter[Category]{Include: []Category{CategoryPolitics}}}
	if _, err := p2.encode(true); err != nil {
		t.Fatalf("politics must pass strict, got %v", err)
	}
	// bad country shape rejected in strict mode.
	p3 := &SourceParams{Search: "x", Countries: Filter[Country]{Include: []Country{"USA"}}}
	if _, err := p3.encode(true); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("strict must reject 'USA', got %v", err)
	}
	// hi and se are accepted in strict mode (documented inconsistently).
	p4 := &NewsParams{Languages: Filter[Language]{Include: []Language{LanguageHindi, LanguageSwedishLegacy}}}
	if _, err := p4.encode(true); err != nil {
		t.Fatalf("hi/se must pass strict, got %v", err)
	}
}

func TestFilterIsZero(t *testing.T) {
	var f Filter[string]
	if !f.IsZero() {
		t.Fatal("empty filter must be IsZero")
	}
	f.Exclude = []string{"x"}
	if f.IsZero() {
		t.Fatal("filter with exclude must not be IsZero")
	}
}

func BenchmarkEncodeNewsParams(b *testing.B) {
	p := &NewsParams{
		Keywords:   Filter[string]{Include: []string{"space"}, Exclude: []string{"mars"}},
		Categories: Filter[Category]{Include: []Category{CategoryTechnology}},
		Countries:  Filter[Country]{Include: []Country{CountryUS}},
		Sort:       SortPopularity,
		Limit:      50,
		Offset:     100,
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := p.encode(false); err != nil {
			b.Fatal(err)
		}
	}
}
