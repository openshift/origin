package images

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

const changeTimeoutSeconds = 3 * 60

var _ = g.Describe("[Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exutil.FixturePath("testdata", "scoped-router.yaml")
		oc         = exutil.NewCLI("scoped-router", exutil.KubeConfigPath())
	)

	g.BeforeEach(func() {
		imagePrefix := os.Getenv("OS_IMAGE_PREFIX")
		if len(imagePrefix) == 0 {
			imagePrefix = "openshift/origin"
		}
		err := oc.AsAdmin().Run("new-app").Args("-f", configPath, "-p", "IMAGE="+imagePrefix+"-haproxy-router").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router", func() {
		g.It("should serve the correct routes when scoped to a single namespace and label set", func() {
			defer func() {
				if g.CurrentGinkgoTestDescription().Failed {
					dumpScopedRouterLogs(oc, g.CurrentGinkgoTestDescription().FullTestText)
				}
			}()
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
		})

		g.It("should override the route host with a custom value", func() {
			defer func() {
				if g.CurrentGinkgoTestDescription().Failed {
					dumpScopedRouterLogs(oc, g.CurrentGinkgoTestDescription().FullTestText)
				}
			}()
			oc.SetOutputDir(exutil.TestContext.OutputDir)
			ns := oc.KubeFramework().Namespace.Name
			execPodName := exutil.CreateExecPodOrFail(oc.AdminKubeClient().CoreV1(), ns, "execpod")
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By(fmt.Sprintf("creating a scoped router from a config file %q", configPath))

			var routerIP string
			err := wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(ns).Get("router-override", metav1.GetOptions{})
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
			err = waitForRouterOKResponseExec(ns, execPodName, healthzURI, routerIP, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the valid route to respond")
			err = waitForRouterOKResponseExec(ns, execPodName, routerURL+"/Letter", fmt.Sprintf(pattern, "route-1", ns), changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking that the stored domain name does not match a route")
			host := "first.example.com"
			err = expectRouteStatusCodeExec(ns, execPodName, routerURL+"/Letter", host, http.StatusServiceUnavailable)
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, host := range []string{"route-1", "route-2"} {
				host = fmt.Sprintf(pattern, host, ns)
				g.By(fmt.Sprintf("checking that %s matches a route", host))
				err = expectRouteStatusCodeExec(ns, execPodName, routerURL+"/Letter", host, http.StatusOK)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("checking that the router reported the correct ingress and override")
			r, err := oc.RouteClient().Route().Routes(ns).Get("route-1", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			ingress := ingressForName(r, "test-override")
			o.Expect(ingress).NotTo(o.BeNil())
			o.Expect(ingress.Host).To(o.Equal(fmt.Sprintf(pattern, "route-1", ns)))
			status, condition := routeapi.IngressConditionStatus(ingress, routeapi.RouteAdmitted)
			o.Expect(status).To(o.Equal(kapi.ConditionTrue))
			o.Expect(condition.LastTransitionTime).NotTo(o.BeNil())
		})
	})
})

func waitForRouterOKResponseExec(ns, execPodName, url, host string, timeoutSeconds int) error {
	cmd := fmt.Sprintf(`
		set -e
		for i in $(seq 1 %d); do
			code=$( curl -k -s -o /dev/null -w '%%{http_code}\n' --header 'Host: %s' %q ) || rc=$?
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
		`, timeoutSeconds, host, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if lines[len(lines)-1] != "200" {
		return fmt.Errorf("last response from server was not 200:\n%s", output)
	}
	return nil
}

func expectRouteStatusCodeRepeatedExec(ns, execPodName, url, host string, statusCode int, times int) error {
	cmd := fmt.Sprintf(`
		set -e
		for i in $(seq 1 %d); do
			code=$( curl -s -o /dev/null -w '%%{http_code}\n' --header 'Host: %s' %q ) || rc=$?
			if [[ "${rc:-0}" -eq 0 ]]; then
				echo $code
				if [[ $code -ne %d ]]; then
					exit 1
				fi
			else
				echo "error ${rc}" 1>&2
			fi
		done
		`, times, host, url, statusCode)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return nil
}

func expectRouteStatusCodeExec(ns, execPodName, url, host string, statusCode int) error {
	cmd := fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' --header 'Host: %s' %q", host, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	if output != strconv.Itoa(statusCode) {
		return fmt.Errorf("last response from server was not %d: %s", statusCode, output)
	}
	return nil
}

func getAuthenticatedRouteURLViaPod(ns, execPodName, url, host, user, pass string) (string, error) {
	cmd := fmt.Sprintf("curl -s -u %s:%s --header 'Host: %s' %q", user, pass, host, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}

func dumpScopedRouterLogs(oc *exutil.CLI, name string) {
	log, _ := e2e.GetPodLogs(oc.AdminKubeClient(), oc.KubeFramework().Namespace.Name, "scoped-router", "router")
	e2e.Logf("Scoped Router test %s logs:\n %s", name, log)
}

func ingressForName(r *routeapi.Route, name string) *routeapi.RouteIngress {
	for i, ingress := range r.Status.Ingress {
		if ingress.RouterName == name {
			return &r.Status.Ingress[i]
		}
	}
	return nil
}
