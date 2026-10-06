package edge_topologies

import (
	"context"
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
		pcs           *tnfPCSRunner
	)

	g.BeforeEach(func() {
		ctx := context.Background()
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)
		if pcs == nil {
			coreClient := oc.AdminKubeClient().CoreV1()
			pcs = &tnfPCSRunner{oc: oc, namespaces: coreClient.Namespaces(), serviceAccounts: coreClient.ServiceAccounts}
		}
		g.DeferCleanup(func() {
			o.Expect(pcs.cleanup(context.Background())).To(o.Succeed(), "clean up any remaining TNF debug namespace")
		})
		o.Expect(pcs.cleanup(ctx)).To(o.Succeed(), "finish previous remote debug cleanup before starting another scenario")
		utils.SkipIfClusterIsNotHealthy(oc, helpers.NewEtcdClientFactory(oc.KubeClient()))

		nodeCtx, cancelNodes := context.WithTimeout(ctx, tnfCommandTimeout)
		masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(nodeCtx, metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master=",
		})
		cancelNodes()
		o.Expect(err).NotTo(o.HaveOccurred(), "list control-plane nodes before TNF metric testing")
		o.Expect(len(masterNodes.Items)).To(o.Equal(2), "TNF metrics tests require exactly two control-plane nodes")

		nodes = nodes[:0]
		for _, node := range masterNodes.Items {
			nodes = append(nodes, node.Name)
		}
		sort.Strings(nodes)

		podCtx, cancelPods := context.WithTimeout(ctx, tnfCommandTimeout)
		prometheusPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-monitoring").List(podCtx, metav1.ListOptions{})
		cancelPods()
		o.Expect(err).NotTo(o.HaveOccurred(), "list monitoring pods before selecting a Ready Prometheus pod")
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
		o.Expect(verifyTNFPrometheusRule(ctx, oc, prometheusPod)).To(o.Succeed(), "load the expected TNF alert rules before disruption")

		g.By("verifying all 53 TNF gauges are healthy before disruption")
		healthy := expectedHealthyTNFGauges(nodes)
		o.Expect(waitForTNFGauges(ctx, oc, healthy)).To(o.Succeed(), "establish the healthy TNF metric baseline")
		gauges, err := queryTNFGauges(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "query the TNF metric inventory")
		o.Expect(len(gauges)).To(o.Equal(53), "expected exactly 53 TNF gauges")

		g.By("verifying no TNF alerts are firing before disruption")
		o.Expect(waitForNoFiringTNFAlerts(ctx, oc, prometheusPod)).To(o.Succeed(), "clear firing TNF alerts before disruption")
	})

	g.It("reports cluster maintenance and recovery", func() {
		exerciseTNFMetricsDisruption(
			oc,
			"cluster maintenance",
			clusterMaintenanceTNFGauges(nodes),
			expectedHealthyTNFGauges(nodes),
			func(ctx context.Context) error {
				return pcs.run(ctx, nodes[0], "property", "set", "maintenance-mode=true")
			},
			func(ctx context.Context) error {
				return pcs.run(ctx, nodes[0], "property", "set", "maintenance-mode=false")
			},
		)
	})

	g.It("reports an unmanaged resource and recovery", func() {
		exerciseTNFMetricsDisruption(
			oc,
			"unmanaged etcd resource",
			resourceUnmanagedTNFGauges(nodes),
			expectedHealthyTNFGauges(nodes),
			func(ctx context.Context) error { return pcs.run(ctx, nodes[0], "resource", "unmanage", "etcd") },
			func(ctx context.Context) error { return pcs.run(ctx, nodes[0], "resource", "manage", "etcd") },
		)
	})

	g.It("reports node maintenance and recovery", func() {
		targetNode := nodes[1]
		exerciseTNFMetricsDisruption(
			oc,
			"node maintenance",
			nodeMaintenanceTNFGauges(targetNode),
			expectedHealthyTNFGauges(nodes),
			func(ctx context.Context) error {
				return pcs.run(ctx, nodes[0], "node", "maintenance", targetNode)
			},
			func(ctx context.Context) error {
				return pcs.run(ctx, nodes[0], "node", "unmaintenance", targetNode)
			},
		)
	})

	g.It("reports a disabled fence device and recovery", func() {
		targetNode := nodes[0]
		pacemakerCluster, err := apis.GetPacemakerCluster(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "read PacemakerCluster before selecting the fencing agent")
		fenceDevice, err := singleTNFFencingAgent(pacemakerCluster, targetNode)
		o.Expect(err).NotTo(o.HaveOccurred(), "verify the single-agent prerequisite before disabling fencing")

		exerciseTNFMetricsDisruption(
			oc,
			"disabled fence device",
			fenceDisabledTNFGauges(targetNode),
			expectedHealthyTNFGauges(nodes),
			func(ctx context.Context) error {
				return pcs.run(ctx, nodes[0], "stonith", "disable", fenceDevice)
			},
			func(ctx context.Context) error { return pcs.run(ctx, nodes[0], "stonith", "enable", fenceDevice) },
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
		o.Expect(retryTNFOperation(cleanupCtx, tnfPollInterval, tnfCommandTimeout, restore)).To(o.Succeed(), "restore Pacemaker state during cleanup after %s", description)
		o.Expect(waitForTNFGauges(cleanupCtx, oc, healthy)).To(o.Succeed(), "recover TNF metrics during cleanup after %s", description)
		o.Expect(waitForTNFClusterHealthy(oc)).To(o.Succeed(), "recover cluster health during cleanup after %s", description)
	})

	g.By("triggering " + description)
	o.Expect(disrupt(ctx)).To(o.Succeed(), "apply Pacemaker disruption: %s", description)

	g.By("waiting for the expected TNF gauges to report the disruption")
	expectedDuringDisruption := tnfGaugesWithOverrides(healthy, disruptedOverrides)
	o.Expect(waitForTNFGauges(ctx, oc, expectedDuringDisruption)).To(o.Succeed(), "observe TNF metric transitions during %s", description)

	g.By("restoring state after " + description)
	o.Expect(retryTNFOperation(ctx, tnfPollInterval, tnfCommandTimeout, restore)).To(o.Succeed(), "restore Pacemaker state after %s", description)

	g.By("waiting for all TNF gauges to recover")
	o.Expect(waitForTNFGauges(ctx, oc, healthy)).To(o.Succeed(), "recover TNF metrics after %s", description)

	g.By("verifying the cluster is healthy after recovery")
	o.Expect(waitForTNFClusterHealthy(oc)).To(o.Succeed(), "recover cluster health after %s", description)
	restored = true
}
