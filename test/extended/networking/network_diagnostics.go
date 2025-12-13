package networking

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/operator/v1"
	applyconfigv1 "github.com/openshift/client-go/config/applyconfigurations/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	namespace     = "openshift-network-diagnostics"
	clusterConfig = "cluster"
	condition     = "NetworkDiagnosticsAvailable"
	fieldManager  = "network-diagnostics-e2e"
)

// This is [Serial] because it modifies the cluster/network.config.openshift.io object in each test.
var _ = g.Describe("[sig-network][OCPFeatureGate:NetworkDiagnosticsConfig][Serial]", g.Ordered, func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("network-diagnostics")

	g.BeforeAll(func(ctx context.Context) {
		// Check if the test can write to cluster/network.config.openshift.io
		hasAccess, err := hasNetworkConfigWriteAccess(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if !hasAccess {
			skipper.Skipf("The test is not permitted to modify the cluster/network.config.openshift.io resource")
		}

		// Reset and take ownership of the network diagnostics config
		patch := []byte(`{"spec":{"networkDiagnostics":null}}`)
		_, err = oc.AdminConfigClient().ConfigV1().Networks().Patch(ctx, clusterConfig, types.MergePatchType, patch, metav1.PatchOptions{FieldManager: fieldManager})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.AfterEach(func(ctx context.Context) {
		// Reset network diagnostics config, this will remove changes made by the fieldManager
		netConfigApply := applyconfigv1.Network(clusterConfig).WithSpec(
			applyconfigv1.NetworkSpec().WithNetworkDiagnostics(nil),
		)
		_, err := oc.AdminConfigClient().ConfigV1().Networks().Apply(ctx, netConfigApply,
			metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should be enabled by default", g.Label("Size:M"), func(ctx context.Context) {
		o.Eventually(func() bool {
			g.By("running one network-check-source pod")
			srcPods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=network-check-source"})
			if err != nil {
				framework.Logf("Error getting pods in %s namespace: %v", namespace, err)
				return false
			}
			if len(srcPods.Items) != 1 {
				framework.Logf("Invalid amount of source pods: %d", len(srcPods.Items))
				return false
			}

			g.By("running a network-check-target pod on every node")
			targetPods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=network-check-target"})
			if err != nil {
				framework.Logf("Error getting pods in %s namespace: %v", namespace, err)
				return false
			}
			nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				framework.Logf("Error getting nodes: %v", err)
				return false
			}
			if len(targetPods.Items) != len(nodes.Items) {
				framework.Logf("Invalid amount of destination pods want:%d, got: %d", len(nodes.Items), len(targetPods.Items))
				return false
			}

			cfg, err := oc.AdminConfigClient().ConfigV1().Networks().Get(ctx, clusterConfig, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting cluster config: %v", err)
				return false
			}
			return meta.IsStatusConditionTrue(cfg.Status.Conditions, condition)
		}, 3*time.Minute, 5*time.Second).Should(o.BeTrue())
	})

	g.It("Should remove all network diagnostics pods when disabled", g.Label("Size:M"), func(ctx context.Context) {
		netConfigApply := applyconfigv1.Network(clusterConfig).WithSpec(
			applyconfigv1.NetworkSpec().WithNetworkDiagnostics(
				applyconfigv1.NetworkDiagnostics().WithMode(configv1.NetworkDiagnosticsDisabled),
			),
		)
		_, err := oc.AdminConfigClient().ConfigV1().Networks().Apply(ctx, netConfigApply,
			metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Eventually(func() bool {
			pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				framework.Logf("Error getting pods in %s namespace: %v", namespace, err)
				return false
			}
			if len(pods.Items) != 0 {
				return false
			}

			cfg, err := oc.AdminConfigClient().ConfigV1().Networks().Get(ctx, clusterConfig, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting cluster config: %v", err)
				return false
			}
			return meta.IsStatusConditionFalse(cfg.Status.Conditions, condition)
		}, 3*time.Minute, 5*time.Second).Should(o.BeTrue())
	})

	g.It("Should move the source diagnostics pods based on the new selector and tolerations", g.Label("Size:M"), func(ctx context.Context) {
		// Intentionally omit setting the mode to ensure that the diagnostics are enabled when it is unset
		netConfigApply := applyconfigv1.Network(clusterConfig).WithSpec(
			applyconfigv1.NetworkSpec().WithNetworkDiagnostics(
				applyconfigv1.NetworkDiagnostics().
					WithSourcePlacement(
						applyconfigv1.NetworkDiagnosticsSourcePlacement().
							WithNodeSelector(map[string]string{"node-role.kubernetes.io/master": ""}).
							WithTolerations(corev1.Toleration{
								Operator: corev1.TolerationOpExists,
							}),
					).WithTargetPlacement(nil),
			),
		)
		_, err := oc.AdminConfigClient().ConfigV1().Networks().Apply(ctx, netConfigApply,
			metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Eventually(func() bool {
			pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=network-check-source"})
			if err != nil {
				framework.Logf("Error getting pods in %s namespace: %v", namespace, err)
				return false
			}
			if len(pods.Items) == 0 {
				framework.Logf("No diagnostics pods found")
				return false
			}
			for _, pod := range pods.Items {
				if pod.Spec.NodeName == "" {
					framework.Logf("Diagnostics pod %s is not scheduled to any node", pod.Name)
					return false
				}
				node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
				if err != nil {
					framework.Logf("Error getting node %s: %v", pod.Spec.NodeName, err)
					return false
				}
				if _, ok := node.Labels["node-role.kubernetes.io/master"]; !ok {
					framework.Logf("Diagnostics pod %s is not scheduled to a master node", pod.Name)
					return false
				}
			}
			cfg, err := oc.AdminConfigClient().ConfigV1().Networks().Get(ctx, clusterConfig, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting cluster config: %v", err)
				return false
			}
			return meta.IsStatusConditionTrue(cfg.Status.Conditions, condition)
		}, 3*time.Minute, 5*time.Second).Should(o.BeTrue())
	})

	g.It("Should move the target diagnostics pods based on the new selector and tolerations", g.Label("Size:M"), func(ctx context.Context) {
		netConfigApply := applyconfigv1.Network(clusterConfig).WithSpec(
			applyconfigv1.NetworkSpec().WithNetworkDiagnostics(
				applyconfigv1.NetworkDiagnostics().
					WithMode(configv1.NetworkDiagnosticsAll).
					WithTargetPlacement(
						applyconfigv1.NetworkDiagnosticsTargetPlacement().
							WithNodeSelector(map[string]string{"node-role.kubernetes.io/master": ""}).
							WithTolerations(corev1.Toleration{
								Operator: corev1.TolerationOpExists,
								Key:      "node-role.kubernetes.io/master",
								Effect:   corev1.TaintEffectNoSchedule,
							}),
					),
			),
		)
		_, err := oc.AdminConfigClient().ConfigV1().Networks().Apply(ctx, netConfigApply,
			metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Eventually(func() bool {
			pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=network-check-target"})
			if err != nil {
				framework.Logf("Error getting pods in %s namespace: %v", namespace, err)
				return false
			}
			if len(pods.Items) == 0 {
				framework.Logf("No diagnostics pods found")
				return false
			}
			for _, pod := range pods.Items {
				if pod.Spec.NodeName == "" {
					framework.Logf("Diagnostics pod %s is not scheduled to any node", pod.Name)
					return false
				}
				node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
				if err != nil {
					framework.Logf("Error getting node %s: %v", pod.Spec.NodeName, err)
					return false
				}
				if _, ok := node.Labels["node-role.kubernetes.io/master"]; !ok {
					framework.Logf("Diagnostics pod %s is not scheduled to a master node", pod.Name)
					return false
				}
			}
			cfg, err := oc.AdminConfigClient().ConfigV1().Networks().Get(ctx, clusterConfig, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting cluster config: %v", err)
				return false
			}
			return meta.IsStatusConditionTrue(cfg.Status.Conditions, condition)
		}, 3*time.Minute, 5*time.Second).Should(o.BeTrue())
	})

	g.It("Should function without any target pods", g.Label("Size:M"), func(ctx context.Context) {
		netConfigApply := applyconfigv1.Network(clusterConfig).WithSpec(
			applyconfigv1.NetworkSpec().WithNetworkDiagnostics(
				applyconfigv1.NetworkDiagnostics().
					WithMode(configv1.NetworkDiagnosticsAll).
					WithTargetPlacement(
						applyconfigv1.NetworkDiagnosticsTargetPlacement().
							WithNodeSelector(map[string]string{"alien": ""}),
					),
			),
		)
		_, err := oc.AdminConfigClient().ConfigV1().Networks().Apply(ctx, netConfigApply,
			metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Eventually(func() bool {
			pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=network-check-target"})
			if err != nil {
				framework.Logf("Error getting pods in %s namespace: %v", namespace, err)
				return false
			}
			if len(pods.Items) != 0 {
				framework.Logf("Target diagnostics pods found")
				return false
			}
			cfg, err := oc.AdminConfigClient().ConfigV1().Networks().Get(ctx, clusterConfig, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting cluster config: %v", err)
				return false
			}
			return meta.IsStatusConditionTrue(cfg.Status.Conditions, condition)
		}, 3*time.Minute, 5*time.Second).Should(o.BeTrue())
	})

	g.It("Should set the condition to false if there are no nodes able to host the source pods", g.Label("Size:M"), func(ctx context.Context) {
		netConfigApply := applyconfigv1.Network(clusterConfig).WithSpec(
			applyconfigv1.NetworkSpec().WithNetworkDiagnostics(
				applyconfigv1.NetworkDiagnostics().
					WithMode(configv1.NetworkDiagnosticsAll).
					WithSourcePlacement(
						applyconfigv1.NetworkDiagnosticsSourcePlacement().
							WithNodeSelector(map[string]string{"alien": ""}),
					),
			),
		)
		_, err := oc.AdminConfigClient().ConfigV1().Networks().Apply(ctx, netConfigApply,
			metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Eventually(func() bool {
			// Should not affect the Progressing condition of network.operator
			oper, err := oc.AdminOperatorClient().OperatorV1().Networks().Get(ctx, clusterConfig, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting cluster operator: %v", err)
				return false
			}
			for _, operCondition := range oper.Status.Conditions {
				if operCondition.Type == "Progressing" && operCondition.Status != v1.ConditionFalse {
					framework.Logf("Invalid progressing condition: %v", operCondition)
					return false
				}
			}

			cfg, err := oc.AdminConfigClient().ConfigV1().Networks().Get(ctx, clusterConfig, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting cluster config: %v", err)
				return false
			}
			return meta.IsStatusConditionFalse(cfg.Status.Conditions, condition)
		}, 3*time.Minute, 5*time.Second).Should(o.BeTrue())
	})

})
