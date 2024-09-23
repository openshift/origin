package helpers

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"

	ign3types "github.com/coreos/ignition/v2/config/v3_4/types"
	"github.com/davecgh/go-spew/spew"
	mcfgv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
	"github.com/openshift/machine-config-operator/pkg/daemon/constants"
	"github.com/openshift/machine-config-operator/pkg/daemon/osrelease"
	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vincent-petithory/dataurl"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
)

type CleanupFuncs struct {
	funcs []func()
}

func (c *CleanupFuncs) Add(f func()) {
	c.funcs = append(c.funcs, f)
}

func (c *CleanupFuncs) Run() {
	for _, f := range c.funcs {
		f()
	}
}

func NewCleanupFuncs() CleanupFuncs {
	return CleanupFuncs{
		funcs: []func(){},
	}
}

func ApplyMC(t *testing.T, cs *framework.ClientSet, mc *mcfgv1.MachineConfig) func() {
	_, err := cs.MachineConfigs().Create(context.TODO(), mc, metav1.CreateOptions{})
	require.Nil(t, err)

	return func() {
		require.Nil(t, cs.MachineConfigs().Delete(context.TODO(), mc.Name, metav1.DeleteOptions{}))
	}
}

// Ensures that a given cleanup function only runs once; even if called
// multiple times.
func MakeIdempotent(f func()) func() {
	hasRun := false

	return func() {
		if !hasRun {
			f()
			hasRun = true
		}
	}
}

// Applies a MachineConfig to a given MachineConfigPool, if a MachineConfig is
// provided. If a MachineConfig is not provided (i.e., nil), it will skip the
// apply process and wait for the MachineConfigPool to include the "00-worker"
// MachineConfig. Returns a delete function that will no-op if a MachineConfig
// is not provided.
func maybeApplyMCAndWaitForPoolComplete(t *testing.T, cs *framework.ClientSet, poolName string, mc *mcfgv1.MachineConfig) func() {
	if mc != nil {
		deleteFunc := ApplyMC(t, cs, mc)
		WaitForConfigAndPoolComplete(t, cs, poolName, mc.Name)
		return func() {
			t.Logf("Deleting MachineConfig %s", mc.Name)
			deleteFunc()
		}
	}

	mcName := "00-worker"
	t.Logf("No MachineConfig provided, will wait for pool %q to include MachineConfig %q", poolName, mcName)
	WaitForConfigAndPoolComplete(t, cs, poolName, mcName)
	return func() {}
}

// Creates a pool, adds the provided node to it, applies the MachineConfig (if
// provided), waits for the rollout. Returns an idempotent function which can
// be used both for reverting and cleanup purposes.
func CreatePoolAndApplyMCToNode(t *testing.T, cs *framework.ClientSet, poolName string, node corev1.Node, mc *mcfgv1.MachineConfig) func() {
	workerMCPName := "worker"

	t.Logf("Setting up pool %s", poolName)

	unlabelFunc := LabelNode(t, cs, node, MCPNameToRole(poolName))
	deleteMCPFunc := CreateMCP(t, cs, poolName)

	t.Logf("Target Node: %s", node.Name)

	mcDeleteFunc := maybeApplyMCAndWaitForPoolComplete(t, cs, poolName, mc)

	mcpMCName := GetMcName(t, cs, poolName)
	require.Nil(t, WaitForNodeConfigChange(t, cs, node, mcpMCName))

	return MakeIdempotent(func() {
		t.Logf("Cleaning up MCP %s", poolName)
		t.Logf("Removing label %s from node %s", MCPNameToRole(poolName), node.Name)
		unlabelFunc()

		workerMC := GetMcName(t, cs, workerMCPName)

		// Wait for the worker pool to catch up with the deleted label
		time.Sleep(5 * time.Second)

		t.Logf("Waiting for %s pool to finish applying %s", workerMCPName, workerMC)
		require.Nil(t, WaitForPoolComplete(t, cs, workerMCPName, workerMC))

		t.Logf("Ensuring node has %s pool config before deleting pool %s", workerMCPName, poolName)
		require.Nil(t, WaitForNodeConfigChange(t, cs, node, workerMC))

		t.Logf("Deleting MCP %s", poolName)
		deleteMCPFunc()

		mcDeleteFunc()
	})
}

// Creates a MachineConfigPool, waits for the MachineConfigPool to inherit the
// worker MachineConfig "00-worker", and assigns a node to it. Returns an
// idempotent function which can be used for both reverting and cleanup
// purposes.
func CreatePoolWithNode(t *testing.T, cs *framework.ClientSet, poolName string, node corev1.Node) func() {
	return CreatePoolAndApplyMCToNode(t, cs, poolName, node, nil)
}

