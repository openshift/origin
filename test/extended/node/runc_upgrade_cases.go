package node

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
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

// When a pool uses runc and targets osImageStream rhel-10, MCO must block RHCOS 9→10
// rollout by setting MachineConfigPool Degraded / RenderDegraded. MCO then sets
// ClusterOperator Upgradeable=False (DegradedPool), which CVO aggregates on ClusterVersion.
var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Serial][Disruptive] runc RHCOS 10 upgrade guard", func() {
	defer g.GinkgoRecover()

	var (
		oc       = exutil.NewCLI("runc-rhcos10-guard")
		mcClient *machineconfigclient.Clientset
		nodeName string
	)

	g.BeforeEach(func(ctx context.Context) {
		var err error
		mcClient, err = machineconfigclient.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping on MicroShift cluster")
		}

		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("Skipping on external control plane (Hypershift) cluster")
		}
		if *controlPlaneTopology == configv1.SingleReplicaTopologyMode {
			g.Skip("Skipping on single-replica topology: requires a pure worker node")
		}

		requireOSImageStreams(ctx, mcClient)
	})

	g.It("blocks upgrade of RHCOS 9 to 10 when ContainerRuntimeConfig sets default runtime to runc", func(ctx context.Context) {
		g.By("Labeling one worker into the custom pool")
		var err error
		nodeName, err = labelRandomPureWorker(ctx, oc, runcRHCOS10GuardPool)
		o.Expect(err).NotTo(o.HaveOccurred(), "need a worker node for the custom pool")

		g.By("Creating custom MachineConfigPool pinned to rhel-9")
		o.Expect(createRuncGuardMCP(ctx, mcClient)).To(o.Succeed())

		g.By("Creating ContainerRuntimeConfig that sets default runtime to runc for the custom pool")
		o.Expect(createRuncGuardCRC(ctx, mcClient)).To(o.Succeed())

		g.By("Waiting for node to join the custom pool")
		o.Expect(waitForMCPMachineCount(ctx, mcClient, runcRHCOS10GuardPool, 1, 10*time.Minute)).To(o.Succeed(),
			"node did not join custom MCP; ensure deployed MCO supports OSImageStream v1 API (rebase PR 5891 onto current main)")

		g.By("Waiting for pool rollout on rhel-9 with runc")
		o.Expect(waitForMCP(ctx, mcClient, runcRHCOS10GuardPool, 30*time.Minute)).To(o.Succeed())

		g.By("Checking default runtime is runc on RHCOS 9")
		o.Expect(nodeUsesRuncRuntime(oc, nodeName)).To(o.BeTrue())
		rhelMajor, err := nodeRHELMajorVersion(oc, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rhelMajor).To(o.Equal("9"), "pool should be on RHCOS 9 before attempting rhel-10 stream")

		g.By("Upgrading RHCOS version to RHCOS 10 via osImageStream")
		o.Expect(setPoolOSImageStream(ctx, mcClient, runcRHCOS10GuardPool, streamRHEL10)).To(o.Succeed())
		o.Expect(waitForMCPRenderDegraded(ctx, mcClient, runcRHCOS10GuardPool, 10*time.Minute)).To(o.Succeed())

		g.By("Verifying cluster upgrade is blocked via CO and CVO Upgradeable=False")
		o.Expect(waitForUpgradeBlockedByDegradedPool(ctx, oc)).To(o.Succeed())

		g.By("Verifying node remains ready, not rolling out, on RHCOS 9 with runc after guard blocks rollout")
		o.Expect(verifyNodeReadyAndNotRollingOut(ctx, oc, nodeName)).To(o.Succeed())
		rhelMajor, err = nodeRHELMajorVersion(oc, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rhelMajor).To(o.Equal("9"), "node should remain on RHCOS 9 after guard blocks rollout")
		o.Expect(nodeUsesRuncRuntime(oc, nodeName)).To(o.BeTrue(), "node should keep runc as default runtime after guard blocks rollout")

		g.By("Recovering pool by setting osImageStream back to rhel-9")
		o.Expect(setPoolOSImageStream(ctx, mcClient, runcRHCOS10GuardPool, streamRHEL9)).To(o.Succeed())
		o.Expect(waitForMCPRecovery(ctx, mcClient, runcRHCOS10GuardPool, 30*time.Minute)).To(o.Succeed())

		g.By("Verifying node remains ready, not rolling out, on RHCOS 9 with runc after recovery")
		o.Expect(verifyNodeReadyAndNotRollingOut(ctx, oc, nodeName)).To(o.Succeed())
		rhelMajor, err = nodeRHELMajorVersion(oc, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rhelMajor).To(o.Equal("9"), "node should remain on RHCOS 9 after recovery")
		o.Expect(nodeUsesRuncRuntime(oc, nodeName)).To(o.BeTrue(), "node should keep runc as default runtime after recovery")
	})

	g.AfterEach(func(ctx context.Context) {
		if nodeName != "" {
			roleLabel := poolNodeRoleLabel(runcRHCOS10GuardPool)
			o.Expect(removeNodeLabel(ctx, oc, nodeName, roleLabel)).To(o.Succeed())
		}
		o.Expect(deleteContainerRuntimeConfig(ctx, mcClient, runcGuardCRCName)).To(o.Succeed())
		if nodeName != "" {
			o.Expect(waitForMCPMachineCount(ctx, mcClient, runcRHCOS10GuardPool, 0, 10*time.Minute)).To(o.Succeed())
			o.Expect(waitForNodeWorkerConfigRollback(ctx, oc, nodeName, runcRHCOS10GuardPool, 15*time.Minute)).To(o.Succeed())
		}
		o.Expect(deleteMachineConfigPool(ctx, mcClient, runcRHCOS10GuardPool)).To(o.Succeed())
		if nodeName != "" {
			o.Expect(waitForMCP(ctx, mcClient, "worker", 30*time.Minute)).To(o.Succeed())
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

		if degraded := clusterOperatorConditionStatus(co.Status.Conditions, configv1.OperatorDegraded); degraded == configv1.ConditionTrue {
			return false, fmt.Errorf("ClusterOperator %s Degraded=True; expected Upgradeable=False only while isolated pool guard is active", machineConfigClusterOperator)
		}
		if available := clusterOperatorConditionStatus(co.Status.Conditions, configv1.OperatorAvailable); available != configv1.ConditionTrue {
			return false, fmt.Errorf("ClusterOperator %s Available=%s, expected True", machineConfigClusterOperator, available)
		}

		upgradeable := findClusterStatusCondition(co.Status.Conditions, configv1.OperatorUpgradeable)
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

		if available := clusterVersionConditionStatus(cv.Status.Conditions, configv1.OperatorAvailable); available != configv1.ConditionTrue {
			return false, fmt.Errorf("ClusterVersion Available=%s, expected True", available)
		}
		if progressing := clusterVersionConditionStatus(cv.Status.Conditions, configv1.OperatorProgressing); progressing == configv1.ConditionTrue {
			return false, fmt.Errorf("ClusterVersion Progressing=True while isolated pool guard is active")
		}
		if degraded := clusterVersionConditionStatus(cv.Status.Conditions, configv1.OperatorDegraded); degraded == configv1.ConditionTrue {
			return false, fmt.Errorf("ClusterVersion Degraded=True while isolated pool guard is active")
		}

		cvUpgradeable := findClusterStatusCondition(cv.Status.Conditions, configv1.OperatorUpgradeable)
		if cvUpgradeable == nil || cvUpgradeable.Status != configv1.ConditionFalse {
			status := configv1.ConditionUnknown
			if cvUpgradeable != nil {
				status = cvUpgradeable.Status
			}
			framework.Logf("waiting for ClusterVersion Upgradeable=False, current status=%s", status)
			return false, nil
		}

		// TechPreview/CustomNoUpgrade clusters already report Upgradeable=False on ClusterVersion
		// (e.g. FeatureGatesUpgradeable) and CVO may not list machine-config among the reasons.
		// MCO extended tests assert only co/machine-config Upgradeable=False (DegradedPool).
		if exutil.IsNoUpgradeFeatureSet(oc) {
			framework.Logf("ClusterOperator %s reports Upgradeable=False (reason %s); ClusterVersion already Upgradeable=False on a non-upgradeable feature set",
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

// waitForNodeWorkerConfigRollback waits until the node is fully back on the worker pool rendered
// config. This must complete before deleting the custom MCP; otherwise the node's currentConfig
// can reference a rendered MC that no longer exists.
func waitForNodeWorkerConfigRollback(ctx context.Context, oc *exutil.CLI, nodeName, poolName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
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

func findClusterStatusCondition(conditions []configv1.ClusterOperatorStatusCondition, condType configv1.ClusterStatusConditionType) *configv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

func clusterOperatorConditionStatus(conditions []configv1.ClusterOperatorStatusCondition, condType configv1.ClusterStatusConditionType) configv1.ConditionStatus {
	for _, c := range conditions {
		if c.Type == condType {
			return c.Status
		}
	}
	return configv1.ConditionUnknown
}

func clusterVersionConditionStatus(conditions []configv1.ClusterOperatorStatusCondition, condType configv1.ClusterStatusConditionType) configv1.ConditionStatus {
	for _, c := range conditions {
		if c.Type == condType {
			return c.Status
		}
	}
	return configv1.ConditionUnknown
}

func requireOSImageStreams(ctx context.Context, mcClient *machineconfigclient.Clientset) {
	_, err := mcClient.MachineconfigurationV1().OSImageStreams().Get(ctx, "cluster", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip("OSImageStream API is not available; enable TechPreviewNoUpgrade / OSStreams on the cluster")
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	osi, err := mcClient.MachineconfigurationV1().OSImageStreams().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	streamNames := make([]string, 0, len(osi.Status.AvailableStreams))
	for _, s := range osi.Status.AvailableStreams {
		streamNames = append(streamNames, s.Name)
	}
	o.Expect(streamNames).To(o.ContainElements(streamRHEL9, streamRHEL10),
		"dual stream (rhel-9 and rhel-10) must be available")
	framework.Logf("OSImageStream default=%q streams=%v", osi.Status.DefaultStream, streamNames)
}

func poolNodeRoleLabel(poolName string) string {
	return fmt.Sprintf("node-role.kubernetes.io/%s", poolName)
}

func createRuncGuardMCP(ctx context.Context, mcClient *machineconfigclient.Clientset) error {
	mcp := &machineconfigv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: runcRHCOS10GuardPool,
			Labels: map[string]string{
				fmt.Sprintf("pools.operator.machineconfiguration.openshift.io/%s", runcRHCOS10GuardPool): "",
			},
		},
		Spec: machineconfigv1.MachineConfigPoolSpec{
			OSImageStream: machineconfigv1.OSImageStreamReference{Name: streamRHEL9},
			MachineConfigSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      machineconfigv1.MachineConfigRoleLabelKey,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"worker", runcRHCOS10GuardPool},
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
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
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
					fmt.Sprintf("pools.operator.machineconfiguration.openshift.io/%s", runcRHCOS10GuardPool): "",
				},
			},
			ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
				DefaultRuntime: machineconfigv1.ContainerRuntimeDefaultRuntimeRunc,
			},
		},
	}
	_, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, crc, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func labelRandomPureWorker(ctx context.Context, oc *exutil.CLI, poolName string) (string, error) {
	workers, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
	if err != nil {
		return "", err
	}
	workers = getPureWorkerNodes(workers)
	if len(workers) == 0 {
		return "", fmt.Errorf("no pure worker nodes available to label")
	}

	node := workers[rand.Intn(len(workers))]
	label := poolNodeRoleLabel(poolName)
	nodeCopy := node.DeepCopy()
	if nodeCopy.Labels == nil {
		nodeCopy.Labels = map[string]string{}
	}
	nodeCopy.Labels[label] = ""
	_, err = oc.AdminKubeClient().CoreV1().Nodes().Update(ctx, nodeCopy, metav1.UpdateOptions{})
	if err != nil {
		return "", err
	}
	framework.Logf("Labeled node %s with %s", node.Name, label)
	return node.Name, nil
}

func removeNodeLabel(ctx context.Context, oc *exutil.CLI, nodeName, label string) error {
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if _, ok := node.Labels[label]; !ok {
		return nil
	}
	nodeCopy := node.DeepCopy()
	delete(nodeCopy.Labels, label)
	_, err = oc.AdminKubeClient().CoreV1().Nodes().Update(ctx, nodeCopy, metav1.UpdateOptions{})
	return err
}

func waitForMCPMachineCount(ctx context.Context, mcClient *machineconfigclient.Clientset, poolName string, count int32, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) && count == 0 {
				return true, nil
			}
			return false, err
		}
		if mcp.Status.MachineCount == count {
			framework.Logf("MCP %s machine count reached %d", poolName, count)
			return true, nil
		}
		framework.Logf("MCP %s waiting for machine count %d (current %d)", poolName, count, mcp.Status.MachineCount)
		return false, nil
	})
}

