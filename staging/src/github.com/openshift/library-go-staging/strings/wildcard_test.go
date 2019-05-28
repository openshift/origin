package strings

import "testing"

func TestIsWilcardMatch(t *testing.T) {
	cases := map[string]struct {
		Matching    []string
		NotMatching []string
	}{
		// Explicing word match only explicit word
		"docker": {
			Matching:    []string{"docker"},
			NotMatching: []string{"", " ", "foodocker", "dock", "dockerfoo", "  docker", "*docker*", "?ocker", "foo"},
		},
		// Just wildcard should match everything
		"*": {
			Matching:    []string{"", "foo", "docker.io", " "},
			NotMatching: []string{},
		},
		// The '???' must match exactly 3 characters
		"???": {
			Matching:    []string{"foo", "bar", "...", "   "},
			NotMatching: []string{"longfoo", "", "aa", " aaa", "aaa "},
		},
		// Wildcard prefix should match everything on the left side of the string
		"*.docker.io": {
			Matching:    []string{"registry-1.docker.io", ".docker.io", "??.docker.io", "*.docker.io"},
			NotMatching: []string{"", " ", "docker", "fakedocker.io", "docker.io", "foo.docker.io.fake"},
		},
		"foo.??.registry.url": {
			Matching:    []string{"foo.ab.registry.url"},
			NotMatching: []string{"", "foo.abc.registry.url", "registry.url", "foo.registry.url", "foo.a.registry.url"},
		},
		"*foo.??.registry.*": {
			Matching:    []string{"foo.ab.registry.url", "barfoo.xx.registry.local"},
			NotMatching: []string{"", "foo.x.registry.foo", "xfoo.registry.local", "xfoo.xx.registry"},
		},
	}

	for pattern, result := range cases {
		for _, match := range result.Matching {
			if !IsWildcardMatch(match, pattern) {
				t.Errorf("'%s': failed to match string '%s'", pattern, match)
			}
		}
		for _, match := range result.NotMatching {
			if IsWildcardMatch(match, pattern) {
				t.Errorf("'%s': failed to not match string '%s'", pattern, match)
			}
		}
	}
}
