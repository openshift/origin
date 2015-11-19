package images

import (
	"fmt"
	"net/http"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("Router", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exutil.FixturePath("fixtures", "scoped-router.yaml")
		oc         = exutil.NewCLI("scoped-router", exutil.KubeConfigPath())
	)
	g.Describe("The HAProxy router", func() {
		g.It("should serve the correct routes when scoped to a single namespace and label set", func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("creating a scoped router from a config file %q", configPath))
			err := oc.Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			var routerIP string
			err = wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
				pod, err := oc.KubeFramework().Client.Pods(oc.KubeFramework().Namespace.Name).Get("router")
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
			return false, err
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
