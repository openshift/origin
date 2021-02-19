package router

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	routev1 "github.com/openshift/api/route/v1"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"

	exutil "github.com/openshift/origin/test/extended/util"
)

const changeTimeoutSeconds = 3 * 60

var _ = g.Describe("[sig-network][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		oc          *exutil.CLI
		ns          string
		routerImage string
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(ns)
			if routes, _ := client.List(context.Background(), metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWith("router-", oc)
		}
	})

	oc = exutil.NewCLI("router-scoped")

	g.BeforeEach(func() {
		ns = oc.Namespace()

		var err error
		routerImage, err = exutil.FindRouterImage(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		configPath := exutil.FixturePath("testdata", "router", "router-common.yaml")
		err = oc.AsAdmin().Run("new-app").Args("-f", configPath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router", func() {
		g.It("should serve the correct routes when scoped to a single namespace and label set", func() {

			configPath := exutil.FixturePath("testdata", "router", "router-scoped.yaml")
			g.By(fmt.Sprintf("creating a router from a config file %q", configPath))
			err := oc.AsAdmin().Run("new-app").Args("-f", configPath, "-p", "IMAGE="+routerImage).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			ns := oc.KubeFramework().Namespace.Name
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
			routerURL := fmt.Sprintf("http://%s", routerIP)

			g.By("waiting for the healthz endpoint to respond")
			healthzURI := fmt.Sprintf("http://%s:1936/healthz", routerIP)
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
		})

		g.It("should override the route host with a custom value", func() {

			configPath := exutil.FixturePath("testdata", "router", "router-override.yaml")
			g.By(fmt.Sprintf("creating a router from a config file %q", configPath))
			err := oc.AsAdmin().Run("new-app").Args("-f", configPath, "-p", "IMAGE="+routerImage).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			ns := oc.KubeFramework().Namespace.Name
			execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
			defer func() {
				oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
			}()

			var routerIP string
			err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(ns).Get(context.Background(), "router-override", metav1.GetOptions{})
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
			err = waitForRouterOKResponseExec(ns, execPod.Name, healthzURI, routerIP, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the valid route to respond")
			err = waitForRouterOKResponseExec(ns, execPod.Name, routerURL+"/Letter", fmt.Sprintf(pattern, "route-1", ns), changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking that the stored domain name does not match a route")
			host := "first.example.com"
			err = expectRouteStatusCodeExec(ns, execPod.Name, routerURL+"/Letter", host, http.StatusServiceUnavailable)
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, host := range []string{"route-1", "route-2"} {
				host = fmt.Sprintf(pattern, host, ns)
				g.By(fmt.Sprintf("checking that %s matches a route", host))
				err = expectRouteStatusCodeExec(ns, execPod.Name, routerURL+"/Letter", host, http.StatusOK)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("checking that the router reported the correct ingress and override")
			r, err := oc.RouteClient().RouteV1().Routes(ns).Get(context.Background(), "route-1", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			ingress := ingressForName(r, "test-override")
			e2e.Logf("Selected: %#v, All: %#v", ingress, r.Status.Ingress)
			o.Expect(ingress).NotTo(o.BeNil())
			o.Expect(ingress.Host).To(o.Equal(fmt.Sprintf(pattern, "route-1", ns)))
			status, condition := IngressConditionStatus(ingress, routev1.RouteAdmitted)
			o.Expect(status).To(o.Equal(corev1.ConditionTrue))
			o.Expect(condition.LastTransitionTime).NotTo(o.BeNil())
		})

		g.It("should override the route host for overridden domains with a custom value", func() {

			configPath := exutil.FixturePath("testdata", "router", "router-override-domains.yaml")
			g.By(fmt.Sprintf("creating a router from a config file %q", configPath))
			err := oc.AsAdmin().Run("new-app").Args("-f", configPath, "-p", "IMAGE="+routerImage).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			ns := oc.KubeFramework().Namespace.Name
			execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
			defer func() {
				oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
			}()

			g.By(fmt.Sprintf("creating a scoped router with overridden domains from a config file %q", configPath))

			var routerIP string
			err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(ns).Get(context.Background(), "router-override-domains", metav1.GetOptions{})
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
			pattern := "%s-%s.apps.veto.test"

			g.By("waiting for the healthz endpoint to respond")
			healthzURI := fmt.Sprintf("http://%s:1936/healthz", routerIP)
			err = waitForRouterOKResponseExec(ns, execPod.Name, healthzURI, routerIP, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the valid route to respond")
			err = waitForRouterOKResponseExec(ns, execPod.Name, routerURL+"/Letter", fmt.Sprintf(pattern, "route-override-domain-1", ns), changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking that the stored domain name does not match a route")
			host := "y.a.null.ptr"
			err = expectRouteStatusCodeExec(ns, execPod.Name, routerURL+"/Letter", host, http.StatusServiceUnavailable)
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, host := range []string{"route-override-domain-1", "route-override-domain-2"} {
				host = fmt.Sprintf(pattern, host, ns)
				g.By(fmt.Sprintf("checking that %s matches a route", host))
				err = expectRouteStatusCodeExec(ns, execPod.Name, routerURL+"/Letter", host, http.StatusOK)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("checking that the router reported the correct ingress and override")
			r, err := oc.RouteClient().RouteV1().Routes(ns).Get(context.Background(), "route-override-domain-2", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			ingress := ingressForName(r, "test-override-domains")
			o.Expect(ingress).NotTo(o.BeNil())
			o.Expect(ingress.Host).To(o.Equal(fmt.Sprintf(pattern, "route-override-domain-2", ns)))
			status, condition := IngressConditionStatus(ingress, routev1.RouteAdmitted)
			o.Expect(status).To(o.Equal(corev1.ConditionTrue))
			o.Expect(condition.LastTransitionTime).NotTo(o.BeNil())
		})
	})
})

func waitForRouterOKResponseExec(ns, execPodName, url, host string, timeoutSeconds int) error {
	cmd := fmt.Sprintf(`
		set -e
		pass=%[4]d
		STOP=$(($(date '+%%s') + %[1]d))
		while [ $(date '+%%s') -lt $STOP ]; do
			rc=0
			code=$( curl -k -s -m 5 -o /dev/null -w '%%{http_code}\n' --header 'Host: %[2]s' %[3]q ) || rc=$?
			if [[ "${rc:-0}" -eq 0 ]]; then
				echo $code
				if [[ $code -eq 200 ]]; then
					pass=$(( pass - 1 ))
					if [[ $pass -le 0 ]]; then
						exit 0
					fi
					sleep 1
					continue
				fi
				if [[ $code -ne 503 ]]; then
					exit 1
				fi
				pass=%[4]d
			else
				echo "error ${rc}" 1>&2
			fi
			sleep 1
		done
		`, timeoutSeconds, host, url, 5)
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

func expectRouteStatusCodeRepeatedExec(ns, execPodName, url, host string, statusCode int, times int, proxy bool) error {
	var extraArgs []string
	if proxy {
		extraArgs = append(extraArgs, "--haproxy-protocol")
	}
	args := strings.Join(extraArgs, " ")

	cmd := fmt.Sprintf(`
		set -e
		STOP=$(($(date '+%%s') + %d))
		while [ $(date '+%%s') -lt $STOP ]; do
			rc=0
			code=$( curl %s -s -m 5 -o /dev/null -w '%%{http_code}\n' --header 'Host: %s' %q ) || rc=$?
			if [[ "${rc:-0}" -eq 0 ]]; then
				echo $code
				if [[ $code -ne %d ]]; then
					exit 1
				fi
			else
				echo "error ${rc}" 1>&2
			fi
		done
		`, times, args, host, url, statusCode)
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

func ingressForName(r *routev1.Route, name string) *routev1.RouteIngress {
	for i, ingress := range r.Status.Ingress {
		if ingress.RouterName == name {
			return &r.Status.Ingress[i]
		}
	}
	return nil
}
