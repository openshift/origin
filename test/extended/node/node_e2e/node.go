package node

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/yaml"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node] [Jira:Node/Kubelet] [NodeResource:numNodes=1,label=node_e2e] Kubelet, CRI-O, CPU manager", func() {
	var (
		oc             = exutil.NewCLIWithoutNamespace("node")
		nodeE2EBaseDir = exutil.FixturePath("testdata", "node", "node_e2e")
		podDevFuseYAML = filepath.Join(nodeE2EBaseDir, "pod-dev-fuse.yaml")
	)

	g.BeforeEach(func(ctx context.Context) {
		nodeutils.SkipOnMicroShift(oc)
		nodeutils.EnsureNodeResourceNodesReady(ctx, oc, "node_e2e")
	})

	//author: asahay@redhat.com
	g.It("[OTP] validate KUBELET_LOG_LEVEL", func(ctx context.Context) {
		var kubeservice string
		var kubelet string
		var err error

		node, err := nodeutils.GetNodeResource(ctx, oc, "node_e2e")
		o.Expect(err).NotTo(o.HaveOccurred(), "Error getting NodeResource node")

		g.By("Polling to check kubelet log level on node " + node)
		waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
			g.By("Checking KUBELET_LOG_LEVEL in kubelet.service on node " + node)
			kubeservice, err = nodeutils.ExecOnNodeWithChroot(ctx, oc, node, "/bin/bash", "-c", "systemctl show kubelet.service | grep KUBELET_LOG_LEVEL")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Checking kubelet process for --v=2 flag on node " + node)
			kubelet, err = nodeutils.ExecOnNodeWithChroot(ctx, oc, node, "/bin/bash", "-c", "ps aux | grep [k]ubelet")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying KUBELET_LOG_LEVEL is set and kubelet is running with --v=2")
			if strings.Contains(kubeservice, "KUBELET_LOG_LEVEL") && strings.Contains(kubelet, "--v=2") {
				e2e.Logf("KUBELET_LOG_LEVEL is 2.\n")
				return true, nil
			}
			e2e.Logf("KUBELET_LOG_LEVEL is not 2.\n")
			return false, nil
		})

		if waitErr != nil {
			e2e.Logf("Kubelet Log level is:\n %v\n", kubeservice)
			e2e.Logf("Running Process of kubelet are:\n %v\n", kubelet)
		}
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "KUBELET_LOG_LEVEL is not expected, timed out")
	})

	//author: cmaurya@redhat.com
	g.It("[OTP] validate cgroupv2 is default [OCP-80983]", func(ctx context.Context) {
		g.By("Check cgroup version on NodeResource worker nodes")
		resourceNodes, err := nodeutils.GetNodeResourceNodes(ctx, oc, "node_e2e")
		o.Expect(err).NotTo(o.HaveOccurred(), "Error getting NodeResource nodes")

		for _, resourceNode := range resourceNodes {
			worker := resourceNode.Name
			cgroupV, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, worker, "/bin/bash", "-c", "stat -c %T -f /sys/fs/cgroup")
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("cgroup version on node %s: [%v]", worker, cgroupV)
			o.Expect(cgroupV).To(o.ContainSubstring("cgroup2fs"), "Node %s does not have cgroupv2", worker)
		}

		g.By("Changing cgroup from v2 to v1 should result in error")
		output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("nodes.config.openshift.io", "cluster", "-p", `{"spec": {"cgroupMode": "v1"}}`, "--type=merge").Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("spec.cgroupMode: Unsupported value: \"v1\": supported values: \"v2\", \"\""))
	})

	//author: cmaurya@redhat.com
	g.It("[OTP] Allow dev fuse by default in CRI-O [OCP-70987]", func(ctx context.Context) {
		podName := "pod-devfuse"
		ns := "devfuse-test"

		// Skip on runc: io.kubernetes.cri-o.Devices annotation is only in crun's allowed_annotations.
		// We query crio config directly as ContainerRuntimeConfig API misses platform-default runc.
		g.By("Skip if the default runtime is runc")
		node, err := nodeutils.GetNodeResource(ctx, oc, "node_e2e")
		o.Expect(err).NotTo(o.HaveOccurred(), "Error getting NodeResource node")
		runtime, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, node, "/bin/bash", "-c",
			"crio status config 2>/dev/null | awk -F'\"' '/default_runtime/{print $2}'")
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.TrimSpace(runtime) == "runc" {
			g.Skip("Skipping: not applicable to runc runtime")
		}

		g.By("Create a test namespace")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("namespace", ns).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("namespace", ns, "--ignore-not-found").Execute()

		// Pin the pod to the node reserved for this test via NodeResource.
		// Without this, the default scheduler could place it on any node in
		// the cluster (including one reserved by another concurrently
		// running [NodeResource] test, or a node with a different runtime
		// than the one checked above), which would defeat the point of the
		// per-test node reservation.
		g.By("Create a pod with dev fuse annotation, pinned to the reserved node")
		podDevFuseBytes, err := os.ReadFile(podDevFuseYAML)
		o.Expect(err).NotTo(o.HaveOccurred())
		var devFusePod corev1.Pod
		o.Expect(yaml.Unmarshal(podDevFuseBytes, &devFusePod)).To(o.Succeed())
		devFusePod.Spec.NodeName = node
		pinnedPodYAML, err := yaml.Marshal(&devFusePod)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", "-", "-n", ns).InputString(string(pinnedPodYAML)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for pod to be ready")
		err = wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
			status, pollErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-n", ns, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			if pollErr != nil {
				e2e.Logf("Error polling pod status: %v", pollErr)
				return false, nil
			}
			return status == "True", nil
		})
		if err != nil {
			podStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-n", ns, "-o=jsonpath={.status}").Output()
			e2e.Logf("Pod status on timeout: %s", podStatus)
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "pod did not become ready")

		g.By("Check /dev/fuse is mounted inside the pod")
		output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(podName, "-n", ns, "--", "stat", "/dev/fuse").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("/dev/fuse mount output: %s", output)
		o.Expect(output).To(o.ContainSubstring("fuse"), "dev fuse is not mounted inside pod")
	})
})

// ImageTagMirrorSet and ImageDigestMirrorSet tests (OCP-57401, OCP-70203) live in image_mirror_set.go.
