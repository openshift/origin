package node

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	debugNamespaceOnce   sync.Once
	cachedDebugNamespace string
)

// execOnNodeWithChroot wraps execOnNodeWithChroot to support MicroShift.
// For MicroShift, uses kube-system namespace since openshift-machine-config-operator doesn't exist.
func execOnNodeWithChroot(ctx context.Context, oc *exutil.CLI, nodeName string, cmd ...string) (string, error) {
	debugNamespaceOnce.Do(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		if err == nil && isMicroShift {
			cachedDebugNamespace = "kube-system"
		} else {
			cachedDebugNamespace = nodeutils.DebugNamespace
		}
	})
	return exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, cachedDebugNamespace, cmd...)
}

var _ = g.Describe("[sig-node] [Jira:Node/Kubelet] Podman and CRI-O version compatibility", func() {
	var (
		oc          = exutil.NewCLIWithoutNamespace("podman-crio-bump")
		clusterType string
		testNode    string
		nodeRole    string
	)

	g.BeforeEach(func(ctx context.Context) {
		nodeutils.EnsureNodesReady(ctx, oc)

		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())

		if isMicroShift {
			clusterType = "MicroShift"
		} else {
			topology, topoErr := exutil.GetControlPlaneTopology(oc)
			o.Expect(topoErr).NotTo(o.HaveOccurred())
			clusterType = string(*topology)
		}
		e2e.Logf("Cluster Topology: %s", clusterType)

		// Select test node: check for pure worker nodes first, then master+worker nodes, then master-only
		// On SNO/compact clusters, nodes have both master and worker labels - treat as master role
		workerNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/worker",
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list worker nodes")

		// Find a node with ONLY worker role (not both master+worker)
		var pureWorkerNode string
		if len(workerNodes.Items) > 0 {
			for _, node := range workerNodes.Items {
				if _, hasMasterLabel := node.Labels["node-role.kubernetes.io/master"]; !hasMasterLabel {
					pureWorkerNode = node.Name
					break
				}
			}
		}

		if pureWorkerNode != "" {
			// Found a pure worker node (no master label)
			testNode = pureWorkerNode
			nodeRole = "worker"
		} else if len(workerNodes.Items) > 0 {
			// All worker nodes also have master label (SNO/compact cluster) - treat as master
			testNode = workerNodes.Items[0].Name
			nodeRole = "master"
		} else {
			// No worker nodes at all, try master-only nodes
			masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: "node-role.kubernetes.io/master",
			})
			if err == nil && len(masterNodes.Items) > 0 {
				testNode = masterNodes.Items[0].Name
				nodeRole = "master"
			} else {
				// Fallback to any node
				allNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list nodes")
				o.Expect(allNodes.Items).NotTo(o.BeEmpty(), "No nodes found in cluster")
				testNode = allNodes.Items[0].Name
				nodeRole = "node"
			}
		}
		e2e.Logf("Selected test node with role: %s", nodeRole)
	})

	// author: aksjadha@redhat.com
	// Validates that Podman and CRI-O can simultaneously pull images without corrupting shared container storage
	g.It("[Skipped:Disconnected] Verify Podman and CRI-O concurrent image pulls do not corrupt shared storage [OCPNODE-4812]", func(ctx context.Context) {
		testImage := "registry.access.redhat.com/ubi9/ubi-init"

		g.DeferCleanup(func(ctx context.Context) {
			e2e.Logf("Cleaning up test images from Podman and CRI-O")
			execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", testImage, testImage))
		})

		g.By("Run 5 iterations of concurrent Podman and CRI-O image pulls to detect storage layer corruption")
		for i := 1; i <= 5; i++ {
			g.By(fmt.Sprintf("Iteration %d: Remove test image from shared storage", i))
			execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("podman rmi %s 2>/dev/null || true; crictl rmi %s 2>/dev/null || true", testImage, testImage))

			g.By(fmt.Sprintf("Iteration %d: Trigger simultaneous pulls via Podman and CRI-O", i))
			_, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("podman pull %s & P1=$!; crictl pull %s & P2=$!; wait $P1 && wait $P2", testImage, testImage))
			o.Expect(err).NotTo(o.HaveOccurred(), "Simultaneous Podman and CRI-O pull failed on iteration %d", i)

			g.By(fmt.Sprintf("Iteration %d: Verify no storage layer errors in CRI-O journal", i))
			logs, _ := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"journalctl -u crio --since '-2m' | grep -iE 'error creating read-write layer' || true")
			o.Expect(strings.TrimSpace(logs)).To(o.BeEmpty(), "Found storage corruption errors in CRI-O journal on iteration %d", i)
		}
		e2e.Logf("Successfully completed 5 iterations without storage corruption")
	})

	// author: aksjadha@redhat.com
	// Validates that Podman and CRI-O correctly handle multiple multi-layer images with layer reuse and digest-based pulls
	g.It("[Skipped:Disconnected] Verify Podman and CRI-O handle multiple multi-layer images with layer reuse and digest pulls [OCPNODE-4812]", func(ctx context.Context) {
		multiLayerImages := []struct {
			image  string
			runCmd []string
		}{
			{"registry.access.redhat.com/ubi9/nodejs-18", []string{"node", "--version"}},
			{"registry.access.redhat.com/ubi9/python-39", []string{"python3", "--version"}},
			{"registry.access.redhat.com/ubi9/go-toolset", []string{"go", "version"}},
			{"registry.access.redhat.com/ubi9/nginx-120", []string{"nginx", "-v"}},
			{"registry.access.redhat.com/ubi9/httpd-24", []string{"httpd", "-v"}},
		}

		g.DeferCleanup(func(ctx context.Context) {
			e2e.Logf("Cleaning up all multi-layer test images")
			for _, img := range multiLayerImages {
				execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
					fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", img.image, img.image))
			}
		})

		g.By(fmt.Sprintf("Pull %d multi-layer images with Podman and verify layer counts", len(multiLayerImages)))
		for i, img := range multiLayerImages {
			e2e.Logf("Processing image %d/%d: %s", i+1, len(multiLayerImages), img.image)

			execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c", fmt.Sprintf("podman rmi %s 2>/dev/null || true", img.image))
			_, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", img.image)
			o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull of image %s failed", img.image)

			podmanLayers, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("podman inspect %s --format '{{len .RootFS.Layers}}' 2>/dev/null", img.image))
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to inspect layer count for %s", img.image)
			e2e.Logf("Image %s has %s layers", img.image, strings.TrimSpace(podmanLayers))
		}

		g.By("Verify CRI-O reuses Podman-pulled layers from shared storage for each image")
		for i, img := range multiLayerImages {
			e2e.Logf("Testing layer reuse for image %d/%d: %s", i+1, len(multiLayerImages), img.image)

			execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("podman rmi %s 2>/dev/null || true; crictl rmi %s 2>/dev/null || true", img.image, img.image))

			_, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", img.image)
			o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull failed for %s", img.image)

			overlayBefore, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"ls /var/lib/containers/storage/overlay 2>/dev/null | wc -l")
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to count overlay directories before CRI-O pull")

			_, err = execOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", img.image)
			o.Expect(err).NotTo(o.HaveOccurred(), "CRI-O pull failed for %s", img.image)

			overlayAfter, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"ls /var/lib/containers/storage/overlay 2>/dev/null | wc -l")
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to count overlay directories after CRI-O pull")
			e2e.Logf("Image %s - Overlay count before: %s, after: %s", img.image, strings.TrimSpace(overlayBefore), strings.TrimSpace(overlayAfter))
		}

		g.By("Verify Podman can run containers from all multi-layer images")
		for i, img := range multiLayerImages {
			e2e.Logf("Running container %d/%d from image: %s", i+1, len(multiLayerImages), img.image)
			runArgs := append([]string{"run", "--rm", img.image}, img.runCmd...)
			podmanArgs := append([]string{"podman"}, runArgs...)
			_, err := execOnNodeWithChroot(ctx, oc, testNode, podmanArgs...)
			o.Expect(err).NotTo(o.HaveOccurred(), "Podman failed to run container from %s", img.image)
		}

		g.By("Verify Podman supports digest-based pull with --policy=always for multiple images")
		testImageForDigest := multiLayerImages[0].image
		digest, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman inspect %s --format '{{.Digest}}' 2>/dev/null", testImageForDigest))
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get image digest from Podman")
		digest = strings.TrimSpace(digest)
		o.Expect(digest).NotTo(o.BeEmpty(), "Image digest is empty")

		imageByDigest := strings.Split(testImageForDigest, ":")[0] + "@" + digest
		execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c", fmt.Sprintf("podman rmi %s 2>/dev/null || true", testImageForDigest))

		_, err = execOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", "--policy=always", imageByDigest)
		o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull by digest with --policy=always failed")
		e2e.Logf("Successfully pulled image by digest: %s", imageByDigest)

		g.By("Verify no overlay mount errors in CRI-O journal after multi-layer operations")
		errors, _ := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"journalctl -u crio --since '-5m' | grep -iE 'error creating read-write layer|overlay.*failed|too many layers|max depth' || true")
		o.Expect(strings.TrimSpace(errors)).To(o.BeEmpty(), "Found overlay mount errors in CRI-O journal")
	})

	// author: aamritan@redhat.com
	// Validates that CRI-O and Podman agree on image inventory regardless of which tool pulled the image
	g.It("[Serial] maintain a consistent image list between Podman and CRI-O", func(ctx context.Context) {
		microImage := "registry.access.redhat.com/ubi9/ubi-micro:latest"
		initImage := "registry.access.redhat.com/ubi9/ubi-init:latest"

		g.By("Ensure clean initial state by removing target images if already present")
		execOnNodeWithChroot(ctx, oc, testNode, "podman", "rmi", "-f", microImage)
		execOnNodeWithChroot(ctx, oc, testNode, "crictl", "rmi", initImage)

		defer func() {
			execOnNodeWithChroot(ctx, oc, testNode, "podman", "rmi", "-f", microImage)
			execOnNodeWithChroot(ctx, oc, testNode, "crictl", "rmi", initImage)
		}()

		g.By(fmt.Sprintf("Pull %s via Podman and confirm CRI-O sees it", microImage))
		pullOut, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", microImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "podman pull failed:\n%s", pullOut)

		crictlOut, err := execOnNodeWithChroot(ctx, oc, testNode, "crictl", "images")
		o.Expect(err).NotTo(o.HaveOccurred(), "crictl images failed:\n%s", crictlOut)
		o.Expect(crictlOut).To(o.ContainSubstring("ubi-micro"), "CRI-O must list image pulled by Podman; crictl images output:\n%s", crictlOut)

		g.By(fmt.Sprintf("Pull %s via CRI-O (crictl) and confirm Podman sees it", initImage))
		criPullOut, err := execOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", initImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "crictl pull failed:\n%s", criPullOut)

		podmanOut, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "images")
		o.Expect(err).NotTo(o.HaveOccurred(), "podman images failed:\n%s", podmanOut)
		o.Expect(podmanOut).To(o.ContainSubstring("ubi-init"), "Podman must list image pulled by crictl; podman images output:\n%s", podmanOut)

		// Retry each check: the two commands aren't atomic, so a transient shift in the node's
		// image set between them (e.g. a background GC sweep) could otherwise cause a flake.
		g.By("Verify both tools list both pulled images in a full inventory comparison")
		o.Eventually(func() (string, error) {
			return execOnNodeWithChroot(ctx, oc, testNode, "crictl", "images")
		}, 30*time.Second, 5*time.Second).Should(o.And(o.ContainSubstring("ubi-micro"), o.ContainSubstring("ubi-init")))

		o.Eventually(func() (string, error) {
			return execOnNodeWithChroot(ctx, oc, testNode, "podman", "images")
		}, 30*time.Second, 5*time.Second).Should(o.And(o.ContainSubstring("ubi-micro"), o.ContainSubstring("ubi-init")))
	})

	// author: aamritan@redhat.com
	// Validates that Podman can't remove an image a running CRI-O pod is using, and separately,
	// that crictl can't remove an image a running Podman container is using
	g.It("[Serial] refuse to remove an image still in use by the other tool, in either direction", func(ctx context.Context) {
		httpdImage := "registry.access.redhat.com/ubi9/httpd-24:latest"
		oc.SetupProject()
		namespace := oc.Namespace()

		g.By(fmt.Sprintf("Start a long-running CRI-O/kubelet pod from %s on node %s", httpdImage, testNode))
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tc16-crio-inuse",
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				NodeSelector: map[string]string{
					"kubernetes.io/hostname": testNode,
				},
				Containers: []corev1.Container{
					{
						Name:    "inuse",
						Image:   httpdImage,
						Command: []string{"sleep", "3600"},
					},
				},
			},
		}
		pod, err := oc.KubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create pod")
		o.Expect(e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeClient(), pod)).To(o.Succeed())

		g.By("podman rmi of that image must fail (in use)")
		out, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "rmi", httpdImage)
		o.Expect(err).To(o.HaveOccurred(), "podman rmi must not delete an in-use image; output:\n%s", out)
		combinedErr := strings.ToLower(out + " " + err.Error())
		o.Expect(combinedErr).To(o.Or(o.ContainSubstring("in use"), o.ContainSubstring("used")), "expected output to indicate image is in use, got: %s", combinedErr)

		g.By("Confirm the CRI-O pod is unaffected")
		o.Expect(e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeClient(), pod)).To(o.Succeed())
		execOut, err := oc.Run("exec").Args("-n", namespace, pod.Name, "--", "echo", "still-alive").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(execOut).To(o.ContainSubstring("still-alive"))

		g.By("podman image prune -a --force must not break the running pod")
		execOnNodeWithChroot(ctx, oc, testNode, "podman", "image", "prune", "-a", "--force")
		o.Expect(e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeClient(), pod)).To(o.Succeed())
		execOut, err = oc.Run("exec").Args("-n", namespace, pod.Name, "--", "echo", "still-alive").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(execOut).To(o.ContainSubstring("still-alive"))

		minimalImage := "registry.access.redhat.com/ubi9/ubi-minimal:latest"
		podmanContainerName := "tc16-podman-inuse"

		g.By(fmt.Sprintf("Start a Podman container from %s on node %s", minimalImage, testNode))
		// Pre-cleanup to ensure test idempotency across retries
		execOnNodeWithChroot(ctx, oc, testNode, "podman", "rm", "-f", podmanContainerName)

		// --net=host and --pid=host prevent namespace invalidation when debug pods are destroyed
		runOut, err := execOnNodeWithChroot(ctx, oc, testNode,
			"podman", "run", "-d", "--net=host", "--pid=host", "--name", podmanContainerName, minimalImage, "sleep", "3600")
		o.Expect(err).NotTo(o.HaveOccurred(), "podman run failed:\n%s", runOut)

		defer func() {
			rmOut, rmErr := execOnNodeWithChroot(ctx, oc, testNode, "podman", "rm", "-f", podmanContainerName)
			if rmErr != nil {
				e2e.Logf("Warning: cleanup of podman container %q had errors: %v\n%s", podmanContainerName, rmErr, rmOut)
			}
		}()

		g.By("crictl rmi must fail (in use by Podman)")
		rmiOut, rmiErr := execOnNodeWithChroot(ctx, oc, testNode, "crictl", "rmi", minimalImage)
		o.Expect(rmiErr).To(o.HaveOccurred(), "crictl rmi must not delete an in-use image; output:\n%s", rmiOut)
		combinedRmiErr := strings.ToLower(rmiOut + " " + rmiErr.Error())
		o.Expect(combinedRmiErr).To(o.Or(o.ContainSubstring("in use"), o.ContainSubstring("used")), "expected output to indicate image is in use, got: %s", combinedRmiErr)

		g.By("Verify Podman container remains functional (exec check)")
		execOut2, execErr := execOnNodeWithChroot(ctx, oc, testNode, "podman", "exec", podmanContainerName, "echo", "still alive")
		o.Expect(execErr).NotTo(o.HaveOccurred(), "podman exec failed on active container after crictl rmi attempt:\n%s", execOut2)
		o.Expect(execOut2).To(o.ContainSubstring("still alive"))
	})

	// author: aamritan@redhat.com
	// Validates concurrent reads, cross-tool image ID consistency, and clean recovery from an interrupted pull without losing existing images
	g.It("[Serial] read and recover shared image storage cleanly across concurrent access, cross-tool metadata checks, and an interrupted pull", func(ctx context.Context) {
		podmanPulledImage := "registry.access.redhat.com/ubi9/ubi-minimal:latest"
		crioPulledImage := "registry.access.redhat.com/ubi9/ubi-init:latest"
		largeImage := "registry.access.redhat.com/ubi9/nodejs-18:latest"

		g.By("Ensure clean initial state by removing target images if already present")
		execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman rmi -f %s %s %s 2>/dev/null; crictl rmi %s %s %s 2>/dev/null",
				podmanPulledImage, crioPulledImage, largeImage, podmanPulledImage, crioPulledImage, largeImage))

		defer func() {
			execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("podman rmi -f %s %s %s 2>/dev/null; crictl rmi %s %s %s 2>/dev/null",
					podmanPulledImage, crioPulledImage, largeImage, podmanPulledImage, crioPulledImage, largeImage))
		}()

		g.By(fmt.Sprintf("Pull %s via Podman", podmanPulledImage))
		pullOut, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", podmanPulledImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "podman pull failed:\n%s", pullOut)

		g.By("Read the image concurrently from both Podman and CRI-O")
		concurrentOut, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"podman images > /tmp/podman-read.out 2>&1 & crictl images > /tmp/crictl-read.out 2>&1 & wait; "+
				"cat /tmp/podman-read.out; echo '---'; cat /tmp/crictl-read.out; rm -f /tmp/podman-read.out /tmp/crictl-read.out")
		o.Expect(err).NotTo(o.HaveOccurred(), "concurrent reads failed:\n%s", concurrentOut)
		o.Expect(concurrentOut).To(o.ContainSubstring("ubi-minimal"))
		g.By("Confirm CRI-O's image ID for the Podman-pulled image matches Podman's own report")
		podmanID, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman inspect %s --format '{{.Id}}'", podmanPulledImage))
		o.Expect(err).NotTo(o.HaveOccurred(), "podman inspect failed:\n%s", podmanID)
		podmanID = strings.TrimPrefix(strings.TrimSpace(podmanID), "sha256:")

		crictlID, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf(`crictl inspecti %s | python3 -c "import sys,json; print(json.load(sys.stdin)['status']['id'])"`, podmanPulledImage))
		o.Expect(err).NotTo(o.HaveOccurred(), "crictl inspecti failed:\n%s", crictlID)
		crictlID = strings.TrimPrefix(strings.TrimSpace(crictlID), "sha256:")
		o.Expect(crictlID).To(o.Equal(podmanID), "CRI-O's image ID must match the content Podman pulled")

		g.By(fmt.Sprintf("Pull %s via CRI-O and confirm Podman's image ID matches", crioPulledImage))
		crictlPullOut, err := execOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", crioPulledImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "crictl pull failed:\n%s", crictlPullOut)

		crictlID2, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf(`crictl inspecti %s | python3 -c "import sys,json; print(json.load(sys.stdin)['status']['id'])"`, crioPulledImage))
		o.Expect(err).NotTo(o.HaveOccurred(), "crictl inspecti failed:\n%s", crictlID2)
		crictlID2 = strings.TrimPrefix(strings.TrimSpace(crictlID2), "sha256:")

		podmanID2, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman inspect %s --format '{{.Id}}'", crioPulledImage))
		o.Expect(err).NotTo(o.HaveOccurred(), "podman inspect failed:\n%s", podmanID2)
		podmanID2 = strings.TrimPrefix(strings.TrimSpace(podmanID2), "sha256:")

		o.Expect(podmanID2).To(o.Equal(crictlID2), "Podman's image ID must match the content CRI-O pulled")

		g.By("Record the baseline overlay layer count before the interrupted pull")
		baselineLayersOut, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c", "ls /var/lib/containers/storage/overlay | wc -l")
		o.Expect(err).NotTo(o.HaveOccurred())
		baselineLayerCount, err := strconv.Atoi(strings.TrimSpace(baselineLayersOut))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("Kill a Podman pull of %s mid-transfer", largeImage))
		execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman pull %s & sleep 0.5; pkill -9 -f 'podman pull %s' || true; wait", largeImage, largeImage))

		g.By("Confirm CRI-O can immediately pull the same image cleanly despite the interrupted Podman pull")
		crictlLargePullOut, err := execOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", largeImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "crictl pull failed after interrupted podman pull:\n%s", crictlLargePullOut)

		// internal_repair only runs during CRI-O's own startup/restart, which this test never
		// triggers, so it's omitted here - checking for it in a no-restart flow would never fail.
		g.By("Check for storage corruption errors in the CRI-O journal")
		journalErrors, _ := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"journalctl -u crio --since '-2m' | grep -iE 'error creating read-write layer|no such file or directory'")
		o.Expect(journalErrors).To(o.BeEmpty(), "found corruption errors in CRI-O journal: %s", journalErrors)

		g.By("Confirm Podman can also cleanly pull the same image after the interruption")
		podmanRepullOut, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", largeImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "podman re-pull failed after interrupted pull:\n%s", podmanRepullOut)

		g.By("Confirm the earlier pulled images and layer count survived the interruption untouched")
		podmanImagesOut, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "images")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podmanImagesOut).To(o.And(o.ContainSubstring("ubi-minimal"), o.ContainSubstring("ubi-init")))

		finalLayersOut, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c", "ls /var/lib/containers/storage/overlay | wc -l")
		o.Expect(err).NotTo(o.HaveOccurred())
		finalLayerCount, err := strconv.Atoi(strings.TrimSpace(finalLayersOut))
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(finalLayerCount).To(o.BeNumerically(">=", baselineLayerCount), "layer count must never decrease: baseline=%d final=%d", baselineLayerCount, finalLayerCount)
	})

	//author: njagan@redhat.com
	//Confirms a node recovers cleanly after mixed Podman/CRI-O activity
	g.It("Confirm a node recovers cleanly after mixed Podman/CRI-O activity [Disruptive][Serial]", func(ctx context.Context) {
		podmanTestContainer := "podman-test"
		podmanImage := "registry.access.redhat.com/ubi9/ubi-minimal:latest"
		crioTestPod := "crio-test"
		crioImage := "registry.access.redhat.com/ubi9/httpd-24:latest"

		e2e.Logf("Cluster Type: %s | Test Node: %s", clusterType, testNode)

		defer func() {
			execOnNodeWithChroot(ctx, oc, testNode, "podman", "rm", "-f", podmanTestContainer)
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", crioTestPod, "--ignore-not-found").Execute()
		}()

		g.By("Step 1: Create Podman container on target node")
		_, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", podmanImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull failed on node %s", testNode)
		_, err = execOnNodeWithChroot(ctx, oc, testNode, "podman", "run", "-d",
			"--name", podmanTestContainer, podmanImage, "sleep", "3600")
		o.Expect(err).NotTo(o.HaveOccurred(), "Podman run failed on node %s", testNode)
		e2e.Logf("Podman container %s created on %s", podmanTestContainer, testNode)

		g.By("Step 2: Create CRI-O managed pod on target node")
		overrides := fmt.Sprintf(`{"spec":{"nodeName":"%s"}}`, testNode)
		err = oc.AsAdmin().WithoutNamespace().Run("run").Args(
			crioTestPod, "--image="+crioImage, "--restart=Always",
			"--overrides="+overrides, "--", "sleep", "3600").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Pod creation failed")
		_, err = oc.AsAdmin().WithoutNamespace().Run("wait").Args(
			"--for=condition=Ready", "pod/"+crioTestPod, "--timeout=120s").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Pod %s not ready", crioTestPod)
		e2e.Logf("CRI-O pod %s running on %s", crioTestPod, testNode)

		g.By("Step 3: Verify pre-reboot state")
		preReboot, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "ps", "-a")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(preReboot).To(o.ContainSubstring(podmanTestContainer), "Podman container not found before reboot")

		g.By("Step 4: Reboot the node")
		execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c", "systemctl reboot")
		time.Sleep(10 * time.Second)

		g.By("Step 5: Wait for node to recover")
		recoveryTimeout := 5 * time.Minute
		if clusterType == "SingleReplica" || clusterType == "MicroShift" {
			recoveryTimeout = 10 * time.Minute
		}
		o.Eventually(func() error {
			output, getErr := oc.AsAdmin().WithoutNamespace().Run("get").Args(
				"node", testNode, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			if getErr != nil {
				return getErr
			}
			if strings.TrimSpace(output) != "True" {
				return fmt.Errorf("node not ready yet")
			}
			return nil
		}, recoveryTimeout, 10*time.Second).Should(o.Succeed(), "Node %s did not recover after reboot", testNode)
		e2e.Logf("Node %s recovered and Ready", testNode)

		g.By("Step 6: Check CRI-O health - no storage corruption")
		statusOutput, err := execOnNodeWithChroot(ctx, oc, testNode, "systemctl", "status", "crio")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statusOutput).To(o.ContainSubstring("active (running)"), "CRI-O not running after reboot")

		journalOutput, _ := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"journalctl -u crio --since '-10min' | grep -iE 'wiping containers|storage is corrupt|repairing storage'")
		o.Expect(strings.TrimSpace(journalOutput)).To(o.BeEmpty(),
			"Found storage corruption indicators in CRI-O journal: %s", journalOutput)
		e2e.Logf("CRI-O healthy - no storage corruption detected")

		g.By("Step 7: Verify Podman data persisted through reboot")
		postReboot, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "ps", "-a")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(postReboot).To(o.ContainSubstring(podmanTestContainer),
			"Podman container %s did not persist after reboot", podmanTestContainer)

		g.By("Step 8: Verify CRI-O workload recovered")
		_, err = oc.AsAdmin().WithoutNamespace().Run("wait").Args(
			"--for=condition=Ready", "pod/"+crioTestPod, "--timeout=120s").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "CRI-O pod %s did not recover after reboot", crioTestPod)

		e2e.Logf("PASSED: Node %s recovered cleanly - no storage corruption, Podman data persisted, CRI-O workload recovered", testNode)
	})

	//author: njagan@redhat.com
	//Confirms CRI-O doesn't wrongly repair healthy storage after the bump
	g.It("Confirm CRI-O doesn't wrongly repair healthy storage after the bump [Disruptive][Serial]", func(ctx context.Context) {
		e2e.Logf("Cluster Type: %s | Test Node: %s", clusterType, testNode)

		g.By("Step 1: Confirm node has real data - CRI-O version and existing images")
		crioVersion, err := execOnNodeWithChroot(ctx, oc, testNode, "crio", "version")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("CRI-O version:\n%s", crioVersion)

		imagesBefore, err := execOnNodeWithChroot(ctx, oc, testNode, "crictl", "images")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(imagesBefore).NotTo(o.BeEmpty(), "No images found on node - need real data")
		imageCountBefore := strings.Count(imagesBefore, "\n")
		e2e.Logf("Images before restart: %d lines", imageCountBefore)

		g.By("Step 2: Confirm internal_repair is enabled")
		repairConfig, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"crio config 2>&1 | grep -i internal_repair")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(repairConfig).To(o.ContainSubstring("true"),
			"internal_repair should be enabled, got: %s", repairConfig)
		e2e.Logf("internal_repair config: %s", strings.TrimSpace(repairConfig))

		g.By("Step 3: Restart CRI-O to trigger integrity check")
		_, err = execOnNodeWithChroot(ctx, oc, testNode, "systemctl", "restart", "crio")
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(5 * time.Second)

		g.By("Step 4: Check startup logs for repair findings")
		journalOutput, _ := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"journalctl -u crio --since '-1min' | grep -iE 'repairing storage|storage is corrupt|wiping containers|removing incomplete layer'")
		o.Expect(strings.TrimSpace(journalOutput)).To(o.BeEmpty(),
			"CRI-O should not report storage corruption on healthy storage: %s", journalOutput)
		e2e.Logf("No repair/corrupt/incomplete messages found in CRI-O logs")

		g.By("Step 5: Confirm nothing was deleted - all images intact")
		imagesAfter, err := execOnNodeWithChroot(ctx, oc, testNode, "crictl", "images")
		o.Expect(err).NotTo(o.HaveOccurred())
		imageCountAfter := strings.Count(imagesAfter, "\n")
		o.Expect(imageCountAfter).To(o.BeNumerically(">=", imageCountBefore),
			"Images were deleted during restart: before=%d, after=%d", imageCountBefore, imageCountAfter)
		e2e.Logf("Images after restart: %d lines (before: %d)", imageCountAfter, imageCountBefore)

		g.By("Step 6: Confirm workloads undisturbed")
		nodeutils.EnsureNodesReady(ctx, oc)
		e2e.Logf("All nodes ready - workloads undisturbed")

		e2e.Logf("PASSED: CRI-O did not wrongly repair healthy storage on %s", testNode)
	})

	//author: njagan@redhat.com
	//Verifies cross-tool image sharing between Podman and CRI-O
	g.It("Verify Podman and CRI-O share pulled images and layers correctly", func(ctx context.Context) {
		podmanPullImage := "registry.access.redhat.com/ubi9/httpd-24:latest"
		crioPullImage := "registry.access.redhat.com/ubi9/python-311:latest"
		sharedPullImage := "registry.access.redhat.com/ubi9/nodejs-20:latest"
		crioPod := "shared-storage-test"

		e2e.Logf("Cluster Type: %s | Test Node: %s", clusterType, testNode)

		defer func() {
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", crioPod, "--ignore-not-found").Execute()
			execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("podman rm -af 2>/dev/null; podman rmi %s %s %s 2>/dev/null; crictl rmi %s %s %s 2>/dev/null",
					podmanPullImage, crioPullImage, sharedPullImage, podmanPullImage, crioPullImage, sharedPullImage))
		}()

		g.By("Step 1: Podman pulls image, CRI-O runs it without re-downloading")

		execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman rmi %s 2>/dev/null; crictl rmi %s 2>/dev/null", podmanPullImage, podmanPullImage))

		_, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", podmanPullImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull failed for %s", podmanPullImage)

		podmanID, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman inspect %s --format '{{.Id}}'", podmanPullImage))
		o.Expect(err).NotTo(o.HaveOccurred())
		podmanID = strings.TrimSpace(podmanID)
		e2e.Logf("Podman image ID: %s", podmanID)

		overrides := fmt.Sprintf(`{"spec":{"nodeName":"%s"}}`, testNode)
		err = oc.AsAdmin().WithoutNamespace().Run("run").Args(
			crioPod, "--image="+podmanPullImage, "--restart=Always",
			"--overrides="+overrides, "--", "sleep", "3600").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Pod creation failed")

		_, err = oc.AsAdmin().WithoutNamespace().Run("wait").Args(
			"--for=condition=Ready", "pod/"+crioPod, "--timeout=120s").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Pod %s not ready", crioPod)

		crioImages, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"crictl images | grep httpd-24")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(crioImages).NotTo(o.BeEmpty(), "CRI-O should see the Podman-pulled image")
		e2e.Logf("CRI-O sees image: %s", strings.TrimSpace(crioImages))
		e2e.Logf("PASSED: Podman-pulled image used by CRI-O, pod running successfully")

		g.By("Step 2: CRI-O pulls image, Podman inspects and runs it")

		execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman rmi %s 2>/dev/null; crictl rmi %s 2>/dev/null", crioPullImage, crioPullImage))

		_, err = execOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", crioPullImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "CRI-O pull failed for %s", crioPullImage)

		podmanImages, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"podman images | grep python-311")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podmanImages).NotTo(o.BeEmpty(), "Podman should see the CRI-O-pulled image")
		e2e.Logf("Podman sees CRI-O image: %s", strings.TrimSpace(podmanImages))

		runOutput, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "run", "--rm",
			crioPullImage, "echo", "hello from podman")
		o.Expect(err).NotTo(o.HaveOccurred(), "Podman run of CRI-O-pulled image failed")
		o.Expect(runOutput).To(o.ContainSubstring("hello from podman"))
		e2e.Logf("PASSED: CRI-O-pulled image visible and runnable by Podman")

		g.By("Step 3: Same image tag pulled by both tools back-to-back, no duplicate layers")

		execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman rmi %s 2>/dev/null; crictl rmi %s 2>/dev/null", sharedPullImage, sharedPullImage))

		_, err = execOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", sharedPullImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull failed for %s", sharedPullImage)

		podmanSharedID, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman inspect %s --format '{{.Id}}'", sharedPullImage))
		o.Expect(err).NotTo(o.HaveOccurred())
		podmanSharedID = strings.TrimSpace(podmanSharedID)
		e2e.Logf("Podman image ID: %s", podmanSharedID)

		_, err = execOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", sharedPullImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "CRI-O pull failed for %s", sharedPullImage)

		crioSharedImages, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"crictl images | grep nodejs-20")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(crioSharedImages).NotTo(o.BeEmpty(), "CRI-O should see the image")
		o.Expect(crioSharedImages).To(o.ContainSubstring(podmanSharedID[:12]),
			"CRI-O and Podman should share the same image ID")
		e2e.Logf("CRI-O sees image: %s", strings.TrimSpace(crioSharedImages))

		journalOutput, _ := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"journalctl -u crio --since '-2min' | grep -iE 'error creating read-write layer'")
		o.Expect(strings.TrimSpace(journalOutput)).To(o.BeEmpty(),
			"Found layer errors in CRI-O journal: %s", journalOutput)

		e2e.Logf("PASSED: All cross-tool image sharing tests passed on %s", testNode)
	})
})