// Selects a random node from the "worker" pool, creates a MachineConfigPool,
// and applies a MachineConfig to it. Returns an idempotent function which can
// be used for both reverting and cleanup purposes.
func CreatePoolAndApplyMC(t *testing.T, cs *framework.ClientSet, poolName string, mc *mcfgv1.MachineConfig) func() {
	node := GetRandomNode(t, cs, "worker")
	return CreatePoolAndApplyMCToNode(t, cs, poolName, node, mc)
}

// GetMcName returns the current configuration name of the machine config pool poolName
func GetMcName(t *testing.T, cs *framework.ClientSet, poolName string) string {
	// grab the initial machineconfig used by the worker pool
	// this MC is gonna be the one which is going to be reapplied once the previous MC is deleted
	mcp, err := cs.MachineConfigPools().Get(context.TODO(), poolName, metav1.GetOptions{})
	require.Nil(t, err)
	return mcp.Status.Configuration.Name
}

// WaitForConfigAndPoolComplete is a helper function that gets a renderedConfig and waits for its pool to complete.
// The return value is the final rendered config.
func WaitForConfigAndPoolComplete(t *testing.T, cs *framework.ClientSet, pool, mcName string) string {
	config, err := WaitForRenderedConfig(t, cs, pool, mcName)
	require.Nil(t, err, "failed to render machine config %s from pool %s", mcName, pool)
	err = WaitForPoolComplete(t, cs, pool, config)
	require.Nil(t, err, "pool %s did not update to config %s", pool, config)
	return config
}

// WaitForRenderedConfig polls a MachineConfigPool until it has
// included the given mcName in its config, and returns the new
// rendered config name.
func WaitForRenderedConfig(t *testing.T, cs *framework.ClientSet, pool, mcName string) (string, error) {
	return WaitForRenderedConfigs(t, cs, pool, mcName)
}

// WaitForRenderedConfigs polls a MachineConfigPool until it has
// included the given mcNames in its config, and returns the new
// rendered config name.
func WaitForRenderedConfigs(t *testing.T, cs *framework.ClientSet, pool string, mcNames ...string) (string, error) {
	var renderedConfig string
	startTime := time.Now()
	found := make(map[string]bool)

	ctx := context.Background()

	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		// Set up the list
		for _, name := range mcNames {
			found[name] = false
		}

		// Update found based on the MCP
		mcp, err := cs.MachineConfigPools().Get(ctx, pool, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, mc := range mcp.Spec.Configuration.Source {
			if _, ok := found[mc.Name]; ok {
				found[mc.Name] = true
			}
		}

		// If any are still false, then they weren't included in the MCP
		for _, nameFound := range found {
			if !nameFound {
				return false, nil
			}
		}

		// All the required names were found
		renderedConfig = mcp.Spec.Configuration.Name
		return true, nil
	}); err != nil {
		return "", fmt.Errorf("machine configs %v hasn't been picked by pool %s (waited %s): %w", notFoundNames(found), pool, time.Since(startTime), err)
	}
	t.Logf("Pool %s has rendered configs %v with %s (waited %v)", pool, mcNames, renderedConfig, time.Since(startTime))
	return renderedConfig, nil
}

func notFoundNames(foundNames map[string]bool) []string {
	out := []string{}
	for name, found := range foundNames {
		if !found {
			out = append(out, name)
		}
	}
	return out
}

// WaitForPoolComplete polls a pool until it has completed an update to target
func WaitForPoolComplete(t *testing.T, cs *framework.ClientSet, pool, target string) error {
	startTime := time.Now()
	ctx := context.TODO()

	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 20*time.Minute, false, func(ctx context.Context) (bool, error) {
		mcp, err := cs.MachineConfigPools().Get(ctx, pool, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if mcp.Status.Configuration.Name != target {
			return false, nil
		}
		if mcfgv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdated) {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("pool %s didn't report %s to updated (waited %s): %w", pool, target, time.Since(startTime), err)
	}
	t.Logf("Pool %s has completed %s (waited %v)", pool, target, time.Since(startTime))
	return nil
}

func WaitForNodeConfigChange(t *testing.T, cs *framework.ClientSet, node corev1.Node, mcName string) error {
	startTime := time.Now()
	ctx := context.TODO()

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 20*time.Minute, true, func(ctx context.Context) (bool, error) {
		n, err := cs.CoreV1Interface.Nodes().Get(ctx, node.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		current := n.Annotations[constants.CurrentMachineConfigAnnotationKey]
		desired := n.Annotations[constants.DesiredMachineConfigAnnotationKey]

		state := n.Annotations[constants.MachineConfigDaemonStateAnnotationKey]

		return current == desired && desired == mcName && state == constants.MachineConfigDaemonStateDone, nil
	})

	if err != nil {
		return fmt.Errorf("node config change did not occur (waited %v): %w", time.Since(startTime), err)
	}

	t.Logf("Node %s changed config to %s (waited %v)", node.Name, mcName, time.Since(startTime))
	return nil
}

// WaitForPoolComplete polls a pool until it has completed any update
func WaitForPoolCompleteAny(t *testing.T, cs *framework.ClientSet, pool string) error {
	ctx := context.TODO()

	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		mcp, err := cs.MachineConfigPools().Get(ctx, pool, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		// wait until the cluter is back to normal (for the next test at least)
		if mcfgv1.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdated) {
			return true, nil
		}
		return false, nil
	}); err != nil {
		t.Errorf("Machine config pool did not complete an update: %v", err)
	}
	return nil
}

