package controlplane

import (
	"context"
	"time"

	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption"
)

// AvailableTest tests that the control plane remains is available
// before and after a cluster upgrade.
type AvailableTest struct {
}

func (AvailableTest) Name() string        { return "control-plane-available" }
func (AvailableTest) DisplayName() string { return "Kubernetes and OpenShift APIs remain available" }

// Setup does nothing
func (t *AvailableTest) Setup(f *framework.Framework) {
}

// Test runs a connectivity check to the core APIs.
func (t *AvailableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	config, err := framework.LoadConfig()
	framework.ExpectNoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	m := monitor.NewMonitorWithInterval(time.Second)
	err = monitor.StartAPIMonitoring(ctx, m, config, 15*time.Second)
	framework.ExpectNoError(err, "unable to monitor API")

	start := time.Now()
	m.StartSampling(ctx)

	// wait to ensure API is still up after the test ends
	<-done
	time.Sleep(15 * time.Second)
	cancel()
	end := time.Now()

	disruption.ExpectNoDisruption(f, 0.08, end.Sub(start), m.Events(time.Time{}, time.Time{}), "API was unreachable during disruption")
}

// Teardown cleans up any remaining resources.
func (t *AvailableTest) Teardown(f *framework.Framework) {
}
