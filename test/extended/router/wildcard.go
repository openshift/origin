package router

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	watchtools "k8s.io/client-go/tools/watch"

	routev1 "github.com/openshift/origin/pkg/route/apis/route"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	exdns "github.com/openshift/origin/test/extended/dns"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		oc *exutil.CLI

		ns string
	)

	oc = exutil.NewCLI("router-wildcard", exutil.KubeConfigPath())

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
		g.It("should enable a route whose default host can be resolved by external DNS", func() {
			g.By("creating a simple route with a default hostname")
			route := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns,
					Name:      "simple",
				},
				Spec: routev1.RouteSpec{
					To: routev1.RouteTargetReference{
						Name: "nowhere",
					},
				},
			}
			_, err := oc.RouteClient().Route().Routes(ns).Create(route)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the route to be assigned a host")
			var hostname string
			err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				route, err := oc.RouteClient().Route().Routes(route.Namespace).Get(route.Name, metav1.GetOptions{})
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

			g.By(fmt.Sprintf("verifying the route host %q can be resolved through DNS", hostname))
			probeCommand := probeCommand(hostname, changeTimeoutSeconds)
			pod := exdns.CreateDNSPod(ns, probeCommand)

			g.By("submitting the probe pod to kubernetes")
			podClient := oc.AdminKubeClient().Core().Pods(ns)
			defer func() {
				g.By("deleting the probe pod")
				defer g.GinkgoRecover()
				podClient.Delete(pod.Name, metav1.NewDeleteOptions(0))
			}()
			updated, err := podClient.Create(pod)
			if err != nil {
				e2e.Failf("Failed to create %s pod: %v", pod.Name, err)
			}

			w, err := podClient.Watch(metav1.SingleObject(metav1.ObjectMeta{Name: pod.Name, ResourceVersion: updated.ResourceVersion}))
			if err != nil {
				e2e.Failf("Failed: %v", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), e2e.PodStartTimeout)
			defer cancel()
			succeeded := true
			if _, err = watchtools.UntilWithoutRetry(ctx, w, exdns.PodSucceeded); err != nil {
				e2e.Logf("Failed: %v", err)
				succeeded = false
			}

			g.By("retrieving the probe pod logs")
			r, err := podClient.GetLogs(pod.Name, &corev1.PodLogOptions{Container: "querier"}).Stream()
			if err != nil {
				e2e.Failf("Failed to get pod logs %s: %v", pod.Name, err)
			}
			out, err := ioutil.ReadAll(r)
			if err != nil {
				e2e.Failf("Failed to read pod logs %s: %v", pod.Name, err)
			}
			e2e.Logf("Got results from pod:\n%s", out)
			if !succeeded {
				e2e.Failf("DNS probe pod failed")
			}
		})
	})
})

func probeCommand(hostname string, timeoutSeconds int) string {
	return fmt.Sprintf(`
set +x
for i in $(seq 1 %d); do
  test -n "$(dig +noall +answer %s)" && exit 0
  sleep 1
done
dig %s
exit 1
`, timeoutSeconds, hostname, hostname)
}
