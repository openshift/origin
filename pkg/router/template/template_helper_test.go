package templaterouter

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
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

func createTempMapFile(prefix string, data []string) (string, error) {
	name := ""
	tempFile, err := ioutil.TempFile("", prefix)
	if err != nil {
		return "", fmt.Errorf("unexpected error creating temp file: %v", err)
	}

	name = tempFile.Name()
	if err = tempFile.Close(); err != nil {
		return name, fmt.Errorf("unexpected error creating temp file: %v", err)
	}

	if err := ioutil.WriteFile(name, []byte(strings.Join(data, "\n")), 0664); err != nil {
		return name, fmt.Errorf("unexpected error writing temp file %s: %v", name, err)
	}

	return name, nil
}

func TestSortMapData(t *testing.T) {
	testData := []string{
		`^api-stg\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ stg:api-route`,
		`^api-prod\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ prod:api-route`,
		`^[^\.]*\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ prod:wildcard-route`,
		`^3dev\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ dev:api-route`,
		`^api-prod\.127\.0\.0\.1\.nip\.io(:[0-9]+)?/x/y/z(/.*)?$ prod:api-path-route`,
		`^3app-admin\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ dev:admin-route`,
		`^[^\.]*\.foo\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ devel2:foo-wildcard-route`,
		`^zzz-production\.wildcard\.test(:[0-9]+)?/x/y/z(/.*)?$ test:api-route`,
		`^backend-app\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ prod:backend-route`,
		`^[^\.]*\.foo\.wildcard\.test(:[0-9]+)?(/.*)?$ devel2:foo-wildcard-test`,
	}

	expectedOrder := []string{
		"test:api-route",
		"prod:backend-route",
		"stg:api-route",
		"prod:api-path-route",
		"prod:api-route",
		"dev:api-route",
		"dev:admin-route",
		"devel2:foo-wildcard-test",
		"devel2:foo-wildcard-route",
		"prod:wildcard-route",
	}

	fileName, err := createTempMapFile("sort-map-data-test", testData)
	if err != nil {
		t.Errorf("TestSortMapData error: %v", err)
	}

	lines := sortedMapData(fileName, true)
	if len(lines) != len(expectedOrder) {
		t.Errorf("TestSortMapData sorted data length %d did not match expected length %d",
			len(lines), len(expectedOrder))
	}
	for idx, suffix := range expectedOrder {
		if !strings.HasSuffix(lines[idx], suffix) {
			t.Errorf("TestSortMapData sorted data %s at index %d did not match expectation %s",
				lines[idx], idx, suffix)
		}
	}
}

func TestSortMapCertConfigData(t *testing.T) {
	testData := []string{
		`/path/to/certs/stg:api-route.pem stg:api-route`,
		`/path/to/certs/prod:api-route.pem prod:api-route`,
		`/path/to/certs/prod:wildcard-route.pem prod:wildcard-route`,
		`/path/to/certs/dev:api-route.pem dev:api-route`,
		`/path/to/certs/prod:api-path-route prod:api-path-route`,
		`/path/to/certs/dev:admin-route.pem dev:admin-route`,
		`/path/to/certs/devel2:foo-wildcard-route.pem devel2:foo-wildcard-route`,
		`/path/to/certs/test:api-route.pem test:api-route`,
		`/path/to/certs/prod:backend-route.pem prod:backend-route`,
		`/path/to/certs/devel2:foo-wildcard-test.pem devel2:foo-wildcard-test`,
	}

	expectedOrder := []string{
		"test:api-route",
		"stg:api-route",
		"prod:wildcard-route",
		"prod:backend-route",
		"prod:api-route",
		"prod:api-path-route",
		"devel2:foo-wildcard-test",
		"devel2:foo-wildcard-route",
		"dev:api-route",
		"dev:admin-route",
	}

	fileName, err := createTempMapFile("sort-map-cert-config-test", testData)
	if err != nil {
		t.Errorf("TestSortMapCertConfigData error: %v", err)
	}

	lines := sortedMapData(fileName, false)
	if len(lines) != len(expectedOrder) {
		t.Errorf("TestSortMapCertConfigData sorted data length %d did not match expected length %d",
			len(lines), len(expectedOrder))
	}
	for idx, suffix := range expectedOrder {
		if !strings.HasSuffix(lines[idx], suffix) {
			t.Errorf("TestSortMapCertConfigData sorted data %s at index %d did not match expectation %s",
				lines[idx], idx, suffix)
		}
	}
}
