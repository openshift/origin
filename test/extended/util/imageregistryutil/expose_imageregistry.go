package imageregistryutil

import (
	"context"
	"fmt"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
)

func ExposeImageRegistry(ctx context.Context, routeClient routeclient.Interface, routeName string) (*routev1.Route, error) {
	return exposeImageRegistryGenerateName(ctx, routeClient, routeName, false)
}

func ExposeImageRegistryGenerateName(ctx context.Context, routeClient routeclient.Interface, routePrefix string) (*routev1.Route, error) {
	return exposeImageRegistryGenerateName(ctx, routeClient, routePrefix, true)
}

func exposeImageRegistryGenerateName(ctx context.Context, routeClient routeclient.Interface, routeNameOrPrefix string, generateName bool) (*routev1.Route, error) {
	createdRoute := &routev1.Route{
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
	}
	if generateName {
		createdRoute.GenerateName = routeNameOrPrefix
	} else {
		createdRoute.Name = routeNameOrPrefix
	}
	route, err := routeClient.RouteV1().Routes("openshift-image-registry").Create(ctx, createdRoute, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		route, err = routeClient.RouteV1().Routes("openshift-image-registry").Get(ctx, route.Name, metav1.GetOptions{})
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
