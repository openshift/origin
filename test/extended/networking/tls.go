package networking

import (
	"context"
	"crypto/tls"
	"fmt"
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	configv1 "github.com/openshift/api/config/v1"
	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	node "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
	operatorutil "github.com/openshift/origin/test/extended/util/operator"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	MCPRolloutStartTimeout          = 10 * time.Minute
	MCPRolloutCompleteTimeout       = 120 * time.Minute
	NodeStabilityTimeout            = 90 * time.Minute
	OperatorSettleTimeMinutes       = 60
	OperatorProgressingStartTimeout = 5 * time.Minute
	APIServerOperatorTimeout        = 10 * time.Minute
)

type TLSAdherenceNotSupportedError struct {
	Message string
}

func (e *TLSAdherenceNotSupportedError) Error() string {
	return e.Message
}

func IsTLSAdherenceNotSupported(err error) bool {
	_, ok := err.(*TLSAdherenceNotSupportedError)
	return ok
}

var _ = g.Describe("[sig-network][Serial][Disruptive][Suite:openshift/tls-observed-config] TLS Profile Compliance", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("networking-tls")

	g.It("should verify TLS compliance across Modern and Intermediate profiles with different adherence policies", ote.Informing(), func(ctx context.Context) {
		isOpenShift, err := IsOpenShiftCluster(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to check if cluster is OpenShift")
		if !isOpenShift {
			g.Skip("TLS Profile Compliance testing requires OpenShift cluster with config.openshift.io APIs")
		}

		// Scenario 1: Modern TLS Profile with LegacyAdheringComponentsOnly
		g.By("Configuring and verifying Modern TLS Profile with LegacyAdheringComponentsOnly")
		err = ConfigureTLSProfileWithAdherence(ctx, oc, configv1.TLSProfileModernType, configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly)
		if IsTLSAdherenceNotSupported(err) {
			g.Skip(fmt.Sprintf("Skipping test - tlsAdherence API field not supported in this cluster version: %s", err.Error()))
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to configure Modern TLS profile with LegacyAdheringComponentsOnly")
		verifyAllComponentsCompliance(ctx, oc)

		// Scenario 2: Modern TLS Profile with StrictAllComponents
		g.By("Configuring and verifying Modern TLS Profile with StrictAllComponents")
		err = ConfigureTLSProfileWithAdherence(ctx, oc, configv1.TLSProfileModernType, configv1.TLSAdherencePolicyStrictAllComponents)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to configure Modern TLS profile with StrictAllComponents")
		verifyAllComponentsCompliance(ctx, oc)

		// Scenario 3: Intermediate TLS Profile with StrictAllComponents
		g.By("Configuring and verifying Intermediate TLS Profile with StrictAllComponents")
		err = ConfigureTLSProfileWithAdherence(ctx, oc, configv1.TLSProfileIntermediateType, configv1.TLSAdherencePolicyStrictAllComponents)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to configure Intermediate TLS profile with StrictAllComponents")
		verifyAllComponentsCompliance(ctx, oc)
	})
})

func verifyAllComponentsCompliance(ctx context.Context, oc *exutil.CLI) {
	k8sClient := oc.AdminKubeClient()
	configClient := oc.AdminConfigClient()

	e2e.Logf("Starting TLS compliance checks with retry (timeout: %v, interval: %v)", 5*time.Minute, 30*time.Second)
	err := wait.PollUntilContextTimeout(ctx, 30*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		var checkErr error

		// Test multus-cni
		g.By("Testing TLS compliance for multus-admission-controller in openshift-multus (ports 6443 & 8443)")
		checkErr = VerifyMultusTLSComplianceInPod(ctx, oc, configClient, k8sClient, "openshift-multus", "app=multus-admission-controller")
		if checkErr != nil {
			e2e.Logf("Multus TLS check failed (will retry): %v", checkErr)
			return false, nil
		}

		// Test ovn-kubernetes nodes
		g.By("Testing TLS compliance for ovn-kubernetes nodes in openshift-ovn-kubernetes (ports 9103, 9105)")
		checkErr = VerifyOVNKubernetesNodeTLSComplianceInPod(ctx, oc, configClient, k8sClient, "openshift-ovn-kubernetes", "app=ovnkube-node")
		if checkErr != nil {
			e2e.Logf("OVN-Kubernetes node TLS check failed (will retry): %v", checkErr)
			return false, nil
		}

		// Test cluster-network-operator
		g.By("Testing TLS compliance for cluster-network-operator in openshift-network-operator (ports 9104, 9103, 9105)")
		checkErr = VerifyCNOTLSComplianceInPod(ctx, oc, configClient, k8sClient, "openshift-network-operator", "name=network-operator")
		if checkErr != nil {
			e2e.Logf("CNO TLS check failed (will retry): %v", checkErr)
			return false, nil
		}

		// Test networking-console-plugin
		g.By("Testing TLS compliance for networking-console-plugin in openshift-network-console (port 9443)")
		checkErr = VerifyNetworkConsoleTLSComplianceInPod(ctx, oc, configClient, k8sClient, "openshift-network-console", "app.kubernetes.io/name=networking-console-plugin")
		if checkErr != nil {
			e2e.Logf("Networking Console Plugin TLS check failed (will retry): %v", checkErr)
			return false, nil
		}

		e2e.Logf("All TLS compliance checks passed")
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "TLS compliance verification failed after retries")
}

func IsOpenShiftCluster(ctx context.Context, oc *exutil.CLI) (bool, error) {
	configClient, err := configv1client.NewForConfig(oc.AdminConfig())
	if err != nil {
		return false, err
	}

	_, err = configClient.ConfigV1().FeatureGates().Get(
		ctx,
		"cluster",
		metav1.GetOptions{},
	)
	if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
		return false, nil
	}
	return err == nil, err
}

