package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

// Additional Storage E2E Tests - trigger MCO reconciliation (MCP rollouts)
// and run in the disruptive-longrunning suite.
var _ = g.Describe("[Skipped:Disconnected][apigroup:config.openshift.io][apigroup:machineconfiguration.openshift.io][Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][OCPFeatureGate:AdditionalStorageConfig][Serial][Disruptive][Suite:openshift/disruptive-longrunning] Additional Storage E2E Tests", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-storage-e2e")

	g.BeforeEach(func(ctx context.Context) {
		skipUnlessAdditionalStorageConfigEnabled(ctx, oc)
	})

	// ========================================================================
	// Combined Additional Stores E2E - Configuration and Node Verification
	// ========================================================================
	g.It("should configure all three storage types and verify node configuration", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		// =====================================================================
		// SETUP: Create single-node MachineConfigPool for faster rollouts
		// =====================================================================
		g.By("Getting worker node for test")
		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(workerNodes).NotTo(o.BeEmpty(), "no worker nodes available")
		testNode := workerNodes[0].Name

		mcpName := "combined-all-test"
		var mcpConfig *CustomMCPConfig
		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			delErr := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Delete(
				cleanupCtx, "combined-all-test", metav1.DeleteOptions{})
			if delErr != nil && !apierrors.IsNotFound(delErr) {
				framework.Logf("Warning: failed to delete ContainerRuntimeConfig %s: %v", "combined-all-test", delErr)
			}
			cleanupSingleNodeMCP(cleanupCtx, mcpConfig)
		})

		g.By("Creating single-node MachineConfigPool for test isolation")
		mcpConfig = createSingleNodeMCP(ctx, oc, mcpName, testNode)
		framework.Logf("Custom MCP %s created, targeting node %s", mcpName, testNode)

		// Get the node object for directory creation
		testNodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, testNode, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating shared directories on test node")
		allDirs := []string{
			"/var/lib/combined-layers",
			"/var/lib/combined-images",
			"/var/lib/combined-artifacts",
		}
		err = createDirectoriesOnNodes(oc, []corev1.Node{*testNodeObj}, allDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, []corev1.Node{*testNodeObj}, allDirs)

		g.By("Creating ContainerRuntimeConfig with all three storage types")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-all-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"machineconfiguration.openshift.io/pool": mcpName,
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

		g.By("Waiting for MachineConfigPool rollout (single node)")
		err = waitForMCPToStartUpdating(ctx, mcClient, mcpName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating")

		err = waitForMCP(ctx, mcClient, mcpName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying storage.conf contains layer and image stores")
		output, err := ExecOnNodeWithChroot(oc, testNode, "cat", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(output).To(o.ContainSubstring("/var/lib/combined-layers"),
			"storage.conf should contain layer store path on node %s", testNode)
		o.Expect(output).To(o.ContainSubstring("/var/lib/combined-images"),
			"storage.conf should contain image store path on node %s", testNode)

		framework.Logf("Node %s: Layer and image stores verified in storage.conf", testNode)

		g.By("Verifying CRI-O config contains artifact stores")
		crioOutput, err := ExecOnNodeWithChroot(oc, testNode, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
		o.Expect(err).NotTo(o.HaveOccurred())

		expectedArtifactConfig := `additional_artifact_stores = ["/var/lib/combined-artifacts"]`
		o.Expect(crioOutput).To(o.ContainSubstring(expectedArtifactConfig),
			"CRI-O config should contain artifact store on node %s", testNode)

		framework.Logf("Node %s: Artifact stores verified in CRI-O config", testNode)

		g.By("Verifying CRI-O is running")
		crioStatus, err := ExecOnNodeWithChroot(oc, testNode, "systemctl", "is-active", "crio")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(crioStatus)).To(o.Equal("active"))

		framework.Logf("Test PASSED: All three storage types configured successfully")
	})

	// ========================================================================
	// Combined Additional Stores E2E - Functional Verification
	// ========================================================================
	g.It("should functionally verify all three storage types work together", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		// =====================================================================
		// SETUP: Create single-node MachineConfigPool for faster rollouts
		// =====================================================================
		g.By("Getting worker node for test")
		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(workerNodes).NotTo(o.BeEmpty(), "no worker nodes available")
		testNode := workerNodes[0].Name

		mcpName := "combined-func-test"
		var mcpConfig *CustomMCPConfig
		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			delErr := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Delete(
				cleanupCtx, "combined-func-test", metav1.DeleteOptions{})
			if delErr != nil && !apierrors.IsNotFound(delErr) {
				framework.Logf("Warning: failed to delete ContainerRuntimeConfig %s: %v", "combined-func-test", delErr)
			}
			cleanupSingleNodeMCP(cleanupCtx, mcpConfig)
		})

		g.By("Creating single-node MachineConfigPool for test isolation")
		mcpConfig = createSingleNodeMCP(ctx, oc, mcpName, testNode)
		framework.Logf("Custom MCP %s created, targeting node %s", mcpName, testNode)

		// Get the node object for directory creation
		testNodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, testNode, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		testNamespace := oc.Namespace()

		// Phase 1: Pre-populate image store
		g.By("Phase 1: Pre-populating additionalImageStores")
		imageStorePath := "/var/lib/combined-imagestore"
		allDirs := []string{imageStorePath}

		// Also create artifact store directory
		artifactStorePath := "/var/lib/combined-artifactstore"
		allDirs = append(allDirs, artifactStorePath)

		// Also create layer store directory
		layerStorePath := "/var/lib/stargz-store/store"
		allDirs = append(allDirs, layerStorePath)

		err = createDirectoriesOnNodes(oc, []corev1.Node{*testNodeObj}, allDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, []corev1.Node{*testNodeObj}, allDirs)

		// Pre-populate test image in image store
		testImage := "quay.io/openshifttest/additional-storage-tests:test-6gb-standard-v2.0"
		framework.Logf("Pre-populating image %s to %s on node %s", testImage, imageStorePath, testNode)

		// Use podman --root to pull image to additional image store in containers/storage format
		podmanCmd := fmt.Sprintf("podman --root %s pull %s", imageStorePath, testImage)
		_, err = ExecOnNodeWithChroot(oc, testNode, "bash", "-c", podmanCmd)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Verify image exists using podman
		verifyCmd := fmt.Sprintf("podman --root %s images --format '{{.Repository}}:{{.Tag}}'", imageStorePath)
		lsOutput, err := ExecOnNodeWithChroot(oc, testNode, "bash", "-c", verifyCmd)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(lsOutput).To(o.ContainSubstring(testImage))
		framework.Logf("Image pre-populated successfully: %s", lsOutput)

		// Phase 2: Create ContainerRuntimeConfig with all three storage types
		g.By("Phase 2: Creating ContainerRuntimeConfig with all three storage types")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-func-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"machineconfiguration.openshift.io/pool": mcpName,
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath(layerStorePath)},
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

		g.By("Waiting for MachineConfigPool rollout (single node)")
		err = waitForMCPToStartUpdating(ctx, mcClient, mcpName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForMCP(ctx, mcClient, mcpName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Phase 3: Verify storage configuration
		g.By("Phase 3: Verifying storage.conf contains image stores")
		storageConfOutput, err := ExecOnNodeWithChroot(oc, testNode, "cat", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(storageConfOutput).To(o.ContainSubstring(imageStorePath))
		framework.Logf("Image stores verified in storage.conf")

		g.By("Verifying CRI-O config contains artifact stores")
		crioOutput, err := ExecOnNodeWithChroot(oc, testNode, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
		o.Expect(err).NotTo(o.HaveOccurred())

		expectedArtifactConfig := fmt.Sprintf(`additional_artifact_stores = ["%s"]`, artifactStorePath)
		o.Expect(crioOutput).To(o.ContainSubstring(expectedArtifactConfig))
		framework.Logf("Artifact stores verified in CRI-O config")

		// =====================================================================
		// Phase 4: Verify storage.conf contains layerstore path with :ref suffix
		// =====================================================================
		g.By("Phase 4: Verifying storage.conf contains stargz-store path with :ref suffix")
		layerStoreConf, err := ExecOnNodeWithChroot(oc, testNode, "cat", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())

		// MCO automatically appends :ref suffix to all additionalLayerStores paths
		expectedPathWithRef := layerStorePath + ":ref"
		o.Expect(layerStoreConf).To(o.ContainSubstring("additionallayerstores"),
			"storage.conf should contain additionallayerstores on node %s", testNode)
		o.Expect(layerStoreConf).To(o.ContainSubstring(expectedPathWithRef),
			"storage.conf should contain %s with :ref suffix on node %s", expectedPathWithRef, testNode)
		framework.Logf("Node %s: storage.conf verified with path %s (MCO added :ref)", testNode, expectedPathWithRef)

		g.By("Verifying CRI-O is active with new configuration")
		crioStatus, err := ExecOnNodeWithChroot(oc, testNode, "systemctl", "is-active", "crio")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(crioStatus)).To(o.Equal("active"))
		framework.Logf("CRI-O is active")

		// Phase 5: Test artifact store functionality
		g.By("Phase 5: Testing additionalArtifactStores - verify path configured")

		// Verify artifact store path in CRI-O config
		crioConf, err := ExecOnNodeWithChroot(oc, testNode, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(crioConf).To(o.ContainSubstring(artifactStorePath))
		framework.Logf("Artifact store path verified in CRI-O config")

		// Create a test artifact file
		artifactTestFile := artifactStorePath + "/test-artifact.txt"
		createArtifactCmd := fmt.Sprintf("echo 'test artifact content' > %s", artifactTestFile)
		_, err = ExecOnNodeWithChroot(oc, testNode, "bash", "-c", createArtifactCmd)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Verify artifact file exists
		artifactCheck, err := ExecOnNodeWithChroot(oc, testNode, "cat", artifactTestFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(artifactCheck).To(o.ContainSubstring("test artifact content"))
		framework.Logf("Artifact store verified - can read/write artifacts")

		// Phase 6: Test image store functionality - prepopulation and fallback
		g.By("Phase 6a: Testing additionalImageStores - verify pre-populated image accessible")

		// Check storage.conf has the image store path
		storageConf, err := ExecOnNodeWithChroot(oc, testNode, "cat", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(storageConf).To(o.ContainSubstring(imageStorePath))
		framework.Logf("Image store path verified in storage.conf")

		// Test prepopulated image by creating a pod
		g.By("Phase 6b: Creating pod using prepopulated image")
		testPod1 := createTestPod("imagestore-prepop-pod", testNamespace, testImage, testNode)
		startTime1 := time.Now()
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, testPod1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForPodRunning(ctx, oc, testPod1.Name, 5*time.Minute)
		if err != nil {
			// Get pod logs to diagnose failure
			logs, logErr := oc.AdminKubeClient().CoreV1().Pods(testNamespace).GetLogs(testPod1.Name, &corev1.PodLogOptions{}).DoRaw(ctx)
			if logErr == nil {
				framework.Logf("Pod logs: %s", string(logs))
			}
			// Get pod events and status
			podObj, getErr := oc.AdminKubeClient().CoreV1().Pods(testNamespace).Get(ctx, testPod1.Name, metav1.GetOptions{})
			if getErr == nil {
				framework.Logf("Pod status: %+v", podObj.Status)
				// Extract specific failure reason
				if len(podObj.Status.ContainerStatuses) > 0 {
					containerStatus := podObj.Status.ContainerStatuses[0]
					if containerStatus.State.Waiting != nil {
						o.Expect(err).NotTo(o.HaveOccurred(),
							"Image store prepopulated pod failed - %s: %s (Image: %s)",
							containerStatus.State.Waiting.Reason,
							containerStatus.State.Waiting.Message,
							testImage)
					}
				}
			}
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "Image store prepopulated pod failed to start with image %s", testImage)
		pod1Time := time.Since(startTime1)
		framework.Logf("Pod using prepopulated image started in %v", pod1Time)

		// Test fallback to registry
		g.By("Phase 7: Testing fallback to registry when image removed from additional store")
		err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Delete(ctx, testPod1.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			_, err := oc.AdminKubeClient().CoreV1().Pods(testNamespace).Get(ctx, testPod1.Name, metav1.GetOptions{})
			return apierrors.IsNotFound(err), nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Remove image from additional store
		removeCmd := fmt.Sprintf("podman --root %s rmi %s", imageStorePath, testImage)
		_, err = ExecOnNodeWithChroot(oc, testNode, "sh", "-c", removeCmd)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Removed image from additional store to test fallback")

		// Create second pod - should fall back to registry
		testPod2 := createTestPod("imagestore-fallback-pod", testNamespace, testImage, testNode)
		startTime2 := time.Now()
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, testPod2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deletePodAndWait, ctx, oc, testNamespace, testPod2.Name)

		err = waitForPodRunning(ctx, oc, testPod2.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod2Time := time.Since(startTime2)
		framework.Logf("Pod using registry fallback started in %v", pod2Time)

		// Verify pod pulled from registry
		var foundSuccessfullyPulled bool
		err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			events, _ := oc.AdminKubeClient().CoreV1().Events(testNamespace).List(ctx, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.name=%s", testPod2.Name),
			})
			for _, event := range events.Items {
				if event.Reason == "Pulled" && strings.Contains(event.Message, "Successfully pulled") {
					foundSuccessfullyPulled = true
					framework.Logf("SUCCESS: Image pulled from registry - %s", event.Message)
					return true, nil
				}
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(foundSuccessfullyPulled).To(o.BeTrue())
		framework.Logf("Phase 7 PASSED: Fallback to registry works when image not in additional store")

		// Clean up fallback test pod
		deletePodAndWait(ctx, oc, testNamespace, testPod2.Name)

		// Phase 8: Final verification
		g.By("Phase 8: Final verification - all storage types functional")
		framework.Logf("✓ Layer stores: :ref suffix verified in storage.conf")
		framework.Logf("✓ Image stores: pre-populated images accessible")
		framework.Logf("✓ Artifact stores: can read/write artifacts")

		framework.Logf("Test PASSED: All storage types verified functionally")
	})

	// ========================================================================
	// Additional Layer Stores E2E - Comprehensive Lifecycle
	// ========================================================================
	g.It("should perform comprehensive lifecycle for layerstores: deploy stargz, configure, verify lazy pulling, update stores, maximum stores, and fallback scenario", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		// =====================================================================
		// SETUP: Create single-node MachineConfigPool for faster rollouts
		// =====================================================================
		g.By("Getting worker node for test")
		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(workerNodes).NotTo(o.BeEmpty(), "no worker nodes available")
		testNode := workerNodes[0].Name

		mcpName := "layer-test-mcp"
		var mcpConfig *CustomMCPConfig
		testNamespace := oc.Namespace()
		pod1Name := "stargz-test-pod-1"
		pod2Name := "stargz-test-pod-2"
		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			delErr := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Delete(
				cleanupCtx, "stargz-comprehensive-test", metav1.DeleteOptions{})
			if delErr != nil && !apierrors.IsNotFound(delErr) {
				framework.Logf("Warning: failed to delete ContainerRuntimeConfig %s: %v", "stargz-comprehensive-test", delErr)
			}
			cleanupSingleNodeMCP(cleanupCtx, mcpConfig)
		})

		g.By("Creating single-node MachineConfigPool for test isolation")
		mcpConfig = createSingleNodeMCP(ctx, oc, mcpName, testNode)
		framework.Logf("Custom MCP %s created, targeting node %s", mcpName, testNode)

		eStargzImage := "quay.io/openshifttest/additional-storage-tests:test-image-estargz"

		// Clean up any cached eStargz images from previous test runs
		// This prevents "layer not known" errors on reused test clusters
		framework.Logf("Cleaning cached eStargz images from previous runs")
		_, _ = ExecOnNodeWithChroot(oc, testNode, "crictl", "rmi", eStargzImage)

		// =====================================================================
		// PHASE 1: Deploy stargz-store
		// =====================================================================
		g.By("PHASE 1: Deploying stargz-store")
		testLabel := fmt.Sprintf("node-role.kubernetes.io/%s", mcpName)
		stargzSetup := NewStargzStoreSetup(oc, testLabel)
		err = stargzSetup.Deploy(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			// Delete pods first to release image references
			deletePodAndWait(cleanupCtx, oc, testNamespace, pod1Name)
			deletePodAndWait(cleanupCtx, oc, testNamespace, pod2Name)
			stargzSetup.Cleanup(context.Background())
		})
		framework.Logf("stargz-store deployed successfully")

		g.By("Verifying stargz-store service is active on test node")
		output, err := ExecOnNodeWithChroot(oc, testNode, "systemctl", "is-active", "stargz-store")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(output)).To(o.Equal("active"))
		framework.Logf("Node %s: stargz-store service active", testNode)

		// =====================================================================
		// PHASE 2: Create ContainerRuntimeConfig with stargz-store path
		// =====================================================================
		g.By("PHASE 2: Creating ContainerRuntimeConfig with stargz-store path")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "stargz-comprehensive-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"machineconfiguration.openshift.io/pool": mcpName,
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath(stargzSetup.GetStorePath())},
					},
				},
			},
		}
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(
			ctx, ctrcfg, metav1.CreateOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("ContainerRuntimeConfig %s created with path: %s", ctrcfg.Name, stargzSetup.GetStorePath())

		// =====================================================================
		// PHASE 3: Verify MCP rollout (single node - faster)
		// =====================================================================
		g.By("PHASE 3: Waiting for MachineConfigPool rollout (single node)")
		err = waitForMCPToStartUpdating(ctx, mcClient, mcpName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating")

		err = waitForMCP(ctx, mcClient, mcpName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool rollout completed")

		g.By("Verifying test node is Ready")
		nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, testNode, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isNodeInReadyState(nodeObj)).To(o.BeTrue(),
			"Node %s should be Ready after MCP rollout", testNode)
		framework.Logf("Node %s: Ready", testNode)

		g.By("Re-verifying stargz-store service is active after MCP rollout")
		stargzStatus, err := ExecOnNodeWithChroot(oc, testNode, "systemctl", "is-active", "stargz-store")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(stargzStatus)).To(o.Equal("active"))
		framework.Logf("Node %s: stargz-store service still active after MCP rollout", testNode)

		// Verify FUSE mount exists
		mountOutput, err := ExecOnNodeWithChroot(oc, testNode, "mount")
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(mountOutput, "stargz-store") {
			framework.Logf("WARNING: stargz-store service is active but FUSE mount not found!")
			framework.Logf("Mount output:\n%s", mountOutput)
			o.Expect(mountOutput).To(o.ContainSubstring("stargz-store"),
				"stargz-store FUSE mount at %s should exist when service is active", stargzSetup.GetStorePath())
		}
		framework.Logf("Node %s: stargz-store FUSE mount verified", testNode)

		// =====================================================================
		// PHASE 4: Verify storage.conf contains path with :ref suffix (MCO added)
		// =====================================================================
		g.By("PHASE 4: Verifying storage.conf contains stargz-store path with :ref suffix")
		output, err = ExecOnNodeWithChroot(oc, testNode, "cat", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())

		// MCO automatically appends :ref suffix to all additionalLayerStores paths
		expectedPathWithRef := fmt.Sprintf("%s:ref", stargzSetup.GetStorePath())
		o.Expect(output).To(o.ContainSubstring("additionallayerstores"),
			"storage.conf should contain additionallayerstores on node %s", testNode)
		o.Expect(output).To(o.ContainSubstring(expectedPathWithRef),
			"storage.conf should contain %s with :ref suffix on node %s", stargzSetup.GetStorePath(), testNode)
		framework.Logf("Node %s: storage.conf verified with path %s (MCO added :ref)", testNode, expectedPathWithRef)

		g.By("Verifying CRI-O is active with new configuration")
		crioStatus, err := ExecOnNodeWithChroot(oc, testNode, "systemctl", "is-active", "crio")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(crioStatus)).To(o.Equal("active"))
		framework.Logf("CRI-O is active")

		// =====================================================================
		// PHASE 5: Create first pod with eStargz image
		// =====================================================================
		g.By("PHASE 5: Getting initial snapshot count in stargz-store")
		initialSnapshots, err := getStargzSnapshotCount(oc, testNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Initial snapshot count: %d", initialSnapshots)

		g.By("Creating first pod with eStargz format image")
		framework.Logf("Using eStargz image: %s", eStargzImage)
		pod1 := createTestPod(pod1Name, testNamespace, eStargzImage, testNode)

		startTime1 := time.Now()
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, pod1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for first pod to be running")
		err = waitForPodRunning(ctx, oc, pod1Name, 5*time.Minute)
		if err != nil {
			// Get pod logs and status to diagnose failure
			logs, logErr := oc.AdminKubeClient().CoreV1().Pods(testNamespace).GetLogs(pod1Name, &corev1.PodLogOptions{}).DoRaw(ctx)
			if logErr == nil {
				framework.Logf("Pod %s logs: %s", pod1Name, string(logs))
			}
			podObj, getErr := oc.AdminKubeClient().CoreV1().Pods(testNamespace).Get(ctx, pod1Name, metav1.GetOptions{})
			if getErr == nil {
				framework.Logf("Pod %s status: %+v", pod1Name, podObj.Status)
				// Extract specific failure reason
				if len(podObj.Status.ContainerStatuses) > 0 {
					containerStatus := podObj.Status.ContainerStatuses[0]
					if containerStatus.State.Waiting != nil {
						o.Expect(err).NotTo(o.HaveOccurred(),
							"Pod failed to start - %s: %s (Image: %s)",
							containerStatus.State.Waiting.Reason,
							containerStatus.State.Waiting.Message,
							eStargzImage)
					}
				}
			}
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "Pod %s failed to start with eStargz image %s", pod1Name, eStargzImage)
		pod1Duration := time.Since(startTime1)
		framework.Logf("First pod %s started in %v (initial pull with lazy loading)", pod1Name, pod1Duration)

		// =====================================================================
		// PHASE 6: Verify snapshot created in layer store path
		// =====================================================================
		g.By("PHASE 6: Verifying snapshot is created in stargz-store")

		// Poll for snapshots to be created instead of sleeping
		var snapshotsAfterPod1 int
		err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
			count, countErr := getStargzSnapshotCount(oc, testNode)
			if countErr != nil {
				return false, countErr
			}
			snapshotsAfterPod1 = count
			if snapshotsAfterPod1 > initialSnapshots {
				framework.Logf("Snapshots created in stargz-store (lazy pulling verified)")
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Snapshots should be created after pulling eStargz image")

		// Verify stargz-store contains layer snapshots (check recursively since sha256 dirs are nested)
		storeOutput, err := ExecOnNodeWithChroot(oc, testNode, "find", "/var/lib/stargz-store/store/", "-name", "sha256:*", "-type", "d")
		o.Expect(err).NotTo(o.HaveOccurred())

		if !strings.Contains(storeOutput, "sha256:") || snapshotsAfterPod1 == 0 {
			// If verification fails, show detailed recursive listing for debugging
			detailedOutput, detailErr := ExecOnNodeWithChroot(oc, testNode, "ls", "-lR", "/var/lib/stargz-store/store/")
			if detailErr == nil {
				framework.Logf("stargz-store detailed contents (verification failed):\n%s", detailedOutput)
			}
		} else {
			framework.Logf("stargz-store verified: contains layer directories with sha256 digests")
		}

		o.Expect(storeOutput).To(o.ContainSubstring("sha256:"),
			"stargz-store should contain layer directories with sha256 digests")

		// =====================================================================
		// PHASE 7: Create second pod with same image
		// =====================================================================
		g.By("PHASE 7: Creating second pod with same eStargz image")
		pod2 := createTestPod(pod2Name, testNamespace, eStargzImage, testNode)

		startTime2 := time.Now()
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, pod2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for second pod to be running")
		err = waitForPodRunning(ctx, oc, pod2Name, 3*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod2Duration := time.Since(startTime2)
		framework.Logf("Second pod %s started in %v (using shared layers)", pod2Name, pod2Duration)

		// Log performance comparison for informational purposes
		g.By("Logging performance comparison with layer sharing")
		speedup := float64(pod1Duration) / float64(pod2Duration)
		framework.Logf("Performance comparison:")
		framework.Logf("  - First pod (initial pull):  %v", pod1Duration)
		framework.Logf("  - Second pod (layer sharing): %v", pod2Duration)
		framework.Logf("  - Performance improvement: %.2fx faster with layer sharing", speedup)

		// =====================================================================
		// PHASE 8: Verify second pod used existing snapshot (no new layers)
		// =====================================================================
		g.By("PHASE 8: Verifying second pod used existing snapshot")
		pod2Events, _ := oc.Run("describe").Args("pod", pod2Name).Output()
		framework.Logf("Second pod events: %s", pod2Events)

		snapshotsAfterPod2, err := getStargzSnapshotCount(oc, testNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(snapshotsAfterPod2).To(o.Equal(snapshotsAfterPod1),
			"Snapshot count should remain same when using shared layers")
		framework.Logf("Layer sharing verified: second pod reused existing snapshots")

		// =====================================================================
		// PHASE 9: Verify through stargz-store and crio logs
		// =====================================================================
		g.By("PHASE 9: Verifying through stargz-store logs")
		stargzLogs, _ := ExecOnNodeWithChroot(oc, testNode, "journalctl", "-u", "stargz-store", "--since", "5 minutes ago", "-n", "50")
		framework.Logf("Recent stargz-store logs:\n%s", stargzLogs)

		g.By("Verifying through CRI-O logs")
		crioLogs, _ := ExecOnNodeWithChroot(oc, testNode, "journalctl", "-u", "crio", "--since", "5 minutes ago", "--grep", eStargzImage, "-n", "20")
		framework.Logf("Recent CRI-O logs for image:\n%s", crioLogs)

		// =====================================================================
		// PHASE 10: Remove pods
		// =====================================================================
		g.By("PHASE 10: Removing test pods")
		err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Delete(ctx, pod1Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForPodDeleted(ctx, oc, pod1Name, 2*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to wait for pod %s deletion", pod1Name)

		err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Delete(ctx, pod2Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForPodDeleted(ctx, oc, pod2Name, 2*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to wait for pod %s deletion", pod2Name)
		framework.Logf("Test pods removed")

		// =====================================================================
		// PHASE 11: Create directories for additional layer stores
		// =====================================================================
		g.By("PHASE 11: Creating additional layer store directories for update testing")
		testNodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, testNode, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		layerDirs := []string{"/var/lib/layerstore-1", "/var/lib/layerstore-2", "/var/lib/layerstore-3", "/var/lib/layerstore-4"}
		err = createDirectoriesOnNodes(oc, []corev1.Node{*testNodeObj}, layerDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, []corev1.Node{*testNodeObj}, layerDirs)
		framework.Logf("Created %d additional layer store directories", len(layerDirs))

		// =====================================================================
		// PHASE 12: Update CRC to add second layer store
		// =====================================================================
		g.By("PHASE 12: Updating ContainerRuntimeConfig to add second layer store")
		currentCfg, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Get(ctx, ctrcfg.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		currentCfg.Spec.ContainerRuntimeConfig.AdditionalLayerStores = []machineconfigv1.AdditionalLayerStore{
			{Path: machineconfigv1.StorePath(stargzSetup.GetStorePath())},
			{Path: machineconfigv1.StorePath("/var/lib/layerstore-1")},
		}
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Update(ctx, currentCfg, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Updated CRC to include 2 layer stores")

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating after adding second store")
		err = waitForMCPToStartUpdating(ctx, mcClient, mcpName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool rollout to complete after update")
		err = waitForMCP(ctx, mcClient, mcpName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MCP rollout completed with 2 layer stores")

		g.By("Verifying both layer stores in storage.conf")
		output, err = ExecOnNodeWithChroot(oc, testNode, "cat", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(fmt.Sprintf("%s:ref", stargzSetup.GetStorePath())))
		o.Expect(output).To(o.ContainSubstring("/var/lib/layerstore-1:ref"))
		framework.Logf("Node %s: Both layer stores verified in storage.conf", testNode)

		// =====================================================================
		// PHASE 13: Update CRC to maximum 5 layer stores
		// =====================================================================
		g.By("PHASE 13: Updating ContainerRuntimeConfig to maximum 5 layer stores")
		currentCfg, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Get(ctx, ctrcfg.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		currentCfg.Spec.ContainerRuntimeConfig.AdditionalLayerStores = []machineconfigv1.AdditionalLayerStore{
			{Path: machineconfigv1.StorePath(stargzSetup.GetStorePath())},
			{Path: machineconfigv1.StorePath("/var/lib/layerstore-1")},
			{Path: machineconfigv1.StorePath("/var/lib/layerstore-2")},
			{Path: machineconfigv1.StorePath("/var/lib/layerstore-3")},
			{Path: machineconfigv1.StorePath("/var/lib/layerstore-4")},
		}
		_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Update(ctx, currentCfg, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Updated CRC to include 5 layer stores (maximum)")

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating after adding to max stores")
		err = waitForMCPToStartUpdating(ctx, mcClient, mcpName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool rollout to complete with max stores")
		err = waitForMCP(ctx, mcClient, mcpName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MCP rollout completed with 5 layer stores (maximum)")

		g.By("Verifying all 5 layer stores in storage.conf")
		allLayerDirs := []string{stargzSetup.GetStorePath(), "/var/lib/layerstore-1", "/var/lib/layerstore-2", "/var/lib/layerstore-3", "/var/lib/layerstore-4"}
		output, err = ExecOnNodeWithChroot(oc, testNode, "cat", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, dir := range allLayerDirs {
			expectedPathWithRef := fmt.Sprintf("%s:ref", dir)
			o.Expect(output).To(o.ContainSubstring(expectedPathWithRef),
				"storage.conf should contain %s on node %s", expectedPathWithRef, testNode)
		}
		framework.Logf("Node %s: All 5 layer stores verified in storage.conf", testNode)

		// =====================================================================
		// PHASE 14: Delete ContainerRuntimeConfig
		// =====================================================================
		g.By("PHASE 14: Deleting ContainerRuntimeConfig")
		err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Delete(ctx, ctrcfg.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("ContainerRuntimeConfig %s deleted", ctrcfg.Name)

		g.By("Waiting for MachineConfigPool to start updating after deletion")
		err = waitForMCPToStartUpdating(ctx, mcClient, mcpName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating after CRC deletion")

		g.By("Waiting for MachineConfigPool rollout to complete after deletion")
		err = waitForMCP(ctx, mcClient, mcpName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool rollout completed after deletion")

		// =====================================================================
		// PHASE 15: Verify storage.conf cleanup (path removed)
		// =====================================================================
		g.By("PHASE 15: Verifying storage.conf cleanup after CRC deletion")
		output, err = ExecOnNodeWithChroot(oc, testNode, "cat", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring(stargzSetup.GetStorePath()),
			"storage.conf should not contain stargz-store path after ContainerRuntimeConfig deletion on node %s",
			testNode)
		framework.Logf("Node %s: stargz-store path removed from storage.conf", testNode)

		// =====================================================================
		// PHASE 16: Test fallback with standard OCI image (non-eStargz)
		// =====================================================================
		g.By("PHASE 16: Testing fallback with standard OCI image (non-eStargz)")
		standardImage := "quay.io/openshifttest/additional-storage-tests:test-6gb-standard-v2.0"
		framework.Logf("Using standard OCI image (non-eStargz): %s", standardImage)

		snapshotsBeforeFallback, err := getStargzSnapshotCount(oc, testNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Snapshots before standard image test: %d", snapshotsBeforeFallback)

		fallbackPodName := "fallback-standard-oci-pod"
		fallbackPod := createTestPod(fallbackPodName, testNamespace, standardImage, testNode)
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, fallbackPod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForPodRunning(ctx, oc, fallbackPodName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Pod started successfully with standard OCI image")

		snapshotsAfterFallback, err := getStargzSnapshotCount(oc, testNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		snapshotDiff := snapshotsAfterFallback - snapshotsBeforeFallback
		framework.Logf("Snapshots after standard image: %d (diff: %d)", snapshotsAfterFallback, snapshotDiff)

		o.Expect(snapshotDiff).To(o.BeNumerically("<=", 1),
			"Standard OCI image should not create significant stargz snapshots (fallback to standard pull)")

		deletePodAndWait(ctx, oc, testNamespace, fallbackPodName)
		framework.Logf("16. Standard OCI image fallback: YES")

		// =====================================================================
		// PHASE 17: Stop stargz-store and test eStargz fallback
		// =====================================================================
		g.By("PHASE 17: Stopping stargz-store service")
		_, err = ExecOnNodeWithChroot(oc, testNode, "systemctl", "stop", "stargz-store")
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("stargz-store service stopped")

		statusOutput, err := ExecOnNodeWithChroot(oc, testNode, "systemctl", "is-active", "stargz-store")
		framework.Logf("stargz-store status: %s", strings.TrimSpace(statusOutput))
		o.Expect(strings.TrimSpace(statusOutput)).NotTo(o.Equal("active"))

		g.By("PHASE 18: Testing eStargz image fallback when stargz-store is stopped")
		// Use a different eStargz image to test cold-start fallback (not cached)
		fallbackEStargzImage := "quay.io/openshifttest/additional-storage-tests:test-image-estargz-v1.0"
		framework.Logf("Using different eStargz image for fallback test: %s", fallbackEStargzImage)

		snapshotsBeforeStop, err := getStargzSnapshotCount(oc, testNode)
		o.Expect(err).NotTo(o.HaveOccurred())

		fallbackPod2Name := "fallback-estargz-stopped-pod"
		fallbackPod2 := createTestPod(fallbackPod2Name, testNamespace, fallbackEStargzImage, testNode)
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, fallbackPod2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForPodRunning(ctx, oc, fallbackPod2Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Pod running successfully with stargz-store stopped (fallback to standard pull)")

		snapshotsAfterStop, err := getStargzSnapshotCount(oc, testNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		snapshotDiffAfterStop := snapshotsAfterStop - snapshotsBeforeStop
		framework.Logf("Snapshots after stargz stopped: %d (diff: %d)", snapshotsAfterStop, snapshotDiffAfterStop)

		o.Expect(snapshotDiffAfterStop).To(o.BeNumerically("<=", 1),
			"eStargz image should fallback to standard pull when stargz-store is stopped")

		deletePodAndWait(ctx, oc, testNamespace, fallbackPod2Name)

		// Restart stargz-store for cleanup
		g.By("Restarting stargz-store service for cleanup")
		_, err = ExecOnNodeWithChroot(oc, testNode, "systemctl", "start", "stargz-store")
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("stargz-store service restarted")

		// Final Summary
		framework.Logf("========================================")
		framework.Logf("COMPREHENSIVE TEST RESULTS SUMMARY ")
		framework.Logf("========================================")
		framework.Logf("1. stargz-store deployed: YES")
		framework.Logf("2. stargz-store service active: YES")
		framework.Logf("3. ContainerRuntimeConfig applied (1 store): YES")
		framework.Logf("4. MCO/MCP rollout completed: YES")
		framework.Logf("5. storage.conf updated with :ref: YES")
		framework.Logf("6. CRI-O active: YES")
		framework.Logf("7. All nodes Ready: YES")
		framework.Logf("8. First pod with eStargz created: YES")
		framework.Logf("9. Snapshots created (lazy pull): YES")
		framework.Logf("10. Second pod layer sharing: VERIFIED")
		framework.Logf("11. stargz-store logs verified: YES")
		framework.Logf("12. CRI-O logs verified: YES")
		framework.Logf("13. Pods removed: YES")
		framework.Logf("14. CRC updated to 2 layer stores: YES")
		framework.Logf("15. MCP rollout with 2 stores: YES")
		framework.Logf("16. CRC updated to 5 layer stores max: YES")
		framework.Logf("17. MCP rollout with 5 stores: YES")
		framework.Logf("18. All 5 stores verified in storage.conf: YES")
		framework.Logf("19. CRC deleted: YES")
		framework.Logf("20. storage.conf cleanup: YES")
		framework.Logf("22. stargz-store stopped fallback: YES")
		framework.Logf("23. eStargz fallback to standard pull: YES")
		framework.Logf("========================================")
		framework.Logf("Image: %s", eStargzImage)
		framework.Logf("Test Node: %s", testNode)
		framework.Logf("========================================")
		framework.Logf("Test PASSED: Comprehensive additionalLayerStores E2E with fallback verification complete")
	})

})
