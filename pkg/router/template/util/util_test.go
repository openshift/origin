package util

import (
	"regexp"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
)

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
		r := regexp.MustCompile(GenerateRouteRegexp(tt.hostname, tt.path, tt.wildcard))
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

func TestGenCertificateHostName(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wildcard bool
		expected string
	}{
		{
			name:     "normal host",
			hostname: "www.example.org",
			wildcard: false,
			expected: "www.example.org",
		},
		{
			name:     "wildcard host",
			hostname: "www.wildcard.test",
			wildcard: true,
			expected: "*.wildcard.test",
		},
		{
			name:     "domain",
			hostname: "example.org",
			wildcard: false,
			expected: "example.org",
		},
		{
			name:     "domain wildcard",
			hostname: "example.org",
			wildcard: true,
			expected: "*.org",
		},
		{
			name:     "tld",
			hostname: "com",
			wildcard: false,
			expected: "com",
		},
		{
			name:     "tld wildcard (not really valid usage)",
			hostname: "com",
			wildcard: true,
			expected: "com",
		},
		{
			name:     "nested host",
			hostname: "www.subdomain.org.locality.country.myco.com",
			wildcard: false,
			expected: "www.subdomain.org.locality.country.myco.com",
		},
		{
			name:     "nested host wildcard",
			hostname: "www.subdomain.org.locality.country.myco.com",
			wildcard: true,
			expected: "*.subdomain.org.locality.country.myco.com",
		},
	}

	for _, tc := range tests {
		name := GenCertificateHostName(tc.hostname, tc.wildcard)
		if name != tc.expected {
			t.Errorf("%s: expected %s to match %s, but didn't", tc.name, tc.expected, name)
		}
	}
}

func TestGenerateBackendNamePrefix(t *testing.T) {
	testPrefixes := []struct {
		name           string
		termination    routev1.TLSTerminationType
		expectedPrefix string
	}{
		{
			name:           "http route",
			termination:    routev1.TLSTerminationType(""),
			expectedPrefix: "be_http",
		},
		{
			name:           "edge secured route",
			termination:    routev1.TLSTerminationEdge,
			expectedPrefix: "be_edge_http",
		},
		{
			name:           "reencrypt route",
			termination:    routev1.TLSTerminationReencrypt,
			expectedPrefix: "be_secure",
		},
		{
			name:           "passthrough route",
			termination:    routev1.TLSTerminationPassthrough,
			expectedPrefix: "be_tcp",
		},
		{
			name:           "unknown route",
			termination:    routev1.TLSTerminationType("foo"),
			expectedPrefix: "be_http",
		},
	}

	for _, tc := range testPrefixes {
		prefix := GenerateBackendNamePrefix(tc.termination)
		if prefix != tc.expectedPrefix {
			t.Errorf("TestGenerateBackendNamePrefix: expected %s to get %s, but got %s", tc.name, tc.expectedPrefix, prefix)
		}
	}
}
