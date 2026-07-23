package node

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

// IsAdditionalStorageConfigEnabled checks whether the AdditionalStorageConfig
// feature gate is enabled on the cluster and the platform is supported.
// Returns false with a reason string if the test should be skipped.
func IsAdditionalStorageConfigEnabled(ctx context.Context, oc *exutil.CLI) (bool, string) {
	isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
	if err != nil {
		return false, fmt.Sprintf("cannot verify cluster type: %v", err)
	}
	if isMicroShift {
		return false, "MicroShift cluster - MachineConfig resources are not available"
	}

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Sprintf("cannot verify platform type: %v", err)
	}
	if infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.Type == configv1.AzurePlatformType {
		return false, "Microsoft Azure cluster"
	}

	fgs, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Sprintf("cannot verify FeatureGate: %v", err)
	}
	for _, fg := range fgs.Status.FeatureGates {
		for _, enabledFG := range fg.Enabled {
			if enabledFG.Name == "AdditionalStorageConfig" {
				return true, ""
			}
		}
	}
	return false, "AdditionalStorageConfig feature gate is not enabled"
}

// API validation tests - use DryRun to avoid triggering MCO reconciliation
var _ = g.Describe("[apigroup:config.openshift.io][apigroup:machineconfiguration.openshift.io][Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][OCPFeatureGate:AdditionalStorageConfig][Suite:openshift/conformance/parallel] Additional Storage API Validation", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-storage-api")

	g.BeforeEach(func(ctx context.Context) {
		if enabled, reason := IsAdditionalStorageConfigEnabled(ctx, oc); !enabled {
			g.Skip(reason)
		}
	})

	// ========================================================================
	// Combined Additional Stores API Validation
	// ========================================================================
	g.Context("Combined Additional Stores", func() {
		// Reject if any store type has invalid path
		g.It("should reject if any store type has invalid path in combined config ", func(ctx context.Context) {
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

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
			o.Expect(err).To(o.HaveOccurred(), "Expected API to reject invalid relative image path")
			o.Expect(err.Error()).To(o.Or(
				o.ContainSubstring("relative"),
				o.ContainSubstring("absolute"),
				o.ContainSubstring("path"),
			), "Error message should mention path validation issue")
			framework.Logf("Test PASSED: Combined config with invalid image path rejected: %v", err)
		})

		// Reject if layer stores exceed max while other stores are valid
		g.It("should reject if layer stores exceed max even with valid image/artifact stores ", func(ctx context.Context) {
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

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring("must have at most 5 items"))
			framework.Logf("Test PASSED: Exceeding layer store max rejected: %v", err)
		})

		// Reject duplicate paths within same store type in combined config
		g.It("should reject duplicate paths within same store type in combined config ", func(ctx context.Context) {
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

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring("duplicate"))
			framework.Logf("Test PASSED: Duplicate paths in same store type rejected: %v", err)
		})
	})

	// ========================================================================
	// Additional Layer Stores API Validation
	// ========================================================================
	g.Context("Additional Layer Stores", func() {
		// Should fail if additionalLayerStores path is empty
		// Note: Go API returns "Required value" while YAML returns "at least 1 chars long"
		g.It("should reject empty path for additionalLayerStores ", func(ctx context.Context) {
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

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(
				ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
			)
			o.Expect(err).To(o.HaveOccurred())
			framework.Logf("Expected substring: 'Required value' (Go API) or 'at least 1 chars long' (YAML)")
			framework.Logf("Actual error: %v", err)
			o.Expect(err.Error()).To(o.ContainSubstring("Required value"))
			framework.Logf("Test PASSED: Empty path correctly rejected")
		})

		// Should fail if additionalLayerStores path is not absolute
		g.It("should reject relative path for additionalLayerStores ", func(ctx context.Context) {
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

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(
				ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
			)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring("path must be absolute and contain only alphanumeric characters"))
			framework.Logf("Test PASSED: Relative path correctly rejected: %v", err)
		})

		// Should fail if additionalLayerStores path contains spaces
		g.It("should reject path with spaces for additionalLayerStores ", func(ctx context.Context) {
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

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(
				ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
			)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring("path must be absolute and contain only alphanumeric characters"))
			framework.Logf("Test PASSED: Path with spaces correctly rejected: %v", err)
		})

		// Should fail if additionalLayerStores path contains invalid characters
		g.It("should reject path with invalid characters for additionalLayerStores ", func(ctx context.Context) {
			mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating ContainerRuntimeConfig with path containing @ symbol")
			ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "layer-invalid-char-at-symbol-test",
				},
				Spec: machineconfigv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pools.operator.machineconfiguration.openshift.io/worker": "",
						},
					},
					ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
						AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
							{Path: machineconfigv1.StorePath("/var/lib/stargz@store")},
						},
					},
				},
			}

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(
				ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
			)
			o.Expect(err).To(o.HaveOccurred(), "Expected API to reject path with invalid character '@'")
			framework.Logf("Path with '@' correctly rejected: %v", err)
		})

		// Should fail if additionalLayerStores path is too long (>256 bytes)
		g.It("should reject path exceeding 256 characters for additionalLayerStores ", func(ctx context.Context) {
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

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(
				ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
			)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.Or(o.ContainSubstring("256"), o.ContainSubstring("Too long")))
			framework.Logf("Test PASSED: Long path correctly rejected: %v", err)
		})

		// Should fail if additionalLayerStores exceeds maximum of 5 items
		g.It("should reject more than 5 additionalLayerStores ", func(ctx context.Context) {
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

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(
				ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
			)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring("must have at most 5 items"))
			framework.Logf("Test PASSED: 6 layer stores correctly rejected: %v", err)
		})

		// Should fail if additionalLayerStores path contains consecutive forward slashes
		g.It("should reject path with consecutive forward slashes for additionalLayerStores ", func(ctx context.Context) {
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

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(
				ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
			)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring("consecutive"))
			framework.Logf("Test PASSED: Consecutive slashes correctly rejected: %v", err)
		})

		// Should fail if additionalLayerStores contains duplicate paths
		g.It("should reject duplicate paths in additionalLayerStores ", func(ctx context.Context) {
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

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(
				ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
			)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring("duplicate"))
			framework.Logf("Test PASSED: Duplicate paths correctly rejected: %v", err)
		})
	})

	// ========================================================================
	// Additional Image Stores API Validation
	// ========================================================================
	g.Context("Additional Image Stores", func() {
		// Smoke test - validates that path validation is wired up for additionalImageStores
		g.It("should reject invalid additionalImageStores path [Validation Smoke Test]", func(ctx context.Context) {
			mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating ContainerRuntimeConfig with invalid path (contains special character)")
			ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "imagestore-validation-smoke-test",
				},
				Spec: machineconfigv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pools.operator.machineconfiguration.openshift.io/worker": "",
						},
					},
					ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
						AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
							{Path: machineconfigv1.StorePath("/var/lib/image@store")},
						},
					},
				},
			}

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(
				ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
			)
			o.Expect(err).To(o.HaveOccurred(), "Expected validation to reject invalid path")
			framework.Logf("Smoke test PASSED: Validation correctly rejected invalid path: %v", err)
		})
	})

	// ========================================================================
	// Additional Artifact Stores API Validation
	// ========================================================================
	g.Context("Additional Artifact Stores", func() {
		// Smoke test - validates that path validation is wired up for additionalArtifactStores
		g.It("should reject invalid additionalArtifactStores path [Validation Smoke Test]", func(ctx context.Context) {
			mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating ContainerRuntimeConfig with invalid path (contains special character)")
			ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "artifactstore-validation-smoke-test",
				},
				Spec: machineconfigv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pools.operator.machineconfiguration.openshift.io/worker": "",
						},
					},
					ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
						AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
							{Path: machineconfigv1.StorePath("/var/lib/artifact@store")},
						},
					},
				},
			}

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(
				ctx, ctrcfg, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
			)
			o.Expect(err).To(o.HaveOccurred(), "Expected validation to reject invalid path")
			framework.Logf("Smoke test PASSED: Validation correctly rejected invalid path: %v", err)
		})
	})
})
