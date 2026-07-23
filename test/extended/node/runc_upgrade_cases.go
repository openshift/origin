package node

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	v1helpers "github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	runcRHCOS10GuardPool = "runc-rhcos10-guard"
	streamRHEL9          = "rhel-9"
	streamRHEL10         = "rhel-10"

	runcGuardCRCName          = "99-runc-rhcos10-guard-runc"
	runcCRCDefaultRuntimePath = "/etc/crio/crio.conf.d/01-ctrcfg-defaultRuntime"

	machineConfigClusterOperator = "machine-config"

	degradedPoolUpgradeableReason  = "DegradedPool"
	degradedPoolUpgradeableMessage = "One or more machine config pools are degraded"
)

var rhelMajorOSImagePattern = regexp.MustCompile(`Linux\s+([0-9]+)`)

// When a pool uses runc and targets osImageStream rhel-10, MCO must block RHCOS 9→10
// rollout by setting MachineConfigPool Degraded / RenderDegraded. MCO then sets
// ClusterOperator Upgradeable=False (DegradedPool), which CVO aggregates on ClusterVersion.
var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Serial][Disruptive][OCPFeatureGate:OSStreams] runc RHCOS 10 upgrade guard", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLI("runc-rhcos10-guard")
		mcClient             *machineconfigclient.Clientset
		nodeName             string
		clusterDefaultStream string
	)

	g.BeforeEach(func(ctx context.Context) {
		var err error
		mcClient, err = machineconfigclient.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			// MicroShift defaults to crun and does not support configuring runc via ContainerRuntimeConfig.
			g.Skip("Skipping on MicroShift cluster: runc cannot be configured")
		}

		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("Skipping on external control plane (Hypershift) cluster")
		}
		if *controlPlaneTopology == configv1.SingleReplicaTopologyMode {
			g.Skip("Skipping on single-replica topology: requires a pure worker node")
		}

		clusterDefaultStream = requireOSImageStreams(ctx, mcClient)
	})

	g.It("blocks RHCOS 9 to 10 osImageStream upgrade when ContainerRuntimeConfig sets runc default runtime", ote.Informing(), func(ctx context.Context) {
		g.By("Creating custom MachineConfigPool pinned to rhel-9 with runc ContainerRuntimeConfig")
		o.Expect(createRuncGuardPool(ctx, mcClient)).To(o.Succeed())

		g.By("Labeling one worker into the custom pool")
		var err error
		nodeName, err = labelFirstPureWorker(ctx, oc, runcRHCOS10GuardPool)
		o.Expect(err).NotTo(o.HaveOccurred(), "need a worker node for the custom pool")

		g.By("Waiting for pool rollout on rhel-9 with runc")
		o.Expect(waitForMCPWithLabeledNode(ctx, oc, mcClient, runcRHCOS10GuardPool, nodeName, 30*time.Minute)).To(o.Succeed(),
			"node did not join custom MCP")

		g.By("Checking default runtime is runc on RHCOS 9")
		o.Expect(expectRuncRuntimeOnNode(ctx, oc, nodeName, 2*time.Minute)).To(o.Succeed())
		rhelMajor, err := nodeRHELMajorVersion(ctx, oc, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rhelMajor).To(o.Equal("9"), "pool should be on RHCOS 9 before attempting rhel-10 stream")

		g.By("Upgrading RHCOS version to RHCOS 10 via osImageStream")
		o.Expect(setPoolOSImageStream(ctx, mcClient, runcRHCOS10GuardPool, streamRHEL10)).To(o.Succeed())
		o.Expect(waitForMCPRenderDegraded(ctx, mcClient, runcRHCOS10GuardPool, 10*time.Minute)).To(o.Succeed())

		g.By("Verifying cluster upgrade is blocked via CO and CVO Upgradeable=False")
		o.Expect(waitForUpgradeBlockedByDegradedPool(ctx, oc)).To(o.Succeed())

		g.By("Verifying node remains ready, not rolling out, on RHCOS 9 with runc after guard blocks rollout")
		o.Expect(verifyNodeReadyAndNotRollingOut(ctx, oc, nodeName)).To(o.Succeed())
		rhelMajor, err = nodeRHELMajorVersion(ctx, oc, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rhelMajor).To(o.Equal("9"), "node should remain on RHCOS 9 after guard blocks rollout")
		o.Expect(expectRuncRuntimeOnNode(ctx, oc, nodeName, 2*time.Minute)).To(o.Succeed(),
			"node should keep runc as default runtime after guard blocks rollout")

		g.By("Recovering pool by setting osImageStream back to rhel-9")
		o.Expect(setPoolOSImageStream(ctx, mcClient, runcRHCOS10GuardPool, streamRHEL9)).To(o.Succeed())
		o.Expect(WaitForMCP(ctx, mcClient, runcRHCOS10GuardPool, 10*time.Minute, WaitMCPAllowDegraded())).To(o.Succeed())

		g.By("Verifying cluster upgradeability recovers after pool returns to rhel-9")
		// MCO may take up to ~30 minutes to propagate Upgradeable after RenderDegraded clears.
		o.Expect(waitForClusterUpgradeable(ctx, oc, 30*time.Minute)).To(o.Succeed())

		g.By("Verifying node remains ready, not rolling out, on RHCOS 9 with runc after recovery")
		o.Expect(verifyNodeReadyAndNotRollingOut(ctx, oc, nodeName)).To(o.Succeed())
		rhelMajor, err = nodeRHELMajorVersion(ctx, oc, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rhelMajor).To(o.Equal("9"), "node should remain on RHCOS 9 after recovery")
		o.Expect(expectRuncRuntimeOnNode(ctx, oc, nodeName, 2*time.Minute)).To(o.Succeed(),
			"node should keep runc as default runtime after recovery")

		if clusterDefaultStream == streamRHEL10 {
			g.By("Recovering pool to cluster default RHCOS 10 with crun after removing runc config")
			node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			priorRenderedConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
			o.Expect(priorRenderedConfig).NotTo(o.BeEmpty(), "node should have a stable rendered config before CRC removal")

			o.Expect(deleteContainerRuntimeConfig(ctx, mcClient, runcGuardCRCName)).To(o.Succeed())
			o.Expect(waitForPoolConfigRolloutAfterCRCRemoval(ctx, oc, mcClient, runcRHCOS10GuardPool, nodeName, priorRenderedConfig, 30*time.Minute)).To(o.Succeed(),
				"pool should re-render and roll out on the node without runc ContainerRuntimeConfig before moving to rhel-10")
			o.Expect(waitForRuncRemovedFromNode(ctx, oc, nodeName, 15*time.Minute)).To(o.Succeed(),
				"CRC removal should drop the runc CRI-O drop-in before moving to rhel-10")

			g.By("Moving pool to cluster default RHCOS 10 stream")
			o.Expect(setPoolOSImageStream(ctx, mcClient, runcRHCOS10GuardPool, streamRHEL10)).To(o.Succeed())
			o.Expect(waitForNodeRHELMajorVersion(ctx, oc, nodeName, "10", 45*time.Minute)).To(o.Succeed(),
				"node should reboot onto RHCOS 10 after rhel-10 stream change without runc")
			o.Expect(WaitForMCP(ctx, mcClient, runcRHCOS10GuardPool, 30*time.Minute)).To(o.Succeed())

			g.By("Verifying node rolled out to RHCOS 10 with crun")
			o.Expect(verifyNodeReadyAndNotRollingOut(ctx, oc, nodeName)).To(o.Succeed())
			o.Expect(assertCrunRuntimeOnNode(ctx, oc, nodeName)).To(o.Succeed(), "node should use crun after runc config is removed")
		}
	})

	g.AfterEach(func(ctx context.Context) {
		// Do not use Expect here; a failed assertion would skip subsequent cleanup steps.
		if nodeName != "" {
			roleLabel := poolNodeRoleLabel(runcRHCOS10GuardPool)
			if err := removeNodeLabel(ctx, oc, nodeName, roleLabel); err != nil {
				framework.Logf("cleanup: failed to remove node label %s from %s: %v", roleLabel, nodeName, err)
			}
		}
		if err := deleteContainerRuntimeConfig(ctx, mcClient, runcGuardCRCName); err != nil {
			framework.Logf("cleanup: failed to delete ContainerRuntimeConfig %s: %v", runcGuardCRCName, err)
		}
		if nodeName != "" {
			if err := WaitForMCP(ctx, mcClient, runcRHCOS10GuardPool, 10*time.Minute, WaitMCPWithMachineCount(0)); err != nil {
				framework.Logf("cleanup: failed waiting for MCP %s machine count 0: %v", runcRHCOS10GuardPool, err)
			}
			if err := waitForNodeWorkerConfigRollback(ctx, oc, nodeName, runcRHCOS10GuardPool, 15*time.Minute); err != nil {
				framework.Logf("cleanup: failed waiting for node %s worker config rollback: %v", nodeName, err)
			}
		}
		if err := deleteMachineConfigPool(ctx, mcClient, runcRHCOS10GuardPool); err != nil {
			framework.Logf("cleanup: failed to delete MachineConfigPool %s: %v", runcRHCOS10GuardPool, err)
		}
		if nodeName != "" {
			if err := WaitForMCP(ctx, mcClient, "worker", 30*time.Minute); err != nil {
				framework.Logf("cleanup: failed waiting for worker MCP to become ready: %v", err)
			}
		}
	})
})

