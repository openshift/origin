package extensions

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterExtensionBinariesByTags(t *testing.T) {
	// Test data
	testBinaries := []TestBinary{
		{imageTag: "tests", binaryPath: "/usr/bin/openshift-tests"},
		{imageTag: "hyperkube", binaryPath: "/usr/bin/k8s-tests-ext.gz"},
		{imageTag: "machine-api-operator", binaryPath: "/machine-api-tests-ext.gz"},
		{imageTag: "custom-operator", binaryPath: "/custom-tests-ext.gz"},
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
			t.Setenv("EXTENSION_BINARY_OVERRIDE_EXCLUDE_TAGS", tt.excludeTags)
			t.Setenv("EXTENSION_BINARY_OVERRIDE_INCLUDE_TAGS", tt.includeTags)

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
		})
	}
}

func TestExtensionBinaryArchitecture(t *testing.T) {
	tests := []struct {
		name                string
		setOCPArchitecture  bool
		ocpArchitecture     string
		runtimeArchitecture string
		wantArchitecture    string
	}{
		{
			name:                "arm64 target overrides amd64 runtime",
			setOCPArchitecture:  true,
			ocpArchitecture:     "arm64",
			runtimeArchitecture: "amd64",
			wantArchitecture:    "arm64",
		},
		{
			name:                "amd64 target is honored",
			setOCPArchitecture:  true,
			ocpArchitecture:     "amd64",
			runtimeArchitecture: "arm64",
			wantArchitecture:    "amd64",
		},
		{
			name:                "unset target uses runtime architecture",
			runtimeArchitecture: runtime.GOARCH,
			wantArchitecture:    runtime.GOARCH,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OCP_ARCH", tt.ocpArchitecture)
			if !tt.setOCPArchitecture {
				assert.NoError(t, os.Unsetenv("OCP_ARCH"))
			}

			assert.Equal(t, tt.wantArchitecture, extensionBinaryArchitecture(tt.runtimeArchitecture))
		})
	}
}

func TestFilterExtensionBinariesByArchitecture(t *testing.T) {
	tests := []struct {
		name          string
		architectures []string
		architecture  string
		wantIncluded  bool
	}{
		{
			name:         "empty allowlist is available on all architectures",
			architecture: "arm64",
			wantIncluded: true,
		},
		{
			name:          "matching architecture is included",
			architectures: []string{"amd64", "arm64"},
			architecture:  "arm64",
			wantIncluded:  true,
		},
		{
			name:          "non-matching architecture is omitted",
			architectures: []string{"amd64"},
			architecture:  "arm64",
			wantIncluded:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binary := TestBinary{
				imageTag:      "test-extension",
				binaryPath:    "/usr/bin/test-extension",
				architectures: tt.architectures,
			}

			filtered := filterExtensionBinariesByArchitecture([]TestBinary{binary}, tt.architecture)

			assert.Equal(t, tt.wantIncluded, len(filtered) == 1)
		})
	}
}

func TestVSphereExtensionBinaryArchitectures(t *testing.T) {
	tests := []struct {
		name         string
		architecture string
		wantIncluded bool
	}{
		{
			name:         "included on amd64",
			architecture: "amd64",
			wantIncluded: true,
		},
		{
			name:         "omitted on arm64",
			architecture: "arm64",
			wantIncluded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := filterExtensionBinariesByArchitecture(extensionBinaries, tt.architecture)
			included := false
			for _, binary := range filtered {
				if binary.imageTag == "vsphere-csi-driver-operator" {
					included = true
					assert.Equal(t, []string{"amd64"}, binary.architectures)
					break
				}
			}

			assert.Equal(t, tt.wantIncluded, included)
		})
	}
}
