package extensions

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterExtensionBinariesByTags(t *testing.T) {
	// Test data
	testBinaries := []TestBinary{
		{imageTag: "tests", BinaryPath: "/usr/bin/openshift-tests"},
		{imageTag: "hyperkube", BinaryPath: "/usr/bin/k8s-tests-ext.gz"},
		{imageTag: "machine-api-operator", BinaryPath: "/machine-api-tests-ext.gz"},
		{imageTag: "custom-operator", BinaryPath: "/custom-tests-ext.gz"},
	}

	tests := []struct {
		name              string
		excludeTags       string
		includeTags       string
		expectedImageTags []string
		expectedCount     int
	}{
		{
			name:              "no environment variables set",
			excludeTags:       "",
			includeTags:       "",
			expectedImageTags: []string{"tests", "hyperkube", "machine-api-operator", "custom-operator"},
			expectedCount:     4,
		},
		{
			name:              "exclude single tag",
			excludeTags:       "hyperkube",
			includeTags:       "",
			expectedImageTags: []string{"tests", "machine-api-operator", "custom-operator"},
			expectedCount:     3,
		},
		{
			name:              "exclude multiple tags",
			excludeTags:       "hyperkube,machine-api-operator",
			includeTags:       "",
			expectedImageTags: []string{"tests", "custom-operator"},
			expectedCount:     2,
		},
		{
			name:              "exclude with spaces",
			excludeTags:       " hyperkube , machine-api-operator ",
			includeTags:       "",
			expectedImageTags: []string{"tests", "custom-operator"},
			expectedCount:     2,
		},
		{
			name:              "include single tag",
			excludeTags:       "",
			includeTags:       "tests",
			expectedImageTags: []string{"tests"},
			expectedCount:     1,
		},
		{
			name:              "include multiple tags",
			excludeTags:       "",
			includeTags:       "tests,hyperkube",
			expectedImageTags: []string{"tests", "hyperkube"},
			expectedCount:     2,
		},
		{
			name:              "include with spaces",
			excludeTags:       "",
			includeTags:       " tests , hyperkube ",
			expectedImageTags: []string{"tests", "hyperkube"},
			expectedCount:     2,
		},
		{
			name:              "include takes precedence over exclude",
			excludeTags:       "tests,hyperkube",
			includeTags:       "tests",
			expectedImageTags: []string{"tests"},
			expectedCount:     1,
		},
		{
			name:              "include non-existent tag",
			excludeTags:       "",
			includeTags:       "non-existent",
			expectedImageTags: []string{},
			expectedCount:     0,
		},
		{
			name:              "exclude all tags",
			excludeTags:       "tests,hyperkube,machine-api-operator,custom-operator",
			includeTags:       "",
			expectedImageTags: []string{},
			expectedCount:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			if tt.excludeTags != "" {
				os.Setenv("EXTENSION_BINARY_OVERRIDE_EXCLUDE_TAGS", tt.excludeTags)
			} else {
				os.Unsetenv("EXTENSION_BINARY_OVERRIDE_EXCLUDE_TAGS")
			}

			if tt.includeTags != "" {
				os.Setenv("EXTENSION_BINARY_OVERRIDE_INCLUDE_TAGS", tt.includeTags)
			} else {
				os.Unsetenv("EXTENSION_BINARY_OVERRIDE_INCLUDE_TAGS")
			}

			// Call the function
			result := filterExtensionBinariesByTags(testBinaries)

			// Verify the count
			assert.Equal(t, tt.expectedCount, len(result), "Expected %d binaries, got %d", tt.expectedCount, len(result))

			// Verify the image tags
			var actualImageTags []string
			for _, binary := range result {
				actualImageTags = append(actualImageTags, binary.imageTag)
			}

			assert.ElementsMatch(t, tt.expectedImageTags, actualImageTags, "Expected image tags %v, got %v", tt.expectedImageTags, actualImageTags)

			// Clean up environment variables
			os.Unsetenv("EXTENSION_BINARY_OVERRIDE_EXCLUDE_TAGS")
			os.Unsetenv("EXTENSION_BINARY_OVERRIDE_INCLUDE_TAGS")
		})
	}
}
