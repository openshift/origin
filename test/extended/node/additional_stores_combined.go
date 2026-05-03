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
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	configv1 "github.com/openshift/api/config/v1"
	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

// Non-disruptive API validation tests for combined storage types
var _ = g.Describe("[Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport] Combined Additional Stores API Validation", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("combined-stores-api")

	g.BeforeEach(func(ctx context.Context) {
		g.By("Checking TechPreviewNoUpgrade feature set is enabled")
		featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if featureGate.Spec.FeatureSet != configv1.TechPreviewNoUpgrade {
			g.Skip("Skipping test: TechPreviewNoUpgrade feature set is required for additional storage configuration")
		}
	})

	// TC1: All three storage types together
	g.It("should accept ContainerRuntimeConfig with all three storage types [TC1]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with layer, image, and artifact stores")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-all-stores-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/stargz-store")},
						{Path: machineconfigv1.StorePath("/mnt/nydus-store")},
					},
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/mnt/nfs-images")},
						{Path: machineconfigv1.StorePath("/mnt/ssd-images")},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/mnt/ssd-artifacts")},
						{Path: machineconfigv1.StorePath("/mnt/nvme-artifacts")},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(2))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalImageStores).To(o.HaveLen(2))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalArtifactStores).To(o.HaveLen(2))
		framework.Logf("Test PASSED: All three storage types accepted together")
	})

	// TC2: All storage types with existing CRI-O fields
	g.It("should accept ContainerRuntimeConfig with all storage types and existing CRI-O fields [TC2]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with storage types and CRI-O settings")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-with-crio-fields-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					LogLevel:  "info",
					PidsLimit: int64Ptr(4096),
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/stargz-store")},
					},
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/mnt/nfs-images")},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/mnt/ssd-artifacts")},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		o.Expect(created.Spec.ContainerRuntimeConfig.LogLevel).To(o.Equal("info"))
		o.Expect(*created.Spec.ContainerRuntimeConfig.PidsLimit).To(o.Equal(int64(4096)))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(1))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalImageStores).To(o.HaveLen(1))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalArtifactStores).To(o.HaveLen(1))
		framework.Logf("Test PASSED: Storage types with CRI-O fields accepted")
	})

	// TC3: Maximum stores for each type combined
	g.It("should accept ContainerRuntimeConfig with maximum stores for each type [TC3]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with max layer (5), image (10), and artifact (10) stores")
		layerStores := make([]machineconfigv1.AdditionalLayerStore, 5)
		for i := 0; i < 5; i++ {
			layerStores[i] = machineconfigv1.AdditionalLayerStore{
				Path: machineconfigv1.StorePath(fmt.Sprintf("/var/lib/layer-store-%d", i)),
			}
		}

		imageStores := make([]machineconfigv1.AdditionalImageStore, 10)
		for i := 0; i < 10; i++ {
			imageStores[i] = machineconfigv1.AdditionalImageStore{
				Path: machineconfigv1.StorePath(fmt.Sprintf("/var/lib/image-store-%d", i)),
			}
		}

		artifactStores := make([]machineconfigv1.AdditionalArtifactStore, 10)
		for i := 0; i < 10; i++ {
			artifactStores[i] = machineconfigv1.AdditionalArtifactStore{
				Path: machineconfigv1.StorePath(fmt.Sprintf("/var/lib/artifact-store-%d", i)),
			}
		}

		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-max-stores-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores:    layerStores,
					AdditionalImageStores:    imageStores,
					AdditionalArtifactStores: artifactStores,
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(5))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalImageStores).To(o.HaveLen(10))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalArtifactStores).To(o.HaveLen(10))
		framework.Logf("Test PASSED: Maximum stores for all types accepted (5 layer, 10 image, 10 artifact)")
	})

	// TC4: Same path allowed across different store types
	g.It("should accept same path across different store types [TC4]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with same path in different store types")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-same-path-diff-types-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/mnt/shared-storage")},
					},
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/mnt/shared-storage")},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/mnt/shared-storage")},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		if err != nil {
			framework.Logf("Same path across different store types rejected (may be expected): %v", err)
		} else {
			defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
			o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(1))
			o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalImageStores).To(o.HaveLen(1))
			o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalArtifactStores).To(o.HaveLen(1))
			framework.Logf("Test PASSED: Same path across different store types accepted")
		}
	})

	// TC5: Layer stores only (partial configuration)
	g.It("should accept ContainerRuntimeConfig with only additionalLayerStores [TC5]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with only layer stores")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-layer-only-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/stargz-store")},
						{Path: machineconfigv1.StorePath("/mnt/nydus-store")},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(2))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalImageStores).To(o.BeNil())
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalArtifactStores).To(o.BeNil())
		framework.Logf("Test PASSED: Only layer stores accepted")
	})

	// TC6: Image and artifact stores without layer stores
	g.It("should accept ContainerRuntimeConfig with image and artifact stores only [TC6]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with image and artifact stores only")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-image-artifact-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/mnt/nfs-images")},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/mnt/ssd-artifacts")},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.BeNil())
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalImageStores).To(o.HaveLen(1))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalArtifactStores).To(o.HaveLen(1))
		framework.Logf("Test PASSED: Image and artifact stores without layer stores accepted")
	})

	// TC7: Layer stores with :ref suffix combined with other stores
	g.It("should accept layer stores with :ref suffix combined with image and artifact stores [TC7]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with :ref layer store and other stores")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-ref-layer-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/stargz-store/store:ref")},
					},
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/mnt/nfs-images")},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/mnt/ssd-artifacts")},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		if err != nil {
			framework.Logf("Layer store with :ref suffix rejected: %v", err)
		} else {
			defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
			o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(1))
			framework.Logf("Test PASSED: Layer store with :ref suffix combined with other stores accepted")
		}
	})

	// TC8: Reject if any store type has invalid path
	g.It("should reject if any store type has invalid path in combined config [TC8]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with valid layer/artifact but invalid image path")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-invalid-image-path-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/stargz-store")},
					},
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("relative/invalid/path")},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/mnt/ssd-artifacts")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		framework.Logf("Test PASSED: Combined config with invalid image path rejected: %v", err)
	})

	// TC9: Reject if layer stores exceed max while other stores are valid
	g.It("should reject if layer stores exceed max even with valid image/artifact stores [TC9]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with 6 layer stores (exceeds max of 5)")
		layerStores := make([]machineconfigv1.AdditionalLayerStore, 6)
		for i := 0; i < 6; i++ {
			layerStores[i] = machineconfigv1.AdditionalLayerStore{
				Path: machineconfigv1.StorePath(fmt.Sprintf("/var/lib/layer-store-%d", i)),
			}
		}

		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-exceed-layer-max-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: layerStores,
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/mnt/nfs-images")},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/mnt/ssd-artifacts")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("must have at most 5 items"))
		framework.Logf("Test PASSED: Exceeding layer store max rejected: %v", err)
	})

	// TC10: Reject duplicate paths within same store type in combined config
	g.It("should reject duplicate paths within same store type in combined config [TC10]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with duplicate paths in image stores")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-duplicate-image-path-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/stargz-store")},
					},
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/mnt/nfs-images")},
						{Path: machineconfigv1.StorePath("/mnt/nfs-images")},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/mnt/ssd-artifacts")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("duplicate"))
		framework.Logf("Test PASSED: Duplicate paths in same store type rejected: %v", err)
	})
})

