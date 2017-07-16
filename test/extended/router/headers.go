package images

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][networking][router] router headers", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exutil.FixturePath("testdata", "router-http-echo-server.yaml")
		oc         = exutil.NewCLI("router-headers", exutil.KubeConfigPath())

		routerIP, routerNs string
	)

	g.BeforeEach(func() {
		svc, err := oc.AdminKubeClient().Core().Services("default").Get("router", metav1.GetOptions{})
		if kapierrs.IsNotFound(err) {
			g.Skip("no router installed on the cluster")
			return
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		routerIP = svc.Spec.ClusterIP
		routerNs = oc.KubeFramework().Namespace.Name
	})

	g.Describe("The HAProxy router", func() {
		g.It("should set Forwarded headers appropriately", func() {
			defer func() {
				// This should be done if the test fails but
				// for now always dump the logs.
				// if g.CurrentGinkgoTestDescription().Failed
				dumpRouterHeadersLogs(oc, g.CurrentGinkgoTestDescription().FullTestText)
			}()
			oc.SetOutputDir(exutil.TestContext.OutputDir)
			ns := oc.KubeFramework().Namespace.Name
			execPodName := exutil.CreateExecPodOrFail(oc.AdminKubeClient().Core(), ns, "execpod")
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By(fmt.Sprintf("creating an http echo server from a config file %q", configPath))

			err := oc.Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			var clientIP string
			err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.Core().Pods(ns).Get("execpod", metav1.GetOptions{})
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
			healthzURI := fmt.Sprintf("http://%s:1936/healthz", routerIP)
			err = waitForRouterOKResponseExec(ns, execPodName, healthzURI, routerIP, changeTimeoutSeconds)
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