// WaitForPausedConfig waits for configuration to be pending in a paused pool
func WaitForPausedConfig(t *testing.T, cs *framework.ClientSet, pool string) error {
	ctx := context.TODO()

	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		mcp, err := cs.MachineConfigPools().Get(ctx, pool, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		// paused == not updated, not updating, not degraded
		if mcfgv1.IsMachineConfigPoolConditionFalse(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdated) &&
			mcfgv1.IsMachineConfigPoolConditionFalse(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdating) &&
			mcfgv1.IsMachineConfigPoolConditionFalse(mcp.Status.Conditions, mcfgv1.MachineConfigPoolDegraded) {
			return true, nil
		}
		return false, nil
	}); err != nil {
		t.Errorf("Machine config pool never entered the state where configuration was waiting behind pause: %v", err)
	}
	return nil
}

// WaitForPodStart waits for a pod with the given name prefix in the given namespace to start.
func WaitForPodStart(cs *framework.ClientSet, podPrefix, namespace string) error {
	ctx := context.TODO()

	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		podList, err := cs.CoreV1Interface.Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, pod := range podList.Items {
			if strings.HasPrefix(pod.Name, podPrefix) {
				for _, condition := range pod.Status.Conditions {
					if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
						return true, nil
					}
				}
			}
		}
		return false, nil
	})
}

// WaitForPodStop waits for a pod with the given name prefix in the given namespace to stop (i.e., to be deleted).
func WaitForPodStop(cs *framework.ClientSet, podPrefix, namespace string) error {
	ctx := context.TODO()

	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		podList, err := cs.CoreV1Interface.Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, pod := range podList.Items {
			if strings.HasPrefix(pod.Name, podPrefix) {
				// Pod with prefix still exists, so we return false
				return false, nil
			}
		}
		// If we reached here, it means no pod with the given prefix exists, so we return true
		return true, nil
	})
}

// CheckDeploymentExists checks if a deployment with the given name in the given namespace exists.
func CheckDeploymentExists(cs *framework.ClientSet, name, namespace string) (bool, error) {
	ctx := context.TODO()
	_, err := cs.Deployments(namespace).Get(ctx, name, metav1.GetOptions{})

	if err == nil {
		return true, nil
	}

	if errors.IsNotFound(err) {
		return false, nil
	}

	return false, err
}

// GetMonitoringToken retrieves the token from the openshift-monitoring secrets in the prometheus-k8s namespace.
// It is equivalent to "oc sa get-token prometheus-k8s -n openshift-monitoring"
func GetMonitoringToken(_ *testing.T, cs *framework.ClientSet) (string, error) {
	token, err := cs.
		ServiceAccounts("openshift-monitoring").
		CreateToken(
			context.TODO(),
			"prometheus-k8s",
			&authenticationv1.TokenRequest{
				Spec: authenticationv1.TokenRequestSpec{},
			},
			metav1.CreateOptions{},
		)
	if err != nil {
		return "", fmt.Errorf("Could not request openshift-monitoring token")
	}
	return token.Status.Token, nil
}

// WaitForCertStatusToChange queries a controllerconfig until the ControllerCertificates changes.
func WaitForCertStatusToChange(t *testing.T, cs *framework.ClientSet, oldData string) error {
	ctx := context.TODO()
	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		controllerConfig, err := cs.ControllerConfigs().Get(context.TODO(), "machine-config-controller", metav1.GetOptions{})
		require.Nil(t, err)

		// shows us that the status is updating

		for _, cert := range controllerConfig.Status.ControllerCertificates {
			t.Logf("comparing: %s to %s\n", oldData, cert.Subject)
			t.Logf("also, other data: %s %s\n", cert.BundleFile, cert.Subject)
			if cert.BundleFile == "KubeAPIServerServingCAData" {
				if oldData != cert.Subject {
					return true, nil
				}
			}
		}

		return false, nil
	}); err != nil {
		return err
	}

	return nil
}

