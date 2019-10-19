package controlplane

import (
	"context"
	"time"

	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	"github.com/openshift/origin/pkg/monitor"
)

// AvailableTest tests that the control plane remains is available
// before and after a cluster upgrade.
type AvailableTest struct {
}

// Name returns the tracking name of the test.
func (AvailableTest) Name() string { return "control-plane-upgrade" }

// Setup does nothing
func (t *AvailableTest) Setup(f *framework.Framework) {
}

// Test runs a connectivity check to the core APIs.
func (t *AvailableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	config, err := framework.LoadConfig()
	framework.ExpectNoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	m := monitor.NewMonitor()
	err = monitor.StartAPIMonitoring(ctx, m, config)
	framework.ExpectNoError(err, "unable to monitor API")
	m.StartSampling(ctx)

	// wait to ensure API is still up after the test ends
	<-done
	time.Sleep(15 * time.Second)
	cancel()

	conditions := m.Conditions(time.Time{}, time.Time{})
	for _, interval := range conditions {
		framework.Logf("Condition: %s", interval)
	}
	if len(conditions) > 0 {
		framework.Failf("API was down during upgrade")
	}

}

// Teardown cleans up any remaining resources.
func (t *AvailableTest) Teardown(f *framework.Framework) {
}