// waitForUpgradeBlockedByDegradedPool waits for MCO to propagate an isolated MCP render failure
// to ClusterOperator and ClusterVersion Upgradeable=False. CO/CVO Degraded may take ~30 minutes
// to flip; this check mirrors MCO extended tests that assert Upgradeable without waiting for Degraded.
func waitForUpgradeBlockedByDegradedPool(ctx context.Context, oc *exutil.CLI) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, machineConfigClusterOperator, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if degraded := findClusterConditionStatus(co.Status.Conditions, configv1.OperatorDegraded); degraded == configv1.ConditionTrue {
			return false, fmt.Errorf("ClusterOperator %s Degraded=True; expected Upgradeable=False only while isolated pool guard is active", machineConfigClusterOperator)
		}
		if available := findClusterConditionStatus(co.Status.Conditions, configv1.OperatorAvailable); available != configv1.ConditionTrue {
			return false, fmt.Errorf("ClusterOperator %s Available=%s, expected True", machineConfigClusterOperator, available)
		}

		upgradeable := v1helpers.FindStatusCondition(co.Status.Conditions, configv1.OperatorUpgradeable)
		if upgradeable == nil || upgradeable.Status != configv1.ConditionFalse {
			status := configv1.ConditionUnknown
			if upgradeable != nil {
				status = upgradeable.Status
			}
			framework.Logf("waiting for ClusterOperator %s Upgradeable=False, current status=%s", machineConfigClusterOperator, status)
			return false, nil
		}
		if upgradeable.Reason != degradedPoolUpgradeableReason {
			framework.Logf("waiting for ClusterOperator %s Upgradeable reason=%s, current reason=%q", machineConfigClusterOperator, degradedPoolUpgradeableReason, upgradeable.Reason)
			return false, nil
		}
		if !strings.Contains(upgradeable.Message, degradedPoolUpgradeableMessage) {
			framework.Logf("waiting for ClusterOperator %s Upgradeable message to contain %q, current message=%q", machineConfigClusterOperator, degradedPoolUpgradeableMessage, upgradeable.Message)
			return false, nil
		}

		cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if available := findClusterConditionStatus(cv.Status.Conditions, configv1.OperatorAvailable); available != configv1.ConditionTrue {
			return false, fmt.Errorf("ClusterVersion Available=%s, expected True", available)
		}
		if progressing := findClusterConditionStatus(cv.Status.Conditions, configv1.OperatorProgressing); progressing == configv1.ConditionTrue {
			return false, fmt.Errorf("ClusterVersion Progressing=True while isolated pool guard is active")
		}
		if degraded := findClusterConditionStatus(cv.Status.Conditions, configv1.OperatorDegraded); degraded == configv1.ConditionTrue {
			return false, fmt.Errorf("ClusterVersion Degraded=True while isolated pool guard is active")
		}

		cvUpgradeable := v1helpers.FindStatusCondition(cv.Status.Conditions, configv1.OperatorUpgradeable)
		if cvUpgradeable == nil || cvUpgradeable.Status != configv1.ConditionFalse {
			status := configv1.ConditionUnknown
			if cvUpgradeable != nil {
				status = cvUpgradeable.Status
			}
			framework.Logf("waiting for ClusterVersion Upgradeable=False, current status=%s", status)
			return false, nil
		}

		// TechPreview/CustomNoUpgrade clusters, and clusters with ClusterVersion overrides (e.g.
		// from mco-push replace), may already report Upgradeable=False without listing
		// machine-config in the CVO message. MCO extended tests assert only
		// co/machine-config Upgradeable=False (DegradedPool).
		if clusterVersionUpgradeableGloballyBlocked(oc, cv, cvUpgradeable) {
			framework.Logf("ClusterOperator %s reports Upgradeable=False (reason %s); ClusterVersion Upgradeable=False without machine-config in message (feature set, CV overrides, or stale override reason present)",
				machineConfigClusterOperator, degradedPoolUpgradeableReason)
			return true, nil
		}
		if !strings.Contains(cvUpgradeable.Message, machineConfigClusterOperator) {
			framework.Logf("waiting for ClusterVersion Upgradeable message to mention %s, current message=%q", machineConfigClusterOperator, cvUpgradeable.Message)
			return false, nil
		}

		framework.Logf("ClusterOperator %s and ClusterVersion %q report Upgradeable=False (reason %s) with isolated MCP guard active",
			machineConfigClusterOperator, cv.Status.Desired.Version, degradedPoolUpgradeableReason)
		return true, nil
	})
}

