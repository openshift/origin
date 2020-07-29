package router

import (
	"context"
	"net"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	v1 "github.com/openshift/api/config/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	discoveryclientset "k8s.io/client-go/kubernetes/typed/discovery/v1beta1"

	clusterconfigset "github.com/openshift/client-go/config/clientset/versioned"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/url"
)

var _ = g.Describe("[sig-network][Feature:IPv6DualStack][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		host, ns string
		oc       *exutil.CLI
	)
	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(ns)
			if routes, _ := client.List(context.Background(), metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWith("router-", oc)
		}
	})

	oc = exutil.NewCLI("router-service-ipfamily")

	g.BeforeEach(func() {
		var err error
		host, err = exutil.WaitForRouterServiceIP(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		ns = oc.KubeFramework().Namespace.Name

	})

	g.Describe("The HAProxy router", func() {
		g.It("should serve a basic route using a service with ipFamily set to IPV4 on a single stack ipv4 cluster", func() {
			configPath := exutil.FixturePath("testdata", "router", "router-ipfamily-v4.yaml")
			err := oc.AsAdmin().Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for route resource to become available")
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig())
			err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
				routes, err := client.RouteV1().Routes(ns).List(context.Background(), metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				return len(routes.Items) == 1, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verify service has ipFamily field set properly")
			svc, err := oc.AdminKubeClient().CoreV1().Services(ns).Get(context.Background(), "endpoints", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(svc).NotTo(o.BeNil())
			// svc.Spec.IPFamily is not yet available on single stack clusters
			//o.Expect(svc.Spec.IPFamily).To(o.Equal("IPv4"))

			g.By("verify pod uses proper ip type")
			var podIP string
			err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.KubeFramework().Namespace.Name).Get(context.Background(), "endpoint", metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if len(pod.Status.PodIP) == 0 {
					return false, nil
				}
				podIP = pod.Status.PodIP
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			// Probably a better way to verify ipv4
			o.Expect(net.ParseIP(podIP).To4()).NotTo(o.BeNil())

			g.By("verify endpointslice has proper IP family set")
			// Get endpointslice here and check addressType field
			discoveryClient := discoveryclientset.NewForConfigOrDie(oc.AdminConfig())
			endpointSlices, err := discoveryClient.EndpointSlices(ns).List(context.Background(), metav1.ListOptions{})
			o.Expect(endpointSlices.Items).ToNot(o.HaveLen(0))
			for _, endpointSlice := range endpointSlices.Items {
				o.Expect(endpointSlice.AddressType).To(o.Equal(discoveryv1beta1.AddressTypeIPv4))
			}

			g.By("verifying the route works as intended")
			t := url.NewTester(oc.AdminKubeClient(), ns).WithErrorPassthrough(true)
			defer t.Close()
			t.Within(
				3*time.Minute,
				url.Expect("GET", "http://test.example.com/test").Through(host).HasStatusCode(200),
				url.Expect("GET", "http://test.example.com/foo").Through(host).HasStatusCode(503),
			)
		})

		g.It("should serve a basic route using a service with ipFamily set to IPV6 on a single stack ipv6 cluster", func() {
			g.Skip("Single stack ipv6 not currently available in CI")
			configPath := exutil.FixturePath("testdata", "router", "router-ipfamily-v6.yaml")
			err := oc.Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for route resource to become available")
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig())
			err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
				routes, err := client.RouteV1().Routes(ns).List(context.Background(), metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				return len(routes.Items) == 1, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verify service has ipFamily field set properly")
			svc, err := oc.AdminKubeClient().CoreV1().Services(ns).Get(context.Background(), "endpoints", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(svc).ToNot(o.BeNil())
			// svc.Spec.IPFamily is not yet available on single stack clusters
			//o.Expect(svc.Spec.IPFamily).To(o.Equal("IPv6"))

			g.By("verify pod uses proper ip type")
			var podIP string
			err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.KubeFramework().Namespace.Name).Get(context.Background(), "endpoint", metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if len(pod.Status.PodIP) == 0 {
					return false, nil
				}
				podIP = pod.Status.PodIP
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			// Definitely a better way to verify ipv6
			o.Expect(net.ParseIP(podIP).To4()).To(o.BeNil())
			o.Expect(strings.Contains(podIP, ":")).To(o.BeTrue())

			g.By("verify endpointslice has proper IP family set")
			// Get endpointslice here and check addressType field
			discoveryClient := discoveryclientset.NewForConfigOrDie(oc.AdminConfig())
			endpointSlices, err := discoveryClient.EndpointSlices(ns).List(context.Background(), metav1.ListOptions{})
			o.Expect(endpointSlices.Items).ToNot(o.HaveLen(0))
			for _, endpointSlice := range endpointSlices.Items {
				o.Expect(endpointSlice.AddressType).To(o.Equal(discoveryv1beta1.AddressTypeIPv6))
			}

			// IPv6 formatting, use escaped brackets for curl
			host = "\\[" + host + "\\]"

			g.By("verifying the route works as intended")
			t := url.NewTester(oc.AdminKubeClient(), ns).WithErrorPassthrough(true)
			defer t.Close()
			t.Within(
				3*time.Minute,
				url.Expect("GET", "http://test.example.com/test").Through(host).HasStatusCode(200),
				url.Expect("GET", "http://test.example.com/foo").Through(host).HasStatusCode(503),
			)
		})
	})
})

func checkDualStackEnabled(oc *exutil.CLI) bool {
	// Check to see if the dual stack feature gate is enabled in the openshift config
	configClient := clusterconfigset.NewForConfigOrDie(oc.AdminConfig())
	gate, err := configClient.ConfigV1().FeatureGates().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(string(gate.Spec.FeatureGateSelection.FeatureSet), string(v1.IPv6DualStackNoUpgrade)) {
		return true
	}
	return false
}
