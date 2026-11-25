package mediastack

import (
	"net/http"
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
	s = accessKeyRe.ReplaceAllString(s, "access_key=REDACTED")
	if key != "" {
		s = strings.ReplaceAll(s, key, "REDACTED")
	}
	return s
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

// redactHeader returns a copy of h with access_key occurrences replaced
// by REDACTED in every value.
func redactHeader(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, vs := range h {
		cp := make([]string, len(vs))
		for i, v := range vs {
			cp[i] = redactKey(v, "")
		}
		out[k] = cp
	}
	return out
}

// redactBody returns body with access_key occurrences replaced. It is
// applied to response bodies retained in APIError.Body so that %+v
// dumps and error chains can never leak the key.
func redactBody(body []byte) []byte {
	return accessKeyRe.ReplaceAll(body, []byte("access_key=REDACTED"))
}

// truncate returns at most n bytes of b.
func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}
