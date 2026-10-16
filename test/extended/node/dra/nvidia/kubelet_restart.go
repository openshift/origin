package nvidia

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

// restartKubelet restarts kubelet on the node and waits for the service to come back under a
// new PID.
//
// The Node's Ready condition is not a usable signal here: a kubelet restart normally completes
// well inside the node-status grace period, so the node never reports NotReady and a
// NotReady-then-Ready wait would pass without the restart having happened at all.
func restartKubelet(ctx context.Context, validator *GPUValidator, nodeName string) {
	g.By(fmt.Sprintf("Restarting kubelet on node %s", nodeName))

	before, err := validator.runOnNode(nodeName, "systemctl", "show", "-p", "MainPID", "--value", "kubelet")
	framework.ExpectNoError(err, "Failed to read the kubelet PID on node %s", nodeName)
	before = strings.TrimSpace(before)
	o.Expect(before).NotTo(o.Or(o.BeEmpty(), o.Equal("0")), "kubelet is not running on node %s", nodeName)
	framework.Logf("kubelet MainPID on %s before the restart: %s", nodeName, before)

	// --no-block returns once systemd has queued the job, before kubelet is torn down. Without
	// it the teardown kills this very debug pod's exec stream and the restart surfaces as a
	// command failure that cannot be told apart from a real one.
	if _, err := validator.runOnNode(nodeName, "systemctl", "restart", "--no-block", "kubelet"); err != nil {
		framework.Logf("Restart command on %s returned %v; confirming the restart took effect", nodeName, err)
	}

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		// While kubelet is down the debug pod cannot be created, so these calls fail and the
		// poll simply retries.
		active, err := validator.runOnNode(nodeName, "systemctl", "is-active", "kubelet")
		if err != nil || strings.TrimSpace(active) != "active" {
			return false, nil
		}
		current, err := validator.runOnNode(nodeName, "systemctl", "show", "-p", "MainPID", "--value", "kubelet")
		if err != nil {
			return false, nil
		}
		current = strings.TrimSpace(current)
		if current == "0" || current == before {
			return false, nil
		}
		framework.Logf("kubelet MainPID on %s after the restart: %s", nodeName, current)
		return true, nil
	})
	framework.ExpectNoError(err, "kubelet did not come back under a new PID on node %s", nodeName)
}

