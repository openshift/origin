package imageregistryutil

import (
	"context"
	"fmt"
	"math/rand"
	"os/exec"
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
func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

func exposeRouteFromSVC(oc *exutil.CLI, rType, ns, route, service string) string {
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("route", rType, route, "--service="+service, "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	regRoute, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("route", route, "-n", ns, "-o=jsonpath={.spec.host}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return regRoute
}

func waitRouteReady(route string) {
	curlCmd := "curl -k https://" + route
	var output []byte
	var curlErr error
	pollErr := wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
		output, curlErr = exec.Command("bash", "-c", curlCmd).CombinedOutput()
		if curlErr != nil {
			e2e.Logf("the route is not ready, go to next round")
			return false, nil
		}
		return true, nil
	})
	if pollErr != nil {
		e2e.Logf("output is: %v with error %v", string(output), curlErr.Error())
	}
	exutil.AssertWaitPollNoErr(pollErr, "The route can't be used")
}
