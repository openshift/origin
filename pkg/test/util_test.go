package test

import (
	_ "embed"
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

//go:embed junit_upgrade_1674689374.xml
var badXML []byte

// TestStripANSI tests removal of ANSI control sequences. In some cases, ANSI control sequences
// for displaying colors may end up in test outputs, so we need to ensure they are stripped out. If
// they are not, most XML parsers will consider the XML unparsable.
func TestStripANSI(t *testing.T) {
	var suite junitapi.JUnitTestSuite

	err := xml.Unmarshal(badXML, &suite)
	assert.Error(t, err)

	result := StripANSI(badXML)
	err = xml.Unmarshal(result, &suite)
	assert.NoError(t, err)
}
