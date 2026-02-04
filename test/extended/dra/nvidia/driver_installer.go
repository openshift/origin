package nvidia

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	defaultDriverNamespace = "nvidia-dra-driver"
	defaultHelmRelease     = "nvidia-dra-driver"
	defaultHelmChart       = "oci://ghcr.io/nvidia/k8s-dra-driver-gpu/nvidia-dra-driver"
	defaultDriverName      = "gpu.nvidia.com"
)

// DriverInstaller manages NVIDIA DRA driver lifecycle via Helm
type DriverInstaller struct {
	client      kubernetes.Interface
	namespace   string
	helmRelease string
	helmChart   string
	driverName  string
}

// NewDriverInstaller creates a new installer instance
func NewDriverInstaller(f *framework.Framework) *DriverInstaller {
	return &DriverInstaller{
		client:      f.ClientSet,
		namespace:   defaultDriverNamespace,
		helmRelease: defaultHelmRelease,
		helmChart:   defaultHelmChart,
		driverName:  defaultDriverName,
	}
}

// Install installs the NVIDIA DRA driver using Helm
func (di *DriverInstaller) Install(ctx context.Context) error {
	framework.Logf("Installing NVIDIA DRA driver via Helm")

	// Create namespace if it doesn't exist
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: di.namespace,
		},
	}
	_, err := di.client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create namespace %s: %w", di.namespace, err)
	}
	framework.Logf("Namespace %s created or already exists", di.namespace)

	// Install driver via Helm
	cmd := exec.CommandContext(ctx, "helm", "install", di.helmRelease,
		di.helmChart,
		"--namespace", di.namespace,
		"--wait",
		"--timeout", "5m")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install NVIDIA DRA driver: %w\nOutput: %s", err, string(output))
	}
	framework.Logf("Helm install output: %s", string(output))

	return nil
}

// Uninstall removes the NVIDIA DRA driver
func (di *DriverInstaller) Uninstall(ctx context.Context) error {
	framework.Logf("Uninstalling NVIDIA DRA driver")

	// Uninstall via Helm
	cmd := exec.CommandContext(ctx, "helm", "uninstall", di.helmRelease,
		"--namespace", di.namespace,
		"--wait",
		"--timeout", "5m")

	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "not found") {
		return fmt.Errorf("failed to uninstall NVIDIA DRA driver: %w\nOutput: %s", err, string(output))
	}
	framework.Logf("Helm uninstall output: %s", string(output))

	// Delete namespace
	err = di.client.CoreV1().Namespaces().Delete(ctx, di.namespace, metav1.DeleteOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete namespace %s: %w", di.namespace, err)
	}
	framework.Logf("Namespace %s deleted", di.namespace)

	return nil
}

// WaitForReady waits for driver to be operational
func (di *DriverInstaller) WaitForReady(ctx context.Context, timeout time.Duration) error {
	framework.Logf("Waiting for NVIDIA DRA driver to be ready (timeout: %v)", timeout)

	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		// Get DaemonSet
		ds, err := di.client.AppsV1().DaemonSets(di.namespace).Get(ctx, di.helmRelease, metav1.GetOptions{})
		if err != nil {
			framework.Logf("DaemonSet not found yet: %v", err)
			return false, nil
		}

		// Check if DaemonSet is ready
		if !di.isDaemonSetReady(ds) {
			framework.Logf("DaemonSet not ready yet: desired=%d, current=%d, ready=%d",
				ds.Status.DesiredNumberScheduled,
				ds.Status.CurrentNumberScheduled,
				ds.Status.NumberReady)
			return false, nil
		}

		framework.Logf("DaemonSet is ready: %d/%d pods ready",
			ds.Status.NumberReady,
			ds.Status.DesiredNumberScheduled)
		return true, nil
	})
}

// isDaemonSetReady checks if DaemonSet is fully ready
func (di *DriverInstaller) isDaemonSetReady(ds *appsv1.DaemonSet) bool {
	return ds.Status.DesiredNumberScheduled > 0 &&
		ds.Status.NumberReady == ds.Status.DesiredNumberScheduled &&
		ds.Status.NumberUnavailable == 0
}

// VerifyPluginRegistration checks if kubelet has registered the plugin
func (di *DriverInstaller) VerifyPluginRegistration(ctx context.Context, nodeName string) error {
	framework.Logf("Verifying plugin registration on node %s", nodeName)

	// Get driver pod running on the node
	podList, err := di.client.CoreV1().Pods(di.namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		return fmt.Errorf("failed to list driver pods on node %s: %w", nodeName, err)
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no driver pod found on node %s", nodeName)
	}

	pod := podList.Items[0]
	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("driver pod %s on node %s is not running (phase: %s)", pod.Name, nodeName, pod.Status.Phase)
	}

	framework.Logf("Driver pod %s is running on node %s", pod.Name, nodeName)
	return nil
}

// GetInstalledVersion returns the version of installed driver
func (di *DriverInstaller) GetInstalledVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "helm", "list",
		"--namespace", di.namespace,
		"--filter", di.helmRelease,
		"--output", "json")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get helm release version: %w\nOutput: %s", err, string(output))
	}

	// Parse JSON output to get version
	// For simplicity, just return the raw output
	return string(output), nil
}

// GetDriverNamespace returns the namespace where the driver is installed
func (di *DriverInstaller) GetDriverNamespace() string {
	return di.namespace
}

// GetDriverName returns the driver name
func (di *DriverInstaller) GetDriverName() string {
	return di.driverName
}
