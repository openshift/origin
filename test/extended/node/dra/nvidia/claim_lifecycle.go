package nvidia

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node][Feature:NVIDIA-DRA][Suite:openshift/nvidia-dra][Serial][Skipped:Disconnected]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("nvidia-dra-lifecycle", admissionapi.LevelPrivileged)

	var (
		validator *GPUValidator
		builder   *ResourceBuilder
	)

	g.BeforeEach(func(ctx context.Context) {
		validator, builder = setupGPUTest(ctx, oc, "ResourceClaim sharing")
	})

	g.Context("ResourceClaim Sharing on Real Hardware", func() {
		// Upstream already covers two containers referencing one claim with a mock driver. What
		// only real hardware can show is that both containers were given the same physical GPU,
		// which is why this compares device UUIDs rather than device counts.
		g.It("should allow multiple containers in a single pod to share one GPU claim", func(ctx context.Context) {
			deviceClassName := "test-multi-container-" + oc.Namespace()
			claimName := "test-multi-container-claim"
			podName := "test-multi-container-pod"

			createGPUClaim(ctx, oc, builder, deviceClassName, claimName, 1)
			startGPUPod(ctx, oc, oc.KubeFramework().ClientSet,
				builder.BuildMultiContainerPodWithClaim(podName, claimName, ""), claimName, "multi-container")

			g.By("Verifying GPU access in first container")
			err := validator.ValidateGPUInContainer(ctx, oc.Namespace(), podName, "gpu-container-1", 1)
			framework.ExpectNoError(err, "First container should have GPU access")

			g.By("Verifying GPU access in second container")
			err = validator.ValidateGPUInContainer(ctx, oc.Namespace(), podName, "gpu-container-2", 1)
			framework.ExpectNoError(err, "Second container should have GPU access")

			g.By("Verifying only one ResourceClaim reservation exists (single NodePrepareResources)")
			status, err := validator.ValidateClaimStatus(ctx, oc.Namespace(), claimName)
			framework.ExpectNoError(err, "Failed to read status of shared ResourceClaim %s", claimName)
			o.Expect(status.Allocated).To(o.BeTrue())
			o.Expect(status.DeviceCount).To(o.Equal(1), "Should have exactly 1 GPU allocated for the shared claim")
			o.Expect(status.ReservedFor).To(o.HaveLen(1), "The shared claim should have one pod reservation")

			g.By("Verifying both containers see the UUID of the allocated GPU")
			slices, err := oc.KubeFramework().ClientSet.ResourceV1().ResourceSlices().List(ctx, metav1.ListOptions{})
			framework.ExpectNoError(err, "Failed to list ResourceSlices")
			allocated := status.Devices[0]
			var allocatedUUID string
			for _, slice := range slices.Items {
				if slice.Spec.Driver != allocated.Driver || slice.Spec.Pool.Name != allocated.Pool {
					continue
				}
				for _, device := range slice.Spec.Devices {
					if device.Name != allocated.Device {
						continue
					}
					for key, attr := range device.Attributes {
						if strings.TrimPrefix(string(key), "gpu.nvidia.com/") == "uuid" && attr.StringValue != nil {
							allocatedUUID = *attr.StringValue
						}
					}
				}
			}
			o.Expect(allocatedUUID).NotTo(o.BeEmpty(), "Allocated GPU should publish a UUID in its ResourceSlice")
			for _, containerName := range []string{"gpu-container-1", "gpu-container-2"} {
				stdout, stderr, execErr := e2epod.Exec(oc.KubeFramework().TContext(ctx), e2epod.ExecOptions{
					Command:       []string{"nvidia-smi", "--query-gpu=uuid", "--format=csv,noheader"},
					Namespace:     oc.Namespace(),
					PodName:       podName,
					ContainerName: containerName,
					CaptureStdout: true,
					CaptureStderr: true,
				})
				framework.ExpectNoError(execErr, "Failed to read GPU UUID from %s: %s", containerName, stderr)
				o.Expect(strings.TrimSpace(stdout)).To(o.Equal(allocatedUUID), "%s should see the allocated GPU", containerName)
			}
			framework.Logf("Multi-container pod shares claim %s with %d device(s)", claimName, status.DeviceCount)
		})
	})

	// Upstream covers request-level CEL selectors against a mock driver's attributes. These two
	// specs run the same mechanism against the capacity the real NVIDIA driver publishes.
	//
	// The pair is deliberate: both use the same attribute path and differ only in the threshold.
	// A wrong path makes CEL match nothing, which would let the negative spec pass for entirely
	// the wrong reason, so the matching spec is what keeps it honest.
	g.Context("Request-level CEL Selectors on Real Hardware", func() {
		const gpuMemoryCapacity = `device.capacity["gpu.nvidia.com"].memory.compareTo(quantity("%s")) >= 0`

		g.It("should allocate a GPU matching a request-level CEL selector", func(ctx context.Context) {
			deviceClassName := "test-cel-match-" + oc.Namespace()
			claimName := "test-cel-match-claim"
			podName := "test-cel-match-pod"

			createGPUClaimWithSelector(ctx, oc, builder, deviceClassName, claimName,
				fmt.Sprintf(gpuMemoryCapacity, "1Gi"), 1)
			startGPUPod(ctx, oc, oc.KubeFramework().ClientSet,
				builder.BuildPodWithClaim(podName, claimName, ""), claimName, "CEL selector")

			g.By("Verifying the claim was allocated an NVIDIA GPU")
			err := validator.ValidateDeviceAllocation(ctx, oc.Namespace(), claimName)
			framework.ExpectNoError(err, "Claim %s was not allocated", claimName)

			g.By("Verifying the selected GPU is accessible")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "GPU should be accessible via a CEL-selected claim")
		})

		g.It("should leave a pod unschedulable when no GPU matches its request-level CEL selector", func(ctx context.Context) {
			deviceClassName := "test-cel-nomatch-" + oc.Namespace()
			claimName := "test-cel-nomatch-claim"
			podName := "test-cel-nomatch-pod"

			// No real GPU has this much memory. If the request-level selector were ignored, the
			// DeviceClass alone would still match every NVIDIA GPU and the pod would schedule.
			createGPUClaimWithSelector(ctx, oc, builder, deviceClassName, claimName,
				fmt.Sprintf(gpuMemoryCapacity, "100Ti"), 1)

			g.By("Creating a pod whose claim no device can satisfy")
			pod := builder.BuildPodWithClaim(podName, claimName, "")
			_, err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create unsatisfiable GPU pod %s", podName)
			registerPodCleanup(oc.KubeFramework().ClientSet, oc.Namespace(), podName, claimName)

			g.By("Verifying the pod stays unschedulable")
			err = e2epod.WaitForPodNameUnschedulableInNamespace(ctx, oc.KubeFramework().ClientSet, podName, oc.Namespace())
			framework.ExpectNoError(err, "Pod %s should stay unschedulable when no GPU matches its selector", podName)

			g.By("Verifying the ResourceClaim was not allocated")
			status, err := validator.ValidateClaimStatus(ctx, oc.Namespace(), claimName)
			framework.ExpectNoError(err, "Failed to read status of ResourceClaim %s", claimName)
			o.Expect(status.Allocated).To(o.BeFalse(),
				"ResourceClaim should not be allocated when no GPU matches its request-level selector")
		})
	})
})
