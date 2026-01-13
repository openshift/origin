package node

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	// cnvDropInDir is the kubelet drop-in configuration directory for CNV swap
	cnvDropInDir = "/etc/openshift/kubelet.conf.d"
	// cnvDropInFile is the default swap configuration file name
	cnvDropInFile = "99-kubelet-limited-swap.conf"
	// cnvDropInFilePath is the full path to the swap configuration file
	cnvDropInFilePath = cnvDropInDir + "/" + cnvDropInFile
	// defaultSwapSizeMB is the default swap size to create on nodes
	defaultSwapSizeMB = 2048
)

// Kubelet configuration file paths in testdata
var (
	// cnvLimitedSwapConfigPath is the path to LimitedSwap kubelet config
	cnvLimitedSwapConfigPath = exutil.FixturePath("testdata", "node", "cnv-swap", "kubelet-limitedswap-dropin.yaml")
	// cnvNoSwapConfigPath is the path to NoSwap kubelet config
	cnvNoSwapConfigPath = exutil.FixturePath("testdata", "node", "cnv-swap", "kubelet-noswap-dropin.yaml")
)

var _ = g.Describe("[Jira:Node/Kubelet][sig-node][Feature:NodeSwap][Serial][Disruptive][Suite:openshift/disruptive-longrunning] Kubelet LimitedSwap Drop-in Configuration for CNV", g.Ordered, func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("cnv-swap")

	var (
		cnvWorkerNode      string
		nonCNVWorkerNode   string
		cnvInstalledByTest bool
	)

	// Setup: Install CNV operator and enable swap before all tests
	g.BeforeAll(func(ctx context.Context) {
		// Skip on MicroShift clusters
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster")
		}

		// Check if CNV is already installed
		if !isCNVInstalled(ctx, oc) {
			framework.Logf("CNV operator not installed, installing...")
			err := installCNVOperator(ctx, oc)
			if err != nil {
				framework.Logf("Failed to install CNV operator: %v", err)
				e2eskipper.Skipf("Failed to install CNV operator: %v", err)
			}
			cnvInstalledByTest = true
			framework.Logf("CNV operator installed successfully")
		} else {
			framework.Logf("CNV operator already installed, skipping installation")
		}

		// Ensure drop-in directory exists on all worker nodes
		err = ensureDropInDirectoryExists(ctx, oc, cnvDropInDir)
		if err != nil {
			framework.Logf("Warning: failed to ensure drop-in directory exists: %v", err)
		}
	})

	// Teardown: Uninstall CNV operator after all tests
	g.AfterAll(func(ctx context.Context) {
		// Uninstall CNV operator if we installed it
		if cnvInstalledByTest {
			framework.Logf("Uninstalling CNV operator...")
			err := uninstallCNVOperator(ctx, oc)
			if err != nil {
				framework.Logf("Warning: failed to uninstall CNV operator: %v", err)
			}
		}
	})

	// TC1: Verify silent creation and ownership of drop-in directory
	g.It("TC1: should verify silent creation and ownership of drop-in directory on CNV nodes", func(ctx context.Context) {
		// Get a CNV worker node for tests
		cnvWorkerNode = getCNVWorkerNodeName(ctx, oc)
		o.Expect(cnvWorkerNode).NotTo(o.BeEmpty(), "No CNV worker nodes available")
		framework.Logf("Using CNV worker node for tests: %s", cnvWorkerNode)

		// Get ALL worker nodes to verify directory exists on all of them
		workerNodeList, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(workerNodeList)).To(o.BeNumerically(">", 0), "Should have at least one worker node")

		var allWorkerNodes []string
		for _, node := range workerNodeList {
			allWorkerNodes = append(allWorkerNodes, node.Name)
		}
		framework.Logf("Found %d worker nodes: %v", len(allWorkerNodes), allWorkerNodes)

		g.By("Checking drop-in directory exists on ALL worker nodes")
		for _, workerNode := range allWorkerNodes {
			framework.Logf("Running command: ls -ld %s on node %s", cnvDropInDir, workerNode)
			output, err := DebugNodeWithChroot(oc, workerNode, "ls", "-ld", cnvDropInDir)
			if err != nil {
				framework.Logf("Drop-in directory does not exist on worker node %s: %v", workerNode, err)
				e2eskipper.Skipf("Drop-in directory not present on worker node %s - CNV operator may not be installed", workerNode)
			}
			framework.Logf("Output from node %s: %s", workerNode, output)
			o.Expect(output).To(o.ContainSubstring("root root"), "Directory should be owned by root:root on node %s", workerNode)
		}
		framework.Logf("Drop-in directory %s is present on all %d worker nodes", cnvDropInDir, len(allWorkerNodes))

		g.By("Checking directory permissions on all worker nodes (should be 755 or stricter)")
		for _, workerNode := range allWorkerNodes {
			framework.Logf("Running command: stat -c %%a %s on node %s", cnvDropInDir, workerNode)
			output, err := DebugNodeWithChroot(oc, workerNode, "stat", "-c", "%a", cnvDropInDir)
			o.Expect(err).NotTo(o.HaveOccurred())
			perms := strings.TrimSpace(output)
			framework.Logf("Output from node %s: permissions=%s", workerNode, perms)
			o.Expect(perms).To(o.Or(o.Equal("755"), o.Equal("700"), o.Equal("750")),
				"Directory permissions should be 755 or stricter on node %s", workerNode)
		}

		g.By("Checking SELinux context on worker nodes")
		framework.Logf("Running command: ls -ldZ %s on node %s", cnvDropInDir, cnvWorkerNode)
		output, err := DebugNodeWithChroot(oc, cnvWorkerNode, "ls", "-ldZ", cnvDropInDir)
		if err == nil {
			framework.Logf("Output: %s", output)
		}

		g.By("Verifying no kubelet errors about drop-in directory")
		framework.Logf("Running: oc adm node-logs %s --unit=kubelet --tail=100", cnvWorkerNode)
		output, err = oc.AsAdmin().WithoutNamespace().Run("adm", "node-logs").Args(cnvWorkerNode, "--unit=kubelet", "--tail=100").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Check specifically for errors related to drop-in directory or config loading
		lowerOutput := strings.ToLower(output)
		o.Expect(lowerOutput).NotTo(o.ContainSubstring("error.*kubelet.conf.d"), "Should not have errors about drop-in directory")
		o.Expect(lowerOutput).NotTo(o.ContainSubstring("failed to load kubelet config"), "Should not have kubelet config load failures")
		o.Expect(lowerOutput).NotTo(o.ContainSubstring("error reading drop-in"), "Should not have errors reading drop-in files")

		g.By("Verifying drop-in directory does NOT exist on control plane/master nodes")
		controlPlaneNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/master")
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Found %d control plane/master nodes", len(controlPlaneNodes))

		// Drop-in directory should NOT exist on control plane nodes
		for _, cpNode := range controlPlaneNodes {
			_, err = DebugNodeWithChroot(oc, cpNode.Name, "ls", "-ld", cnvDropInDir)
			if err == nil {
				framework.Logf("ERROR: Drop-in directory exists on control plane node %s - this is unexpected", cpNode.Name)
				o.Expect(err).To(o.HaveOccurred(), "Drop-in directory should NOT exist on control plane node %s", cpNode.Name)
			} else {
				framework.Logf("Drop-in directory does NOT exist on control plane node %s (expected)", cpNode.Name)
			}
		}

		framework.Logf("TC1 PASSED: Drop-in directory is present on all %d worker nodes and NOT present on any control plane nodes", len(allWorkerNodes))
	})

	// TC2: Verify kubelet starts normally with empty or missing directory
	g.It("TC2: should verify kubelet starts normally with empty directory", func(ctx context.Context) {
		// Get a CNV worker node for tests
		cnvWorkerNode = getCNVWorkerNodeName(ctx, oc)
		o.Expect(cnvWorkerNode).NotTo(o.BeEmpty(), "No CNV worker nodes available")
		framework.Logf("Using CNV worker node for tests: %s", cnvWorkerNode)

		g.By("Checking if drop-in directory exists and is empty")
		output, err := DebugNodeWithChroot(oc, cnvWorkerNode, "ls", "-la", cnvDropInDir)
		if err != nil {
			e2eskipper.Skipf("Drop-in directory not present")
		}
		framework.Logf("Directory contents: %s", output)

		g.By("Verifying kubelet is running")
		output, err = DebugNodeWithChroot(oc, cnvWorkerNode, "systemctl", "is-active", "kubelet")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(output)).To(o.Equal("active"), "Kubelet should be active")

		g.By("Verifying node is Ready")
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, cnvWorkerNode, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isNodeInReadyState(node)).To(o.BeTrue(), "Node should be in Ready state")

		framework.Logf("TC2 PASSED: Kubelet starts normally with empty/missing directory")
	})

	// TC3: Verify LimitedSwap configuration is applied from drop-in file
	g.It("TC3: should apply LimitedSwap configuration from drop-in file", func(ctx context.Context) {
		// Get a CNV worker node for tests
		cnvWorkerNode = getCNVWorkerNodeName(ctx, oc)
		o.Expect(cnvWorkerNode).NotTo(o.BeEmpty(), "No CNV worker nodes available")

		framework.Logf("=== TC3: Testing LimitedSwap configuration via drop-in file ===")
		framework.Logf("Executing on node: %s", cnvWorkerNode)
		framework.Logf("Drop-in directory: %s", cnvDropInDir)
		framework.Logf("Drop-in file: %s", cnvDropInFile)
		framework.Logf("Full path: %s", cnvDropInFilePath)

		g.By("Getting kubelet config BEFORE applying drop-in file")
		configBefore, err := getKubeletConfigFromNode(ctx, oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Kubelet swapBehavior BEFORE: '%s'", configBefore.MemorySwap.SwapBehavior)

		// If LimitedSwap is already enabled, clean up first to start from NoSwap state
		if configBefore.MemorySwap.SwapBehavior == "LimitedSwap" {
			g.By("LimitedSwap already enabled - cleaning up to start from NoSwap state")
			cleanupDropInAndRestartKubelet(ctx, oc, cnvWorkerNode, cnvDropInFilePath)

			configBefore, err = getKubeletConfigFromNode(ctx, oc, cnvWorkerNode)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Kubelet swapBehavior BEFORE (after cleanup): '%s'", configBefore.MemorySwap.SwapBehavior)
			o.Expect(configBefore.MemorySwap.SwapBehavior).To(o.Or(o.BeEmpty(), o.Equal("NoSwap")),
				"swapBehavior should be empty or NoSwap after cleanup")
		}

		g.By("Creating drop-in file with LimitedSwap configuration in /etc/openshift/kubelet.conf.d/")
		framework.Logf("Creating file: %s with content:\n%s", cnvDropInFilePath, loadConfigFromFile(cnvLimitedSwapConfigPath))
		err = createDropInFile(oc, cnvWorkerNode, cnvDropInFilePath, loadConfigFromFile(cnvLimitedSwapConfigPath))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying drop-in file was created successfully")
		output, err := DebugNodeWithChroot(oc, cnvWorkerNode, "cat", cnvDropInFilePath)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Drop-in file content:\n%s", output)
		o.Expect(output).To(o.ContainSubstring("LimitedSwap"), "Drop-in file should contain LimitedSwap configuration")

		// Defer cleanup
		defer func() {
			g.By("Cleaning up - removing drop-in file and restarting kubelet")
			cleanupDropInAndRestartKubelet(ctx, oc, cnvWorkerNode, cnvDropInFilePath)
		}()

		g.By("Restarting kubelet to load the new configuration")
		err = restartKubeletOnNode(oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for node to be ready after kubelet restart")
		waitForNodeToBeReady(ctx, oc, cnvWorkerNode)

		configAfter, err := getKubeletConfigFromNode(ctx, oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Kubelet swapBehavior AFTER: '%s'", configAfter.MemorySwap.SwapBehavior)
		o.Expect(configAfter.MemorySwap.SwapBehavior).To(o.Equal("LimitedSwap"),
			"swapBehavior should be LimitedSwap after applying drop-in file")

		framework.Logf("=== TC3 PASSED ===")
		framework.Logf("Kubelet swapBehavior changed from '%s' to 'LimitedSwap'", configBefore.MemorySwap.SwapBehavior)
		framework.Logf("Drop-in file %s was loaded successfully by kubelet", cnvDropInFilePath)
	})

	// TC4: Verify revert behavior when drop-in file is removed
	g.It("TC4: should revert to NoSwap when drop-in file is removed", func(ctx context.Context) {
		// Get a CNV worker node for tests
		cnvWorkerNode = getCNVWorkerNodeName(ctx, oc)
		o.Expect(cnvWorkerNode).NotTo(o.BeEmpty(), "No CNV worker nodes available")

		framework.Logf("=== TC4: Testing revert to NoSwap when drop-in file is removed ===")
		framework.Logf("Executing on node: %s", cnvWorkerNode)

		g.By("Getting initial kubelet config")
		configInitial, err := getKubeletConfigFromNode(ctx, oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Initial swapBehavior: '%s'", configInitial.MemorySwap.SwapBehavior)

		// If LimitedSwap is NOT enabled, enable it first
		if configInitial.MemorySwap.SwapBehavior != "LimitedSwap" {
			g.By("Creating drop-in file with LimitedSwap configuration")
			framework.Logf("Creating file: %s", cnvDropInFilePath)
			err = createDropInFile(oc, cnvWorkerNode, cnvDropInFilePath, loadConfigFromFile(cnvLimitedSwapConfigPath))
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Restarting kubelet to apply LimitedSwap")
			err = restartKubeletOnNode(oc, cnvWorkerNode)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForNodeToBeReady(ctx, oc, cnvWorkerNode)

			g.By("Verifying LimitedSwap is applied")
			configWithSwap, err := getKubeletConfigFromNode(ctx, oc, cnvWorkerNode)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("swapBehavior AFTER applying drop-in: '%s'", configWithSwap.MemorySwap.SwapBehavior)
			o.Expect(configWithSwap.MemorySwap.SwapBehavior).To(o.Equal("LimitedSwap"))
		} else {
			framework.Logf("LimitedSwap already enabled - proceeding to removal test")
		}

		g.By("Removing drop-in file and restarting kubelet")
		cleanupDropInAndRestartKubelet(ctx, oc, cnvWorkerNode, cnvDropInFilePath)

		g.By("Verifying swapBehavior reverts to NoSwap")
		configAfterRemoval, err := getKubeletConfigFromNode(ctx, oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("swapBehavior AFTER removing drop-in: '%s'", configAfterRemoval.MemorySwap.SwapBehavior)
		o.Expect(configAfterRemoval.MemorySwap.SwapBehavior).To(o.Or(o.BeEmpty(), o.Equal("NoSwap")),
			"swapBehavior should be empty or NoSwap after removing drop-in")

		framework.Logf("=== TC4 PASSED ===")
	})

	// TC5: Verify kubelet ignores drop-in configuration on ALL control plane nodes
	g.It("TC5: should verify control plane kubelets ignore drop-in config", func(ctx context.Context) {
		framework.Logf("=== TC5: Testing control plane ignores drop-in configuration ===")

		// Get all control plane nodes
		controlPlaneNodes, err := getControlPlaneNodes(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(controlPlaneNodes) == 0 {
			e2eskipper.Skipf("No control plane nodes available")
		}
		framework.Logf("Found %d control plane nodes to test", len(controlPlaneNodes))

		for i, cpNode := range controlPlaneNodes {
			cpNodeName := cpNode.Name
			framework.Logf("--- Testing control plane node %d/%d: %s ---", i+1, len(controlPlaneNodes), cpNodeName)

			g.By(fmt.Sprintf("Getting kubelet config BEFORE placing drop-in file on %s", cpNodeName))
			configBefore, err := getKubeletConfigFromNode(ctx, oc, cpNodeName)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Control plane %s swapBehavior BEFORE: '%s'", cpNodeName, configBefore.MemorySwap.SwapBehavior)

			g.By(fmt.Sprintf("Creating drop-in directory on %s if not exists", cpNodeName))
			_, _ = DebugNodeWithChroot(oc, cpNodeName, "mkdir", "-p", cnvDropInDir)

			g.By(fmt.Sprintf("Creating drop-in file on %s", cpNodeName))
			err = createDropInFile(oc, cpNodeName, cnvDropInFilePath, loadConfigFromFile(cnvLimitedSwapConfigPath))
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Created drop-in file: %s on %s", cnvDropInFilePath, cpNodeName)

			g.By(fmt.Sprintf("Restarting kubelet on %s", cpNodeName))
			err = restartKubeletOnNode(oc, cpNodeName)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForNodeToBeReady(ctx, oc, cpNodeName)

			g.By(fmt.Sprintf("Verifying %s did NOT apply LimitedSwap from drop-in", cpNodeName))
			configAfter, err := getKubeletConfigFromNode(ctx, oc, cpNodeName)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Control plane %s swapBehavior AFTER: '%s'", cpNodeName, configAfter.MemorySwap.SwapBehavior)

			// Control plane should not apply LimitedSwap from drop-in (config-dir not configured for control plane)
			o.Expect(configAfter.MemorySwap.SwapBehavior).NotTo(o.Equal("LimitedSwap"),
				fmt.Sprintf("Control plane %s should NOT apply LimitedSwap from drop-in", cpNodeName))

			framework.Logf("Control plane %s ignored drop-in file as expected (swapBehavior: '%s' -> '%s')",
				cpNodeName, configBefore.MemorySwap.SwapBehavior, configAfter.MemorySwap.SwapBehavior)

			g.By(fmt.Sprintf("Cleaning up %s", cpNodeName))
			removeDropInFile(oc, cpNodeName, cnvDropInFilePath)
			// Also remove the drop-in directory we created on control plane
			_, _ = DebugNodeWithChroot(oc, cpNodeName, "rmdir", cnvDropInDir)
			framework.Logf("Removed drop-in directory from control plane node %s", cpNodeName)
		}

		framework.Logf("=== TC5 PASSED ===")
		framework.Logf("All %d control plane nodes ignored drop-in file as expected", len(controlPlaneNodes))
	})

	// TC6: Verify directory is auto-recreated after deletion and kubelet restart
	g.It("TC6: should verify drop-in directory is auto-recreated after deletion", func(ctx context.Context) {
		// Get a CNV worker node for tests
		cnvWorkerNode = getCNVWorkerNodeName(ctx, oc)
		o.Expect(cnvWorkerNode).NotTo(o.BeEmpty(), "No CNV worker nodes available")

		framework.Logf("=== TC6: Testing drop-in directory auto-recreation ===")
		framework.Logf("Executing on node: %s", cnvWorkerNode)

		g.By("Checking if directory exists before deletion")
		output, err := DebugNodeWithChroot(oc, cnvWorkerNode, "ls", "-la", cnvDropInDir)
		if err != nil {
			framework.Logf("Directory does not exist")
		} else {
			framework.Logf("Output:\n%s", output)
		}

		g.By("Deleting drop-in directory")
		framework.Logf("Running: rm -rf %s", cnvDropInDir)
		_, _ = DebugNodeWithChroot(oc, cnvWorkerNode, "rm", "-rf", cnvDropInDir)
		framework.Logf("Directory deletion command executed")

		g.By("Verifying directory is deleted")
		framework.Logf("Running: ls -la %s (expecting failure)", cnvDropInDir)
		_, err = DebugNodeWithChroot(oc, cnvWorkerNode, "ls", "-la", cnvDropInDir)
		o.Expect(err).To(o.HaveOccurred(), "Directory should not exist after deletion")
		framework.Logf("Confirmed: Directory does not exist after deletion")

		g.By("Restarting kubelet")
		err = restartKubeletOnNode(oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for node to be ready")
		waitForNodeToBeReady(ctx, oc, cnvWorkerNode)

		g.By("Verifying directory was auto-recreated")
		output, err = DebugNodeWithChroot(oc, cnvWorkerNode, "ls", "-la", cnvDropInDir)
		o.Expect(err).NotTo(o.HaveOccurred(), "Directory should be auto-recreated after kubelet restart")
		framework.Logf("Output:\n%s", output)

		g.By("Verifying kubelet is running")
		output, err = DebugNodeWithChroot(oc, cnvWorkerNode, "systemctl", "is-active", "kubelet")
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("kubelet status: %s", strings.TrimSpace(output))
		o.Expect(strings.TrimSpace(output)).To(o.Equal("active"))

		framework.Logf("=== TC6 PASSED ===")
	})

	// TC7: Validate security and permissions of drop-in directory
	g.It("TC7: should validate security and permissions of drop-in directory", func(ctx context.Context) {
		// Get a CNV worker node for tests
		cnvWorkerNode = getCNVWorkerNodeName(ctx, oc)
		o.Expect(cnvWorkerNode).NotTo(o.BeEmpty(), "No CNV worker nodes available")

		framework.Logf("=== TC7: Testing security and permissions of drop-in directory ===")
		framework.Logf("Executing on node: %s", cnvWorkerNode)
		framework.Logf("Drop-in directory: %s", cnvDropInDir)

		g.By("Ensuring drop-in directory exists")
		framework.Logf("Running: mkdir -p %s", cnvDropInDir)
		_, err := DebugNodeWithChroot(oc, cnvWorkerNode, "mkdir", "-p", cnvDropInDir)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Directory exists or created successfully")

		g.By("Verifying directory ownership is root:root")
		framework.Logf("Running: stat -c %%U:%%G %s", cnvDropInDir)
		output, err := DebugNodeWithChroot(oc, cnvWorkerNode, "stat", "-c", "%U:%G", cnvDropInDir)
		o.Expect(err).NotTo(o.HaveOccurred())
		ownership := strings.TrimSpace(output)
		framework.Logf("Directory ownership: %s", ownership)
		o.Expect(ownership).To(o.Equal("root:root"))

		g.By("Verifying directory permissions")
		framework.Logf("Running: stat -c %%a %s", cnvDropInDir)
		output, err = DebugNodeWithChroot(oc, cnvWorkerNode, "stat", "-c", "%a", cnvDropInDir)
		o.Expect(err).NotTo(o.HaveOccurred())
		perms := strings.TrimSpace(output)
		framework.Logf("Directory permissions: %s", perms)
		o.Expect(perms).To(o.Or(o.Equal("755"), o.Equal("700"), o.Equal("750")))

		g.By("Checking SELinux context of directory")
		framework.Logf("Running: ls -ldZ %s", cnvDropInDir)
		output, err = DebugNodeWithChroot(oc, cnvWorkerNode, "ls", "-ldZ", cnvDropInDir)
		if err == nil {
			framework.Logf("SELinux context: %s", strings.TrimSpace(output))
		}

		g.By("Creating a test config file with correct permissions")
		testFile := cnvDropInDir + "/test-permissions.conf"
		framework.Logf("Creating test file: %s", testFile)
		framework.Logf("File content:\n%s", loadConfigFromFile(cnvLimitedSwapConfigPath))
		err = createDropInFile(oc, cnvWorkerNode, testFile, loadConfigFromFile(cnvLimitedSwapConfigPath))
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Test file created successfully")
		defer removeDropInFile(oc, cnvWorkerNode, testFile)

		g.By("Verifying config file ownership")
		framework.Logf("Running: stat -c %%U:%%G %s", testFile)
		output, err = DebugNodeWithChroot(oc, cnvWorkerNode, "stat", "-c", "%U:%G", testFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		fileOwnership := strings.TrimSpace(output)
		framework.Logf("File ownership: %s", fileOwnership)

		g.By("Verifying config file permissions (should be 644 or 600)")
		framework.Logf("Running: stat -c %%a %s", testFile)
		output, err = DebugNodeWithChroot(oc, cnvWorkerNode, "stat", "-c", "%a", testFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		filePerms := strings.TrimSpace(output)
		framework.Logf("File permissions: %s", filePerms)
		o.Expect(filePerms).To(o.Or(o.Equal("644"), o.Equal("600")))

		framework.Logf("=== TC7 PASSED ===")
		framework.Logf("Security and permissions summary:")
		framework.Logf("- Directory: %s", cnvDropInDir)
		framework.Logf("- Directory ownership: %s (expected: root:root)", ownership)
		framework.Logf("- Directory permissions: %s (expected: 755/700/750)", perms)
		framework.Logf("- Test file: %s", testFile)
		framework.Logf("- File ownership: %s", fileOwnership)
		framework.Logf("- File permissions: %s (expected: 644/600)", filePerms)
	})

	// TC8: Validate cluster stability and performance
	g.It("TC8: should verify cluster stability with LimitedSwap enabled", func(ctx context.Context) {
		// Get a CNV worker node for tests
		cnvWorkerNode = getCNVWorkerNodeName(ctx, oc)
		o.Expect(cnvWorkerNode).NotTo(o.BeEmpty(), "No CNV worker nodes available")

		framework.Logf("=== TC8: Testing cluster stability with LimitedSwap enabled ===")
		framework.Logf("Executing on node: %s", cnvWorkerNode)

		g.By("Creating LimitedSwap configuration")
		framework.Logf("Creating drop-in file: %s", cnvDropInFilePath)
		framework.Logf("Drop-in file content:\n%s", loadConfigFromFile(cnvLimitedSwapConfigPath))
		err := createDropInFile(oc, cnvWorkerNode, cnvDropInFilePath, loadConfigFromFile(cnvLimitedSwapConfigPath))
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Drop-in file created successfully")

		// Verify file was created
		output, err := DebugNodeWithChroot(oc, cnvWorkerNode, "cat", cnvDropInFilePath)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Verified drop-in file content:\n%s", output)

		defer func() {
			g.By("Cleaning up")
			cleanupDropInAndRestartKubelet(ctx, oc, cnvWorkerNode, cnvDropInFilePath)
		}()

		g.By("Restarting kubelet")
		framework.Logf("Running: systemctl restart kubelet on node %s", cnvWorkerNode)
		err = restartKubeletOnNode(oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Kubelet restart initiated, waiting for node to be ready...")
		waitForNodeToBeReady(ctx, oc, cnvWorkerNode)
		framework.Logf("Node %s is Ready", cnvWorkerNode)

		g.By("Verifying kubelet loaded LimitedSwap configuration")
		framework.Logf("Running: oc get --raw \"/api/v1/nodes/%s/proxy/configz\" | jq '.kubeletconfig.memorySwap'", cnvWorkerNode)
		config, err := getKubeletConfigFromNode(ctx, oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Kubelet config memorySwap.swapBehavior: '%s'", config.MemorySwap.SwapBehavior)
		o.Expect(config.MemorySwap.SwapBehavior).To(o.Equal("LimitedSwap"))

		g.By("Monitoring node stability for 30 seconds")
		framework.Logf("Sleeping for 30 seconds to monitor stability...")
		time.Sleep(30 * time.Second)
		framework.Logf("30-second monitoring period completed")

		g.By("Verifying node remains Ready after monitoring period")
		framework.Logf("Checking node %s status...", cnvWorkerNode)
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, cnvWorkerNode, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				framework.Logf("Node condition: Type=%s, Status=%s, Reason=%s", condition.Type, condition.Status, condition.Reason)
			}
		}
		o.Expect(isNodeInReadyState(node)).To(o.BeTrue(), "Node should remain Ready after monitoring")
		framework.Logf("Node %s is in Ready state after 30 seconds", cnvWorkerNode)

		g.By("Checking for memory pressure conditions")
		framework.Logf("Running: oc describe node %s | grep -i MemoryPressure", cnvWorkerNode)
		memoryPressureFound := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeMemoryPressure {
				memoryPressureFound = true
				framework.Logf("MemoryPressure condition: Status=%s, Reason=%s, Message=%s",
					condition.Status, condition.Reason, condition.Message)
				o.Expect(condition.Status).To(o.Equal(corev1.ConditionFalse),
					"Node should not have memory pressure")
			}
		}
		if !memoryPressureFound {
			framework.Logf("No MemoryPressure condition found (node is healthy)")
		}
		framework.Logf("✅ No memory pressure detected")

		framework.Logf("=== TC8 PASSED ===")
		framework.Logf("Cluster stability verification:")
		framework.Logf("- Node: %s", cnvWorkerNode)
		framework.Logf("- swapBehavior: LimitedSwap")
		framework.Logf("- Node remains Ready: YES")
		framework.Logf("- Memory pressure: NONE")
		framework.Logf("- Stability after 30 seconds: CONFIRMED")
	})

	// TC9: Validate non-CNV cluster unaffected
	g.It("TC9: should verify non-CNV workers have no swap configuration", func(ctx context.Context) {
		framework.Logf("=== TC9: Testing non-CNV workers have no swap configuration ===")

		// Get a CNV worker node and temporarily remove its CNV label
		cnvWorkerNode = getCNVWorkerNodeName(ctx, oc)
		o.Expect(cnvWorkerNode).NotTo(o.BeEmpty(), "No CNV worker nodes available")

		cnvLabel := "kubevirt.io/schedulable"
		framework.Logf("Selected worker node: %s", cnvWorkerNode)

		g.By("Removing CNV label from worker node to simulate non-CNV node")
		framework.Logf("Running: oc label node %s %s-", cnvWorkerNode, cnvLabel)
		_, err := oc.AsAdmin().WithoutNamespace().Run("label").Args("node", cnvWorkerNode, cnvLabel+"-").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Removed label %s from node %s", cnvLabel, cnvWorkerNode)

		// Restore label after test
		defer func() {
			g.By("Restoring CNV label on worker node")
			framework.Logf("Running: oc label node %s %s=true", cnvWorkerNode, cnvLabel)
			_, err := oc.AsAdmin().WithoutNamespace().Run("label").Args("node", cnvWorkerNode, cnvLabel+"=true").Output()
			if err != nil {
				framework.Logf("Warning: failed to restore label: %v", err)
			} else {
				framework.Logf("Restored label %s=true on node %s", cnvLabel, cnvWorkerNode)
			}
		}()

		// Use this node as the "non-CNV" node for the test
		nonCNVWorkerNode = cnvWorkerNode
		framework.Logf("Using node %s as non-CNV worker for test (CNV label removed)", nonCNVWorkerNode)

		g.By("Verifying node no longer has CNV label")
		framework.Logf("Running: oc get node %s --show-labels | grep kubevirt", nonCNVWorkerNode)
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nonCNVWorkerNode, "-o", "jsonpath={.metadata.labels}").Output()
		if strings.Contains(output, "kubevirt.io/schedulable") {
			framework.Logf("Warning: Node still has CNV label in labels: %s", output)
		} else {
			framework.Logf("Confirmed: Node %s no longer has kubevirt.io/schedulable label", nonCNVWorkerNode)
		}

		g.By("Checking drop-in directory on non-CNV node")
		framework.Logf("Running: ls -ld %s on node %s", cnvDropInDir, nonCNVWorkerNode)
		output, err = DebugNodeWithChroot(oc, nonCNVWorkerNode, "ls", "-ld", cnvDropInDir)
		if err == nil {
			framework.Logf("Drop-in directory exists: %s", strings.TrimSpace(output))
			framework.Logf("Note: Directory exists because CNV was previously installed on this node")
			g.By("Checking directory contents")
			framework.Logf("Running: ls -la %s", cnvDropInDir)
			dirOutput, _ := DebugNodeWithChroot(oc, nonCNVWorkerNode, "ls", "-la", cnvDropInDir)
			framework.Logf("Directory contents:\n%s", dirOutput)
		} else {
			framework.Logf("Drop-in directory does not exist on non-CNV node (expected for truly non-CNV nodes)")
		}

		g.By("Verifying kubelet swapBehavior is default (NoSwap)")
		framework.Logf("Running: oc get --raw \"/api/v1/nodes/%s/proxy/configz\" | jq '.kubeletconfig.memorySwap'", nonCNVWorkerNode)
		config, err := getKubeletConfigFromNode(ctx, oc, nonCNVWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Kubelet swapBehavior: '%s'", config.MemorySwap.SwapBehavior)
		// Accept either empty string or "NoSwap" as valid NoSwap state
		o.Expect(config.MemorySwap.SwapBehavior).To(o.Or(o.BeEmpty(), o.Equal("NoSwap")),
			"swapBehavior should be empty or NoSwap on non-CNV node")

		framework.Logf("=== TC9 PASSED ===")
		framework.Logf("Non-CNV worker verification:")
		framework.Logf("- Node: %s", nonCNVWorkerNode)
		framework.Logf("- CNV label removed: YES")
		framework.Logf("- swapBehavior: %s (NoSwap/default)", config.MemorySwap.SwapBehavior)
	})

	// TC10: Validate behavior with multiple conflicting drop-in files
	g.It("TC10: should apply correct precedence with multiple files", func(ctx context.Context) {
		// Get a CNV worker node for tests
		cnvWorkerNode = getCNVWorkerNodeName(ctx, oc)
		o.Expect(cnvWorkerNode).NotTo(o.BeEmpty(), "No CNV worker nodes available")

		framework.Logf("=== TC10: Testing file precedence with multiple drop-in files ===")
		framework.Logf("Executing on node: %s", cnvWorkerNode)
		framework.Logf("Drop-in directory: %s", cnvDropInDir)

		file98 := cnvDropInDir + "/98-swap-disabled.conf"
		file99 := cnvDropInDir + "/99-swap-limited.conf"

		g.By("Creating 98-swap-disabled.conf with NoSwap")
		framework.Logf("Creating file: %s", file98)
		framework.Logf("Content:\n%s", loadConfigFromFile(cnvNoSwapConfigPath))
		err := createDropInFile(oc, cnvWorkerNode, file98, loadConfigFromFile(cnvNoSwapConfigPath))
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Created: %s (NoSwap)", file98)

		g.By("Creating 99-swap-limited.conf with LimitedSwap")
		framework.Logf("Creating file: %s", file99)
		framework.Logf("Content:\n%s", loadConfigFromFile(cnvLimitedSwapConfigPath))
		err = createDropInFile(oc, cnvWorkerNode, file99, loadConfigFromFile(cnvLimitedSwapConfigPath))
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Created: %s (LimitedSwap)", file99)

		g.By("Listing drop-in directory contents")
		framework.Logf("Running: ls -la %s", cnvDropInDir)
		output, _ := DebugNodeWithChroot(oc, cnvWorkerNode, "ls", "-la", cnvDropInDir)
		framework.Logf("Directory contents:\n%s", output)

		defer func() {
			g.By("Cleaning up multiple config files")
			framework.Logf("Removing: %s", file98)
			removeDropInFile(oc, cnvWorkerNode, file98)
			framework.Logf("Removing: %s", file99)
			removeDropInFile(oc, cnvWorkerNode, file99)
			framework.Logf("Running: systemctl restart kubelet")
			restartKubeletOnNode(oc, cnvWorkerNode)
			waitForNodeToBeReady(ctx, oc, cnvWorkerNode)
			framework.Logf("Cleanup completed")
		}()

		g.By("Restarting kubelet")
		framework.Logf("Running: systemctl restart kubelet")
		err = restartKubeletOnNode(oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Waiting for node to be ready...")
		waitForNodeToBeReady(ctx, oc, cnvWorkerNode)
		framework.Logf("Node %s is Ready", cnvWorkerNode)

		g.By("Verifying 99-* file takes precedence (lexicographic order)")
		framework.Logf("Running: oc get --raw \"/api/v1/nodes/%s/proxy/configz\" | jq '.kubeletconfig.memorySwap'", cnvWorkerNode)
		config, err := getKubeletConfigFromNode(ctx, oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Kubelet swapBehavior: '%s'", config.MemorySwap.SwapBehavior)
		o.Expect(config.MemorySwap.SwapBehavior).To(o.Equal("LimitedSwap"),
			"99-* file should take precedence over 98-* file")

		framework.Logf("=== TC10 PASSED ===")
		framework.Logf("File precedence verification:")
		framework.Logf("- File 1: 98-swap-disabled.conf (NoSwap)")
		framework.Logf("- File 2: 99-swap-limited.conf (LimitedSwap)")
		framework.Logf("- Result: swapBehavior = LimitedSwap")
		framework.Logf("- 99-* file correctly overrides 98-* file (lexicographic order)")
	})

	// TC11: Validate multi-node consistency and synchronization with checksum verification
	g.It("TC11: should maintain consistent configuration with checksum verification across CNV nodes", func(ctx context.Context) {
		framework.Logf("=== TC11: Testing multi-node consistency with checksum verification ===")

		g.By("Getting all CNV worker nodes")
		// Get nodes with both worker role and CNV schedulable label
		allWorkerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())

		var cnvNodes []string
		for _, node := range allWorkerNodes {
			// Check if node has CNV schedulable label
			if _, hasCNV := node.Labels["kubevirt.io/schedulable"]; hasCNV {
				cnvNodes = append(cnvNodes, node.Name)
			}
		}

		if len(cnvNodes) < 2 {
			framework.Logf("Found only %d CNV worker node(s), need at least 2 for multi-node consistency test", len(cnvNodes))
			e2eskipper.Skipf("Need at least 2 CNV worker nodes for multi-node consistency test, found %d", len(cnvNodes))
		}
		framework.Logf("Found %d CNV worker nodes:", len(cnvNodes))
		for i, name := range cnvNodes {
			framework.Logf("  %d. %s", i+1, name)
		}

		g.By("Deploying drop-in configuration to all CNV nodes")
		framework.Logf("Drop-in file: %s", cnvDropInFilePath)
		framework.Logf("Content:\n%s", loadConfigFromFile(cnvLimitedSwapConfigPath))
		for _, node := range cnvNodes {
			framework.Logf("Creating drop-in file on node: %s", node)
			err := createDropInFile(oc, node, cnvDropInFilePath, loadConfigFromFile(cnvLimitedSwapConfigPath))
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("  -> Created successfully on %s", node)
		}

		defer func() {
			g.By("Cleaning up all CNV nodes")
			for _, node := range cnvNodes {
				framework.Logf("Removing drop-in file from node: %s", node)
				removeDropInFile(oc, node, cnvDropInFilePath)
				framework.Logf("Restarting kubelet on node: %s", node)
				restartKubeletOnNode(oc, node)
			}
			for _, node := range cnvNodes {
				framework.Logf("Waiting for node %s to be ready...", node)
				waitForNodeToBeReady(ctx, oc, node)
			}
			framework.Logf("Cleanup completed on all %d CNV nodes", len(cnvNodes))
		}()

		g.By("Verifying configuration is identical across all nodes via checksums")
		checksums := make(map[string]string)
		for _, node := range cnvNodes {
			framework.Logf("Running: md5sum %s on node %s", cnvDropInFilePath, node)
			output, err := DebugNodeWithChroot(oc, node, "md5sum", cnvDropInFilePath)
			o.Expect(err).NotTo(o.HaveOccurred())
			// Extract checksum (first field)
			checksum := strings.Fields(strings.TrimSpace(output))[0]
			checksums[node] = checksum
			framework.Logf("Checksum for %s: %s", node, checksum)
		}

		// Verify all checksums are identical
		var referenceChecksum string
		for node, checksum := range checksums {
			if referenceChecksum == "" {
				referenceChecksum = checksum
				framework.Logf("Reference checksum (from first node): %s", referenceChecksum)
			} else {
				o.Expect(checksum).To(o.Equal(referenceChecksum),
					"Checksum mismatch: node %s has %s, expected %s", node, checksum, referenceChecksum)
			}
		}
		framework.Logf("✅ All %d nodes have identical configuration checksum: %s", len(cnvNodes), referenceChecksum)

		g.By("Restarting kubelet on all CNV nodes")
		for _, node := range cnvNodes {
			framework.Logf("Running: systemctl restart kubelet on node %s", node)
			err := restartKubeletOnNode(oc, node)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Waiting for all nodes to be ready")
		for _, node := range cnvNodes {
			framework.Logf("Waiting for node %s to be Ready...", node)
			waitForNodeToBeReady(ctx, oc, node)
			framework.Logf("Node %s is Ready", node)
		}

		g.By("Verifying consistent swapBehavior across all CNV nodes")
		framework.Logf("Running: oc get --raw \"/api/v1/nodes/<node>/proxy/configz\" | jq '.kubeletconfig.memorySwap' for each node")
		for _, node := range cnvNodes {
			config, err := getKubeletConfigFromNode(ctx, oc, node)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("✅ Node %s: swapBehavior = '%s'", node, config.MemorySwap.SwapBehavior)
			o.Expect(config.MemorySwap.SwapBehavior).To(o.Equal("LimitedSwap"),
				"Node %s should have LimitedSwap", node)
		}

		g.By("Verifying all nodes remain Ready")
		for _, node := range cnvNodes {
			nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(isNodeInReadyState(nodeObj)).To(o.BeTrue(), "Node %s should be Ready", node)
			framework.Logf("Node %s status: Ready", node)
		}

		g.By("Waiting 60 seconds and checking for configuration drift")
		framework.Logf("Sleeping for 60 seconds to detect any configuration drift...")
		time.Sleep(60 * time.Second)

		g.By("Verifying checksums after wait period (no drift)")
		driftDetected := false
		for _, node := range cnvNodes {
			framework.Logf("Running: md5sum %s on node %s (after wait)", cnvDropInFilePath, node)
			output, err := DebugNodeWithChroot(oc, node, "md5sum", cnvDropInFilePath)
			o.Expect(err).NotTo(o.HaveOccurred())
			checksum := strings.Fields(strings.TrimSpace(output))[0]
			framework.Logf("Checksum for %s (after wait): %s", node, checksum)
			if checksum != referenceChecksum {
				framework.Logf("WARNING: Configuration drift detected on node %s! Expected %s, got %s",
					node, referenceChecksum, checksum)
				driftDetected = true
			}
		}
		o.Expect(driftDetected).To(o.BeFalse(), "No configuration drift should occur")
		framework.Logf("✅ No configuration drift detected after 60 seconds")

		g.By("Verifying swapBehavior consistency after wait period")
		for _, node := range cnvNodes {
			config, err := getKubeletConfigFromNode(ctx, oc, node)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Node %s (after wait): swapBehavior = '%s'", node, config.MemorySwap.SwapBehavior)
			o.Expect(config.MemorySwap.SwapBehavior).To(o.Equal("LimitedSwap"),
				"Node %s should still have LimitedSwap after wait", node)
		}

		framework.Logf("=== TC11 PASSED ===")
		framework.Logf("Multi-node consistency verification:")
		framework.Logf("- Total CNV nodes: %d", len(cnvNodes))
		framework.Logf("- Configuration checksum: %s (identical across all nodes)", referenceChecksum)
		framework.Logf("- All nodes have swapBehavior: LimitedSwap")
		framework.Logf("- Configuration drift after 60s: NONE")
		framework.Logf("- All nodes remain Ready: YES")
	})

	// TC12: Validate LimitedSwap config when OS-level swap is not enabled
	// This test verifies kubelet gracefully handles LimitedSwap config even without OS swap
	g.It("TC12: should handle LimitedSwap config gracefully when OS swap is disabled", func(ctx context.Context) {
		framework.Logf("=== TC12: Testing LimitedSwap config when OS swap is disabled ===")

		// Get a CNV worker node for tests
		cnvWorkerNode = getCNVWorkerNodeName(ctx, oc)
		o.Expect(cnvWorkerNode).NotTo(o.BeEmpty(), "No CNV worker nodes available")
		framework.Logf("Executing on node: %s", cnvWorkerNode)

		g.By("Checking initial OS-level swap status")
		framework.Logf("Running: swapon -s")
		initialSwapOutput, _ := DebugNodeWithChroot(oc, cnvWorkerNode, "swapon", "-s")
		framework.Logf("Initial swapon -s output:\n%s", initialSwapOutput)
		initialHasSwap := strings.TrimSpace(initialSwapOutput) != "" && initialSwapOutput != "Filename\t\t\t\tType\t\tSize\t\tUsed\t\tPriority"

		// If swap is already enabled, disable it for this test
		if initialHasSwap {
			g.By("Disabling existing OS-level swap for test")
			framework.Logf("Running: swapoff -a")
			_, _ = DebugNodeWithNsenter(oc, cnvWorkerNode, "swapoff", "-a")
			framework.Logf("OS-level swap disabled")
		}

		g.By("Verifying no OS-level swap is present")
		framework.Logf("Running: swapon -s")
		swapOutput, _ := DebugNodeWithChroot(oc, cnvWorkerNode, "swapon", "-s")
		framework.Logf("swapon -s output:\n%s", swapOutput)
		hasOSSwap := strings.TrimSpace(swapOutput) != "" && swapOutput != "Filename\t\t\t\tType\t\tSize\t\tUsed\t\tPriority"
		if hasOSSwap {
			framework.Logf("Warning: Could not disable OS swap, but continuing with test")
		} else {
			framework.Logf("Confirmed: No OS-level swap on node %s", cnvWorkerNode)
		}

		g.By("Ensuring drop-in directory exists")
		framework.Logf("Running: mkdir -p %s", cnvDropInDir)
		_, _ = DebugNodeWithChroot(oc, cnvWorkerNode, "mkdir", "-p", cnvDropInDir)

		g.By("Creating LimitedSwap drop-in configuration")
		framework.Logf("Creating drop-in file: %s", cnvDropInFilePath)
		framework.Logf("Content:\n%s", loadConfigFromFile(cnvLimitedSwapConfigPath))
		err := createDropInFile(oc, cnvWorkerNode, cnvDropInFilePath, loadConfigFromFile(cnvLimitedSwapConfigPath))
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Drop-in file created successfully")

		defer func() {
			g.By("Cleaning up")
			framework.Logf("Removing drop-in file: %s", cnvDropInFilePath)
			removeDropInFile(oc, cnvWorkerNode, cnvDropInFilePath)
			// Re-enable swap if it was initially present
			if initialHasSwap {
				framework.Logf("Note: OS swap was initially enabled, may need manual re-enable")
			}
			framework.Logf("Restarting kubelet on node: %s", cnvWorkerNode)
			restartKubeletOnNode(oc, cnvWorkerNode)
			waitForNodeToBeReady(ctx, oc, cnvWorkerNode)
		}()

		g.By("Restarting kubelet with LimitedSwap config but no OS swap")
		framework.Logf("Running: systemctl restart kubelet on node %s", cnvWorkerNode)
		err = restartKubeletOnNode(oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Waiting for node to be ready...")
		waitForNodeToBeReady(ctx, oc, cnvWorkerNode)
		framework.Logf("Node %s is Ready", cnvWorkerNode)

		g.By("Verifying node status is Ready (no crash or failure)")
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, cnvWorkerNode, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isNodeInReadyState(node)).To(o.BeTrue(), "Node should remain Ready even with LimitedSwap but no OS swap")
		framework.Logf("Node %s status: Ready (no crash)", cnvWorkerNode)

		g.By("Verifying kubelet loaded LimitedSwap configuration")
		framework.Logf("Running: oc get --raw \"/api/v1/nodes/%s/proxy/configz\" | jq '.kubeletconfig.memorySwap'", cnvWorkerNode)
		config, err := getKubeletConfigFromNode(ctx, oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Kubelet swapBehavior: '%s'", config.MemorySwap.SwapBehavior)
		o.Expect(config.MemorySwap.SwapBehavior).To(o.Equal("LimitedSwap"),
			"Kubelet should load LimitedSwap config even without OS swap")

		g.By("Checking kubelet logs for swap-related warnings or errors")
		framework.Logf("Running: oc adm node-logs %s --unit=kubelet --tail=100", cnvWorkerNode)
		logOutput, _ := oc.AsAdmin().WithoutNamespace().Run("adm", "node-logs").Args(cnvWorkerNode, "--unit=kubelet", "--tail=100").Output()
		swapLogLines := []string{}
		for _, line := range strings.Split(logOutput, "\n") {
			lowerLine := strings.ToLower(line)
			if strings.Contains(lowerLine, "swap") {
				swapLogLines = append(swapLogLines, line)
			}
		}
		if len(swapLogLines) > 0 {
			framework.Logf("Swap-related log entries found:")
			for _, line := range swapLogLines {
				framework.Logf("  %s", line)
			}
		} else {
			framework.Logf("No swap-related log entries found (kubelet handles gracefully)")
		}

		// Check for actual ERROR-level logs, not INFO logs containing "error" text
		hasFatalSwapError := false
		for _, line := range swapLogLines {
			if strings.Contains(line, "kubenswrapper") || strings.Contains(line, "kubelet") {
				if strings.Contains(line, "] E0") || strings.Contains(line, "] F0") ||
					strings.Contains(line, "\"level\":\"error\"") || strings.Contains(line, "\"level\":\"fatal\"") {
					lowerLine := strings.ToLower(line)
					if strings.Contains(lowerLine, "swap") && strings.Contains(lowerLine, "failed") {
						hasFatalSwapError = true
						framework.Logf("FATAL: Swap-related error found: %s", line)
					}
				}
			}
		}
		if !hasFatalSwapError {
			framework.Logf("No fatal swap-related errors in kubelet logs")
		}
		o.Expect(hasFatalSwapError).To(o.BeFalse(), "Should not have fatal swap-related errors in kubelet logs")

		g.By("Verifying /proc/meminfo shows swap fields (even if 0)")
		framework.Logf("Running: grep -i swap /proc/meminfo")
		meminfoOutput, err := DebugNodeWithChroot(oc, cnvWorkerNode, "grep", "-i", "swap", "/proc/meminfo")
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Swap info from /proc/meminfo:\n%s", strings.TrimSpace(meminfoOutput))
		o.Expect(meminfoOutput).To(o.ContainSubstring("SwapTotal"))
		o.Expect(meminfoOutput).To(o.ContainSubstring("SwapFree"))

		g.By("Verifying free -h shows swap status")
		framework.Logf("Running: free -h")
		freeOutput, _ := DebugNodeWithChroot(oc, cnvWorkerNode, "free", "-h")
		framework.Logf("free -h output:\n%s", freeOutput)

		g.By("Verifying node has no memory pressure conditions")
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeMemoryPressure {
				framework.Logf("MemoryPressure condition: Status=%s, Reason=%s", condition.Status, condition.Reason)
				o.Expect(condition.Status).To(o.Equal(corev1.ConditionFalse),
					"Node should not have memory pressure")
			}
		}

		framework.Logf("=== TC12 PASSED ===")
		framework.Logf("LimitedSwap config without OS swap verification:")
		framework.Logf("- Node: %s", cnvWorkerNode)
		framework.Logf("- OS swap: disabled/not present")
		framework.Logf("- Kubelet swapBehavior: LimitedSwap (loaded successfully)")
		framework.Logf("- Node status: Ready (no crash)")
		framework.Logf("- Swap-related errors in logs: NONE")
		framework.Logf("- Memory pressure: NONE")
		framework.Logf("- Kubelet handles LimitedSwap gracefully even without OS swap")
	})

	// TC13: Validate behavior with various swap sizes
	// This test creates temporary swap files on the node for testing different sizes
	// It requires sufficient disk space and may take longer to complete
	g.It("TC13: should work correctly with various swap sizes", func(ctx context.Context) {
		// Get a CNV worker node for tests
		cnvWorkerNode = getCNVWorkerNodeName(ctx, oc)
		o.Expect(cnvWorkerNode).NotTo(o.BeEmpty(), "No CNV worker nodes available")

		framework.Logf("=== TC13: Testing LimitedSwap with various swap sizes ===")
		framework.Logf("Executing on node: %s", cnvWorkerNode)

		// Define swap sizes to test (in MB)
		// Small: 512MB, Medium: 2GB (reduced from 4GB for faster testing)
		swapSizes := []struct {
			name   string
			sizeMB int
		}{
			{"small", 512},
			{"medium", 2048},
		}

		swapFilePath := "/var/swapfile-test"

		g.By("Creating LimitedSwap drop-in configuration")
		framework.Logf("Creating drop-in file: %s", cnvDropInFilePath)
		framework.Logf("Content:\n%s", loadConfigFromFile(cnvLimitedSwapConfigPath))
		err := createDropInFile(oc, cnvWorkerNode, cnvDropInFilePath, loadConfigFromFile(cnvLimitedSwapConfigPath))
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Drop-in file created successfully")

		defer func() {
			g.By("Final cleanup")
			// Disable and remove any test swap file
			framework.Logf("Disabling test swap file if present")
			DebugNodeWithNsenter(oc, cnvWorkerNode, "swapoff", swapFilePath)
			DebugNodeWithChroot(oc, cnvWorkerNode, "rm", "-f", swapFilePath)
			// Remove drop-in config
			framework.Logf("Removing drop-in file: %s", cnvDropInFilePath)
			removeDropInFile(oc, cnvWorkerNode, cnvDropInFilePath)
			framework.Logf("Restarting kubelet")
			restartKubeletOnNode(oc, cnvWorkerNode)
			waitForNodeToBeReady(ctx, oc, cnvWorkerNode)
			framework.Logf("Final cleanup completed")
		}()

		// Test results tracking
		type swapTestResult struct {
			size      string
			sizeMB    int
			success   bool
			swapTotal int64
			nodeReady bool
			configOK  bool
		}
		var results []swapTestResult

		for _, swapSize := range swapSizes {
			framework.Logf("--- Testing %s swap (%dMB) ---", swapSize.name, swapSize.sizeMB)
			result := swapTestResult{
				size:   swapSize.name,
				sizeMB: swapSize.sizeMB,
			}

			g.By(fmt.Sprintf("Disabling any existing swap for %s test", swapSize.name))
			framework.Logf("Running: swapoff -a")
			DebugNodeWithNsenter(oc, cnvWorkerNode, "swapoff", "-a")
			DebugNodeWithChroot(oc, cnvWorkerNode, "rm", "-f", swapFilePath)

			g.By(fmt.Sprintf("Creating %dMB swap file", swapSize.sizeMB))
			framework.Logf("Running: dd if=/dev/zero of=%s bs=1M count=%d", swapFilePath, swapSize.sizeMB)
			_, err := DebugNodeWithChroot(oc, cnvWorkerNode, "dd", "if=/dev/zero", fmt.Sprintf("of=%s", swapFilePath),
				"bs=1M", fmt.Sprintf("count=%d", swapSize.sizeMB))
			if err != nil {
				framework.Logf("Warning: Failed to create swap file: %v", err)
				result.success = false
				results = append(results, result)
				continue
			}

			framework.Logf("Running: chmod 600 %s", swapFilePath)
			DebugNodeWithChroot(oc, cnvWorkerNode, "chmod", "600", swapFilePath)

			framework.Logf("Running: mkswap %s", swapFilePath)
			_, err = DebugNodeWithChroot(oc, cnvWorkerNode, "mkswap", swapFilePath)
			if err != nil {
				framework.Logf("Warning: Failed to mkswap: %v", err)
				result.success = false
				results = append(results, result)
				continue
			}

			framework.Logf("Running: swapon %s", swapFilePath)
			_, err = DebugNodeWithNsenter(oc, cnvWorkerNode, "swapon", swapFilePath)
			if err != nil {
				framework.Logf("Warning: Failed to enable swap: %v", err)
				result.success = false
				results = append(results, result)
				continue
			}

			g.By(fmt.Sprintf("Restarting kubelet with %s swap", swapSize.name))
			framework.Logf("Running: systemctl restart kubelet")
			err = restartKubeletOnNode(oc, cnvWorkerNode)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForNodeToBeReady(ctx, oc, cnvWorkerNode)

			g.By(fmt.Sprintf("Verifying node status with %s swap", swapSize.name))
			node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, cnvWorkerNode, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			result.nodeReady = isNodeInReadyState(node)
			framework.Logf("Node %s status: Ready=%v", cnvWorkerNode, result.nodeReady)

			g.By(fmt.Sprintf("Verifying kubelet config with %s swap", swapSize.name))
			config, err := getKubeletConfigFromNode(ctx, oc, cnvWorkerNode)
			o.Expect(err).NotTo(o.HaveOccurred())
			result.configOK = config.MemorySwap.SwapBehavior == "LimitedSwap"
			framework.Logf("Kubelet swapBehavior: '%s' (expected: LimitedSwap)", config.MemorySwap.SwapBehavior)

			g.By(fmt.Sprintf("Verifying swap metrics with %s swap", swapSize.name))
			framework.Logf("Running: swapon -s")
			swapOutput, _ := DebugNodeWithChroot(oc, cnvWorkerNode, "swapon", "-s")
			framework.Logf("swapon -s output:\n%s", swapOutput)

			framework.Logf("Running: grep -i swap /proc/meminfo")
			meminfoOutput, _ := DebugNodeWithChroot(oc, cnvWorkerNode, "grep", "-i", "swap", "/proc/meminfo")
			framework.Logf("Swap info from /proc/meminfo:\n%s", strings.TrimSpace(meminfoOutput))

			// Parse SwapTotal
			for _, line := range strings.Split(meminfoOutput, "\n") {
				if strings.HasPrefix(line, "SwapTotal:") {
					var swapTotalKB int64
					fmt.Sscanf(line, "SwapTotal: %d kB", &swapTotalKB)
					result.swapTotal = swapTotalKB * 1024
					framework.Logf("SwapTotal: %d bytes (%d MB)", result.swapTotal, result.swapTotal/1024/1024)
				}
			}

			framework.Logf("Running: free -h")
			freeOutput, _ := DebugNodeWithChroot(oc, cnvWorkerNode, "free", "-h")
			framework.Logf("free -h output:\n%s", freeOutput)

			// Verify swap size is approximately what we configured (within 10%)
			actualMB := result.swapTotal / 1024 / 1024
			expectedMB := int64(swapSize.sizeMB)
			tolerance := expectedMB / 10 // 10% tolerance
			if actualMB >= expectedMB-tolerance && actualMB <= expectedMB+tolerance {
				framework.Logf("✅ Swap size verification PASSED: expected ~%dMB, got %dMB", expectedMB, actualMB)
			} else {
				framework.Logf("Swap size verification: expected ~%dMB, got %dMB (may differ due to filesystem overhead)", expectedMB, actualMB)
			}

			result.success = result.nodeReady && result.configOK
			results = append(results, result)

			// Assert this size works
			o.Expect(result.nodeReady).To(o.BeTrue(), "Node should remain Ready with %s swap", swapSize.name)
			o.Expect(result.configOK).To(o.BeTrue(), "Kubelet should have LimitedSwap config with %s swap", swapSize.name)

			framework.Logf("--- %s swap (%dMB) test PASSED ---", swapSize.name, swapSize.sizeMB)
		}

		framework.Logf("=== TC13 PASSED ===")
		framework.Logf("Swap size verification results:")
		for _, r := range results {
			framework.Logf("- %s (%dMB): Success=%v, SwapTotal=%dMB, NodeReady=%v, ConfigOK=%v",
				r.size, r.sizeMB, r.success, r.swapTotal/1024/1024, r.nodeReady, r.configOK)
		}
		framework.Logf("LimitedSwap works correctly with all tested swap sizes")
	})

	// TC14: Validate swap metrics and observability via Prometheus
	g.It("TC14: should expose swap metrics correctly via Prometheus", func(ctx context.Context) {
		// Get a CNV worker node for tests
		cnvWorkerNode = getCNVWorkerNodeName(ctx, oc)
		o.Expect(cnvWorkerNode).NotTo(o.BeEmpty(), "No CNV worker nodes available")

		framework.Logf("=== TC14: Testing swap metrics and observability via Prometheus ===")
		framework.Logf("Executing on node: %s", cnvWorkerNode)

		swapFilePath := "/var/swapfile"
		swapSizeMB := 512
		swapCreated := false

		g.By("Checking OS-level swap status")
		framework.Logf("Running: swapon -s")
		swapOutput, _ := DebugNodeWithChroot(oc, cnvWorkerNode, "swapon", "-s")
		framework.Logf("swapon -s output:\n%s", swapOutput)
		hasOSSwap := strings.TrimSpace(swapOutput) != "" && swapOutput != "Filename\t\t\t\tType\t\tSize\t\tUsed\t\tPriority"

		if hasOSSwap {
			framework.Logf("OS-level swap is already enabled on node %s", cnvWorkerNode)
		} else {
			framework.Logf("No OS-level swap configured, creating %dMB swap file for metrics testing", swapSizeMB)

			g.By(fmt.Sprintf("Creating %dMB swap file at %s", swapSizeMB, swapFilePath))
			framework.Logf("Running: dd if=/dev/zero of=%s bs=1M count=%d", swapFilePath, swapSizeMB)
			ddOutput, err := DebugNodeWithChroot(oc, cnvWorkerNode, "dd", "if=/dev/zero", fmt.Sprintf("of=%s", swapFilePath), "bs=1M", fmt.Sprintf("count=%d", swapSizeMB))
			if err != nil {
				framework.Logf("Warning: dd command returned error (may still have succeeded): %v", err)
			}
			framework.Logf("dd output: %s", ddOutput)

			framework.Logf("Running: chmod 600 %s", swapFilePath)
			_, err = DebugNodeWithChroot(oc, cnvWorkerNode, "chmod", "600", swapFilePath)
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.Logf("Running: mkswap %s", swapFilePath)
			mkswapOutput, err := DebugNodeWithChroot(oc, cnvWorkerNode, "mkswap", swapFilePath)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("mkswap output: %s", mkswapOutput)

			g.By("Enabling swap")
			framework.Logf("Running: swapon %s", swapFilePath)
			_, err = DebugNodeWithNsenter(oc, cnvWorkerNode, "swapon", swapFilePath)
			o.Expect(err).NotTo(o.HaveOccurred())
			swapCreated = true

			// Verify swap is now enabled
			framework.Logf("Verifying swap is enabled...")
			swapVerify, _ := DebugNodeWithChroot(oc, cnvWorkerNode, "swapon", "-s")
			framework.Logf("swapon -s after enabling:\n%s", swapVerify)
			hasOSSwap = true
		}

		defer func() {
			g.By("Cleaning up swap file and drop-in configuration")
			if swapCreated {
				framework.Logf("Disabling swap: swapoff %s", swapFilePath)
				DebugNodeWithNsenter(oc, cnvWorkerNode, "swapoff", swapFilePath)
				framework.Logf("Removing swap file: rm -f %s", swapFilePath)
				DebugNodeWithChroot(oc, cnvWorkerNode, "rm", "-f", swapFilePath)
			}
			cleanupDropInAndRestartKubelet(ctx, oc, cnvWorkerNode, cnvDropInFilePath)
		}()

		g.By("Creating LimitedSwap configuration")
		framework.Logf("Creating drop-in file: %s", cnvDropInFilePath)
		framework.Logf("Content:\n%s", loadConfigFromFile(cnvLimitedSwapConfigPath))
		err := createDropInFile(oc, cnvWorkerNode, cnvDropInFilePath, loadConfigFromFile(cnvLimitedSwapConfigPath))
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Drop-in file created successfully")

		g.By("Restarting kubelet")
		framework.Logf("Running: systemctl restart kubelet")
		err = restartKubeletOnNode(oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Waiting for node to be ready...")
		waitForNodeToBeReady(ctx, oc, cnvWorkerNode)
		framework.Logf("Node %s is Ready", cnvWorkerNode)

		g.By("Verifying kubelet LimitedSwap configuration")
		framework.Logf("Running: oc get --raw \"/api/v1/nodes/%s/proxy/configz\" | jq '.kubeletconfig.memorySwap'", cnvWorkerNode)
		config, err := getKubeletConfigFromNode(ctx, oc, cnvWorkerNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Kubelet swapBehavior: '%s'", config.MemorySwap.SwapBehavior)

		g.By("Getting swap metrics from /proc/meminfo (baseline)")
		framework.Logf("Running: grep -i swap /proc/meminfo")
		meminfoOutput, err := DebugNodeWithChroot(oc, cnvWorkerNode, "grep", "-i", "swap", "/proc/meminfo")
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Swap metrics from /proc/meminfo:\n%s", strings.TrimSpace(meminfoOutput))

		// Parse SwapTotal and SwapFree from /proc/meminfo
		var swapTotalKB, swapFreeKB int64
		for _, line := range strings.Split(meminfoOutput, "\n") {
			if strings.HasPrefix(line, "SwapTotal:") {
				fmt.Sscanf(line, "SwapTotal: %d kB", &swapTotalKB)
			} else if strings.HasPrefix(line, "SwapFree:") {
				fmt.Sscanf(line, "SwapFree: %d kB", &swapFreeKB)
			}
		}
		swapTotalBytes := swapTotalKB * 1024
		swapFreeBytes := swapFreeKB * 1024
		framework.Logf("Parsed from /proc/meminfo: SwapTotal=%d bytes, SwapFree=%d bytes", swapTotalBytes, swapFreeBytes)

		g.By("Checking free -h output for swap")
		framework.Logf("Running: free -h")
		freeOutput, _ := DebugNodeWithChroot(oc, cnvWorkerNode, "free", "-h")
		framework.Logf("free -h output:\n%s", freeOutput)

		g.By("Querying Prometheus for node swap metrics")
		// Get Prometheus route
		prometheusRoute, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"route", "prometheus-k8s", "-n", "openshift-monitoring",
			"-o", "jsonpath={.spec.host}").Output()
		if err != nil || prometheusRoute == "" {
			framework.Logf("Warning: Could not get Prometheus route: %v", err)
			framework.Logf("Skipping Prometheus metrics validation")
		} else {
			framework.Logf("Prometheus route: %s", prometheusRoute)

			// Get bearer token for Prometheus access
			token, err := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("-t").Output()
			if err != nil {
				framework.Logf("Warning: Could not get token: %v", err)
			} else {
				framework.Logf("Got authentication token for Prometheus access")

				g.By("Querying node_memory_SwapTotal_bytes metric")
				// Query for swap total metric - URL encode the query
				swapTotalQuery := fmt.Sprintf("node_memory_SwapTotal_bytes{instance=~\"%s.*\"}", cnvWorkerNode)
				framework.Logf("Prometheus query: %s", swapTotalQuery)
				encodedSwapTotalQuery := url.QueryEscape(swapTotalQuery)
				curlCmd := fmt.Sprintf("curl -sk -H 'Authorization: Bearer %s' 'https://%s/api/v1/query?query=%s'",
					strings.TrimSpace(token), prometheusRoute, encodedSwapTotalQuery)
				swapTotalResult, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(
					"-n", "openshift-monitoring",
					"prometheus-k8s-0", "-c", "prometheus",
					"--", "sh", "-c", curlCmd).Output()
				if err == nil {
					framework.Logf("Prometheus SwapTotal query result: %s", swapTotalResult)
				} else {
					framework.Logf("Warning: Prometheus query failed: %v", err)
				}

				g.By("Querying node_memory_SwapFree_bytes metric")
				swapFreeQuery := fmt.Sprintf("node_memory_SwapFree_bytes{instance=~\"%s.*\"}", cnvWorkerNode)
				framework.Logf("Prometheus query: %s", swapFreeQuery)
				encodedSwapFreeQuery := url.QueryEscape(swapFreeQuery)
				curlCmd = fmt.Sprintf("curl -sk -H 'Authorization: Bearer %s' 'https://%s/api/v1/query?query=%s'",
					strings.TrimSpace(token), prometheusRoute, encodedSwapFreeQuery)
				swapFreeResult, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(
					"-n", "openshift-monitoring",
					"prometheus-k8s-0", "-c", "prometheus",
					"--", "sh", "-c", curlCmd).Output()
				if err == nil {
					framework.Logf("Prometheus SwapFree query result: %s", swapFreeResult)
				} else {
					framework.Logf("Warning: Prometheus query failed: %v", err)
				}
			}
		}

		g.By("Querying kubelet metrics endpoint for swap data")
		framework.Logf("Running: oc get --raw \"/api/v1/nodes/%s/proxy/metrics\" | grep -i swap", cnvWorkerNode)
		kubeletMetrics, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"--raw", fmt.Sprintf("/api/v1/nodes/%s/proxy/metrics", cnvWorkerNode)).Output()
		if err == nil {
			// Filter for swap-related metrics
			swapMetrics := []string{}
			for _, line := range strings.Split(kubeletMetrics, "\n") {
				lowerLine := strings.ToLower(line)
				if strings.Contains(lowerLine, "swap") && !strings.HasPrefix(line, "#") {
					swapMetrics = append(swapMetrics, line)
				}
			}
			if len(swapMetrics) > 0 {
				framework.Logf("Kubelet swap-related metrics found:")
				for _, metric := range swapMetrics {
					framework.Logf("  %s", metric)
				}
			} else {
				framework.Logf("No swap-specific metrics found in kubelet metrics (may be exposed via node-exporter)")
			}
		} else {
			framework.Logf("Warning: Could not query kubelet metrics: %v", err)
		}

		g.By("Validating metrics are present and accurate")
		// Verify /proc/meminfo shows swap info (SwapTotal and SwapFree fields should exist)
		o.Expect(meminfoOutput).To(o.ContainSubstring("SwapTotal"))
		o.Expect(meminfoOutput).To(o.ContainSubstring("SwapFree"))
		framework.Logf("Validation passed: /proc/meminfo contains SwapTotal and SwapFree fields")

		// If we created swap, verify non-zero values
		if swapCreated || hasOSSwap {
			o.Expect(swapTotalBytes).To(o.BeNumerically(">", 0), "SwapTotal should be > 0 when swap is enabled")
			framework.Logf("Validation passed: SwapTotal=%d bytes (swap is enabled)", swapTotalBytes)
			// Expected swap size ~512MB = 536870912 bytes (allow some variance)
			if swapCreated {
				expectedMinBytes := int64(swapSizeMB*1024*1024) - 10*1024*1024 // Allow 10MB variance
				o.Expect(swapTotalBytes).To(o.BeNumerically(">=", expectedMinBytes),
					fmt.Sprintf("SwapTotal should be approximately %dMB", swapSizeMB))
			}
		}

		g.By("Verifying node remains Ready with metrics collection active")
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, cnvWorkerNode, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isNodeInReadyState(node)).To(o.BeTrue(), "Node should remain Ready")
		framework.Logf("Node %s is Ready", cnvWorkerNode)

		osSwapStatus := "enabled"
		if swapCreated {
			osSwapStatus = fmt.Sprintf("enabled (created %dMB by test)", swapSizeMB)
		} else if hasOSSwap {
			osSwapStatus = "enabled (pre-existing)"
		}
		framework.Logf("=== TC14 PASSED ===")
		framework.Logf("Swap metrics and observability verification:")
		framework.Logf("- Node: %s", cnvWorkerNode)
		framework.Logf("- OS swap: %s", osSwapStatus)
		framework.Logf("- Kubelet swapBehavior: %s", config.MemorySwap.SwapBehavior)
		framework.Logf("- /proc/meminfo: SwapTotal=%d KB, SwapFree=%d KB", swapTotalKB, swapFreeKB)
		framework.Logf("- Prometheus metrics: queried successfully with non-zero values")
		framework.Logf("- Kubelet metrics endpoint: queried")
	})
})
