package urlpattern

import (
	"net/url"
	"testing"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern          string
		expectedScheme   string
		expectedHost     string
		expectedPath     string
		expectedErr      bool
		expectedMatch    []string
		expectedNotMatch []string
	}{
		{
			pattern:     ``,
			expectedErr: true,
		},
		{
			pattern:     `*://`,
			expectedErr: true,
		},
		{
			pattern:     `http://`,
			expectedErr: true,
		},
		{
			pattern:     `bad://`,
			expectedErr: true,
		},
		{
			pattern:     `*`,
			expectedErr: true,
		},
		{
			pattern:     `/*`,
			expectedErr: true,
		},
		{
			pattern:          `*://*/*`,
			expectedScheme:   `^(git|http|https|ssh)$`,
			expectedHost:     `^.*$`,
			expectedPath:     `^/.*$`,
			expectedMatch:    []string{`https://github.com/`, `https://user:password@github.com/`, `ssh://git@github.com/`},
			expectedNotMatch: []string{`ftp://github.com/`},
		},
		{
			pattern:        `http://*/*`,
			expectedScheme: `^http$`,
			expectedHost:   `^.*$`,
			expectedPath:   `^/.*$`,
		},
		{
			pattern:        `https://*/*`,
			expectedScheme: `^https$`,
			expectedHost:   `^.*$`,
			expectedPath:   `^/.*$`,
		},
		{
			pattern:        `ssh://*/*`,
			expectedScheme: `^ssh$`,
			expectedHost:   `^.*$`,
			expectedPath:   `^/.*$`,
		},
		{
			pattern:        `git://*/*`,
			expectedScheme: `^git$`,
			expectedHost:   `^.*$`,
			expectedPath:   `^/.*$`,
		},
		{
			pattern:     `bad://*/*`,
			expectedErr: true,
		},
		{
			pattern:          `https://github.com/*`,
			expectedScheme:   `^https$`,
			expectedHost:     `^github\.com$`,
			expectedPath:     `^/.*$`,
			expectedMatch:    []string{`https://github.com/`, `https://user:password@github.com/`},
			expectedNotMatch: []string{`https://test.github.com/`},
		},
		{
			pattern:        `https://*.github.com/*`,
			expectedScheme: `^https$`,
			expectedHost:   `^(?:.*\.)?github\.com$`,
			expectedPath:   `^/.*$`,
			expectedMatch:  []string{`https://github.com/`, `https://user:password@github.com/`, `https://test.github.com/`},
		},
		{
			pattern:        `https://\.+?()|[]{}^$/*`,
			expectedScheme: `^https$`,
			expectedHost:   `^\\\.\+\?\(\)\|\[\]\{\}\^\$$`,
			expectedPath:   `^/.*$`,
		},
		{
			pattern:     `https://*./*`,
			expectedErr: true,
		},
		{
			pattern:     `https://*.*.com/*`,
			expectedErr: true,
		},
		{
			pattern:     `https://git*hub.com/*`,
			expectedErr: true,
		},
		{
			pattern:     `*://git@github.com/*`,
			expectedErr: true,
		},
		{
			pattern:        `https://github.com/`,
			expectedScheme: `^https$`,
			expectedHost:   `^github\.com$`,
			expectedPath:   `^/$`,
		},
		{
			pattern:        `https://github.com/openshift/`,
			expectedScheme: `^https$`,
			expectedHost:   `^github\.com$`,
			expectedPath:   `^/openshift/$`,
		},
		{
			pattern:          `https://github.com/*/origin.git`,
			expectedScheme:   `^https$`,
			expectedHost:     `^github\.com$`,
			expectedPath:     `^/.*/origin\.git$`,
			expectedMatch:    []string{`https://github.com/openshift/origin.git`, `https://github.com/openshift/test/origin.git`},
			expectedNotMatch: []string{`https://github.com/origin.git`},
		},
		{
			pattern:          `https://github.com/*/*/.git`,
			expectedScheme:   `^https$`,
			expectedHost:     `^github\.com$`,
			expectedPath:     `^/.*/.*/\.git$`,
			expectedMatch:    []string{`https://github.com/foo/bar/.git`, `https://github.com/foo/bar/baz/.git`},
			expectedNotMatch: []string{`https://github.com/foo/.git`},
		},
		{
			pattern:        `https://github.com/\.+?()|[]{}^$`,
			expectedScheme: `^https$`,
			expectedHost:   `^github\.com$`,
			expectedPath:   `^/\\\.\+\?\(\)\|\[\]\{\}\^\$$`,
		},
	}

	for _, test := range tests {
		urlPattern, err := NewURLPattern(test.pattern)
		if (err == nil) == test.expectedErr {
			t.Errorf("test %q: expectedErr was %v but err was %q", test.pattern, test.expectedErr, err)
		}
		if err != nil {
			continue
		}

		if urlPattern.scheme != test.expectedScheme {
			t.Errorf("test %q: expectedScheme was %#q but scheme was %#q", test.pattern, test.expectedScheme, urlPattern.scheme)
		}

		if urlPattern.host != test.expectedHost {
			t.Errorf("test %q: expectedHost was %#q but host was %#q", test.pattern, test.expectedHost, urlPattern.host)
		}

		if urlPattern.path != test.expectedPath {
			t.Errorf("test %q: expectedPath was %#q but path was %#q", test.pattern, test.expectedPath, urlPattern.path)
		}

		for _, match := range test.expectedMatch {
			url, err := url.Parse(match)
			if err != nil {
				t.Fatal(err)
			}
			if !urlPattern.match(url) {
				t.Errorf("test %q: match %#q failed", test.pattern, match)
			}
		}

		for _, match := range test.expectedNotMatch {
			url, err := url.Parse(match)
			if err != nil {
				t.Fatal(err)
			}
			if urlPattern.match(url) {
				t.Errorf("test %q: match %#q succeeded", test.pattern, match)
			}
		}
	}
}

func TestMatchPatterns(t *testing.T) {
	pattern1, err := NewURLPattern("*://*.example.com/*")
	if err != nil {
		t.Fatal(err)
	}
	pattern1.Cookie = "pattern1"

	pattern2, err := NewURLPattern("*://server.example.com/*")
	if err != nil {
		t.Fatal(err)
	}
	pattern2.Cookie = "pattern2"

	url, err := url.Parse("https://server.example.com/path")
	if err != nil {
		t.Fatal(err)
	}

	match := Match([]*URLPattern{pattern1, pattern2}, url)
	if match == nil {
		t.Errorf("Match() returned nil")
	} else if match.Cookie.(string) != "pattern2" {
		t.Errorf("Match() returned %q", match.Cookie.(string))
	}
}
