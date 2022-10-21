package monitorapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocatorParts(t *testing.T) {
	assert.Equal(t,
		map[string]string{
			"e2e-test": "\"test a\"",
		},
		LocatorParts(`e2e-test/"test a"`))
	assert.Equal(t,
		map[string]string{
			"e2e-test":   "\"test a\"",
			"status":     "Passed",
			"jUnitSuite": "openshift-tests-upgrade",
		},
		LocatorParts(`e2e-test/"test a" jUnitSuite/openshift-tests-upgrade status/Passed`))
}
