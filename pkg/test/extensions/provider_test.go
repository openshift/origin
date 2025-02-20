package extensions

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBinaryPathOverride(t *testing.T) {
	tests := []struct {
		name         string
		imageTag     string
		binaryPath   string
		envVars      map[string]string
		expectedPath string
	}{
		{
			name:       "Global override for image",
			imageTag:   "hyperkube",
			binaryPath: "/usr/bin/k8s-tests-ext",
			envVars: map[string]string{
				"EXTENSION_BINARY_OVERRIDE_HYPERKUBE": "/custom/global/path",
			},
			expectedPath: "/custom/global/path",
		},
		{
			name:       "Specific override for binary",
			imageTag:   "hyperkube",
			binaryPath: "/usr/bin/k8s-tests-ext",
			envVars: map[string]string{
				"EXTENSION_BINARY_OVERRIDE_HYPERKUBE_USR_BIN_K8S_TESTS_EXT": "/custom/specific/path",
			},
			expectedPath: "/custom/specific/path",
		},
		{
			name:         "No overrides",
			imageTag:     "hyperkube",
			binaryPath:   "/usr/bin/k8s-tests-ext",
			envVars:      map[string]string{},
			expectedPath: "",
		},
		{
			name:       "Specific override takes precedence over global",
			imageTag:   "hyperkube",
			binaryPath: "/usr/bin/k8s-tests-ext",
			envVars: map[string]string{
				"EXTENSION_BINARY_OVERRIDE_HYPERKUBE":                       "/custom/global/path",
				"EXTENSION_BINARY_OVERRIDE_HYPERKUBE_USR_BIN_K8S_TESTS_EXT": "/custom/specific/path",
			},
			expectedPath: "/custom/specific/path",
		},
		{
			name:       "Special characters in image and binary path",
			imageTag:   "special-image",
			binaryPath: "/usr/local/bin/special-tests-1.2.3",
			envVars: map[string]string{
				"EXTENSION_BINARY_OVERRIDE_SPECIAL_IMAGE_USR_LOCAL_BIN_SPECIAL_TESTS_1_2_3": "/custom/path/for-special",
			},
			expectedPath: "/custom/path/for-special",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				err := os.Setenv(k, v)
				assert.NoError(t, err)
			}

			result := binaryPathOverride(tt.imageTag, tt.binaryPath)

			if result != tt.expectedPath {
				t.Errorf("binaryPathOverride(%q, %q) = %q; want %q",
					tt.imageTag, tt.binaryPath, result, tt.expectedPath)
			}

			for k := range tt.envVars {
				err := os.Unsetenv(k)
				assert.NoError(t, err)
			}
		})
	}
}
