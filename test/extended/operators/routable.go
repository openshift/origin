package operators

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
	exurl "github.com/openshift/origin/test/extended/util/url"
)

var _ = g.Describe("[Feature:Platform] Managed cluster should", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("operators-routable", exutil.KubeConfigPath())

		// routeHostWait is how long to wait for routes to be assigned a host
		routeHostWait = 30 * time.Second

		// endpointWait is how long to wait for endpoints to be reachable
		endpointWait = 3 * time.Minute
	)

	g.BeforeEach(func() {
		_, ns, err := exutil.GetRouterPodTemplate(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "couldn't find default router")

		svc, err := oc.AdminKubeClient().CoreV1().Services(ns).Get("router-default", metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				g.Skip("default router is not exposed by a load balancer service")
			}
			o.Expect(err).NotTo(o.HaveOccurred(), "error getting default router service: %v", err)
		}

		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			g.Skip("default router is not exposed by a load balancer service")
		}
	})

	g.It("should expose cluster services outside the cluster", func() {
		ns := oc.KubeFramework().Namespace.Name

		tester := exurl.NewTester(oc.AdminKubeClient(), ns).WithErrorPassthrough(true)

		tests := []*exurl.Test{}

		routes := []struct {
			ns     string
			name   string
			scheme string
			expect int
		}{
			{ns: "openshift-console", name: "console", scheme: "https", expect: 200},
			{ns: "openshift-monitoring", name: "prometheus-k8s", scheme: "https", expect: 403},
		}
		for _, r := range routes {
			g.By(fmt.Sprintf("verifying the %s/%s route has an ingress host", r.ns, r.name))
			var hostname string
			err := wait.Poll(time.Second, routeHostWait, func() (bool, error) {
				route, err := oc.AdminRouteClient().RouteV1().Routes(r.ns).Get(r.name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if len(route.Status.Ingress) == 0 || len(route.Status.Ingress[0].Host) == 0 {
					return false, nil
				}
				hostname = route.Status.Ingress[0].Host
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			url := fmt.Sprintf("%s://%s", r.scheme, hostname)
			tests = append(tests, exurl.Expect("GET", url).SkipTLSVerification().HasStatusCode(r.expect))
			g.By(fmt.Sprintf("verifying the %s/%s route serves %d from %s", r.ns, r.name, r.expect, url))
		}

		tester.Within(endpointWait, tests...)
	})
})
