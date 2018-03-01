package images

import (
	"fmt"
	"net/http"
	"os"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exutil.FixturePath("testdata", "scoped-router.yaml")
		oc         = exutil.NewCLI("unprivileged-router", exutil.KubeConfigPath())
	)

	g.BeforeEach(func() {
		imagePrefix := os.Getenv("OS_IMAGE_PREFIX")
		if len(imagePrefix) == 0 {
			imagePrefix = "openshift/origin"
		}
		err := oc.AsAdmin().Run("new-app").Args("-f", configPath,
			`-p=IMAGE=`+imagePrefix+`-haproxy-router`,
			`-p=SCOPE=["--name=test-unprivileged", "--namespace=$(POD_NAMESPACE)", "--loglevel=4", "--labels=select=first", "--update-status=false"]`,
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router", func() {
		g.It("should run even if it has no access to update status", func() {
			defer func() {
				if g.CurrentGinkgoTestDescription().Failed {
					dumpScopedRouterLogs(oc, g.CurrentGinkgoTestDescription().FullTestText)
				}
			}()
			g.Skip("test temporarily disabled")
			oc.SetOutputDir(exutil.TestContext.OutputDir)
			ns := oc.KubeFramework().Namespace.Name
			execPodName := exutil.CreateExecPodOrFail(oc.AdminKubeClient().CoreV1(), ns, "execpod")
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By(fmt.Sprintf("creating a scoped router from a config file %q", configPath))

			var routerIP string
			err := wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.KubeFramework().Namespace.Name).Get("scoped-router", metav1.GetOptions{})
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
			routerURL := fmt.Sprintf("http://%s", routerIP)

			g.By("waiting for the healthz endpoint to respond")
			healthzURI := fmt.Sprintf("http://%s:1936/healthz", routerIP)
			err = waitForRouterOKResponseExec(ns, execPodName, healthzURI, routerIP, changeTimeoutSeconds)
			if err != nil {
				dumpScopedRouterLogs(oc, fmt.Sprintf("%s - %s", g.CurrentGinkgoTestDescription().TestText, "waiting for the healthz endpoint to respond"))
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the valid route to respond")
			err = waitForRouterOKResponseExec(ns, execPodName, routerURL+"/Letter", "FIRST.example.com", changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, host := range []string{"second.example.com", "third.example.com"} {
				g.By(fmt.Sprintf("checking that %s does not match a route", host))
				err = expectRouteStatusCodeExec(ns, execPodName, routerURL+"/Letter", host, http.StatusServiceUnavailable)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("checking that the route doesn't have an ingress status")
			r, err := oc.RouteClient().Route().Routes(ns).Get("route-1", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			ingress := ingressForName(r, "test-unprivileged")
			o.Expect(ingress).To(o.BeNil())
		})
	})
})