func setPoolOSImageStream(ctx context.Context, mcClient *machineconfigclient.Clientset, poolName, stream string) error {
	mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	mcp.Spec.OSImageStream = machineconfigv1.OSImageStreamReference{Name: stream}
	_, err = mcClient.MachineconfigurationV1().MachineConfigPools().Update(ctx, mcp, metav1.UpdateOptions{})
	return err
}

// waitForMCPRecovery waits for a pool to clear Degraded/RenderDegraded and become ready again.
// Unlike waitForMCP, it tolerates transient degraded state while MCO re-renders after recovery.
func waitForMCPRecovery(ctx context.Context, mcClient *machineconfigclient.Clientset, poolName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		degraded := false
		renderDegraded := false
		updating := false
		updated := false
		for _, c := range mcp.Status.Conditions {
			switch c.Type {
			case machineconfigv1.MachineConfigPoolDegraded:
				degraded = c.Status == corev1.ConditionTrue
			case machineconfigv1.MachineConfigPoolRenderDegraded:
				renderDegraded = c.Status == corev1.ConditionTrue
			case machineconfigv1.MachineConfigPoolUpdating:
				updating = c.Status == corev1.ConditionTrue
			case machineconfigv1.MachineConfigPoolUpdated:
				updated = c.Status == corev1.ConditionTrue
			}
		}

		isReady := !degraded && !renderDegraded && !updating && updated &&
			mcp.Status.MachineCount > 0 && mcp.Status.ReadyMachineCount == mcp.Status.MachineCount

		if isReady {
			framework.Logf("MCP %s recovered: %d/%d machines ready", poolName, mcp.Status.ReadyMachineCount, mcp.Status.MachineCount)
		} else {
			framework.Logf("MCP %s recovery in progress: degraded=%v renderDegraded=%v updating=%v updated=%v machines=%d/%d",
				poolName, degraded, renderDegraded, updating, updated, mcp.Status.ReadyMachineCount, mcp.Status.MachineCount)
		}
		return isReady, nil
	})
}

