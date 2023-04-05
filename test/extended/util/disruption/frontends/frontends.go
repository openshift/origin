package frontends

import (
	"context"

	apiconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

// NewOAuthRouteAvailableWithNewConnectionsTest tests that the oauth route
// remains available during and after a cluster upgrade, using a new connection
// for each request.
func NewOAuthRouteAvailableWithNewConnectionsTest() upgrades.Test {
	return disruption.NewBackendDisruptionTest(
		"[sig-network-edge] OAuth remains available via cluster ingress using new connections",
		createOAuthRouteAvailableWithNewConnections(),
	)
}

// NewOAuthRouteAvailableWithConnectionReuseTest tests that the oauth route
// remains available during and after a cluster upgrade, reusing a connection
// for requests.
func NewOAuthRouteAvailableWithConnectionReuseTest() upgrades.Test {
	return disruption.NewBackendDisruptionTest(
		"[sig-network-edge] OAuth remains available via cluster ingress using reused connections",
		createOAuthRouteAvailableWithConnectionReuse(),
	)
}

// NewConsoleRouteAvailableWithNewConnectionsTest tests that the console route
// remains available during and after a cluster upgrade, using a new connection
// for each request.
func NewConsoleRouteAvailableWithNewConnectionsTest() upgrades.Test {
	return disruption.NewBackendDisruptionTest(
		"[sig-network-edge] Console remains available via cluster ingress using new connections",
		CreateConsoleRouteAvailableWithNewConnections(),
	)
}

// NewConsoleRouteAvailableWithConnectionReuseTest tests that the console route
// remains available during and after a cluster upgrade, reusing a connection
// for requests.
func NewConsoleRouteAvailableWithConnectionReuseTest() upgrades.Test {
	return disruption.NewBackendDisruptionTest(
		"[sig-network-edge] Console remains available via cluster ingress using reused connections",
		createConsoleRouteAvailableWithConnectionReuse(),
	)
}

func getTopologies(f *framework.Framework) (controlPlaneTopology, infraTopology apiconfigv1.TopologyMode) {
	oc := util.NewCLIWithFramework(f)
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(),
		"cluster", metav1.GetOptions{})

	framework.ExpectNoError(err, "unable to determine cluster topology")

	return infra.Status.ControlPlaneTopology, infra.Status.InfrastructureTopology
}