func ConfigureTLSProfileWithAdherence(ctx context.Context, oc *exutil.CLI, tlsProfileType configv1.TLSProfileType, tlsAdherencePolicy configv1.TLSAdherencePolicy) error {
	configClient, err := configv1client.NewForConfig(oc.AdminConfig())
	if err != nil {
		return fmt.Errorf("failed to create config client: %w", err)
	}

	machineConfigClient, err := machineconfigclient.NewForConfig(oc.AdminConfig())
	if err != nil {
		return fmt.Errorf("failed to create machine config client: %w", err)
	}

	k8sClient, err := kubernetes.NewForConfig(oc.AdminConfig())
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	e2e.Logf("=== Configuring %s TLS Profile with %s ===", tlsProfileType, tlsAdherencePolicy)

	e2e.Logf("Enabling TLSAdherence feature gate")
	fg, err := configClient.ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get featuregate: %w", err)
	}

	tlsFeatureAlreadyEnabled := isAlreadyEnabled(fg)
	if tlsFeatureAlreadyEnabled {
		e2e.Logf("TLSAdherence feature gate already enabled - proceeding to TLS profile configuration")
	} else {
		// Capture time BEFORE feature gate change
		featureGateChangeTime := metav1.Now()

		if err := patchFeatureGate(ctx, configClient, fg); err != nil {
			return fmt.Errorf("failed to patch FeatureGate: %w", err)
		}
		e2e.Logf("TLSAdherence feature gate enabled")

		e2e.Logf("Waiting for MCP rollout to start after feature gate enablement (timeout: %v)", MCPRolloutStartTimeout)
		if err := waitForMCPRolloutStart(ctx, machineConfigClient, MCPRolloutStartTimeout); err != nil {
			e2e.Logf("WARNING: MCP rollout did not start within timeout: %v", err)
		} else {
			e2e.Logf("Waiting for MCP rollout to complete (this may take up to %v minutes)", MCPRolloutCompleteTimeout/time.Minute)
			if err := waitForAllMCPsComplete(ctx, machineConfigClient, MCPRolloutCompleteTimeout); err != nil {
				return fmt.Errorf("failed waiting for MCP rollout after feature gate enablement: %w", err)
			}
			e2e.Logf("MCP rollout completed after feature gate enablement")
		}

		e2e.Logf("Waiting for all nodes to be ready after feature gate enablement")
		if err := waitForNodesStability(ctx, k8sClient, NodeStabilityTimeout, featureGateChangeTime); err != nil {
			return fmt.Errorf("failed waiting for nodes after feature gate enablement: %w", err)
		}
		e2e.Logf("All nodes are ready after feature gate enablement")

		e2e.Logf("Waiting for cluster operators to settle after feature gate enablement")
		if err := operatorutil.WaitForOperatorsToSettle(ctx, configClient, OperatorSettleTimeMinutes); err != nil {
			return fmt.Errorf("failed waiting for operators after feature gate enablement: %w", err)
		}
		e2e.Logf("All cluster operators settled after feature gate enablement")
	}

	// Capture time BEFORE APIServer TLS profile change
	tlsProfileChangeTime := metav1.Now()

	e2e.Logf("Configuring APIServer with %s TLS profile and tlsAdherence=%s", tlsProfileType, tlsAdherencePolicy)
	if err := patchAPIServerTLSProfile(ctx, configClient, tlsProfileType, tlsAdherencePolicy); err != nil {
		return err
	}
	e2e.Logf("APIServer TLS profile configured successfully")

	// TODO: This hardcodes implementation details about when MCP rollouts are triggered.
	// Consider either (a) dynamically checking MCP status to detect if rollout was triggered,
	// or (b) making the waiting logic work uniformly regardless of whether MCP rollout occurs.
	requiresMCPRollout := ((tlsProfileType == configv1.TLSProfileModernType &&
		tlsAdherencePolicy == configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly) ||
		(tlsProfileType == configv1.TLSProfileIntermediateType &&
			tlsAdherencePolicy == configv1.TLSAdherencePolicyStrictAllComponents))

	if requiresMCPRollout {
		e2e.Logf("Waiting for MCP rollout to start after TLS profile configuration (timeout: %v)", MCPRolloutStartTimeout)
		if err := waitForMCPRolloutStart(ctx, machineConfigClient, MCPRolloutStartTimeout); err != nil {
			return fmt.Errorf("MCP rollout did not start within timeout: %w", err)
		}

		e2e.Logf("Waiting for MCP rollout to complete (this may take up to %v minutes)", MCPRolloutCompleteTimeout/time.Minute)
		if err := waitForAllMCPsComplete(ctx, machineConfigClient, MCPRolloutCompleteTimeout); err != nil {
			return fmt.Errorf("failed waiting for MCP rollout: %w", err)
		}
		e2e.Logf("MCP rollout completed")

		e2e.Logf("Waiting for TLS-related operators to stabilize with new configuration")
		if err := waitForAPIServerOperatorStable(ctx, configClient, APIServerOperatorTimeout); err != nil {
			return fmt.Errorf("TLS-related operators did not stabilize: %w", err)
		}

		e2e.Logf("Waiting for all nodes to be ready and stable (timeout: %v)", NodeStabilityTimeout)
		if err := waitForNodesStability(ctx, k8sClient, NodeStabilityTimeout, tlsProfileChangeTime); err != nil {
			return err
		}
		e2e.Logf("All nodes are ready and stable")

		e2e.Logf("Waiting for cluster operators to settle (timeout: %v)", time.Duration(OperatorSettleTimeMinutes)*time.Minute)
		if err := operatorutil.WaitForOperatorsToSettle(ctx, configClient, OperatorSettleTimeMinutes); err != nil {
			return err
		}
		e2e.Logf("All cluster operators settled")
	} else {
		// No MCP rollout path (e.g., Modern + StrictAllComponents)
		// Wait for TLS-related operators only - don't use timestamp check as operators
		// may already be settled from previous scenario with old timestamps
		e2e.Logf("Waiting for TLS-related operators to stabilize with new configuration")
		if err := waitForAPIServerOperatorStable(ctx, configClient, APIServerOperatorTimeout); err != nil {
			return fmt.Errorf("TLS-related operators did not stabilize: %w", err)
		}

		// Wait for all operators to settle (no timestamp check to avoid race with sequential scenarios)
		e2e.Logf("Waiting for cluster operators to settle (timeout: %v)", time.Duration(OperatorSettleTimeMinutes)*time.Minute)
		if err := operatorutil.WaitForOperatorsToSettle(ctx, configClient, OperatorSettleTimeMinutes); err != nil {
			return fmt.Errorf("cluster operators did not settle: %w", err)
		}
		e2e.Logf("All cluster operators settled")

		e2e.Logf("TLS configuration applied (no MCP rollout required for this profile)")
	}

	e2e.Logf("Verifying TLSAdherence is active in feature gate status")
	if err := verifyTLSAdherenceActive(ctx, configClient); err != nil {
		return err
	}
	e2e.Logf("TLSAdherence is active in feature gate status")

	e2e.Logf("Verifying APIServer TLS profile configuration")
	apiserver, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get APIServer: %w", err)
	}

	if apiserver.Spec.TLSSecurityProfile == nil || apiserver.Spec.TLSSecurityProfile.Type != tlsProfileType {
		return fmt.Errorf("APIServer TLS profile is not %s", tlsProfileType)
	}

	if apiserver.Spec.TLSAdherence != tlsAdherencePolicy {
		return fmt.Errorf("APIServer tlsAdherence is %s, expected %s", apiserver.Spec.TLSAdherence, tlsAdherencePolicy)
	}

	if apiserver.Spec.TLSAdherence == "" {
		e2e.Logf("APIServer configuration verified: tlsSecurityProfile.type=%s, tlsAdherence=<empty> (default)", tlsProfileType)
		e2e.Logf("APIServer %s TLS profile and tlsAdherence=<empty> (default) verified", tlsProfileType)
	} else {
		e2e.Logf("APIServer configuration verified: tlsSecurityProfile.type=%s, tlsAdherence=%s", tlsProfileType, apiserver.Spec.TLSAdherence)
		e2e.Logf("APIServer %s TLS profile and tlsAdherence=%s verified", tlsProfileType, apiserver.Spec.TLSAdherence)
	}

	e2e.Logf("=== %s TLS Profile successfully configured and cluster is stable ===", tlsProfileType)

	return nil
}

