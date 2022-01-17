package dns

import (
	"github.com/openshift/origin/test/extended/util/disruption"
	v1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/service"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

type dnsUpgradeTest struct {
	jig		*service.TestJig
	dnsService	*v1.Service
	backendDisruptionTest disruption.BackendDisruptionUpgradeTest
}

func dnsAvailabilityTest() upgrades.Test {
	dnsTest := &dnsUpgradeTest{}
	// Get existing DNS service for the clusterIP
	// run a dig/nslookup against the clusterIP
		// should we retry on failure?
		// log why it failed
	return dnsTest
}

// Name should return a test name sans spaces.
func (t *dnsUpgradeTest) Name() string { return t.backendDisruptionTest.Name() }

// Setup should create and verify whatever objects need to
// exist before the upgrade disruption starts.
func (t *dnsUpgradeTest) Setup(f *framework.Framework) {

}

// Test will run during the upgrade. When the upgrade is
// complete, done will be closed and final validation can
// begin.
func (t *dnsUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {

}

// Teardown should clean up any objects that are created that
// aren't already cleaned up by the framework. This will
// always be called, even if Setup failed.
func (t *dnsUpgradeTest) Teardown(f *framework.Framework) {

}