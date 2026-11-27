package nvidia

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	// GPU Operator namespace (used for validation only)
	gpuOperatorNamespace = "nvidia-gpu-operator"

	// DRA Driver constants
	draDriverNamespace = "nvidia-dra-driver"
	draDriverRelease   = "nvidia-dra-driver"
	// The driver moved to kubernetes-sigs/dra-driver-nvidia-gpu and switched from
	// CalVer to SemVer; charts are published as OCI artifacts. The legacy NGC repo
	// (helm.ngc.nvidia.com) stops at 25.12.0, which vendors go-nvlib v0.9.0 and
	// cannot enumerate GPU architectures newer than early 2026 (the kubelet plugin's
	// gpus container crash-loops and no gpu.nvidia.com ResourceSlices are published).
	draDriverChart          = "oci://registry.k8s.io/dra-driver-nvidia/charts/dra-driver-nvidia-gpu"
	draDriverDefaultVersion = "0.5.1"
	// The OCI chart defaults its resource names to dra-driver-nvidia-gpu; keep the
	// legacy names so the readiness checks and cleanup below still find the driver.
	draDriverNameOverride    = "nvidia-dra-driver-gpu"
	draDriverController      = "nvidia-dra-driver-gpu-controller"
	draDriverKubeletPlugin   = "nvidia-dra-driver-gpu-kubelet-plugin"
	draDriverControllerSA    = "nvidia-dra-driver-gpu-service-account-controller"
	draDriverKubeletPluginSA = "nvidia-dra-driver-gpu-service-account-kubeletplugin"
	draDriverComputeDomainSA = "compute-domain-daemon-service-account"

	// The DRA driver name and the DeviceClass the driver publishes for whole GPUs. These are
	// part of the driver's API rather than of any one deployment, so they identify the driver
	// whether it came from this suite's Helm install or from the GPU Operator.
	nvidiaDRADriverName      = "gpu.nvidia.com"
	nvidiaDRADeviceClassName = "gpu.nvidia.com"
)

// PrerequisitesInstaller validates GPU Operator and manages DRA driver installation
type PrerequisitesInstaller struct {
	client    kubernetes.Interface
	framework *framework.Framework
}

// NewPrerequisitesInstaller creates a new installer
func NewPrerequisitesInstaller(f *framework.Framework) *PrerequisitesInstaller {
	return &PrerequisitesInstaller{
		client:    f.ClientSet,
		framework: f,
	}
}

// InstallAll makes sure a gpu.nvidia.com DRA driver is serving the cluster, installing one with
// Helm only when none is.
func (pi *PrerequisitesInstaller) InstallAll(ctx context.Context) error {
	framework.Logf("=== Validating NVIDIA GPU Stack Prerequisites ===")

	// GPU Operator v26.7 and later deploy the DRA driver themselves through GPUCluster. When a
	// driver is already publishing GPUs there is nothing left to install, and nothing worth
	// checking about how it got there.
	if pi.IsDRADriverInstalled(ctx) {
		framework.Logf("A %s DRA driver is already serving the cluster; skipping installation", nvidiaDRADriverName)
		framework.Logf("=== All prerequisites validated and ready ===")
		return nil
	}

	// From here on this suite installs the driver itself, which needs the GPU Operator's NVIDIA
	// driver and container toolkit already in place.
	framework.Logf("No %s DRA driver found; validating the GPU Operator before installing one", nvidiaDRADriverName)
	if !pi.IsGPUOperatorInstalled(ctx) {
		return fmt.Errorf("GPU Operator not found - must be pre-installed on the cluster. " +
			"Install GPU Operator via OLM before running these tests")
	}
	framework.Logf("GPU Operator detected")

	if err := pi.WaitForGPUOperator(ctx, 5*time.Minute); err != nil {
		return fmt.Errorf("GPU Operator not ready: %w. Ensure GPU Operator is fully deployed", err)
	}
	framework.Logf("GPU Operator is ready")

	if err := pi.ensureHelm(ctx); err != nil {
		return fmt.Errorf("helm not available: %w", err)
	}

	// Pinned version, pulled directly from the OCI registry.
	if err := pi.InstallDRADriver(ctx); err != nil {
		framework.Logf("DRA Driver installation failed, rolling back cluster mutations...")
		pi.RollbackDRAMutations(ctx)
		return fmt.Errorf("failed to install DRA Driver: %w", err)
	}

	framework.Logf("Waiting for DRA Driver to be ready...")
	if err := pi.WaitForDRADriver(ctx, 5*time.Minute); err != nil {
		framework.Logf("DRA Driver failed to become ready, rolling back cluster mutations...")
		pi.RollbackDRAMutations(ctx)
		return fmt.Errorf("DRA Driver failed to become ready: %w", err)
	}

	framework.Logf("=== All prerequisites validated and ready ===")
	return nil
}

