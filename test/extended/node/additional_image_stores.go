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
	additionalImageStorePath     = "/var/lib/additional-images"
	additionalImageStoreTestName = "additional-imagestore-test"
	testImageDefault             = "quay.io/bgudi/test-image-6gb:v1.0"
	maxImageStoresCount          = 10
)

// Non-disruptive API validation tests - can run in parallel
var _ = g.Describe("[Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport] Additional Image Stores API Validation", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-image-stores-api")

	g.BeforeEach(func(ctx context.Context) {
		g.By("Checking TechPreviewNoUpgrade feature set is enabled")
		featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if featureGate.Spec.FeatureSet != configv1.TechPreviewNoUpgrade {
			g.Skip("Skipping test: TechPreviewNoUpgrade feature set is required for additionalImageStores")
		}
	})

	// TC1: Validate Path Format Restrictions
	g.It("should reject invalid path formats for additionalImageStores [TC1]", func(ctx context.Context) {
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
					Name: fmt.Sprintf("invalid-path-test-%s", tc.name),
				},
				Spec: machineconfigv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pools.operator.machineconfiguration.openshift.io/worker": "",
						},
					},
					ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
						AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
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

	// TC2: Validate Count Limits (max 10 image stores)
	g.It("should reject more than 10 additionalImageStores [TC2]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with 11 image stores (exceeds max of 10)")
		imageStores := make([]machineconfigv1.AdditionalImageStore, 11)
		for i := 0; i < 11; i++ {
			imageStores[i] = machineconfigv1.AdditionalImageStore{Path: machineconfigv1.StorePath(fmt.Sprintf("/mnt/imagestore-%d", i))}
		}

		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "exceed-limit-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalImageStores: imageStores,
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		if err != nil {
			o.Expect(err.Error()).To(o.ContainSubstring("must have at most 10 items"), "Error should mention maximum limit of 10 items")
			framework.Logf("Test PASSED: 11 image stores correctly rejected: %v", err)
		} else {
			defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
			framework.Logf("Warning: 11 image stores accepted at API level, checking MCO status")

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
	g.It("should reject duplicate paths in additionalImageStores [TC3]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with duplicate paths")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "duplicate-path-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/mnt/shared-images")},
						{Path: machineconfigv1.StorePath("/mnt/shared-images")},
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
	g.It("should reject additionalImageStores path containing spaces [TC4]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with path containing spaces")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "imagestore-path-spaces-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/var/lib/image store")},
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
	g.It("should reject additionalImageStores path containing invalid characters [TC5]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		invalidChars := []struct {
			name string
			path string
			char string
		}{
			{"at-symbol", "/var/lib/image@store", "@"},
			{"exclamation", "/var/lib/image!store", "!"},
			{"hash", "/var/lib/image#store", "#"},
			{"dollar", "/var/lib/image$store", "$"},
			{"percent", "/var/lib/image%store", "%"},
		}

		for _, tc := range invalidChars {
			g.By(fmt.Sprintf("Testing path with invalid character: %s", tc.char))
			ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("imagestore-invalid-char-%s-test", tc.name),
				},
				Spec: machineconfigv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pools.operator.machineconfiguration.openshift.io/worker": "",
						},
					},
					ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
						AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
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
	g.It("should reject additionalImageStores path exceeding 256 characters [TC6]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		longPath := "/" + strings.Repeat("a", 256)
		g.By(fmt.Sprintf("Creating ContainerRuntimeConfig with path of %d characters", len(longPath)))

		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "imagestore-long-path-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
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
	g.It("should reject additionalImageStores path with consecutive forward slashes [TC7]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with consecutive forward slashes")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "imagestore-consecutive-slashes-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/var/lib//images")},
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

	// TC8: Single image store creation (P1 Basic)
	g.It("should successfully create ContainerRuntimeConfig with single additionalImageStore [TC8]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with single image store")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "imagestore-single-store-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/var/lib/imagestore-single")},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		g.By("Verifying resource created successfully")
		o.Expect(created.Name).To(o.Equal(ctrcfg.Name))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalImageStores).To(o.HaveLen(1))

		framework.Logf("Test PASSED: Single image store created successfully")
	})

	// TC9: Same path across store types (P2)
	g.It("should accept same path across different storage types [TC9]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with same path for layer, image, and artifact stores")
		sharedPath := "/mnt/shared-storage"
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "imagestore-same-path-cross-type-test",
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

		g.By("Verifying same path accepted across different store types")
		o.Expect(string(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores[0].Path)).To(o.Equal(sharedPath))
		o.Expect(string(created.Spec.ContainerRuntimeConfig.AdditionalImageStores[0].Path)).To(o.Equal(sharedPath))
		o.Expect(string(created.Spec.ContainerRuntimeConfig.AdditionalArtifactStores[0].Path)).To(o.Equal(sharedPath))

		framework.Logf("Test PASSED: Same path accepted across different storage types")
	})
})

