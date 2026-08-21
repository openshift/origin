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
	MCPRolloutStartTimeout    = 10 * time.Minute
	MCPRolloutCompleteTimeout = 120 * time.Minute // Increased from 60m to allow full cluster rollout with TLS profile changes
	NodeStabilityTimeout      = 90 * time.Minute  // Increased from 60m to allow CNI re-initialization on all nodes
	OperatorSettleTimeMinutes = 60
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

// NOTE: This test makes cluster-wide configuration changes and runs serially.
//
// The test automatically enables TLSAdherence feature gate if not already enabled,
// configures APIServer TLS profiles, and triggers MachineConfigPool rollouts.
// It is marked [Serial] to prevent interference with other tests running concurrently.
var _ = g.Describe("[sig-network][Serial][Suite:openshift/tls-observed-config]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("networking-tls")

	g.Context("TLS Profile Compliance", func() {
		g.BeforeEach(func(ctx context.Context) {
			isOpenShift, err := IsOpenShiftCluster(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to check if cluster is OpenShift")
			if !isOpenShift {
				g.Skip("TLS Profile Compliance testing requires OpenShift cluster with config.openshift.io APIs")
			}
		})

		type tlsProfileTest struct {
			profileType     configv1.TLSProfileType
			adherencePolicy configv1.TLSAdherencePolicy
			description     string
		}

		tlsProfiles := []tlsProfileTest{
			{configv1.TLSProfileModernType, configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly, "Modern TLS Profile with LegacyAdheringComponentsOnly"},
			{configv1.TLSProfileModernType, configv1.TLSAdherencePolicyStrictAllComponents, "Modern TLS Profile with StrictAllComponents"},
			{configv1.TLSProfileIntermediateType, configv1.TLSAdherencePolicyStrictAllComponents, "Intermediate TLS Profile with StrictAllComponents"},
		}

		for _, profile := range tlsProfiles {
			profile := profile
			g.Context(profile.description, func() {
				g.BeforeEach(func(ctx context.Context) {
					err := ConfigureTLSProfileWithAdherence(ctx, oc, profile.profileType, profile.adherencePolicy)
					if IsTLSAdherenceNotSupported(err) {
						g.Skip(fmt.Sprintf("Skipping test - tlsAdherence API field not supported in this cluster version: %s", err.Error()))
					}
					o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to configure %s TLS profile with %s", profile.profileType, profile.adherencePolicy))
				})

				g.It("should verify TLS compliance for all networking components", func(ctx context.Context) {
					k8sClient := oc.AdminKubeClient()
					configClient := oc.AdminConfigClient()

					// Retry TLS compliance checks to allow time for configuration to propagate
					// kube-rbac-proxy containers dynamically reload TLS settings without restarting
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
				})
			})
		}
	})
})

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
		e2e.Logf("TLSAdherence feature gate already enabled")
	} else {
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
		if err := waitForNodesStability(ctx, k8sClient, NodeStabilityTimeout); err != nil {
			return fmt.Errorf("failed waiting for nodes after feature gate enablement: %w", err)
		}
		e2e.Logf("All nodes are ready after feature gate enablement")

		e2e.Logf("Waiting for cluster operators to settle after feature gate enablement")
		if err := operatorutil.WaitForOperatorsToSettle(ctx, configClient, OperatorSettleTimeMinutes); err != nil {
			return fmt.Errorf("failed waiting for operators after feature gate enablement: %w", err)
		}
		e2e.Logf("All cluster operators settled after feature gate enablement")
	}

	e2e.Logf("Configuring APIServer with %s TLS profile and tlsAdherence=%s", tlsProfileType, tlsAdherencePolicy)
	if err := patchAPIServerTLSProfile(ctx, configClient, tlsProfileType, tlsAdherencePolicy); err != nil {
		return err
	}
	e2e.Logf("APIServer TLS profile configured successfully")

	// Modern + StrictAllComponents does not require MCP rollout (application-level enforcement only)
	// Modern + LegacyAdheringComponentsOnly requires MCP rollout
	// Intermediate + StrictAllComponents requires MCP rollout
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

		e2e.Logf("Waiting for all nodes to be ready and stable (60 min timeout)")
		if err := waitForNodesStability(ctx, k8sClient, NodeStabilityTimeout); err != nil {
			return err
		}
		e2e.Logf("All nodes are ready and stable")

		e2e.Logf("Waiting for cluster operators to settle (60 min timeout)")
		if err := operatorutil.WaitForOperatorsToSettle(ctx, configClient, OperatorSettleTimeMinutes); err != nil {
			return err
		}
		e2e.Logf("All cluster operators settled")
	} else {
		e2e.Logf("Waiting for cluster operators to settle (60 min timeout)")
		if err := operatorutil.WaitForOperatorsToSettle(ctx, configClient, OperatorSettleTimeMinutes); err != nil {
			return err
		}
		e2e.Logf("All cluster operators settled")

		e2e.Logf("Waiting for all nodes to be ready and stable (60 min timeout)")
		if err := waitForNodesStability(ctx, k8sClient, NodeStabilityTimeout); err != nil {
			return err
		}
		e2e.Logf("All nodes are ready and stable")
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

	if tlsAdherencePolicy != "" && apiserver.Spec.TLSAdherence == "" {
		return &TLSAdherenceNotSupportedError{
			Message: fmt.Sprintf("tlsAdherence API field not supported in this cluster version (tried to set %s but got empty value)", tlsAdherencePolicy),
		}
	}

	if apiserver.Spec.TLSAdherence != "" && apiserver.Spec.TLSAdherence != tlsAdherencePolicy {
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
			// Retry on transient API errors (504, 503, etc.) during cluster reconfiguration
			e2e.Logf("Error listing MCPs (will retry): %v", err)
			return false, nil
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

	// Get the list of MCPs with retry logic for transient API errors
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

		// Wrap node.WaitForMCP with retry logic to handle transient API errors (504, 503, etc.)
		// during cluster reconfiguration. The underlying node.WaitForMCP fails immediately on
		// API errors, but during TLS profile changes the API server can be temporarily overloaded.
		mcpErr := wait.PollUntilContextTimeout(ctx, 30*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
			err := node.WaitForMCP(ctx, clientset, mcp.Name, timeout, node.WaitMCPAllowDegraded())
			if err != nil {
				// Check if this is a transient API error (timeout, connection refused, etc.)
				errMsg := err.Error()
				if strings.Contains(errMsg, "Timeout") ||
					strings.Contains(errMsg, "timeout") ||
					strings.Contains(errMsg, "connection refused") ||
					strings.Contains(errMsg, "TLS handshake timeout") ||
					strings.Contains(errMsg, "i/o timeout") ||
					strings.Contains(errMsg, "context deadline exceeded") {
					e2e.Logf("Transient API error waiting for MCP %s (will retry): %v", mcp.Name, err)
					return false, nil
				}
				// Non-transient error, fail immediately
				return false, err
			}
			return true, nil
		})

		if mcpErr != nil {
			return fmt.Errorf("MCP %s did not complete: %w", mcp.Name, mcpErr)
		}
		e2e.Logf("MCP %s complete: %d/%d machines ready", mcp.Name, mcp.Status.ReadyMachineCount, mcp.Status.MachineCount)
	}

	// Wait for API server to stabilize after MCP rollout to prevent 504 timeouts
	// when starting subsequent heavy operations (e.g., another MCP rollout)
	e2e.Logf("Waiting for API server to stabilize after MCP rollout")
	consecutiveSuccesses := 0
	requiredSuccesses := 3

	stabilityErr := wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			// Lightweight health check - query MCPs to verify API server responsiveness
			_, err := client.MachineconfigurationV1().MachineConfigPools().List(ctx, metav1.ListOptions{Limit: 1})
			if err != nil {
				if apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) {
					e2e.Logf("API server not responsive yet, continuing to wait...")
					consecutiveSuccesses = 0
					return false, nil
				}
				return false, fmt.Errorf("unexpected error checking API server health: %w", err)
			}

			consecutiveSuccesses++
			e2e.Logf("API server health check passed (%d/%d)", consecutiveSuccesses, requiredSuccesses)

			return consecutiveSuccesses >= requiredSuccesses, nil
		})

	if stabilityErr != nil {
		return fmt.Errorf("API server did not stabilize after MCP rollout: %w", stabilityErr)
	}

	e2e.Logf("API server is stable")
	return nil
}

