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

	oc := exutil.NewCLIWithPodSecurityLevel("nvidia-dra-security", admissionapi.LevelPrivileged)

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
			g.Skip("No GPU nodes available in the cluster - skipping SELinux/Security tests")
		}

		if prerequisitesError != nil {
			g.Fail(fmt.Sprintf("Prerequisites validation failed: %v", prerequisitesError))
		}
		if !prerequisitesInstalled {
			g.Skip("Prerequisites not installed - skipping SELinux/Security tests")
		}
	})

	g.Context("SELinux and Security", func() {
		g.It("should allow GPU access under SELinux Enforcing mode with no AVC denials", func(ctx context.Context) {
			deviceClassName := "test-selinux-" + oc.Namespace()
			claimName := "test-selinux-claim"
			podName := "test-selinux-pod"

			g.By("Creating DeviceClass and ResourceClaim")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating GPU pod")
			pod := builder.BuildLongRunningPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "GPU pod failed to start")

			g.By("Identifying the GPU node")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, podName, metav1.GetOptions{})
			framework.ExpectNoError(err)
			gpuNode := pod.Spec.NodeName
			o.Expect(gpuNode).NotTo(o.BeEmpty())

			g.By("Verifying SELinux is in Enforcing mode on the GPU node")
			err = validator.ValidateSELinuxEnforcing(ctx, gpuNode)
			framework.ExpectNoError(err, "SELinux should be Enforcing on OpenShift nodes")

			g.By("Verifying nvidia-smi works under SELinux Enforcing")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "GPU should be accessible under SELinux Enforcing")

			g.By("Checking for SELinux AVC denials related to NVIDIA/GPU")
			err = validator.ValidateNoSELinuxDenials(ctx, gpuNode, "nvidia")
			framework.ExpectNoError(err, "No SELinux AVC denials should exist for NVIDIA GPU access")

			err = validator.ValidateNoSELinuxDenials(ctx, gpuNode, "crio")
			framework.ExpectNoError(err, "No SELinux AVC denials should exist for CRI-O CDI processing")
		})

		g.It("should allow GPU access with restricted SCC (non-privileged workload)", func(ctx context.Context) {
			deviceClassName := "test-restricted-scc-" + oc.Namespace()
			claimName := "test-restricted-scc-claim"
			podName := "test-restricted-scc-pod"

			g.By("Creating DeviceClass and ResourceClaim")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating GPU pod with restricted security context (no privilege escalation, drop ALL caps)")
			pod := builder.BuildPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create restricted GPU pod")

			g.By("Waiting for pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Restricted GPU pod failed to start")

			g.By("Verifying GPU access works without privileged SCC")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "GPU should be accessible via CDI without privileged SCC")

			g.By("Verifying pod is NOT running with privileged SCC")
			scc, err := validator.GetPodSCC(ctx, oc.Namespace(), podName)
			framework.ExpectNoError(err)
			o.Expect(scc).NotTo(o.Equal("privileged"),
				"Workload pod should NOT require privileged SCC for GPU access via DRA/CDI")
			framework.Logf("GPU workload pod running with SCC: %s", scc)
		})

		g.It("should allow non-root user to access GPU via DRA claim", func(ctx context.Context) {
			deviceClassName := "test-nonroot-" + oc.Namespace()
			claimName := "test-nonroot-claim"
			podName := "test-nonroot-pod"

			g.By("Creating DeviceClass and ResourceClaim")
			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err := createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			g.By("Creating Pod with runAsNonRoot and non-root UID")
			pod := builder.BuildNonRootPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create non-root GPU pod")

			g.By("Waiting for pod to be running")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err, "Non-root GPU pod failed to start")

			g.By("Verifying the container is running as non-root")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, podName, metav1.GetOptions{})
			framework.ExpectNoError(err)
			idCmd := []string{"id", "-u"}
			stdout, _, err := e2epod.ExecWithOptions(oc.KubeFramework(), e2epod.ExecOptions{
				Command:       idCmd,
				Namespace:     oc.Namespace(),
				PodName:       podName,
				ContainerName: pod.Spec.Containers[0].Name,
				CaptureStdout: true,
				CaptureStderr: true,
			})
			framework.ExpectNoError(err, "Failed to check user ID in container")
			uid := strings.TrimSpace(stdout)
			o.Expect(uid).NotTo(o.Equal("0"), "Container should not be running as root")
			framework.Logf("Container running as UID: %s", uid)

			g.By("Verifying GPU access as non-root user")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "GPU should be accessible as non-root user via DRA/CDI")
		})

		g.It("should use expected SCCs for DRA driver and workload pods", func(ctx context.Context) {
			g.By("Checking NVIDIA DRA driver DaemonSet pods run with privileged SCC")
			driverPods, err := oc.KubeFramework().ClientSet.CoreV1().Pods("nvidia-dra-driver").List(ctx, metav1.ListOptions{
				LabelSelector: "app.kubernetes.io/name=nvidia-dra-driver-gpu",
			})
			framework.ExpectNoError(err, "Failed to list DRA driver pods")
			o.Expect(driverPods.Items).NotTo(o.BeEmpty(), "DRA driver pods should be running")

			for _, driverPod := range driverPods.Items {
				scc, err := validator.GetPodSCC(ctx, driverPod.Namespace, driverPod.Name)
				framework.ExpectNoError(err)
				o.Expect(scc).To(o.Equal("privileged"),
					fmt.Sprintf("DRA driver pod %s should run with privileged SCC", driverPod.Name))
				framework.Logf("DRA driver pod %s running with SCC: %s", driverPod.Name, scc)
			}

			g.By("Creating a GPU workload pod and verifying it does NOT require privileged SCC")
			deviceClassName := "test-scc-audit-" + oc.Namespace()
			claimName := "test-scc-audit-claim"
			podName := "test-scc-audit-pod"

			deviceClass := builder.BuildDeviceClass(deviceClassName)
			err = createDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClass)
			framework.ExpectNoError(err)
			defer deleteDeviceClass(ctx, oc.KubeFramework().DynamicClient, deviceClassName)

			claim := builder.BuildResourceClaim(claimName, deviceClassName, 1)
			err = createResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claim)
			framework.ExpectNoError(err)
			defer deleteResourceClaim(ctx, oc.KubeFramework().DynamicClient, oc.Namespace(), claimName)

			pod := builder.BuildPodWithClaim(podName, claimName, "")
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeFramework().ClientSet, pod)
			framework.ExpectNoError(err)

			workloadSCC, err := validator.GetPodSCC(ctx, oc.Namespace(), podName)
			framework.ExpectNoError(err)
			o.Expect(workloadSCC).NotTo(o.Equal("privileged"),
				"GPU workload pod should NOT need privileged SCC")
			framework.Logf("GPU workload pod %s running with SCC: %s (expected non-privileged)", podName, workloadSCC)
		})
	})
})
