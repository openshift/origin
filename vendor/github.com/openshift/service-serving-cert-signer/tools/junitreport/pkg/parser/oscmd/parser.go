package oscmd

import (
	"github.com/openshift/service-serving-cert-signer/tools/junitreport/pkg/builder"
	"github.com/openshift/service-serving-cert-signer/tools/junitreport/pkg/parser"
	"github.com/openshift/service-serving-cert-signer/tools/junitreport/pkg/parser/stack"
)

// NewParser returns a new parser that's capable of parsing `os::cmd` test output
func NewParser(builder builder.TestSuitesBuilder, stream bool) parser.TestOutputParser {
	return stack.NewParser(builder, newTestDataParser(), newTestSuiteDataParser(), stream)
}
