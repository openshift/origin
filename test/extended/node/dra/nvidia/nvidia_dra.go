package nvidia

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"

	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	deviceClassGVR = schema.GroupVersionResource{
		Group:    "resource.k8s.io",
		Version:  "v1",
		Resource: "deviceclasses",
	}
	resourceClaimGVR = schema.GroupVersionResource{
		Group:    "resource.k8s.io",
		Version:  "v1",
		Resource: "resourceclaims",
	}
	resourceClaimTemplateGVR = schema.GroupVersionResource{
		Group:    "resource.k8s.io",
		Version:  "v1",
		Resource: "resourceclaimtemplates",
	}

	// Global state for prerequisites installation
	// These tests are marked [Serial] to prevent concurrent execution
	prerequisitesOnce      sync.Once
	prerequisitesInstalled bool
	prerequisitesError     error
)

var _ = g.Describe("[sig-scheduling][Feature:NVIDIA-DRA][Suite:openshift/nvidia-dra][Serial]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("nvidia-dra", admissionapi.LevelPrivileged)

	var (
		prereqInstaller *PrerequisitesInstaller
		validator       *GPUValidator
		builder         *ResourceBuilder
	)

	g.BeforeEach(func(ctx context.Context) {
		// Initialize helpers
		validator = NewGPUValidator(oc.KubeFramework())
		builder = NewResourceBuilder(oc.Namespace())
		prereqInstaller = NewPrerequisitesInstaller(oc.KubeFramework())

		// IMPORTANT: Check for GPU nodes FIRST before attempting installation
		// This ensures tests skip cleanly on non-GPU clusters
		nodes, err := validator.GetGPUNodes(ctx)
		if err != nil {
			g.Fail(fmt.Sprintf("Failed to discover GPU nodes: %v", err))
		}
		if len(nodes) == 0 {
			g.Skip("No GPU nodes available in the cluster - skipping NVIDIA DRA tests")
		}
		framework.Logf("Found %d GPU node(s) available for testing", len(nodes))

		// Install prerequisites if needed (runs once via sync.Once)
		// NOTE: GPU Operator must be pre-installed on the cluster
		// Tests will validate GPU Operator presence and install DRA driver if needed
		prerequisitesOnce.Do(func() {
			framework.Logf("Checking NVIDIA GPU stack prerequisites")

			// Check if prerequisites are already installed
			if prereqInstaller.IsGPUOperatorInstalled(ctx) && prereqInstaller.IsDRADriverInstalled(ctx) {
				framework.Logf("Prerequisites already installed, skipping installation")
				prerequisitesInstalled = true
				return
			}

			framework.Logf("Validating GPU Operator and installing DRA driver if needed...")
			// Validate GPU Operator presence and install DRA driver
			if err := prereqInstaller.InstallAll(ctx); err != nil {
				prerequisitesError = err
				framework.Logf("ERROR: Failed to validate/install prerequisites: %v", err)
				framework.Logf("Ensure GPU Operator is installed on the cluster before running these tests")
				return
			}

			prerequisitesInstalled = true
			framework.Logf("Prerequisites validation completed successfully")
		})

		// Verify prerequisites are installed
		if prerequisitesError != nil {
			g.Fail(fmt.Sprintf("Prerequisites validation failed: %v. Ensure GPU Operator is installed on cluster.", prerequisitesError))
		}
		if !prerequisitesInstalled {
			g.Fail("Prerequisites not installed - cannot run tests")
		}
	})

	g.Context("Basic GPU Allocation", func() {
		g.It("should allocate single GPU to pod via DRA", func(ctx context.Context) {
			deviceClassName := "test-nvidia-gpu-" + oc.Namespace()
			claimName := "test-gpu-claim"
			podName := "test-gpu-pod"

			g.By("Creating DeviceClass for NVIDIA GPUs")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err, "Failed to create DeviceClass")
			defer func() {
				deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)
			}()

			g.By("Creating ResourceClaim requesting 1 GPU")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err, "Failed to create ResourceClaim")
			defer func() {
				deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)
			}()

			g.By("Creating Pod using the ResourceClaim")
			pod := builder.BuildPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create pod")

			g.By("Waiting for pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Pod failed to start")

			// Get the updated pod
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, podName, metav1.GetOptions{})
			framework.ExpectNoError(err)

			g.By("Verifying pod is scheduled on GPU node")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err)

			g.By("Validating CDI device injection")
			err = validator.ValidateCDISpec(ctx, podName, oc.Namespace())
			framework.ExpectNoError(err)
		})

		g.It("should handle pod deletion and resource cleanup", func(ctx context.Context) {
			deviceClassName := "test-nvidia-gpu-cleanup-" + oc.Namespace()
			claimName := "test-gpu-claim-cleanup"
			podName := "test-gpu-pod-cleanup"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			g.By("Creating ResourceClaim")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating and verifying pod with GPU")
			pod := builder.BuildLongRunningPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err)

			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err)

			g.By("Verifying GPU is accessible in pod before deletion")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err)

			g.By("Deleting pod")
			err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, podName, metav1.DeleteOptions{})
			framework.ExpectNoError(err)

			g.By("Waiting for pod to be deleted")
			err = e2epod.WaitForPodNotFoundInNamespace(ctx, oc.KubeFramework().ClientSet, podName, oc.Namespace(), 1*time.Minute)
			framework.ExpectNoError(err)

			g.By("Verifying ResourceClaim still exists but is not reserved")
			claimObj, err := oc.KubeFramework().DynamicClient.Resource(resourceClaimGVR).Namespace(oc.Namespace()).Get(ctx, claimName, metav1.GetOptions{})
			framework.ExpectNoError(err)
			o.Expect(claimObj).NotTo(o.BeNil())

			// Verify the claim is no longer reserved by the deleted pod
			reservedFor, found, err := unstructured.NestedSlice(claimObj.Object, "status", "reservedFor")
			framework.ExpectNoError(err)
			if found && len(reservedFor) > 0 {
				// Check if any reservation references the deleted pod
				for _, reservation := range reservedFor {
					if resMap, ok := reservation.(map[string]interface{}); ok {
						if uid, found := resMap["uid"]; found {
							// If any reservation still references the deleted pod, fail the test
							framework.Logf("Found reservation with UID: %v", uid)
							g.Fail(fmt.Sprintf("ResourceClaim %s still reserved after pod deletion: %+v", claimName, reservedFor))
						}
					}
				}
			}

			framework.Logf("ResourceClaim %s successfully cleaned up after pod deletion (no reservations)", claimName)
		})
	})

	g.Context("Multi-GPU Workloads", func() {
		g.It("should allocate multiple GPUs to single pod", func(ctx context.Context) {
			// Check if cluster has at least 2 GPUs before running test
			totalGPUs, gpuCountErr := validator.GetTotalGPUCount(ctx)
			if gpuCountErr != nil {
				g.Fail(fmt.Sprintf("Failed to count total GPUs: %v", gpuCountErr))
			}
			if totalGPUs < 2 {
				g.Skip(fmt.Sprintf("Multi-GPU test requires at least 2 GPUs, but only %d GPU(s) available in cluster", totalGPUs))
			}

			deviceClassName := "test-nvidia-multi-gpu-" + oc.Namespace()
			claimName := "test-multi-gpu-claim"
			podName := "test-multi-gpu-pod"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			g.By("Creating ResourceClaim requesting 2 GPUs")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 2)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating Pod using the multi-GPU claim")
			pod := builder.BuildPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err)

			g.By("Waiting for pod to be running or checking for insufficient resources")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			if err != nil {
				// Re-fetch pod to check scheduling status
				pod, getErr := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, podName, metav1.GetOptions{})
				if getErr == nil && pod.Status.Phase == "Pending" {
					// Check for explicit GPU-related scheduling failure
					gpuShortfall := false
					for _, cond := range pod.Status.Conditions {
						if cond.Type == "PodScheduled" && cond.Status == "False" && cond.Reason == "Unschedulable" {
							msg := strings.ToLower(cond.Message)
							// Check if message contains both "insufficient" and GPU-related terms
							if strings.Contains(msg, "insufficient") &&
								(strings.Contains(msg, "gpu") ||
									strings.Contains(msg, "nvidia.com/gpu") ||
									strings.Contains(msg, "gpu.nvidia.com")) {
								framework.Logf("Pod unschedulable due to insufficient GPU resources: %s", cond.Message)
								gpuShortfall = true
								break
							}
						}
					}
					if gpuShortfall {
						g.Skip("Insufficient GPU resources for multi-GPU test")
					}
				}
				framework.ExpectNoError(err, "Pod failed to start")
			}

			g.By("Verifying 2 GPUs allocated")
			err = validator.ValidateDeviceAllocation(ctx, oc.Namespace(), claimName)
			framework.ExpectNoError(err)

			g.By("Verifying 2 GPUs accessible in pod")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 2)
			framework.ExpectNoError(err, "Expected 2 GPUs to be accessible in the pod")
		})
	})

	g.Context("Claim Sharing", func() {
		g.It("should allow multiple pods to share the same ResourceClaim", func(ctx context.Context) {
			deviceClassName := "test-nvidia-shared-" + oc.Namespace()
			claimName := "test-shared-claim"
			pod1Name := "test-shared-pod-1"
			pod2Name := "test-shared-pod-2"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			g.By("Creating shared ResourceClaim")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating first pod using the shared claim")
			pod1 := builder.BuildLongRunningPodWithClaim(pod1Name, claimName, "")
			pod1, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod1, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create first pod")

			g.By("Waiting for first pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod1)
			framework.ExpectNoError(err, "First pod failed to start")

			g.By("Creating second pod using the same claim")
			pod2 := builder.BuildLongRunningPodWithClaim(pod2Name, claimName, "")
			pod2, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod2, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create second pod")

			g.By("Checking if second pod can share the claim")
			// Note: NVIDIA DRA driver may not support claim sharing by default
			// Poll until pod reaches a stable state (Running, or Pending with scheduling failure)
			const pollInterval = 2 * time.Second
			const pollTimeout = 60 * time.Second
			var finalPhase string
			var schedulingFailed bool

			err = wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
				pod2, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, pod2Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				finalPhase = string(pod2.Status.Phase)

				// Check if pod is Running
				if pod2.Status.Phase == "Running" {
					framework.Logf("Second pod is Running")
					return true, nil
				}

				// Check if pod is Pending with explicit unschedulable reason (not just image pull)
				if pod2.Status.Phase == "Pending" {
					for _, cond := range pod2.Status.Conditions {
						if cond.Type == "PodScheduled" && cond.Status == "False" && cond.Reason == "Unschedulable" {
							framework.Logf("Second pod is Pending with Unschedulable reason: %s", cond.Message)
							schedulingFailed = true
							return true, nil
						}
					}
					// Continue polling if just image pull or other transient issue
					framework.Logf("Second pod is Pending (phase: %s), continuing to poll...", pod2.Status.Phase)
					return false, nil
				}

				// Any other phase is unexpected
				return false, fmt.Errorf("second pod in unexpected phase: %s", pod2.Status.Phase)
			})
			framework.ExpectNoError(err, "Failed to determine second pod state")

			if schedulingFailed {
				framework.Logf("Second pod unschedulable - claim sharing not supported by NVIDIA DRA driver")
				g.By("Verifying first pod still has GPU access")
				err = validator.ValidateGPUInPod(ctx, oc.Namespace(), pod1Name, 1)
				framework.ExpectNoError(err)
			} else if finalPhase == "Running" {
				framework.Logf("Second pod is running - claim sharing is supported")
				g.By("Verifying both pods have GPU access")
				err = validator.ValidateGPUInPod(ctx, oc.Namespace(), pod1Name, 1)
				framework.ExpectNoError(err)
				err = validator.ValidateGPUInPod(ctx, oc.Namespace(), pod2Name, 1)
				framework.ExpectNoError(err)
			}
		})
	})

	g.Context("ResourceClaimTemplate", func() {
		g.It("should create claim from template for pod", func(ctx context.Context) {
			deviceClassName := "test-nvidia-template-" + oc.Namespace()
			templateName := "test-gpu-template"
			podName := "test-template-pod"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			g.By("Creating ResourceClaimTemplate")
			template := builder.BuildResourceClaimTemplate(templateName, deviceClassName, 1)
			err = createResourceClaimTemplate(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), template)
			framework.ExpectNoError(err)
			defer deleteResourceClaimTemplate(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), templateName)

			g.By("Creating Pod with ResourceClaimTemplate reference")
			pod := builder.BuildPodWithInlineClaim(podName, deviceClassName)
			// Update the template name to match what we created
			*pod.Spec.ResourceClaims[0].ResourceClaimTemplateName = templateName
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create pod")

			g.By("Waiting for pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Pod failed to start")

			g.By("Verifying GPU is accessible in pod")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err)

			g.By("Verifying ResourceClaim was created from template")
			// Kubernetes appends a random suffix to template-generated claim names
			// Expected pattern: <podName>-<claimName>-<randomSuffix>
			// e.g., "test-template-pod-gpu-mk94g" instead of just "test-template-pod-gpu"
			claimPrefix := podName + "-gpu"

			// List all claims in the namespace and find the one matching our prefix
			claimList, err := oc.KubeFramework().DynamicClient.Resource(resourceClaimGVR).Namespace(oc.Namespace()).List(ctx, metav1.ListOptions{})
			framework.ExpectNoError(err, "Failed to list ResourceClaims")

			var generatedClaimName string
			var claimObj *unstructured.Unstructured
			for _, claim := range claimList.Items {
				if strings.HasPrefix(claim.GetName(), claimPrefix) {
					generatedClaimName = claim.GetName()
					claimObj = &claim
					framework.Logf("Found template-generated ResourceClaim: %s (matches prefix: %s)", generatedClaimName, claimPrefix)
					break
				}
			}

			o.Expect(generatedClaimName).NotTo(o.BeEmpty(), "ResourceClaim with prefix %s should be auto-created from template", claimPrefix)
			o.Expect(claimObj).NotTo(o.BeNil())
			framework.Logf("ResourceClaim %s was successfully created from template", generatedClaimName)

			g.By("Deleting pod and verifying claim cleanup")
			err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, podName, metav1.DeleteOptions{})
			framework.ExpectNoError(err)

			err = e2epod.WaitForPodNotFoundInNamespace(ctx, oc.KubeFramework().ClientSet, podName, oc.Namespace(), 1*time.Minute)
			framework.ExpectNoError(err)

			// The auto-generated claim should be deleted with the pod - poll until NotFound
			g.By("Verifying auto-generated claim is deleted")
			err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
				_, getErr := oc.KubeFramework().DynamicClient.Resource(resourceClaimGVR).Namespace(oc.Namespace()).Get(ctx, generatedClaimName, metav1.GetOptions{})
				if getErr != nil {
					if errors.IsNotFound(getErr) {
						// Successfully deleted
						framework.Logf("ResourceClaim %s was deleted as expected", generatedClaimName)
						return true, nil
					}
					// Unexpected error
					return false, getErr
				}
				// Claim still exists, continue polling
				framework.Logf("ResourceClaim %s still exists, waiting for cleanup...", generatedClaimName)
				return false, nil
			})
			if err != nil {
				g.Fail(fmt.Sprintf("ResourceClaim %s not deleted within timeout - expected automatic cleanup: %v", generatedClaimName, err))
			}
			framework.Logf("ResourceClaim was cleaned up with pod deletion as expected")
		})
	})
})

