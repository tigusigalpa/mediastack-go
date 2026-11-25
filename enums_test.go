package mediastack

import "testing"

func TestCategoryValid(t *testing.T) {
	if !CategoryBusiness.Valid() {
		t.Fatal("business must be valid")
	}
	if Category("").Valid() {
		t.Fatal("empty must be invalid")
	}
	if Category("a,b").Valid() {
		t.Fatal("comma value must be invalid")
	}
	if !Category("anything").Valid() {
		t.Fatal("unknown but well-formed value must be valid")
	}
}

func TestEnumStrings(t *testing.T) {
	if CategoryTechnology.String() != "technology" {
		t.Fatal("Category.String")
	}
	if LanguageEnglish.String() != "en" {
		t.Fatal("Language.String")
	}
	if CountryUS.String() != "us" {
		t.Fatal("Country.String")
	}
	if SortPopularity.String() != "popularity" {
		t.Fatal("SortOrder.String")
	}
	if DateToday.String() != "today" {
		t.Fatal("DatePreset.String")
	}
}

func TestSortOrderValid(t *testing.T) {
	for _, s := range []SortOrder{SortPublishedDesc, SortPublishedAsc, SortPopularity} {
		if !s.Valid() {
			t.Fatalf("%s must be valid", s)
		}
	}
	for _, s := range []SortOrder{"", "newest", "published"} {
		if s.Valid() {
			t.Fatalf("%q must be invalid", s)
		}
	}
}

func TestDatePresetValid(t *testing.T) {
	for _, p := range []DatePreset{
		DateToday, DateThisWeek, DateThisMonth, DateLast7Days,
		DateLast30Days, DateLast3Months, DateYearToDate,
	} {
		if !p.Valid() {
			t.Fatalf("%s must be valid", p)
		}
	}
	if DatePreset("yesterday").Valid() {
		t.Fatal("yesterday must be invalid")
	}
}

func TestLanguageSets(t *testing.T) {
	if got := len(Languages()); got != 13 {
		t.Fatalf("canonical set must have 13 languages, got %d", got)
	}
	for _, l := range Languages() {
		if l == LanguageHindi || l == LanguageSwedishLegacy {
			t.Fatalf("%s must not be in the canonical set", l)
		}
	}
	if !LanguageHindi.Valid() || !LanguageSwedishLegacy.Valid() {
		t.Fatal("hi and se must still be Valid (non-empty, no comma)")
	}
}

func TestCategoriesList(t *testing.T) {
	found := false
	for _, c := range Categories() {
		if c == CategoryPolitics {
			found = true
		}
	}
	if !found {
		t.Fatal("politics must be in Categories()")
	}
	if len(Categories()) != 8 {
		t.Fatalf("want 8 categories, got %d", len(Categories()))
	}
}