// clusterVersionUpgradeableGloballyBlocked reports whether ClusterVersion Upgradeable=False
// is expected for reasons unrelated to an isolated MCP render guard (feature set, CV overrides).
func clusterVersionUpgradeableGloballyBlocked(oc *exutil.CLI, cv *configv1.ClusterVersion, cvUpgradeable *configv1.ClusterOperatorStatusCondition) bool {
	if exutil.IsNoUpgradeFeatureSet(oc) || len(cv.Spec.Overrides) > 0 {
		return true
	}
	if cvUpgradeable == nil {
		return false
	}
	return strings.Contains(cvUpgradeable.Message, "cluster version overrides")
}

// waitForClusterUpgradeable waits until machine-config Upgradeable=True after an isolated MCP
// guard is cleared. ClusterVersion Upgradeable=True is required only when the cluster is not
// already globally non-upgradeable (mirrors waitForUpgradeBlockedByDegradedPool).
func waitForClusterUpgradeable(ctx context.Context, oc *exutil.CLI, timeout time.Duration) error {
	var lastCOReason, lastCOMessage, lastCVReason, lastCVMessage string

	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, machineConfigClusterOperator, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		coUpgradeable := v1helpers.FindStatusCondition(co.Status.Conditions, configv1.OperatorUpgradeable)
		if coUpgradeable == nil || coUpgradeable.Status != configv1.ConditionTrue {
			status := configv1.ConditionUnknown
			if coUpgradeable != nil {
				status = coUpgradeable.Status
				lastCOReason = coUpgradeable.Reason
				lastCOMessage = coUpgradeable.Message
			}
			framework.Logf("waiting for ClusterOperator %s Upgradeable=True, current status=%s reason=%q message=%q",
				machineConfigClusterOperator, status, lastCOReason, lastCOMessage)
			return false, nil
		}

		cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		cvUpgradeable := v1helpers.FindStatusCondition(cv.Status.Conditions, configv1.OperatorUpgradeable)
		if cvUpgradeable != nil {
			lastCVReason = cvUpgradeable.Reason
			lastCVMessage = cvUpgradeable.Message
		}

		if clusterVersionUpgradeableGloballyBlocked(oc, cv, cvUpgradeable) {
			framework.Logf("ClusterOperator %s Upgradeable=True after pool recovery; ClusterVersion Upgradeable not required (feature set, CV overrides, or stale override reason present)",
				machineConfigClusterOperator)
			return true, nil
		}

		if cvUpgradeable != nil && cvUpgradeable.Status == configv1.ConditionTrue {
			framework.Logf("ClusterOperator %s and ClusterVersion %q report Upgradeable=True after pool recovery",
				machineConfigClusterOperator, cv.Status.Desired.Version)
			return true, nil
		}

		if cvUpgradeable != nil && cvUpgradeable.Status == configv1.ConditionFalse &&
			strings.Contains(cvUpgradeable.Message, machineConfigClusterOperator) {
			framework.Logf("waiting for ClusterVersion Upgradeable to recover from machine-config block, current reason=%q message=%q",
				cvUpgradeable.Reason, cvUpgradeable.Message)
			return false, nil
		}

		status := configv1.ConditionUnknown
		if cvUpgradeable != nil {
			status = cvUpgradeable.Status
		}
		framework.Logf("ClusterOperator %s Upgradeable=True after pool recovery; ClusterVersion Upgradeable=%s reason=%q (not blocked by machine-config)",
			machineConfigClusterOperator, status, lastCVReason)
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("cluster upgradeability did not recover within %s (co/machine-config reason=%q message=%q; clusterversion reason=%q message=%q): %w",
			timeout, lastCOReason, lastCOMessage, lastCVReason, lastCVMessage, err)
	}
	return nil
}

