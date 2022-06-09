package cidisruptiontester

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/test/extended/util/disruption"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

// ciDisruptionUpgradeTest tests the actual CI cluster where tests are running is able to reach an external service. This
// is used to compare if we're actually observing disruption in the cluster under test, or if the CI cluster itself
// is losing networking.
//
// The service in question is maintained by the TRT team, and running on the DPCR cluster in
// the trt-monitoring namespace. (ci-disruption-tester svc)
type ciDisruptionUpgradeTest struct {
	backendDisruptionTest disruption.BackendDisruptionUpgradeTest
}

const (
	// allowedExternalDisruption is the amount of time we'll allow to flake in a CI run.
	// At present we do not have confidence that we can consistently hit an external service
	// and we do not know where the issue lies yet. Allowing 10 minutes means this test will
	// effectively never fail, but will flake if we experience ANY disruption. We can use this
	// to gather data, and correlate with real disruption in graphs.
	allowedExternalDisruption = 600 * time.Second

	externalDisruptionDescription = `CI cluster where tests are running may have network issues with new ` +
		`connections (not the cluster under test). Use intervals charts to correlate this disruption ` +
		`against observed cluster disruption and determine if we're seeing real disruption or not.`
)

func NewCIDisruptionWithNewConnectionsTest() upgrades.Test {
	ciDisruptTest := &ciDisruptionUpgradeTest{}
	backend := backenddisruption.NewSimpleBackend(
		"http://static.redhat.com/test/rhel-networkmanager.txt",
		"ci-cluster-network-liveness",
		"",
		backenddisruption.NewConnectionType)
	allowed := allowedExternalDisruption
	ciDisruptTest.backendDisruptionTest =
		disruption.NewBackendDisruptionTestWithFixedAllowedDisruption(
			"[sig-trt] CI cluster remains able to communicate with an external service with new connections",
			backend,
			&allowed,
			externalDisruptionDescription,
		)

	return ciDisruptTest
}

func NewCIDisruptionWithReusedConnectionsTest() upgrades.Test {
	ciDisruptTest := &ciDisruptionUpgradeTest{}
	backend := backenddisruption.NewSimpleBackend(
		"http://static.redhat.com/test/rhel-networkmanager.txt",
		"ci-cluster-network-liveness",
		"",
		backenddisruption.ReusedConnectionType)
	allowed := allowedExternalDisruption
	ciDisruptTest.backendDisruptionTest =
		disruption.NewBackendDisruptionTestWithFixedAllowedDisruption(
			"[sig-trt] CI cluster remains able to communicate with an external service with reused connections",
			backend,
			&allowed,
			externalDisruptionDescription,
		)

	return ciDisruptTest
}

func (t *ciDisruptionUpgradeTest) Name() string { return t.backendDisruptionTest.Name() }

func (t *ciDisruptionUpgradeTest) DisplayName() string {
	return t.backendDisruptionTest.DisplayName()
}

// Test runs a connectivity check to the service.
func (t *ciDisruptionUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	t.backendDisruptionTest.Test(f, done, upgrade)
}

func (t *ciDisruptionUpgradeTest) Teardown(f *framework.Framework) {
	t.backendDisruptionTest.Teardown(f)
}

func (t *ciDisruptionUpgradeTest) Setup(f *framework.Framework) {
	t.backendDisruptionTest.Setup(f)
}
