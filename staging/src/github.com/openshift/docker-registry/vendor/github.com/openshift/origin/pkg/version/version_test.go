package version

import (
	"testing"
)

func TestLastSemanticVersion(t *testing.T) {
	testCases := []struct {
		in, out string
	}{
		{"v1.3", "v1.3"},
		{"v1.3+dirty", "v1.3"},
		{"v1.3-11+abcdef-dirty", "v1.3-11"},
		{"v1.3-11+abcdef", "v1.3-11"},
		{"v1.3-11", "v1.3-11"},
		{"v1.3.0+abcdef", "v1.3.0"},
		{"v1.3+abcdef", "v1.3"},
		{"v1.3.0-alpha.1", "v1.3.0-alpha.1"},
		{"v1.3.0-alpha.1-dirty", "v1.3.0-alpha.1-dirty"},
		{"v1.3.0-alpha.1+abc-dirty", "v1.3.0-alpha.1"},
		{"v1.3.0-alpha.1+abcdef-dirty", "v1.3.0-alpha.1"},
	}
	for _, test := range testCases {
		out := Info{GitVersion: test.in}.LastSemanticVersion()
		if out != test.out {
			t.Errorf("expected %s for %s, got %s", test.out, test.in, out)
		}
	}
}
