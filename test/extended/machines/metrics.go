package operators

import (
	g "github.com/onsi/ginkgo"

	prom "github.com/openshift/origin/test/extended/prometheus"
	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Feature:Machines][Serial] Prometheus", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("prometheus")

		url, bearerToken string
	)

	g.BeforeEach(func() {
		var ok bool
		url, bearerToken, ok = prom.LocatePrometheus(oc)
		if !ok {
			e2e.Skipf("Prometheus could not be located on this cluster, skipping prometheus test")
		}
	})
	g.Describe("when installed on the cluster", func() {
		g.It("should have machine api operator metrics", func() {
			oc.SetupProject()
			ns := oc.Namespace()
			execPodName := e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) { pod.Spec.Containers[0].Image = "centos:7" })
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			tests := map[string][]prom.MetricTest{
				`mapi_mao_collector_up`: {prom.MetricTest{GreaterThanEqual: true, Value: 1}},
			}
			prom.RunQueries(tests, oc, ns, execPodName, url, bearerToken)
		})
	})
})
