package mediastack

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Filter expresses include and exclude lists for a single API parameter.
// The API encodes inclusion as a comma-separated list and exclusion by
// prefixing an element with a minus sign (for example
// "cnn,-bbc" or "us,gb,-de"). Filter models the two lists separately so
// callers never deal with the wire encoding.
//
// Elements must be non-empty and must not contain commas; both are
// enforced by validation before a request is sent.
type Filter[T ~string] struct {
	// Include lists values the result set must match.
	Include []T

	// Exclude lists values that are subtracted from the result set.
	// They are encoded after the Include values with a "-" prefix.
	Exclude []T
}

// IsZero reports whether the filter has no values.
func (f Filter[T]) IsZero() bool {
	return len(f.Include) == 0 && len(f.Exclude) == 0
}

// encodeFilter renders a Filter as the comma-separated wire format, for
// example "us,gb,-de". It returns an error for empty elements or
// elements containing a comma.
func encodeFilter[T ~string](f Filter[T]) (string, error) {
	if f.IsZero() {
		return "", nil
	}
	parts := make([]string, 0, len(f.Include)+len(f.Exclude))
	for i, v := range f.Include {
		s := string(v)
		if err := checkListElement(s); err != nil {
			return "", fmt.Errorf("include value at index %d %w", i, err)
		}
		parts = append(parts, s)
	}
	for i, v := range f.Exclude {
		s := string(v)
		if err := checkListElement(s); err != nil {
			return "", fmt.Errorf("exclude value at index %d %w", i, err)
		}
		parts = append(parts, "-"+s)
	}
	return strings.Join(parts, ","), nil
}

func checkListElement(s string) error {
	if s == "" {
		return errors.New("must not be empty")
	}
	if strings.Contains(s, ",") {
		return errors.New("must not contain commas")
	}
	return nil
}

// NewsParams holds every query parameter accepted by GET /v1/news.
// The zero value is valid and means "no filters, API defaults".
//
// Documentation: https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/API%20Endpoints/getNews
type NewsParams struct {
	// Keywords performs full-text search over article titles and
	// descriptions. Phrases may contain spaces; use Exclude for words
	// that must not appear.
	Keywords Filter[string]

	// Sources filters by source identifiers (see Source.ID), for
	// example Include: []string{"cnn", "bbc"}.
	Sources Filter[string]

	// Categories filters by news category.
	Categories Filter[Category]

	// Countries filters by ISO 3166-1 alpha-2 country code.
	Countries Filter[Country]

	// Languages filters by two-letter language code.
	Languages Filter[Language]

	// Date restricts articles to a day, an inclusive day range or a
	// named preset. Required by the API for historical data; build it
	// with OnDay, Between or Preset.
	Date DateFilter

	// Sort selects the ordering. Validated against the three documented
	// values; the zero value uses the API default (published_desc).
	Sort SortOrder

	// Limit is the page size, 1..MaxLimit. The zero value means
	// "not sent": the API then applies its own default of DefaultLimit
	// (25). Use DefaultLimit explicitly if you need 25 on the wire.
	Limit int

	// Offset is the starting position of the page. Offset+Limit must
	// not exceed MaxPaginationWindow (10,000).
	Offset int

	// OnlyWithImage filters to articles that have an image. A nil value
	// omits the parameter entirely (API default applies); &true and
	// &false send the parameter explicitly.
	OnlyWithImage *bool

	// Extra is merged into the query string last and overrides every
	// computed value. It is an escape hatch for parameters the SDK does
	// not yet model.
	Extra url.Values
}

// SourceParams holds every query parameter accepted by GET /v1/sources.
// Search is required by the API: a request without it is rejected, so the
// SDK reports ErrMissingSearch before any network call.
//
// Documentation: https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/API%20Endpoints/getSources
type SourceParams struct {
	// Search is the required search term matched against source names.
	Search string

	// Categories filters by news category.
	Categories Filter[Category]

	// Countries filters by ISO 3166-1 alpha-2 country code.
	Countries Filter[Country]

	// Languages filters by two-letter language code.
	Languages Filter[Language]

	// Limit is the page size, 1..MaxLimit. Zero means "not sent" and
	// lets the API apply DefaultLimit.
	Limit int

	// Offset is the starting position of the page. Offset+Limit must
	// not exceed MaxPaginationWindow (10,000).
	Offset int

	// Extra is merged into the query string last and overrides every
	// computed value.
	Extra url.Values
}

// DateFilter represents the date parameter of GET /v1/news: a single
// day, an inclusive day range, or a named preset.
//
// Build values with OnDay, Between or Preset, or parse the wire format
// with ParseDateFilter. The zero value is valid and omits the parameter.
type DateFilter struct {
	from    time.Time
	to      time.Time
	preset  DatePreset
	single  bool
	invalid bool
}

