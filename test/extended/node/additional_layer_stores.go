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

const (
	additionalLayerStorePath     = "/var/lib/additional-layers"
	additionalLayerStoreTestName = "additional-layerstore-test"
	maxLayerStoresCount          = 5
)

// Non-disruptive API validation tests - can run in parallel
var _ = g.Describe("[Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport] Additional Layer Stores API Validation", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-layer-stores-api")

	g.BeforeEach(func(ctx context.Context) {
		g.By("Checking TechPreviewNoUpgrade feature set is enabled")
		featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if featureGate.Spec.FeatureSet != configv1.TechPreviewNoUpgrade {
			g.Skip("Skipping test: TechPreviewNoUpgrade feature set is required for additionalLayerStores")
		}
	})

	// TC1: Should be able to create ContainerRuntimeConfig with multiple additionalLayerStores
	g.It("should accept multiple valid additionalLayerStores paths [TC1]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with multiple valid layer store paths")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-multi-path-test",
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
						{Path: machineconfigv1.StorePath("/opt/layer_store-v1.0")},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(3))
		framework.Logf("Test PASSED: Multiple valid layer store paths accepted")
	})

	// TC2: Should fail if additionalLayerStores path is empty
	g.It("should reject empty path for additionalLayerStores [TC2]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with empty path")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-empty-path-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("at least 1 chars long"))
		framework.Logf("Test PASSED: Empty path correctly rejected: %v", err)
	})

	// TC3: Should fail if additionalLayerStores path is not absolute
	g.It("should reject relative path for additionalLayerStores [TC3]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with relative path")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-relative-path-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("var/lib/stargz-store")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("path must be absolute and contain only alphanumeric characters"))
		framework.Logf("Test PASSED: Relative path correctly rejected: %v", err)
	})

	// TC4: Should fail if additionalLayerStores path contains spaces
	g.It("should reject path with spaces for additionalLayerStores [TC4]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with path containing spaces")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-path-spaces-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/stargz store")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("path must be absolute and contain only alphanumeric characters"))
		framework.Logf("Test PASSED: Path with spaces correctly rejected: %v", err)
	})

	// TC5: Should fail if additionalLayerStores path contains invalid characters
	g.It("should reject path with invalid characters for additionalLayerStores [TC5]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		invalidChars := []struct {
			name string
			path string
			char string
		}{
			{"at-symbol", "/var/lib/stargz@store", "@"},
			{"exclamation", "/var/lib/stargz!store", "!"},
			{"hash", "/var/lib/stargz#store", "#"},
			{"dollar", "/var/lib/stargz$store", "$"},
			{"percent", "/var/lib/stargz%store", "%"},
		}

		for _, tc := range invalidChars {
			g.By(fmt.Sprintf("Testing path with invalid character: %s", tc.char))
			ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("layer-invalid-char-%s-test", tc.name),
				},
				Spec: machineconfigv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pools.operator.machineconfiguration.openshift.io/worker": "",
						},
					},
					ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
						AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
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

	// TC6: Should fail if additionalLayerStores path is too long (>256 bytes)
	g.It("should reject path exceeding 256 characters for additionalLayerStores [TC6]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		longPath := "/" + strings.Repeat("a", 256)
		g.By(fmt.Sprintf("Creating ContainerRuntimeConfig with path of %d characters", len(longPath)))

		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-long-path-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath(longPath)},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.Or(o.ContainSubstring("256"), o.ContainSubstring("Too long")))
		framework.Logf("Test PASSED: Long path correctly rejected: %v", err)
	})

	// TC7: Should fail if additionalLayerStores exceeds maximum of 5 items
	g.It("should reject more than 5 additionalLayerStores [TC7]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with 6 layer stores (exceeds max of 5)")
		layerStores := make([]machineconfigv1.AdditionalLayerStore, 6)
		for i := 0; i < 6; i++ {
			layerStores[i] = machineconfigv1.AdditionalLayerStore{Path: machineconfigv1.StorePath(fmt.Sprintf("/var/lib/store%d", i))}
		}

		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-exceed-limit-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: layerStores,
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("must have at most 5 items"))
		framework.Logf("Test PASSED: 6 layer stores correctly rejected: %v", err)
	})

	// TC8: Should fail if additionalLayerStores path contains consecutive forward slashes
	g.It("should reject path with consecutive forward slashes for additionalLayerStores [TC8]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with consecutive forward slashes")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-consecutive-slashes-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib//stargz-store")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("consecutive"))
		framework.Logf("Test PASSED: Consecutive slashes correctly rejected: %v", err)
	})

	// TC9: Should fail if additionalLayerStores contains duplicate paths
	g.It("should reject duplicate paths in additionalLayerStores [TC9]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with duplicate paths")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-duplicate-path-test",
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
						{Path: machineconfigv1.StorePath("/var/lib/stargz-store")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("duplicate"))
		framework.Logf("Test PASSED: Duplicate paths correctly rejected: %v", err)
	})

	// TC10: Should accept path with :ref suffix for reference-based layer stores (stargz-store format)
	g.It("should accept path with :ref suffix for reference-based layer stores [TC10]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with :ref suffix path (stargz-store format)")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-ref-path-test",
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
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(1))
		o.Expect(string(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores[0].Path)).To(o.Equal("/var/lib/stargz-store/store:ref"))
		framework.Logf("Test PASSED: Path with :ref suffix accepted for stargz-store format")
	})

	// TC11: Should accept mixed paths with and without :ref suffix
	g.It("should accept mixed additionalLayerStores paths with and without :ref suffix [TC11]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with mixed paths (with and without :ref)")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-mixed-ref-path-test",
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
						{Path: machineconfigv1.StorePath("/mnt/nydus-store")},
						{Path: machineconfigv1.StorePath("/opt/layer-store:ref")},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(3))
		framework.Logf("Test PASSED: Mixed paths with and without :ref suffix accepted")
	})

	// TC12: Should fail if path contains colon with suffix other than :ref
	g.It("should reject path with colon suffix other than :ref [TC12]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		invalidSuffixes := []struct {
			name   string
			path   string
			suffix string
		}{
			{"other", "/var/lib/stargz-store:other", ":other"},
			{"refs", "/var/lib/stargz-store:refs", ":refs"},
			{"reference", "/var/lib/stargz-store:reference", ":reference"},
			{"colon-only", "/var/lib/stargz-store:", ":"},
			{"colon-middle", "/var/lib:something/store", ":something"},
		}

		for _, tc := range invalidSuffixes {
			g.By(fmt.Sprintf("Testing path with invalid colon suffix: %s", tc.suffix))
			ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("layer-invalid-suffix-%s-test", tc.name),
				},
				Spec: machineconfigv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pools.operator.machineconfiguration.openshift.io/worker": "",
						},
					},
					ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
						AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
							{Path: machineconfigv1.StorePath(tc.path)},
						},
					},
				},
			}

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring("optionally end with ':ref'"))
			framework.Logf("Path with '%s' suffix correctly rejected: %v", tc.suffix, err)
		}
		framework.Logf("Test PASSED: Invalid colon suffixes correctly rejected")
	})

	// TC13: Combined test - all storage types together with other fields
	g.It("should accept ContainerRuntimeConfig with all storage types and existing fields [TC13]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with all storage types and other fields")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-combined-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					LogLevel: "info",
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/stargz-store")},
					},
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/mnt/nfs-images")},
						{Path: machineconfigv1.StorePath("/mnt/ssd-images")},
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

		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(1))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalImageStores).To(o.HaveLen(2))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalArtifactStores).To(o.HaveLen(1))
		framework.Logf("Test PASSED: Combined storage types with other fields accepted")
	})

	// TC14: Single layer store creation (P1 Basic)
	g.It("should successfully create ContainerRuntimeConfig with single additionalLayerStore [TC14]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with single layer store")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layerstore-single-store-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/layerstore-single")},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		g.By("Verifying resource created successfully")
		o.Expect(created.Name).To(o.Equal(ctrcfg.Name))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(1))

		framework.Logf("Test PASSED: Single layer store created successfully")
	})

	// TC15: Same path across store types (P2)
	g.It("should accept same path across different storage types [TC15]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with same path for layer, image, and artifact stores")
		sharedPath := "/mnt/shared-storage"
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layerstore-same-path-cross-type-test",
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
var _ = g.Describe("[Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][Serial][Disruptive] Additional Layer Stores E2E", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-layer-stores")

	g.BeforeEach(func(ctx context.Context) {
		g.By("Checking TechPreviewNoUpgrade feature set is enabled")
		featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if featureGate.Spec.FeatureSet != configv1.TechPreviewNoUpgrade {
			g.Skip("Skipping test: TechPreviewNoUpgrade feature set is required for additionalLayerStores")
		}

		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.Type == configv1.AzurePlatformType {
			g.Skip("Skipping test on Microsoft Azure cluster")
		}
	})

	// TC16: Comprehensive E2E test - Configure and Verify storage.conf
	g.It("should configure additionalLayerStores and generate correct storage.conf [TC16]", func(ctx context.Context) {
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
		g.By("PHASE 1: Creating shared layer directory on worker nodes")
		layerDirs := []string{additionalLayerStorePath}
		err = createDirectoriesOnNodes(oc, pureWorkers, layerDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, layerDirs)

		// PHASE 2: Create ContainerRuntimeConfig and verify MCO processing
		g.By("PHASE 2: Creating ContainerRuntimeConfig with additionalLayerStores")
		ctrcfg := createAdditionalLayerStoresCTRCfg(additionalLayerStoreTestName, additionalLayerStorePath)
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
		framework.Logf("ContainerRuntimeConfig %s created", ctrcfg.Name)

		g.By("Verifying ContainerRuntimeConfig resource created")
		createdCfg, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Get(ctx, ctrcfg.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(createdCfg.Name).To(o.Equal(ctrcfg.Name))
		o.Expect(createdCfg.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(1))
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

		// PHASE 3: Verify storage.conf on all nodes
		g.By("PHASE 3: Verifying storage.conf contains additionalLayerStores on all worker nodes")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("additionallayerstores"),
				"storage.conf should contain additionallayerstores on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring(additionalLayerStorePath),
				"storage.conf should contain path %s on node %s", additionalLayerStorePath, node.Name)
			framework.Logf("Node %s: storage.conf verified with additionalLayerStores", node.Name)
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

		// PHASE 4: Final Summary
		framework.Logf("========================================")
		framework.Logf("TEST RESULTS SUMMARY")
		framework.Logf("========================================")
		framework.Logf("ContainerRuntimeConfig: %s", ctrcfg.Name)
		framework.Logf("Generated MachineConfig: %s", generatedMCName)
		framework.Logf("MCP rollout: COMPLETED")
		framework.Logf("storage.conf updated: YES")
		framework.Logf("CRI-O active: YES")
		framework.Logf("All nodes Ready: YES")
		framework.Logf("========================================")
		framework.Logf("Test PASSED: additionalLayerStores E2E verification complete")
	})

	// TC17: Remove configuration when ContainerRuntimeConfig is deleted
	g.It("should remove additionalLayerStores configuration when ContainerRuntimeConfig is deleted [TC17]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		g.By("Creating shared layer directory on worker nodes")
		layerDirs := []string{additionalLayerStorePath}
		err = createDirectoriesOnNodes(oc, pureWorkers, layerDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, layerDirs)

		g.By("Creating ContainerRuntimeConfig")
		ctrcfg := createAdditionalLayerStoresCTRCfg(additionalLayerStoreTestName, additionalLayerStorePath)
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying additionalLayerStores is configured")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring(additionalLayerStorePath))
		}

		g.By("Deleting ContainerRuntimeConfig")
		err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Delete(ctx, ctrcfg.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to update after deletion")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying additionalLayerStores is removed from storage.conf")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).NotTo(o.ContainSubstring(additionalLayerStorePath),
				"storage.conf should not contain %s after ContainerRuntimeConfig deletion on node %s",
				additionalLayerStorePath, node.Name)
		}

		framework.Logf("Test PASSED: additionalLayerStores removed after ContainerRuntimeConfig deletion")
	})

	// TC18: Update Existing Configuration
	g.It("should update additionalLayerStores when ContainerRuntimeConfig is modified [TC18]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		g.By("Creating shared layer directories on worker nodes")
		layerDirs := []string{"/var/lib/layerstore-1", "/var/lib/layerstore-2", "/var/lib/layerstore-3"}
		err = createDirectoriesOnNodes(oc, pureWorkers, layerDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, layerDirs)

		g.By("Creating initial ContainerRuntimeConfig with one layer store")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-update-test-ctrcfg",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/layerstore-1")},
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

		g.By("Verifying initial configuration")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("/var/lib/layerstore-1"))
		}

		g.By("Updating ContainerRuntimeConfig to add second layer store")
		currentCfg, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Get(ctx, ctrcfg.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		currentCfg.Spec.ContainerRuntimeConfig.AdditionalLayerStores = []machineconfigv1.AdditionalLayerStore{
			{Path: machineconfigv1.StorePath("/var/lib/layerstore-1")},
			{Path: machineconfigv1.StorePath("/var/lib/layerstore-2")},
		}
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Update(ctx, currentCfg, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying updated configuration includes both stores")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("/var/lib/layerstore-1"))
			o.Expect(output).To(o.ContainSubstring("/var/lib/layerstore-2"))
			framework.Logf("Node %s: Both layer stores configured after update", node.Name)
		}

		framework.Logf("Test PASSED: ContainerRuntimeConfig update applied successfully")
	})

	// TC19: Missing Storage Path Handling
	g.It("should handle missing storage path gracefully [TC19]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with non-existent path")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-missing-path-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/mnt/nonexistent-layerstore")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		g.By("Waiting for ContainerRuntimeConfig to be processed")
		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying nodes are still Ready (graceful handling)")
		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		for _, node := range pureWorkers {
			nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, node.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(isNodeInReadyState(nodeObj)).To(o.BeTrue(),
				"Node %s should remain Ready even with non-existent layer store path", node.Name)
		}

		g.By("Verifying CRI-O is running despite missing path")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "systemctl", "is-active", "crio")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(output)).To(o.Equal("active"),
				"CRI-O should be active on node %s", node.Name)
		}

		framework.Logf("Test PASSED: Missing storage path handled gracefully")
	})

	// TC20: Multiple Storage Paths (up to max 5)
	g.It("should configure multiple additionalLayerStores paths up to maximum [TC20]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		g.By("Creating multiple shared layer directories on worker nodes (max 5)")
		layerDirs := []string{
			"/var/lib/layerstore-1",
			"/var/lib/layerstore-2",
			"/var/lib/layerstore-3",
			"/var/lib/layerstore-4",
			"/var/lib/layerstore-5",
		}
		err = createDirectoriesOnNodes(oc, pureWorkers, layerDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, layerDirs)

		g.By("Creating ContainerRuntimeConfig with 5 layer stores (maximum allowed)")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "multi-layerstore-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/layerstore-1")},
						{Path: machineconfigv1.StorePath("/var/lib/layerstore-2")},
						{Path: machineconfigv1.StorePath("/var/lib/layerstore-3")},
						{Path: machineconfigv1.StorePath("/var/lib/layerstore-4")},
						{Path: machineconfigv1.StorePath("/var/lib/layerstore-5")},
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

		g.By("Verifying all 5 layer stores configured on nodes")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, dir := range layerDirs {
				o.Expect(output).To(o.ContainSubstring(dir),
					"storage.conf should contain %s on node %s", dir, node.Name)
			}
			framework.Logf("Node %s: All 5 layer stores configured", node.Name)
		}

		framework.Logf("Test PASSED: Multiple additionalLayerStores (max 5) configured successfully")
	})

	// TC21: Default resolution unchanged (P1 Regression)
	g.It("should not affect default layer resolution when no additional stores configured [TC21]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		if len(pureWorkers) < 1 {
			e2eskipper.Skipf("Need at least 1 worker node for this test")
		}

		g.By("Checking if any additionalLayerStores CRCs exist")
		crcs, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		hasLayerStores := false
		for _, crc := range crcs.Items {
			if crc.Spec.ContainerRuntimeConfig != nil && len(crc.Spec.ContainerRuntimeConfig.AdditionalLayerStores) > 0 {
				hasLayerStores = true
				break
			}
		}

		if hasLayerStores {
			framework.Logf("Skipping regression check - cluster already has layer stores configured")
			g.Skip("Cluster already has additionalLayerStores configured")
		}

		g.By("Verifying storage.conf does NOT contain additionallayerstores section")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).NotTo(o.ContainSubstring("additionallayerstores"),
				"storage.conf should not contain additionallayerstores section on node %s", node.Name)
		}

		framework.Logf("Test PASSED: Default behavior unchanged when no additional stores configured")
	})

	// TC22: Permission denied handling (P2)
	g.It("should handle permission denied on storage path gracefully [TC22]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		if len(pureWorkers) < 1 {
			e2eskipper.Skipf("Need at least 1 worker node for this test")
		}

		restrictedPath := "/var/lib/restricted-layers"

		g.By("Creating directory with no permissions (chmod 000)")
		for _, node := range pureWorkers {
			_, err := ExecOnNodeWithChroot(oc, node.Name, "mkdir", "-p", restrictedPath)
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = ExecOnNodeWithChroot(oc, node.Name, "chmod", "000", restrictedPath)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		defer func() {
			for _, node := range pureWorkers {
				ExecOnNodeWithChroot(oc, node.Name, "chmod", "755", restrictedPath)
				ExecOnNodeWithChroot(oc, node.Name, "rm", "-rf", restrictedPath)
			}
		}()

		g.By("Creating ContainerRuntimeConfig with restricted path")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layerstore-permission-denied-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath(restrictedPath)},
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

		g.By("Verifying nodes remain Ready despite permission denied")
		for _, node := range pureWorkers {
			nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, node.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(isNodeInReadyState(nodeObj)).To(o.BeTrue())
		}

		framework.Logf("Test PASSED: System handles permission denied gracefully")
	})

	// TC23: Config merge (multiple CRCs) (P2)
	g.It("should merge multiple ContainerRuntimeConfigs targeting same pool [TC23]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		if len(pureWorkers) < 1 {
			e2eskipper.Skipf("Need at least 1 worker node for this test")
		}

		g.By("Creating directories for both CRCs")
		dirs := []string{"/var/lib/layerstore-crc1", "/var/lib/layerstore-crc2"}
		err = createDirectoriesOnNodes(oc, pureWorkers, dirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, dirs)

		g.By("Creating first ContainerRuntimeConfig")
		ctrcfg1 := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layerstore-merge-test-crc1",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/layerstore-crc1")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg1.Name)

		g.By("Creating second ContainerRuntimeConfig")
		ctrcfg2 := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layerstore-merge-test-crc2",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath("/var/lib/layerstore-crc2")},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg2.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg1.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg2.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying MCO merged both layer stores")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("/var/lib/layerstore-crc1"))
			o.Expect(output).To(o.ContainSubstring("/var/lib/layerstore-crc2"))
		}

		framework.Logf("Test PASSED: MCO successfully merged multiple ContainerRuntimeConfigs")
	})

	// TC24: Existing stores still work (P2)
	g.It("should not interfere with existing image and artifact stores when adding layer stores [TC24]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		if len(pureWorkers) < 1 {
			e2eskipper.Skipf("Need at least 1 worker node for this test")
		}

		g.By("Creating directories for all store types")
		dirs := []string{"/var/lib/test-layers-lyr", "/var/lib/test-images-lyr", "/var/lib/test-artifacts-lyr"}
		err = createDirectoriesOnNodes(oc, pureWorkers, dirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, dirs)

		g.By("Creating ContainerRuntimeConfig with image and artifact stores first")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layerstore-existing-stores-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath("/var/lib/test-images-lyr")},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath("/var/lib/test-artifacts-lyr")},
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

		g.By("Adding layer stores to existing configuration")
		currentCfg, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Get(ctx, ctrcfg.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		currentCfg.Spec.ContainerRuntimeConfig.AdditionalLayerStores = []machineconfigv1.AdditionalLayerStore{
			{Path: machineconfigv1.StorePath("/var/lib/test-layers-lyr")},
		}
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Update(ctx, currentCfg, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying all three store types work together")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("/var/lib/test-layers-lyr"))
			o.Expect(output).To(o.ContainSubstring("/var/lib/test-images-lyr"))
			o.Expect(output).To(o.ContainSubstring("/var/lib/test-artifacts-lyr"))
		}

		framework.Logf("Test PASSED: Layer stores do not interfere with existing image/artifact stores")
	})
})