func isAlreadyEnabled(fg *configv1.FeatureGate) bool {
	if fg.Spec.FeatureSet == configv1.CustomNoUpgrade && fg.Spec.CustomNoUpgrade != nil {
		for _, enabled := range fg.Spec.CustomNoUpgrade.Enabled {
			if enabled == "TLSAdherence" {
				return true
			}
		}
	}
	return false
}

func patchFeatureGate(ctx context.Context, configClient configv1client.Interface, fg *configv1.FeatureGate) error {
	if fg.Spec.FeatureSet == configv1.CustomNoUpgrade && fg.Spec.CustomNoUpgrade != nil {
		fg.Spec.CustomNoUpgrade.Enabled = append(fg.Spec.CustomNoUpgrade.Enabled, "TLSAdherence")
	} else {
		fg.Spec.FeatureSet = configv1.CustomNoUpgrade
		fg.Spec.CustomNoUpgrade = &configv1.CustomFeatureGates{
			Enabled: []configv1.FeatureGateName{"TLSAdherence"},
		}
	}

	_, err := configClient.ConfigV1().FeatureGates().Update(ctx, fg, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update featuregate: %w", err)
	}
	return nil
}

func patchAPIServerTLSProfile(ctx context.Context, configClient configv1client.Interface, tlsProfileType configv1.TLSProfileType, tlsAdherencePolicy configv1.TLSAdherencePolicy) error {
	apiserver, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get apiserver: %w", err)
	}

	switch tlsProfileType {
	case configv1.TLSProfileModernType:
		apiserver.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
			Type:   configv1.TLSProfileModernType,
			Modern: &configv1.ModernTLSProfile{},
		}
	case configv1.TLSProfileIntermediateType:
		apiserver.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
			Type:         configv1.TLSProfileIntermediateType,
			Intermediate: &configv1.IntermediateTLSProfile{},
		}
	case configv1.TLSProfileOldType:
		apiserver.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
			Type: configv1.TLSProfileOldType,
			Old:  &configv1.OldTLSProfile{},
		}
	default:
		return fmt.Errorf("unsupported TLS profile type: %s (must be Modern, Intermediate, or Old)", tlsProfileType)
	}

	apiserver.Spec.TLSAdherence = tlsAdherencePolicy

	_, err = configClient.ConfigV1().APIServers().Update(ctx, apiserver, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update apiserver: %w", err)
	}

	return nil
}

