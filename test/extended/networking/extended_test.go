package networking

import (
	"testing"

	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e"
)

// init initialize the extended testing suite.
func init() {
	// Don't initialize the flags for upstream E2Es, we only care about
	// running the extended networking tests.
	exutil.InitTest()
}

func TestExtended(t *testing.T) {
	e2e.RunE2ETests(t)
}