func waitForNodesStability(ctx context.Context, client kubernetes.Interface, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 30*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			e2e.Logf("Error getting nodes: %v", err)
			return false, nil
		}

		notReady := []string{}
		for _, node := range nodes.Items {
			if !isNodeReady(&node) {
				notReady = append(notReady, node.Name)
			}
		}

		if len(notReady) > 0 {
			e2e.Logf("Waiting for nodes to be ready: %s", strings.Join(notReady, ", "))
			return false, nil
		}

		e2e.Logf("All %d nodes are ready", len(nodes.Items))
		return true, nil
	})
}

func isNodeReady(node *corev1.Node) bool {
	return slices.ContainsFunc(node.Status.Conditions, func(condition corev1.NodeCondition) bool {
		return condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue
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
) error {
	apiserver, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get APIServer config: %w", err)
	}

	// Determine TLS 1.2 rejection expectation based on profile, adherence policy, and component type
	var expectTLS12Reject bool
	var testDescription string

	if apiserver.Spec.TLSSecurityProfile != nil {
		profileType := apiserver.Spec.TLSSecurityProfile.Type
		adherencePolicy := apiserver.Spec.TLSAdherence

		switch {
		case profileType == configv1.TLSProfileModernType && adherencePolicy == configv1.TLSAdherencePolicyStrictAllComponents:
			// Modern + StrictAllComponents: ALL components reject TLS 1.2
			expectTLS12Reject = true
			testDescription = "Modern + StrictAllComponents: TLS 1.3 only, TLS 1.2 should be rejected"

		case profileType == configv1.TLSProfileModernType && adherencePolicy == configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly:
			// Modern + LegacyAdheringComponentsOnly:
			// - Adhering components (Multus): reject TLS 1.2
			// - Non-adhering components (OVN, CNO, Network Console): accept TLS 1.2
			expectTLS12Reject = isAdheringComponent
			if isAdheringComponent {
				testDescription = "Modern + LegacyAdheringComponentsOnly (adhering component): TLS 1.3 only, TLS 1.2 should be rejected"
			} else {
				testDescription = "Modern + LegacyAdheringComponentsOnly (non-adhering component): both TLS 1.2 and TLS 1.3 should work"
			}

		case profileType == configv1.TLSProfileIntermediateType && adherencePolicy == configv1.TLSAdherencePolicyStrictAllComponents:
			// Intermediate + StrictAllComponents: ALL components accept TLS 1.2
			expectTLS12Reject = false
			testDescription = "Intermediate + StrictAllComponents: both TLS 1.2 and TLS 1.3 should work"

		default:
			// Default: accept both TLS 1.2 and 1.3
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
			// Always test TLS 1.3 first (should always succeed)
			tls13Config := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
			e2e.Logf("Testing TLS 1.3 on port %s (should succeed)", port)
			if err := exutil.CheckTLSConnection(localPort, tls13Config, nil); err != nil {
				return fmt.Errorf("TLS 1.3 test failed: %w", err)
			}
			e2e.Logf("TLS 1.3 test PASSED on port %s", port)

			// Skip TLS 1.2 test for networking-console-plugin only in Test 2 (Modern + StrictAllComponents)
			if componentName == "Network Console" &&
				apiserver.Spec.TLSSecurityProfile != nil &&
				apiserver.Spec.TLSSecurityProfile.Type == configv1.TLSProfileModernType &&
				apiserver.Spec.TLSAdherence == configv1.TLSAdherencePolicyStrictAllComponents {
				e2e.Logf("Skipping TLS 1.2 test for %s on port %s (Modern + StrictAllComponents)", componentName, port)
			} else {
				// Always test TLS 1.2 (but with different expectations)
				tls12Config := &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}
				if expectTLS12Reject {
					// TLS 1.2 should be rejected
					e2e.Logf("Testing TLS 1.2 on port %s (should be REJECTED)", port)
					if err := exutil.CheckTLSConnection(localPort, tls12Config, nil); err == nil {
						return fmt.Errorf("TLS 1.2 connection on port %s SUCCEEDED but should have been REJECTED", port)
					}
					e2e.Logf("TLS 1.2 correctly REJECTED on port %s", port)
				} else {
					// TLS 1.2 should succeed
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
		[]string{"6443", "8443"}, "Multus", true) // Multus is an adhering component
}

func VerifyOVNKubernetesTLSComplianceInPod(ctx context.Context, oc *exutil.CLI, configClient configv1client.Interface, k8sClient kubernetes.Interface, namespace, labelSelector string) error {
	return verifyTLSComplianceInPods(ctx, oc, configClient, k8sClient, namespace, labelSelector,
		[]string{"9108"}, "OVN-Kubernetes Control Plane", false) // OVN is a non-adhering component
}

func VerifyOVNKubernetesNodeTLSComplianceInPod(ctx context.Context, oc *exutil.CLI, configClient configv1client.Interface, k8sClient kubernetes.Interface, namespace, labelSelector string) error {
	return verifyTLSComplianceInPods(ctx, oc, configClient, k8sClient, namespace, labelSelector,
		[]string{"9103", "9105"}, "OVN-Kubernetes Node", false) // OVN is a non-adhering component
}

func VerifyCNOTLSComplianceInPod(ctx context.Context, oc *exutil.CLI, configClient configv1client.Interface, k8sClient kubernetes.Interface, namespace, labelSelector string) error {
	return verifyTLSComplianceInPods(ctx, oc, configClient, k8sClient, namespace, labelSelector,
		[]string{"9104", "9103", "9105"}, "CNO", false) // CNO is a non-adhering component
}

func VerifyNetworkConsoleTLSComplianceInPod(ctx context.Context, oc *exutil.CLI, configClient configv1client.Interface, k8sClient kubernetes.Interface, namespace, labelSelector string) error {
	return verifyTLSComplianceInPods(ctx, oc, configClient, k8sClient, namespace, labelSelector,
		[]string{"9443"}, "Network Console", false) // Network Console is a non-adhering component
}
