package router

import (
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		fixture = exutil.FixturePath("testdata", "reencrypt-serving-cert.yaml")
		oc      *exutil.CLI

		ns string
	)

	oc = exutil.NewCLI("router-reencrypt", exutil.KubeConfigPath())

	g.BeforeEach(func() {
		_, routerNs, err := exutil.GetRouterPodTemplate(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "couldn't find default router")

		routerSvc, err := oc.AdminKubeClient().CoreV1().Services(routerNs).Get("router-default", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "expected %s/%s to exist", routerNs, "router-default")

		if routerSvc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			g.Skip("default router is not exposed by a load balancer service; skipping wildcard DNS test")
		}

		ns = oc.KubeFramework().Namespace.Name
	})

	g.Describe("The default ClusterIngress", func() {
		g.It("should support default wildcard reencrypt routes through external DNS", func() {
			execPodName := exutil.CreateExecPodOrFail(oc.AdminKubeClient().Core(), ns, "execpod")
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By(fmt.Sprintf("deploying a service using a reencrypt route using only defaults"))
			err := oc.Run("create").Args("-f", fixture).Execute()
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

			url := "https://" + hostname
			g.By(fmt.Sprintf("verifying the route serves 200 from %s", url))
			err = waitForURLOK(ns, execPodName, url, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

func waitForURLOK(ns, execPodName, url string, timeoutSeconds int) error {
	cmd := fmt.Sprintf(`
		set -e
		for i in $(seq 1 %d); do
			code=$( curl -m 1 -k -s -o /dev/null -w '%%{http_code}\n' %q ) || rc=$?
			if [[ "${rc:-0}" -eq 0 ]]; then
				echo $code
				if [[ $code -eq 200 ]]; then
					exit 0
				fi
				if [[ $code -ne 503 ]]; then
					exit 1
				fi
			else
				echo "error ${rc}" 1>&2
			fi
			sleep 1
		done
		`, timeoutSeconds, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return fmt.Errorf("failed to receive 200 from %s: %v\n%s", url, err, output)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if lines[len(lines)-1] != "200" {
		return fmt.Errorf("last response from %s was not 200:\n%s", url, output)
	}
	return nil
}
