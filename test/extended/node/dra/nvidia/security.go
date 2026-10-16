package nvidia

import (
	"context"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

// These specs run in a namespace at pod-security level Restricted and create their workloads as
// the project user. SCC admission picks the first SCC the requesting user may use, so an
// admin-created pod could be admitted under any SCC in the cluster and would not show that
// restricted-v2 is enough for GPU access.
var _ = g.Describe("[sig-node][Feature:NVIDIA-DRA][Suite:openshift/nvidia-dra][Serial][Skipped:Disconnected]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("nvidia-dra-security", admissionapi.LevelRestricted)

	var (
		validator *GPUValidator
		builder   *ResourceBuilder
	)

	g.BeforeEach(func(ctx context.Context) {
		validator, builder = setupGPUTest(ctx, oc, "SELinux/Security")
	})

	g.Context("SELinux and Security", func() {
		g.It("should allow GPU access under SELinux Enforcing mode with no AVC denials", func(ctx context.Context) {
			deviceClassName := "test-selinux-" + oc.Namespace()
			claimName := "test-selinux-claim"
			podName := "test-selinux-pod"

			createGPUClaim(ctx, oc, builder, deviceClassName, claimName, 1)

			// Record the time on each GPU node before scheduling; the scheduler picks the node
			// later, and ausearch takes a node-local date and time rather than an epoch.
			nodes, err := validator.GetGPUNodes(ctx)
			framework.ExpectNoError(err, "Failed to find GPU nodes for AVC audit")
			startTimes := make(map[string]string, len(nodes))
			for _, node := range nodes {
				startTimes[node.Name], err = validator.runOnNode(node.Name, "date", "+%x %X")
				framework.ExpectNoError(err, "Failed to record AVC audit start time on node %s", node.Name)
				startTimes[node.Name] = strings.TrimSpace(startTimes[node.Name])
			}

			pod := startGPUPod(ctx, oc, oc.KubeClient(),
				builder.BuildLongRunningPodWithClaim(podName, claimName, ""), claimName, "SELinux")

			gpuNode := pod.Spec.NodeName
			o.Expect(gpuNode).NotTo(o.BeEmpty(), "GPU pod should be scheduled on a node")

			g.By("Verifying SELinux is in Enforcing mode on the GPU node")
			err = validator.ValidateSELinuxEnforcing(ctx, gpuNode)
			framework.ExpectNoError(err, "SELinux should be Enforcing on OpenShift nodes")

			g.By("Verifying nvidia-smi works under SELinux Enforcing")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "GPU should be accessible under SELinux Enforcing")

			// CRI-O leaves the SELinux label empty for privileged pods and for pods with
			// hostPID/hostIPC, so without this the test would also pass for an unconfined
			// workload, where GPU access proves nothing about SELinux.
			g.By("Verifying the GPU workload runs in the confined container_t domain")
			stdout, _, err := e2epod.ExecWithOptions(oc.KubeFramework(), e2epod.ExecOptions{
				Command:       []string{"cat", "/proc/self/attr/current"},
				Namespace:     oc.Namespace(),
				PodName:       podName,
				ContainerName: pod.Spec.Containers[0].Name,
				CaptureStdout: true,
				CaptureStderr: true,
			})
			framework.ExpectNoError(err, "Failed to read workload SELinux domain")
			o.Expect(stdout).To(o.ContainSubstring(":container_t:"), "GPU workload should be confined")

			g.By("Checking for workload SELinux AVC denials related to NVIDIA/GPU")
			err = validator.ValidateNoSELinuxDenials(ctx, gpuNode, startTimes[gpuNode], "nvidia")
			framework.ExpectNoError(err, "No SELinux AVC denials should exist for NVIDIA GPU access")
		})

		g.It("should allow GPU access with restricted SCC (non-privileged workload)", func(ctx context.Context) {
			deviceClassName := "test-restricted-scc-" + oc.Namespace()
			claimName := "test-restricted-scc-claim"
			podName := "test-restricted-scc-pod"

			createGPUClaim(ctx, oc, builder, deviceClassName, claimName, 1)
			startGPUPod(ctx, oc, oc.KubeClient(),
				builder.BuildPodWithClaim(podName, claimName, ""), claimName, "restricted")

			g.By("Verifying GPU access works without privileged SCC")
			err := validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "GPU should be accessible via CDI without privileged SCC")

			// restricted-v2 is the SCC bound to system:authenticated. restricted-v3 is too, but it
			// requires a pod-level user namespace and so cannot match this pod.
			g.By("Verifying the workload was admitted under restricted-v2")
			scc, err := validator.GetPodSCC(ctx, oc.Namespace(), podName)
			framework.ExpectNoError(err, "Failed to read SCC for restricted workload %s", podName)
			o.Expect(scc).To(o.Equal("restricted-v2"),
				"GPU workload should be admitted under restricted-v2")
			framework.Logf("GPU workload pod running with SCC: %s", scc)
		})

		g.It("should allow non-root user to access GPU via DRA claim", func(ctx context.Context) {
			deviceClassName := "test-nonroot-" + oc.Namespace()
			claimName := "test-nonroot-claim"
			podName := "test-nonroot-pod"

			createGPUClaim(ctx, oc, builder, deviceClassName, claimName, 1)
			// The pod leaves runAsUser unset so restricted-v2 assigns a UID from the namespace's
			// range, which is the case typical OpenShift users hit. A hard-coded UID would be
			// outside that range and could only be admitted under a more permissive SCC.
			pod := startGPUPod(ctx, oc, oc.KubeClient(),
				builder.BuildPodWithClaim(podName, claimName, ""), claimName, "non-root")

			g.By("Verifying the container is running as non-root")
			stdout, _, err := e2epod.ExecWithOptions(oc.KubeFramework(), e2epod.ExecOptions{
				Command:       []string{"id", "-u"},
				Namespace:     oc.Namespace(),
				PodName:       podName,
				ContainerName: pod.Spec.Containers[0].Name,
				CaptureStdout: true,
				CaptureStderr: true,
			})
			framework.ExpectNoError(err, "Failed to check user ID in container")
			uid := strings.TrimSpace(stdout)
			o.Expect(uid).To(o.MatchRegexp(`^[1-9][0-9]*$`), "Container should run with a non-root UID")
			framework.Logf("Container running as UID: %s", uid)
			scc, err := validator.GetPodSCC(ctx, oc.Namespace(), podName)
			framework.ExpectNoError(err, "Failed to read SCC for non-root workload")
			o.Expect(scc).To(o.Equal("restricted-v2"), "The project user should use restricted-v2")

			g.By("Verifying GPU access as non-root user")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "GPU should be accessible as non-root user via DRA/CDI")
		})
	})
})
