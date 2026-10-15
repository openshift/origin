package nvidia

import (
	"context"
	"fmt"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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
	oc         *exutil.CLI
}

// NewGPUValidator creates a new validator instance
func NewGPUValidator(f *framework.Framework, cli ...*exutil.CLI) *GPUValidator {
	gv := &GPUValidator{
		client:     f.ClientSet,
		restConfig: f.ClientConfig(),
		framework:  f,
	}
	if len(cli) > 0 {
		gv.oc = cli[0]
	}
	return gv
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
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
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

	return nil
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

// GetMaxGPUCountPerNode reports the largest GPU pool that can satisfy one pod.
func (gv *GPUValidator) GetMaxGPUCountPerNode(ctx context.Context) (int, error) {
	slices, err := gv.client.ResourceV1().ResourceSlices().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list ResourceSlices: %w", err)
	}
	counts := map[string]int{}
	maximum := 0
	for _, slice := range slices.Items {
		if slice.Spec.Driver != resourceBuilderDriver || slice.Spec.NodeName == nil {
			continue
		}
		node := *slice.Spec.NodeName
		counts[node] += len(slice.Spec.Devices)
		if counts[node] > maximum {
			maximum = counts[node]
		}
	}
	return maximum, nil
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

	// The allocated GPU may have any minor number; requiring nvidia0 would reject
	// a valid claim scheduled to a different GPU.
	devicePaths := []string{"/dev/nvidiactl"}

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
			return fmt.Errorf("device %s not accessible in pod %s/%s: %w\nOutput: %s", devicePath, namespace, podName, err, output)
		}
		framework.Logf("Successfully validated device %s in pod %s/%s: %s", devicePath, namespace, podName, stdout)
	}
	stdout, stderr, err := e2epod.ExecWithOptions(gv.framework, e2epod.ExecOptions{
		Command:       []string{"sh", "-c", `set -- /dev/nvidia[0-9]*; test -e "$1"`},
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: pod.Spec.Containers[0].Name,
		CaptureStdout: true,
		CaptureStderr: true,
	})
	if err != nil {
		return fmt.Errorf("no GPU device node accessible in pod %s/%s: %w: %s%s", namespace, podName, err, stdout, stderr)
	}

	framework.Logf("CDI device validation completed for pod %s/%s", namespace, podName)
	return nil
}

// ClaimStatusInfo holds structured status info for a ResourceClaim
type ClaimStatusInfo struct {
	Allocated   bool
	ReservedFor []string
	DeviceCount int
	Devices     []DeviceInfo
}

// DeviceInfo describes an allocated device
type DeviceInfo struct {
	Driver  string
	Pool    string
	Device  string
	Request string
}

// ValidateGPUInContainer validates GPU accessibility in a specific container
func (gv *GPUValidator) ValidateGPUInContainer(ctx context.Context, namespace, podName, containerName string, expectedGPUCount int) error {
	framework.Logf("Validating GPU accessibility in pod %s/%s container %s (expected %d GPUs)", namespace, podName, containerName, expectedGPUCount)

	nvidiaSmiCmd := []string{"nvidia-smi", "--query-gpu=index,name", "--format=csv,noheader"}
	stdout, stderr, err := e2epod.ExecWithOptions(gv.framework, e2epod.ExecOptions{
		Command:       nvidiaSmiCmd,
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: containerName,
		CaptureStdout: true,
		CaptureStderr: true,
	})
	output := stdout + stderr
	if err != nil {
		return fmt.Errorf("failed to execute nvidia-smi in pod %s/%s container %s: %w\nOutput: %s",
			namespace, podName, containerName, err, output)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	actualGPUCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			actualGPUCount++
		}
	}

	if actualGPUCount != expectedGPUCount {
		return fmt.Errorf("expected %d GPUs but found %d in pod %s/%s container %s\nnvidia-smi output:\n%s",
			expectedGPUCount, actualGPUCount, namespace, podName, containerName, output)
	}

	framework.Logf("Successfully validated %d GPU(s) in pod %s/%s container %s", actualGPUCount, namespace, podName, containerName)
	return nil
}

// runOnNode uses the release payload's debug image and runs the command in the host root.
func (gv *GPUValidator) runOnNode(nodeName string, command ...string) (string, error) {
	if gv.oc == nil {
		return "", fmt.Errorf("node inspection requires an OpenShift CLI")
	}
	return exutil.DebugNodeRetryWithOptionsAndChroot(gv.oc, nodeName, "openshift-config", command...)
}