func waitForMCPRenderDegraded(ctx context.Context, mcClient *machineconfigclient.Clientset, poolName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		degraded := false
		renderDegraded := false
		var renderMessage string
		for _, c := range mcp.Status.Conditions {
			switch c.Type {
			case machineconfigv1.MachineConfigPoolDegraded:
				degraded = c.Status == corev1.ConditionTrue
			case machineconfigv1.MachineConfigPoolRenderDegraded:
				if c.Status == corev1.ConditionTrue {
					renderDegraded = true
					renderMessage = c.Message
				}
			}
		}

		if degraded && renderDegraded &&
			strings.Contains(renderMessage, "runc") &&
			strings.Contains(renderMessage, streamRHEL10) {
			framework.Logf("MCP %s is degraded as expected: %s", poolName, renderMessage)
			return true, nil
		}

		framework.Logf("MCP %s waiting for runc+rhel-10 guard: degraded=%v renderDegraded=%v message=%q",
			poolName, degraded, renderDegraded, renderMessage)
		return false, nil
	})
}

func nodeUsesRuncRuntime(oc *exutil.CLI, nodeName string) bool {
	out, err := ExecOnNodeWithChroot(oc, nodeName, "grep", "default_runtime", runcCRCDefaultRuntimePath)
	o.Expect(err).NotTo(o.HaveOccurred(), "CRC default-runtime drop-in should exist on node %s", nodeName)
	return strings.Contains(out, "runc")
}

func nodeRHELMajorVersion(oc *exutil.CLI, nodeName string) (string, error) {
	out, err := ExecOnNodeWithChroot(oc, nodeName, "grep", "^VERSION_ID=", "/etc/os-release")
	if err != nil {
		return "", err
	}
	// VERSION_ID="9.8" or VERSION_ID=9.8
	out = strings.TrimSpace(out)
	out = strings.TrimPrefix(out, "VERSION_ID=")
	out = strings.Trim(out, `"`)
	if idx := strings.Index(out, "."); idx > 0 {
		out = out[:idx]
	}
	return out, nil
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
