package templaterouter

import (
	"regexp"
	"testing"
)

func TestFirstMatch(t *testing.T) {
	testCases := []struct {
		name    string
		pattern string
		inputs  []string
		match   string
	}{
		// Make sure we are anchoring the regex at the start and end
		{
			name:    "exact match no-substring",
			pattern: `asd`,
			inputs:  []string{"123asd123", "asd456", "123asd", "asd"},
			match:   "asd",
		},
		// Test that basic regex stuff works
		{
			name:    "don't match newline",
			pattern: `.*asd.*`,
			inputs:  []string{"123\nasd123", "123asd123", "asd"},
			match:   "123asd123",
		},
		{
			name:    "match newline",
			pattern: `(?s).*asd.*`,
			inputs:  []string{"123\nasd123", "123asd123"},
			match:   "123\nasd123",
		},
		{
			name:    "match multiline",
			pattern: `(?m)(^asd\d$\n?)+`,
			inputs:  []string{"asd1\nasd2\nasd3\n", "asd1"},
			match:   "asd1\nasd2\nasd3\n",
		},
		{
			name:    "don't match multiline",
			pattern: `(^asd\d$\n?)+`,
			inputs:  []string{"asd1\nasd2\nasd3\n", "asd1", "asd2"},
			match:   "asd1",
		},
		// Make sure that we group their pattern separately from the anchors
		{
			name:    "prefix alternation",
			pattern: `|asd`,
			inputs:  []string{"anything"},
			match:   "",
		},
		{
			name:    "postfix alternation",
			pattern: `asd|`,
			inputs:  []string{"anything"},
			match:   "",
		},
		// Make sure that a change in anchor behaviors doesn't break us
		{
			name:    "substring behavior",
			pattern: `(?m)asd`,
			inputs:  []string{"asd\n123"},
			match:   "",
		},
	}

	for _, tt := range testCases {
		match := firstMatch(tt.pattern, tt.inputs...)
		if match != tt.match {
			t.Errorf("%s: expected match of %v to %s is '%s', but didn't", tt.name, tt.inputs, tt.pattern, tt.match)
		}
	}
}

func TestGenerateRouteRegexp(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		path     string
		wildcard bool

		match   []string
		nomatch []string
	}{
		{
			name:     "no path",
			hostname: "example.com",
			path:     "",
			wildcard: false,
			match: []string{
				"example.com",
				"example.com:80",
				"example.com/",
				"example.com/sub",
				"example.com/sub/",
			},
			nomatch: []string{"other.com"},
		},
		{
			name:     "root path with trailing slash",
			hostname: "example.com",
			path:     "/",
			wildcard: false,
			match: []string{
				"example.com",
				"example.com:80",
				"example.com/",
				"example.com/sub",
				"example.com/sub/",
			},
			nomatch: []string{"other.com"},
		},
		{
			name:     "subpath with trailing slash",
			hostname: "example.com",
			path:     "/sub/",
			wildcard: false,
			match: []string{
				"example.com/sub/",
				"example.com/sub/subsub",
			},
			nomatch: []string{
				"other.com",
				"example.com",
				"example.com:80",
				"example.com/",
				"example.com/sub",    // path with trailing slash doesn't match URL without
				"example.com/subpar", // path segment boundary match required
			},
		},
		{
			name:     "subpath without trailing slash",
			hostname: "example.com",
			path:     "/sub",
			wildcard: false,
			match: []string{
				"example.com/sub",
				"example.com/sub/",
				"example.com/sub/subsub",
			},
			nomatch: []string{
				"other.com",
				"example.com",
				"example.com:80",
				"example.com/",
				"example.com/subpar", // path segment boundary match required
			},
		},
		{
			name:     "wildcard",
			hostname: "www.example.com",
			path:     "/",
			wildcard: true,
			match: []string{
				"www.example.com",
				"www.example.com/",
				"www.example.com/sub",
				"www.example.com/sub/",
				"www.example.com:80",
				"www.example.com:80/",
				"www.example.com:80/sub",
				"www.example.com:80/sub/",
				"foo.example.com",
				"foo.example.com/",
				"foo.example.com/sub",
				"foo.example.com/sub/",
			},
			nomatch: []string{
				"wwwexample.com",
				"foo.bar.example.com",
			},
		},
		{
			name:     "non-wildcard",
			hostname: "www.example.com",
			path:     "/",
			wildcard: false,
			match: []string{
				"www.example.com",
				"www.example.com/",
				"www.example.com/sub",
				"www.example.com/sub/",
				"www.example.com:80",
				"www.example.com:80/",
				"www.example.com:80/sub",
				"www.example.com:80/sub/",
			},
			nomatch: []string{
				"foo.example.com",
				"foo.example.com/",
				"foo.example.com/sub",
				"foo.example.com/sub/",
				"wwwexample.com",
				"foo.bar.example.com",
			},
		},
	}

	for _, tt := range tests {
		r := regexp.MustCompile(generateRouteRegexp(tt.hostname, tt.path, tt.wildcard))
		for _, s := range tt.match {
			if !r.Match([]byte(s)) {
				t.Errorf("%s: expected %s to match %s, but didn't", tt.name, r, s)
			}
		}
		for _, s := range tt.nomatch {
			if r.Match([]byte(s)) {
				t.Errorf("%s: expected %s not to match %s, but did", tt.name, r, s)
			}
		}
	}
}

func TestMatchPattern(t *testing.T) {
	testMatches := []struct {
		name    string
		pattern string
		input   string
	}{
		// Test that basic regex stuff works
		{
			name:    "exact match",
			pattern: `asd`,
			input:   "asd",
		},
		{
			name:    "basic regex",
			pattern: `.*asd.*`,
			input:   "123asd123",
		},
		{
			name:    "match newline",
			pattern: `(?s).*asd.*`,
			input:   "123\nasd123",
		},
		{
			name:    "match multiline",
			pattern: `(?m)(^asd\d$\n?)+`,
			input:   "asd1\nasd2\nasd3\n",
		},
	}

	testNoMatches := []struct {
		name    string
		pattern string
		input   string
	}{
		// Make sure we are anchoring the regex at the start and end
		{
			name:    "no-substring",
			pattern: `asd`,
			input:   "123asd123",
		},
		// Make sure that we group their pattern separately from the anchors
		{
			name:    "prefix alternation",
			pattern: `|asd`,
			input:   "anything",
		},
		{
			name:    "postfix alternation",
			pattern: `asd|`,
			input:   "anything",
		},
		// Make sure that a change in anchor behaviors doesn't break us
		{
			name:    "substring behavior",
			pattern: `(?m)asd`,
			input:   "asd\n123",
		},
		// Check some other regex things that should fail
		{
			name:    "don't match newline",
			pattern: `.*asd.*`,
			input:   "123\nasd123",
		},
		{
			name:    "don't match multiline",
			pattern: `(^asd\d$\n?)+`,
			input:   "asd1\nasd2\nasd3\n",
		},
	}

	for _, tt := range testMatches {
		match := matchPattern(tt.pattern, tt.input)
		if !match {
			t.Errorf("%s: expected %s to match %s, but didn't", tt.name, tt.input, tt.pattern)
		}
	}

	for _, tt := range testNoMatches {
		match := matchPattern(tt.pattern, tt.input)
		if match {
			t.Errorf("%s: expected %s not to match %s, but did", tt.name, tt.input, tt.pattern)
		}
	}
}