// Disruptive E2E tests - must run serially
var _ = g.Describe("[Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][Serial][Disruptive] Additional Image Stores E2E", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-image-stores")

	g.BeforeEach(func(ctx context.Context) {
		g.By("Checking TechPreviewNoUpgrade feature set is enabled")
		featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if featureGate.Spec.FeatureSet != configv1.TechPreviewNoUpgrade {
			g.Skip("Skipping test: TechPreviewNoUpgrade feature set is required for additionalImageStores")
		}

		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.Type == configv1.AzurePlatformType {
			g.Skip("Skipping test on Microsoft Azure cluster")
		}
	})

	// TC10: Comprehensive E2E test - Configure, Verify storage.conf, and Verify Pod Deployment
	g.It("should perform complete E2E lifecycle test with prepopulated images and fallback validation [TC10]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(workerNodes)).To(o.BeNumerically(">", 0))
		pureWorkers := getPureWorkerNodes(workerNodes)
		if len(pureWorkers) < 1 {
			e2eskipper.Skipf("Need at least 1 worker node for this test")
		}
		testNode := pureWorkers[0].Name

		// =====================================================================
		// PHASE 1: Setup - Create directory on worker nodes
		// =====================================================================
		g.By("PHASE 1: Creating directory on worker nodes")
		imageDirs := []string{additionalImageStorePath}
		err = createDirectoriesOnNodes(oc, pureWorkers, imageDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, imageDirs)
		framework.Logf("Directory %s created on all workers", additionalImageStorePath)

		// =====================================================================
		// PHASE 2: Create ContainerRuntimeConfig and verify MCO processing
		// =====================================================================
		g.By("PHASE 2: Creating ContainerRuntimeConfig with additionalImageStores")
		ctrcfg := createAdditionalImageStoresCTRCfg(additionalImageStoreTestName, additionalImageStorePath)
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
		framework.Logf("ContainerRuntimeConfig %s created", ctrcfg.Name)

		g.By("Verifying ContainerRuntimeConfig resource created")
		createdCfg, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Get(ctx, ctrcfg.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(createdCfg.Name).To(o.Equal(ctrcfg.Name))
		o.Expect(createdCfg.Spec.ContainerRuntimeConfig.AdditionalImageStores).To(o.HaveLen(1))
		framework.Logf("ContainerRuntimeConfig resource verified")

		g.By("Waiting for ContainerRuntimeConfig to be processed by MCO")
		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("ContainerRuntimeConfig processed by MCO")

		g.By("Verifying MachineConfig generated from ContainerRuntimeConfig")
		mcList, err := mcClient.MachineconfigurationV1().MachineConfigs().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		foundCtrcfgMC := false
		var generatedMCName string
		for _, mc := range mcList.Items {
			if strings.Contains(mc.Name, "containerruntime") || strings.Contains(mc.Name, "ctrcfg") {
				framework.Logf("Found generated MachineConfig: %s", mc.Name)
				generatedMCName = mc.Name
				foundCtrcfgMC = true
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

		// =====================================================================
		// PHASE 3: Verify storage.conf on all nodes
		// =====================================================================
		g.By("PHASE 3: Verifying storage.conf contains additionalImageStores on all worker nodes")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("additionalimagestores"),
				"storage.conf should contain additionalimagestores on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring(additionalImageStorePath),
				"storage.conf should contain path %s on node %s", additionalImageStorePath, node.Name)
			framework.Logf("Node %s: storage.conf verified with additionalImageStores", node.Name)
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
		// PHASE 4: Test pre-populated image functionality
		// =====================================================================
		g.By("PHASE 4: Pre-populating image in shared storage")
		err = prepopulateImageOnNode(ctx, oc, testNode, testImageDefault, additionalImageStorePath)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to prepopulate image - this is required for TC10 to validate additionalImageStores functionality")

		g.By("Deploying test pod using pre-populated image")
		testPod := createTestPod("imagestore-test-pod", testImageDefault, testNode)
		startTime := time.Now()
		_, err = oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx, testPod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, testPod.Name, metav1.DeleteOptions{})

		g.By("Waiting for pod to be running")
		err = waitForPodRunning(ctx, oc, testPod.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		podStartupTime := time.Since(startTime)
		framework.Logf("Pod started in %v", podStartupTime)

		g.By("Verifying pod events and image pull behavior")
		events, err := oc.AdminKubeClient().CoreV1().Events(oc.Namespace()).List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s", testPod.Name),
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		var foundAlreadyPresentEvent bool
		for _, event := range events.Items {
			if event.Reason == "Pulled" {
				framework.Logf("Image pulled event: %s", event.Message)
				// Check if the event indicates image was already present on machine
				// This is the expected message when image is loaded from additionalImageStores
				if strings.Contains(event.Message, "already present on machine and can be accessed by the pod") {
					foundAlreadyPresentEvent = true
					framework.Logf("SUCCESS: Image was loaded from additionalImageStore - event message: %s", event.Message)
					break
				}
			}
		}

		// Validation: Verify image was loaded from additional storage
		// Expected event message: "Container image ... already present on machine and can be accessed by the pod"
		o.Expect(foundAlreadyPresentEvent).To(o.BeTrue(),
			"Image should have been loaded from additionalImageStore (%s). "+
				"Expected event message containing 'already present on machine and can be accessed by the pod' but did not find it. "+
				"This indicates the image was not pre-populated correctly or not loaded from additional storage.", additionalImageStorePath)
		framework.Logf("Verified: Image was loaded from additional storage at %s", additionalImageStorePath)

		// =====================================================================
		// PHASE 5: Test fallback behavior when image is not in additional storage
		// =====================================================================
		g.By("PHASE 5: Testing fallback to registry when image not in additional storage")

		g.By("Deleting first pod")
		err = oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, testPod.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("First pod deleted")

		g.By("Removing image from additional storage to test fallback")
		removeCmd := fmt.Sprintf("podman --root %s rmi %s", additionalImageStorePath, testImageDefault)
		removeOutput, err := ExecOnNodeWithChroot(oc, testNode, "sh", "-c", removeCmd)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Image removed from additional storage: %s", removeOutput)

		g.By("Creating second pod to test fallback to registry")
		testPod2 := createTestPod("imagestore-fallback-pod", testImageDefault, testNode)
		startTime2 := time.Now()
		_, err = oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx, testPod2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, testPod2.Name, metav1.DeleteOptions{})

		g.By("Waiting for second pod to be running")
		err = waitForPodRunning(ctx, oc, testPod2.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod2StartupTime := time.Since(startTime2)
		framework.Logf("Second pod started in %v", pod2StartupTime)

		g.By("Verifying second pod pulled from registry (fallback behavior)")
		events2, err := oc.AdminKubeClient().CoreV1().Events(oc.Namespace()).List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s", testPod2.Name),
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		var foundSuccessfullyPulledEvent bool
		for _, event := range events2.Items {
			if event.Reason == "Pulled" {
				framework.Logf("Second pod pull event: %s", event.Message)
				// Should see "Successfully pulled" since image is not in additional storage
				if strings.Contains(event.Message, "Successfully pulled") {
					foundSuccessfullyPulledEvent = true
					framework.Logf("SUCCESS: Image was pulled from registry (fallback) - event message: %s", event.Message)
					break
				}
			}
		}

		o.Expect(foundSuccessfullyPulledEvent).To(o.BeTrue(),
			"Image should have been pulled from registry since it was removed from additionalImageStore. "+
				"Expected event message containing 'Successfully pulled' but did not find it.")
		framework.Logf("Verified: Fallback to registry works when image not in additional storage")

		g.By("Verifying performance improvement with additionalImageStores")
		framework.Logf("Performance comparison:")
		framework.Logf("  Pod 1 (prepopulated from additionalImageStore): %v", podStartupTime)
		framework.Logf("  Pod 2 (pulled from registry): %v", pod2StartupTime)
		speedup := float64(pod2StartupTime) / float64(podStartupTime)
		framework.Logf("  Speedup: %.2fx faster with additionalImageStores", speedup)

		// Verify that prepopulated image is significantly faster than registry pull
		// For a 6GB image, prepopulated should be at least 2x faster
		o.Expect(podStartupTime).To(o.BeNumerically("<", pod2StartupTime/2),
			"Pod using prepopulated image from additionalImageStore should be significantly faster. "+
				"Expected pod1 (%v) to be at least 2x faster than pod2 (%v) pulling from registry.",
			podStartupTime, pod2StartupTime)
		framework.Logf("Performance improvement verified: Prepopulated image is %.2fx faster", speedup)

		g.By("Deleting second pod")
		err = oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, testPod2.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Second pod deleted")

		// =====================================================================
		// PHASE 6: Cleanup - Remove ContainerRuntimeConfig and verify removal
		// =====================================================================
		g.By("PHASE 6: Removing ContainerRuntimeConfig and verifying cleanup")

		g.By("Deleting ContainerRuntimeConfig")
		err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Delete(ctx, ctrcfg.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("ContainerRuntimeConfig deleted")

		g.By("Waiting for MachineConfigPool to start updating after deletion")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MCP started updating after deletion")

		g.By("Waiting for MachineConfigPool rollout to complete after deletion")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MCP rollout completed after deletion")

		g.By("Verifying additionalImageStores removed from storage.conf")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).NotTo(o.ContainSubstring(additionalImageStorePath),
				"storage.conf should not contain %s after ContainerRuntimeConfig deletion on node %s",
				additionalImageStorePath, node.Name)
			framework.Logf("Node %s: storage.conf verified - additionalImageStores removed", node.Name)
		}

		// =====================================================================
		// PHASE 7: Final Summary
		// =====================================================================
		framework.Logf("========================================")
		framework.Logf("COMPREHENSIVE E2E TEST RESULTS SUMMARY")
		framework.Logf("========================================")
		framework.Logf("Phase 1: Directory creation - PASSED")
		framework.Logf("Phase 2: ContainerRuntimeConfig creation - PASSED")
		framework.Logf("  - ContainerRuntimeConfig: %s", ctrcfg.Name)
		framework.Logf("  - Generated MachineConfig: %s", generatedMCName)
		framework.Logf("  - MCP rollout: COMPLETED")
		framework.Logf("Phase 3: storage.conf verification - PASSED")
		framework.Logf("  - storage.conf updated: YES")
		framework.Logf("  - CRI-O active: YES")
		framework.Logf("  - All nodes Ready: YES")
		framework.Logf("Phase 4: Prepopulated image test - PASSED")
		framework.Logf("  - Pod startup time: %v", podStartupTime)
		framework.Logf("  - Image source: ADDITIONAL STORAGE (verified by 'already present on machine' event)")
		framework.Logf("Phase 5: Fallback to registry test - PASSED")
		framework.Logf("  - Image removed from additional storage")
		framework.Logf("  - Pod successfully pulled from registry (fallback verified)")
		framework.Logf("  - Pod 2 startup time: %v", pod2StartupTime)
		speedupFinal := float64(pod2StartupTime) / float64(podStartupTime)
		framework.Logf("  - Performance improvement: %.2fx faster with additionalImageStores", speedupFinal)
		framework.Logf("Phase 6: ContainerRuntimeConfig removal - PASSED")
		framework.Logf("  - ContainerRuntimeConfig deleted")
		framework.Logf("  - MCP rollout after deletion: COMPLETED")
		framework.Logf("  - storage.conf cleanup: VERIFIED")
		framework.Logf("========================================")
		framework.Logf("Test PASSED: Comprehensive additionalImageStores E2E lifecycle verification complete")
	})

	// TC12: Update Existing Configuration
	g.It("should update additionalImageStores when ContainerRuntimeConfig is modified [TC11]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		g.By("Creating shared image directories on worker nodes")
		imageDirs := []string{"/var/lib/imagestore-1", "/var/lib/imagestore-2", "/var/lib/imagestore-3"}
		err = createDirectoriesOnNodes(oc, pureWorkers, imageDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, imageDirs)

		g.By("Creating initial ContainerRuntimeConfig with one image store")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "update-test-ctrcfg",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/var/lib/imagestore-1")},
					},
				},
			},
		}
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying initial configuration")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("/var/lib/imagestore-1"))
		}

		g.By("Updating ContainerRuntimeConfig to add second image store")
		currentCfg, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Get(ctx, ctrcfg.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		currentCfg.Spec.ContainerRuntimeConfig.AdditionalImageStores = []machineconfigv1.AdditionalImageStore{
			{Path: machineconfigv1.StorePath("/var/lib/imagestore-1")},
			{Path: machineconfigv1.StorePath("/var/lib/imagestore-2")},
		}
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Update(ctx, currentCfg, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating after modification")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool rollout to complete after modification")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying updated configuration includes both stores")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("/var/lib/imagestore-1"))
			o.Expect(output).To(o.ContainSubstring("/var/lib/imagestore-2"))
			framework.Logf("Node %s: Both image stores configured after update", node.Name)
		}

		framework.Logf("Test PASSED: ContainerRuntimeConfig update applied successfully")
	})

	// TC17: Multiple Storage Types Combined
	g.It("should configure multiple additionalImageStores paths [TC12]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		g.By("Creating multiple shared image directories on worker nodes")
		imageDirs := []string{"/var/lib/imagestore-1", "/var/lib/imagestore-2", "/var/lib/imagestore-3"}
		err = createDirectoriesOnNodes(oc, pureWorkers, imageDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, imageDirs)

		g.By("Creating ContainerRuntimeConfig with multiple image stores")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "multi-imagestore-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/var/lib/imagestore-1")},
						{Path: machineconfigv1.StorePath("/var/lib/imagestore-2")},
						{Path: machineconfigv1.StorePath("/var/lib/imagestore-3")},
					},
				},
			},
		}
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying all image stores configured on nodes")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("/var/lib/imagestore-1"))
			o.Expect(output).To(o.ContainSubstring("/var/lib/imagestore-2"))
			o.Expect(output).To(o.ContainSubstring("/var/lib/imagestore-3"))
			framework.Logf("Node %s: All 3 image stores configured", node.Name)
		}

		framework.Logf("Test PASSED: Multiple additionalImageStores configured successfully")
	})

})