// ensureHelm checks if Helm is available
func (pi *PrerequisitesInstaller) ensureHelm(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "helm", "version", "--short")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm command not found or failed: %w\nOutput: %s", err, string(output))
	}
	framework.Logf("Helm version: %s", strings.TrimSpace(string(output)))
	return nil
}

// InstallDRADriver installs NVIDIA DRA Driver via Helm with pinned version
func (pi *PrerequisitesInstaller) InstallDRADriver(ctx context.Context) error {
	framework.Logf("Installing NVIDIA DRA Driver")

	// Create namespace
	if err := pi.createNamespace(ctx, draDriverNamespace); err != nil {
		return err
	}

	// Label GPU nodes for DRA kubelet plugin scheduling
	if err := pi.labelGPUNodesForDRA(ctx); err != nil {
		return fmt.Errorf("failed to label GPU nodes: %w", err)
	}

	// Grant SCC permissions
	if err := pi.grantSCCPermissions(ctx); err != nil {
		return fmt.Errorf("failed to grant SCC permissions: %w", err)
	}

	// Check if already installed
	if pi.isHelmReleaseInstalled(ctx, draDriverRelease, draDriverNamespace) {
		framework.Logf("DRA Driver already installed, skipping")
		return nil
	}

	// Build Helm install command with version pinning for hermetic tests.
	// Version can be overridden via NVIDIA_DRA_DRIVER_VERSION, and the chart
	// reference itself via NVIDIA_DRA_DRIVER_CHART, so a newer driver can be used
	// on hardware the pinned chart cannot enumerate.
	version := os.Getenv("NVIDIA_DRA_DRIVER_VERSION")
	if version == "" {
		version = draDriverDefaultVersion
	}
	chart := os.Getenv("NVIDIA_DRA_DRIVER_CHART")
	if chart == "" {
		chart = draDriverChart
	}
	framework.Logf("Installing DRA Driver chart %s version: %s", chart, version)

	args := []string{
		"install", draDriverRelease, chart,
		"--namespace", draDriverNamespace,
		"--version", version,
		"--set", fmt.Sprintf("nameOverride=%s", draDriverNameOverride),
		"--set", "nvidiaDriverRoot=/run/nvidia/driver",
		"--set", "gpuResourcesEnabledOverride=true",
		"--set", "image.pullPolicy=IfNotPresent",
		"--set-string", "kubeletPlugin.nodeSelector.nvidia\\.com/dra-kubelet-plugin=true",
		"--set", "controller.tolerations[0].key=node-role.kubernetes.io/master",
		"--set", "controller.tolerations[0].operator=Exists",
		"--set", "controller.tolerations[0].effect=NoSchedule",
		"--set", "controller.tolerations[1].key=node-role.kubernetes.io/control-plane",
		"--set", "controller.tolerations[1].operator=Exists",
		"--set", "controller.tolerations[1].effect=NoSchedule",
		"--wait",
		"--timeout", "5m",
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install DRA Driver: %w\nOutput: %s", err, string(output))
	}

	framework.Logf("DRA Driver installed successfully")
	return nil
}

