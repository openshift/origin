package example

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	driverNamespace      = "dra-example-driver"
	driverRelease        = "dra-example-driver"
	driverServiceAccount = "dra-example-driver-service-account"
	driverCRBName        = "dra-example-driver-privileged-scc"

	upstreamRepoURL  = "https://github.com/kubernetes-sigs/dra-example-driver.git"
	helmChartRelPath = "deployments/helm/dra-example-driver"
)

// PrerequisitesInstaller manages dra-example-driver installation and cleanup
type PrerequisitesInstaller struct {
	client    kubernetes.Interface
	framework *framework.Framework
	cloneDir  string
}

// NewPrerequisitesInstaller creates a new installer
func NewPrerequisitesInstaller(f *framework.Framework) *PrerequisitesInstaller {
	return &PrerequisitesInstaller{
		client:    f.ClientSet,
		framework: f,
	}
}

// InstallAll installs the dra-example-driver prerequisites
func (pi *PrerequisitesInstaller) InstallAll(ctx context.Context) error {
	framework.Logf("=== Installing DRA Example Driver Prerequisites ===")

	if err := pi.ensureHelm(ctx); err != nil {
		return fmt.Errorf("helm not available: %w", err)
	}

	if pi.IsDriverInstalled(ctx) {
		framework.Logf("DRA example driver already installed, waiting for device publication...")
		return pi.WaitForDriver(ctx, 5*time.Minute)
	}

	if err := pi.ensureNamespaceGone(ctx); err != nil {
		return fmt.Errorf("failed to ensure clean namespace state: %w", err)
	}

	if err := pi.cloneUpstreamRepo(ctx); err != nil {
		return fmt.Errorf("failed to clone upstream repo: %w", err)
	}

	if err := pi.createNamespace(ctx); err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	if err := pi.grantSCCPermissions(ctx); err != nil {
		pi.RollbackMutations(ctx)
		return fmt.Errorf("failed to grant SCC permissions: %w", err)
	}

	if err := pi.helmInstall(ctx); err != nil {
		pi.RollbackMutations(ctx)
		return fmt.Errorf("failed to install via Helm: %w", err)
	}

	framework.Logf("Waiting for DRA example driver to be ready...")
	if err := pi.WaitForDriver(ctx, 5*time.Minute); err != nil {
		pi.RollbackMutations(ctx)
		return fmt.Errorf("driver failed to become ready: %w", err)
	}

	framework.Logf("=== DRA Example Driver installation complete ===")
	return nil
}

func (pi *PrerequisitesInstaller) ensureHelm(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "helm", "version", "--short")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm command not found or failed: %w\nOutput: %s", err, string(output))
	}
	framework.Logf("Helm version: %s", strings.TrimSpace(string(output)))

	gitCmd := exec.CommandContext(ctx, "git", "version")
	gitOutput, gitErr := gitCmd.CombinedOutput()
	if gitErr != nil {
		return fmt.Errorf("git command not found or failed: %w\nOutput: %s", gitErr, string(gitOutput))
	}
	framework.Logf("Git version: %s", strings.TrimSpace(string(gitOutput)))
	return nil
}