// waitForNodeWorkerConfigRollback waits until the node is fully back on the worker pool rendered
// config. This must complete before deleting the custom MCP; otherwise the node's currentConfig
// can reference a rendered MC that no longer exists.
func waitForNodeWorkerConfigRollback(ctx context.Context, oc *exutil.CLI, nodeName, poolName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			framework.Logf("Node %s no longer exists; skipping worker config rollback wait", nodeName)
			return true, nil
		}
		if err != nil {
			return false, err
		}
		currentConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
		desiredConfig := node.Annotations["machineconfiguration.openshift.io/desiredConfig"]
		rolledBack := currentConfig != "" &&
			!strings.Contains(currentConfig, poolName) &&
			currentConfig == desiredConfig
		if !rolledBack {
			framework.Logf("Node %s waiting for worker rollback: current=%q desired=%q",
				nodeName, currentConfig, desiredConfig)
		}
		return rolledBack, nil
	})
}

func verifyNodeReadyAndNotRollingOut(ctx context.Context, oc *exutil.CLI, nodeName string) error {
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	ready := false
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}
	if !ready {
		return fmt.Errorf("node %s is not Ready", nodeName)
	}

	currentConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
	desiredConfig := node.Annotations["machineconfiguration.openshift.io/desiredConfig"]
	if currentConfig == "" || desiredConfig == "" {
		return fmt.Errorf("node %s missing MCO config annotations (current=%q desired=%q)", nodeName, currentConfig, desiredConfig)
	}
	if currentConfig != desiredConfig {
		return fmt.Errorf("node %s is rolling out MCO config (current=%q desired=%q)", nodeName, currentConfig, desiredConfig)
	}

	framework.Logf("Node %s is Ready and not rolling out MCO config (%s)", nodeName, currentConfig)
	return nil
}

