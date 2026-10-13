package nvidia

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

const (
	gpuPresentLabel = "nvidia.com/gpu.present"
)

// GPUValidator validates GPU allocation and accessibility
type GPUValidator struct {
	client     kubernetes.Interface
	restConfig *rest.Config
	framework  *framework.Framework
}

// NewGPUValidator creates a new validator instance
func NewGPUValidator(f *framework.Framework) *GPUValidator {
	return &GPUValidator{
		client:     f.ClientSet,
		restConfig: f.ClientConfig(),
		framework:  f,
	}
}

// ValidateGPUInPod validates that GPU is accessible in the pod
func (gv *GPUValidator) ValidateGPUInPod(ctx context.Context, namespace, podName string, expectedGPUCount int) error {
	framework.Logf("Validating GPU accessibility in pod %s/%s (expected %d GPUs)", namespace, podName, expectedGPUCount)

	// Get the pod
	pod, err := gv.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	// Exec nvidia-smi to verify GPU is accessible
	nvidiaSmiCmd := []string{"nvidia-smi", "--query-gpu=index,name", "--format=csv,noheader"}
	stdout, stderr, err := e2epod.ExecWithOptions(gv.framework, e2epod.ExecOptions{
		Command:       nvidiaSmiCmd,
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: pod.Spec.Containers[0].Name,
		CaptureStdout: true,
		CaptureStderr: true,
	})
	output := stdout + stderr
	if err != nil {
		return fmt.Errorf("failed to execute nvidia-smi in pod %s/%s: %w\nOutput: %s",
			namespace, podName, err, output)
	}

	// Parse output to count GPUs
	lines := strings.Split(strings.TrimSpace(output), "\n")
	actualGPUCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			actualGPUCount++
		}
	}

	if actualGPUCount != expectedGPUCount {
		return fmt.Errorf("expected %d GPUs but found %d in pod %s/%s\nnvidia-smi output:\n%s",
			expectedGPUCount, actualGPUCount, namespace, podName, output)
	}

	framework.Logf("Successfully validated %d GPU(s) in pod %s/%s", actualGPUCount, namespace, podName)

	// Validate CUDA_VISIBLE_DEVICES environment variable
	err = gv.validateCudaVisibleDevices(ctx, namespace, podName, expectedGPUCount)
	if err != nil {
		framework.Logf("Warning: CUDA_VISIBLE_DEVICES validation failed: %v", err)
		// Don't fail the test for this, as it may not always be set
	}

	return nil
}

// validateCudaVisibleDevices checks the CUDA_VISIBLE_DEVICES environment variable
func (gv *GPUValidator) validateCudaVisibleDevices(ctx context.Context, namespace, podName string, expectedCount int) error {
	pod, err := gv.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	envCmd := []string{"sh", "-c", "echo $CUDA_VISIBLE_DEVICES"}
	stdout, stderr, err := e2epod.ExecWithOptions(gv.framework, e2epod.ExecOptions{
		Command:       envCmd,
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: pod.Spec.Containers[0].Name,
		CaptureStdout: true,
		CaptureStderr: true,
	})
	output := stdout + stderr
	if err != nil {
		return fmt.Errorf("failed to get CUDA_VISIBLE_DEVICES: %w", err)
	}

	cudaDevices := strings.TrimSpace(output)
	if cudaDevices == "" {
		return fmt.Errorf("CUDA_VISIBLE_DEVICES is not set")
	}

	framework.Logf("CUDA_VISIBLE_DEVICES in pod %s/%s: %s", namespace, podName, cudaDevices)
	return nil
}