func waitForMCPRolloutStart(ctx context.Context, client machineconfigclient.Interface, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		mcps, err := client.MachineconfigurationV1().MachineConfigPools().List(ctx, metav1.ListOptions{})
		if err != nil {
			if apierrors.IsTimeout(err) ||
				apierrors.IsServerTimeout(err) ||
				apierrors.IsServiceUnavailable(err) ||
				apierrors.IsTooManyRequests(err) {
				e2e.Logf("Transient API error listing MCPs (will retry): %v", err)
				return false, nil
			}

			errMsg := err.Error()
			if strings.Contains(errMsg, "connection refused") ||
				strings.Contains(errMsg, "i/o timeout") ||
				strings.Contains(errMsg, "context deadline exceeded") ||
				strings.Contains(errMsg, "time allotted") {
				e2e.Logf("Transient API error listing MCPs (will retry): %v", err)
				return false, nil
			}

			return false, err
		}

		for _, mcp := range mcps.Items {
			for _, cond := range mcp.Status.Conditions {
				if cond.Type == "Updating" && cond.Status == corev1.ConditionTrue {
					e2e.Logf("MCP %s has started updating", mcp.Name)
					return true, nil
				}
			}
		}
		return false, nil
	})
}

func areAllMCPsComplete(ctx context.Context, client machineconfigclient.Interface) (bool, error) {
	mcps, err := client.MachineconfigurationV1().MachineConfigPools().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list MCPs: %w", err)
	}

	for _, mcp := range mcps.Items {
		if mcp.Status.MachineCount == 0 {
			continue
		}

		updated := false
		updating := false

		for _, cond := range mcp.Status.Conditions {
			if cond.Type == "Updated" && cond.Status == corev1.ConditionTrue {
				updated = true
			}
			if cond.Type == "Updating" && cond.Status == corev1.ConditionTrue {
				updating = true
			}
		}

		if !updated || updating {
			return false, nil
		}
	}

	return true, nil
}

