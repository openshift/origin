package versioning

import (
	"testing"

	"github.com/blang/semver"
)

func TestBetween(t *testing.T) {
	tests := []struct {
		name         string
		versionRange VersionRange
		needle       semver.Version

		expected bool
	}{
		{
			name:         "over",
			versionRange: NewRangeOrDie("1.1.0", "1.2.0"),
			needle:       semver.MustParse("1.2.0"),
			expected:     false,
		},
		{
			name:         "under",
			versionRange: NewRangeOrDie("1.1.0", "1.2.0"),
			needle:       semver.MustParse("1.0.10"),
			expected:     false,
		},
		{
			name:         "boundary",
			versionRange: NewRangeOrDie("1.1.0", "1.2.0"),
			needle:       semver.MustParse("1.1.0"),
			expected:     true,
		},
		{
			name:         "in",
			versionRange: NewRangeOrDie("1.1.0", "1.2.0"),
			needle:       semver.MustParse("1.1.1"),
			expected:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := test.versionRange.Between(&test.needle)
			if test.expected != actual {
				t.Errorf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}
