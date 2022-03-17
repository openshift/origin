package imageregistry

import (
	"github.com/openshift/origin/test/extended/util/imageregistryutil"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/test/extended/util/disruption"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

func NewImageRegistryAvailableWithNewConnectionsTest() upgrades.Test {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	return disruption.NewBackendDisruptionTest(
		"[sig-imageregistry] Image registry remains available using new connections",
		backenddisruption.NewRouteBackend(
			restConfig,
			"openshift-image-registry",
			"test-disruption-new",
			"image-registry",
			"/healthz",
			backenddisruption.NewConnectionType),
	).
		WithPreSetup(imageregistryutil.SetupImageRegistryFor("test-disruption-new")).
		WithPostTeardown(imageregistryutil.TeardownImageRegistryFor("test-disruption-new"))

}

func NewImageRegistryAvailableWithReusedConnectionsTest() upgrades.Test {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	return disruption.NewBackendDisruptionTest(
		"[sig-imageregistry] Image registry remains available using reused connections",
		backenddisruption.NewRouteBackend(
			restConfig,
			"openshift-image-registry",
			"test-disruption-reused",
			"image-registry",
			"/healthz",
			backenddisruption.ReusedConnectionType),
	).
		WithPreSetup(imageregistryutil.SetupImageRegistryFor("test-disruption-reused")).
		WithPostTeardown(imageregistryutil.TeardownImageRegistryFor("test-disruption-reused"))

}
