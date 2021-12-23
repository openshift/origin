package imageregistry

import (
	"context"
	"fmt"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/test/extended/util/disruption"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
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
		WithPreSetup(setupImageRegistryFor("test-disruption-new")).
		WithPostTeardown(teardownImageRegistryFor("test-disruption-new"))

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
		WithPreSetup(setupImageRegistryFor("test-disruption-reused")).
		WithPostTeardown(teardownImageRegistryFor("test-disruption-reused"))

}

// Setup creates a route that exposes the registry to tests.
func setupImageRegistryFor(routeName string) disruption.SetupFunc {
	return func(f *framework.Framework, _ disruption.BackendSampler) error {
		ctx := context.Background()

		routeClient, err := routeclient.NewForConfig(f.ClientConfig())
		if err != nil {
			return err
		}

		route, err := routeClient.RouteV1().Routes("openshift-image-registry").Create(ctx, &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name: routeName,
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "image-registry",
				},
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromInt(5000),
				},
				TLS: &routev1.TLSConfig{
					Termination:                   routev1.TLSTerminationPassthrough,
					InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}

		err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
			route, err = routeClient.RouteV1().Routes("openshift-image-registry").Get(ctx, routeName, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			for _, ingress := range route.Status.Ingress {
				if len(ingress.Host) > 0 {
					return true, nil
				}
			}
			return false, nil
		})
		if err != nil {
			return fmt.Errorf("failed to get route host: %w", err)
		}

		// in CI we observe a gap between the route having status and the route actually being exposed consistently.
		// this results in a 503 for 4 seconds observed so far.  I'm choosing 30 seconds more or less at random.
		time.Sleep(30 * time.Second)

		return nil
	}
}

func teardownImageRegistryFor(routeName string) disruption.TearDownFunc {
	return func(f *framework.Framework) error {
		ctx := context.Background()

		routeClient, err := routeclient.NewForConfig(f.ClientConfig())
		if err != nil {
			return err
		}
		framework.ExpectNoError(err)

		err = routeClient.RouteV1().Routes("openshift-image-registry").Delete(ctx, routeName, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete route: %w", err)
		}
		return nil
	}
}
