package nvidia

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	// GPU Operator namespace (used for validation only)
	gpuOperatorNamespace = "nvidia-gpu-operator"

	// DRA Driver constants
	draDriverNamespace       = "nvidia-dra-driver-gpu"
	draDriverRelease         = "nvidia-dra-driver-gpu"
	draDriverChart           = "nvidia/nvidia-dra-driver-gpu"
	draDriverControllerSA    = "nvidia-dra-driver-gpu-service-account-controller"
	draDriverKubeletPluginSA = "nvidia-dra-driver-gpu-service-account-kubeletplugin"
	draDriverComputeDomainSA = "compute-domain-daemon-service-account"
)

// PrerequisitesInstaller validates GPU Operator and manages DRA driver installation
type PrerequisitesInstaller struct {
	client kubernetes.Interface
}

// NewPrerequisitesInstaller creates a new installer
func NewPrerequisitesInstaller(f *framework.Framework) *PrerequisitesInstaller {
	return &PrerequisitesInstaller{
		client: f.ClientSet,
	}
}

// InstallAll validates GPU Operator is present and installs DRA Driver
func (pi *PrerequisitesInstaller) InstallAll(ctx context.Context) error {
	framework.Logf("=== Validating NVIDIA GPU Stack Prerequisites ===")

	// Step 1: Validate GPU Operator is already installed
	framework.Logf("Checking if GPU Operator is installed...")
	if !pi.IsGPUOperatorInstalled(ctx) {
		return fmt.Errorf("GPU Operator not found - must be pre-installed on the cluster. " +
			"Install GPU Operator via OLM before running these tests")
	}
	framework.Logf("GPU Operator detected")

	// Step 2: Wait for GPU Operator to be ready
	framework.Logf("Waiting for GPU Operator to be ready...")
	if err := pi.WaitForGPUOperator(ctx, 5*time.Minute); err != nil {
		return fmt.Errorf("GPU Operator not ready: %w. Ensure GPU Operator is fully deployed", err)
	}
	framework.Logf("GPU Operator is ready")

	// Step 3: Check if DRA Driver already installed (skip if present)
	if pi.IsDRADriverInstalled(ctx) {
		framework.Logf("DRA Driver already installed, skipping installation")
	} else {
		// Step 4: Ensure Helm is available
		if err := pi.ensureHelm(ctx); err != nil {
			return fmt.Errorf("helm not available: %w", err)
		}

		// Step 5: Add NVIDIA Helm repository
		if err := pi.addHelmRepoForDRADriver(ctx); err != nil {
			return fmt.Errorf("failed to add Helm repository: %w", err)
		}

		// Step 6: Install DRA Driver (latest version)
		if err := pi.InstallDRADriver(ctx); err != nil {
			return fmt.Errorf("failed to install DRA Driver: %w", err)
		}
	}

	// Step 7: Wait for DRA Driver to be ready
	framework.Logf("Waiting for DRA Driver to be ready...")
	if err := pi.WaitForDRADriver(ctx, 5*time.Minute); err != nil {
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

// addHelmRepoForDRADriver adds NVIDIA Helm repository for DRA driver installation
func (pi *PrerequisitesInstaller) addHelmRepoForDRADriver(ctx context.Context) error {
	framework.Logf("Adding NVIDIA Helm repository for DRA driver")

	// Add repo
	cmd := exec.CommandContext(ctx, "helm", "repo", "add", "nvidia", "https://nvidia.github.io/gpu-operator")
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "already exists") {
		return fmt.Errorf("failed to add helm repo: %w\nOutput: %s", err, string(output))
	}

	// Update repo
	cmd = exec.CommandContext(ctx, "helm", "repo", "update")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update helm repo: %w\nOutput: %s", err, string(output))
	}

	framework.Logf("NVIDIA Helm repository added and updated")
	return nil
}

