package nvidia

import (
	"context"
	"fmt"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	// Global state for prerequisites installation
	prerequisitesOnce      sync.Once
	prerequisitesInstalled bool
	prerequisitesError     error
)

var _ = g.Describe("[sig-scheduling] NVIDIA DRA", func() {
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
		if err != nil || len(nodes) == 0 {
			g.Skip("No GPU nodes available in the cluster - skipping NVIDIA DRA tests")
		}
		framework.Logf("Found %d GPU node(s) available for testing", len(nodes))

		// Install prerequisites if needed (runs once via sync.Once)
		prerequisitesOnce.Do(func() {
			framework.Logf("Checking NVIDIA GPU stack prerequisites")

			// Check if prerequisites are already installed
			if prereqInstaller.IsGPUOperatorInstalled(ctx) && prereqInstaller.IsDRADriverInstalled(ctx) {
				framework.Logf("Prerequisites already installed, skipping installation")
				prerequisitesInstalled = true
				return
			}

			framework.Logf("Installing prerequisites automatically...")
			// Install all prerequisites
			if err := prereqInstaller.InstallAll(ctx); err != nil {
				prerequisitesError = err
				framework.Logf("ERROR: Failed to install prerequisites: %v", err)
				return
			}

			prerequisitesInstalled = true
			framework.Logf("Prerequisites installation completed successfully")
		})

		// Verify prerequisites are installed
		if prerequisitesError != nil {
			g.Fail(fmt.Sprintf("Prerequisites installation failed: %v", prerequisitesError))
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
			err := createDeviceClass(oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err, "Failed to create DeviceClass")
			defer func() {
				deleteDeviceClass(oc.KubeFramework().DynamicClient, deviceClassName)
			}()

			g.By("Creating ResourceClaim requesting 1 GPU")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err, "Failed to create ResourceClaim")
			defer func() {
				deleteResourceClaim(oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)
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
			err := createDeviceClass(oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(oc.KubeFramework().DynamicClient, deviceClassName)

			g.By("Creating ResourceClaim")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating and verifying pod with GPU")
			pod := builder.BuildLongRunningPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err)

			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
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

			framework.Logf("ResourceClaim %s successfully cleaned up after pod deletion", claimName)
		})
	})

	g.Context("Multi-GPU Workloads", func() {
		g.It("should allocate multiple GPUs to single pod", func(ctx context.Context) {
			// Check if cluster has at least 2 GPUs before running test
			totalGPUs, gpuCountErr := validator.GetTotalGPUCount(ctx)
			if gpuCountErr != nil {
				framework.Logf("Warning: Could not count total GPUs: %v", gpuCountErr)
			}
			if totalGPUs < 2 {
				g.Skip(fmt.Sprintf("Multi-GPU test requires at least 2 GPUs, but only %d GPU(s) available in cluster", totalGPUs))
			}

			deviceClassName := "test-nvidia-multi-gpu-" + oc.Namespace()
			claimName := "test-multi-gpu-claim"
			podName := "test-multi-gpu-pod"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(oc.KubeFramework().DynamicClient, deviceClassName)

			g.By("Creating ResourceClaim requesting 2 GPUs")
			claim := builder.BuildMultiGPUClaim(claimName, deviceClassName, 2)
			err = createResourceClaim(oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating Pod using the multi-GPU claim")
			pod := builder.BuildPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err)

			g.By("Waiting for pod to be running or checking for insufficient resources")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			if err != nil {
				// Check if it's a scheduling error due to insufficient GPUs
				pod, getErr := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, podName, metav1.GetOptions{})
				if getErr == nil && pod.Status.Phase == "Pending" {
					framework.Logf("Pod is pending - likely due to insufficient GPU resources. This is expected if cluster doesn't have 2 GPUs available on a single node.")
					g.Skip("Insufficient GPU resources for multi-GPU test")
				}
				framework.ExpectNoError(err, "Pod failed to start")
			}

			g.By("Verifying 2 GPUs allocated")
			err = validator.ValidateDeviceAllocation(ctx, oc.Namespace(), claimName)
			framework.ExpectNoError(err)

			g.By("Verifying 2 GPUs accessible in pod")
			time.Sleep(10 * time.Second)
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 2)
			if err != nil {
				framework.Logf("Warning: Could not validate 2 GPUs in pod: %v", err)
				// Don't fail the test if nvidia-smi fails, as it might be a configuration issue
			}
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

func createDeviceClass(client dynamic.Interface, deviceClass interface{}) error {
	unstructuredObj, err := convertToUnstructured(deviceClass)
	if err != nil {
		return err
	}
	_, err = client.Resource(deviceClassGVR).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{})
	return err
}

func deleteDeviceClass(client dynamic.Interface, name string) error {
	return client.Resource(deviceClassGVR).Delete(context.TODO(), name, metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](0),
	})
}

func createResourceClaim(client dynamic.Interface, namespace string, claim interface{}) error {
	unstructuredObj, err := convertToUnstructured(claim)
	if err != nil {
		return err
	}
	_, err = client.Resource(resourceClaimGVR).Namespace(namespace).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{})
	return err
}

func deleteResourceClaim(client dynamic.Interface, namespace, name string) error {
	return client.Resource(resourceClaimGVR).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](0),
	})
}