// Kubelet restart with an active GPU claim is the DRA path that upstream can only exercise with
// a mock driver. On real hardware the NVIDIA kubelet plugin has to re-register with the plugin
// watcher, kubelet has to recover the prepared claim from its checkpoint, and CRI-O has to keep
// serving the already-injected CDI devices to a container it did not recreate.
var _ = g.Describe("[sig-node][Feature:NVIDIA-DRA][Suite:openshift/nvidia-dra][Serial][Disruptive][Skipped:Disconnected]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("nvidia-dra-kubelet-restart", admissionapi.LevelPrivileged)

	var (
		validator *GPUValidator
		builder   *ResourceBuilder
	)

	g.BeforeEach(func(ctx context.Context) {
		validator, builder = setupGPUTest(ctx, oc, "kubelet restart")
	})

	g.Context("Kubelet Restart", func() {
		g.It("should keep a running GPU workload and its claim across a kubelet restart", func(ctx context.Context) {
			deviceClassName := "test-kubelet-restart-" + oc.Namespace()
			claimName := "test-kubelet-restart-claim"
			podName := "test-kubelet-restart-pod"

			createGPUClaim(ctx, oc, builder, deviceClassName, claimName, 1)
			pod := startGPUPod(ctx, oc, oc.KubeFramework().ClientSet,
				builder.BuildLongRunningPodWithClaim(podName, claimName, ""), claimName, "kubelet restart")

			g.By("Recording the container identity before the restart")
			nodeName := pod.Spec.NodeName
			o.Expect(nodeName).NotTo(o.BeEmpty(), "GPU pod should be scheduled on a node")
			o.Expect(pod.Status.ContainerStatuses).NotTo(o.BeEmpty(), "GPU pod should report a container status")
			containerID := pod.Status.ContainerStatuses[0].ContainerID
			restartCount := pod.Status.ContainerStatuses[0].RestartCount
			o.Expect(containerID).NotTo(o.BeEmpty(), "Running GPU container should report a container ID")
			podUID := string(pod.UID)

			restartKubelet(ctx, validator, nodeName)

			g.By("Waiting for the node to report Ready")
			o.Expect(e2enode.WaitForNodeToBeReady(ctx, oc.KubeFramework().ClientSet, nodeName, 5*time.Minute)).To(o.BeTrue(),
				"Node %s did not report Ready after the kubelet restart", nodeName)

			g.By("Verifying the workload container was recovered, not recreated")
			err := e2epod.WaitForPodNameRunningInNamespace(ctx, oc.KubeFramework().ClientSet, podName, oc.Namespace())
			framework.ExpectNoError(err, "GPU pod %s is not Running after the kubelet restart", podName)
			pod, err = oc.KubeFramework().ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, podName, metav1.GetOptions{})
			framework.ExpectNoError(err, "Failed to get GPU pod %s after the kubelet restart", podName)
			o.Expect(pod.Status.ContainerStatuses).NotTo(o.BeEmpty(), "GPU pod should report a container status after the restart")
			o.Expect(pod.Status.ContainerStatuses[0].ContainerID).To(o.Equal(containerID),
				"kubelet recreated the GPU container instead of recovering the prepared claim from its checkpoint")
			o.Expect(pod.Status.ContainerStatuses[0].RestartCount).To(o.Equal(restartCount),
				"GPU container restarted across the kubelet restart")

			g.By("Verifying the ResourceClaim is still allocated and reserved for the pod")
			status, err := validator.ValidateClaimStatus(ctx, oc.Namespace(), claimName)
			framework.ExpectNoError(err, "Failed to read status of ResourceClaim %s after the kubelet restart", claimName)
			o.Expect(status.Allocated).To(o.BeTrue(), "ResourceClaim lost its allocation across the kubelet restart")
			o.Expect(status.ReservedFor).To(o.ContainElement(podUID),
				"ResourceClaim is no longer reserved for the surviving GPU pod")

			g.By("Verifying the injected CDI devices still work in the surviving container")
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "GPU is not accessible after the kubelet restart")
		})

		g.It("should prepare a new GPU claim after a kubelet restart", func(ctx context.Context) {
			deviceClassName := "test-post-restart-" + oc.Namespace()
			claimName := "test-post-restart-claim"
			podName := "test-post-restart-pod"

			g.By("Selecting a GPU node")
			nodes, err := validator.GetGPUNodes(ctx)
			framework.ExpectNoError(err, "Failed to discover GPU nodes")
			o.Expect(nodes).NotTo(o.BeEmpty(), "At least one GPU node is required")
			nodeName := nodes[0].Name

			restartKubelet(ctx, validator, nodeName)

			g.By("Waiting for the node to report Ready")
			o.Expect(e2enode.WaitForNodeToBeReady(ctx, oc.KubeFramework().ClientSet, nodeName, 5*time.Minute)).To(o.BeTrue(),
				"Node %s did not report Ready after the kubelet restart", nodeName)

			createGPUClaim(ctx, oc, builder, deviceClassName, claimName, 1)

			pod := builder.BuildPodWithClaim(podName, claimName, "")
			// Pin with a metadata.name field match rather than the kubernetes.io/hostname label,
			// which carries the kubelet's hostname and can be overridden. The scheduler still
			// runs, so the claim is allocated normally.
			pod.Spec.Affinity = &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchFields: []corev1.NodeSelectorRequirement{
									{
										Key:      "metadata.name",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{nodeName},
									},
								},
							},
						},
					},
				},
			}
			// Reaching Running requires NodePrepareResources to have succeeded, so a failure here
			// means the DRA kubelet plugin did not re-register with the plugin watcher. Asserting
			// that a ResourceSlice still exists would not show this: slices are cluster-scoped
			// objects that outlive the restart whether or not the plugin came back.
			startGPUPod(ctx, oc, oc.KubeFramework().ClientSet, pod, claimName, "post-restart")

			g.By("Verifying the new claim was allocated and the GPU is usable")
			err = validator.ValidateDeviceAllocation(ctx, oc.Namespace(), claimName)
			framework.ExpectNoError(err, "Claim %s was not allocated after the kubelet restart", claimName)
			err = validator.ValidateGPUInPod(ctx, oc.Namespace(), podName, 1)
			framework.ExpectNoError(err, "GPU is not accessible in a pod created after the kubelet restart")
		})
	})
})