func WaitForCADataToAppear(t *testing.T, cs *framework.ClientSet) error {
	err := wait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		controllerConfig, err := cs.ControllerConfigs().Get(context.TODO(), "machine-config-controller", metav1.GetOptions{})
		require.Nil(t, err)
		nodes, err := GetNodesByRole(cs, "worker")
		require.Nil(t, err)
		for _, cert := range controllerConfig.Spec.ImageRegistryBundleUserData {
			foundCA, _ := ExecCmdOnNodeWithError(cs, nodes[0], "ls", canonicalizeNodeFilePath(filepath.Join("/etc/docker/certs.d", cert.File)))
			if strings.Contains(foundCA, "ca.crt") {
				return true, nil
			}
		}
		return false, nil
	})
	return err
}

// WaitForMCDToSyncCert waits for the MCD to write annotation on the latest controllerconfig resourceVersion,
// to indicate that is has completed the certificate write
func WaitForMCDToSyncCert(t *testing.T, cs *framework.ClientSet, node corev1.Node, resourceVersion string) error {
	ctx := context.TODO()

	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 4*time.Minute, true, func(ctx context.Context) (bool, error) {
		n, err := cs.CoreV1Interface.Nodes().Get(ctx, node.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if n.Annotations[constants.ControllerConfigResourceVersionKey] == resourceVersion {
			return true, nil
		}
		return false, nil
	}); err != nil {
		t.Errorf("Machine config daemon never wrote updated controllerconfig resourceVersion annotation: %v", err)
	}
	return nil
}

// ForceKubeApiserverCertificateRotation sets the kube-apiserver-to-kubelet-signer's not-after date to nil, which causes the
// apiserver to rotate it
func ForceKubeApiserverCertificateRotation(cs *framework.ClientSet) error {
	// Take note that the slash had to be encoded as ~1 because it's a reference: https://www.rfc-editor.org/rfc/rfc6901#section-3
	certPatch := fmt.Sprintf(`[{"op":"replace","path":"/metadata/annotations/auth.openshift.io~1certificate-not-after","value": null }]`)
	_, err := cs.Secrets("openshift-kube-apiserver-operator").Patch(context.TODO(), "kube-apiserver-to-kubelet-signer", types.JSONPatchType, []byte(certPatch), metav1.PatchOptions{})
	return err
}

// ForceImageRegistryCertRotationCertificateRotation sets the imae registry cert's not-after date to nil, which causes the
// apiserver to rotate it
func ForceImageRegistryCertRotationCertificateRotation(cs *framework.ClientSet) error {
	cfg, err := cs.ConfigV1Interface.Images().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}
	// Take note that the slash had to be encoded as ~1 because it's a reference: https://www.rfc-editor.org/rfc/rfc6901#section-3
	certPatch := fmt.Sprintf(`[{"op":"replace","path":"/metadata/annotations/auth.openshift.io~1certificate-not-after","value": null }]`)
	_, err = cs.Secrets("openshift-config").Patch(context.TODO(), cfg.Spec.AdditionalTrustedCA.Name, types.JSONPatchType, []byte(certPatch), metav1.PatchOptions{})
	return err
}

// GetKubeletCABundleFromConfigmap fetches the latest kubelet ca bundle data from
func GetKubeletCABundleFromConfigmap(cs *framework.ClientSet) (string, error) {
	certBundle, err := cs.ConfigMaps("openshift-config-managed").Get(context.TODO(), "kube-apiserver-client-ca", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Could not get in-cluster kube-apiserver-client-ca configmap")
	}
	if cert, ok := certBundle.Data["ca-bundle.crt"]; ok {
		return cert, nil
	}
	return "", fmt.Errorf("Could not find ca-bundle")
}

// Applies a given label to a node and returns an unlabel function.
func LabelNode(t *testing.T, cs *framework.ClientSet, node corev1.Node, label string) func() {
	ctx := context.Background()

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		n, err := cs.CoreV1Interface.Nodes().Get(ctx, node.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		n.Labels[label] = ""
		_, err = cs.CoreV1Interface.Nodes().Update(ctx, n, metav1.UpdateOptions{})
		return err
	})

	require.Nil(t, err, "unable to label %s node %s with infra: %s", label, node.Name, err)

	return MakeIdempotent(func() {

		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			n, err := cs.CoreV1Interface.Nodes().Get(ctx, node.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			delete(n.Labels, label)
			_, err = cs.CoreV1Interface.Nodes().Update(ctx, n, metav1.UpdateOptions{})
			return err
		})
		require.Nil(t, err, "unable to remove label %q from node %q: %s", label, node.Name, err)
	})
}

