package nvidia

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	oc := exutil.NewCLIWithPodSecurityLevel("nvidia-dra-cdi", admissionapi.LevelPrivileged)

	var (
		validator *GPUValidator
		builder   *ResourceBuilder
	)

	g.BeforeEach(func(ctx context.Context) {
		validator, builder = setupGPUTest(ctx, oc, "CRI-O CDI")
	})

	g.Context("CRI-O CDI Integration", func() {
		g.It("should inject CDI devices into container via CRI-O", func(ctx context.Context) {
			deviceClassName := "test-cdi-inject-" + oc.Namespace()
			claimName := "test-cdi-inject-claim"
			podName := "test-cdi-inject-pod"

			createGPUClaim(ctx, oc, builder, deviceClassName, claimName, 1)
			pod := startGPUPod(ctx, oc, oc.KubeFramework().ClientSet,
				builder.BuildPodWithClaim(podName, claimName, ""), claimName, "CDI injection")

			g.By("Verifying /dev/nvidia* devices are present inside container")
			err := validator.ValidateCDISpec(ctx, podName, oc.Namespace())
			framework.ExpectNoError(err, "CDI device injection validation failed")

			g.By("Verifying nvidia-smi can access the allocated GPU")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "GPU not accessible via CDI injection")

			// nvidia-smi working does not prove CDI was used: with CDI disabled the GPU Operator
			// puts CRI-O in legacy hook mode, and that hook injects GPUs too. The DRA driver's CDI
			// spec overrides NVIDIA_VISIBLE_DEVICES with "void", so this is what tells the two
			// injection paths apart.
			g.By("Verifying the DRA CDI spec set NVIDIA_VISIBLE_DEVICES=void")
			stdout, _, err := e2epod.Exec(oc.KubeFramework().TContext(ctx), e2epod.ExecOptions{
				Command:       []string{"cat", "/proc/1/environ"},
				Namespace:     oc.Namespace(),
				PodName:       podName,
				ContainerName: pod.Spec.Containers[0].Name,
				CaptureStdout: true,
				CaptureStderr: true,
			})
			framework.ExpectNoError(err, "Failed to read the container process environment")
			o.Expect(strings.Split(stdout, "\x00")).To(o.ContainElement("NVIDIA_VISIBLE_DEVICES=void"),
				"The DRA CDI spec should set NVIDIA_VISIBLE_DEVICES=void")
		})

		g.It("should have CDI spec files present on the GPU node", func(ctx context.Context) {
			deviceClassName := "test-cdi-spec-" + oc.Namespace()
			claimName := "test-cdi-spec-claim"
			podName := "test-cdi-spec-pod"

			createGPUClaim(ctx, oc, builder, deviceClassName, claimName, 1)
			pod := startGPUPod(ctx, oc, oc.KubeFramework().ClientSet,
				builder.BuildLongRunningPodWithClaim(podName, claimName, ""), claimName, "CDI spec")

			g.By("Identifying the GPU node where pod is scheduled")
			o.Expect(pod.Spec.NodeName).NotTo(o.BeEmpty(), "Pod should be scheduled on a node")

			// The DRA driver writes one spec per claim, named after the claim UID. Listing
			// /var/run/cdi would also match GPU Operator specs and leftovers from earlier runs.
			claim, err := oc.KubeFramework().ClientSet.ResourceV1().ResourceClaims(oc.Namespace()).Get(ctx, claimName, metav1.GetOptions{})
			framework.ExpectNoError(err, "Failed to get the ResourceClaim UID")
			g.By("Verifying the claim's CDI spec exists while the pod runs")
			err = validator.ValidateCDISpecFilesOnNode(ctx, pod.Spec.NodeName, string(claim.UID), true)
			framework.ExpectNoError(err, "Claim CDI spec was not found on the GPU node")

			// The driver only logs a failure to remove the spec on unprepare, so a leak is
			// invisible unless the test looks for it.
			g.By("Deleting the pod and verifying its CDI spec is removed")
			err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Delete(ctx, podName, metav1.DeleteOptions{})
			framework.ExpectNoError(err, "Failed to delete CDI workload pod")
			err = e2epod.WaitForPodNotFoundInNamespace(ctx, oc.KubeFramework().ClientSet, podName, oc.Namespace(), 2*time.Minute)
			framework.ExpectNoError(err, "CDI workload pod was not deleted")
			err = validator.ValidateCDISpecFilesOnNode(ctx, pod.Spec.NodeName, string(claim.UID), false)
			framework.ExpectNoError(err, "Claim CDI spec remained on the GPU node after pod deletion")
		})

		g.It("should inject multiple CDI devices into single container", func(ctx context.Context) {
			maxGPUs, err := validator.GetMaxGPUCountPerNode(ctx)
			framework.ExpectNoError(err, "Failed to count GPUs per node")
			if maxGPUs < 2 {
				g.Skip(fmt.Sprintf("Multiple CDI device test requires 2 GPUs on one node, but the largest GPU pool has %d", maxGPUs))
			}

			deviceClassName := "test-cdi-multi-" + oc.Namespace()
			claimName := "test-cdi-multi-claim"
			podName := "test-cdi-multi-pod"

			createGPUClaim(ctx, oc, builder, deviceClassName, claimName, 2)
			startGPUPod(ctx, oc, oc.KubeFramework().ClientSet,
				builder.BuildPodWithClaim(podName, claimName, ""), claimName, "multi-GPU")

			g.By("Verifying both GPUs accessible via nvidia-smi")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 2)
			framework.ExpectNoError(err, "Expected 2 GPUs accessible in container")

			g.By("Verifying ResourceClaim has 2 devices allocated")
			err = validator.ValidateDeviceAllocation(ctx, oc.Namespace(), claimName)
			framework.ExpectNoError(err, "Claim allocation validation failed")
		})

		g.It("should inject CDI devices into init containers", func(ctx context.Context) {
			deviceClassName := "test-cdi-init-" + oc.Namespace()
			claimName := "test-cdi-init-claim"
			podName := "test-cdi-init-pod"

			createGPUClaim(ctx, oc, builder, deviceClassName, claimName, 1)
			pod := startGPUPod(ctx, oc, oc.KubeFramework().ClientSet,
				builder.BuildPodWithInitContainerAndClaim(podName, claimName, ""), claimName, "init container")

			g.By("Verifying init container completed successfully (nvidia-smi ran)")
			o.Expect(pod.Status.InitContainerStatuses).NotTo(o.BeEmpty(), "Init container status should be present")
			initStatus := pod.Status.InitContainerStatuses[0]
			o.Expect(initStatus.State.Terminated).NotTo(o.BeNil(), "Init container should have terminated")
			o.Expect(initStatus.State.Terminated.ExitCode).To(o.Equal(int32(0)), "Init container nvidia-smi should exit 0")

			g.By("Verifying main container has GPU access")
			err := validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "Main container should have GPU access after init container")
		})
	})
})
