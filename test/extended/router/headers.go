package router

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exutil.FixturePath("testdata", "router", "router-http-echo-server.yaml")
		oc         = exutil.NewCLI("router-headers", exutil.KubeConfigPath())

		routerIP  string
		metricsIP string
	)

	g.BeforeEach(func() {
		var err error
		routerIP, err = exutil.WaitForDefaultIngressControllerRoutableEndpoint(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		metricsIP, err = exutil.WaitForRouterInternalIP(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		if routerIP != metricsIP {
			// On 4.x clusters, there's couple of bugs to fix before
			// we can enable this test:
			//  1. Selector on router-internal-default service
			//  2. There is a weird 10.131.0.1 IP in the mix which
			//     shows up as the client ip and not the node IP.
			// Dec 17 17:03:32.836: Unexpected header: '10.131.0.1' (expected 10.0.139.23); All headers: http.Header{"User-Agent":[]string{"curl/7.61.0"}, "Accept":[]string{"*/*"}, "X-Forwarded-Host":[]string{"router-headers.example.com"}, "X-Forwarded-Port":[]string{"80"}, "X-Forwarded-Proto":[]string{"http"}, "Forwarded":[]string{"for=10.131.0.1;host=router-headers.example.com;proto=http;proto-version="}, "X-Forwarded-For":[]string{"10.131.0.1"}}
			g.Skip("skipped on 4.0 clusters")
			return
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should set Forwarded headers appropriately", func() {
			defer func() {
				// This should be done if the test fails but
				// for now always dump the logs.
				// if g.CurrentGinkgoTestDescription().Failed
				dumpRouterHeadersLogs(oc, g.CurrentGinkgoTestDescription().FullTestText)
			}()

			ns := oc.KubeFramework().Namespace.Name
			execPodName := exutil.CreateExecPodOrFail(oc.AdminKubeClient().CoreV1(), ns, "execpod")
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By(fmt.Sprintf("creating an http echo server from a config file %q", configPath))

			err := oc.Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			var clientIP string
			err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(ns).Get("execpod", metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if len(pod.Status.PodIP) == 0 {
					return false, nil
				}

				clientIP = pod.Status.PodIP
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// router expected to listen on port 80
			routerURL := fmt.Sprintf("http://%s", routerIP)

			g.By("waiting for the healthz endpoint to respond")
			healthzURI := fmt.Sprintf("http://%s:1936/healthz", metricsIP)
			err = waitForRouterOKResponseExec(ns, execPodName, healthzURI, metricsIP, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			host := "router-headers.example.com"
			g.By(fmt.Sprintf("waiting for the route to become active"))
			err = waitForRouterOKResponseExec(ns, execPodName, routerURL, host, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("making a request and reading back the echoed headers"))
			var payload string
			payload, err = getRoutePayloadExec(ns, execPodName, routerURL, host)
			o.Expect(err).NotTo(o.HaveOccurred())

			// The trailing \n is being stripped, so add it back
			payload = payload + "\n"

			// parse the echoed request
			reader := bufio.NewReader(strings.NewReader(payload))
			req, err := http.ReadRequest(reader)
			o.Expect(err).NotTo(o.HaveOccurred())

			// check that the header is what we expect
			g.By(fmt.Sprintf("inspecting the echoed headers"))
			ffHeader := req.Header.Get("X-Forwarded-For")
			if ffHeader != clientIP {
				e2e.Failf("Unexpected header: '%s' (expected %s); All headers: %#v", ffHeader, clientIP, req.Header)
			}
		})
	})
})

func dumpRouterHeadersLogs(oc *exutil.CLI, name string) {
	log, _ := e2e.GetPodLogs(oc.AdminKubeClient(), oc.KubeFramework().Namespace.Name, "router-headers", "router")
	e2e.Logf("Weighted Router test %s logs:\n %s", name, log)
}

func getRoutePayloadExec(ns, execPodName, url, host string) (string, error) {
	cmd := fmt.Sprintf(`
		set -e
		payload=$( curl -s --header 'Host: %s' %q ) || rc=$?
		if [[ "${rc:-0}" -eq 0 ]]; then
			printf "${payload}"
			exit 0
		else
			echo "error ${rc}" 1>&2
			exit 1
		fi
		`, host, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}