func (pi *PrerequisitesInstaller) cloneUpstreamRepo(ctx context.Context) error {
	tmpDir, err := os.MkdirTemp("", "dra-example-driver-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	pi.cloneDir = tmpDir

	framework.Logf("Cloning upstream dra-example-driver to %s", tmpDir)

	ref := os.Getenv("DRA_EXAMPLE_DRIVER_REF")
	if ref == "" {
		ref = "main"
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", "--branch", ref, upstreamRepoURL, tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tmpDir)
		pi.cloneDir = ""
		return fmt.Errorf("failed to clone repo: %w\nOutput: %s", err, string(output))
	}

	framework.Logf("Cloned dra-example-driver (ref: %s)", ref)
	return nil
}

// ensureNamespaceGone guarantees the driver namespace is fully deleted before
// a fresh install begins. This handles the race where a previous run's cleanup
// left the namespace in Terminating state (async GC of finalizers, Helm
// release Secrets, or DaemonSet pods). It also runs a best-effort
// helm uninstall to clear any stale release that would block re-creation.
func (pi *PrerequisitesInstaller) ensureNamespaceGone(ctx context.Context) error {
	ns, err := pi.client.CoreV1().Namespaces().Get(ctx, driverNamespace, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check namespace %s: %w", driverNamespace, err)
	}

	framework.Logf("Namespace %s exists (phase=%s), cleaning up before fresh install...", driverNamespace, ns.Status.Phase)

	if ns.Status.Phase != corev1.NamespaceTerminating {
		cmd := exec.CommandContext(ctx, "helm", "uninstall", driverRelease,
			"--namespace", driverNamespace, "--wait", "--timeout", "2m")
		output, helmErr := cmd.CombinedOutput()
		if helmErr != nil && !strings.Contains(string(output), "not found") {
			framework.Logf("Warning: helm uninstall during pre-cleanup: %v (output: %s)", helmErr, strings.TrimSpace(string(output)))
		}

		if delErr := pi.client.CoreV1().Namespaces().Delete(ctx, driverNamespace, metav1.DeleteOptions{}); delErr != nil && !errors.IsNotFound(delErr) {
			framework.Logf("Warning: failed to delete namespace %s: %v", driverNamespace, delErr)
		}
	}

	framework.Logf("Waiting for namespace %s to be fully removed...", driverNamespace)
	return wait.PollUntilContextTimeout(ctx, 3*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, getErr := pi.client.CoreV1().Namespaces().Get(ctx, driverNamespace, metav1.GetOptions{})
		if errors.IsNotFound(getErr) {
			framework.Logf("Namespace %s fully removed", driverNamespace)
			return true, nil
		}
		if getErr != nil {
			framework.Logf("Error checking namespace %s (will retry): %v", driverNamespace, getErr)
			return false, nil
		}
		framework.Logf("Namespace %s still exists, waiting for GC...", driverNamespace)
		return false, nil
	})
}

func (pi *PrerequisitesInstaller) createNamespace(ctx context.Context) error {
	requiredLabels := map[string]string{
		"pod-security.kubernetes.io/enforce": "privileged",
		"pod-security.kubernetes.io/warn":    "privileged",
		"pod-security.kubernetes.io/audit":   "privileged",
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   driverNamespace,
			Labels: requiredLabels,
		},
	}
	_, err := pi.client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err == nil {
		framework.Logf("Created namespace %s with privileged PSA labels", driverNamespace)
		return nil
	}
	if !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace %s: %w", driverNamespace, err)
	}

	existing, err := pi.client.CoreV1().Namespaces().Get(ctx, driverNamespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get existing namespace %s: %w", driverNamespace, err)
	}

	needsUpdate := false
	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	for k, v := range requiredLabels {
		if existing.Labels[k] != v {
			existing.Labels[k] = v
			needsUpdate = true
		}
	}
	if needsUpdate {
		_, err = pi.client.CoreV1().Namespaces().Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update PSA labels on namespace %s: %w", driverNamespace, err)
		}
		framework.Logf("Updated namespace %s with required PSA labels", driverNamespace)
	} else {
		framework.Logf("Namespace %s already exists with correct PSA labels", driverNamespace)
	}
	return nil
}

func (pi *PrerequisitesInstaller) grantSCCPermissions(ctx context.Context) error {
	framework.Logf("Granting privileged SCC to DRA example driver ServiceAccount")

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: driverCRBName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:openshift:scc:privileged",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      driverServiceAccount,
				Namespace: driverNamespace,
			},
		},
	}

	_, err := pi.client.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ClusterRoleBinding %s: %w", driverCRBName, err)
	}
	framework.Logf("SCC permissions granted to %s/%s", driverNamespace, driverServiceAccount)
	return nil
}

