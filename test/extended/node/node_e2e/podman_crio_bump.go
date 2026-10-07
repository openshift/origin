package node

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

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

		topology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get control plane topology")
		clusterType = string(*topology)
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
	// Validates sync=filesystem configuration and basic Podman/CRI-O shared storage operations
	g.It("[Disruptive][Skipped:Disconnected] Verify sync=filesystem configuration and basic storage operations [OCPNODE-4812]", func(ctx context.Context) {
		testImage := "registry.access.redhat.com/ubi9/ubi-minimal"
		syncConfigPath := "/etc/crio/crio.conf.d/99-sync-filesystem.conf"

		g.DeferCleanup(func(ctx context.Context) {
			e2e.Logf("Cleaning up test images")
			execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", testImage, testImage))
		})

		g.By("Step 1: Check if sync=filesystem is currently enabled in CRI-O configuration")
		crioConfig, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c", "crio config 2>/dev/null | grep -i 'sync' || true")
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get CRI-O config")
		if strings.Contains(crioConfig, "overlay.sync=filesystem") {
			e2e.Logf("sync=filesystem is already enabled (OCP 5.0+)")
		} else {
			e2e.Logf("sync=filesystem is not currently enabled")
		}

		g.By("Step 2: Create /etc/crio/crio.conf.d/99-sync-filesystem.conf with overlay.skip_mount_home and overlay.sync")
		syncConfigContent := `[crio]
storage_option = [
  "overlay.skip_mount_home=true",
  "overlay.sync=filesystem",
]
`
		_, err = execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("cat > %s << 'EOF'\n%sEOF", syncConfigPath, syncConfigContent))
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create sync=filesystem config")
		e2e.Logf("Created %s with overlay.skip_mount_home=true and overlay.sync=filesystem", syncConfigPath)

		g.By("Step 3: Schedule CRI-O service restart using systemd-run")
		// Use systemd-run to schedule restart after 3 seconds, allowing debug pod to exit cleanly
		_, err = execOnNodeWithChroot(ctx, oc, testNode, "systemd-run", "--on-active=3s", "systemctl", "restart", "crio")
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to schedule CRI-O restart")
		e2e.Logf("CRI-O service restart scheduled via systemd-run (executes in 3 seconds)")

		// Wait for scheduled restart to begin and CRI-O to start coming back up
		time.Sleep(5 * time.Second)

		g.By("Step 4: Wait for CRI-O service to become active after restart")
		var crioActive bool
		waitErr := wait.PollImmediate(2*time.Second, 30*time.Second, func() (bool, error) {
			status, err := execOnNodeWithChroot(ctx, oc, testNode, "systemctl", "is-active", "crio")
			if err == nil && strings.TrimSpace(status) == "active" {
				crioActive = true
				return true, nil
			}
			return false, nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "CRI-O did not become active within timeout")
		o.Expect(crioActive).To(o.BeTrue(), "CRI-O service not active")
		e2e.Logf("CRI-O service is active")

		g.By("Step 5: Verify overlay.skip_mount_home=true and overlay.sync=filesystem persisted after CRI-O restart")
		configAfter, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c", "crio config 2>/dev/null | grep -i 'storage_option' -A2")
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to read CRI-O config after restart")
		o.Expect(configAfter).To(o.ContainSubstring("overlay.sync=filesystem"), "overlay.sync=filesystem not found after restart")
		o.Expect(configAfter).To(o.ContainSubstring("overlay.skip_mount_home=true"), "overlay.skip_mount_home=true not found after restart")
		e2e.Logf("Verified overlay.skip_mount_home=true and overlay.sync=filesystem persisted after restart")

		g.By("Step 6: Pull test image with Podman")
		execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c", fmt.Sprintf("podman rmi %s 2>/dev/null || true", testImage))
		_, err = execOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", testImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull failed")
		e2e.Logf("Podman successfully pulled %s", testImage)

		g.By("Step 7: Run container with Podman to verify image functionality")
		osRelease, err := execOnNodeWithChroot(ctx, oc, testNode, "podman", "run", "--rm", testImage, "cat", "/etc/os-release")
		o.Expect(err).NotTo(o.HaveOccurred(), "Podman run failed")
		o.Expect(osRelease).To(o.ContainSubstring("Red Hat"), "Container os-release does not contain expected content")
		e2e.Logf("Podman successfully ran container from image")

		g.By("Step 8: Verify CRI-O recognizes Podman-pulled image in shared storage")
		crioImages, err := execOnNodeWithChroot(ctx, oc, testNode, "crictl", "images")
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list CRI-O images")
		o.Expect(crioImages).To(o.ContainSubstring("ubi-minimal"), "CRI-O did not recognize Podman-pulled image in shared storage")
		e2e.Logf("CRI-O recognized Podman-pulled image in shared storage")

		g.By("Step 9: Pull same image with CRI-O to verify storage sharing without conflicts")
		_, err = execOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", testImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "CRI-O pull failed")
		e2e.Logf("CRI-O successfully pulled image from shared storage")

		g.By("Step 10: Verify no storage errors in CRI-O journal")
		journalErrors, _ := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"journalctl -u crio --since '-10m' | grep -iE 'error creating read-write layer' || true")
		o.Expect(strings.TrimSpace(journalErrors)).To(o.BeEmpty(), "Found storage errors in CRI-O journal")
		e2e.Logf("No storage errors found in CRI-O journal")

		e2e.Logf("Validation completed: sync=filesystem enabled, storage sharing works without conflicts")
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

	// author: aksjadha@redhat.com
	// Validates that IDMS mirror configuration is applied to Podman and CRI-O and fallback to real source works
	g.It("[Serial][Disruptive][Skipped:Disconnected] Verify IDMS mirror configuration is applied to Podman and CRI-O with fallback behavior [OCPNODE-4812]", func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to check if cluster is MicroShift")

		if isMicroShift || clusterType == "External" {
			g.Skip("IDMS and MachineConfigPool are not available on MicroShift or External topology clusters")
		}

		testImage := "registry.access.redhat.com/ubi9/ubi-minimal"
		idmsName := "test-idms-fallback"
		var pullImageByDigest string

		cleanupIDMS := func(ctx context.Context) {
			g.By("Cleanup: Delete IDMS and wait for MCP rollout")
			err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("imagedigestmirrorset", idmsName, "--ignore-not-found").Execute()
			if err == nil {
				e2e.Logf("IDMS %s deleted", idmsName)

				mcpName := "worker"
				if nodeRole == "master" {
					mcpName = "master"
				}

				e2e.Logf("Waiting for MCP %s rollout to start after IDMS deletion", mcpName)
				waitErr := wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
					updating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
						"-o=jsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
					return strings.TrimSpace(updating) == "True", nil
				})
				o.Expect(waitErr).NotTo(o.HaveOccurred(), "MCP rollout did not start after IDMS deletion")

				e2e.Logf("Waiting for MCP %s rollout to complete after IDMS deletion", mcpName)
				waitErr = wait.PollImmediate(10*time.Second, 10*time.Minute, func() (bool, error) {
					updated, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
						"-o=jsonpath={.status.conditions[?(@.type=='Updated')].status}").Output()
					updating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
						"-o=jsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
					return strings.TrimSpace(updated) == "True" && strings.TrimSpace(updating) == "False", nil
				})
				o.Expect(waitErr).NotTo(o.HaveOccurred(), "MCP rollout did not complete after IDMS deletion")
			}

			if pullImageByDigest != "" {
				execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
					fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", pullImageByDigest, pullImageByDigest))
			}
			execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", testImage, testImage))
		}
		g.DeferCleanup(cleanupIDMS)

		g.By("Step 1: Create ImageDigestMirrorSet with fake mirror and real source as fallback")
		idmsYaml := fmt.Sprintf(`apiVersion: config.openshift.io/v1
kind: ImageDigestMirrorSet
metadata:
  name: %s
spec:
  imageDigestMirrors:
  - source: registry.access.redhat.com/ubi9/ubi-minimal
    mirrorSourcePolicy: AllowContactingSource
    mirrors:
    - quay.io/does-not-exist-idms-test/ubi9-minimal
    - registry.access.redhat.com/ubi9/ubi-minimal
`, idmsName)

		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", "-").InputString(idmsYaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create IDMS")
		e2e.Logf("Created IDMS %s with fake mirror quay.io/does-not-exist-idms-test", idmsName)

		mcpName := "worker"
		if clusterType == "SingleReplica" {
			mcpName = "master"
		}

		g.By(fmt.Sprintf("Step 2: Wait for MCP %s rollout to start after IDMS creation", mcpName))
		waitErr := wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
			updating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
				"-o=jsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
			return strings.TrimSpace(updating) == "True", nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "MCP rollout did not start after IDMS creation")

		g.By(fmt.Sprintf("Step 3: Wait for MCP %s rollout to complete", mcpName))
		waitErr = wait.PollImmediate(10*time.Second, 10*time.Minute, func() (bool, error) {
			updated, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
				"-o=jsonpath={.status.conditions[?(@.type=='Updated')].status}").Output()
			updating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
				"-o=jsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
			return strings.TrimSpace(updated) == "True" && strings.TrimSpace(updating) == "False", nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "MCP rollout did not complete")
		e2e.Logf("MCP %s rollout completed", mcpName)

		g.By("Step 4: Verify IDMS mirror configuration is present in node registries.conf")
		var mirrorFound bool
		waitErr = wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
			mirrorConfig, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"cat /etc/containers/registries.conf | grep -A6 ubi-minimal || true")
			if err == nil && strings.Contains(mirrorConfig, "does-not-exist-idms-test") {
				mirrorFound = true
				return true, nil
			}
			return false, nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "IDMS mirror config not found in node registries.conf")
		o.Expect(mirrorFound).To(o.BeTrue(), "IDMS mirror was not applied to registries.conf on node")
		e2e.Logf("Verified IDMS mirror configuration in registries.conf")

		g.By("Step 5: Clear image cache from Podman and CRI-O shared storage")
		execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", testImage, testImage))

		crioCheck, _ := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"crictl images | grep ubi-minimal || true")
		podmanCheck, _ := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"podman images | grep ubi-minimal || true")
		o.Expect(strings.TrimSpace(crioCheck)).To(o.BeEmpty(), "CRI-O cache not cleared")
		o.Expect(strings.TrimSpace(podmanCheck)).To(o.BeEmpty(), "Podman cache not cleared")

		g.By("Step 6: Get image digest for digest-based pull")
		digestCmd := fmt.Sprintf("skopeo inspect docker://%s 2>/dev/null | python3 -c \"import sys,json; print(json.load(sys.stdin)['Digest'])\"", testImage)
		digest, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c", digestCmd)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get image digest via skopeo")
		digest = strings.TrimSpace(digest)
		o.Expect(digest).NotTo(o.BeEmpty(), "Image digest is empty")

		pullImageByDigest = testImage + "@" + digest
		e2e.Logf("Image digest: %s", digest)

		g.By("Step 7: Pull image by digest via CRI-O and verify fake mirror was attempted")
		_, err = execOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", pullImageByDigest)
		o.Expect(err).NotTo(o.HaveOccurred(), "CRI-O pull by digest failed")

		crioLogs, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"journalctl -u crio --since '-2m'")
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get CRI-O journal logs")
		o.Expect(crioLogs).To(o.ContainSubstring("does-not-exist-idms-test"),
			"CRI-O did not attempt fake mirror - IDMS configuration was not applied by CRI-O")
		e2e.Logf("Verified CRI-O attempted fake mirror and fell back to real source")

		g.By("Step 8: Clear cache again and pull image by digest via Podman with debug logging")
		execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", pullImageByDigest, pullImageByDigest))

		crioVerify, _ := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"crictl images | grep ubi-minimal || true")
		podmanVerify, _ := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"podman images | grep ubi-minimal || true")
		o.Expect(strings.TrimSpace(crioVerify)).To(o.BeEmpty(), "CRI-O cache not cleared before Podman test")
		o.Expect(strings.TrimSpace(podmanVerify)).To(o.BeEmpty(), "Podman cache not cleared before Podman test")

		g.By("Step 9: Pull image by digest via Podman and verify fake mirror was attempted")
		podmanOutput, err := execOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman pull --log-level=debug %s 2>&1", pullImageByDigest))
		o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull by digest failed")

		o.Expect(podmanOutput).To(o.ContainSubstring("does-not-exist-idms-test"),
			"Podman did not attempt fake mirror - IDMS configuration was not applied by Podman")
		e2e.Logf("Verified Podman attempted fake mirror and fell back to real source")
	})
})