// WaitForGPUOperator waits for the GPU Operator components this suite's own Helm install depends
// on: the NVIDIA kernel driver and the container toolkit that provides CDI.
//
// Only the install path calls this. The DaemonSet names are not stable across GPU Operator
// releases, and a cluster can run a pre-installed host driver with no DaemonSet at all, so a
// missing DaemonSet is reported and tolerated while a present one must become ready. The
// authoritative proof that the whole stack works is the ResourceSlice wait in WaitForDRADriver,
// which no rename can satisfy by accident.
func (pi *PrerequisitesInstaller) WaitForGPUOperator(ctx context.Context, timeout time.Duration) error {
	framework.Logf("Waiting for GPU Operator to be ready (timeout: %v)", timeout)

	// The NVIDIA driver; DRA tests do not require the device plugin daemonset.
	if err := pi.waitForOperatorDaemonSet(ctx, timeout, "NVIDIA driver", "nvidia-driver-daemonset", "driver-daemonset"); err != nil {
		return fmt.Errorf("driver daemonset not ready: %w", err)
	}

	// The container toolkit, which provides the CDI support DRA needs.
	if err := pi.waitForOperatorDaemonSet(ctx, timeout, "CDI container toolkit", "nvidia-container-toolkit"); err != nil {
		return fmt.Errorf("CDI not available: %w", err)
	}

	// Wait for GPU nodes to be labeled by NFD
	if err := pi.waitForGPUNodes(ctx, timeout); err != nil {
		return fmt.Errorf("no GPU nodes labeled: %w", err)
	}

	framework.Logf("GPU Operator is ready")
	return nil
}

// WaitForDRADriver waits for the DRA driver this suite installed with Helm to be ready. It names
// the chart's own resources, so it is only valid after InstallDRADriver; a driver deployed by the
// GPU Operator is detected by IsDRADriverInstalled instead.
func (pi *PrerequisitesInstaller) WaitForDRADriver(ctx context.Context, timeout time.Duration) error {
	framework.Logf("Waiting for DRA Driver to be ready (timeout: %v)", timeout)

	// Wait for controller deployment
	if err := pi.waitForDeployment(ctx, draDriverNamespace, draDriverController, timeout); err != nil {
		return fmt.Errorf("controller deployment not ready: %w", err)
	}

	// Wait for kubelet plugin daemonset
	if err := pi.waitForDaemonSet(ctx, draDriverNamespace, draDriverKubeletPlugin, timeout); err != nil {
		return fmt.Errorf("kubelet plugin daemonset not ready: %w", err)
	}

	// Wait for GPU slices to be published
	framework.Logf("Waiting for GPU ResourceSlices to be published...")
	validator := NewGPUValidator(pi.framework)
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		gpuCount, err := validator.GetTotalGPUCount(ctx)
		if err != nil {
			framework.Logf("Error checking GPU count: %v", err)
			return false, nil // Retry on error
		}
		if gpuCount > 0 {
			framework.Logf("Found %d published GPU(s) in ResourceSlices", gpuCount)
			return true, nil
		}
		framework.Logf("No GPU slices published yet, waiting...")
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("no published GPU slices within timeout: %w", err)
	}

	framework.Logf("DRA Driver is ready")
	return nil
}

// UninstallAll uninstalls DRA Driver (GPU Operator is cluster infrastructure, not removed)
func (pi *PrerequisitesInstaller) UninstallAll(ctx context.Context) error {
	framework.Logf("=== Cleaning up NVIDIA DRA Driver ===")

	// Only uninstall DRA Driver (GPU Operator is cluster infrastructure)
	if err := pi.UninstallDRADriver(ctx); err != nil {
		framework.Logf("Warning: failed to uninstall DRA Driver: %v", err)
	}

	framework.Logf("=== Cleanup complete ===")
	return nil
}

