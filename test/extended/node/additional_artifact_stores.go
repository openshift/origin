package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	additionalArtifactStorePath     = "/var/lib/additional-artifacts"
	additionalArtifactStoreTestName = "additional-artifactstore-test"
	maxArtifactStoresCount          = 10
)

// API validation tests - creating CRCs triggers MCO reconciliation making these disruptive
var _ = g.Describe("[apigroup:config.openshift.io][apigroup:machineconfiguration.openshift.io][Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][OCPFeatureGate:AdditionalStorageConfig][Suite:openshift/disruptive-longrunning] Additional Artifact Stores API Validation", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-artifact-stores-api")

	g.BeforeEach(func(ctx context.Context) {
		skipUnlessAdditionalStorageConfigEnabled(ctx, oc)
	})

	// TC1: Validate Path Format Restrictions
	g.DescribeTable("should reject invalid path formats for additionalArtifactStores [TC1]",
		func(ctx context.Context, testPath, testName, description string) {
			mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
			o.Expect(err).NotTo(o.HaveOccurred())

			ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("artifact-invalid-path-test-%s", testName),
				},
				Spec: machineconfigv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pools.operator.machineconfiguration.openshift.io/worker": "",
						},
					},
					ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
						AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
							{Path: machineconfigv1.StorePath(testPath)},
						},
					},
				},
			}

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
			o.Expect(err).To(o.HaveOccurred(), "Expected API to reject invalid path: %s", description)
			framework.Logf("Path '%s' correctly rejected: %v", testPath, err)
		},
		g.Entry("relative path without leading slash", "relative/path", "relative-path", "relative path without leading slash"),
		g.Entry("empty path", "", "empty-path", "empty path"),
	)

	// TC2: Validate Count Limits (max 10 artifact stores)
	g.It("should reject more than 10 additionalArtifactStores [TC2]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with 11 artifact stores (exceeds max of 10)")
		artifactStores := make([]machineconfigv1.AdditionalArtifactStore, 11)
		for i := 0; i < 11; i++ {
			artifactStores[i] = machineconfigv1.AdditionalArtifactStore{Path: machineconfigv1.StorePath(fmt.Sprintf("/mnt/artifactstore-%d", i))}
		}

		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "artifact-exceed-limit-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalArtifactStores: artifactStores,
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred(), "Expected API to reject exceeding maximum of 10 artifact stores")
		o.Expect(err.Error()).To(o.ContainSubstring("must have at most"), "Error should mention maximum limit")
		framework.Logf("Test PASSED: 11 artifact stores correctly rejected: %v", err)
	})

	// TC3: Validate Path Uniqueness Within Store Type
	g.It("should reject duplicate paths in additionalArtifactStores [TC3]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with duplicate paths")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "artifact-duplicate-path-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/mnt/shared-artifacts")},
						{Path: machineconfigv1.StorePath("/mnt/shared-artifacts")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred(), "Expected API to reject duplicate paths in additionalArtifactStores")
		framework.Logf("Test PASSED: Duplicate paths correctly rejected: %v", err)
	})

	// TC4: Path contains spaces
	g.It("should reject additionalArtifactStores path containing spaces [TC4]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with path containing spaces")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "artifact-path-spaces-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/var/lib/artifact store")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("alphanumeric"))
		framework.Logf("Test PASSED: Path with spaces correctly rejected: %v", err)
	})

	// TC5: Path contains invalid characters
	g.DescribeTable("should reject additionalArtifactStores path containing invalid characters [TC5]",
		func(ctx context.Context, testPath, testName, invalidChar string) {
			mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
			o.Expect(err).NotTo(o.HaveOccurred())

			ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("artifact-invalid-char-%s-test", testName),
				},
				Spec: machineconfigv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pools.operator.machineconfiguration.openshift.io/worker": "",
						},
					},
					ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
						AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
							{Path: machineconfigv1.StorePath(testPath)},
						},
					},
				},
			}

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
			o.Expect(err).To(o.HaveOccurred(), "Expected API to reject path with invalid character '%s'", invalidChar)
			framework.Logf("Path with '%s' correctly rejected: %v", invalidChar, err)
		},
		g.Entry("path with @ symbol", "/var/lib/artifact@store", "at-symbol", "@"),
		g.Entry("path with ! exclamation", "/var/lib/artifact!store", "exclamation", "!"),
		g.Entry("path with # hash", "/var/lib/artifact#store", "hash", "#"),
		g.Entry("path with $ dollar", "/var/lib/artifact$store", "dollar", "$"),
		g.Entry("path with % percent", "/var/lib/artifact%store", "percent", "%"),
	)

	// TC6: Path too long (>256 bytes)
	g.It("should reject additionalArtifactStores path exceeding 256 characters [TC6]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		longPath := "/" + strings.Repeat("a", 256)
		g.By(fmt.Sprintf("Creating ContainerRuntimeConfig with path of %d characters", len(longPath)))

		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "artifact-long-path-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath(longPath)},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred(), "Expected API to reject path exceeding 256 characters")
		o.Expect(err.Error()).To(o.Or(o.ContainSubstring("256"), o.ContainSubstring("long")), "Error should mention path length limit")
		framework.Logf("Test PASSED: Long path correctly rejected: %v", err)
	})

	// TC7: Path contains consecutive forward slashes
	g.It("should reject additionalArtifactStores path with consecutive forward slashes [TC7]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with consecutive forward slashes")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "artifact-consecutive-slashes-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/var/lib//artifacts")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred(), "Expected API to reject path with consecutive forward slashes")
		o.Expect(err.Error()).To(o.ContainSubstring("consecutive"), "Error should mention consecutive slashes")
		framework.Logf("Test PASSED: Consecutive slashes correctly rejected: %v", err)
	})
})

