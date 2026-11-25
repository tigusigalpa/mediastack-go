package mediastack

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

// accessKeyRe matches the access_key query parameter inside URLs and
// free text. It is intentionally broad so that keys embedded in error
// bodies or third-party messages are still caught.
var accessKeyRe = regexp.MustCompile(`access_key=[^&\s"'<>]+`)

// redactKey replaces every access_key parameter occurrence and every
// literal appearance of key in s with REDACTED. Pass an empty key to
// redact only the parameter form.
func redactKey(s, key string) string {
	if key == "" {
		return accessKeyRe.ReplaceAllString(s, "access_key=REDACTED")
	}
	escaped := url.QueryEscape(key)
	queryValue := strings.ReplaceAll(escaped, "%2C", ",")
	jsonKey, _ := json.Marshal(key)
	s = strings.NewReplacer(
		key, "REDACTED",
		escaped, "REDACTED",
		queryValue, "REDACTED",
		string(jsonKey[1:len(jsonKey)-1]), "REDACTED",
	).Replace(s)
	return accessKeyRe.ReplaceAllString(s, "access_key=REDACTED")
}

// redactURL returns a string form of u with the access_key parameter
// replaced by REDACTED. Other parameters are preserved.
func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	c := *u
	q := c.Query()
	if _, ok := q["access_key"]; ok {
		q.Set("access_key", "REDACTED")
		c.RawQuery = q.Encode()
	}
	return c.String()
}

// truncate returns at most n bytes of b.
func truncate(b string, n int) string {
	if len(b) <= n {
		return b
	}
	return b[:n]
}
