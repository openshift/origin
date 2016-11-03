package images

import (
	"fmt"
	"net/http"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/kubernetes/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[networking][router] openshift routers", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exutil.FixturePath("testdata", "scoped-router.yaml")
		oc         = exutil.NewCLI("scoped-router", exutil.KubeConfigPath())
	)

	g.BeforeEach(func() {
		// defer oc.Run("delete").Args("-f", configPath).Execute()
		err := oc.AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "system:router", oc.Username()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("create").Args("-f", configPath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router", func() {
		g.It("should serve the correct routes when scoped to a single namespace and label set", func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("creating a scoped router from a config file %q", configPath))

			var routerIP string
			err := wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
				pod, err := oc.KubeFramework().Client.Pods(oc.KubeFramework().Namespace.Name).Get("scoped-router")
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
			err = waitForRouterOKResponse(healthzURI, routerIP, 2*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the valid route to respond")
			err = waitForRouterOKResponse(routerURL, "first.example.com", 2*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, host := range []string{"second.example.com", "third.example.com"} {
				g.By(fmt.Sprintf("checking that %s does not match a route", host))
				req, err := requestViaReverseProxy("GET", routerURL, host)
				o.Expect(err).NotTo(o.HaveOccurred())
				resp, err := http.DefaultClient.Do(req)
				o.Expect(err).NotTo(o.HaveOccurred())
				resp.Body.Close()
				if resp.StatusCode != http.StatusServiceUnavailable {
					e2e.Failf("should have had a 503 status code for %s", host)
				}
			}
		})
		g.It("should override the route host with a custom value", func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)
			ns := oc.KubeFramework().Namespace.Name

			g.By(fmt.Sprintf("creating a scoped router from a config file %q", configPath))

			var routerIP string
			err := wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
				pod, err := oc.KubeFramework().Client.Pods(ns).Get("router-override")
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
			pattern := "%s-%s.myapps.mycompany.com"

			g.By("waiting for the healthz endpoint to respond")
			healthzURI := fmt.Sprintf("http://%s:1936/healthz", routerIP)
			err = waitForRouterOKResponse(healthzURI, routerIP, 2*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the valid route to respond")
			err = waitForRouterOKResponse(routerURL, fmt.Sprintf(pattern, "route-1", ns), 2*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking that the stored domain name does not match a route")
			host := "first.example.com"
			req, err := requestViaReverseProxy("GET", routerURL, host)
			o.Expect(err).NotTo(o.HaveOccurred())
			resp, err := http.DefaultClient.Do(req)
			o.Expect(err).NotTo(o.HaveOccurred())
			resp.Body.Close()
			if resp.StatusCode != http.StatusServiceUnavailable {
				e2e.Failf("should have had a 503 status code for %s", host)
			}

			for _, host := range []string{"route-1", "route-2"} {
				host = fmt.Sprintf(pattern, host, ns)
				g.By(fmt.Sprintf("checking that %s does not match a route", host))
				req, err := requestViaReverseProxy("GET", routerURL, host)
				o.Expect(err).NotTo(o.HaveOccurred())
				resp, err := http.DefaultClient.Do(req)
				o.Expect(err).NotTo(o.HaveOccurred())
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					e2e.Failf("should have had a 200 status code for %s", host)
				}
			}
		})
	})
})

func waitForRouterOKResponse(url, host string, timeout time.Duration) error {
	return wait.Poll(time.Second, timeout, func() (bool, error) {
		req, err := requestViaReverseProxy("GET", url, host)
		if err != nil {
			return false, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false, nil
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusServiceUnavailable {
			// not ready yet
			return false, nil
		}
		if resp.StatusCode != http.StatusOK {
			e2e.Logf("unexpected response: %#v", resp.StatusCode)
			return false, nil
		}
		return true, nil
	})
}

func requestViaReverseProxy(method, url, host string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Host = host
	return req, nil
}
