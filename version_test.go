package mediastack

import (
	"strings"
	"testing"
)

func TestVersionFormat(t *testing.T) {
	parts := strings.Split(Version, ".")
	if len(parts) != 3 {
		t.Fatalf("Version %q is not semver", Version)
	}
}

func TestUserAgent(t *testing.T) {
	ua := UserAgent()
	if !strings.HasPrefix(ua, "mediastack-go/"+Version) {
		t.Fatalf("UserAgent = %q", ua)
	}
	if !strings.Contains(ua, ModulePath) {
		t.Fatalf("UserAgent = %q must contain module path", ua)
	}
}

func TestConstants(t *testing.T) {
	if Namespace != "Tigusigalpa/Mediastack" {
		t.Fatalf("Namespace = %q", Namespace)
	}
	if DefaultBaseURL != "https://api.mediastack.com" {
		t.Fatalf("DefaultBaseURL = %q", DefaultBaseURL)
	}
	if APIVersion != "v1" || EnvAccessKey != "MEDIASTACK_ACCESS_KEY" {
		t.Fatal("constants changed")
	}
	if MaxLimit != 100 || MaxPaginationWindow != 10000 || DefaultLimit != 25 {
		t.Fatal("numeric constants changed")
	}
}