func findClusterConditionStatus(conditions []configv1.ClusterOperatorStatusCondition, condType configv1.ClusterStatusConditionType) configv1.ConditionStatus {
	if c := v1helpers.FindStatusCondition(conditions, condType); c != nil {
		return c.Status
	}
	return configv1.ConditionUnknown
}

func requireOSImageStreams(ctx context.Context, mcClient *machineconfigclient.Clientset) string {
	osi, err := mcClient.MachineconfigurationV1().OSImageStreams().Get(ctx, "cluster", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip("OSImageStream API is not available; enable OSStreams feature gate on the cluster")
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	streamNames := make([]string, 0, len(osi.Status.AvailableStreams))
	for _, s := range osi.Status.AvailableStreams {
		streamNames = append(streamNames, s.Name)
	}
	o.Expect(streamNames).To(o.ContainElements(streamRHEL9, streamRHEL10),
		"dual stream (rhel-9 and rhel-10) must be available")
	framework.Logf("OSImageStream default=%q streams=%v", osi.Status.DefaultStream, streamNames)
	return osi.Status.DefaultStream
}

func poolNodeRoleLabel(poolName string) string {
	return fmt.Sprintf("node-role.kubernetes.io/%s", poolName)
}

func poolOperatorLabel(poolName string) string {
	return fmt.Sprintf("pools.operator.machineconfiguration.openshift.io/%s", poolName)
}

func createRuncGuardPool(ctx context.Context, mcClient *machineconfigclient.Clientset) error {
	if err := createRuncGuardMCP(ctx, mcClient); err != nil {
		return err
	}
	return createRuncGuardCRC(ctx, mcClient)
}

func createRuncGuardMCP(ctx context.Context, mcClient *machineconfigclient.Clientset) error {
	mcp := &machineconfigv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: runcRHCOS10GuardPool,
			Labels: map[string]string{
				poolOperatorLabel(runcRHCOS10GuardPool): "",
			},
		},
		Spec: machineconfigv1.MachineConfigPoolSpec{
			OSImageStream: machineconfigv1.OSImageStreamReference{Name: streamRHEL9},
			MachineConfigSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      machineconfigv1.MachineConfigRoleLabelKey,
					Operator: metav1.LabelSelectorOpIn,
					// worker is required by custom-machine-config-pool-selector VAP for custom pools.
					Values: []string{"worker", runcRHCOS10GuardPool},
				}},
			},
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					poolNodeRoleLabel(runcRHCOS10GuardPool): "",
				},
			},
		},
	}
	_, err := mcClient.MachineconfigurationV1().MachineConfigPools().Create(ctx, mcp, metav1.CreateOptions{})
	return err
}

