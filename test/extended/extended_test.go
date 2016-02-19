package extended

import (
	"testing"

	_ "github.com/openshift/origin/test/extended/builds"
	_ "github.com/openshift/origin/test/extended/cli"
	_ "github.com/openshift/origin/test/extended/deployments"
	_ "github.com/openshift/origin/test/extended/images"
	_ "github.com/openshift/origin/test/extended/jenkins"
	_ "github.com/openshift/origin/test/extended/jobs"
	_ "github.com/openshift/origin/test/extended/router"
	_ "github.com/openshift/origin/test/extended/security"
	e2e "k8s.io/kubernetes/test/e2e"

	exutil "github.com/openshift/origin/test/extended/util"
)

// init initialize the extended testing suite.
func init() {
	// Kubernetes: Pure end to end tests ~ These flags pass through.
	e2e.RegisterFlags()
	exutil.InitTest()
}

func TestExtended(t *testing.T) {
	// TODO Send a second arg once a name is supported upstream.
	e2e.RunE2ETests(t)
}
