package links

import "regexp"

// Matches URL+rel links defined by https://tools.ietf.org/html/rfc5988
// Examples header values:
//   <http://www.example.com/foo?page=3>; rel="next"
//   <http://www.example.com/foo?page=3>; rel="next", <http://www.example.com/foo?page=1>; rel="prev"
var linkRegex = regexp.MustCompile(`\<(.+?)\>\s*;\s*rel="(.+?)"(?:\s*,\s*)?`)

// ParseLinks extracts link relations from the given header value.
func ParseLinks(header string) map[string]string {
	links := map[string]string{}
	if len(header) == 0 {
		return links
	}

	matches := linkRegex.FindAllStringSubmatch(header, -1)
	for _, match := range matches {
		url := match[1]
		rel := match[2]
		links[rel] = url
	}
	return links
}
