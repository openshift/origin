package router

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	admissionapi "k8s.io/pod-security-admission/api"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network][Feature:Router][apigroup:route.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc          *exutil.CLI
		ns          string
		routerImage string
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(ns)
			if routes, _ := client.List(context.Background(), metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWith("router-", oc)
		}
	})

	oc = exutil.NewCLIWithPodSecurityLevel("unprivileged-router", admissionapi.LevelBaseline)

	g.BeforeEach(func() {
		ns = oc.Namespace()

		var err error
		routerImage, err = exutil.FindRouterImage(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		configPath := exutil.FixturePath("testdata", "router", "router-common.yaml")
		err = oc.AsAdmin().Run("apply").Args("-f", configPath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router", func() {
		g.It("should run even if it has no access to update status [apigroup:image.openshift.io]", g.Label("Size:M"), func() {

			routerPod := createScopedRouterPod(routerImage, "test-unprivileged", defaultPemData, "false")
			g.By("creating a router")
			ns := oc.KubeFramework().Namespace.Name
			_, err := oc.AdminKubeClient().CoreV1().Pods(ns).Create(context.Background(), routerPod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
			defer func() {
				oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
			}()

			var routerIP string
			err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.KubeFramework().Namespace.Name).Get(context.Background(), "router-scoped", metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if len(pod.Status.PodIP) == 0 {
					return false, nil
				}
				routerIP = pod.Status.PodIP
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// router expected to listen on port 80
			routerURL := fmt.Sprintf("http://%s", exutil.IPUrl(routerIP))

			g.By("waiting for the healthz endpoint to respond")
			healthzURI := fmt.Sprintf("http://%s/healthz", net.JoinHostPort(routerIP, "1936"))
			err = waitForRouterOKResponseExec(ns, execPod.Name, healthzURI, routerIP, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the valid route to respond")
			err = waitForRouterOKResponseExec(ns, execPod.Name, routerURL+"/Letter", "FIRST.example.com", changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, host := range []string{"second.example.com", "third.example.com"} {
				g.By(fmt.Sprintf("checking that %s does not match a route", host))
				err = expectRouteStatusCodeExec(ns, execPod.Name, routerURL+"/Letter", host, http.StatusServiceUnavailable)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("checking that the route doesn't have an ingress status")
			r, err := oc.RouteClient().RouteV1().Routes(ns).Get(context.Background(), "route-1", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			ingress := ingressForName(r, "test-unprivileged")
			o.Expect(ingress).To(o.BeNil())
		})
	})
})
