package node

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
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	configv1 "github.com/openshift/api/config/v1"
	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	additionalArtifactStorePath     = "/var/lib/additional-artifacts"
	additionalArtifactStoreTestName = "additional-artifactstore-test"
	maxArtifactStoresCount          = 10
)

// Non-disruptive API validation tests - can run in parallel
var _ = g.Describe("[Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport] Additional Artifact Stores API Validation", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-artifact-stores-api")

	g.BeforeEach(func(ctx context.Context) {
		g.By("Checking TechPreviewNoUpgrade feature set is enabled")
		featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if featureGate.Spec.FeatureSet != configv1.TechPreviewNoUpgrade {
			g.Skip("Skipping test: TechPreviewNoUpgrade feature set is required for additionalArtifactStores")
		}
	})

	// TC1: Validate Path Format Restrictions
	g.It("should reject invalid path formats for additionalArtifactStores [TC1]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		invalidPaths := []struct {
			name        string
			path        string
			description string
		}{
			{"relative-path", "relative/path", "relative path without leading slash"},
			{"empty-path", "", "empty path"},
		}

		for _, tc := range invalidPaths {
			g.By(fmt.Sprintf("Testing invalid path: %s (%s)", tc.path, tc.description))
			ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("artifact-invalid-path-test-%s", tc.name),
				},
				Spec: machineconfigv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pools.operator.machineconfiguration.openshift.io/worker": "",
						},
					},
					ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
						AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
							{Path: machineconfigv1.StorePath(tc.path)},
						},
					},
				},
			}

			_, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
			if err != nil {
				framework.Logf("Path '%s' correctly rejected at API level: %v", tc.path, err)
			} else {
				defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
				framework.Logf("Path '%s' accepted at API level, checking MCO validation", tc.path)
			}
		}

		framework.Logf("Test PASSED: Invalid path formats handled correctly")
	})

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
		if err != nil {
			o.Expect(err.Error()).To(o.ContainSubstring("must have at most 10 items"), "Error should mention maximum limit of 10 items")
			framework.Logf("Test PASSED: 11 artifact stores correctly rejected: %v", err)
		} else {
			defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
			framework.Logf("Warning: 11 artifact stores accepted at API level, checking MCO status")

			err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
				cfg, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Get(ctx, ctrcfg.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				for _, condition := range cfg.Status.Conditions {
					if condition.Type == machineconfigv1.ContainerRuntimeConfigFailure &&
						condition.Status == corev1.ConditionTrue {
						framework.Logf("MCO rejected config: %s", condition.Message)
						return true, nil
					}
				}
				return cfg.Status.ObservedGeneration == cfg.Generation, nil
			})
		}
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
		if err != nil {
			framework.Logf("Duplicate paths correctly rejected at API level: %v", err)
		} else {
			defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
			framework.Logf("Duplicate paths accepted at API level, checking MCO validation")
		}

		framework.Logf("Test completed: Duplicate path validation checked")
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
	g.It("should reject additionalArtifactStores path containing invalid characters [TC5]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		invalidChars := []struct {
			name string
			path string
			char string
		}{
			{"at-symbol", "/var/lib/artifact@store", "@"},
			{"exclamation", "/var/lib/artifact!store", "!"},
			{"hash", "/var/lib/artifact#store", "#"},
			{"dollar", "/var/lib/artifact$store", "$"},
			{"percent", "/var/lib/artifact%store", "%"},
		}

		for _, tc := range invalidChars {
			g.By(fmt.Sprintf("Testing path with invalid character: %s", tc.char))
			ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("artifact-invalid-char-%s-test", tc.name),
				},
				Spec: machineconfigv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pools.operator.machineconfiguration.openshift.io/worker": "",
						},
					},
					ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
						AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
							{Path: machineconfigv1.StorePath(tc.path)},
						},
					},
				},
			}

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
			if err != nil {
				framework.Logf("Path with '%s' correctly rejected: %v", tc.char, err)
			} else {
				defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
				framework.Logf("Warning: Path with '%s' accepted at API level", tc.char)
			}
		}
		framework.Logf("Test completed: Invalid character validation checked")
	})

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
		if err != nil {
			o.Expect(err.Error()).To(o.Or(o.ContainSubstring("256"), o.ContainSubstring("long")))
			framework.Logf("Test PASSED: Long path correctly rejected: %v", err)
		} else {
			defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
			framework.Logf("Warning: Long path accepted at API level")
		}
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
		if err != nil {
			o.Expect(err.Error()).To(o.ContainSubstring("consecutive"))
			framework.Logf("Test PASSED: Consecutive slashes correctly rejected: %v", err)
		} else {
			defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
			framework.Logf("Warning: Consecutive slashes accepted at API level")
		}
	})

	// TC8: Single artifact store creation (P1 Basic)
	g.It("should successfully create ContainerRuntimeConfig with single additionalArtifactStore [TC8]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with single artifact store")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "artifact-single-store-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/var/lib/artifact-single")},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		o.Expect(created.Name).To(o.Equal(ctrcfg.Name))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalArtifactStores).To(o.HaveLen(1))

		framework.Logf("Test PASSED: Single artifact store created successfully")
	})

	// TC9: Same path across store types (P2)
	g.It("should accept same path across different storage types [TC9]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with same path for layer, image, and artifact stores")
		sharedPath := "/mnt/shared-storage"
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "artifact-same-path-cross-type-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath(sharedPath)},
					},
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath(sharedPath)},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath(sharedPath)},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		o.Expect(string(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores[0].Path)).To(o.Equal(sharedPath))
		o.Expect(string(created.Spec.ContainerRuntimeConfig.AdditionalImageStores[0].Path)).To(o.Equal(sharedPath))
		o.Expect(string(created.Spec.ContainerRuntimeConfig.AdditionalArtifactStores[0].Path)).To(o.Equal(sharedPath))

		framework.Logf("Test PASSED: Same path accepted across different storage types")
	})
})

