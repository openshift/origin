package links

import (
	"reflect"
	"testing"
)

func TestLinks(t *testing.T) {
	testcases := map[string]struct {
		header string
		links  map[string]string
	}{
		"empty": {
			header: "",
			links:  map[string]string{},
		},
		"unparseable": {
			header: "foo bar baz",
			links:  map[string]string{},
		},
		"single link": {
			header: `<https://example.com/user/orgs?per_page=1&page=2>; rel="next"`,
			links: map[string]string{
				"next": "https://example.com/user/orgs?per_page=1&page=2",
			},
		},
		"single link with unknown suffix": {
			header: `<https://example.com/user/orgs?per_page=1&page=2>; rel="next", foo bar baz`,
			links: map[string]string{
				"next": "https://example.com/user/orgs?per_page=1&page=2",
			},
		},
		"duplicate link": {
			header: `<https://example.com/user/orgs?per_page=1&page=2>; rel="next", <https://example.com/user/orgs?per_page=1&page=3>; rel="next"`,
			links: map[string]string{
				"next": "https://example.com/user/orgs?per_page=1&page=3",
			},
		},
		"no whitespace": {
			header: `<https://example.com/user/orgs?per_page=1&page=2>;rel="next",<https://example.com/user/orgs?per_page=1&page=8>;rel="last"`,
			links: map[string]string{
				"next": "https://example.com/user/orgs?per_page=1&page=2",
				"last": "https://example.com/user/orgs?per_page=1&page=8",
			},
		},
		"extra whitespace": {
			header: `  <https://example.com/user/orgs?per_page=1&page=2>;  rel="next"  ,		<https://example.com/user/orgs?per_page=1&page=8>		;		rel="last"		`,
			links: map[string]string{
				"next": "https://example.com/user/orgs?per_page=1&page=2",
				"last": "https://example.com/user/orgs?per_page=1&page=8",
			},
		},
	}

	for k, tc := range testcases {
		links := ParseLinks(tc.header)
		if !reflect.DeepEqual(links, tc.links) {
			t.Errorf("%s: Expected\n%#v\ngot\n%#v", k, tc.links, links)
		}
	}
}