// UninstallDRADriver uninstalls DRA Driver and cleans up cluster-scoped resources
func (pi *PrerequisitesInstaller) UninstallDRADriver(ctx context.Context) error {
	framework.Logf("Uninstalling DRA Driver and cleaning up cluster resources")

	// Step 1: Uninstall Helm release
	cmd := exec.CommandContext(ctx, "helm", "uninstall", draDriverRelease,
		"--namespace", draDriverNamespace,
		"--wait",
		"--timeout", "5m")

	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "not found") {
		return fmt.Errorf("failed to uninstall DRA Driver: %w\nOutput: %s", err, string(output))
	}

	// Step 2: Delete ClusterRoleBindings created by grantSCCPermissions
	serviceAccounts := []string{
		draDriverControllerSA,
		draDriverKubeletPluginSA,
		draDriverComputeDomainSA,
	}

	for _, sa := range serviceAccounts {
		crbName := fmt.Sprintf("nvidia-dra-privileged-%s", sa)
		err := pi.client.RbacV1().ClusterRoleBindings().Delete(ctx, crbName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			framework.Logf("Warning: failed to delete ClusterRoleBinding %s: %v", crbName, err)
		} else {
			framework.Logf("Deleted ClusterRoleBinding %s", crbName)
		}
	}

	// Step 3: Remove DRA kubelet plugin labels from GPU nodes
	nodes, err := pi.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "nvidia.com/dra-kubelet-plugin=true",
	})
	if err != nil && !errors.IsNotFound(err) {
		framework.Logf("Warning: failed to list labeled nodes: %v", err)
	} else {
		for _, node := range nodes.Items {
			patchData := []byte(`{"metadata":{"labels":{"nvidia.com/dra-kubelet-plugin":null}}}`)
			_, err := pi.client.CoreV1().Nodes().Patch(ctx, node.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
			if err != nil && !errors.IsNotFound(err) {
				framework.Logf("Warning: failed to remove label from node %s: %v", node.Name, err)
			} else {
				framework.Logf("Removed DRA label from node %s", node.Name)
			}
		}
	}

	// Step 4: Delete namespace
	if err := pi.client.CoreV1().Namespaces().Delete(ctx, draDriverNamespace, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete namespace: %w", err)
		}
	}

	framework.Logf("DRA Driver uninstalled and cluster resources cleaned up")
	return nil
}

// RollbackDRAMutations performs best-effort cleanup of cluster-scoped mutations made during DRA installation
// This is called when installation or waiting fails, to avoid leaving partial state
func (pi *PrerequisitesInstaller) RollbackDRAMutations(ctx context.Context) {
	framework.Logf("Rolling back DRA cluster mutations (best-effort)...")

	// Step 1: Delete ClusterRoleBindings
	serviceAccounts := []string{
		draDriverControllerSA,
		draDriverKubeletPluginSA,
		draDriverComputeDomainSA,
	}

	for _, sa := range serviceAccounts {
		crbName := fmt.Sprintf("nvidia-dra-privileged-%s", sa)
		err := pi.client.RbacV1().ClusterRoleBindings().Delete(ctx, crbName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			framework.Logf("Warning: failed to delete ClusterRoleBinding %s during rollback: %v", crbName, err)
		} else if err == nil {
			framework.Logf("Rolled back ClusterRoleBinding %s", crbName)
		}
	}

	// Step 2: Remove DRA kubelet plugin labels from GPU nodes
	nodes, err := pi.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "nvidia.com/dra-kubelet-plugin=true",
	})
	if err == nil {
		for _, node := range nodes.Items {
			patchData := []byte(`{"metadata":{"labels":{"nvidia.com/dra-kubelet-plugin":null}}}`)
			_, err := pi.client.CoreV1().Nodes().Patch(ctx, node.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
			if err != nil && !errors.IsNotFound(err) {
				framework.Logf("Warning: failed to remove label from node %s during rollback: %v", node.Name, err)
			} else if err == nil {
				framework.Logf("Rolled back DRA label from node %s", node.Name)
			}
		}
	} else {
		framework.Logf("Warning: failed to list labeled nodes during rollback: %v", err)
	}

	// Step 3: Delete namespace
	err = pi.client.CoreV1().Namespaces().Delete(ctx, draDriverNamespace, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		framework.Logf("Warning: failed to delete namespace %s during rollback: %v", draDriverNamespace, err)
	} else if err == nil {
		framework.Logf("Rolled back namespace %s", draDriverNamespace)
	}

	framework.Logf("Rollback complete (errors above are warnings, original error will be returned)")
}

