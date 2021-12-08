package frontends

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"

	"github.com/blang/semver"
	apiconfigv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
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
		"[sig-network-edge] OAuth remains available via cluster backendSampler ingress using new connections",
		backenddisruption.NewRouteBackend(
			"openshift-authentication",
			"oauth-openshift",
			"ingress-to-oauth-server",
			"/healthz",
			backenddisruption.NewConnectionType).
			WithExpectedBody("ok"),
	).WithAllowedDisruption(allowedIngressDisruption)
}

// NewOAuthRouteAvailableWithConnectionReuseTest tests that the oauth route
// remains available during and after a cluster upgrade, reusing a connection
// for requests.
func NewOAuthRouteAvailableWithConnectionReuseTest() upgrades.Test {
	return disruption.NewBackendDisruptionTest(
		"[sig-network-edge] OAuth remains available via cluster ingress using reused connections",
		backenddisruption.NewRouteBackend(
			"openshift-authentication",
			"oauth-openshift",
			"ingress-to-oauth-server",
			"/healthz",
			backenddisruption.ReusedConnectionType).
			WithExpectedBody("ok"),
	).WithAllowedDisruption(allowedIngressDisruption)
}

// NewConsoleRouteAvailableWithNewConnectionsTest tests that the console route
// remains available during and after a cluster upgrade, using a new connection
// for each request.
func NewConsoleRouteAvailableWithNewConnectionsTest() upgrades.Test {
	return disruption.NewBackendDisruptionTest(
		"[sig-network-edge] Console remains available via cluster ingress using new connections",
		backenddisruption.NewRouteBackend(
			"openshift-console",
			"console",
			"ingress-to-console",
			"/healthz",
			backenddisruption.NewConnectionType).
			WithExpectedBodyRegex(`(Red Hat OpenShift Container Platform|OKD)`),
	).WithAllowedDisruption(allowedIngressDisruption)
}

// NewConsoleRouteAvailableWithConnectionReuseTest tests that the console route
// remains available during and after a cluster upgrade, reusing a connection
// for requests.
func NewConsoleRouteAvailableWithConnectionReuseTest() upgrades.Test {
	return disruption.NewBackendDisruptionTest(
		"[sig-network-edge] Console remains available via cluster ingress using reused connections",
		backenddisruption.NewRouteBackend(
			"openshift-console",
			"console",
			"ingress-to-console",
			"/healthz",
			backenddisruption.ReusedConnectionType).
			WithExpectedBodyRegex(`(Red Hat OpenShift Container Platform|OKD)`),
	).WithAllowedDisruption(allowedIngressDisruption)
}

func getTopologies(f *framework.Framework) (controlPlaneTopology, infraTopology apiconfigv1.TopologyMode) {
	oc := util.NewCLIWithFramework(f)
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(),
		"cluster", metav1.GetOptions{})

	framework.ExpectNoError(err, "unable to determine cluster topology")

	return infra.Status.ControlPlaneTopology, infra.Status.InfrastructureTopology
}

func allowedIngressDisruption(f *framework.Framework, totalDuration time.Duration) (*time.Duration, error) {
	// Fetch network type for considering whether we allow disruption. For OVN, we currently have to allow disruption
	// as those tests are failing: BZ: https://bugzilla.redhat.com/show_bug.cgi?id=1983829
	c, err := configv1client.NewForConfig(f.ClientConfig())
	if err != nil {
		return nil, err
	}
	network, err := c.ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// starting from 4.8, enforce the requirement that frontends remains available
	hasAllFixes, err := util.AllClusterVersionsAreGTE(semver.Version{Major: 4, Minor: 8}, f.ClientConfig())
	if err != nil {
		framework.Logf("Cannot require full cluster ingress backendSampler availability; some versions could not be checked: %v", err)
	}

	toleratedDisruption := 0.20
	switch controlPlaneTopology, _ := getTopologies(f); {
	case controlPlaneTopology == apiconfigv1.SingleReplicaTopologyMode:
		// we cannot avoid API downtime during upgrades on single-node control plane topologies (we observe around ~10% disruption)
		framework.Logf("Control-plane topology is single-replica - allowing disruption")
	case network.Status.NetworkType == "OVNKubernetes":
		framework.Logf("Network type is OVNKubernetes, temporarily allowing disruption due to BZ https://bugzilla.redhat.com/show_bug.cgi?id=1983829")
	// framework.ProviderIs("gce") removed here in 4.9 due to regression. BZ: https://bugzilla.redhat.com/show_bug.cgi?id=1983758
	case framework.ProviderIs("azure"), framework.ProviderIs("aws"):
		if hasAllFixes {
			framework.Logf("Cluster contains no versions older than 4.8, tolerating no disruption")
			toleratedDisruption = 0
		}
	}

	allowedDisruptionNanoseconds := int64(float64(totalDuration.Nanoseconds()) * toleratedDisruption)
	allowedDisruption := time.Duration(allowedDisruptionNanoseconds)

	return &allowedDisruption, nil
}