func createRuncGuardCRC(ctx context.Context, mcClient *machineconfigclient.Clientset) error {
	crc := &machineconfigv1.ContainerRuntimeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: runcGuardCRCName,
		},
		Spec: machineconfigv1.ContainerRuntimeConfigSpec{
			MachineConfigPoolSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					poolOperatorLabel(runcRHCOS10GuardPool): "",
				},
			},
			ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
				DefaultRuntime: machineconfigv1.ContainerRuntimeDefaultRuntimeRunc,
			},
		},
	}
	_, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, crc, metav1.CreateOptions{})
	return err
}

// waitForMCPWithLabeledNode waits for a pool to reach 1 ready machine while verifying the labeled
// test node still exists. Fails fast if the node is replaced/deleted instead of polling until timeout.
func waitForMCPWithLabeledNode(ctx context.Context, oc *exutil.CLI, mcClient *machineconfigclient.Clientset, poolName, nodeName string, timeout time.Duration) error {
	framework.Logf("Waiting for MCP %s to adopt labeled node %s (timeout: %v)...", poolName, nodeName, timeout)

	var lastReady, lastTotal int32
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, fmt.Errorf("node %s was removed from the cluster during MCP rollout (likely MachineSet replacement)", nodeName)
		}
		if err != nil {
			return false, err
		}

		roleLabel := poolNodeRoleLabel(poolName)
		if _, ok := node.Labels[roleLabel]; !ok {
			return false, fmt.Errorf("node %s lost label %q required by pool %s", nodeName, roleLabel, poolName)
		}

		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		lastReady = mcp.Status.ReadyMachineCount
		lastTotal = mcp.Status.MachineCount

		updated, degraded, renderDegraded, updating := mcpPoolConditions(mcp)
		if degraded {
			return false, fmt.Errorf("MachineConfigPool %s is degraded while waiting for node %s", poolName, nodeName)
		}
		if renderDegraded {
			return false, fmt.Errorf("MachineConfigPool %s render is degraded while waiting for node %s", poolName, nodeName)
		}

		isReady := !updating && updated &&
			mcp.Status.MachineCount == 1 &&
			mcp.Status.ReadyMachineCount == mcp.Status.MachineCount
		if isReady {
			framework.Logf("MachineConfigPool %s is ready with node %s: %d/%d machines ready",
				poolName, nodeName, lastReady, lastTotal)
			return true, nil
		}

		framework.Logf("MachineConfigPool %s not ready yet for node %s: updating=%v degraded=%v renderDegraded=%v updated=%v machines=%d/%d",
			poolName, nodeName, updating, degraded, renderDegraded, updated, lastReady, lastTotal)
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("pool %s did not reach 1 ready machine with node %s (last machines=%d/%d): %w",
			poolName, nodeName, lastReady, lastTotal, err)
	}
	return nil
}

func mcpPoolConditions(mcp *machineconfigv1.MachineConfigPool) (updated, degraded, renderDegraded, updating bool) {
	for _, condition := range mcp.Status.Conditions {
		switch condition.Type {
		case machineconfigv1.MachineConfigPoolUpdating:
			updating = condition.Status == corev1.ConditionTrue
		case machineconfigv1.MachineConfigPoolDegraded:
			degraded = condition.Status == corev1.ConditionTrue
		case machineconfigv1.MachineConfigPoolRenderDegraded:
			renderDegraded = condition.Status == corev1.ConditionTrue
		case machineconfigv1.MachineConfigPoolUpdated:
			updated = condition.Status == corev1.ConditionTrue
		}
	}
	return updated, degraded, renderDegraded, updating
}