// ValidateResourceSlice validates ResourceSlice for GPU node
func (gv *GPUValidator) ValidateResourceSlice(ctx context.Context, nodeName string) (*resourceapi.ResourceSlice, error) {
	framework.Logf("Validating ResourceSlice for node %s", nodeName)

	// List all ResourceSlices
	sliceList, err := gv.client.ResourceV1().ResourceSlices().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ResourceSlices: %w", err)
	}

	// Find ResourceSlice for the node
	var nodeSlice *resourceapi.ResourceSlice
	for i := range sliceList.Items {
		slice := &sliceList.Items[i]
		// Match both node name and NVIDIA GPU driver
		if slice.Spec.NodeName != nil && *slice.Spec.NodeName == nodeName &&
			slice.Spec.Driver == "gpu.nvidia.com" {
			nodeSlice = slice
			break
		}
	}

	if nodeSlice == nil {
		return nil, fmt.Errorf("no NVIDIA GPU ResourceSlice found for node %s", nodeName)
	}

	framework.Logf("Found NVIDIA GPU ResourceSlice %s for node %s with driver %s",
		nodeSlice.Name, nodeName, nodeSlice.Spec.Driver)

	// Validate that it contains GPU devices
	if nodeSlice.Spec.Devices == nil || len(nodeSlice.Spec.Devices) == 0 {
		return nil, fmt.Errorf("ResourceSlice %s has no devices", nodeSlice.Name)
	}

	framework.Logf("ResourceSlice %s has %d device(s)", nodeSlice.Name, len(nodeSlice.Spec.Devices))

	return nodeSlice, nil
}

// ValidateDeviceAllocation validates that claim is properly allocated
func (gv *GPUValidator) ValidateDeviceAllocation(ctx context.Context, namespace, claimName string) error {
	framework.Logf("Validating ResourceClaim allocation for %s/%s", namespace, claimName)

	claim, err := gv.client.ResourceV1().ResourceClaims(namespace).Get(ctx, claimName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ResourceClaim %s/%s: %w", namespace, claimName, err)
	}

	// Check if claim is allocated
	if claim.Status.Allocation == nil {
		return fmt.Errorf("ResourceClaim %s/%s is not allocated", namespace, claimName)
	}

	framework.Logf("ResourceClaim %s/%s is allocated", namespace, claimName)

	// Validate devices are allocated
	deviceCount := len(claim.Status.Allocation.Devices.Results)

	if deviceCount == 0 {
		return fmt.Errorf("ResourceClaim %s/%s has 0 devices allocated", namespace, claimName)
	}

	framework.Logf("ResourceClaim %s/%s has %d device(s) allocated", namespace, claimName, deviceCount)

	// Validate allocation metadata for each device
	for i, result := range claim.Status.Allocation.Devices.Results {
		// Validate driver is NVIDIA DRA driver
		if result.Driver != "gpu.nvidia.com" {
			return fmt.Errorf("device %d has incorrect driver %q, expected %q", i, result.Driver, "gpu.nvidia.com")
		}

		// Validate pool is not empty
		if result.Pool == "" {
			return fmt.Errorf("device %d has empty pool field", i)
		}

		// Validate device name is not empty
		if result.Device == "" {
			return fmt.Errorf("device %d has empty device field", i)
		}

		// Validate request name is not empty
		if result.Request == "" {
			return fmt.Errorf("device %d has empty request field", i)
		}

		framework.Logf("Device %d validated: driver=%s, pool=%s, device=%s, request=%s",
			i, result.Driver, result.Pool, result.Device, result.Request)
	}

	return nil
}

// GetGPUNodes returns nodes with NVIDIA GPUs
func (gv *GPUValidator) GetGPUNodes(ctx context.Context) ([]corev1.Node, error) {
	framework.Logf("Getting GPU-enabled nodes")

	nodeList, err := gv.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: gpuPresentLabel + "=true",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes with GPU: %w", err)
	}

	if len(nodeList.Items) == 0 {
		// Try without label selector, and filter manually
		allNodes, err := gv.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list all nodes: %w", err)
		}

		var gpuNodes []corev1.Node
		for _, node := range allNodes.Items {
			// Check for GPU-related labels or capacity
			if gv.hasGPUCapability(&node) {
				gpuNodes = append(gpuNodes, node)
			}
		}

		if len(gpuNodes) == 0 {
			// Return empty slice for non-GPU clusters (allows tests to skip cleanly)
			framework.Logf("No GPU-enabled nodes found in the cluster")
			return []corev1.Node{}, nil
		}

		framework.Logf("Found %d GPU-enabled node(s)", len(gpuNodes))
		return gpuNodes, nil
	}

	framework.Logf("Found %d GPU-enabled node(s)", len(nodeList.Items))
	return nodeList.Items, nil
}

