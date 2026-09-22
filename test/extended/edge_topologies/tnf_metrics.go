package edge_topologies

import (
	"context"
	"fmt"
	"sort"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/edge_topologies/utils"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/apis"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	podutils "k8s.io/kubernetes/pkg/api/v1/pod"
)

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][TNFMetrics][Serial][Disruptive] TNF metrics", func() {
	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLIWithoutNamespace("tnf-metrics").AsAdmin()
		nodes         []string
		prometheusPod string
	)

	g.BeforeEach(func() {
		ctx := context.Background()
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)
		utils.SkipIfClusterIsNotHealthy(oc, helpers.NewEtcdClientFactory(oc.KubeClient()))

		masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master=",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(masterNodes.Items).To(o.HaveLen(2), "TNF metrics tests require exactly two control-plane nodes")

		nodes = nodes[:0]
		for _, node := range masterNodes.Items {
			nodes = append(nodes, node.Name)
		}
		sort.Strings(nodes)

		prometheusPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-monitoring").List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		readyPrometheusPods := []string{}
		for i := range prometheusPods.Items {
			pod := &prometheusPods.Items[i]
			if strings.HasPrefix(pod.Name, "prometheus-k8s-") && podutils.IsPodReady(pod) {
				readyPrometheusPods = append(readyPrometheusPods, pod.Name)
			}
		}
		sort.Strings(readyPrometheusPods)
		o.Expect(readyPrometheusPods).NotTo(o.BeEmpty(), "no Ready prometheus-k8s pod found")
		prometheusPod = readyPrometheusPods[0]

		g.By("verifying all TNF alert rules are loaded by Prometheus")
		o.Expect(verifyTNFPrometheusRule(ctx, oc, prometheusPod)).To(o.Succeed())

		g.By("verifying all 53 TNF gauges are healthy before disruption")
		healthy := expectedHealthyTNFGauges(nodes)
		o.Expect(waitForTNFGauges(ctx, oc, healthy)).To(o.Succeed())
		gauges, err := queryTNFGauges(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(gauges).To(o.HaveLen(53), "expected exactly 53 TNF gauges")

		g.By("verifying no TNF alerts are firing before disruption")
		o.Expect(waitForNoFiringTNFAlerts(ctx, oc, prometheusPod)).To(o.Succeed())
	})

	g.It("reports cluster maintenance and recovery", func() {
		exerciseTNFMetricsDisruption(
			oc,
			"cluster maintenance",
			clusterMaintenanceTNFGauges(nodes),
			expectedHealthyTNFGauges(nodes),
			func(ctx context.Context) error {
				return runTNFPCS(ctx, oc, nodes[0], "property", "set", "maintenance-mode=true")
			},
			func(ctx context.Context) error {
				return runTNFPCS(ctx, oc, nodes[0], "property", "set", "maintenance-mode=false")
			},
		)
	})

	g.It("reports an unmanaged resource and recovery", func() {
		exerciseTNFMetricsDisruption(
			oc,
			"unmanaged etcd resource",
			resourceUnmanagedTNFGauges(nodes),
			expectedHealthyTNFGauges(nodes),
			func(ctx context.Context) error { return runTNFPCS(ctx, oc, nodes[0], "resource", "unmanage", "etcd") },
			func(ctx context.Context) error { return runTNFPCS(ctx, oc, nodes[0], "resource", "manage", "etcd") },
		)
	})

	g.It("reports node maintenance and recovery", func() {
		targetNode := nodes[1]
		exerciseTNFMetricsDisruption(
			oc,
			fmt.Sprintf("maintenance for node %s", targetNode),
			nodeMaintenanceTNFGauges(targetNode),
			expectedHealthyTNFGauges(nodes),
			func(ctx context.Context) error {
				return runTNFPCS(ctx, oc, nodes[0], "node", "maintenance", targetNode)
			},
			func(ctx context.Context) error {
				return runTNFPCS(ctx, oc, nodes[0], "node", "unmaintenance", targetNode)
			},
		)
	})

	g.It("reports a disabled fence device and recovery", func() {
		targetNode := nodes[0]
		pacemakerCluster, err := apis.GetPacemakerCluster(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		fenceDevice, err := apis.FindStartedFencingAgent(pacemakerCluster, targetNode)
		o.Expect(err).NotTo(o.HaveOccurred())

		exerciseTNFMetricsDisruption(
			oc,
			fmt.Sprintf("disabled fence device %s", fenceDevice),
			fenceDisabledTNFGauges(targetNode),
			expectedHealthyTNFGauges(nodes),
			func(ctx context.Context) error {
				return runTNFPCS(ctx, oc, nodes[0], "stonith", "disable", fenceDevice)
			},
			func(ctx context.Context) error { return runTNFPCS(ctx, oc, nodes[0], "stonith", "enable", fenceDevice) },
		)
	})
})

func exerciseTNFMetricsDisruption(
	oc *exutil.CLI,
	description string,
	disruptedOverrides, healthy map[metricKey]float64,
	disrupt, restore func(context.Context) error,
) {
	ctx := context.Background()
	restored := false
	g.DeferCleanup(func() {
		if restored {
			return
		}
		cleanupCtx := context.Background()
		g.By("restoring state after " + description)
		o.Expect(retryTNFOperation(cleanupCtx, tnfPollInterval, tnfCommandTimeout, restore)).To(o.Succeed())
		o.Expect(waitForTNFGauges(cleanupCtx, oc, healthy)).To(o.Succeed())
		o.Expect(waitForTNFClusterHealthy(oc)).To(o.Succeed())
	})

	g.By("triggering " + description)
	o.Expect(disrupt(ctx)).To(o.Succeed())

	g.By("waiting for the expected TNF gauges to report the disruption")
	expectedDuringDisruption := tnfGaugesWithOverrides(healthy, disruptedOverrides)
	o.Expect(waitForTNFGauges(ctx, oc, expectedDuringDisruption)).To(o.Succeed())

	g.By("restoring state after " + description)
	o.Expect(retryTNFOperation(ctx, tnfPollInterval, tnfCommandTimeout, restore)).To(o.Succeed())

	g.By("waiting for all TNF gauges to recover")
	o.Expect(waitForTNFGauges(ctx, oc, healthy)).To(o.Succeed())

	g.By("verifying the cluster is healthy after recovery")
	o.Expect(waitForTNFClusterHealthy(oc)).To(o.Succeed())
	restored = true
}