func labelFirstPureWorker(ctx context.Context, oc *exutil.CLI, poolName string) (string, error) {
	workers, err := getPureWorkerNodesFromCluster(ctx, oc)
	if err != nil {
		return "", err
	}
	if len(workers) == 0 {
		return "", fmt.Errorf("no pure worker nodes without custom roles available")
	}

	node := workers[0]
	label := poolNodeRoleLabel(poolName)
	patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:""}}}`, label))
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, patchErr := oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, node.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		return patchErr
	})
	if err != nil {
		return "", err
	}
	framework.Logf("Labeled node %s with %s", node.Name, label)
	return node.Name, nil
}

func removeNodeLabel(ctx context.Context, oc *exutil.CLI, nodeName, label string) error {
	patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, label))
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, patchErr := oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patchData, metav1.PatchOptions{})
		if apierrors.IsNotFound(patchErr) {
			return nil
		}
		return patchErr
	})
	return err
}

func setPoolOSImageStream(ctx context.Context, mcClient *machineconfigclient.Clientset, poolName, stream string) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		mcp.Spec.OSImageStream = machineconfigv1.OSImageStreamReference{Name: stream}
		_, err = mcClient.MachineconfigurationV1().MachineConfigPools().Update(ctx, mcp, metav1.UpdateOptions{})
		return err
	})
}

func waitForMCPRenderDegraded(ctx context.Context, mcClient *machineconfigclient.Clientset, poolName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		renderDegraded := false
		var renderMessage string
		for _, c := range mcp.Status.Conditions {
			if c.Type == machineconfigv1.MachineConfigPoolRenderDegraded && c.Status == corev1.ConditionTrue {
				renderDegraded = true
				renderMessage = c.Message
			}
		}

		// The runc guard is enforced in the render controller; RenderDegraded is the
		// authoritative signal. Degraded may propagate later via the node controller.
		if renderDegraded &&
			strings.Contains(renderMessage, "runc") &&
			strings.Contains(renderMessage, streamRHEL10) {
			framework.Logf("MCP %s render degraded as expected: %s", poolName, renderMessage)
			return true, nil
		}

		framework.Logf("MCP %s waiting for runc+rhel-10 guard: renderDegraded=%v message=%q",
			poolName, renderDegraded, renderMessage)
		return false, nil
	})
}

func hasRuncRuntimeOnNode(ctx context.Context, oc *exutil.CLI, nodeName string) (bool, error) {
	// Use a shell guard so missing drop-ins do not make oc debug exit non-zero (which spams test logs).
	readDropIn := fmt.Sprintf("if [ -f %q ]; then grep default_runtime %q; fi", runcCRCDefaultRuntimePath, runcCRCDefaultRuntimePath)
	out, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "sh", "-c", readDropIn)
	if err != nil {
		if isTransientNodeDebugError(err) {
			return false, err
		}
		return false, fmt.Errorf("failed to check runc runtime on node %s: %w", nodeName, err)
	}
	return strings.Contains(out, "runc"), nil
}

func expectRuncRuntimeOnNode(ctx context.Context, oc *exutil.CLI, nodeName string, timeout time.Duration) error {
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		hasRunc, err := hasRuncRuntimeOnNode(ctx, oc, nodeName)
		if err != nil {
			if isTransientNodeDebugError(err) {
				framework.Logf("Transient debug error checking runc on node %s: %v", nodeName, err)
				return false, nil
			}
			return false, err
		}
		if !hasRunc {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if wait.Interrupted(err) {
			return fmt.Errorf("node %s does not have runc as default runtime within %s", nodeName, timeout)
		}
		return err
	}
	return nil
}

func isTransientNodeDebugError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unable to create the debug pod") ||
		strings.Contains(msg, "unable to attach to the debug pod") ||
		strings.Contains(msg, "the node is currently unavailable") ||
		strings.Contains(msg, "node is not ready")
}

// waitForPoolConfigRolloutAfterCRCRemoval waits for MCO to re-render the pool without the CRC and
// roll the new config out to the labeled node. MCP can report ready with the old rendered config
// briefly after CRC deletion; node annotations are the authoritative rollout signal.
func waitForPoolConfigRolloutAfterCRCRemoval(ctx context.Context, oc *exutil.CLI, mcClient *machineconfigclient.Clientset, poolName, nodeName, priorRenderedConfig string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, fmt.Errorf("node %s was removed from the cluster during pool re-render after CRC removal", nodeName)
		}
		if err != nil {
			return false, err
		}

		currentConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
		desiredConfig := node.Annotations["machineconfiguration.openshift.io/desiredConfig"]

		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		updated, degraded, renderDegraded, updating := mcpPoolConditions(mcp)
		if degraded {
			return false, fmt.Errorf("MachineConfigPool %s is degraded during post-CRC rollout", poolName)
		}
		if renderDegraded {
			return false, fmt.Errorf("MachineConfigPool %s render is degraded during post-CRC rollout", poolName)
		}

		poolReady := !updating && updated &&
			mcp.Status.MachineCount == 1 &&
			mcp.Status.ReadyMachineCount == mcp.Status.MachineCount
		rerendered := desiredConfig != "" && desiredConfig != priorRenderedConfig
		synced := currentConfig != "" && currentConfig == desiredConfig

		if poolReady && rerendered && synced {
			framework.Logf("Node %s rolled out post-CRC rendered config %s (was %s)", nodeName, currentConfig, priorRenderedConfig)
			return true, nil
		}

		framework.Logf("Node %s waiting for post-CRC pool rollout: current=%q desired=%q prior=%q poolReady=%v updating=%v",
			nodeName, currentConfig, desiredConfig, priorRenderedConfig, poolReady, updating)
		return false, nil
	})
}

func waitForRuncRemovedFromNode(ctx context.Context, oc *exutil.CLI, nodeName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, fmt.Errorf("node %s was removed from the cluster while waiting for runc removal", nodeName)
		}
		if err != nil {
			return false, err
		}
		if node.Spec.Unschedulable {
			framework.Logf("Node %s is unschedulable while waiting for runc removal", nodeName)
		}

		hasRunc, err := hasRuncRuntimeOnNode(ctx, oc, nodeName)
		if err != nil {
			if isTransientNodeDebugError(err) {
				framework.Logf("Node %s debug unavailable during runc removal rollout, retrying: %v", nodeName, err)
				return false, nil
			}
			return false, err
		}
		if !hasRunc {
			framework.Logf("Node %s no longer has runc default runtime configured", nodeName)
			return true, nil
		}
		framework.Logf("Node %s still has runc default runtime configured", nodeName)
		return false, nil
	})
}

func waitForNodeRHELMajorVersion(ctx context.Context, oc *exutil.CLI, nodeName, expectedMajor string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 15*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		ready := false
		for _, c := range node.Status.Conditions {
			if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}
		if !ready {
			framework.Logf("Node %s waiting for Ready while rolling out RHCOS %s (current OSImage=%q)",
				nodeName, expectedMajor, node.Status.NodeInfo.OSImage)
			return false, nil
		}

		major, err := rhelMajorFromOSImage(node.Status.NodeInfo.OSImage)
		if err != nil {
			framework.Logf("Node %s waiting to parse RHCOS major version: %v (OSImage=%q)",
				nodeName, err, node.Status.NodeInfo.OSImage)
			return false, nil
		}
		if major != expectedMajor {
			framework.Logf("Node %s waiting for RHCOS %s, currently RHCOS %s (OSImage=%q)",
				nodeName, expectedMajor, major, node.Status.NodeInfo.OSImage)
			return false, nil
		}

		currentConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
		desiredConfig := node.Annotations["machineconfiguration.openshift.io/desiredConfig"]
		if currentConfig == "" || desiredConfig == "" || currentConfig != desiredConfig {
			framework.Logf("Node %s on RHCOS %s but still rolling out MCO config (current=%q desired=%q)",
				nodeName, major, currentConfig, desiredConfig)
			return false, nil
		}

		framework.Logf("Node %s is Ready on RHCOS %s with stable MCO config %s", nodeName, major, currentConfig)
		return true, nil
	})
}

func assertCrunRuntimeOnNode(ctx context.Context, oc *exutil.CLI, nodeName string) error {
	hasRunc, err := hasRuncRuntimeOnNode(ctx, oc, nodeName)
	if err != nil {
		if isTransientNodeDebugError(err) {
			return fmt.Errorf("failed to verify runtime on node %s: debug unavailable: %w", nodeName, err)
		}
		return fmt.Errorf("failed to verify runtime on node %s: %w", nodeName, err)
	}
	if hasRunc {
		return fmt.Errorf("node %s still has runc default runtime configured on RHCOS 10", nodeName)
	}
	// RHCOS 10 defaults to crun; absence of the CRC runc drop-in is sufficient.
	return nil
}

func nodeRHELMajorVersion(ctx context.Context, oc *exutil.CLI, nodeName string) (string, error) {
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	major, err := rhelMajorFromOSImage(node.Status.NodeInfo.OSImage)
	if err != nil {
		return "", fmt.Errorf("%w on node %s", err, nodeName)
	}
	return major, nil
}

func rhelMajorFromOSImage(osImage string) (string, error) {
	switch {
	case strings.Contains(osImage, "CoreOS 10."):
		return "10", nil
	case strings.Contains(osImage, "CoreOS 9."):
		return "9", nil
	}

	if matches := rhelMajorOSImagePattern.FindStringSubmatch(osImage); len(matches) >= 2 {
		return matches[1], nil
	}
	return "", fmt.Errorf("could not parse RHEL major version from OSImage %q", osImage)
}

func deleteContainerRuntimeConfig(ctx context.Context, mcClient *machineconfigclient.Clientset, name string) error {
	err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func deleteMachineConfigPool(ctx context.Context, mcClient *machineconfigclient.Clientset, name string) error {
	err := mcClient.MachineconfigurationV1().MachineConfigPools().Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
