package images

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/kubernetes/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[networking][router] weighted openshift router", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exutil.FixturePath("testdata", "weighted-router.yaml")
		oc         = exutil.NewCLI("weighted-router", exutil.KubeConfigPath())
	)

	g.BeforeEach(func() {
		// defer oc.Run("delete").Args("-f", configPath).Execute()
		err := oc.AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "system:router", oc.Username()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("create").Args("-f", configPath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router", func() {
		g.It("should appropriately serve a route that points to two services", func() {

			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("creating a weighted router from a config file %q", configPath))

			var routerIP string
			err := wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
				pod, err := oc.KubeFramework().Client.Pods(oc.KubeFramework().Namespace.Name).Get("weighted-router")
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

			host := "weighted.example.com"
			g.By(fmt.Sprintf("checking that 10 requests go through successfully"))
			for i := 1; i <= 100; i++ {
				err = waitForRouterOKResponse(routerURL, "weighted.example.com", 1*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			g.By(fmt.Sprintf("checking that stats can be obtained successfully"))
			statsURL := fmt.Sprintf("http://%s:1936/;csv", routerIP)
			req, err := requestViaReverseProxy("GET", statsURL, host)
			req.SetBasicAuth("admin", "password")
			resp, err := http.DefaultClient.Do(req)
			o.Expect(err).NotTo(o.HaveOccurred())
			if resp.StatusCode != http.StatusOK {
				e2e.Failf("unexpected response: %#v", resp.StatusCode)
			}

			g.By(fmt.Sprintf("checking that weights are respected by the router"))
			stats, err := ioutil.ReadAll(resp.Body)
			defer resp.Body.Close()
			trafficValues, err := parseStats(string(stats), "weightedroute", 7)
			o.Expect(err).NotTo(o.HaveOccurred())

			if len(trafficValues) != 2 {
				e2e.Failf("Expected 2 weighted backends for incoming traffic, found %d", len(trafficValues))
			}

			trafficEP1, err := strconv.Atoi(trafficValues[0])
			o.Expect(err).NotTo(o.HaveOccurred())
			trafficEP2, err := strconv.Atoi(trafficValues[1])
			o.Expect(err).NotTo(o.HaveOccurred())

			weightedRatio := float32(trafficEP1) / float32(trafficEP2)
			if weightedRatio < 5 && weightedRatio > 0.2 {
				e2e.Failf("Unexpected weighted ratio for incoming traffic: %v (%d/%d)", weightedRatio, trafficEP1, trafficEP2)
			}

			g.By(fmt.Sprintf("checking that zero weights are also respected by the router"))
			host = "zeroweight.example.com"
			req, _ = requestViaReverseProxy("GET", routerURL, host)
			resp, err = http.DefaultClient.Do(req)
			o.Expect(err).NotTo(o.HaveOccurred())
			if resp.StatusCode != http.StatusServiceUnavailable {
				e2e.Failf("Expected zero weighted route to return a 503, but got %v", resp.StatusCode)
			}
		})
	})
})

func parseStats(stats string, backendSubstr string, statsField int) ([]string, error) {
	r := csv.NewReader(strings.NewReader(stats))
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	fieldValues := make([]string, 0)
	for _, rec := range records {
		if strings.Contains(rec[0], backendSubstr) && !strings.Contains(rec[1], "BACKEND") {
			fieldValues = append(fieldValues, rec[statsField])
		}
	}
	return fieldValues, nil
}