// OnDay filters articles published on a single day. The time is
// normalized to UTC before formatting.
func OnDay(t time.Time) DateFilter {
	return DateFilter{from: t.UTC(), single: true}
}

// Between filters articles published in the inclusive UTC day range
// [from, to]. If to is before from the returned filter is not panicking
// garbage: it is marked invalid and surfaces as a ValidationError when
// the request is encoded.
func Between(from, to time.Time) DateFilter {
	d := DateFilter{from: from.UTC(), to: to.UTC()}
	if to.Before(from) {
		d.invalid = true
	}
	return d
}

// Preset filters using one of the named date options, for example
// Preset(DateLast7Days).
func Preset(p DatePreset) DateFilter {
	return DateFilter{preset: p}
}

// ParseDateFilter parses the wire format of the date parameter:
// "YYYY-MM-DD", "YYYY-MM-DD,YYYY-MM-DD", or a preset name such as
// "last7days". It returns an error for anything else and for a range
// whose end precedes its start.
func ParseDateFilter(s string) (DateFilter, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DateFilter{}, nil
	}
	if p := DatePreset(s); p.Valid() {
		return Preset(p), nil
	}
	parts := strings.Split(s, ",")
	switch len(parts) {
	case 1:
		t, err := time.Parse("2006-01-02", parts[0])
		if err != nil {
			return DateFilter{}, fmt.Errorf("mediastack: invalid date %q, want YYYY-MM-DD", s)
		}
		return OnDay(t), nil
	case 2:
		from, err := time.Parse("2006-01-02", strings.TrimSpace(parts[0]))
		if err != nil {
			return DateFilter{}, fmt.Errorf("mediastack: invalid date range %q", s)
		}
		to, err := time.Parse("2006-01-02", strings.TrimSpace(parts[1]))
		if err != nil {
			return DateFilter{}, fmt.Errorf("mediastack: invalid date range %q", s)
		}
		if to.Before(from) {
			return DateFilter{}, fmt.Errorf("mediastack: date range %q ends before it starts", s)
		}
		return Between(from, to), nil
	default:
		return DateFilter{}, fmt.Errorf("mediastack: invalid date filter %q", s)
	}
}

// IsZero reports whether the filter is unset.
func (d DateFilter) IsZero() bool {
	return !d.single && d.preset == "" && d.from.IsZero() && d.to.IsZero() && !d.invalid
}

// String returns the wire representation of the filter, or the empty
// string when the filter is unset or invalid.
func (d DateFilter) String() string {
	s, err := d.encode()
	if err != nil {
		return ""
	}
	return s
}