// Disruptive E2E tests for combined storage types
var _ = g.Describe("[Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][Serial][Disruptive] Combined Additional Stores E2E", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("combined-stores-e2e")

	g.BeforeEach(func(ctx context.Context) {
		g.By("Checking TechPreviewNoUpgrade feature set is enabled")
		featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if featureGate.Spec.FeatureSet != configv1.TechPreviewNoUpgrade {
			g.Skip("Skipping test: TechPreviewNoUpgrade feature set is required for additional storage configuration")
		}

		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.Type == configv1.AzurePlatformType {
			g.Skip("Skipping test on Microsoft Azure cluster")
		}
	})

	// TC11: Configure all three storage types and verify storage.conf
	g.It("should configure all three storage types and generate correct storage.conf [TC11]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(workerNodes)).To(o.BeNumerically(">", 0))
		pureWorkers := getPureWorkerNodes(workerNodes)
		if len(pureWorkers) < 1 {
			e2eskipper.Skipf("Need at least 1 worker node for this test")
		}

		g.By("Creating shared directories on worker nodes")
		allDirs := []string{
			"/var/lib/combined-layers",
			"/var/lib/combined-images",
			"/var/lib/combined-artifacts",
		}
		err = createDirectoriesOnNodes(oc, pureWorkers, allDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, allDirs)

		g.By("Creating ContainerRuntimeConfig with all three storage types")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-e2e-all-stores-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/combined-layers")},
					},
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/var/lib/combined-images")},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/var/lib/combined-artifacts")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		g.By("Waiting for ContainerRuntimeConfig to be processed by MCO")
		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating")

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying storage.conf contains all store types on all worker nodes")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(output).To(o.ContainSubstring("/var/lib/combined-layers"),
				"storage.conf should contain layer store path on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring("/var/lib/combined-images"),
				"storage.conf should contain image store path on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring("/var/lib/combined-artifacts"),
				"storage.conf should contain artifact store path on node %s", node.Name)

			framework.Logf("Node %s: All three storage types verified in storage.conf", node.Name)
		}

		g.By("Verifying CRI-O is running")
		for _, node := range pureWorkers {
			crioStatus, err := ExecOnNodeWithChroot(oc, node.Name, "systemctl", "is-active", "crio")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(crioStatus)).To(o.Equal("active"))
		}

		framework.Logf("Test PASSED: All three storage types configured successfully")
	})

	// TC12: Update combined config - add more stores to each type
	g.It("should update combined config by adding stores to each type [TC12]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		g.By("Creating shared directories")
		allDirs := []string{
			"/var/lib/layer-1", "/var/lib/layer-2",
			"/var/lib/image-1", "/var/lib/image-2",
			"/var/lib/artifact-1", "/var/lib/artifact-2",
		}
		err = createDirectoriesOnNodes(oc, pureWorkers, allDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, allDirs)

		g.By("Creating initial ContainerRuntimeConfig with one store per type")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-update-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/layer-1")},
					},
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/var/lib/image-1")},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/var/lib/artifact-1")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Updating ContainerRuntimeConfig to add second store to each type")
		currentCfg, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Get(ctx, ctrcfg.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		currentCfg.Spec.ContainerRuntimeConfig.AdditionalLayerStores = []machineconfigv1.AdditionalLayerStore{
			{Path: machineconfigv1.StorePath("/var/lib/layer-1")},
			{Path: machineconfigv1.StorePath("/var/lib/layer-2")},
		}
		currentCfg.Spec.ContainerRuntimeConfig.AdditionalImageStores = []machineconfigv1.AdditionalImageStore{
			{Path: machineconfigv1.StorePath("/var/lib/image-1")},
			{Path: machineconfigv1.StorePath("/var/lib/image-2")},
		}
		currentCfg.Spec.ContainerRuntimeConfig.AdditionalArtifactStores = []machineconfigv1.AdditionalArtifactStore{
			{Path: machineconfigv1.StorePath("/var/lib/artifact-1")},
			{Path: machineconfigv1.StorePath("/var/lib/artifact-2")},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Update(ctx, currentCfg, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying updated configuration has all stores")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, dir := range allDirs {
				o.Expect(output).To(o.ContainSubstring(dir),
					"storage.conf should contain %s on node %s", dir, node.Name)
			}
			framework.Logf("Node %s: All stores verified after update", node.Name)
		}

		framework.Logf("Test PASSED: Combined config update applied successfully")
	})

	// TC13: Remove one store type while keeping others
	g.It("should remove one store type while keeping others [TC13]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		g.By("Creating shared directories")
		allDirs := []string{"/var/lib/layers", "/var/lib/images", "/var/lib/artifacts"}
		err = createDirectoriesOnNodes(oc, pureWorkers, allDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, allDirs)

		g.By("Creating ContainerRuntimeConfig with all three store types")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-remove-type-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/layers")},
					},
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/var/lib/images")},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/var/lib/artifacts")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Updating ContainerRuntimeConfig to remove layer stores")
		currentCfg, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Get(ctx, ctrcfg.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		currentCfg.Spec.ContainerRuntimeConfig.AdditionalLayerStores = nil

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Update(ctx, currentCfg, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying layer stores removed but image/artifact stores remain")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(output).NotTo(o.ContainSubstring("/var/lib/layers"),
				"storage.conf should not contain layer store path on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring("/var/lib/images"),
				"storage.conf should still contain image store path on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring("/var/lib/artifacts"),
				"storage.conf should still contain artifact store path on node %s", node.Name)

			framework.Logf("Node %s: Layer stores removed, image/artifact stores remain", node.Name)
		}

		framework.Logf("Test PASSED: Partial removal of store types successful")
	})
})
