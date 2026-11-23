package networking

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/operator/v1"
	applyconfigv1 "github.com/openshift/client-go/config/applyconfigurations/config/v1"
	clusteroperatorhelpers "github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubectl/pkg/util/podutils"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	namespace     = "openshift-network-diagnostics"
	clusterConfig = "cluster"
	condition     = "NetworkDiagnosticsAvailable"
	fieldManager  = "network-diagnostics-e2e"

	diagnosticsSourceSelector  = "app=network-check-source"
	diagnosticsTargetSelector  = "app=network-check-target"
	networkClusterOperatorName = "network"
	networkClusterOperatorRef  = "clusteroperator/" + networkClusterOperatorName

	diagnosticsRecoveryTimeout           = 10 * time.Minute
	diagnosticsRecoveryPollInterval      = 5 * time.Second
	diagnosticsRecoveryConsecutivePasses = 2
	diagnosticsAPIRequestTimeout         = 30 * time.Second
	// The network operator appends the root cause at the end of its condition messages, so keep enough of them.
	diagnosticsMaxMessageLength = 1024
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
		err := resetNetworkDiagnosticsConfig(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should be enabled by default", func(ctx context.Context) {
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

	g.It("Should remove all network diagnostics pods when disabled", func(ctx context.Context) {
		snapshot, snapshotErr := takeDiagnosticsSnapshot(ctx, oc)
		o.Expect(snapshotErr).NotTo(o.HaveOccurred())
		podsRemoved := false
		// Registered before disabling so it runs after AfterEach has restored the config, even if this spec fails.
		g.DeferCleanup(func(ctx context.Context) {
			ensureDiagnosticsReenabled(ctx, oc)
			waitForDiagnosticsRecovery(ctx, oc, snapshot, podsRemoved)
		})

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
		podsRemoved = true
	})

	g.It("Should move the source diagnostics pods based on the new selector and tolerations", func(ctx context.Context) {
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

	g.It("Should move the target diagnostics pods based on the new selector and tolerations", func(ctx context.Context) {
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

	g.It("Should function without any target pods", func(ctx context.Context) {
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

	g.It("Should set the condition to false if there are no nodes able to host the source pods", func(ctx context.Context) {
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

type diagnosticsSnapshot struct {
	namespaceUID types.UID
	podUIDs      sets.Set[types.UID]
}

type diagnosticsObservation struct {
	namespace  *corev1.Namespace
	sourcePods []corev1.Pod
	targetPods []corev1.Pod
	operator   *configv1.ClusterOperator
}

type diagnosticsPodGroup struct {
	selector string
	pods     []corev1.Pod
}

func (obs diagnosticsObservation) podGroups() []diagnosticsPodGroup {
	return []diagnosticsPodGroup{
		{selector: diagnosticsSourceSelector, pods: obs.sourcePods},
		{selector: diagnosticsTargetSelector, pods: obs.targetPods},
	}
}

func resetNetworkDiagnosticsConfig(ctx context.Context, oc *exutil.CLI) error {
	netConfigApply := applyconfigv1.Network(clusterConfig).WithSpec(
		applyconfigv1.NetworkSpec().WithNetworkDiagnostics(nil),
	)
	_, err := oc.AdminConfigClient().ConfigV1().Networks().Apply(ctx, netConfigApply,
		metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
	return err
}

func takeDiagnosticsSnapshot(ctx context.Context, oc *exutil.CLI) (diagnosticsSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, diagnosticsAPIRequestTimeout)
	defer cancel()

	obs, err := observeDiagnosticsWorkloads(ctx, oc)
	if err != nil {
		return diagnosticsSnapshot{}, err
	}
	snapshot := diagnosticsSnapshot{podUIDs: sets.New[types.UID]()}
	if obs.namespace != nil {
		snapshot.namespaceUID = obs.namespace.UID
	}
	for _, group := range obs.podGroups() {
		for _, pod := range group.pods {
			snapshot.podUIDs.Insert(pod.UID)
		}
	}
	return snapshot, nil
}

func observeDiagnosticsWorkloads(ctx context.Context, oc *exutil.CLI) (diagnosticsObservation, error) {
	var obs diagnosticsObservation
	ns, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
	case err != nil:
		return obs, fmt.Errorf("failed to get namespace %s: %w", namespace, err)
	default:
		obs.namespace = ns
	}
	if obs.sourcePods, err = listDiagnosticsPods(ctx, oc, diagnosticsSourceSelector); err != nil {
		return obs, err
	}
	if obs.targetPods, err = listDiagnosticsPods(ctx, oc, diagnosticsTargetSelector); err != nil {
		return obs, err
	}
	return obs, nil
}

func observeDiagnostics(ctx context.Context, oc *exutil.CLI) (diagnosticsObservation, error) {
	obs, err := observeDiagnosticsWorkloads(ctx, oc)
	if err != nil {
		return obs, err
	}
	co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, networkClusterOperatorName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
	case err != nil:
		return obs, fmt.Errorf("failed to get %s: %w", networkClusterOperatorRef, err)
	default:
		obs.operator = co
	}
	return obs, nil
}

func listDiagnosticsPods(ctx context.Context, oc *exutil.CLI, selector string) ([]corev1.Pod, error) {
	pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods matching %s in namespace %s: %w", selector, namespace, err)
	}
	return pods.Items, nil
}

// ensureDiagnosticsReenabled re-applies the reset when AfterEach did not restore the config, so a failed
// restore does not leave diagnostics disabled for the specs that follow.
func ensureDiagnosticsReenabled(ctx context.Context, oc *exutil.CLI) {
	getCtx, cancelGet := context.WithTimeout(ctx, diagnosticsAPIRequestTimeout)
	defer cancelGet()
	cfg, err := oc.AdminConfigClient().ConfigV1().Networks().Get(getCtx, clusterConfig, metav1.GetOptions{})
	if err == nil && cfg.Spec.NetworkDiagnostics.Mode != configv1.NetworkDiagnosticsDisabled {
		return
	}
	if err != nil {
		framework.Logf("Failed to read network.config/%s, re-applying the network diagnostics reset: %v", clusterConfig, err)
	} else {
		framework.Logf("Network diagnostics are still disabled, re-applying the network diagnostics reset")
	}
	applyCtx, cancelApply := context.WithTimeout(ctx, diagnosticsAPIRequestTimeout)
	defer cancelApply()
	err = resetNetworkDiagnosticsConfig(applyCtx, oc)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to re-enable network diagnostics")
}

// waitForDiagnosticsRecovery waits until re-enabled diagnostics are running again and the network operator is
// healthy, so the specs that follow do not start while the old namespace is still being torn down. Every poll
// gets its own API deadline, bounded by the overall recovery timeout, so a hung request cannot stall the wait.
func waitForDiagnosticsRecovery(ctx context.Context, oc *exutil.CLI, snapshot diagnosticsSnapshot, podsRemoved bool) {
	recoveryCtx, cancel := context.WithTimeout(ctx, diagnosticsRecoveryTimeout)
	defer cancel()
	start := time.Now()
	lastSummary := "<none>"
	var recovered diagnosticsObservation
	o.Eventually(ctx, func() error {
		readCtx, cancelRead := context.WithTimeout(recoveryCtx, diagnosticsAPIRequestTimeout)
		defer cancelRead()
		obs, err := observeDiagnostics(readCtx, oc)
		if err != nil {
			return fmt.Errorf("%w; last observed state:\n%s", err, lastSummary)
		}
		summary, err := evaluateDiagnosticsRecovery(obs, snapshot, podsRemoved)
		if summary != lastSummary {
			framework.Logf("Network diagnostics state after re-enabling:\n%s", summary)
			lastSummary = summary
		}
		if err != nil {
			return fmt.Errorf("%w; observed state:\n%s", err, summary)
		}
		recovered = obs
		return nil
	}).WithTimeout(diagnosticsRecoveryTimeout).
		WithPolling(diagnosticsRecoveryPollInterval).
		MustPassRepeatedly(diagnosticsRecoveryConsecutivePasses).
		Should(o.Succeed(), "network diagnostics did not recover after re-enabling")

	framework.Logf("Network diagnostics recovered after %s (namespace replaced: %s, source pods: %d, target pods: %d)",
		time.Since(start).Round(time.Second), describeNamespaceReplacement(recovered.namespace, snapshot),
		len(recovered.sourcePods), len(recovered.targetPods))
}

// evaluateDiagnosticsRecovery returns the observed state and an error listing every unmet recovery criterion.
// Objects deleted while diagnostics were disabled keep their deletionTimestamp until they are gone, so they cannot
// pass the namespace and pod checks. The operator conditions are only trusted together with those checks because
// the network operator reports Degraded only after apply failures persist, so an early Degraded=False can predate
// the failure.
func evaluateDiagnosticsRecovery(obs diagnosticsObservation, snapshot diagnosticsSnapshot, podsRemoved bool) (string, error) {
	var state, problems []string

	if ns := obs.namespace; ns == nil {
		state = append(state, fmt.Sprintf("namespace %s: not found", namespace))
		problems = append(problems, fmt.Sprintf("namespace %s does not exist", namespace))
	} else {
		state = append(state, fmt.Sprintf("namespace %s: uid=%s phase=%s deletionTimestamp=%s replaced=%s",
			namespace, ns.UID, ns.Status.Phase, formatDiagnosticsTimestamp(ns.DeletionTimestamp),
			describeNamespaceReplacement(ns, snapshot)))
		if ns.Status.Phase != corev1.NamespaceActive {
			problems = append(problems, fmt.Sprintf("namespace %s is %s, not Active", namespace, ns.Status.Phase))
		}
		if ns.DeletionTimestamp != nil {
			problems = append(problems, fmt.Sprintf("namespace %s is terminating", namespace))
		}
	}

	for _, group := range obs.podGroups() {
		if len(group.pods) == 0 {
			state = append(state, fmt.Sprintf("pods %s: none", group.selector))
			problems = append(problems, fmt.Sprintf("no pods match %s", group.selector))
		}
		for i := range group.pods {
			pod := &group.pods[i]
			ready := podutils.IsPodReady(pod)
			terminating := pod.DeletionTimestamp != nil
			preDisable := snapshot.podUIDs.Has(pod.UID)
			state = append(state, fmt.Sprintf("pod %s (%s): uid=%s phase=%s ready=%t terminating=%t preDisable=%t node=%s",
				pod.Name, group.selector, pod.UID, pod.Status.Phase, ready, terminating, preDisable, pod.Spec.NodeName))
			if terminating {
				problems = append(problems, fmt.Sprintf("pod %s is terminating", pod.Name))
			}
			if !ready {
				problems = append(problems, fmt.Sprintf("pod %s is not Ready", pod.Name))
			}
			if podsRemoved && preDisable {
				problems = append(problems, fmt.Sprintf("pod %s existed before diagnostics were disabled", pod.Name))
			}
		}
	}

	if obs.operator == nil {
		state = append(state, fmt.Sprintf("%s: not found", networkClusterOperatorRef))
		problems = append(problems, fmt.Sprintf("%s does not exist", networkClusterOperatorRef))
	} else {
		conditions := obs.operator.Status.Conditions
		state = append(state, fmt.Sprintf("%s: %s; %s; %s", networkClusterOperatorRef,
			describeOperatorCondition(conditions, configv1.OperatorAvailable),
			describeOperatorCondition(conditions, configv1.OperatorDegraded),
			describeOperatorCondition(conditions, configv1.OperatorProgressing)))
		if !clusteroperatorhelpers.IsStatusConditionTrue(conditions, configv1.OperatorAvailable) {
			problems = append(problems, fmt.Sprintf("%s is not Available=True", networkClusterOperatorRef))
		}
		if !clusteroperatorhelpers.IsStatusConditionFalse(conditions, configv1.OperatorDegraded) {
			problems = append(problems, fmt.Sprintf("%s is not Degraded=False", networkClusterOperatorRef))
		}
	}

	summary := strings.Join(state, "\n")
	if len(problems) > 0 {
		return summary, errors.New(strings.Join(problems, "; "))
	}
	return summary, nil
}

func describeNamespaceReplacement(ns *corev1.Namespace, snapshot diagnosticsSnapshot) string {
	if snapshot.namespaceUID == "" {
		return "unknown (no namespace before disabling)"
	}
	replaced := ns != nil && ns.UID != snapshot.namespaceUID
	return fmt.Sprintf("%t (pre-disable uid=%s)", replaced, snapshot.namespaceUID)
}

func describeOperatorCondition(conditions []configv1.ClusterOperatorStatusCondition, conditionType configv1.ClusterStatusConditionType) string {
	c := clusteroperatorhelpers.FindStatusCondition(conditions, conditionType)
	if c == nil {
		return fmt.Sprintf("%s=<missing>", conditionType)
	}
	return fmt.Sprintf("%s=%s reason=%s message=%q", c.Type, c.Status, c.Reason, truncateDiagnosticsMessage(c.Message))
}

func formatDiagnosticsTimestamp(t *metav1.Time) string {
	if t == nil {
		return "<none>"
	}
	return t.UTC().Format(time.RFC3339)
}

func truncateDiagnosticsMessage(message string) string {
	runes := []rune(message)
	if len(runes) <= diagnosticsMaxMessageLength {
		return message
	}
	return string(runes[:diagnosticsMaxMessageLength]) + "..."
}