// Gets a random node from a given pool
func GetRandomNode(t *testing.T, cs *framework.ClientSet, pool string) corev1.Node {
	nodes, err := GetNodesByRole(cs, pool)
	require.Nil(t, err)
	require.NotEmpty(t, nodes)

	// Disable gosec here to avoid throwing
	// G404: Use of weak random number generator (math/rand instead of crypto/rand)
	// #nosec
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	return nodes[rnd.Intn(len(nodes))]
}

// LabelRandomNodeFromPool gets all nodes in pool and chooses one at random to label
func LabelRandomNodeFromPool(t *testing.T, cs *framework.ClientSet, pool, label string) func() {
	infraNode := GetRandomNode(t, cs, pool)
	return LabelNode(t, cs, infraNode, label)
}

// GetSingleNodeByRoll gets all nodes by role pool, and asserts there should only be one
func GetSingleNodeByRole(t *testing.T, cs *framework.ClientSet, role string) corev1.Node {
	nodes, err := GetNodesByRole(cs, role)
	require.Nil(t, err)
	require.Len(t, nodes, 1)
	return nodes[0]
}

// GetNodesByRole gets all nodes labeled with role role
func GetNodesByRole(cs *framework.ClientSet, role string) ([]corev1.Node, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{fmt.Sprintf("node-role.kubernetes.io/%s", role): ""}).String(),
	}
	nodes, err := cs.CoreV1Interface.Nodes().List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

// CreateMCP create a machine config pool with name mcpName
// it will also use mcpName as the label selector, so any node you want to be included
// in the pool should have a label node-role.kubernetes.io/mcpName = ""
func CreateMCP(t *testing.T, cs *framework.ClientSet, mcpName string) func() {
	infraMCP := &mcfgv1.MachineConfigPool{}
	infraMCP.Name = mcpName
	nodeSelector := metav1.LabelSelector{}
	infraMCP.Spec.NodeSelector = &nodeSelector
	infraMCP.Spec.NodeSelector.MatchLabels = make(map[string]string)
	infraMCP.Spec.NodeSelector.MatchLabels[MCPNameToRole(mcpName)] = ""
	mcSelector := metav1.LabelSelector{}
	infraMCP.Spec.MachineConfigSelector = &mcSelector
	infraMCP.Spec.MachineConfigSelector.MatchExpressions = []metav1.LabelSelectorRequirement{
		{
			Key:      mcfgv1.MachineConfigRoleLabelKey,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{"worker", mcpName},
		},
	}
	infraMCP.ObjectMeta.Labels = make(map[string]string)
	infraMCP.ObjectMeta.Labels[mcpName] = ""

	_, err := cs.MachineConfigPools().Create(context.TODO(), infraMCP, metav1.CreateOptions{})
	switch {
	case err == nil:
		t.Logf("Created MachineConfigPool %q", mcpName)
	case errors.IsAlreadyExists(err):
		t.Logf("Reusing existing MachineConfigPool %q", mcpName)
	default:
		t.Fatalf("Could not create MachineConfigPool %q: %s", mcpName, err)
	}

	return func() {
		err := cs.MachineConfigPools().Delete(context.TODO(), mcpName, metav1.DeleteOptions{})
		require.Nil(t, err)
		t.Logf("Deleted MachineConfigPool %q", mcpName)
	}
}

type SSHPaths struct {
	// The path where SSH keys are expected to be found.
	Expected string
	// The path where SSH keys are *not* expected to be found.
	NotExpected string
}

// Determines where to expect SSH keys for the core user on a given node based upon the node's OS.
func GetSSHPaths(os osrelease.OperatingSystem) SSHPaths {
	if os.IsEL9() || os.IsSCOS() || os.IsFCOS() {
		return SSHPaths{
			Expected:    constants.RHCOS9SSHKeyPath,
			NotExpected: constants.RHCOS8SSHKeyPath,
		}
	}

	return SSHPaths{
		Expected:    constants.RHCOS8SSHKeyPath,
		NotExpected: constants.RHCOS9SSHKeyPath,
	}
}

// MCPNameToRole converts a mcpName to a node role label
func MCPNameToRole(mcpName string) string {
	return fmt.Sprintf("node-role.kubernetes.io/%s", mcpName)
}

// CreateMC creates a machine config object with name and role
func CreateMC(name, role string) *mcfgv1.MachineConfig {
	return NewMachineConfig(name, MCLabelForRole(role), "", nil)
}

