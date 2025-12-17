package operators

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
	exurl "github.com/openshift/origin/test/extended/util/url"
)

var _ = g.Describe("[sig-arch] Managed cluster", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLIWithPodSecurityLevel("operators-routable", admissionapi.LevelBaseline)

		// routeHostWait is how long to wait for routes to be assigned a host
		routeHostWait = 30 * time.Second

		// endpointWait is how long to wait for endpoints to be reachable
		endpointWait = 3 * time.Minute
	)

	g.BeforeEach(func() {
		_, ns, err := exutil.GetRouterPodTemplate(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "couldn't find default router")

		svc, err := oc.AdminKubeClient().CoreV1().Services(ns).Get(context.Background(), "router-default", metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				g.Skip("default router is not exposed by a load balancer service")
			}
			o.Expect(err).NotTo(o.HaveOccurred(), "error getting default router service: %v", err)
		}

		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			g.Skip("default router is not exposed by a load balancer service")
		}

		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		if err != nil {
			o.Expect(err).NotTo(o.HaveOccurred(), "error determining if running on MicroShift: %v", err)
		}
		if isMicroShift {
			g.Skip("MicroShift does not support ingress-canary or monitoring")
		}
	})

	g.It("should expose cluster services outside the cluster [apigroup:route.openshift.io]", g.Label("Size:M"), func() {
		ns := oc.KubeFramework().Namespace.Name

		tester := exurl.NewTester(oc.AdminKubeClient(), ns).WithErrorPassthrough(true)

		tests := []*exurl.Test{}

		routes := []struct {
			ns     string
			name   string
			scheme string
			path   string
			expect []int
		}{
			{ns: "openshift-ingress-canary", name: "canary", scheme: "https", path: "", expect: []int{200}},
			{ns: "openshift-monitoring", name: "prometheus-k8s", scheme: "https", path: "api/v1/targets", expect: []int{403, 401}},
		}
		for _, r := range routes {
			g.By(fmt.Sprintf("verifying the %s/%s route has an ingress host", r.ns, r.name))
			var hostname string
			err := wait.Poll(time.Second, routeHostWait, func() (bool, error) {
				route, err := oc.AdminRouteClient().RouteV1().Routes(r.ns).Get(context.Background(), r.name, metav1.GetOptions{})
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
			var url string
			if r.path == "" {
				url = fmt.Sprintf("%s://%s", r.scheme, hostname)
			} else {
				url = fmt.Sprintf("%s://%s/%s", r.scheme, hostname, r.path)
			}
			tests = append(tests, exurl.Expect("GET", url).SkipTLSVerification().HasStatusCode(r.expect...))
			g.By(fmt.Sprintf("verifying the %s/%s route serves %d from %s", r.ns, r.name, r.expect, url))
		}

		tester.Within(endpointWait, tests...)
	})
})
