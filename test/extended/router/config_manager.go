package router

import (
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

const timeoutSeconds = 3 * 60

var _ = g.Describe("[Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exutil.FixturePath("testdata", "router-config-manager.yaml")
		oc         *exutil.CLI
		ns         string
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).Route().Routes(ns)
			if routes, _ := client.List(metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWith("router-", oc)
		}
	})

	oc = exutil.NewCLI("router-config-manager", exutil.KubeConfigPath())

	g.BeforeEach(func() {
		ns = oc.Namespace()

		routerImage, _ := exutil.FindImageFormatString(oc)
		routerImage = strings.Replace(routerImage, "${component}", "haproxy-router", -1)

		err := oc.AsAdmin().Run("new-app").Args("-f", configPath, "-p", "IMAGE="+routerImage).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router", func() {
		g.It("should serve the correct routes when running with the haproxy config manager", func() {
			g.Skip("TODO: This test is flaking, fix it")
			ns := oc.KubeFramework().Namespace.Name
			execPodName := exutil.CreateExecPodOrFail(oc.AdminKubeClient().CoreV1(), ns, "execpod")
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By(fmt.Sprintf("creating a router with haproxy config manager from a config file %q", configPath))

			var routerIP string
			err := wait.Poll(time.Second, timeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.KubeFramework().Namespace.Name).Get("router-haproxy-cfgmgr", metav1.GetOptions{})
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

			g.By("waiting for the healthz endpoint to respond")
			healthzURI := fmt.Sprintf("http://%s:1936/healthz", routerIP)
			err = waitForRouterOKResponseExec(ns, execPodName, healthzURI, routerIP, timeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the valid routes to respond")
			err = waitForRouteToRespond(ns, execPodName, "http", "insecure.hapcm.test", "/", routerIP, 0)
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, host := range []string{"edge.allow.hapcm.test", "reencrypt.hapcm.test", "passthrough.hapcm.test"} {
				err = waitForRouteToRespond(ns, execPodName, "https", host, "/", routerIP, 0)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("mini stress test by adding (and removing) different routes and checking that they are exposed")
			for i := 0; i < 16; i++ {
				name := fmt.Sprintf("hapcm-stress-insecure-%d", i)
				hostName := fmt.Sprintf("stress.insecure-%d.hapcm.test", i)
				err := oc.AsAdmin().Run("expose").Args("service", "insecure-service", "--name", name, "--hostname", hostName, "--labels", "select=haproxy-cfgmgr").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				err = waitForRouteToRespond(ns, execPodName, "http", hostName, "/", routerIP, 0)
				o.Expect(err).NotTo(o.HaveOccurred())

				err = oc.AsAdmin().Run("delete").Args("route", name).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				routeTypes := []string{"edge", "reencrypt", "passthrough"}
				for _, t := range routeTypes {
					name := fmt.Sprintf("hapcm-stress-%s-%d", t, i)
					hostName := fmt.Sprintf("stress.%s-%d.hapcm.test", t, i)
					serviceName := "secure-service"
					if t == "edge" {
						serviceName = "insecure-service"
					}

					err := oc.AsAdmin().Run("create").Args("route", t, name, "--service", serviceName, "--hostname", hostName).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
					err = oc.AsAdmin().Run("label").Args("route", name, "select=haproxy-cfgmgr").Execute()
					o.Expect(err).NotTo(o.HaveOccurred())

					err = waitForRouteToRespond(ns, execPodName, "https", hostName, "/", routerIP, 0)
					o.Expect(err).NotTo(o.HaveOccurred())

					err = oc.AsAdmin().Run("delete").Args("route", name).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
				}
			}
		})
	})
})

func waitForRouteToRespond(ns, execPodName, proto, host, abspath, ipaddr string, port int) error {
	if port == 0 {
		switch proto {
		case "http":
			port = 80
		case "https":
			port = 443
		default:
			port = 80
		}
	}
	uri := fmt.Sprintf("%s://%s:%d%s", proto, host, port, abspath)
	cmd := fmt.Sprintf(`
		set -e
		for i in $(seq 1 %d); do
			code=$( curl -k -s -o /dev/null -w '%%{http_code}\n' --resolve %s:%d:%s %q ) || rc=$?
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
		`, timeoutSeconds, host, port, ipaddr, uri)
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