// Asserts that a given file is present on the underlying node.
func AssertFileOnNode(t *testing.T, cs *framework.ClientSet, node corev1.Node, path string) bool {
	t.Helper()

	path = canonicalizeNodeFilePath(path)

	out, err := ExecCmdOnNodeWithError(cs, node, "stat", path)

	return assert.NoError(t, err, "expected to find file %s on %s, got:\n%s\nError: %s", path, node.Name, out, err)
}

// Asserts that a given file is *not* present on the underlying node.
func AssertFileNotOnNode(t *testing.T, cs *framework.ClientSet, node corev1.Node, path string) bool {
	t.Helper()

	path = canonicalizeNodeFilePath(path)

	out, err := ExecCmdOnNodeWithError(cs, node, "stat", path)

	return assert.Error(t, err, "expected not to find file %s on %s, got:\n%s", path, node.Name, out) &&
		assert.Contains(t, out, "No such file or directory", "expected command output to contain 'No such file or directory', got: %s", out)
}

// Adds the /rootfs onto a given file path, if not already present.
func canonicalizeNodeFilePath(path string) string {
	rootfs := "/rootfs"

	if !strings.HasPrefix(path, rootfs) {
		return filepath.Join(rootfs, path)
	}

	return path
}

// ExecCmdOnNode finds a node's mcd, and oc rsh's into it to execute a command on the node
// all commands should use /rootfs as root
func ExecCmdOnNode(t *testing.T, cs *framework.ClientSet, node corev1.Node, subArgs ...string) string {
	t.Helper()

	cmd, err := execCmdOnNode(cs, node, subArgs...)
	require.Nil(t, err, "could not prepare to exec cmd %v on node %s: %s", subArgs, node.Name, err)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	require.Nil(t, err, "failed to exec cmd %v on node %s: %s", subArgs, node.Name, string(out))
	return string(out)
}

