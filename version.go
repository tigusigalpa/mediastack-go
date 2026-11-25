package mediastack

// Version is the semantic version of this SDK. It is a variable, not a
// constant, so release builds can override it with:
//
//	go build -ldflags "-X github.com/tigusigalpa/mediastack-go.Version=1.2.3"
var Version = "1.0.1"

// UserAgent returns the value sent in the User-Agent request header.
// It embeds Version, so it tracks -ldflags overrides automatically.
func UserAgent() string {
	return "mediastack-go/" + Version + " (+https://github.com/tigusigalpa/mediastack-go)"
}