// Helper methods

func (pi *PrerequisitesInstaller) createNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := pi.client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace %s: %w", name, err)
	}
	framework.Logf("Namespace %s created or already exists", name)
	return nil
}

func (pi *PrerequisitesInstaller) labelGPUNodesForDRA(ctx context.Context) error {
	framework.Logf("Labeling GPU nodes for DRA kubelet plugin")

	nodes, err := pi.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "nvidia.com/gpu.present=true",
	})
	if err != nil {
		return fmt.Errorf("failed to list GPU nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return fmt.Errorf("no GPU nodes found to label")
	}

	for _, node := range nodes.Items {
		if node.Labels["nvidia.com/dra-kubelet-plugin"] == "true" {
			framework.Logf("Node %s already has DRA kubelet plugin label", node.Name)
			continue
		}

		// Use Patch to avoid resourceVersion conflicts
		patchData := []byte(`{"metadata":{"labels":{"nvidia.com/dra-kubelet-plugin":"true"}}}`)
		_, err := pi.client.CoreV1().Nodes().Patch(ctx, node.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to label node %s: %w", node.Name, err)
		}
		framework.Logf("Labeled GPU node %s with nvidia.com/dra-kubelet-plugin=true", node.Name)
	}

	return nil
}

func (pi *PrerequisitesInstaller) grantSCCPermissions(ctx context.Context) error {
	framework.Logf("Granting SCC permissions to DRA driver service accounts")

	serviceAccounts := []string{
		draDriverControllerSA,
		draDriverKubeletPluginSA,
		draDriverComputeDomainSA,
	}

	for _, sa := range serviceAccounts {
		// Create ClusterRoleBinding to grant privileged SCC
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("nvidia-dra-privileged-%s", sa),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "system:openshift:scc:privileged",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      sa,
					Namespace: draDriverNamespace,
				},
			},
		}

		_, err := pi.client.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create ClusterRoleBinding for %s: %w", sa, err)
		}
		framework.Logf("SCC permissions granted to %s", sa)
	}

	return nil
}

func (pi *PrerequisitesInstaller) isHelmReleaseInstalled(ctx context.Context, release, namespace string) bool {
	cmd := exec.CommandContext(ctx, "helm", "status", release, "--namespace", namespace)
	err := cmd.Run()
	return err == nil
}

func (pi *PrerequisitesInstaller) waitForDaemonSet(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		ds, err := pi.client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				framework.Logf("DaemonSet %s/%s not found yet", namespace, name)
				return false, nil
			}
			return false, err
		}

		ready := ds.Status.DesiredNumberScheduled > 0 &&
			ds.Status.NumberReady == ds.Status.DesiredNumberScheduled &&
			ds.Status.NumberUnavailable == 0

		if !ready {
			framework.Logf("DaemonSet %s/%s not ready: desired=%d, ready=%d, unavailable=%d",
				namespace, name, ds.Status.DesiredNumberScheduled, ds.Status.NumberReady, ds.Status.NumberUnavailable)
		}

		return ready, nil
	})
}

// waitForOperatorDaemonSet waits for the GPU Operator DaemonSet whose name contains any of
// nameFragments. A DaemonSet that is absent right now is reported and tolerated, because the
// operator has already been confirmed running and its component names change between releases;
// one that exists must become ready within timeout.
func (pi *PrerequisitesInstaller) waitForOperatorDaemonSet(ctx context.Context, timeout time.Duration, component string, nameFragments ...string) error {
	dsList, err := pi.client.AppsV1().DaemonSets(gpuOperatorNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list DaemonSets in %s: %w", gpuOperatorNamespace, err)
	}

	for _, fragment := range nameFragments {
		for _, ds := range dsList.Items {
			if strings.Contains(ds.Name, fragment) {
				framework.Logf("Waiting for the %s DaemonSet %s/%s", component, gpuOperatorNamespace, ds.Name)
				return pi.waitForDaemonSet(ctx, gpuOperatorNamespace, ds.Name, timeout)
			}
		}
	}

	framework.Logf("No %s DaemonSet in %s matching %v; continuing, the published %s ResourceSlices are the real check",
		component, gpuOperatorNamespace, nameFragments, nvidiaDRADriverName)
	return nil
}