func formatDay(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// encode renders the date parameter value.
func (d DateFilter) encode() (string, error) {
	if d.IsZero() {
		return "", nil
	}
	if d.invalid {
		return "", errors.New("range end must not be before range start")
	}
	if d.preset != "" {
		if !d.preset.Valid() {
			return "", fmt.Errorf("unknown preset %q", string(d.preset))
		}
		return string(d.preset), nil
	}
	if d.single {
		return formatDay(d.from), nil
	}
	return formatDay(d.from) + "," + formatDay(d.to), nil
}

// setFilter encodes f into v under name, recording failures on ve.
func setFilter[T ~string](v url.Values, ve *ValidationError, name string, f Filter[T]) {
	s, err := encodeFilter(f)
	if err != nil {
		ve.add(name, err.Error())
		return
	}
	if s != "" {
		v.Set(name, s)
	}
}

// encode renders NewsParams into query values. Validation problems are
// aggregated into a *ValidationError; a pagination window that exceeds
// MaxPaginationWindow yields an error wrapping ErrMaxOffset.
func (p *NewsParams) encode(strict bool) (url.Values, error) {
	if p == nil {
		p = &NewsParams{}
	}
	v := url.Values{}
	ve := &ValidationError{Fields: map[string][]string{}}

	setFilter(v, ve, "keywords", p.Keywords)
	setFilter(v, ve, "sources", p.Sources)
	setFilter(v, ve, "categories", p.Categories)
	setFilter(v, ve, "countries", p.Countries)
	setFilter(v, ve, "languages", p.Languages)
	if s, err := p.Date.encode(); err != nil {
		ve.add("date", err.Error())
	} else if s != "" {
		v.Set("date", s)
	}

	if strict {
		strictEnumCheck(ve, "categories", p.Categories, knownCategories)
		strictEnumCheck(ve, "languages", p.Languages, knownLanguages)
		strictCountryCheck(ve, "countries", p.Countries)
	}

	if p.Sort != "" && !p.Sort.Valid() {
		ve.add("sort", fmt.Sprintf("must be one of %q, %q or %q, got %q",
			SortPublishedDesc, SortPublishedAsc, SortPopularity, string(p.Sort)))
	} else if p.Sort != "" {
		v.Set("sort", string(p.Sort))
	}
	validatePage(ve, p.Limit, p.Offset)

	if p.OnlyWithImage != nil {
		v.Set("only_with_image", strconv.FormatBool(*p.OnlyWithImage))
	}

	if err := ve.err(); err != nil {
		return nil, err
	}
	if err := checkWindow(p.Limit, p.Offset); err != nil {
		return nil, err
	}

	if p.Limit > 0 {
		v.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Offset > 0 {
		v.Set("offset", strconv.Itoa(p.Offset))
	}
	mergeExtra(v, p.Extra)
	return v, nil
}

// encode renders SourceParams into query values. A missing Search term
// fails with ErrMissingSearch before any other validation.
func (p *SourceParams) encode(strict bool) (url.Values, error) {
	if p == nil {
		p = &SourceParams{}
	}
	if strings.TrimSpace(p.Search) == "" {
		return nil, ErrMissingSearch
	}
	v := url.Values{"search": {p.Search}}
	ve := &ValidationError{Fields: map[string][]string{}}

	setFilter(v, ve, "categories", p.Categories)
	setFilter(v, ve, "countries", p.Countries)
	setFilter(v, ve, "languages", p.Languages)

	if strict {
		strictEnumCheck(ve, "categories", p.Categories, knownCategories)
		strictEnumCheck(ve, "languages", p.Languages, knownLanguages)
		strictCountryCheck(ve, "countries", p.Countries)
	}

	validatePage(ve, p.Limit, p.Offset)

	if err := ve.err(); err != nil {
		return nil, err
	}
	if err := checkWindow(p.Limit, p.Offset); err != nil {
		return nil, err
	}

	if p.Limit > 0 {
		v.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Offset > 0 {
		v.Set("offset", strconv.Itoa(p.Offset))
	}
	mergeExtra(v, p.Extra)
	return v, nil
}

// validatePage checks limit and offset bounds shared by both endpoints.
func validatePage(ve *ValidationError, limit, offset int) {
	if limit < 0 || limit > MaxLimit {
		ve.add("limit", fmt.Sprintf("must be between 1 and %d, got %d", MaxLimit, limit))
	}
	if offset < 0 {
		ve.add("offset", fmt.Sprintf("must be >= 0, got %d", offset))
	}
}

// checkWindow enforces the offset+limit <= MaxPaginationWindow contract.
// The effective limit is the API default when the caller left it unset.
func checkWindow(limit, offset int) error {
	eff := limit
	if eff <= 0 {
		eff = DefaultLimit
	}
	if sum := offset + eff; sum > MaxPaginationWindow {
		return fmt.Errorf("%w: offset+limit must not exceed %d, got %d",
			ErrMaxOffset, MaxPaginationWindow, sum)
	}
	return nil
}

// mergeExtra overrides computed query values with caller-supplied ones.
func mergeExtra(v, extra url.Values) {
	for k, vs := range extra {
		cp := make([]string, len(vs))
		copy(cp, vs)
		v[k] = cp
	}
}

// strictEnumCheck reports filter values outside the documented set. It
// runs only when the client was created with WithStrictEnums.
func strictEnumCheck[T ~string](ve *ValidationError, field string, f Filter[T], known func(string) bool) {
	for _, v := range f.Include {
		if !known(string(v)) {
			ve.add(field, fmt.Sprintf("unknown value %q", string(v)))
		}
	}
	for _, v := range f.Exclude {
		if !known(string(v)) {
			ve.add(field, fmt.Sprintf("unknown value %q", string(v)))
		}
	}
}

func knownCategories(s string) bool {
	for _, c := range Categories() {
		if string(c) == s {
			return true
		}
	}
	return false
}

func knownLanguages(s string) bool {
	for _, l := range Languages() {
		if string(l) == s {
			return true
		}
	}
	// hi and se are documented inconsistently; strict mode still accepts
	// them because the API may honour them.
	return s == string(LanguageHindi) || s == string(LanguageSwedishLegacy)
}

// strictCountryCheck enforces the alpha-2 shape only; the SDK does not
// carry the full ISO list.
func strictCountryCheck(ve *ValidationError, field string, f Filter[Country]) {
	check := func(v Country) {
		s := string(v)
		if len(s) != 2 || !isLowerAlpha(s) {
			ve.add(field, fmt.Sprintf("must be an ISO 3166-1 alpha-2 code, got %q", s))
		}
	}
	for _, v := range f.Include {
		check(v)
	}
	for _, v := range f.Exclude {
		check(v)
	}
}

func isLowerAlpha(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
