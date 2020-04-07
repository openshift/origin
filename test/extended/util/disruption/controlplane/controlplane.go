package controlplane

import (
	"context"
	"time"

	"k8s.io/client-go/rest"

	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption"
)

// KubeAvailableTest tests that the Kubernetes control plane remains available during and after a cluster upgrade.
type KubeAvailableTest struct {
	availableTest
}

func (KubeAvailableTest) Name() string        { return "kubernetes-api-available" }
func (KubeAvailableTest) DisplayName() string { return "Kubernetes APIs remain available" }
func (t *KubeAvailableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	t.availableTest.test(f, done, upgrade, monitor.StartKubeAPIMonitoring)
}

// OpenShiftAvailableTest tests that the OpenShift APIs remains available during and after a cluster upgrade.
type OpenShiftAvailableTest struct {
	availableTest
}

func (OpenShiftAvailableTest) Name() string        { return "openshift-api-available" }
func (OpenShiftAvailableTest) DisplayName() string { return "OpenShift APIs remain available" }
func (t *OpenShiftAvailableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	t.availableTest.test(f, done, upgrade, monitor.StartOpenShiftAPIMonitoring)
}

type availableTest struct {
}

type starter func(ctx context.Context, m *monitor.Monitor, clusterConfig *rest.Config, timeout time.Duration) error

// Setup does nothing
func (t *availableTest) Setup(f *framework.Framework) {
}

// Test runs a connectivity check to the core APIs.
func (t *availableTest) test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType, startMonitoring starter) {
	config, err := framework.LoadConfig()
	framework.ExpectNoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	m := monitor.NewMonitorWithInterval(time.Second)
	err = startMonitoring(ctx, m, config, 15*time.Second)
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
func (t *availableTest) Teardown(f *framework.Framework) {
}