// ExecCmdOnNodeWithError behaves like ExecCmdOnNode, with the exception that
// any errors are returned to the caller for inspection. This allows one to
// execute a command that is expected to fail; e.g., stat /nonexistant/file.
func ExecCmdOnNodeWithError(cs *framework.ClientSet, node corev1.Node, subArgs ...string) (string, error) {
	cmd, err := execCmdOnNode(cs, node, subArgs...)
	if err != nil {
		return "", err
	}

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ExecCmdOnNode finds a node's mcd, and oc rsh's into it to execute a command on the node
// all commands should use /rootfs as root
func execCmdOnNode(cs *framework.ClientSet, node corev1.Node, subArgs ...string) (*exec.Cmd, error) {
	// Check for an oc binary in $PATH.
	path, err := exec.LookPath("oc")
	if err != nil {
		return nil, fmt.Errorf("could not locate oc command: %w", err)
	}

	// Get the kubeconfig file path
	kubeconfig, err := cs.GetKubeconfig()
	if err != nil {
		return nil, fmt.Errorf("could not get kubeconfig: %w", err)
	}

	mcd, err := mcdForNode(cs, &node)
	if err != nil {
		return nil, fmt.Errorf("could not get MCD for node %s: %w", node.Name, err)
	}

	mcdName := mcd.ObjectMeta.Name

	entryPoint := path
	args := []string{"rsh",
		"-n", "openshift-machine-config-operator",
		"-c", "machine-config-daemon",
		mcdName}
	args = append(args, subArgs...)

	cmd := exec.Command(entryPoint, args...)
	// If one passes a path to a kubeconfig via NewClientSet instead of setting
	// $KUBECONFIG, oc will be unaware of it. To remedy, we explicitly set
	// KUBECONFIG to the value held by the clientset.
	cmd.Env = append(cmd.Env, "KUBECONFIG="+kubeconfig)
	return cmd, nil
}

// IsOKDCluster checks whether the Upstream field on the CV spec references an OKD release controller
func IsOKDCluster(cs *framework.ClientSet) (bool, error) {
	cv, err := cs.ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	// TODO: Adjust this as OKD becomes available for different platforms, e.g., arm64.
	okdReleaseControllers := sets.NewString("https://amd64.origin.releases.ci.openshift.org/graph")
	return okdReleaseControllers.Has(string(cv.Spec.Upstream)), nil
}

func MCLabelForRole(role string) map[string]string {
	mcLabels := make(map[string]string)
	mcLabels[mcfgv1.MachineConfigRoleLabelKey] = role
	return mcLabels
}

func MCLabelForWorkers() map[string]string {
	return MCLabelForRole("worker")
}

// TODO consider also testing for Ign2
// func createIgn2File(path, content, fs string, mode int) ign2types.File {
// 	return ign2types.File{
// 		FileEmbedded1: ign2types.FileEmbedded1{
// 			Contents: ign2types.FileContents{
// 				Source: content,
// 			},
// 			Mode: &mode,
// 		},
// 		Node: ign2types.Node{
// 			Filesystem: fs,
// 			Path:       path,
// 			User: &ign2types.NodeUser{
// 				Name: "root",
// 			},
// 		},
// 	}
// }

// Creates an Ign3 file whose contents are gzipped and encoded according to
// https://datatracker.ietf.org/doc/html/rfc2397
func CreateGzippedIgn3File(path, content string, mode int) (ign3types.File, error) {
	ign3File := ign3types.File{}

	buf := bytes.NewBuffer([]byte{})

	gzipWriter := gzip.NewWriter(buf)
	if _, err := gzipWriter.Write([]byte(content)); err != nil {
		return ign3File, err
	}

	if err := gzipWriter.Close(); err != nil {
		return ign3File, err
	}

	ign3File = CreateEncodedIgn3File(path, buf.String(), mode)
	ign3File.Contents.Compression = StrToPtr("gzip")

	return ign3File, nil
}

// Creates an Ign3 file whose contents are encoded according to
// https://datatracker.ietf.org/doc/html/rfc2397
func CreateEncodedIgn3File(path, content string, mode int) ign3types.File {
	encoded := dataurl.EncodeBytes([]byte(content))

	return CreateIgn3File(path, encoded, mode)
}

func CreateIgn3File(path, content string, mode int) ign3types.File {
	return ign3types.File{
		FileEmbedded1: ign3types.FileEmbedded1{
			Contents: ign3types.Resource{
				Source: &content,
			},
			Mode: &mode,
		},
		Node: ign3types.Node{
			Path: path,
			User: ign3types.NodeUser{
				Name: StrToPtr("root"),
			},
		},
	}
}

func MCDForNode(cs *framework.ClientSet, node *corev1.Node) (*corev1.Pod, error) {
	return mcdForNode(cs, node)
}

// Returns the node's uptime expressed as the total number of seconds the
// system has been up.
// See: https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/6/html/deployment_guide/s2-proc-uptime
func GetNodeUptime(t *testing.T, cs *framework.ClientSet, node corev1.Node) float64 {
	output := ExecCmdOnNode(t, cs, node, "cat", "/rootfs/proc/uptime")
	uptimeString := strings.Split(output, " ")[0]
	uptime, err := strconv.ParseFloat(uptimeString, 64)
	require.Nil(t, err)
	return uptime
}

// Asserts that a node has rebooted, given an initial uptime value.
//
// To Use:
//
// initialUptime := GetNodeUptime(t, cs, node)
// // do stuff that should cause a reboot
// AssertNodeReboot(t, cs, node, initialUptime)
//
// Returns the result of the assertion as is the convention for the testify/assert
func AssertNodeReboot(t *testing.T, cs *framework.ClientSet, node corev1.Node, initialUptime float64) bool {
	current := GetNodeUptime(t, cs, node)
	return assert.Greater(t, initialUptime, current, "node %s did not reboot as expected; initial uptime: %d, current uptime: %d", node.Name, initialUptime, current)
}

// Asserts that a node has not rebooted, given an initial uptime value.
//
// To Use:
//
// initialUptime := GetNodeUptime(t, cs, node)
// // do stuff that should not cause a reboot
// AssertNodeNotReboot(t, cs, node, initialUptime)
//
// Returns the result of the assertion as is the convention for the testify/assert
func AssertNodeNotReboot(t *testing.T, cs *framework.ClientSet, node corev1.Node, initialUptime float64) bool {
	current := GetNodeUptime(t, cs, node)
	return assert.Greater(t, current, initialUptime, "node %s rebooted unexpectedly; initial uptime: %d, current uptime: %d", node.Name, initialUptime, current)
}

// Overrides a global path variable by appending its original value onto a
// tempdir created by t.TempDir(). Returns a function that reverts it back. If
// you wanted to override the origParentDirPath, you would do something like
// this:
//
// cleanupFunc := OverrideGlobalPathVar(t, "origParentDirPath", &origParentDirPath)
// defer cleanupFunc()
//
// NOTE: testing.T will clean up any tempdirs it creates after each test, so
// there is no need to do something like defer os.RemoveAll().
func OverrideGlobalPathVar(t *testing.T, name string, val *string) func() {
	oldValue := *val

	newValue := filepath.Join(t.TempDir(), oldValue)

	*val = newValue

	t.Logf("original value of %s (%s) overridden with %s", name, oldValue, newValue)

	return func() {
		t.Logf("original value of %s restored to %s", name, oldValue)
		*val = oldValue
	}
}

type NodeOSRelease struct {
	// The contents of the /etc/os-release file
	EtcContent string
	// The contents of the /usr/lib/os-release file
	LibContent string
	// The parsed contents
	OS osrelease.OperatingSystem
}

// Retrieves the /etc/os-release and /usr/lib/os-release file contents on a
// given node and parses it through the OperatingSystem code.
func GetOSReleaseForNode(t *testing.T, cs *framework.ClientSet, node corev1.Node) NodeOSRelease {
	etcOSReleaseContent := ExecCmdOnNode(t, cs, node, "cat", filepath.Join("/rootfs", osrelease.EtcOSReleasePath))
	libOSReleaseContent := ExecCmdOnNode(t, cs, node, "cat", filepath.Join("/rootfs", osrelease.LibOSReleasePath))

	os, err := osrelease.LoadOSRelease(etcOSReleaseContent, libOSReleaseContent)
	require.NoError(t, err)

	return NodeOSRelease{
		EtcContent: etcOSReleaseContent,
		LibContent: libOSReleaseContent,
		OS:         os,
	}
}

func mcdForNode(cs *framework.ClientSet, node *corev1.Node) (*corev1.Pod, error) {
	// find the MCD pod that has spec.nodeNAME = node.Name and get its name:
	listOptions := metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node.Name}).String(),
	}
	listOptions.LabelSelector = labels.SelectorFromSet(labels.Set{"k8s-app": "machine-config-daemon"}).String()

	mcdList, err := cs.Pods("openshift-machine-config-operator").List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}
	if len(mcdList.Items) != 1 {
		if len(mcdList.Items) == 0 {
			return nil, fmt.Errorf("failed to find MCD for node %s", node.Name)
		}
		return nil, fmt.Errorf("too many (%d) MCDs for node %s", len(mcdList.Items), node.Name)
	}
	return &mcdList.Items[0], nil
}

