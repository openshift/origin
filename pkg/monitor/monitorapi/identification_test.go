package monitorapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocatorParts(t *testing.T) {
	tests := []struct {
		name          string
		locator       string
		expectedParts map[string]string
	}{
		{
			name:    "multi-word test name",
			locator: `e2e-test/"test a"`,
			expectedParts: map[string]string{
				"e2e-test": "\"test a\"",
			},
		},
		{
			name:    "multi-word test name with other tags",
			locator: `e2e-test/"test a" jUnitSuite/openshift-tests-upgrade status/Passed`,
			expectedParts: map[string]string{
				"e2e-test":   "\"test a\"",
				"jUnitSuite": "openshift-tests-upgrade",
				"status":     "Passed",
			},
		},
		{
			name:    "multi-word test name with quotes",
			locator: `e2e-test/"[sig-builds][Feature:Builds] result image \\"test-docker-build.json\\" something something [apigroup:build.openshift.io][apigroup:image.openshift.io]\"" jUnitSuite/openshift-tests-upgrade status/Passed`,
			expectedParts: map[string]string{
				"e2e-test":   "\"[sig-builds][Feature:Builds] result image \\\"test-docker-build.json\\\" something something [apigroup:build.openshift.io][apigroup:image.openshift.io]\"",
				"status":     "Passed",
				"jUnitSuite": "openshift-tests-upgrade",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := LocatorParts(tc.locator)
			assert.Equal(t, tc.expectedParts["e2e-test"], result["e2e-test"])
		})
	}
}