// commonHelmArgs returns the base set of Helm arguments shared by install and
// upgrade operations (tolerations for master/control-plane nodes, wait, timeout).
func commonHelmArgs() []string {
	return []string{
		"--set", "kubeletPlugin.tolerations[0].key=node-role.kubernetes.io/master",
		"--set", "kubeletPlugin.tolerations[0].operator=Exists",
		"--set", "kubeletPlugin.tolerations[0].effect=NoSchedule",
		"--set", "kubeletPlugin.tolerations[1].key=node-role.kubernetes.io/control-plane",
		"--set", "kubeletPlugin.tolerations[1].operator=Exists",
		"--set", "kubeletPlugin.tolerations[1].effect=NoSchedule",
		"--wait",
		"--timeout", "5m",
	}
}

func (pi *PrerequisitesInstaller) helmInstall(ctx context.Context) error {
	chartPath := filepath.Join(pi.cloneDir, helmChartRelPath)

	framework.Logf("Installing DRA example driver via Helm from %s", chartPath)

	args := []string{"install", driverRelease, chartPath, "--namespace", driverNamespace}
	args = append(args, commonHelmArgs()...)

	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm install failed: %w\nOutput: %s", err, string(output))
	}

	framework.Logf("DRA example driver Helm install succeeded")
	return nil
}

// HelmUpgrade runs `helm upgrade` on the already-installed driver release with
// additional --set overrides. Each element in setValues must be a "key=value"
// string (e.g. "kubeletPlugin.gpuPartitions=4"). The base tolerations and
// wait/timeout flags are always included.
//
// If the upstream repo has not been cloned yet (e.g. because another test
// package performed the initial install), HelmUpgrade clones it automatically.
func (pi *PrerequisitesInstaller) HelmUpgrade(ctx context.Context, setValues ...string) error {
	chartPath, err := pi.resolveChartPath(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve Helm chart path: %w", err)
	}

	args := []string{"upgrade", driverRelease, chartPath, "--namespace", driverNamespace}
	args = append(args, commonHelmArgs()...)
	for _, sv := range setValues {
		args = append(args, "--set", sv)
	}

	framework.Logf("Running helm upgrade with extra values: %v", setValues)
	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm upgrade failed: %w\nOutput: %s", err, string(output))
	}

	framework.Logf("DRA example driver Helm upgrade succeeded")
	return nil
}

// resolveChartPath returns the path to the Helm chart directory, cloning the
// upstream repo if it hasn't been cloned yet by this installer instance.
func (pi *PrerequisitesInstaller) resolveChartPath(ctx context.Context) (string, error) {
	if pi.cloneDir != "" {
		return filepath.Join(pi.cloneDir, helmChartRelPath), nil
	}
	if err := pi.cloneUpstreamRepo(ctx); err != nil {
		return "", err
	}
	return filepath.Join(pi.cloneDir, helmChartRelPath), nil
}

// WaitForDriver waits for the driver to be ready and publishing devices
func (pi *PrerequisitesInstaller) WaitForDriver(ctx context.Context, timeout time.Duration) error {
	framework.Logf("Waiting for DRA example driver DaemonSet to be ready (timeout: %v)", timeout)

	if err := pi.waitForDaemonSet(ctx, timeout); err != nil {
		return fmt.Errorf("kubelet plugin DaemonSet not ready: %w", err)
	}

	framework.Logf("Waiting for ResourceSlices to be published...")
	validator := NewDeviceValidator(pi.framework)
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		count, err := validator.GetTotalDeviceCount(ctx)
		if err != nil {
			framework.Logf("Error checking device count: %v", err)
			return false, nil
		}
		if count > 0 {
			framework.Logf("Found %d published device(s) in ResourceSlices", count)
			return true, nil
		}
		framework.Logf("No devices published yet, waiting...")
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("no published device slices within timeout: %w", err)
	}

	framework.Logf("DRA example driver is ready")
	return nil
}