func waitForAllMCPsComplete(ctx context.Context, client machineconfigclient.Interface, timeout time.Duration) error {
	clientset, ok := client.(*machineconfigclient.Clientset)
	if !ok {
		return fmt.Errorf("failed to convert client to Clientset")
	}

	var mcps *machineconfigv1.MachineConfigPoolList
	listErr := wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		var err error
		mcps, err = client.MachineconfigurationV1().MachineConfigPools().List(ctx, metav1.ListOptions{})
		if err != nil {
			e2e.Logf("Error listing MCPs (will retry): %v", err)
			return false, nil
		}
		return true, nil
	})
	if listErr != nil {
		return fmt.Errorf("failed to list MCPs after retries: %w", listErr)
	}

	for _, mcp := range mcps.Items {
		if mcp.Status.MachineCount == 0 {
			e2e.Logf("Skipping MCP %s (no machines)", mcp.Name)
			continue
		}

		e2e.Logf("Waiting for MCP %s (%d machines) to complete rollout", mcp.Name, mcp.Status.MachineCount)

		mcpErr := wait.PollUntilContextTimeout(ctx, 30*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
			err := node.WaitForMCP(ctx, clientset, mcp.Name, timeout, node.WaitMCPAllowDegraded())
			if err != nil {
				if apierrors.IsTimeout(err) ||
					apierrors.IsServerTimeout(err) ||
					apierrors.IsServiceUnavailable(err) ||
					apierrors.IsTooManyRequests(err) {
					e2e.Logf("Transient API error waiting for MCP %s (will retry): %v", mcp.Name, err)
					return false, nil
				}

				errMsg := err.Error()
				if strings.Contains(errMsg, "connection refused") ||
					strings.Contains(errMsg, "TLS handshake timeout") ||
					strings.Contains(errMsg, "i/o timeout") ||
					strings.Contains(errMsg, "context deadline exceeded") ||
					strings.Contains(errMsg, "stream error") ||
					strings.Contains(errMsg, "time allotted") {
					e2e.Logf("Transient API error waiting for MCP %s (will retry): %v", mcp.Name, err)
					return false, nil
				}
				return false, err
			}
			return true, nil
		})

		if mcpErr != nil {
			return fmt.Errorf("MCP %s did not complete: %w", mcp.Name, mcpErr)
		}
		e2e.Logf("MCP %s complete: %d/%d machines ready", mcp.Name, mcp.Status.ReadyMachineCount, mcp.Status.MachineCount)
	}

	e2e.Logf("All MCPs completed successfully")
	return nil
}

func waitForNodesStability(ctx context.Context, client kubernetes.Interface, timeout time.Duration, afterTime metav1.Time) error {
	return wait.PollUntilContextTimeout(ctx, 30*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			e2e.Logf("Error getting nodes: %v", err)
			return false, nil
		}

		notReady := []string{}
		notUpdated := []string{}
		for _, node := range nodes.Items {
			readyTime, ready := getNodeReadyTime(&node)
			if !ready {
				notReady = append(notReady, node.Name)
			} else if !readyTime.After(afterTime.Time) {
				// Node is Ready but hasn't transitioned to Ready after the config change
				notUpdated = append(notUpdated, fmt.Sprintf("%s(ready-since:%s)", node.Name, readyTime.Format(time.RFC3339)))
			}
		}

		if len(notReady) > 0 {
			e2e.Logf("Waiting for nodes to be ready: %s", strings.Join(notReady, ", "))
			return false, nil
		}

		if len(notUpdated) > 0 {
			e2e.Logf("Waiting for nodes to process config change (ready before %s): %s",
				afterTime.Format(time.RFC3339), strings.Join(notUpdated, ", "))
			return false, nil
		}

		e2e.Logf("All %d nodes are ready and updated after config change", len(nodes.Items))
		return true, nil
	})
}

