package extended

import (
	"testing"

	_ "k8s.io/kubernetes/test/e2e"

	// test sources
	_ "k8s.io/kubernetes/test/e2e/apimachinery"
	_ "k8s.io/kubernetes/test/e2e/apps"
	_ "k8s.io/kubernetes/test/e2e/auth"
	_ "k8s.io/kubernetes/test/e2e/autoscaling"
	_ "k8s.io/kubernetes/test/e2e/common"
	_ "k8s.io/kubernetes/test/e2e/instrumentation"
	_ "k8s.io/kubernetes/test/e2e/kubectl"
	// _ "k8s.io/kubernetes/test/e2e/lifecycle"
	// _ "k8s.io/kubernetes/test/e2e/lifecycle/bootstrap"
	// _ "k8s.io/kubernetes/test/e2e/multicluster"
	_ "k8s.io/kubernetes/test/e2e/network"
	_ "k8s.io/kubernetes/test/e2e/node"
	_ "k8s.io/kubernetes/test/e2e/scalability"
	_ "k8s.io/kubernetes/test/e2e/scheduling"
	_ "k8s.io/kubernetes/test/e2e/servicecatalog"
	_ "k8s.io/kubernetes/test/e2e/storage"
	// _ "k8s.io/kubernetes/test/e2e/ui"

	_ "github.com/openshift/origin/test/extended/builds"
	_ "github.com/openshift/origin/test/extended/cli"
	_ "github.com/openshift/origin/test/extended/cluster"
	_ "github.com/openshift/origin/test/extended/deployments"
	_ "github.com/openshift/origin/test/extended/dns"
	_ "github.com/openshift/origin/test/extended/idling"
	_ "github.com/openshift/origin/test/extended/image_ecosystem"
	_ "github.com/openshift/origin/test/extended/imageapis"
	_ "github.com/openshift/origin/test/extended/images"
	_ "github.com/openshift/origin/test/extended/images/trigger"
	_ "github.com/openshift/origin/test/extended/jobs"
	_ "github.com/openshift/origin/test/extended/localquota"
	_ "github.com/openshift/origin/test/extended/networking"
	_ "github.com/openshift/origin/test/extended/prometheus"
	_ "github.com/openshift/origin/test/extended/router"
	_ "github.com/openshift/origin/test/extended/security"
	_ "github.com/openshift/origin/test/extended/templates"

	exutil "github.com/openshift/origin/test/extended/util"
)

// init initialize the extended testing suite.
func init() {
	exutil.InitTest()
}

func TestExtended(t *testing.T) {
	exutil.ExecuteTest(t, "Extended")
}
