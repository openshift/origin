package images

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exutil.FixturePath("testdata", "reencrypt-serving-cert.yaml")
		oc         = exutil.NewCLI("router-reencrypt", exutil.KubeConfigPath())

		ip, ns string
	)

	g.BeforeEach(func() {
		svc, err := oc.AdminKubeClient().Core().Services("default").Get("router", metav1.GetOptions{})
		if kapierrs.IsNotFound(err) {
			g.Skip("no router installed on the cluster")
			return
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		ip = svc.Spec.ClusterIP
		ns = oc.KubeFramework().Namespace.Name
	})

	g.Describe("The HAProxy router", func() {
		g.It("should support reencrypt to services backed by a serving certificate automatically", func() {
			routerURL := fmt.Sprintf("https://%s", ip)

			execPodName := exutil.CreateExecPodOrFail(oc.AdminKubeClient().Core(), ns, "execpod")
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()
			g.By(fmt.Sprintf("deploying a service using a reencrypt route without a destinationCACertificate"))
			err := oc.Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			var hostname string
			err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				route, err := oc.RouteClient().Route().Routes(ns).Get("serving-cert", metav1.GetOptions{})
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

			// don't assume the router is available via external DNS, because of complexity
			err = waitForRouterOKResponseExec(ns, execPodName, routerURL, hostname, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})