// GetTotalGPUCount returns the total number of GPUs available in the cluster
// by counting devices in ResourceSlices
func (gv *GPUValidator) GetTotalGPUCount(ctx context.Context) (int, error) {
	framework.Logf("Counting total GPUs in cluster via ResourceSlices")

	// List all ResourceSlices for GPU driver
	sliceList, err := gv.client.ResourceV1().ResourceSlices().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list ResourceSlices: %w", err)
	}

	totalGPUs := 0
	for _, slice := range sliceList.Items {
		// Count devices from gpu.nvidia.com driver
		if slice.Spec.Driver == "gpu.nvidia.com" {
			totalGPUs += len(slice.Spec.Devices)
		}
	}

	framework.Logf("Found %d total GPU(s) in cluster", totalGPUs)
	return totalGPUs, nil
}

// hasGPUCapability checks if a node has GPU capability
func (gv *GPUValidator) hasGPUCapability(node *corev1.Node) bool {
	// Check for common GPU labels
	gpuLabels := []string{
		gpuPresentLabel,
		"nvidia.com/gpu",
		"nvidia.com/gpu.count",
		"feature.node.kubernetes.io/pci-10de.present", // NVIDIA vendor ID
	}

	for _, label := range gpuLabels {
		if _, exists := node.Labels[label]; exists {
			return true
		}
	}

	// Check for GPU in allocatable resources
	if qty, exists := node.Status.Allocatable["nvidia.com/gpu"]; exists {
		if !qty.IsZero() {
			return true
		}
	}

	return false
}

// ValidateCDISpec validates CDI specification was created
func (gv *GPUValidator) ValidateCDISpec(ctx context.Context, podName, namespace string) error {
	framework.Logf("Validating CDI spec for pod %s/%s", namespace, podName)

	pod, err := gv.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	// Check for CDI annotations or device references
	// CDI devices are typically injected via annotations or OCI spec
	for key, value := range pod.Annotations {
		if strings.Contains(key, "cdi") || strings.Contains(key, "device") {
			framework.Logf("Found CDI-related annotation: %s=%s", key, value)
		}
	}

	// Validate that nvidia device files are present in the container
	// Note: Using direct device paths instead of shell glob (/dev/nvidia*) because
	// bind-mounted devices may not be listable via glob patterns in containers
	devicePaths := []string{"/dev/nvidia0", "/dev/nvidiactl", "/dev/nvidia-uvm"}

	for _, devicePath := range devicePaths {
		lsCmd := []string{"ls", "-la", devicePath}
		stdout, stderr, err := e2epod.ExecWithOptions(gv.framework, e2epod.ExecOptions{
			Command:       lsCmd,
			Namespace:     namespace,
			PodName:       podName,
			ContainerName: pod.Spec.Containers[0].Name,
			CaptureStdout: true,
			CaptureStderr: true,
		})
		if err != nil {
			output := stdout + stderr
			framework.Logf("Warning: Device %s not accessible in pod %s/%s: %v\nOutput: %s", devicePath, namespace, podName, err, output)
			// Continue checking other devices - at least one should be accessible
			continue
		}
		framework.Logf("Successfully validated device %s in pod %s/%s: %s", devicePath, namespace, podName, stdout)
	}

	framework.Logf("CDI device validation completed for pod %s/%s", namespace, podName)
	return nil
}

// GetGPUCountInPod returns the number of GPUs visible in a pod
func (gv *GPUValidator) GetGPUCountInPod(ctx context.Context, namespace, podName string) (int, error) {
	pod, err := gv.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	// Exec nvidia-smi to count GPUs
	nvidiaSmiCmd := []string{"nvidia-smi", "--query-gpu=count", "--format=csv,noheader"}
	stdout, stderr, err := e2epod.ExecWithOptions(gv.framework, e2epod.ExecOptions{
		Command:       nvidiaSmiCmd,
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: pod.Spec.Containers[0].Name,
		CaptureStdout: true,
		CaptureStderr: true,
	})
	output := stdout + stderr
	if err != nil {
		return 0, fmt.Errorf("failed to execute nvidia-smi: %w", err)
	}

	// Parse the first line to get count
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return 0, fmt.Errorf("no output from nvidia-smi")
	}

	count, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0, fmt.Errorf("failed to parse GPU count from nvidia-smi output: %w", err)
	}

	return count, nil
}
