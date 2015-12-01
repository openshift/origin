package networking

import (
	"testing"

	_ "github.com/openshift/origin/test/extended/networking"

	exutil "github.com/openshift/origin/test/extended/util"
)

// init initialize the extended testing suite.
func init() {
	exutil.InitTest()
}

func TestExtended(t *testing.T) {
	// Avoid using 'networking' in the suite name since that would
	// make it difficult to avoid running non-network kube e2e tests
	// via -ginkgo.focus="etworking".
	exutil.ExecuteTest(t, "Extended Network")
}