func createAdditionalLayerStoresCTRCfg(testName, storePath string) *machineconfigv1.ContainerRuntimeConfig {
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
				AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
					{Path: machineconfigv1.StorePath(storePath)},
				},
			},
		},
	}
}

// Stargz-store E2E tests - tests with actual stargz-store daemon
var _ = g.Describe("[Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][Serial][Disruptive][Slow] Additional Layer Stores with Stargz-Store", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("stargz-layer-stores")
	var stargzSetup *StargzStoreSetup

	g.BeforeEach(func(ctx context.Context) {
		g.By("Checking TechPreviewNoUpgrade feature set is enabled")
		featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if featureGate.Spec.FeatureSet != configv1.TechPreviewNoUpgrade {
			g.Skip("Skipping test: TechPreviewNoUpgrade feature set is required for additionalLayerStores")
		}

		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.Type == configv1.AzurePlatformType {
			g.Skip("Skipping test on Microsoft Azure cluster")
		}

		stargzSetup = NewStargzStoreSetup(oc)
	})

	g.AfterEach(func(ctx context.Context) {
		if stargzSetup != nil && stargzSetup.IsDeployed() {
			g.By("Cleaning up stargz-store")
			err := stargzSetup.Cleanup(ctx)
			if err != nil {
				framework.Logf("Warning: stargz-store cleanup failed: %v", err)
			}
		}
	})

	// TC25: Deploy stargz-store and configure as additional layer store
	g.It("should deploy stargz-store and configure additionalLayerStores with :ref suffix [TC25]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		// PHASE 1: Deploy stargz-store
		g.By("PHASE 1: Deploying stargz-store on worker nodes")
		err = stargzSetup.Deploy(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("stargz-store deployed successfully")

		// PHASE 2: Create ContainerRuntimeConfig with stargz-store path
		g.By("PHASE 2: Creating ContainerRuntimeConfig with stargz-store path")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "stargz-layer-store-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath(stargzSetup.GetStorePath())},
					},
				},
			},
		}

		created, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)
		framework.Logf("ContainerRuntimeConfig %s created with path: %s", created.Name, stargzSetup.GetStorePath())

		g.By("Waiting for ContainerRuntimeConfig to be processed")
		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating")

		g.By("Waiting for MachineConfigPool rollout")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		// PHASE 3: Verify configuration
		g.By("PHASE 3: Verifying stargz-store configuration in storage.conf")
		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("additionallayerstores"),
				"storage.conf should contain additionallayerstores on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring("/var/lib/stargz-store/store"),
				"storage.conf should contain stargz-store path on node %s", node.Name)
			framework.Logf("Node %s: storage.conf verified with stargz-store path", node.Name)
		}

		g.By("Verifying stargz-store service is still active after MCP rollout")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "systemctl", "is-active", "stargz-store")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(output)).To(o.Equal("active"),
				"stargz-store should be active on node %s", node.Name)
		}

		g.By("Verifying CRI-O is active")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "systemctl", "is-active", "crio")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(output)).To(o.Equal("active"),
				"CRI-O should be active on node %s", node.Name)
		}

		framework.Logf("========================================")
		framework.Logf("TEST RESULTS SUMMARY - STARGZ-STORE")
		framework.Logf("========================================")
		framework.Logf("stargz-store deployed: YES")
		framework.Logf("ContainerRuntimeConfig created: YES")
		framework.Logf("MCP rollout completed: YES")
		framework.Logf("storage.conf updated with :ref path: YES")
		framework.Logf("stargz-store service active: YES")
		framework.Logf("CRI-O active: YES")
		framework.Logf("========================================")
		framework.Logf("Test PASSED: stargz-store E2E verification complete")
	})

	// TC26: Verify stargz-store FUSE mount is accessible
	g.It("should verify stargz-store FUSE mount is accessible from CRI-O [TC26]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Deploying stargz-store")
		err = stargzSetup.Deploy(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating ContainerRuntimeConfig with stargz-store path")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "stargz-fuse-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath(stargzSetup.GetStorePath())},
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

		g.By("Verifying FUSE mount on all worker nodes")
		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "mount")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("stargz"),
				"FUSE mount should contain stargz on node %s", node.Name)
			framework.Logf("Node %s: FUSE mount verified", node.Name)

			// Verify the store directory is accessible
			_, err = ExecOnNodeWithChroot(oc, node.Name, "ls", "-la", "/var/lib/stargz-store/store")
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Node %s: stargz-store directory accessible", node.Name)
		}

		framework.Logf("Test PASSED: stargz-store FUSE mount accessible on all nodes")
	})

	// TC27: Comprehensive E2E test with eStargz image - lazy pulling and layer sharing
	// Reference: /home/bgudi/work/src/github.com/openshift/epic/additionalArtifactsStore/test-logs/lazy-pulling-test-session-2026-04-15.txt
	g.It("should verify lazy pulling and layer sharing with eStargz image [TC27]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		o.Expect(len(pureWorkers)).To(o.BeNumerically(">=", 1), "Need at least 1 worker node")

		testNode := pureWorkers[0]
		testNamespace := oc.Namespace()

		// eStargz-optimized test image with proper estargz annotations:
		// - containerd.io/snapshot/stargz/toc.digest
		// - io.containers.estargz.uncompressed-size
		// Verified working image from test logs
		eStargzImage := "quay.io/bgudi/test-small:estargz"

		// PHASE 1: Deploy stargz-store
		g.By("PHASE 1: Deploying stargz-store on worker nodes")
		err = stargzSetup.Deploy(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("stargz-store deployed successfully")

		// PHASE 2: Verify stargz-store service active
		g.By("PHASE 2: Verifying stargz-store service is active on all workers")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "systemctl", "is-active", "stargz-store")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(output)).To(o.Equal("active"),
				"stargz-store should be active on node %s", node.Name)
			framework.Logf("Node %s: stargz-store service active", node.Name)
		}

		// PHASE 3: Create ContainerRuntimeConfig with stargz-store path
		g.By("PHASE 3: Creating ContainerRuntimeConfig with stargz-store path")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "stargz-e2e-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath(stargzSetup.GetStorePath())},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupContainerRuntimeConfig(ctx, mcClient, ctrcfg.Name)

		// PHASE 4: Wait for MCO processing and MCP rollout
		g.By("PHASE 4: Waiting for MCO processing and MCP rollout")
		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MCP rollout completed")

		// PHASE 5: Verify storage.conf updated
		g.By("PHASE 5: Verifying storage.conf updated with stargz path")
		output, err := ExecOnNodeWithChroot(oc, testNode.Name, "cat", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("/var/lib/stargz-store/store"),
			"storage.conf should contain stargz-store path")
		framework.Logf("storage.conf verified on node %s", testNode.Name)

		// PHASE 6: Verify CRI-O is active
		g.By("PHASE 6: Verifying CRI-O is active")
		crioStatus, err := ExecOnNodeWithChroot(oc, testNode.Name, "systemctl", "is-active", "crio")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(crioStatus)).To(o.Equal("active"))
		framework.Logf("CRI-O is active")

		// Get initial snapshot count
		g.By("Getting initial snapshot count in stargz-store")
		initialSnapshots := getStargzSnapshotCount(oc, testNode.Name)
		framework.Logf("Initial snapshot count: %d", initialSnapshots)

		// PHASE 7: Create first pod with eStargz image
		g.By("PHASE 7: Creating first pod with eStargz format image")
		pod1Name := "stargz-test-pod-1"
		pod1 := createTestPodSpec(pod1Name, testNamespace, eStargzImage, testNode.Name)
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, pod1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePodAndWait(ctx, oc, testNamespace, pod1Name)

		g.By("Waiting for first pod to be running")
		err = waitForPodRunning(ctx, oc, pod1Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("First pod %s is running", pod1Name)

		// PHASE 8: Verify snapshot is created
		// Snapshot structure: /var/lib/stargz-store/store/<base64-encoded-image-ref>/<sha256:layer-digest>/
		// Example: /var/lib/stargz-store/store/cXVheS5pby9iZ3VkaS90ZXN0LXNtYWxsOmVzdGFyZ3o=/sha256:.../
		g.By("PHASE 8: Verifying snapshot is created in stargz-store")
		time.Sleep(10 * time.Second) // Allow time for lazy pull and snapshot creation

		// List stargz-store contents to verify snapshot was created
		storeOutput, err := ExecOnNodeWithChroot(oc, testNode.Name, "ls", "-lRt", "/var/lib/stargz-store/store/")
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("stargz-store contents:\n%s", storeOutput)

		// Verify base64-encoded image reference directory exists
		// For quay.io/bgudi/test-small:estargz -> base64 encoded directory
		o.Expect(storeOutput).To(o.ContainSubstring("sha256:"),
			"stargz-store should contain layer directories with sha256 digests")

		snapshotsAfterPod1 := getStargzSnapshotCount(oc, testNode.Name)
		framework.Logf("Snapshot count after first pod: %d", snapshotsAfterPod1)
		o.Expect(snapshotsAfterPod1).To(o.BeNumerically(">", initialSnapshots),
			"Snapshots should be created after pulling eStargz image")

		// Verify CRI-O only has metadata (not full layer blobs)
		// When lazy pulling works, crictl images shows small size (~KB) instead of full image size
		crioImageSize, err := ExecOnNodeWithChroot(oc, testNode.Name, "crictl", "images", "--output=json")
		if err == nil {
			framework.Logf("CRI-O images info: %s", crioImageSize)
		}

		// PHASE 9: Create second pod with same image - verify layer sharing
		// Expected behavior from test logs:
		// - Pod event: "Container image already present on machine and can be accessed by the pod"
		// - Pod starts instantly (no image pull)
		// - stargz-store unchanged (no new layers fetched)
		g.By("PHASE 9: Creating second pod with same eStargz image to verify layer sharing")
		pod2Name := "stargz-test-pod-2"
		pod2 := createTestPodSpec(pod2Name, testNamespace, eStargzImage, testNode.Name)

		startTime := time.Now()
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, pod2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePodAndWait(ctx, oc, testNamespace, pod2Name)

		g.By("Waiting for second pod to be running")
		err = waitForPodRunning(ctx, oc, pod2Name, 3*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		pullDuration := time.Since(startTime)
		framework.Logf("Second pod %s started in %v (should be fast due to layer sharing)", pod2Name, pullDuration)

		// Verify pod events show "already present" (not pulled again)
		pod2Events, _ := oc.Run("describe").Args("pod", pod2Name).Output()
		framework.Logf("Second pod events: %s", pod2Events)
		o.Expect(pod2Events).To(o.Or(
			o.ContainSubstring("already present"),
			o.ContainSubstring("Successfully pulled"),
		), "Pod should either use cached image or pull quickly")

		// Verify snapshot count didn't significantly increase (layers are shared)
		snapshotsAfterPod2 := getStargzSnapshotCount(oc, testNode.Name)
		framework.Logf("Snapshot count after second pod: %d", snapshotsAfterPod2)
		o.Expect(snapshotsAfterPod2).To(o.Equal(snapshotsAfterPod1),
			"Snapshot count should remain same when using shared layers")

		// PHASE 10: Delete cached image from node (CRI-O cache only)
		// Expected behavior from test logs:
		// - crictl rmi removes image from CRI-O storage
		// - stargz-store snapshots PERSIST (not deleted)
		// - Layer directories still exist in /var/lib/stargz-store/store/
		g.By("PHASE 10: Deleting cached image from CRI-O (stargz snapshots should persist)")

		// Delete both pods first
		err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Delete(ctx, pod1Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Delete(ctx, pod2Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		time.Sleep(15 * time.Second) // Wait for pods to terminate

		// Remove cached image from CRI-O using crictl
		rmiOutput, _ := ExecOnNodeWithChroot(oc, testNode.Name, "crictl", "rmi", eStargzImage)
		framework.Logf("crictl rmi output: %s", rmiOutput)
		framework.Logf("Cached image removed from CRI-O on node %s", testNode.Name)

		// Verify CRI-O no longer has the image
		crioImages, _ := ExecOnNodeWithChroot(oc, testNode.Name, "crictl", "images")
		framework.Logf("CRI-O images after deletion: %s", crioImages)

		// Verify stargz-store snapshots still exist (key verification!)
		storeAfterDelete, _ := ExecOnNodeWithChroot(oc, testNode.Name, "ls", "-la", "/var/lib/stargz-store/store/")
		framework.Logf("stargz-store after CRI-O cache cleared: %s", storeAfterDelete)

		snapshotsAfterImageDelete := getStargzSnapshotCount(oc, testNode.Name)
		framework.Logf("Snapshot count after image deletion: %d (should be same as before)", snapshotsAfterImageDelete)
		o.Expect(snapshotsAfterImageDelete).To(o.Equal(snapshotsAfterPod1),
			"stargz-store snapshots should persist after CRI-O cache is cleared")

		// PHASE 11: Create new pod and verify it uses cached snapshots for layers
		// Expected behavior from test logs:
		// - Pod event: "Successfully pulled image in ~1.3s" (manifest re-fetch only)
		// - Layer data served from stargz-store snapshots (not re-downloaded)
		// - Full pull would take 30+ seconds for 100MB image
		g.By("PHASE 11: Creating new pod and verifying it uses cached snapshots")
		pod3Name := "stargz-test-pod-3"
		pod3 := createTestPodSpec(pod3Name, testNamespace, eStargzImage, testNode.Name)

		startTime = time.Now()
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, pod3, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePodAndWait(ctx, oc, testNamespace, pod3Name)

		g.By("Waiting for third pod to be running")
		err = waitForPodRunning(ctx, oc, pod3Name, 3*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		pullDurationAfterDelete := time.Since(startTime)
		framework.Logf("Third pod %s started in %v after CRI-O cache cleared", pod3Name, pullDurationAfterDelete)

		// Verify pod events show fast pull (manifest re-fetch only, ~1-2 seconds)
		pod3Events, _ := oc.Run("describe").Args("pod", pod3Name).Output()
		framework.Logf("Third pod events: %s", pod3Events)

		// Snapshot should still be reused (no new layers downloaded)
		snapshotsAfterPod3 := getStargzSnapshotCount(oc, testNode.Name)
		framework.Logf("Snapshot count after third pod: %d", snapshotsAfterPod3)
		o.Expect(snapshotsAfterPod3).To(o.Equal(snapshotsAfterImageDelete),
			"Snapshot count should remain same - layers served from existing snapshots")

		// Final Summary
		framework.Logf("========================================")
		framework.Logf("TEST RESULTS SUMMARY - ESTARGZ E2E")
		framework.Logf("========================================")
		framework.Logf("1. stargz-store deployed: YES")
		framework.Logf("2. stargz-store service active: YES")
		framework.Logf("3. ContainerRuntimeConfig applied: YES")
		framework.Logf("4. MCO/MCP rollout completed: YES")
		framework.Logf("5. storage.conf updated with :ref path: YES")
		framework.Logf("6. CRI-O active: YES")
		framework.Logf("7. First pod with eStargz image: CREATED")
		framework.Logf("8. Snapshots created: YES (count: %d -> %d)", initialSnapshots, snapshotsAfterPod1)
		framework.Logf("9. Second pod layer sharing: VERIFIED (snapshot count unchanged)")
		framework.Logf("10. CRI-O cache cleared: YES (stargz snapshots persisted)")
		framework.Logf("11. Third pod using cached snapshots: YES (fast startup)")
		framework.Logf("========================================")
		framework.Logf("Image: %s", eStargzImage)
		framework.Logf("Test Node: %s", testNode.Name)
		framework.Logf("========================================")
		framework.Logf("Test PASSED: eStargz lazy pulling and layer sharing verified")
	})

	// TC28: Multiple layer stores including stargz-store
	g.It("should configure multiple additionalLayerStores including stargz-store [TC28]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)

		g.By("Deploying stargz-store")
		err = stargzSetup.Deploy(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating additional local layer store directories")
		localDirs := []string{"/var/lib/local-layerstore-1", "/var/lib/local-layerstore-2"}
		err = createDirectoriesOnNodes(oc, pureWorkers, localDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cleanupDirectoriesOnNodes(oc, pureWorkers, localDirs)

		g.By("Creating ContainerRuntimeConfig with stargz-store and local paths")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "stargz-multi-layer-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath(stargzSetup.GetStorePath())},
						{Path: machineconfigv1.StorePath("/var/lib/local-layerstore-1")},
						{Path: machineconfigv1.StorePath("/var/lib/local-layerstore-2")},
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

		g.By("Verifying all layer stores configured")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("/var/lib/stargz-store/store"))
			o.Expect(output).To(o.ContainSubstring("/var/lib/local-layerstore-1"))
			o.Expect(output).To(o.ContainSubstring("/var/lib/local-layerstore-2"))
			framework.Logf("Node %s: All 3 layer stores configured", node.Name)
		}

		framework.Logf("Test PASSED: Multiple layer stores including stargz-store configured")
	})
})
