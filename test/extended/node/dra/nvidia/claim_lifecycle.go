package nvidia

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node][Feature:NVIDIA-DRA][Suite:openshift/nvidia-dra][Serial]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("nvidia-dra-lifecycle", admissionapi.LevelPrivileged)

	var (
		validator *GPUValidator
		builder   *ResourceBuilder
	)

	g.BeforeEach(func(ctx context.Context) {
		validator = NewGPUValidator(oc.KubeFramework())
		builder = NewResourceBuilder(oc.Namespace())

		nodes, err := validator.GetGPUNodes(ctx)
		if err != nil {
			g.Fail(fmt.Sprintf("Failed to discover GPU nodes: %v", err))
		}
		if len(nodes) == 0 {
			g.Skip("No GPU nodes available in the cluster - skipping ResourceClaim lifecycle tests")
		}

		if prerequisitesError != nil {
			g.Fail(fmt.Sprintf("Prerequisites validation failed: %v", prerequisitesError))
		}
		if !prerequisitesInstalled {
			g.Skip("Prerequisites not installed - skipping ResourceClaim lifecycle tests")
		}
	})

	g.Context("ResourceClaim Lifecycle on Real Hardware", func() {
		g.It("should transition claim through Pending -> Allocated -> Reserved -> Released", func(ctx context.Context) {
			deviceClassName := "test-lifecycle-" + oc.Namespace()
			claimName := "test-lifecycle-claim"
			podName := "test-lifecycle-pod"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			g.By("Creating ResourceClaim and verifying it starts unallocated")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			status, err := validator.ValidateClaimStatus(ctx, oc.Namespace(), claimName)
			framework.ExpectNoError(err)
			o.Expect(status.Allocated).To(o.BeFalse(), "Claim should start unallocated")
			o.Expect(status.ReservedFor).To(o.BeEmpty(), "Claim should have no reservations initially")
			framework.Logf("Claim %s is pending (not yet allocated)", claimName)

			g.By("Creating Pod to trigger allocation and reservation")
			pod := builder.BuildLongRunningPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Pod failed to start")

			g.By("Verifying claim is now Allocated and Reserved")
			status, err = validator.ValidateClaimStatus(ctx, oc.Namespace(), claimName)
			framework.ExpectNoError(err)
			o.Expect(status.Allocated).To(o.BeTrue(), "Claim should be allocated after pod starts")
			o.Expect(status.ReservedFor).NotTo(o.BeEmpty(), "Claim should be reserved for the pod")
			o.Expect(status.DeviceCount).To(o.BeNumerically(">", 0), "Claim should have allocated devices")
			for _, device := range status.Devices {
				o.Expect(device.Driver).To(o.Equal("gpu.nvidia.com"), "Device driver should be NVIDIA")
				o.Expect(device.Pool).NotTo(o.BeEmpty(), "Device pool should not be empty")
				o.Expect(device.Device).NotTo(o.BeEmpty(), "Device name should not be empty")
			}
			framework.Logf("Claim %s is allocated with %d device(s), reserved for %d consumer(s)",
				claimName, status.DeviceCount, len(status.ReservedFor))

			g.By("Deleting pod to trigger release")
			err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, podName, metav1.DeleteOptions{})
			framework.ExpectNoError(err)
			err = e2epod.WaitForPodNotFoundInNamespace(ctx, oc.KubeFramework().ClientSet, podName, oc.Namespace(), 2*time.Minute)
			framework.ExpectNoError(err, "Pod should be deleted")

			g.By("Verifying claim is released (allocated but no longer reserved)")
			err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
				status, err = validator.ValidateClaimStatus(ctx, oc.Namespace(), claimName)
				if err != nil {
					return false, err
				}
				return len(status.ReservedFor) == 0, nil
			})
			framework.ExpectNoError(err, "Claim should be released after pod deletion")
			o.Expect(status.Allocated).To(o.BeTrue(), "Claim should remain allocated after pod deletion")
			o.Expect(status.ReservedFor).To(o.BeEmpty(), "Claim should not be reserved after pod deletion")
			framework.Logf("Claim %s is allocated but released (no reservations)", claimName)
		})

		g.It("should allow multiple containers in a single pod to share one GPU claim", func(ctx context.Context) {
			deviceClassName := "test-multi-container-" + oc.Namespace()
			claimName := "test-multi-container-claim"
			podName := "test-multi-container-pod"

			g.By("Creating DeviceClass and ResourceClaim")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating Pod with two containers sharing the same GPU claim")
			pod := builder.BuildMultiContainerPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create multi-container pod")

			g.By("Waiting for pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Multi-container pod failed to start")

			g.By("Verifying GPU access in first container")
			err = validator.ValidateGPUInContainer(ctx, oc.Namespace(), podName, "gpu-container-1", 1)
			framework.ExpectNoError(err, "First container should have GPU access")

			g.By("Verifying GPU access in second container")
			err = validator.ValidateGPUInContainer(ctx, oc.Namespace(), podName, "gpu-container-2", 1)
			framework.ExpectNoError(err, "Second container should have GPU access")

			g.By("Verifying only one ResourceClaim reservation exists (single NodePrepareResources)")
			status, err := validator.ValidateClaimStatus(ctx, oc.Namespace(), claimName)
			framework.ExpectNoError(err)
			o.Expect(status.Allocated).To(o.BeTrue())
			o.Expect(status.DeviceCount).To(o.Equal(1), "Should have exactly 1 GPU allocated for the shared claim")
			framework.Logf("Multi-container pod shares claim %s with %d device(s)", claimName, status.DeviceCount)
		})

		g.It("should allocate GPU using CEL selector on device attributes", func(ctx context.Context) {
			claimName := "test-cel-selector-claim"
			podName := "test-cel-selector-pod"

			g.By("Creating ResourceClaim with CEL selector targeting NVIDIA driver")
			claim := builder.BuildResourceClaimWithCELSelector(claimName, `device.driver == "gpu.nvidia.com"`, 1)
			err := createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err, "Failed to create ResourceClaim with CEL selector")
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating Pod using the CEL-selected claim")
			pod := builder.BuildPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create pod with CEL selector claim")

			g.By("Waiting for pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Pod with CEL selector claim failed to start")

			g.By("Verifying GPU is accessible")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "GPU should be accessible with CEL-selected claim")

			g.By("Verifying allocation used the correct NVIDIA driver")
			status, err := validator.ValidateClaimStatus(ctx, oc.Namespace(), claimName)
			framework.ExpectNoError(err)
			o.Expect(status.Allocated).To(o.BeTrue())
			o.Expect(status.DeviceCount).To(o.BeNumerically(">", 0))
			for _, device := range status.Devices {
				o.Expect(device.Driver).To(o.Equal("gpu.nvidia.com"),
					"CEL selector should have matched NVIDIA GPU driver")
			}
			framework.Logf("CEL selector correctly allocated %d NVIDIA GPU(s)", status.DeviceCount)
		})
	})
})