func DumpNodesAndPools(t *testing.T, nodes []*corev1.Node, pools []*mcfgv1.MachineConfigPool) {
	t.Helper()

	sb := &strings.Builder{}

	fmt.Fprintln(sb, "")
	fmt.Fprintln(sb, "===== Nodes =====")
	for _, node := range nodes {
		fmt.Fprintln(sb, dumpNode(node, false))
		fmt.Fprintln(sb, "=========")
	}
	fmt.Fprintln(sb, "")
	fmt.Fprintln(sb, "====== Pools ======")

	for _, pool := range pools {
		fmt.Fprintln(sb, dumpPool(pool, false))
		fmt.Fprintln(sb, "===================")
	}

	t.Log(sb.String())
}

func dumpNode(node *corev1.Node, silentNil bool) string {
	sb := &strings.Builder{}

	fmt.Fprintln(sb, "")

	if node != nil {
		if node.Name != "" {
			fmt.Fprintf(sb, "Node Name: %s\n", node.Name)
		}
		fmt.Fprintln(sb, "")
		fmt.Fprintf(sb, "Node Annotations: %s", spew.Sdump(node.Annotations))
		fmt.Fprintln(sb, "")
		fmt.Fprintf(sb, "Node Labels: %s", spew.Sdump(node.Labels))
		fmt.Fprintln(sb, "")

		conditions := "<empty>"
		if len(node.Status.Conditions) != 0 {
			conditions = spew.Sdump(node.Status.Conditions)
		}
		fmt.Fprintf(sb, "Node Conditions: %s\n", conditions)
	} else if !silentNil {
		fmt.Fprintln(sb, "Node was nil")
	}

	return sb.String()
}

func dumpPool(pool *mcfgv1.MachineConfigPool, silentNil bool) string {
	sb := &strings.Builder{}

	fmt.Fprintln(sb, "")

	if pool != nil {
		if pool.Name != "" {
			fmt.Fprintf(sb, "Pool Name: %q\n", pool.Name)
		}
		fmt.Fprintln(sb, "")
		fmt.Fprintf(sb, "Pool Annotations: %s", spew.Sdump(pool.Annotations))
		fmt.Fprintln(sb, "")
		fmt.Fprintf(sb, "Pool Labels: %s", spew.Sdump(pool.Labels))
		fmt.Fprintln(sb, "")
		fmt.Fprintf(sb, "Pool Config: %q\n", pool.Spec.Configuration.Name)
	} else if !silentNil {
		fmt.Fprintln(sb, "Pool was nil")
	}

	return sb.String()
}