func getNodeReadyTime(node *corev1.Node) (metav1.Time, bool) {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionTrue {
				return cond.LastTransitionTime, true
			}
			return metav1.Time{}, false
		}
	}
	return metav1.Time{}, false
}

// waitForOperatorsToSettleAfter waits for all cluster operators to be settled
// (Available=True, Degraded=False, Progressing=False) with a transition timestamp
// after the specified time. This ensures operators have reacted to a config change.
func waitForOperatorsToSettleAfter(ctx context.Context, configClient configv1client.Interface, timeout time.Duration, afterTime metav1.Time) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		coList, err := configClient.ConfigV1().ClusterOperators().List(ctx, metav1.ListOptions{})
		if err != nil {
			e2e.Logf("Error getting ClusterOperators: %v", err)
			return false, nil
		}

		unsettled := []string{}
		notUpdated := []string{}

		for _, co := range coList.Items {
			available := getCondition(co.Status.Conditions, configv1.OperatorAvailable)
			degraded := getCondition(co.Status.Conditions, configv1.OperatorDegraded)
			progressing := getCondition(co.Status.Conditions, configv1.OperatorProgressing)

			// Check if operator is settled
			isSettled := available.Status == configv1.ConditionTrue &&
				degraded.Status == configv1.ConditionFalse &&
				progressing.Status == configv1.ConditionFalse

			if !isSettled {
				unsettled = append(unsettled, fmt.Sprintf("%s(A=%s,D=%s,P=%s)",
					co.Name, available.Status, degraded.Status, progressing.Status))
				continue
			}

			// Operator is settled, but check if it transitioned after the config change
			// Use Progressing transition time since it's the last condition to become False
			// after operators finish processing a change
			if !progressing.LastTransitionTime.After(afterTime.Time) {
				notUpdated = append(notUpdated, fmt.Sprintf("%s(progressing-since:%s)",
					co.Name, progressing.LastTransitionTime.Format(time.RFC3339)))
			}
		}

		if len(unsettled) > 0 {
			e2e.Logf("Waiting for operators to settle: %s", strings.Join(unsettled, ", "))
			return false, nil
		}

		if len(notUpdated) > 0 {
			e2e.Logf("Waiting for operators to transition after config change: %s", strings.Join(notUpdated, ", "))
			return false, nil
		}

		e2e.Logf("All operators settled with transitions after %s", afterTime.Format(time.RFC3339))
		return true, nil
	})
}

// getCondition finds a specific condition type from a list of conditions
func getCondition(conditions []configv1.ClusterOperatorStatusCondition, condType configv1.ClusterStatusConditionType) configv1.ClusterOperatorStatusCondition {
	for _, cond := range conditions {
		if cond.Type == condType {
			return cond
		}
	}
	return configv1.ClusterOperatorStatusCondition{
		Type:   condType,
		Status: configv1.ConditionUnknown,
	}
}

func waitForOperatorsToStartProgressing(ctx context.Context, configClient configv1client.Interface, timeout time.Duration) error {
	relevantOperators := []string{"kube-apiserver", "openshift-apiserver", "network"}

	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, false, func(ctx context.Context) (bool, error) {
		for _, coName := range relevantOperators {
			co, err := configClient.ConfigV1().ClusterOperators().Get(ctx, coName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("Error getting ClusterOperator %s: %v (retrying)", coName, err)
				return false, nil
			}

			// Check if operator is progressing
			for _, cond := range co.Status.Conditions {
				if cond.Type == configv1.OperatorProgressing && cond.Status == configv1.ConditionTrue {
					e2e.Logf("Operator %s started progressing", coName)
					return true, nil
				}
			}
		}
		return false, nil
	})
}

