package controlplane

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/rest"

	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption"
)

// NewKubeAvailableWithNewConnectionsTest tests that the Kubernetes control plane remains available during and after a cluster upgrade.
func NewKubeAvailableWithNewConnectionsTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-api-machinery] Kubernetes APIs remain available for new connections",
		name:            "kubernetes-api-available-new-connections",
		startMonitoring: monitor.StartKubeAPIMonitoringWithNewConnections,
	}
}

// NewOpenShiftAvailableNewConnectionsTest tests that the OpenShift APIs remains available during and after a cluster upgrade.
func NewOpenShiftAvailableNewConnectionsTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-api-machinery] OpenShift APIs remain available for new connections",
		name:            "openshift-api-available-new-connections",
		startMonitoring: monitor.StartOpenShiftAPIMonitoringWithNewConnections,
	}
}

// NewOAuthAvailableNewConnectionsTest tests that the OAuth APIs remains available during and after a cluster upgrade.
func NewOAuthAvailableNewConnectionsTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-api-machinery] OAuth APIs remain available for new connections",
		name:            "oauth-api-available-new-connections",
		startMonitoring: monitor.StartOpenShiftAPIMonitoringWithNewConnections,
	}
}

// NewKubeAvailableWithConnectionReuseTest tests that the Kubernetes control plane remains available during and after a cluster upgrade.
func NewKubeAvailableWithConnectionReuseTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-api-machinery] Kubernetes APIs remain available with reused connections",
		name:            "kubernetes-api-available-reused-connections",
		startMonitoring: monitor.StartKubeAPIMonitoringWithConnectionReuse,
	}
}

// NewOpenShiftAvailableTest tests that the OpenShift APIs remains available during and after a cluster upgrade.
func NewOpenShiftAvailableWithConnectionReuseTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-api-machinery] OpenShift APIs remain available with reused connections",
		name:            "openshift-api-available-reused-connections",
		startMonitoring: monitor.StartOpenShiftAPIMonitoringWithConnectionReuse,
	}
}

// NewOauthAvailableTest tests that the OAuth APIs remains available during and after a cluster upgrade.
func NewOAuthAvailableWithConnectionReuseTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-api-machinery] OAuth APIs remain available with reused connections",
		name:            "oauth-api-available-reused-connections",
		startMonitoring: monitor.StartOpenShiftAPIMonitoringWithConnectionReuse,
	}
}

type availableTest struct {
	// testName is the name to show in unit
	testName string
	// name helps distinguish which API server in particular is unavailable.
	name            string
	startMonitoring starter
}

type starter func(ctx context.Context, m *monitor.Monitor, clusterConfig *rest.Config, timeout time.Duration) error

func (t availableTest) Name() string { return t.name }
func (t availableTest) DisplayName() string {
	return t.testName
}

// Setup does nothing
func (t *availableTest) Setup(f *framework.Framework) {
}

// Test runs a connectivity check to the core APIs.
func (t *availableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	config, err := framework.LoadConfig()
	framework.ExpectNoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	m := monitor.NewMonitorWithInterval(time.Second)
	err = t.startMonitoring(ctx, m, config, 15*time.Second)
	framework.ExpectNoError(err, "unable to monitor API")

	start := time.Now()
	m.StartSampling(ctx)

	// wait to ensure API is still up after the test ends
	<-done
	time.Sleep(15 * time.Second)
	cancel()
	end := time.Now()

	disruption.ExpectNoDisruption(f, 0.08, end.Sub(start), m.Events(time.Time{}, time.Time{}), fmt.Sprintf("API %q was unreachable during disruption", t.name))
}

// Teardown cleans up any remaining resources.
func (t *availableTest) Teardown(f *framework.Framework) {
}