// Disruptive E2E tests - must run serially
var _ = g.Describe("[Skipped:Disconnected][apigroup:config.openshift.io][apigroup:machineconfiguration.openshift.io][Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][OCPFeatureGate:AdditionalStorageConfig][Serial][Disruptive][Suite:openshift/disruptive-longrunning] Additional Artifact Stores E2E", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-artifact-stores")

	g.BeforeEach(func(ctx context.Context) {
		skipUnlessAdditionalStorageConfigEnabled(ctx, oc)
	})

	// TC8: Comprehensive E2E test - Configure and Verify storage.conf
	g.It("should configure additionalArtifactStores and generate correct CRI-O config [TC8]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(workerNodes)).To(o.BeNumerically(">", 0))
		pureWorkers := getPureWorkerNodes(workerNodes)
		// Use pureWorkers if available, otherwise use any worker node (SNO support)
		if len(pureWorkers) == 0 {
			pureWorkers = workerNodes
		}

		// PHASE 1: Setup - Create shared directory on worker nodes
		g.By("PHASE 1: Creating shared artifact directory on worker nodes")
		artifactDirs := []string{additionalArtifactStorePath}
		err = createDirectoriesOnNodes(oc, pureWorkers, artifactDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, pureWorkers, artifactDirs)

		// PHASE 2: Create ContainerRuntimeConfig and verify MCO processing
		g.By("PHASE 2: Creating ContainerRuntimeConfig with additionalArtifactStores")
		ctrcfg := createAdditionalArtifactStoresCTRCfg(additionalArtifactStoreTestName, additionalArtifactStorePath)
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)
		framework.Logf("ContainerRuntimeConfig %s created", ctrcfg.Name)

		g.By("Verifying ContainerRuntimeConfig resource created")
		createdCfg, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Get(ctx, ctrcfg.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(createdCfg.Name).To(o.Equal(ctrcfg.Name))
		o.Expect(createdCfg.Spec.ContainerRuntimeConfig.AdditionalArtifactStores).To(o.HaveLen(1))
		framework.Logf("ContainerRuntimeConfig resource verified")

		g.By("Waiting for ContainerRuntimeConfig to be processed by MCO")
		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("ContainerRuntimeConfig processed by MCO")

		g.By("Verifying MachineConfig generated from ContainerRuntimeConfig")
		mcList, err := mcClient.MachineconfigurationV1().MachineConfigs().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		foundCtrcfgMC := false
		for _, mc := range mcList.Items {
			if strings.Contains(mc.Name, "containerruntime") || strings.Contains(mc.Name, "ctrcfg") {
				framework.Logf("Found generated MachineConfig: %s", mc.Name)
				foundCtrcfgMC = true
				break
			}
		}
		o.Expect(foundCtrcfgMC).To(o.BeTrue(), "Should find MachineConfig generated from ContainerRuntimeConfig")

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating")

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool rollout completed")

		// PHASE 3: Verify CRI-O config on all nodes
		g.By("PHASE 3: Verifying CRI-O config contains additionalArtifactStores on all worker nodes")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
			o.Expect(err).NotTo(o.HaveOccurred())

			// Verify exact format: additional_artifact_stores = ["/var/lib/additional-artifacts"]
			expectedConfig := fmt.Sprintf(`additional_artifact_stores = ["%s"]`, additionalArtifactStorePath)
			o.Expect(output).To(o.ContainSubstring(expectedConfig),
				"CRI-O config should contain '%s' on node %s", expectedConfig, node.Name)
			framework.Logf("Node %s: CRI-O config verified with additionalArtifactStores = [\"%s\"]", node.Name, additionalArtifactStorePath)
		}

		// PHASE 4: Verify CRI-O and node status
		g.By("PHASE 4: Verifying CRI-O is running with new configuration")
		for _, node := range pureWorkers {
			crioStatus, err := ExecOnNodeWithChroot(oc, node.Name, "systemctl", "is-active", "crio")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(crioStatus)).To(o.Equal("active"),
				"CRI-O should be active on node %s", node.Name)
			framework.Logf("Node %s: CRI-O is active", node.Name)
		}

		g.By("Verifying nodes are Ready")
		for _, node := range pureWorkers {
			nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, node.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(isNodeInReadyState(nodeObj)).To(o.BeTrue(),
				"Node %s should be Ready", node.Name)
			framework.Logf("Node %s: Ready", node.Name)
		}

		// =====================================================================
		// PHASE 5: Delete ContainerRuntimeConfig and verify cleanup
		// =====================================================================
		g.By("PHASE 5: Deleting ContainerRuntimeConfig")
		err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Delete(ctx, ctrcfg.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("ContainerRuntimeConfig %s deleted", ctrcfg.Name)

		g.By("Waiting for MachineConfigPool to start updating after deletion")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating after CRC deletion")

		g.By("Waiting for MachineConfigPool rollout to complete after deletion")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool rollout completed after deletion")

		// =====================================================================
		// PHASE 6: Verify CRI-O config cleanup
		// =====================================================================
		g.By("PHASE 6: Verifying CRI-O config file is removed after CRC deletion")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "sh", "-c", "if [ -e /etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores ]; then echo present; fi")
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to check file presence on node %s", node.Name)
			o.Expect(strings.TrimSpace(output)).To(o.BeEmpty(),
				"CRI-O config file should be removed after ContainerRuntimeConfig deletion on node %s", node.Name)
			framework.Logf("Node %s: CRI-O config file removed successfully", node.Name)
		}

		// Final Summary
		framework.Logf("========================================")
		framework.Logf("TEST RESULTS SUMMARY")
		framework.Logf("========================================")
		framework.Logf("Phase 1: Directory creation - PASSED")
		framework.Logf("Phase 2: ContainerRuntimeConfig creation - PASSED")
		framework.Logf("Phase 3: CRI-O config verification - PASSED")
		framework.Logf("Phase 4: CRI-O and node status - PASSED")
		framework.Logf("Phase 5: ContainerRuntimeConfig deletion - PASSED")
		framework.Logf("Phase 6: CRI-O config cleanup - PASSED")
		framework.Logf("========================================")
		framework.Logf("Test PASSED: Comprehensive additionalArtifactStores E2E lifecycle verification complete")
	})

	// TC9: Update Existing Configuration
	g.It("should update additionalArtifactStores when ContainerRuntimeConfig is modified [TC9]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		// Use pureWorkers if available, otherwise use any worker node (SNO support)
		if len(pureWorkers) == 0 {
			pureWorkers = workerNodes
		}

		g.By("Creating shared artifact directories on worker nodes")
		artifactDirs := []string{"/var/lib/artifactstore-1", "/var/lib/artifactstore-2", "/var/lib/artifactstore-3"}
		err = createDirectoriesOnNodes(oc, pureWorkers, artifactDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, pureWorkers, artifactDirs)

		g.By("Creating initial ContainerRuntimeConfig with one artifact store")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "artifact-update-test-ctrcfg",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/var/lib/artifactstore-1")},
					},
				},
			},
		}
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying initial configuration in CRI-O config")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
			o.Expect(err).NotTo(o.HaveOccurred())
			expectedConfig := `additional_artifact_stores = ["/var/lib/artifactstore-1"]`
			o.Expect(output).To(o.ContainSubstring(expectedConfig),
				"CRI-O config should contain '%s' on node %s", expectedConfig, node.Name)
		}

		g.By("Updating ContainerRuntimeConfig to add second artifact store")
		currentCfg, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Get(ctx, ctrcfg.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		currentCfg.Spec.ContainerRuntimeConfig.AdditionalArtifactStores = []machineconfigv1.AdditionalArtifactStore{
			{Path: machineconfigv1.StorePath("/var/lib/artifactstore-1")},
			{Path: machineconfigv1.StorePath("/var/lib/artifactstore-2")},
		}
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Update(ctx, currentCfg, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying updated configuration includes both stores in CRI-O config")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("\"/var/lib/artifactstore-1\""),
				"CRI-O config should contain /var/lib/artifactstore-1 on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring("\"/var/lib/artifactstore-2\""),
				"CRI-O config should contain /var/lib/artifactstore-2 on node %s", node.Name)
			framework.Logf("Node %s: Both artifact stores configured after update", node.Name)
		}

		framework.Logf("Test PASSED: ContainerRuntimeConfig update applied successfully")
	})

	// TC10: Multiple Storage Paths
	g.It("should configure multiple additionalArtifactStores paths [TC10]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		// Use pureWorkers if available, otherwise use any worker node (SNO support)
		if len(pureWorkers) == 0 {
			pureWorkers = workerNodes
		}

		g.By("Creating multiple shared artifact directories on worker nodes")
		artifactDirs := []string{"/var/lib/artifactstore-1", "/var/lib/artifactstore-2", "/var/lib/artifactstore-3"}
		err = createDirectoriesOnNodes(oc, pureWorkers, artifactDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, pureWorkers, artifactDirs)

		g.By("Creating ContainerRuntimeConfig with multiple artifact stores")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "multi-artifactstore-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/var/lib/artifactstore-1")},
						{Path: machineconfigv1.StorePath("/var/lib/artifactstore-2")},
						{Path: machineconfigv1.StorePath("/var/lib/artifactstore-3")},
					},
				},
			},
		}
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying all artifact stores configured in CRI-O config on nodes")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
			o.Expect(err).NotTo(o.HaveOccurred())

			// Verify all 3 paths are in the array
			o.Expect(output).To(o.ContainSubstring("\"/var/lib/artifactstore-1\""),
				"CRI-O config should contain /var/lib/artifactstore-1 on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring("\"/var/lib/artifactstore-2\""),
				"CRI-O config should contain /var/lib/artifactstore-2 on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring("\"/var/lib/artifactstore-3\""),
				"CRI-O config should contain /var/lib/artifactstore-3 on node %s", node.Name)
			framework.Logf("Node %s: All 3 artifact stores configured", node.Name)
		}

		framework.Logf("Test PASSED: Multiple additionalArtifactStores configured successfully")
	})

})

func createAdditionalArtifactStoresCTRCfg(testName, storePath string) *machineconfigv1.ContainerRuntimeConfig {
	return &machineconfigv1.ContainerRuntimeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
		Spec: machineconfigv1.ContainerRuntimeConfigSpec{
			MachineConfigPoolSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"pools.operator.machineconfiguration.openshift.io/worker": "",
				},
			},
			ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
				AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
					{Path: machineconfigv1.StorePath(storePath)},
				},
			},
		},
	}
}