// InstallDRADriver installs NVIDIA DRA Driver via Helm (latest version)
func (pi *PrerequisitesInstaller) InstallDRADriver(ctx context.Context) error {
	framework.Logf("Installing NVIDIA DRA Driver (latest version)")

	// Create namespace
	if err := pi.createNamespace(ctx, draDriverNamespace); err != nil {
		return err
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

	// Build Helm install command
	args := []string{
		"install", draDriverRelease, draDriverChart,
		"--namespace", draDriverNamespace,
		"--set", "nvidiaDriverRoot=/run/nvidia/driver",
		"--set", "gpuResourcesEnabledOverride=true",
		"--set", "featureGates.IMEXDaemonsWithDNSNames=false",
		"--set", "featureGates.MPSSupport=true",
		"--set", "featureGates.TimeSlicingSettings=true",
		"--set", "controller.tolerations[0].key=node-role.kubernetes.io/control-plane",
		"--set", "controller.tolerations[0].operator=Exists",
		"--set", "controller.tolerations[0].effect=NoSchedule",
		"--set", "controller.tolerations[1].key=node-role.kubernetes.io/master",
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

// WaitForGPUOperator waits for GPU Operator to be ready
func (pi *PrerequisitesInstaller) WaitForGPUOperator(ctx context.Context, timeout time.Duration) error {
	framework.Logf("Waiting for GPU Operator to be ready (timeout: %v)", timeout)

	// Wait for driver daemonset
	if err := pi.waitForDaemonSet(ctx, gpuOperatorNamespace, "nvidia-driver-daemonset", timeout); err != nil {
		return fmt.Errorf("driver daemonset not ready: %w", err)
	}

	// Wait for device plugin daemonset
	if err := pi.waitForDaemonSet(ctx, gpuOperatorNamespace, "nvidia-device-plugin-daemonset", timeout); err != nil {
		return fmt.Errorf("device plugin daemonset not ready: %w", err)
	}

	// Wait for GPU nodes to be labeled by NFD
	if err := pi.waitForGPUNodes(ctx, timeout); err != nil {
		return fmt.Errorf("no GPU nodes labeled: %w", err)
	}

	framework.Logf("GPU Operator is ready")
	return nil
}

// WaitForDRADriver waits for DRA Driver to be ready
func (pi *PrerequisitesInstaller) WaitForDRADriver(ctx context.Context, timeout time.Duration) error {
	framework.Logf("Waiting for DRA Driver to be ready (timeout: %v)", timeout)

	// Wait for controller deployment
	if err := pi.waitForDeployment(ctx, draDriverNamespace, draDriverRelease+"-controller", timeout); err != nil {
		return fmt.Errorf("controller deployment not ready: %w", err)
	}

	// Wait for kubelet plugin daemonset
	if err := pi.waitForDaemonSet(ctx, draDriverNamespace, draDriverRelease+"-kubelet-plugin", timeout); err != nil {
		return fmt.Errorf("kubelet plugin daemonset not ready: %w", err)
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

// UninstallDRADriver uninstalls DRA Driver
func (pi *PrerequisitesInstaller) UninstallDRADriver(ctx context.Context) error {
	framework.Logf("Uninstalling DRA Driver")

	cmd := exec.CommandContext(ctx, "helm", "uninstall", draDriverRelease,
		"--namespace", draDriverNamespace,
		"--wait",
		"--timeout", "5m")

	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "not found") {
		return fmt.Errorf("failed to uninstall DRA Driver: %w\nOutput: %s", err, string(output))
	}

	// Delete namespace
	if err := pi.client.CoreV1().Namespaces().Delete(ctx, draDriverNamespace, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete namespace: %w", err)
		}
	}

	framework.Logf("DRA Driver uninstalled")
	return nil
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

// IsDRADriverInstalled checks if DRA Driver is installed (via Helm or other means)
func (pi *PrerequisitesInstaller) IsDRADriverInstalled(ctx context.Context) bool {
	// Check if the namespace exists
	_, err := pi.client.CoreV1().Namespaces().Get(ctx, draDriverNamespace, metav1.GetOptions{})
	if err != nil {
		return false
	}

	// Check if DRA kubelet plugin pods are running
	pods, err := pi.client.CoreV1().Pods(draDriverNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=nvidia-dra-driver-gpu",
	})
	if err != nil || len(pods.Items) == 0 {
		return false
	}

	// Check if at least one pod is running
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Running" {
			framework.Logf("Found running DRA Driver pod: %s", pod.Name)
			return true
		}
	}

	return false
}
