package controlplane

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/kubernetes/test/e2e/upgrades"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption"
)

// NewKubeAvailableWithNewConnectionsTest tests that the Kubernetes control plane remains available during and after a cluster upgrade.
func NewKubeAvailableWithNewConnectionsTest() upgrades.Test {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	backendSampler, err := createKubeAPIMonitoringWithNewConnections(restConfig)
	utilruntime.Must(err)
	return disruption.NewBackendDisruptionTest(
		"[sig-api-machinery] Kubernetes APIs remain available for new connections",
		backendSampler,
	)
}

// NewOpenShiftAvailableNewConnectionsTest tests that the OpenShift APIs remains available during and after a cluster upgrade.
func NewOpenShiftAvailableNewConnectionsTest() upgrades.Test {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	backendSampler, err := createOpenShiftAPIMonitoringWithNewConnections(restConfig)
	utilruntime.Must(err)
	return disruption.NewBackendDisruptionTest(
		"[sig-api-machinery] OpenShift APIs remain available for new connections",
		backendSampler,
	)
}

// NewOAuthAvailableNewConnectionsTest tests that the OAuth APIs remains available during and after a cluster upgrade.
func NewOAuthAvailableNewConnectionsTest() upgrades.Test {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	backendSampler, err := createOAuthAPIMonitoringWithNewConnections(restConfig)
	utilruntime.Must(err)
	return disruption.NewBackendDisruptionTest(
		"[sig-api-machinery] OAuth APIs remain available for new connections",
		backendSampler,
	)
}

// NewKubeAvailableWithConnectionReuseTest tests that the Kubernetes control plane remains available during and after a cluster upgrade.
func NewKubeAvailableWithConnectionReuseTest() upgrades.Test {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	backendSampler, err := createKubeAPIMonitoringWithConnectionReuse(restConfig)
	utilruntime.Must(err)
	return disruption.NewBackendDisruptionTest(
		"[sig-api-machinery] Kubernetes APIs remain available with reused connections",
		backendSampler,
	)
}

// NewOpenShiftAvailableWithConnectionReuseTest tests that the OpenShift APIs remains available during and after a cluster upgrade.
func NewOpenShiftAvailableWithConnectionReuseTest() upgrades.Test {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	backendSampler, err := createOpenShiftAPIMonitoringWithConnectionReuse(restConfig)
	utilruntime.Must(err)
	return disruption.NewBackendDisruptionTest(
		"[sig-api-machinery] OpenShift APIs remain available with reused connections",
		backendSampler,
	)
}

// NewOAuthAvailableWithConnectionReuseTest tests that the OAuth APIs remains available during and after a cluster upgrade.
func NewOAuthAvailableWithConnectionReuseTest() upgrades.Test {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	backendSampler, err := createOAuthAPIMonitoringWithConnectionReuse(restConfig)
	utilruntime.Must(err)
	return disruption.NewBackendDisruptionTest(
		"[sig-api-machinery] OAuth APIs remain available with reused connections",
		backendSampler,
	)
}
