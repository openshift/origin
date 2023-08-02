package imageregistryutil

import (
	"context"
	"fmt"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/test/extended/util/disruption"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
)

func ExposeImageRegistry(ctx context.Context, routeClient routeclient.Interface, routeName string) (*routev1.Route, error) {
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
		return nil, err
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
		return nil, fmt.Errorf("failed to get route host: %w", err)
	}

	// in CI we observe a gap between the route having status and the route actually being exposed consistently.
	// this results in a 503 for 4 seconds observed so far.  I'm choosing 30 seconds more or less at random.
	time.Sleep(30 * time.Second)

	return route, nil
}

// Setup creates a route that exposes the registry to tests.
func SetupImageRegistryFor(routeName string) disruption.SetupFunc {
	return func(f *framework.Framework, _ disruption.BackendSampler) error {
		ctx := context.Background()

		routeClient, err := routeclient.NewForConfig(f.ClientConfig())
		if err != nil {
			return err
		}

		if _, err := ExposeImageRegistry(ctx, routeClient, routeName); err != nil {
			return err
		}

		return nil
	}
}

func TeardownImageRegistryFor(routeName string) disruption.TearDownFunc {
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