// ValidateCDISpecFilesOnNode checks the CDI spec belonging to a particular claim.
func (gv *GPUValidator) ValidateCDISpecFilesOnNode(ctx context.Context, nodeName, claimUID string, shouldExist bool) error {
	path := "/var/run/cdi/k8s.gpu.nvidia.com-claim_" + claimUID + ".yaml"
	framework.Logf("Checking CDI spec %s on node %s (should exist: %t)", path, nodeName, shouldExist)
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		stdout, err := gv.runOnNode(nodeName, "sh", "-c", `if test -f "$1"; then echo present; else echo absent; fi`, "sh", path)
		if err != nil {
			return false, fmt.Errorf("failed to inspect CDI spec %s on node %s: %w", path, nodeName, err)
		}
		present := strings.TrimSpace(stdout) == "present"
		if !present && strings.TrimSpace(stdout) != "absent" {
			return false, fmt.Errorf("unexpected CDI spec check output %q on node %s", stdout, nodeName)
		}
		return present == shouldExist, nil
	})
}

// ValidateNoSELinuxDenials checks AVCs for the test workload since the supplied node-local time.
func (gv *GPUValidator) ValidateNoSELinuxDenials(ctx context.Context, nodeName, since, processKeyword string) error {
	dateAndTime := strings.SplitN(since, " ", 2)
	if len(dateAndTime) != 2 {
		return fmt.Errorf("invalid node-local audit start time %q", since)
	}
	stdout, err := gv.runOnNode(nodeName, "bash", "-c", `output=$(ausearch -m AVC -ts "$1" "$2" 2>&1); status=$?; if [ "$status" -eq 1 ] && [[ "$output" == *"<no matches>"* ]]; then echo '<no matches>'; exit 0; fi; printf '%s\n' "$output"; exit "$status"`, "bash", dateAndTime[0], dateAndTime[1])
	if err != nil {
		return fmt.Errorf("failed to run ausearch on node %s: %w: %s", nodeName, err, stdout)
	}
	output := strings.TrimSpace(stdout)
	if output == "<no matches>" {
		return nil
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "scontext=") && strings.Contains(line, ":container_t:") && strings.Contains(strings.ToLower(line), strings.ToLower(processKeyword)) {
			return fmt.Errorf("SELinux AVC denial for GPU workload on node %s: %s", nodeName, line)
		}
	}
	return nil
}

// ValidateSELinuxEnforcing verifies that SELinux is in Enforcing mode on the node.
func (gv *GPUValidator) ValidateSELinuxEnforcing(ctx context.Context, nodeName string) error {
	stdout, err := gv.runOnNode(nodeName, "getenforce")
	if err != nil {
		return fmt.Errorf("failed to run getenforce on node %s: %w", nodeName, err)
	}
	if mode := strings.TrimSpace(stdout); mode != "Enforcing" {
		return fmt.Errorf("SELinux is %q on node %s, expected Enforcing", mode, nodeName)
	}
	return nil
}

// GetPodSCC returns the SCC annotation value for a pod
func (gv *GPUValidator) GetPodSCC(ctx context.Context, namespace, podName string) (string, error) {
	pod, err := gv.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	scc := pod.Annotations["openshift.io/scc"]
	framework.Logf("Pod %s/%s SCC annotation: %q", namespace, podName, scc)
	return scc, nil
}

// ValidateClaimStatus returns structured status information for a ResourceClaim
func (gv *GPUValidator) ValidateClaimStatus(ctx context.Context, namespace, claimName string) (*ClaimStatusInfo, error) {
	framework.Logf("Getting ResourceClaim status for %s/%s", namespace, claimName)

	claim, err := gv.client.ResourceV1().ResourceClaims(namespace).Get(ctx, claimName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ResourceClaim %s/%s: %w", namespace, claimName, err)
	}

	info := &ClaimStatusInfo{}
	info.Allocated = claim.Status.Allocation != nil

	for _, ref := range claim.Status.ReservedFor {
		info.ReservedFor = append(info.ReservedFor, string(ref.UID))
	}

	if claim.Status.Allocation != nil {
		results := claim.Status.Allocation.Devices.Results
		info.DeviceCount = len(results)
		for _, result := range results {
			info.Devices = append(info.Devices, DeviceInfo{
				Driver:  result.Driver,
				Pool:    result.Pool,
				Device:  result.Device,
				Request: result.Request,
			})
		}
	}

	framework.Logf("ResourceClaim %s/%s: allocated=%v, reservedFor=%d, devices=%d",
		namespace, claimName, info.Allocated, len(info.ReservedFor), info.DeviceCount)
	return info, nil
}