func waitForAPIServerOperatorStable(ctx context.Context, configClient configv1client.Interface, timeout time.Duration) error {
	relevantOperators := []string{"kube-apiserver", "openshift-apiserver", "network"}
	e2e.Logf("Waiting for operators to be stable with new TLS configuration (timeout: %v)", timeout)

	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, false,
		func(ctx context.Context) (bool, error) {
			allStable := true
			for _, coName := range relevantOperators {
				co, err := configClient.ConfigV1().ClusterOperators().Get(ctx, coName, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) {
						e2e.Logf("API server not responsive yet (retrying): %v", err)
						return false, nil
					}
					e2e.Logf("Error getting %s operator (retrying): %v", coName, err)
					return false, nil
				}

				available := false
				progressing := false
				degraded := false

				for _, cond := range co.Status.Conditions {
					switch cond.Type {
					case configv1.OperatorAvailable:
						available = (cond.Status == configv1.ConditionTrue)
					case configv1.OperatorProgressing:
						progressing = (cond.Status == configv1.ConditionTrue)
					case configv1.OperatorDegraded:
						degraded = (cond.Status == configv1.ConditionTrue)
					}
				}

				if !available {
					e2e.Logf("Operator %s not available yet", coName)
					allStable = false
					break
				}
				if progressing {
					e2e.Logf("Operator %s still progressing", coName)
					allStable = false
					break
				}
				if degraded {
					e2e.Logf("Operator %s is degraded", coName)
					allStable = false
					break
				}
			}

			if allStable {
				e2e.Logf("All TLS-related operators are stable (Available=True, Progressing=False, Degraded=False)")
				return true, nil
			}
			return false, nil
		})
}

func verifyTLSAdherenceActive(ctx context.Context, configClient configv1client.Interface) error {
	cv, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get cluster version: %w", err)
	}
	version := cv.Status.Desired.Version

	e2e.Logf("Verifying TLSAdherence is active for cluster version %s", version)

	return wait.PollUntilContextTimeout(ctx, 15*time.Second, 15*time.Minute, true, func(ctx context.Context) (bool, error) {
		fg, err := configClient.ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Error getting FeatureGate: %v", err)
			return false, nil
		}

		for _, fgStatus := range fg.Status.FeatureGates {
			if fgStatus.Version == version {
				for _, enabled := range fgStatus.Enabled {
					if enabled.Name == "TLSAdherence" {
						return true, nil
					}
				}
			}
		}

		e2e.Logf("TLSAdherence not yet in status for version %s (waiting...)", version)
		return false, nil
	})
}