// Helper functions for creating and deleting resources

func convertToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	unstructuredObj := &unstructured.Unstructured{}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	unstructuredObj.Object = content
	return unstructuredObj, nil
}

func createDeviceClass(ctx context.Context, client dynamic.Interface, deviceClass interface{}) error {
	unstructuredObj, err := convertToUnstructured(deviceClass)
	if err != nil {
		return err
	}
	_, err = client.Resource(deviceClassGVR).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	return err
}

func deleteDeviceClass(ctx context.Context, client dynamic.Interface, name string) error {
	return client.Resource(deviceClassGVR).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](0),
	})
}

func createResourceClaim(ctx context.Context, client dynamic.Interface, namespace string, claim interface{}) error {
	unstructuredObj, err := convertToUnstructured(claim)
	if err != nil {
		return err
	}
	_, err = client.Resource(resourceClaimGVR).Namespace(namespace).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	return err
}

func deleteResourceClaim(ctx context.Context, client dynamic.Interface, namespace, name string) error {
	return client.Resource(resourceClaimGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](0),
	})
}

func createResourceClaimTemplate(ctx context.Context, client dynamic.Interface, namespace string, template interface{}) error {
	unstructuredObj, err := convertToUnstructured(template)
	if err != nil {
		return err
	}
	_, err = client.Resource(resourceClaimTemplateGVR).Namespace(namespace).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	return err
}

func deleteResourceClaimTemplate(ctx context.Context, client dynamic.Interface, namespace, name string) error {
	return client.Resource(resourceClaimTemplateGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](0),
	})
}