func (pi *PrerequisitesInstaller) waitForDaemonSet(ctx context.Context, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		dsList, err := pi.client.AppsV1().DaemonSets(driverNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, ds := range dsList.Items {
			if strings.Contains(ds.Name, driverRelease) {
				ready := ds.Status.DesiredNumberScheduled > 0 &&
					ds.Status.NumberReady == ds.Status.DesiredNumberScheduled &&
					ds.Status.NumberUnavailable == 0

				if !ready {
					framework.Logf("DaemonSet %s/%s not ready: desired=%d, ready=%d, unavailable=%d",
						driverNamespace, ds.Name, ds.Status.DesiredNumberScheduled, ds.Status.NumberReady, ds.Status.NumberUnavailable)
					return false, nil
				}

				framework.Logf("DaemonSet %s/%s is ready", driverNamespace, ds.Name)
				return true, nil
			}
		}

		framework.Logf("DaemonSet for %s not found yet in %s", driverRelease, driverNamespace)
		return false, nil
	})
}

// IsDriverInstalled checks if the driver is already installed and healthy.
// Returns false if the namespace is missing, terminating, or has no ready pods.
func (pi *PrerequisitesInstaller) IsDriverInstalled(ctx context.Context) bool {
	ns, err := pi.client.CoreV1().Namespaces().Get(ctx, driverNamespace, metav1.GetOptions{})
	if err != nil {
		return false
	}
	if ns.Status.Phase == corev1.NamespaceTerminating {
		framework.Logf("Namespace %s is terminating, driver not considered installed", driverNamespace)
		return false
	}

	pods, err := pi.client.CoreV1().Pods(driverNamespace).List(ctx, metav1.ListOptions{})
	if err != nil || len(pods.Items) == 0 {
		return false
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		allReady := len(pod.Status.ContainerStatuses) > 0
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				allReady = false
				break
			}
		}
		if allReady {
			framework.Logf("Found fully ready DRA example driver pod: %s", pod.Name)
			return true
		}
	}

	return false
}

// UninstallAll removes the driver and cleans up resources
func (pi *PrerequisitesInstaller) UninstallAll(ctx context.Context) error {
	framework.Logf("=== Cleaning up DRA Example Driver ===")

	cmd := exec.CommandContext(ctx, "helm", "uninstall", driverRelease,
		"--namespace", driverNamespace,
		"--wait",
		"--timeout", "5m")
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "not found") {
		framework.Logf("Warning: helm uninstall failed: %v\nOutput: %s", err, string(output))
	}

	pi.cleanupClusterResources(ctx)

	if err := pi.client.CoreV1().Namespaces().Delete(ctx, driverNamespace, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			framework.Logf("Warning: failed to delete namespace %s: %v", driverNamespace, err)
		}
	}

	if pi.cloneDir != "" {
		os.RemoveAll(pi.cloneDir)
	}

	framework.Logf("=== Cleanup complete ===")
	return nil
}

// RollbackMutations performs best-effort cleanup after partial install failure.
// It attempts helm uninstall first to properly remove Helm-managed resources
// and their finalizers, preventing the namespace from getting stuck in
// Terminating state on the next run.
func (pi *PrerequisitesInstaller) RollbackMutations(ctx context.Context) {
	framework.Logf("Rolling back DRA example driver cluster mutations (best-effort)...")

	cmd := exec.CommandContext(ctx, "helm", "uninstall", driverRelease,
		"--namespace", driverNamespace, "--wait", "--timeout", "2m")
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "not found") {
		framework.Logf("Warning: helm uninstall during rollback: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}

	pi.cleanupClusterResources(ctx)

	err = pi.client.CoreV1().Namespaces().Delete(ctx, driverNamespace, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		framework.Logf("Warning: failed to delete namespace %s during rollback: %v", driverNamespace, err)
	}

	if pi.cloneDir != "" {
		os.RemoveAll(pi.cloneDir)
	}

	framework.Logf("Rollback complete")
}

func (pi *PrerequisitesInstaller) cleanupClusterResources(ctx context.Context) {
	err := pi.client.RbacV1().ClusterRoleBindings().Delete(ctx, driverCRBName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		framework.Logf("Warning: failed to delete ClusterRoleBinding %s: %v", driverCRBName, err)
	} else if err == nil {
		framework.Logf("Deleted ClusterRoleBinding %s", driverCRBName)
	}
}