func verifyTLSComplianceInPods(
	ctx context.Context,
	oc *exutil.CLI,
	configClient configv1client.Interface,
	k8sClient kubernetes.Interface,
	namespace, labelSelector string,
	ports []string,
	componentName string,
	isAdheringComponent bool,
	skipTLS12ForModernStrict bool,
) error {
	apiserver, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get APIServer config: %w", err)
	}

	var expectTLS12Reject bool
	var testDescription string

	if apiserver.Spec.TLSSecurityProfile != nil {
		profileType := apiserver.Spec.TLSSecurityProfile.Type
		adherencePolicy := apiserver.Spec.TLSAdherence

		switch {
		case profileType == configv1.TLSProfileModernType && adherencePolicy == configv1.TLSAdherencePolicyStrictAllComponents:
			expectTLS12Reject = true
			testDescription = "Modern + StrictAllComponents: TLS 1.3 only, TLS 1.2 should be rejected"

		case profileType == configv1.TLSProfileModernType && adherencePolicy == configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly:
			expectTLS12Reject = isAdheringComponent
			if isAdheringComponent {
				testDescription = "Modern + LegacyAdheringComponentsOnly (adhering component): TLS 1.3 only, TLS 1.2 should be rejected"
			} else {
				testDescription = "Modern + LegacyAdheringComponentsOnly (non-adhering component): both TLS 1.2 and TLS 1.3 should work"
			}

		case profileType == configv1.TLSProfileIntermediateType && adherencePolicy == configv1.TLSAdherencePolicyStrictAllComponents:
			expectTLS12Reject = false
			testDescription = "Intermediate + StrictAllComponents: both TLS 1.2 and TLS 1.3 should work"

		default:
			expectTLS12Reject = false
			testDescription = fmt.Sprintf("Profile %s with adherence %s: both TLS 1.2 and TLS 1.3 should work", profileType, adherencePolicy)
		}

		e2e.Logf("Testing %s with profile: %s, adherence: %s, adhering: %v", componentName, profileType, adherencePolicy, isAdheringComponent)
		e2e.Logf("%s", testDescription)
	} else {
		expectTLS12Reject = false
		testDescription = "No profile configured: both TLS 1.2 and TLS 1.3 should work"
		e2e.Logf("Testing %s with profile: %s", componentName, configv1.TLSProfileIntermediateType)
		e2e.Logf("%s", testDescription)
	}

	pods, err := k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods with selector %s: %w", labelSelector, err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found with selector %s in namespace %s", labelSelector, namespace)
	}

	var testPod string
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if slices.ContainsFunc(pod.Status.Conditions, func(condition corev1.PodCondition) bool {
			return condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue
		}) {
			testPod = pod.Name
			break
		}
	}

	if testPod == "" {
		return fmt.Errorf("no ready pods found with selector %s in namespace %s", labelSelector, namespace)
	}

	for _, port := range ports {
		resourceName := fmt.Sprintf("pod/%s", testPod)
		e2e.Logf("Testing TLS on %s/%s port %s", namespace, resourceName, port)

		err := exutil.ForwardPortAndExecute(resourceName, namespace, port, func(localPort int) error {
			tls13Config := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
			e2e.Logf("Testing TLS 1.3 on port %s (should succeed)", port)
			if err := exutil.CheckTLSConnection(localPort, tls13Config, nil); err != nil {
				return fmt.Errorf("TLS 1.3 test failed: %w", err)
			}
			e2e.Logf("TLS 1.3 test PASSED on port %s", port)

			if skipTLS12ForModernStrict &&
				apiserver.Spec.TLSSecurityProfile != nil &&
				apiserver.Spec.TLSSecurityProfile.Type == configv1.TLSProfileModernType &&
				apiserver.Spec.TLSAdherence == configv1.TLSAdherencePolicyStrictAllComponents {
				e2e.Logf("Skipping TLS 1.2 test for %s on port %s (Modern + StrictAllComponents)", componentName, port)
			} else {
				tls12Config := &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}
				if expectTLS12Reject {
					e2e.Logf("Testing TLS 1.2 on port %s (should be REJECTED)", port)
					if err := exutil.CheckTLSConnection(localPort, tls12Config, nil); err == nil {
						return fmt.Errorf("TLS 1.2 connection on port %s SUCCEEDED but should have been REJECTED", port)
					}
					e2e.Logf("TLS 1.2 correctly REJECTED on port %s", port)
				} else {
					e2e.Logf("Testing TLS 1.2 on port %s (should succeed)", port)
					if err := exutil.CheckTLSConnection(localPort, tls12Config, nil); err != nil {
						return fmt.Errorf("TLS 1.2 test failed: %w", err)
					}
					e2e.Logf("TLS 1.2 test PASSED on port %s", port)
				}
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("TLS test failed on port %s: %w", port, err)
		}
	}

	e2e.Logf("All %s TLS endpoints verified in %s/%s", componentName, namespace, testPod)
	return nil
}

func VerifyMultusTLSComplianceInPod(ctx context.Context, oc *exutil.CLI, configClient configv1client.Interface, k8sClient kubernetes.Interface, namespace, labelSelector string) error {
	return verifyTLSComplianceInPods(ctx, oc, configClient, k8sClient, namespace, labelSelector,
		[]string{"6443", "8443"}, "Multus", true, false)
}

func VerifyOVNKubernetesTLSComplianceInPod(ctx context.Context, oc *exutil.CLI, configClient configv1client.Interface, k8sClient kubernetes.Interface, namespace, labelSelector string) error {
	return verifyTLSComplianceInPods(ctx, oc, configClient, k8sClient, namespace, labelSelector,
		[]string{"9108"}, "OVN-Kubernetes Control Plane", false, false)
}

func VerifyOVNKubernetesNodeTLSComplianceInPod(ctx context.Context, oc *exutil.CLI, configClient configv1client.Interface, k8sClient kubernetes.Interface, namespace, labelSelector string) error {
	return verifyTLSComplianceInPods(ctx, oc, configClient, k8sClient, namespace, labelSelector,
		[]string{"9103", "9105"}, "OVN-Kubernetes Node", false, false)
}

func VerifyCNOTLSComplianceInPod(ctx context.Context, oc *exutil.CLI, configClient configv1client.Interface, k8sClient kubernetes.Interface, namespace, labelSelector string) error {
	return verifyTLSComplianceInPods(ctx, oc, configClient, k8sClient, namespace, labelSelector,
		[]string{"9104", "9103", "9105"}, "CNO", false, false)
}

func VerifyNetworkConsoleTLSComplianceInPod(ctx context.Context, oc *exutil.CLI, configClient configv1client.Interface, k8sClient kubernetes.Interface, namespace, labelSelector string) error {
	return verifyTLSComplianceInPods(ctx, oc, configClient, k8sClient, namespace, labelSelector,
		[]string{"9443"}, "Network Console", false, true)
}
