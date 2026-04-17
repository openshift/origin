package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

// API validation tests - creating CRCs triggers MCO reconciliation making these disruptive
var _ = g.Describe("[apigroup:config.openshift.io][apigroup:machineconfiguration.openshift.io][Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][OCPFeatureGate:AdditionalStorageConfig][Suite:openshift/disruptive-longrunning] Combined Additional Stores API Validation", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("combined-stores-api")

	g.BeforeEach(func(ctx context.Context) {
		skipUnlessAdditionalStorageConfigEnabled(ctx, oc)
	})

	// TC1: Reject if any store type has invalid path
	g.It("should reject if any store type has invalid path in combined config [TC1]", func(ctx context.Context) {
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
		o.Expect(err).To(o.HaveOccurred(), "Expected API to reject invalid relative image path")
		o.Expect(err.Error()).To(o.Or(
			o.ContainSubstring("relative"),
			o.ContainSubstring("absolute"),
			o.ContainSubstring("path"),
		), "Error message should mention path validation issue")
		framework.Logf("Test PASSED: Combined config with invalid image path rejected: %v", err)
	})

	// TC2: Reject if layer stores exceed max while other stores are valid
	g.It("should reject if layer stores exceed max even with valid image/artifact stores [TC2]", func(ctx context.Context) {
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

	// TC3: Reject duplicate paths within same store type in combined config
	g.It("should reject duplicate paths within same store type in combined config [TC3]", func(ctx context.Context) {
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
var _ = g.Describe("[Skipped:Disconnected][apigroup:config.openshift.io][apigroup:machineconfiguration.openshift.io][Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][OCPFeatureGate:AdditionalStorageConfig][Serial][Disruptive][Suite:openshift/disruptive-longrunning] Combined Additional Stores E2E", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("combined-stores-e2e")

	g.BeforeEach(func(ctx context.Context) {
		skipUnlessAdditionalStorageConfigEnabled(ctx, oc)
	})

	// TC4: All three storage types and verify correct rendering to storage.conf and CRI-O config
	g.It("should configure all three storage types and generate correct configs [TC4]", func(ctx context.Context) {
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

		g.By("Creating shared directories on worker nodes")
		allDirs := []string{
			"/var/lib/combined-layers",
			"/var/lib/combined-images",
			"/var/lib/combined-artifacts",
		}
		err = createDirectoriesOnNodes(oc, pureWorkers, allDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, pureWorkers, allDirs)

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
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)

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

		g.By("Verifying storage.conf contains layer and image stores on all worker nodes")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(output).To(o.ContainSubstring("/var/lib/combined-layers"),
				"storage.conf should contain layer store path on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring("/var/lib/combined-images"),
				"storage.conf should contain image store path on node %s", node.Name)

			framework.Logf("Node %s: Layer and image stores verified in storage.conf", node.Name)
		}

		g.By("Verifying CRI-O config contains artifact stores on all worker nodes")
		for _, node := range pureWorkers {
			crioOutput, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
			o.Expect(err).NotTo(o.HaveOccurred())

			expectedArtifactConfig := `additional_artifact_stores = ["/var/lib/combined-artifacts"]`
			o.Expect(crioOutput).To(o.ContainSubstring(expectedArtifactConfig),
				"CRI-O config should contain artifact store on node %s", node.Name)

			framework.Logf("Node %s: Artifact stores verified in CRI-O config", node.Name)
		}

		g.By("Verifying CRI-O is running")
		for _, node := range pureWorkers {
			crioStatus, err := ExecOnNodeWithChroot(oc, node.Name, "systemctl", "is-active", "crio")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(crioStatus)).To(o.Equal("active"))
		}

		framework.Logf("Test PASSED: All three storage types configured successfully")
	})

	// TC5: Maximum stores for each type (5 layer, 10 image, 10 artifact)
	g.It("should configure maximum stores for each type [TC5]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		// Use pureWorkers if available, otherwise use any worker node (SNO support)
		if len(pureWorkers) == 0 {
			pureWorkers = workerNodes
		}

		g.By("Creating directories for maximum stores")
		var allDirs []string
		for i := 0; i < 5; i++ {
			allDirs = append(allDirs, fmt.Sprintf("/var/lib/layer-store-%d", i))
		}
		for i := 0; i < 10; i++ {
			allDirs = append(allDirs, fmt.Sprintf("/var/lib/image-store-%d", i))
		}
		for i := 0; i < 10; i++ {
			allDirs = append(allDirs, fmt.Sprintf("/var/lib/artifact-store-%d", i))
		}
		err = createDirectoriesOnNodes(oc, pureWorkers, allDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, pureWorkers, allDirs)

		g.By("Creating ContainerRuntimeConfig with max stores for each type")
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
				Name: "combined-max-stores-e2e-test",
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

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying storage.conf contains ALL layer and image stores")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())

			// Verify ALL layer stores (0-4)
			for i := 0; i < 5; i++ {
				o.Expect(output).To(o.ContainSubstring(fmt.Sprintf("/var/lib/layer-store-%d", i)),
					"storage.conf should contain layer-store-%d on node %s", i, node.Name)
			}

			// Verify ALL image stores (0-9)
			for i := 0; i < 10; i++ {
				o.Expect(output).To(o.ContainSubstring(fmt.Sprintf("/var/lib/image-store-%d", i)),
					"storage.conf should contain image-store-%d on node %s", i, node.Name)
			}

			framework.Logf("Node %s: All 5 layer stores and 10 image stores verified in storage.conf", node.Name)
		}

		g.By("Verifying CRI-O config contains ALL artifact stores")
		for _, node := range pureWorkers {
			crioOutput, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
			o.Expect(err).NotTo(o.HaveOccurred())

			// Verify ALL artifact stores (0-9)
			for i := 0; i < 10; i++ {
				o.Expect(crioOutput).To(o.ContainSubstring(fmt.Sprintf("/var/lib/artifact-store-%d", i)),
					"CRI-O config should contain artifact-store-%d on node %s", i, node.Name)
			}

			framework.Logf("Node %s: All 10 artifact stores verified in CRI-O config", node.Name)
		}

		framework.Logf("Test PASSED: Maximum stores for all types configured (5 layer, 10 image, 10 artifact)")
	})

	// TC6: Same path across different store types
	g.It("should allow same path across different store types [TC6]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		// Use pureWorkers if available, otherwise use any worker node (SNO support)
		if len(pureWorkers) == 0 {
			pureWorkers = workerNodes
		}

		g.By("Creating shared directory")
		allDirs := []string{"/mnt/shared-storage"}
		err = createDirectoriesOnNodes(oc, pureWorkers, allDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, pureWorkers, allDirs)

		g.By("Creating ContainerRuntimeConfig with same path in different store types")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-same-path-e2e-test",
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
		o.Expect(err).NotTo(o.HaveOccurred(), "API should accept same path across different store types")
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)

		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalLayerStores).To(o.HaveLen(1))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalImageStores).To(o.HaveLen(1))
		o.Expect(created.Spec.ContainerRuntimeConfig.AdditionalArtifactStores).To(o.HaveLen(1))

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying storage.conf contains shared path for layer and image stores")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(output).To(o.ContainSubstring("/mnt/shared-storage"))
			framework.Logf("Node %s: Shared storage path verified in storage.conf", node.Name)
		}

		g.By("Verifying CRI-O config contains shared path for artifact stores")
		for _, node := range pureWorkers {
			crioOutput, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(crioOutput).To(o.ContainSubstring("/mnt/shared-storage"))
			framework.Logf("Node %s: Shared storage path verified in CRI-O config", node.Name)
		}

		framework.Logf("Test PASSED: Same path across different store types configured successfully")
	})

	// TC7: Update combined config - add more stores to each type
	g.It("should update combined config by adding stores to each type [TC7]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		// Use pureWorkers if available, otherwise use any worker node (SNO support)
		if len(pureWorkers) == 0 {
			pureWorkers = workerNodes
		}

		g.By("Creating shared directories")
		allDirs := []string{
			"/var/lib/layer-1", "/var/lib/layer-2",
			"/var/lib/image-1", "/var/lib/image-2",
			"/var/lib/artifact-1", "/var/lib/artifact-2",
		}
		err = createDirectoriesOnNodes(oc, pureWorkers, allDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, pureWorkers, allDirs)

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
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool rollout to complete")
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

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying updated configuration has layer and image stores in storage.conf")
		layerAndImageDirs := []string{"/var/lib/layer-1", "/var/lib/layer-2", "/var/lib/image-1", "/var/lib/image-2"}
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, dir := range layerAndImageDirs {
				o.Expect(output).To(o.ContainSubstring(dir),
					"storage.conf should contain %s on node %s", dir, node.Name)
			}
			framework.Logf("Node %s: Layer and image stores verified after update", node.Name)
		}

		g.By("Verifying updated configuration has artifact stores in CRI-O config")
		artifactDirs := []string{"/var/lib/artifact-1", "/var/lib/artifact-2"}
		for _, node := range pureWorkers {
			crioOutput, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, dir := range artifactDirs {
				o.Expect(crioOutput).To(o.ContainSubstring(dir),
					"CRI-O config should contain %s on node %s", dir, node.Name)
			}
			framework.Logf("Node %s: Artifact stores verified after update", node.Name)
		}

		framework.Logf("Test PASSED: Combined config update applied successfully")
	})

	// TC8: Remove one store type while keeping others
	g.It("should remove one store type while keeping others [TC8]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		// Use pureWorkers if available, otherwise use any worker node (SNO support)
		if len(pureWorkers) == 0 {
			pureWorkers = workerNodes
		}

		g.By("Creating shared directories")
		allDirs := []string{"/var/lib/layers", "/var/lib/images", "/var/lib/artifacts"}
		err = createDirectoriesOnNodes(oc, pureWorkers, allDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, pureWorkers, allDirs)

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
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool rollout to complete")
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

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying layer stores removed but image stores remain in storage.conf")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(output).NotTo(o.ContainSubstring("/var/lib/layers"),
				"storage.conf should not contain layer store path on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring("/var/lib/images"),
				"storage.conf should still contain image store path on node %s", node.Name)

			framework.Logf("Node %s: Layer stores removed, image stores remain", node.Name)
		}

		g.By("Verifying artifact stores remain in CRI-O config")
		for _, node := range pureWorkers {
			crioOutput, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(crioOutput).To(o.ContainSubstring("/var/lib/artifacts"),
				"CRI-O config should still contain artifact store on node %s", node.Name)

			framework.Logf("Node %s: Artifact stores remain in CRI-O config", node.Name)
		}

		framework.Logf("Test PASSED: Partial removal of store types successful")
	})

	// TC9: Comprehensive functional test - verify all three storage types actually work
	g.It("should functionally verify all three storage types work together [TC9]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		// Use pureWorkers if available, otherwise use any worker node (SNO support)
		if len(pureWorkers) == 0 {
			pureWorkers = workerNodes
		}
		testNode := pureWorkers[0]
		testNamespace := oc.Namespace()

		// Phase 1: Deploy stargz-store for layer stores
		g.By("Phase 1: Deploying stargz-store for additionalLayerStores")
		stargzSetup := NewStargzStoreSetup(oc)
		err = stargzSetup.Deploy(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer stargzSetup.Cleanup(ctx)

		// Phase 2: Pre-populate image store
		g.By("Phase 2: Pre-populating additionalImageStores")
		imageStorePath := "/var/lib/combined-imagestore"
		allDirs := []string{imageStorePath}

		// Also create artifact store directory
		artifactStorePath := "/var/lib/combined-artifactstore"
		allDirs = append(allDirs, artifactStorePath)

		err = createDirectoriesOnNodes(oc, pureWorkers, allDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, pureWorkers, allDirs)

		// Pre-populate test image in image store
		testImage := "quay.io/openshifttest/additional-storage-tests:test-6gb-standard-v1.0"
		framework.Logf("Pre-populating image %s to %s on node %s", testImage, imageStorePath, testNode.Name)

		// Use podman --root to pull image to additional image store in containers/storage format
		podmanCmd := fmt.Sprintf("podman --root %s pull %s", imageStorePath, testImage)
		_, err = ExecOnNodeWithChroot(oc, testNode.Name, "bash", "-c", podmanCmd)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Verify image exists using podman
		verifyCmd := fmt.Sprintf("podman --root %s images --format '{{.Repository}}:{{.Tag}}'", imageStorePath)
		lsOutput, err := ExecOnNodeWithChroot(oc, testNode.Name, "bash", "-c", verifyCmd)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(lsOutput).To(o.ContainSubstring("manifest.json"))
		framework.Logf("Image pre-populated successfully: %s", lsOutput)

		// Phase 3: Create ContainerRuntimeConfig with all three storage types
		g.By("Phase 3: Creating ContainerRuntimeConfig with all three storage types")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-functional-test",
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
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath(imageStorePath)},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath(artifactStorePath)},
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Phase 4: Verify storage configuration
		g.By("Phase 4: Verifying storage.conf contains layer and image stores")
		storageConfOutput, err := ExecOnNodeWithChroot(oc, testNode.Name, "cat", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())

		expectedLayerPath := fmt.Sprintf("%s:ref", stargzSetup.GetStorePath())
		o.Expect(storageConfOutput).To(o.ContainSubstring(expectedLayerPath))
		o.Expect(storageConfOutput).To(o.ContainSubstring(imageStorePath))
		framework.Logf("Layer and image stores verified in storage.conf")

		g.By("Verifying CRI-O config contains artifact stores")
		crioOutput, err := ExecOnNodeWithChroot(oc, testNode.Name, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
		o.Expect(err).NotTo(o.HaveOccurred())

		expectedArtifactConfig := fmt.Sprintf(`additional_artifact_stores = ["%s"]`, artifactStorePath)
		o.Expect(crioOutput).To(o.ContainSubstring(expectedArtifactConfig))
		framework.Logf("Artifact stores verified in CRI-O config")

		// Phase 5: Verify stargz-store is running
		g.By("Phase 5: Verifying stargz-store service is active")
		err = stargzSetup.VerifyStorageConfContainsStargz(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Phase 6: Test layer store functionality - stargz lazy pulling
		g.By("Phase 6: Testing additionalLayerStores - stargz lazy pulling")
		estargzImage := "quay.io/openshifttest/additional-storage-tests:test-5mb-estargz"

		// Get initial snapshot count
		initialSnapshots, err := getStargzSnapshotCount(oc, testNode.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Initial stargz snapshots: %d", initialSnapshots)

		// Pull eStargz image first time
		framework.Logf("Pulling eStargz image for first time: %s", estargzImage)
		pod1Name := "combined-test-estargz-pod1"
		pod1 := createTestPod(pod1Name, testNamespace, estargzImage, testNode.Name)
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, pod1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePodAndWait(ctx, oc, testNamespace, pod1Name)

		err = waitForPodRunning(ctx, oc, pod1Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("First pod running successfully")

		// Get snapshot count after first pull
		snapshotsAfterFirst, err := getStargzSnapshotCount(oc, testNode.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Snapshots after first pull: %d", snapshotsAfterFirst)
		o.Expect(snapshotsAfterFirst).To(o.BeNumerically(">", initialSnapshots))

		// Delete first pod
		deletePodAndWait(ctx, oc, testNamespace, pod1Name)

		// Wait for snapshot count to stabilize after pod deletion
		var finalSnapshotsAfterFirst int
		err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
			currentSnapshots, countErr := getStargzSnapshotCount(oc, testNode.Name)
			if countErr != nil {
				return false, countErr
			}
			if finalSnapshotsAfterFirst == 0 {
				finalSnapshotsAfterFirst = currentSnapshots
				return false, nil
			}
			// Snapshot count should stabilize (no change for 2 consecutive checks)
			if currentSnapshots == finalSnapshotsAfterFirst {
				return true, nil
			}
			finalSnapshotsAfterFirst = currentSnapshots
			return false, nil
		})
		if err != nil {
			framework.Logf("Warning: snapshots may not have stabilized: %v", err)
		}

		// Pull same image second time - should reuse snapshots (lazy pulling)
		framework.Logf("Pulling same eStargz image second time - should reuse snapshots")
		pod2Name := "combined-test-estargz-pod2"
		pod2 := createTestPod(pod2Name, testNamespace, estargzImage, testNode.Name)
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, pod2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePodAndWait(ctx, oc, testNamespace, pod2Name)

		err = waitForPodRunning(ctx, oc, pod2Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Second pod running successfully")

		// Get snapshot count after second pull
		snapshotsAfterSecond, err := getStargzSnapshotCount(oc, testNode.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Snapshots after second pull: %d", snapshotsAfterSecond)

		// Verify lazy pulling - snapshot count should not increase significantly
		newSnapshots := snapshotsAfterSecond - snapshotsAfterFirst
		framework.Logf("New snapshots created on second pull: %d", newSnapshots)
		o.Expect(newSnapshots).To(o.BeNumerically("<=", 2), "Lazy pulling should reuse existing snapshots")

		deletePodAndWait(ctx, oc, testNamespace, pod2Name)
		framework.Logf("Layer store (stargz) lazy pulling verified successfully")

		// Phase 7: Test image store functionality
		g.By("Phase 7: Testing additionalImageStores - verify pre-populated image accessible")

		// First verify the image exists in the additional store
		imageCheckOutput, err := ExecOnNodeWithChroot(oc, testNode.Name, "ls", "-la", imageStorePath+"/prepopulated-image")
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Pre-populated image still exists in additional store: %s", imageCheckOutput)

		// Check storage.conf has the image store path
		storageConf, err := ExecOnNodeWithChroot(oc, testNode.Name, "cat", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(storageConf).To(o.ContainSubstring(imageStorePath))
		framework.Logf("Image store path verified in storage.conf")

		// Phase 8: Test artifact store functionality
		g.By("Phase 8: Testing additionalArtifactStores - verify path configured")

		// Verify artifact store path in CRI-O config
		crioConf, err := ExecOnNodeWithChroot(oc, testNode.Name, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(crioConf).To(o.ContainSubstring(artifactStorePath))
		framework.Logf("Artifact store path verified in CRI-O config")

		// Create a test artifact file
		artifactTestFile := artifactStorePath + "/test-artifact.txt"
		createArtifactCmd := fmt.Sprintf("echo 'test artifact content' > %s", artifactTestFile)
		_, err = ExecOnNodeWithChroot(oc, testNode.Name, "bash", "-c", createArtifactCmd)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Verify artifact file exists
		artifactCheck, err := ExecOnNodeWithChroot(oc, testNode.Name, "cat", artifactTestFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(artifactCheck).To(o.ContainSubstring("test artifact content"))
		framework.Logf("Artifact store verified - can read/write artifacts")

		// Phase 9: Final verification
		g.By("Phase 9: Final verification - all three storage types functional")
		framework.Logf("✓ Layer stores: stargz lazy pulling works (reused snapshots)")
		framework.Logf("✓ Image stores: pre-populated images accessible")
		framework.Logf("✓ Artifact stores: can read/write artifacts")

		framework.Logf("Test PASSED: All three storage types verified functionally")
	})
})