func (pi *PrerequisitesInstaller) waitForDeployment(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		deploy, err := pi.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				framework.Logf("Deployment %s/%s not found yet", namespace, name)
				return false, nil
			}
			return false, err
		}

		ready := deploy.Status.Replicas > 0 &&
			deploy.Status.ReadyReplicas == deploy.Status.Replicas

		if !ready {
			framework.Logf("Deployment %s/%s not ready: replicas=%d, ready=%d",
				namespace, name, deploy.Status.Replicas, deploy.Status.ReadyReplicas)
		}

		return ready, nil
	})
}

func (pi *PrerequisitesInstaller) waitForGPUNodes(ctx context.Context, timeout time.Duration) error {
	framework.Logf("Waiting for GPU nodes to be labeled by NFD")

	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		nodes, err := pi.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: "nvidia.com/gpu.present=true",
		})
		if err != nil {
			return false, err
		}

		if len(nodes.Items) == 0 {
			framework.Logf("No GPU nodes labeled yet by NFD")
			return false, nil
		}

		framework.Logf("Found %d GPU node(s) labeled by NFD", len(nodes.Items))
		for _, node := range nodes.Items {
			framework.Logf("  - GPU node: %s", node.Name)
		}
		return true, nil
	})
}

// IsGPUOperatorInstalled checks if GPU Operator is installed (via Helm or OLM)
func (pi *PrerequisitesInstaller) IsGPUOperatorInstalled(ctx context.Context) bool {
	// Check if the namespace exists
	_, err := pi.client.CoreV1().Namespaces().Get(ctx, gpuOperatorNamespace, metav1.GetOptions{})
	if err != nil {
		return false
	}

	// Check if GPU Operator pods are running
	pods, err := pi.client.CoreV1().Pods(gpuOperatorNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=gpu-operator",
	})
	if err != nil || len(pods.Items) == 0 {
		return false
	}

	// Check if at least one pod is running or succeeded
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Running" || pod.Status.Phase == "Succeeded" {
			framework.Logf("Found running GPU Operator pod: %s", pod.Name)
			return true
		}
	}

	return false
}

// IsDRADriverInstalled reports whether a gpu.nvidia.com DRA driver is already serving the
// cluster, however it was deployed.
//
// Detection is by capability rather than by deployment. GPU Operator v26.7 and later deploy the
// DRA driver themselves through GPUCluster, under their own namespace, release and label names,
// so looking for this suite's Helm release reports "not installed" on a cluster where DRA works.
//
// Published ResourceSlices decide it: only a registered, running kubelet plugin for the driver
// can publish them, and allocatable devices are exactly what these specs need. The driver's
// DeviceClass is reported alongside, because that is what a claim which does not build its own
// would reference, but it does not gate the result: a DeviceClass is an object anyone can create,
// and refusing a driver that is demonstrably publishing GPUs would send the suite down the Helm
// install path on a cluster that needs nothing, mutating node labels and cluster role bindings
// for no reason.
func (pi *PrerequisitesInstaller) IsDRADriverInstalled(ctx context.Context) bool {
	gpus, err := NewGPUValidator(pi.framework).GetTotalGPUCount(ctx)
	if err != nil {
		framework.Logf("Failed to count devices published by %s: %v", nvidiaDRADriverName, err)
		return false
	}
	if gpus == 0 {
		framework.Logf("No %s ResourceSlices publish any device yet", nvidiaDRADriverName)
		return false
	}

	if _, err := pi.client.ResourceV1().DeviceClasses().Get(ctx, nvidiaDRADeviceClassName, metav1.GetOptions{}); err != nil {
		framework.Logf("DeviceClass %s is not readable (%v); the specs build their own DeviceClass, so continuing", nvidiaDRADeviceClassName, err)
	}

	framework.Logf("Found a %s DRA driver publishing %d GPU(s)", nvidiaDRADriverName, gpus)
	return true
}