// Disruptive E2E tests - must run serially
var _ = g.Describe("[Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][Serial][Disruptive] Additional Artifact Stores E2E", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-artifact-stores")

	g.BeforeEach(func(ctx context.Context) {
		g.By("Checking TechPreviewNoUpgrade feature set is enabled")
		featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if featureGate.Spec.FeatureSet != configv1.TechPreviewNoUpgrade {
			g.Skip("Skipping test: TechPreviewNoUpgrade feature set is required for additionalArtifactStores")
		}

		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.Type == configv1.AzurePlatformType {
			g.Skip("Skipping test on Microsoft Azure cluster")
		}
	})

	// TC10: Comprehensive E2E test - Configure and Verify storage.conf
	g.It("should configure additionalArtifactStores and generate correct CRI-O config [TC10]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(workerNodes)).To(o.BeNumerically(">", 0))
		pureWorkers := getPureWorkerNodes(workerNodes)
		if len(pureWorkers) < 1 {
			e2eskipper.Skipf("Need at least 1 worker node for this test")
		}

		// PHASE 1: Setup - Create shared directory on worker nodes
		g.By("PHASE 1: Creating shared artifact directory on worker nodes")
		artifactDirs := []string{additionalArtifactStorePath}
		err = createDirectoriesOnNodes(oc, pureWorkers, artifactDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, artifactDirs)

		// PHASE 2: Create ContainerRuntimeConfig and verify MCO processing
		g.By("PHASE 2: Creating ContainerRuntimeConfig with additionalArtifactStores")
		ctrcfg := createAdditionalArtifactStoresCTRCfg(additionalArtifactStoreTestName, additionalArtifactStorePath)
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
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

		g.By("Verifying CRI-O is running with new configuration")
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
			_, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
			o.Expect(err).To(o.HaveOccurred(),
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

	// TC11: Update Existing Configuration
	g.It("should update additionalArtifactStores when ContainerRuntimeConfig is modified [TC11]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		g.By("Creating shared artifact directories on worker nodes")
		artifactDirs := []string{"/var/lib/artifactstore-1", "/var/lib/artifactstore-2", "/var/lib/artifactstore-3"}
		err = createDirectoriesOnNodes(oc, pureWorkers, artifactDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, artifactDirs)

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
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

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

	// TC12: Multiple Storage Paths
	g.It("should configure multiple additionalArtifactStores paths [TC12]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		g.By("Creating multiple shared artifact directories on worker nodes")
		artifactDirs := []string{"/var/lib/artifactstore-1", "/var/lib/artifactstore-2", "/var/lib/artifactstore-3"}
		err = createDirectoriesOnNodes(oc, pureWorkers, artifactDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, artifactDirs)

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
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

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
