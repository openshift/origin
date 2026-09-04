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

var _ = g.Describe("[sig-node][Feature:NVIDIA-DRA][Suite:openshift/nvidia-dra][Serial]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("nvidia-dra-cdi", admissionapi.LevelPrivileged)

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
			g.Skip("No GPU nodes available in the cluster - skipping CRI-O CDI tests")
		}

		if prerequisitesError != nil {
			g.Fail(fmt.Sprintf("Prerequisites validation failed: %v", prerequisitesError))
		}
		if !prerequisitesInstalled {
			g.Skip("Prerequisites not installed - skipping CRI-O CDI tests")
		}
	})

	g.Context("CRI-O CDI Integration", func() {
		g.It("should inject CDI devices into container via CRI-O", func(ctx context.Context) {
			deviceClassName := "test-cdi-inject-" + oc.Namespace()
			claimName := "test-cdi-inject-claim"
			podName := "test-cdi-inject-pod"

			g.By("Creating DeviceClass for NVIDIA GPUs")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err, "Failed to create DeviceClass")
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			g.By("Creating ResourceClaim requesting 1 GPU")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err, "Failed to create ResourceClaim")
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating Pod using the ResourceClaim")
			pod := builder.BuildPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create pod")

			g.By("Waiting for pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Pod failed to start")

			g.By("Verifying /dev/nvidia* devices are present inside container")
			err = validator.ValidateCDISpec(ctx, podName, oc.Namespace())
			framework.ExpectNoError(err, "CDI device injection validation failed")

			g.By("Verifying nvidia-smi works (confirms CDI injection functional)")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "GPU not accessible via CDI injection")

			g.By("Verifying CUDA_VISIBLE_DEVICES environment variable is set")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, podName, metav1.GetOptions{})
			framework.ExpectNoError(err)
			envCmd := []string{"sh", "-c", "echo $CUDA_VISIBLE_DEVICES"}
			stdout, _, err := e2epod.ExecWithOptions(oc.KubeFramework(), e2epod.ExecOptions{
				Command:       envCmd,
				Namespace:     oc.Namespace(),
				PodName:       podName,
				ContainerName: pod.Spec.Containers[0].Name,
				CaptureStdout: true,
				CaptureStderr: true,
			})
			framework.ExpectNoError(err, "Failed to read CUDA_VISIBLE_DEVICES")
			cudaDevices := strings.TrimSpace(stdout)
			framework.Logf("CUDA_VISIBLE_DEVICES=%s", cudaDevices)
		})

		g.It("should have CDI spec files present on the GPU node", func(ctx context.Context) {
			deviceClassName := "test-cdi-spec-" + oc.Namespace()
			claimName := "test-cdi-spec-claim"
			podName := "test-cdi-spec-pod"

			g.By("Creating DeviceClass and ResourceClaim")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating and running GPU pod to trigger CDI spec generation")
			pod := builder.BuildLongRunningPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Pod failed to start")

			g.By("Identifying the GPU node where pod is scheduled")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, podName, metav1.GetOptions{})
			framework.ExpectNoError(err)
			o.Expect(pod.Spec.NodeName).NotTo(o.BeEmpty(), "Pod should be scheduled on a node")

			g.By("Verifying CDI spec files exist under /var/run/cdi/ on the node")
			err = validator.ValidateCDISpecFilesOnNode(ctx, pod.Spec.NodeName)
			framework.ExpectNoError(err, "CDI spec files not found on GPU node")
		})

		g.It("should inject multiple CDI devices into single container", func(ctx context.Context) {
			totalGPUs, err := validator.GetTotalGPUCount(ctx)
			framework.ExpectNoError(err, "Failed to count GPUs")
			if totalGPUs < 2 {
				g.Skip(fmt.Sprintf("Multiple CDI device test requires at least 2 GPUs, but only %d available", totalGPUs))
			}

			deviceClassName := "test-cdi-multi-" + oc.Namespace()
			claimName := "test-cdi-multi-claim"
			podName := "test-cdi-multi-pod"

			g.By("Creating DeviceClass")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err = createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			g.By("Creating ResourceClaim requesting 2 GPUs")
			claim := builder.BuildResourceClaim(claimName, deviceClassName, 2)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating Pod with 2 GPUs")
			pod := builder.BuildPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Pod failed to start with 2 GPUs")

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

			g.By("Creating DeviceClass and ResourceClaim")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating Pod with init container referencing GPU claim")
			pod := builder.BuildPodWithInitContainerAndClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create pod with init container")

			g.By("Waiting for pod to be running (init container must complete first)")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Pod with init container failed to start")

			g.By("Verifying init container completed successfully (nvidia-smi ran)")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, podName, metav1.GetOptions{})
			framework.ExpectNoError(err)
			o.Expect(pod.Status.InitContainerStatuses).NotTo(o.BeEmpty(), "Init container status should be present")
			initStatus := pod.Status.InitContainerStatuses[0]
			o.Expect(initStatus.State.Terminated).NotTo(o.BeNil(), "Init container should have terminated")
			o.Expect(initStatus.State.Terminated.ExitCode).To(o.Equal(int32(0)), "Init container nvidia-smi should exit 0")

			g.By("Verifying main container has GPU access")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "Main container should have GPU access after init container")
		})
	})
})
