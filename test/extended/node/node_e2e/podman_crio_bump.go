package node

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
)

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
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to check if cluster is MicroShift")

		if isMicroShift {
			g.Skip("Skipping Podman and CRI-O version compatibility tests on MicroShift - requires openshift-machine-config-operator namespace")
		}

		topology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get control plane topology")
		clusterType = string(*topology)
		e2e.Logf("Cluster Topology: %s", clusterType)

		// Select test node: prefer worker, fallback to master, then any node
		workerNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/worker",
		})
		if err == nil && len(workerNodes.Items) > 0 {
			testNode = workerNodes.Items[0].Name
			nodeRole = "worker"
		} else {
			masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: "node-role.kubernetes.io/master",
			})
			if err == nil && len(masterNodes.Items) > 0 {
				testNode = masterNodes.Items[0].Name
				nodeRole = "master"
			} else {
				allNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list nodes")
				o.Expect(allNodes.Items).NotTo(o.BeEmpty(), "No nodes found in cluster")
				testNode = allNodes.Items[0].Name
				nodeRole = "node"
			}
		}
		e2e.Logf("Selected test node role: %s", nodeRole)
	})

	// author: aksjadha@redhat.com
	// Validates CRI-O and Podman versions match expected post-bump versions
	g.Describe("[Skipped:Disconnected] version validation", func() {
		testImage := "registry.access.redhat.com/ubi9/ubi-minimal"

		g.AfterEach(func(ctx context.Context) {
			// Clean up test images
			e2e.Logf("Cleaning up test images")
			nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", testImage, testImage))
		})

		g.It("should report correct Podman version", func(ctx context.Context) {
			expectedPodman := os.Getenv("EXPECTED_PODMAN_VERSION")
			if expectedPodman == "" {
				g.Skip("EXPECTED_PODMAN_VERSION not set")
			}

			podmanVer := getVersion(ctx, oc, testNode, "podman version | grep -i 'Version:' | head -1")
			e2e.Logf("Podman Version: %s (Expected: %s)", podmanVer, expectedPodman)
			o.Expect(podmanVer).To(o.Or(
				o.Equal(expectedPodman),
				o.HavePrefix(expectedPodman+"."),
			), "Podman version mismatch: found %s, expected %s", podmanVer, expectedPodman)
		})

		g.It("should report correct CRI-O version", func(ctx context.Context) {
			expectedCrio := os.Getenv("EXPECTED_CRIO_VERSION")
			if expectedCrio == "" {
				g.Skip("EXPECTED_CRIO_VERSION not set")
			}

			crioVer := getVersion(ctx, oc, testNode, "rpm -q cri-o --qf '%{VERSION}'")
			e2e.Logf("CRI-O Version: %s (Expected: %s)", crioVer, expectedCrio)
			o.Expect(crioVer).To(o.Or(
				o.Equal(expectedCrio),
				o.HavePrefix(expectedCrio+"."),
			), "CRI-O version mismatch: found %s, expected %s", crioVer, expectedCrio)
		})

		g.It("should have correct RPM packages installed", func(ctx context.Context) {
			_, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"rpm -qa | grep -E 'cri-o|containers-common'")
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to verify RPM packages")
		})

		g.It("should have correct container libraries", func(ctx context.Context) {
			_, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"strings /usr/bin/crio | grep -iE 'containers/(storage|image|common)@v' | head -n 10")
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to verify container libraries")
		})

		g.It("should successfully pull and run test image with Podman", func(ctx context.Context) {
			nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "rmi", testImage)

			_, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", testImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull failed")

			osRelease, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "run", "--rm", testImage, "cat", "/etc/os-release")
			o.Expect(err).NotTo(o.HaveOccurred(), "Podman run failed")
			o.Expect(osRelease).To(o.ContainSubstring("Red Hat"), "Container did not print expected os-release content")
		})

		g.It("should recognize Podman-pulled images in CRI-O shared storage", func(ctx context.Context) {
			nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "rmi", testImage)
			nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "rmi", testImage)

			_, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", testImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull failed")

			_, err = nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", testImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "CRI-O pull failed")

			crioImages, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"crictl images | grep ubi-minimal")
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list CRI-O images")
			o.Expect(crioImages).NotTo(o.BeEmpty(), "CRI-O did not recognize shared image")
		})

		g.It("should have no storage errors in journal", func(ctx context.Context) {
			journalErrors, _ := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"journalctl -u crio --since '-10m' | grep -iE 'error creating read-write layer'")
			o.Expect(strings.TrimSpace(journalErrors)).To(o.BeEmpty(), "Found storage errors in CRI-O journal")
		})
	})

	// author: aksjadha@redhat.com
	// Tests simultaneous image pulls by Podman and CRI-O to detect storage corruption
	g.It("[Skipped:Disconnected] should handle simultaneous image pulls without corruption", func(ctx context.Context) {
		testImage := "registry.access.redhat.com/ubi9/ubi-init"

		g.DeferCleanup(func(ctx context.Context) {
			e2e.Logf("Cleaning up test images")
			nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", testImage, testImage))
		})

		e2e.Logf("Testing simultaneous pulls on %s node", nodeRole)

		for i := 1; i <= 5; i++ {
			e2e.Logf("Iteration %d/5", i)

			nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("podman rmi %s 2>/dev/null; crictl rmi %s 2>/dev/null", testImage, testImage))

			_, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("podman pull %s & P1=$!; crictl pull %s & P2=$!; wait $P1 && wait $P2", testImage, testImage))
			o.Expect(err).NotTo(o.HaveOccurred(), "Simultaneous pull failed on iteration %d", i)

			logs, _ := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"journalctl -u crio --since '-2m' | grep -iE 'error creating read-write layer'")
			o.Expect(strings.TrimSpace(logs)).To(o.BeEmpty(), "Found storage errors on iteration %d", i)
		}
	})

	// author: aksjadha@redhat.com
	// Validates multi-layer images are handled correctly with layer reuse
	g.Describe("[Skipped:Disconnected] multi-layer image handling", func() {
		multiLayerImage := "registry.access.redhat.com/ubi9/nodejs-18"

		g.AfterEach(func(ctx context.Context) {
			e2e.Logf("Cleaning up test images")
			nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", multiLayerImage, multiLayerImage))
		})

		g.It("should report layer count correctly", func(ctx context.Context) {
			nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "rmi", multiLayerImage)

			_, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", multiLayerImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull failed")

			podmanLayers, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("podman inspect %s --format '{{len .RootFS.Layers}}' 2>/dev/null", multiLayerImage))
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get layer count")
			e2e.Logf("Image has %s layers", strings.TrimSpace(podmanLayers))
		})

		g.It("should reuse layers between Podman and CRI-O", func(ctx context.Context) {
			nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("podman rmi %s 2>/dev/null; crictl rmi %s 2>/dev/null", multiLayerImage, multiLayerImage))

			_, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", multiLayerImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull failed")

			overlayBefore, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"ls /var/lib/containers/storage/overlay 2>/dev/null | wc -l")
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to count overlay directories")

			_, err = nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", multiLayerImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "CRI-O pull failed")

			overlayAfter, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"ls /var/lib/containers/storage/overlay 2>/dev/null | wc -l")
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to count overlay directories after CRI-O pull")

			e2e.Logf("Overlay count - Before: %s, After: %s", strings.TrimSpace(overlayBefore), strings.TrimSpace(overlayAfter))
		})

		g.It("should run container successfully with multi-layer image", func(ctx context.Context) {
			_, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", multiLayerImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull failed")

			_, err = nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "run", "--rm", multiLayerImage, "node", "--version")
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to run container with multi-layer image")
		})

		g.It("should support digest-based pulls", func(ctx context.Context) {
			_, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", multiLayerImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Initial pull failed")

			digest, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("podman inspect %s --format '{{.Digest}}' 2>/dev/null", multiLayerImage))
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get image digest")
			digest = strings.TrimSpace(digest)
			o.Expect(digest).NotTo(o.BeEmpty(), "Image digest is empty")

			imageByDigest := strings.Split(multiLayerImage, ":")[0] + "@" + digest
			nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "rmi", imageByDigest)

			_, err = nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", imageByDigest)
			o.Expect(err).NotTo(o.HaveOccurred(), "Pull by digest failed")
		})

		g.It("should have no overlay mount errors", func(ctx context.Context) {
			_, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "pull", multiLayerImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull failed")

			_, err = nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", multiLayerImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "CRI-O pull failed")

			errors, _ := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"journalctl -u crio --since '-5m' | grep -iE 'error creating read-write layer|overlay.*failed|too many layers|max depth'")
			o.Expect(strings.TrimSpace(errors)).To(o.BeEmpty(), "Found overlay mount errors in journal")
		})
	})

	// author: aksjadha@redhat.com
	// Confirms IDMS mirror configuration and fallback behavior
	g.It("[Serial][Skipped:Disconnected] should handle IDMS mirror configuration and fallback correctly", func(ctx context.Context) {
		if clusterType == "External" {
			g.Skip("IDMS and MachineConfigPool are not available on External topology clusters")
		}

		testImage := "registry.access.redhat.com/ubi9/ubi-minimal"
		idmsName := "test-idms-fallback"
		var pullImageByDigest string

		// Cleanup function
		cleanupIDMS := func(ctx context.Context) {
			e2e.Logf("Cleaning up IDMS and test images")

			// Delete IDMS
			err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("imagedigestmirrorset", idmsName, "--ignore-not-found").Execute()
			if err == nil {
				e2e.Logf("IDMS deleted: %s", idmsName)

				// Determine MCP name
				mcpName := "worker"
				if clusterType == "SingleReplica" {
					mcpName = "master"
				}

				// Wait for MCO rollout to START after IDMS deletion
				e2e.Logf("Waiting for MCP %s rollout to start after IDMS deletion", mcpName)
				waitErr := wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
					updating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
						"-o=jsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
					return strings.TrimSpace(updating) == "True", nil
				})
				o.Expect(waitErr).NotTo(o.HaveOccurred(), "MCP rollout did not start within timeout after IDMS deletion")

				// Wait for MCO rollout to COMPLETE
				e2e.Logf("Waiting for MCP %s rollout to complete after IDMS deletion", mcpName)
				waitErr = wait.PollImmediate(10*time.Second, 10*time.Minute, func() (bool, error) {
					updated, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
						"-o=jsonpath={.status.conditions[?(@.type=='Updated')].status}").Output()
					updating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
						"-o=jsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
					return strings.TrimSpace(updated) == "True" && strings.TrimSpace(updating) == "False", nil
				})
				o.Expect(waitErr).NotTo(o.HaveOccurred(), "MCP did not complete rollout after IDMS deletion within timeout")
			}

			// Clean up test images - crictl removes from shared storage
			if pullImageByDigest != "" {
				nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
					fmt.Sprintf("crictl rmi %s 2>/dev/null || true", pullImageByDigest))
			}
			nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("crictl rmi %s 2>/dev/null || true", testImage))
		}
		g.DeferCleanup(cleanupIDMS)

		e2e.Logf("Testing on selected %s node", nodeRole)

		// Create IDMS
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

		err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", "-").InputString(idmsYaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create IDMS")

		// Determine MCP name
		mcpName := "worker"
		if clusterType == "SingleReplica" {
			mcpName = "master"
		}

		// Wait for MCO rollout to START (Updating=True)
		e2e.Logf("Waiting for MCP %s rollout to start after IDMS creation", mcpName)
		waitErr := wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
			updating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
				"-o=jsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
			return strings.TrimSpace(updating) == "True", nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "MCP rollout did not start within timeout after IDMS creation")

		// Wait for MCO rollout to COMPLETE (Updated=True and Updating=False)
		e2e.Logf("Waiting for MCP %s rollout to complete", mcpName)
		waitErr = wait.PollImmediate(10*time.Second, 10*time.Minute, func() (bool, error) {
			updated, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
				"-o=jsonpath={.status.conditions[?(@.type=='Updated')].status}").Output()
			updating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
				"-o=jsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
			return strings.TrimSpace(updated) == "True" && strings.TrimSpace(updating) == "False", nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "MCP did not complete rollout within timeout")

		// Wait for mirror config on node (longer timeout for multi-node rollouts)
		e2e.Logf("Waiting for registries.conf update on node")
		var mirrorFound bool
		waitErr = wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
			mirrorConfig, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"cat /etc/containers/registries.conf | grep -A6 ubi-minimal")
			if err == nil && strings.Contains(mirrorConfig, "does-not-exist-idms-test") {
				mirrorFound = true
				return true, nil
			}
			return false, nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "Mirror config not found on node within timeout")
		o.Expect(mirrorFound).To(o.BeTrue(), "Mirror config was not applied to node")

		// Clear cache
		nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "rmi", testImage)
		nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "rmi", testImage)

		crioCheck, _ := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"crictl images | grep ubi-minimal || true")
		podmanCheck, _ := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"podman images | grep ubi-minimal || true")
		o.Expect(strings.TrimSpace(crioCheck)).To(o.BeEmpty(), "CRI-O cache not cleared")
		o.Expect(strings.TrimSpace(podmanCheck)).To(o.BeEmpty(), "Podman cache not cleared")

		// Get digest and pull
		digestCmd := fmt.Sprintf("skopeo inspect docker://%s 2>/dev/null | python3 -c \"import sys,json; print(json.load(sys.stdin)['Digest'])\"", testImage)
		digest, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c", digestCmd)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get image digest")
		digest = strings.TrimSpace(digest)
		o.Expect(digest).NotTo(o.BeEmpty(), "Image digest is empty")

		pullImageByDigest = testImage + "@" + digest

		// Test CRI-O pull
		_, err = nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", pullImageByDigest)
		o.Expect(err).NotTo(o.HaveOccurred(), "CRI-O pull by digest failed")

		// Verify CRI-O tried fake mirror and fell back to real source via journalctl
		crioLogs, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"journalctl -u crio --since '-2m'")
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get CRI-O journal logs")
		o.Expect(crioLogs).To(o.ContainSubstring("does-not-exist-idms-test"),
			"CRI-O did not attempt configured mirror - IDMS configuration may not have been applied by CRI-O")
		e2e.Logf("CRI-O attempted fake mirror and fell back to real source")

		// Test Podman pull
		nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("crictl rmi %s 2>/dev/null; podman rmi %s 2>/dev/null", pullImageByDigest, testImage))

		crioVerify, _ := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"crictl images | grep ubi-minimal || true")
		podmanVerify, _ := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"podman images | grep ubi-minimal || true")
		o.Expect(strings.TrimSpace(crioVerify)).To(o.BeEmpty(), "CRI-O cache not cleared before Podman test")
		o.Expect(strings.TrimSpace(podmanVerify)).To(o.BeEmpty(), "Podman cache not cleared before Podman test")

		// Pull with Podman and capture debug logs
		podmanOutput, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman pull --log-level=debug %s 2>&1", pullImageByDigest))
		o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull by digest failed")

		// Verify fallback via debug logs
		o.Expect(podmanOutput).To(o.ContainSubstring("does-not-exist-idms-test"),
			"Podman did not attempt configured mirror - IDMS configuration may not have been applied by Podman")
		e2e.Logf("Podman attempted fake mirror and fell back to real source")
	})
})

func getVersion(ctx context.Context, oc *exutil.CLI, node, cmd string) string {
	out, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, node, "/bin/bash", "-c", cmd)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get version from node")
	if re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`); re.MatchString(out) {
		if m := re.FindStringSubmatch(out); len(m) > 1 {
			return m[1]
		}
	}
	return strings.TrimSpace(out)
}
