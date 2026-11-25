package mediastack

import (
	"encoding/json"
	"net/url"
	"time"
)

// Article is a single news article returned by GET /v1/news.
//
// The API marks author, description and image as nullable; this SDK
// decodes JSON null into the empty string instead of pointer fields, so
// all fields are always safe to read.
//
// Schema: https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/Article
type Article struct {
	// Author is the author of the article, or empty when the API
	// returned null.
	Author string `json:"author"`

	// Title is the headline of the article.
	Title string `json:"title"`

	// Description is a short summary or lead paragraph, or empty.
	Description string `json:"description"`

	// URL is the link to the original article.
	URL string `json:"url"`

	// Source is the display name of the publishing source.
	Source string `json:"source"`

	// Image is the URL of the article image, or empty when the API
	// returned null.
	Image string `json:"image"`

	// Category is the article category.
	Category Category `json:"category"`

	// Language is the language the article is written in.
	Language Language `json:"language"`

	// Country is the country the article was published in.
	Country Country `json:"country"`

	// PublishedAt is the publication timestamp. It is parsed tolerantly:
	// RFC 3339 with and without fractional seconds or time zone, plain
	// "YYYY-MM-DD HH:MM:SS", and bare dates are all accepted (the last
	// three are interpreted as UTC). An unparseable value yields the
	// zero time instead of an error.
	PublishedAt time.Time `json:"published_at"`
}

// publishedAtLayouts are tried in order when decoding PublishedAt.
var publishedAtLayouts = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// UnmarshalJSON decodes an Article, parsing published_at tolerantly.
// A malformed or null published_at produces a zero PublishedAt and does
// not fail the decoding of the rest of the article.
func (a *Article) UnmarshalJSON(data []byte) error {
	type plain Article
	aux := struct {
		plain
		PublishedAt json.RawMessage `json:"published_at"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*a = Article(aux.plain)
	a.PublishedAt = parsePublishedAt(aux.PublishedAt)
	return nil
}

// parsePublishedAt attempts every supported layout; returns zero time on
// failure or for JSON null/empty.
func parsePublishedAt(raw json.RawMessage) time.Time {
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || s == "" {
		return time.Time{}
	}
	for _, layout := range publishedAtLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// HasImage reports whether the article has an image URL.
func (a Article) HasImage() bool { return a.Image != "" }

// ImageURL parses Image into a *url.URL. It returns nil, nil when the
// article has no image.
func (a Article) ImageURL() (*url.URL, error) {
	if a.Image == "" {
		return nil, nil
	}
	return url.Parse(a.Image)
}

// LinkURL parses URL into a *url.URL. It returns nil, nil when the
// article has no link.
func (a Article) LinkURL() (*url.URL, error) {
	if a.URL == "" {
		return nil, nil
	}
	return url.Parse(a.URL)
}

// IsZero reports whether the article is the zero value.
func (a Article) IsZero() bool {
	return a == Article{}
}

// String returns a short human-readable summary of the article: its
// title and, when present, its source name. It never contains
// credentials.
func (a Article) String() string {
	if a.Source != "" {
		return a.Title + " — " + a.Source
	}
	return a.Title
}

// Source is a news source returned by GET /v1/sources.
//
// The API historically returns the identifier under two different field
// names: the OpenAPI schema documents "code" while live responses and
// the API Features examples use "id". Both are decoded into ID; there is
// no separate Code field.
//
// Schema: https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/Source
type Source struct {
	// ID is the source identifier. This is the value to pass to the
	// sources filter of GET /v1/news.
	ID string `json:"id"`

	// Name is the display name of the source.
	Name string `json:"name"`

	// Category is the primary category of the source.
	Category Category `json:"category"`

	// Country is the country of the source.
	Country Country `json:"country"`

	// Language is the primary language of the source.
	Language Language `json:"language"`

	// URL is the website of the source.
	URL string `json:"url"`
}

// UnmarshalJSON decodes a Source, accepting the identifier under either
// the "id" or the legacy "code" key.
func (s *Source) UnmarshalJSON(data []byte) error {
	aux := struct {
		ID       string   `json:"id"`
		Code     string   `json:"code"`
		Name     string   `json:"name"`
		Category Category `json:"category"`
		Country  Country  `json:"country"`
		Language Language `json:"language"`
		URL      string   `json:"url"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	s.ID = aux.ID
	if s.ID == "" {
		s.ID = aux.Code
	}
	s.Name = aux.Name
	s.Category = aux.Category
	s.Country = aux.Country
	s.Language = aux.Language
	s.URL = aux.URL
	return nil
}

// String returns a short human-readable summary of the source.
func (s Source) String() string {
	if s.ID != "" {
		return s.Name + " (" + s.ID + ")"
	}
	return s.Name
}

// NewsResponse is the payload of a successful GET /v1/news call.
//
// Schema: https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/NewsResponse
type NewsResponse struct {
	// Pagination describes the page window of this response.
	Pagination Pagination `json:"pagination"`

	// Data holds the articles of this page. It is never nil after a
	// successful call; an empty result set decodes to an empty slice.
	Data []Article `json:"data"`

	// Meta carries transport-level information about the HTTP exchange.
	Meta ResponseMeta `json:"-"`
}

// SourceResponse is the payload of a successful GET /v1/sources call.
//
// Schema: https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0#/SourceResponse
type SourceResponse struct {
	// Pagination describes the page window of this response.
	Pagination Pagination `json:"pagination"`

	// Data holds the sources of this page. It is never nil after a
	// successful call.
	Data []Source `json:"data"`

	// Meta carries transport-level information about the HTTP exchange.
	Meta ResponseMeta `json:"-"`
}

// ResponseMeta carries transport-level information about a response.
// It is attached to NewsResponse and SourceResponse and never
// deserialized from the response body.
type ResponseMeta struct {
	// StatusCode is the HTTP status of the final response.
	StatusCode int

	// RequestID is the value of the cf-ray header (or X-Request-Id when
	// absent). Quote it when contacting APILayer support.
	RequestID string

	// Attempts is the number of HTTP attempts performed, including the
	// first one.
	Attempts int

	// Duration is the time spent in HTTP attempts, including reading
	// response bodies. It excludes retry backoff and limiter waits.
	Duration time.Duration

	// FetchedAt is the time the final response body was read, in the
	// client's clock (see WithClock).
	FetchedAt time.Time

	// RateLimit holds the quota headers of the final response. Its
	// Present field is false when the API did not send them.
	RateLimit RateLimit
}
