package mediastack

import "strings"

// Category is a news category accepted by the categories parameter.
//
// The OpenAPI contract documents seven categories; the API Features page
// additionally documents "politics". The client does not restrict values
// to the documented set by default: unknown values are sent to the server,
// which answers with a validation_error when it does not support them.
// Use WithStrictEnums to opt into client-side checking.
type Category string

// Documented news categories.
const (
	CategoryGeneral       Category = "general"
	CategoryBusiness      Category = "business"
	CategoryEntertainment Category = "entertainment"
	CategoryHealth        Category = "health"
	CategoryScience       Category = "science"
	CategorySports        Category = "sports"
	CategoryTechnology    Category = "technology"

	// CategoryPolitics is documented on the API Features page but missing
	// from the OpenAPI schema. It is accepted by this SDK; see the README
	// section on documentation discrepancies.
	CategoryPolitics Category = "politics"
)

// Categories returns every documented Category value, including
// CategoryPolitics.
func Categories() []Category {
	return []Category{
		CategoryGeneral,
		CategoryBusiness,
		CategoryEntertainment,
		CategoryHealth,
		CategoryScience,
		CategorySports,
		CategoryTechnology,
		CategoryPolitics,
	}
}

// String returns the wire value of the category.
func (c Category) String() string { return string(c) }

// Valid reports whether the category is non-empty and contains no comma.
// Valid does not check membership in the documented set; the API may know
// more categories than this SDK.
func (c Category) Valid() bool {
	return c != "" && !strings.Contains(string(c), ",")
}

// Language is a two-letter language code accepted by the languages
// parameter.
type Language string

// Documented languages. The canonical set contains thirteen languages;
// LanguageHindi and LanguageSwedishLegacy reflect documentation
// discrepancies described in the README.
const (
	LanguageArabic     Language = "ar"
	LanguageGerman     Language = "de"
	LanguageEnglish    Language = "en"
	LanguageSpanish    Language = "es"
	LanguageFrench     Language = "fr"
	LanguageHebrew     Language = "he"
	LanguageItalian    Language = "it"
	LanguageDutch      Language = "nl"
	LanguageNorwegian  Language = "no"
	LanguagePortuguese Language = "pt"
	LanguageRussian    Language = "ru"
	LanguageSwedish    Language = "sv"
	LanguageChinese    Language = "zh"

	// LanguageHindi is listed on the API Features page but absent from the
	// Options page, which documents thirteen languages without it. The SDK
	// accepts it; see the README section on documentation discrepancies.
	LanguageHindi Language = "hi"

	// LanguageSwedishLegacy is the value the Options documentation page
	// lists for Swedish. "se" is the ISO 3166-1 code for Sweden, not a
	// language code; prefer LanguageSwedish ("sv").
	LanguageSwedishLegacy Language = "se"
)

// Languages returns the canonical set of thirteen documented languages.
// LanguageHindi and LanguageSwedishLegacy are intentionally excluded; see
// the README for why.
func Languages() []Language {
	return []Language{
		LanguageArabic,
		LanguageGerman,
		LanguageEnglish,
		LanguageSpanish,
		LanguageFrench,
		LanguageHebrew,
		LanguageItalian,
		LanguageDutch,
		LanguageNorwegian,
		LanguagePortuguese,
		LanguageRussian,
		LanguageSwedish,
		LanguageChinese,
	}
}

// String returns the wire value of the language code.
func (l Language) String() string { return string(l) }

// Valid reports whether the language code is non-empty and contains no
// comma. Valid does not check membership in the documented set.
func (l Language) Valid() bool {
	return l != "" && !strings.Contains(string(l), ",")
}

// Country is an ISO 3166-1 alpha-2 country code accepted by the countries
// parameter. The full list of supported countries is published at
// https://mediastack.com/sources. The SDK does not enumerate all codes;
// arbitrary alpha-2 values are forwarded to the API.
type Country string

// Frequently used country codes. This is a convenience subset, not the
// complete list; any ISO 3166-1 alpha-2 value is accepted.
const (
	CountryUS Country = "us"
	CountryGB Country = "gb"
	CountryDE Country = "de"
	CountryFR Country = "fr"
	CountryAU Country = "au"
	CountryCA Country = "ca"
	CountryIN Country = "in"
	CountryBR Country = "br"
	CountryJP Country = "jp"
	CountryRU Country = "ru"
)

// String returns the wire value of the country code.
func (c Country) String() string { return string(c) }

// Valid reports whether the country code is non-empty and contains no
// comma. Valid does not check membership in the ISO list.
func (c Country) Valid() bool {
	return c != "" && !strings.Contains(string(c), ",")
}

// SortOrder is a value of the sort parameter of GET /v1/news. Unlike the
// other enumerations this is a closed set: the API documents exactly
// three values and the client validates them.
type SortOrder string

// Documented sort orders.
const (
	// SortPublishedDesc orders articles from newest to oldest. It is the
	// API default when sort is omitted.
	SortPublishedDesc SortOrder = "published_desc"

	// SortPublishedAsc orders articles from oldest to newest.
	SortPublishedAsc SortOrder = "published_asc"

	// SortPopularity orders articles by popularity.
	SortPopularity SortOrder = "popularity"
)

// String returns the wire value of the sort order.
func (s SortOrder) String() string { return string(s) }

// Valid reports whether the sort order is one of the three documented
// values.
func (s SortOrder) Valid() bool {
	switch s {
	case SortPublishedDesc, SortPublishedAsc, SortPopularity:
		return true
	}
	return false
}

// DatePreset is a named value of the date parameter of GET /v1/news.
type DatePreset string

// Named date presets documented by the API.
const (
	DateToday       DatePreset = "today"
	DateThisWeek    DatePreset = "thisweek"
	DateThisMonth   DatePreset = "thismonth"
	DateLast7Days   DatePreset = "last7days"
	DateLast30Days  DatePreset = "last30days"
	DateLast3Months DatePreset = "last3months"
	DateYearToDate  DatePreset = "yeartodate"
)

// String returns the wire value of the preset.
func (p DatePreset) String() string { return string(p) }

// Valid reports whether the preset is one of the documented values.
func (p DatePreset) Valid() bool {
	switch p {
	case DateToday, DateThisWeek, DateThisMonth, DateLast7Days,
		DateLast30Days, DateLast3Months, DateYearToDate:
		return true
	}
	return false
}
