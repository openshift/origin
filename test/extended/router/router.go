package router

import (
	"context"
	"net/http"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	admissionapi "k8s.io/pod-security-admission/api"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/url"
)

var _ = g.Describe("[sig-network][Feature:Router][apigroup:operator.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		host, ns string
		ipUrl    string
		oc       *exutil.CLI

		configPath = exutil.FixturePath("testdata", "router", "ingress.yaml")
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			currlabel := "router=router"
			for _, ns := range []string{"default", "openshift-ingress", "tectonic-ingress"} {
				//Search the router by label
				if ns == "openshift-ingress" {
					currlabel = "router=router-default"
				}
				exutil.DumpPodLogsStartingWithInNamespace("router", ns, oc.AsAdmin())
				selector, err := labels.Parse(currlabel)
				if err != nil {
					panic(err)
				}
				exutil.DumpPodsCommand(oc.AdminKubeClient(), ns, selector, "cat /var/lib/haproxy/conf/haproxy.config")
			}
		}
	})

	oc = exutil.NewCLIWithPodSecurityLevel("router-stress", admissionapi.LevelBaseline)

	g.BeforeEach(func() {
		var err error
		host, err = exutil.WaitForRouterServiceIP(oc)
		// bracket IPv6
		ipUrl = exutil.IPUrl(host)
		o.Expect(err).NotTo(o.HaveOccurred())

		ns = oc.KubeFramework().Namespace.Name
	})

	g.Describe("The HAProxy router", func() {
		g.It("should respond with 503 to unrecognized hosts", g.Label("Size:S"), func() {
			t := url.NewTester(oc.AdminKubeClient(), ns).WithErrorPassthrough(true)
			defer t.Close()
			t.Within(
				time.Minute,
				url.Expect("GET", "https://www.google.com").Through(ipUrl).SkipTLSVerification().HasStatusCode(503),
				url.Expect("GET", "http://www.google.com").Through(ipUrl).HasStatusCode(503),
			)
		})

		g.It("should serve routes that were created from an ingress [apigroup:route.openshift.io]", g.Label("Size:M"), func() {
			g.By("deploying an ingress rule")
			err := oc.Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the ingress rule to be converted to routes")
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig())
			err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
				routes, err := client.RouteV1().Routes(ns).List(context.Background(), metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				return len(routes.Items) == 4, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the router reports the correct behavior")
			t := url.NewTester(oc.AdminKubeClient(), ns).WithErrorPassthrough(true)
			defer t.Close()
			t.Within(
				3*time.Minute,
				url.Expect("GET", "http://1.ingress-test.com/test").Through(ipUrl).HasStatusCode(200),
				url.Expect("GET", "http://1.ingress-test.com/other/deep").Through(ipUrl).HasStatusCode(200),
				url.Expect("GET", "http://1.ingress-test.com/").Through(ipUrl).HasStatusCode(503),
				url.Expect("GET", "http://2.ingress-test.com/").Through(ipUrl).HasStatusCode(200),
				url.Expect("GET", "https://3.ingress-test.com/").Through(ipUrl).SkipTLSVerification().HasStatusCode(200),
				url.Expect("GET", "http://3.ingress-test.com/").Through(ipUrl).RedirectsTo("https://3.ingress-test.com/", http.StatusFound),
			)
		})
	})
})
