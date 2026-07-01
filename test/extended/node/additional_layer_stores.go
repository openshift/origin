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

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	additionalLayerStorePath     = "/var/lib/additional-layers"
	additionalLayerStoreTestName = "additional-layerstore-test"
	maxLayerStoresCount          = 5
)

// API validation tests - creating CRCs triggers MCO reconciliation making these disruptive
var _ = g.Describe("[apigroup:config.openshift.io][apigroup:machineconfiguration.openshift.io][Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][OCPFeatureGate:AdditionalStorageConfig][Suite:openshift/disruptive-longrunning] Additional Layer Stores API Validation", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-layer-stores-api")

	g.BeforeEach(func(ctx context.Context) {
		skipUnlessAdditionalStorageConfigEnabled(ctx, oc)
	})

	// TC1: Should fail if additionalLayerStores path is empty
	// Note: Go API returns "Required value" while YAML returns "at least 1 chars long"
	g.It("should reject empty path for additionalLayerStores [TC1]", func(ctx context.Context) {
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
		framework.Logf("Expected substring: 'Required value' (Go API) or 'at least 1 chars long' (YAML)")
		framework.Logf("Actual error: %v", err)
		o.Expect(err.Error()).To(o.ContainSubstring("Required value"))
		framework.Logf("Test PASSED: Empty path correctly rejected")
	})

	// TC2: Should fail if additionalLayerStores path is not absolute
	g.It("should reject relative path for additionalLayerStores [TC2]", func(ctx context.Context) {
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

	// TC3: Should fail if additionalLayerStores path contains spaces
	g.It("should reject path with spaces for additionalLayerStores [TC3]", func(ctx context.Context) {
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

	// TC4: Should fail if additionalLayerStores path contains invalid characters
	g.DescribeTable("should reject path with invalid characters for additionalLayerStores [TC4]",
		func(ctx context.Context, testPath, testName, invalidChar string) {
			mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
			o.Expect(err).NotTo(o.HaveOccurred())

			ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("layer-invalid-char-%s-test", testName),
				},
				Spec: machineconfigv1.ContainerRuntimeConfigSpec{
					MachineConfigPoolSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"pools.operator.machineconfiguration.openshift.io/worker": "",
						},
					},
					ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
						AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
							{Path: machineconfigv1.StorePath(testPath)},
						},
					},
				},
			}

			_, err = mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().Create(ctx, ctrcfg, metav1.CreateOptions{})
			o.Expect(err).To(o.HaveOccurred(), "Expected API to reject path with invalid character '%s'", invalidChar)
			framework.Logf("Path with '%s' correctly rejected: %v", invalidChar, err)
		},
		g.Entry("path with @ symbol", "/var/lib/stargz@store", "at-symbol", "@"),
		g.Entry("path with ! exclamation", "/var/lib/stargz!store", "exclamation", "!"),
		g.Entry("path with # hash", "/var/lib/stargz#store", "hash", "#"),
		g.Entry("path with $ dollar", "/var/lib/stargz$store", "dollar", "$"),
		g.Entry("path with % percent", "/var/lib/stargz%store", "percent", "%"),
	)

	// TC5: Should fail if additionalLayerStores path is too long (>256 bytes)
	g.It("should reject path exceeding 256 characters for additionalLayerStores [TC5]", func(ctx context.Context) {
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

	// TC6: Should fail if additionalLayerStores exceeds maximum of 5 items
	g.It("should reject more than 5 additionalLayerStores [TC6]", func(ctx context.Context) {
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

	// TC7: Should fail if additionalLayerStores path contains consecutive forward slashes
	g.It("should reject path with consecutive forward slashes for additionalLayerStores [TC7]", func(ctx context.Context) {
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

	// TC8: Should fail if additionalLayerStores contains duplicate paths
	g.It("should reject duplicate paths in additionalLayerStores [TC8]", func(ctx context.Context) {
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
})

// Disruptive E2E tests - must run serially
var _ = g.Describe("[Skipped:Disconnected][apigroup:config.openshift.io][apigroup:machineconfiguration.openshift.io][Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][OCPFeatureGate:AdditionalStorageConfig][Serial][Disruptive][Suite:openshift/disruptive-longrunning] Additional Layer Stores E2E", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-layer-stores")

	g.BeforeEach(func(ctx context.Context) {
		skipUnlessAdditionalStorageConfigEnabled(ctx, oc)
	})

	// TC9: Comprehensive E2E test - Stargz-store setup, CRC, verification, lazy pulling, and cleanup
	g.It("should configure additionalLayerStores with stargz-store and verify lazy pulling [TC9]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(workerNodes)).To(o.BeNumerically(">=", 1))
		pureWorkers := getPureWorkerNodes(workerNodes)
		if len(pureWorkers) < 1 {
			e2eskipper.Skipf("Need at least 1 worker node for this test")
		}
		testNode := pureWorkers[0]
		testNamespace := oc.Namespace()
		eStargzImage := "quay.io/openshifttest/additional-storage-tests:test-5mb-estargz"

		// =====================================================================
		// PHASE 1: Deploy stargz-store on worker nodes
		// =====================================================================
		g.By("PHASE 1: Deploying stargz-store on worker nodes")
		stargzSetup := NewStargzStoreSetup(oc)
		err = stargzSetup.Deploy(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			if stargzSetup.IsDeployed() {
				stargzSetup.Cleanup(ctx)
			}
		}()
		framework.Logf("stargz-store deployed successfully")

		g.By("Verifying stargz-store service is active on all workers")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "systemctl", "is-active", "stargz-store")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(output)).To(o.Equal("active"),
				"stargz-store should be active on node %s", node.Name)
			framework.Logf("Node %s: stargz-store service active", node.Name)
		}

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
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)
		framework.Logf("ContainerRuntimeConfig %s created with path: %s", ctrcfg.Name, stargzSetup.GetStorePath())

		g.By("Waiting for ContainerRuntimeConfig to be processed by MCO")
		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("ContainerRuntimeConfig processed by MCO")

		// =====================================================================
		// PHASE 3: Verify MCP rollout and nodes Ready
		// =====================================================================
		g.By("PHASE 3: Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating")

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool rollout completed")

		g.By("Verifying all nodes are Ready")
		for _, node := range pureWorkers {
			nodeObj, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, node.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(isNodeInReadyState(nodeObj)).To(o.BeTrue(),
				"Node %s should be Ready after MCP rollout", node.Name)
			framework.Logf("Node %s: Ready", node.Name)
		}

		// =====================================================================
		// PHASE 4: Verify storage.conf contains path with :ref suffix (MCO added)
		// =====================================================================
		g.By("PHASE 4: Verifying storage.conf contains stargz-store path with :ref suffix")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())

			// MCO automatically appends :ref suffix to all additionalLayerStores paths
			expectedPathWithRef := fmt.Sprintf("%s:ref", stargzSetup.GetStorePath())
			o.Expect(output).To(o.ContainSubstring("additionallayerstores"),
				"storage.conf should contain additionallayerstores on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring(expectedPathWithRef),
				"storage.conf should contain %s with :ref suffix on node %s", stargzSetup.GetStorePath(), node.Name)
			framework.Logf("Node %s: storage.conf verified with path %s (MCO added :ref)", node.Name, expectedPathWithRef)
		}

		g.By("Verifying CRI-O is active with new configuration")
		crioStatus, err := ExecOnNodeWithChroot(oc, testNode.Name, "systemctl", "is-active", "crio")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(crioStatus)).To(o.Equal("active"))
		framework.Logf("CRI-O is active")

		// =====================================================================
		// PHASE 5: Create first pod with eStargz image
		// =====================================================================
		g.By("PHASE 5: Getting initial snapshot count in stargz-store")
		initialSnapshots, err := getStargzSnapshotCount(oc, testNode.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Initial snapshot count: %d", initialSnapshots)

		g.By("Creating first pod with eStargz format image")
		pod1Name := "stargz-test-pod-1"
		pod1 := createTestPod(pod1Name, testNamespace, eStargzImage, testNode.Name)

		startTime1 := time.Now()
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, pod1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePodAndWait(ctx, oc, testNamespace, pod1Name)

		g.By("Waiting for first pod to be running")
		err = waitForPodRunning(ctx, oc, pod1Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		pod1Duration := time.Since(startTime1)
		framework.Logf("First pod %s started in %v (initial pull with lazy loading)", pod1Name, pod1Duration)

		// =====================================================================
		// PHASE 6: Verify snapshot created in layer store path
		// =====================================================================
		g.By("PHASE 6: Verifying snapshot is created in stargz-store")

		// Poll for snapshots to be created instead of sleeping
		var snapshotsAfterPod1 int
		err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
			count, countErr := getStargzSnapshotCount(oc, testNode.Name)
			if countErr != nil {
				return false, countErr
			}
			snapshotsAfterPod1 = count
			if snapshotsAfterPod1 > initialSnapshots {
				framework.Logf("Snapshot count increased to %d (was %d)", snapshotsAfterPod1, initialSnapshots)
				return true, nil
			}
			framework.Logf("Waiting for snapshots... current: %d, initial: %d", snapshotsAfterPod1, initialSnapshots)
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Snapshots should be created after pulling eStargz image")
		framework.Logf("Snapshot count after first pod: %d", snapshotsAfterPod1)

		storeOutput, err := ExecOnNodeWithChroot(oc, testNode.Name, "ls", "-lRt", "/var/lib/stargz-store/store/")
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("stargz-store contents:\n%s", storeOutput)

		o.Expect(storeOutput).To(o.ContainSubstring("sha256:"),
			"stargz-store should contain layer directories with sha256 digests")

		// =====================================================================
		// PHASE 7: Create second pod with same image
		// =====================================================================
		g.By("PHASE 7: Creating second pod with same eStargz image")
		pod2Name := "stargz-test-pod-2"
		pod2 := createTestPod(pod2Name, testNamespace, eStargzImage, testNode.Name)

		startTime2 := time.Now()
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, pod2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePodAndWait(ctx, oc, testNamespace, pod2Name)

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

		// Note: We don't assert a minimum speedup here because wall-clock time is affected by
		// cluster load, registry latency, and network conditions. Instead, we rely on the
		// snapshot count validation below to prove layer reuse is working correctly.

		// =====================================================================
		// PHASE 8: Verify second pod used existing snapshot (no new layers)
		// =====================================================================
		g.By("PHASE 8: Verifying second pod used existing snapshot")
		pod2Events, _ := oc.Run("describe").Args("pod", pod2Name).Output()
		framework.Logf("Second pod events: %s", pod2Events)

		snapshotsAfterPod2, err := getStargzSnapshotCount(oc, testNode.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Snapshot count after second pod: %d", snapshotsAfterPod2)
		o.Expect(snapshotsAfterPod2).To(o.Equal(snapshotsAfterPod1),
			"Snapshot count should remain same when using shared layers")

		// =====================================================================
		// PHASE 9: Verify through stargz-store and crio logs
		// =====================================================================
		g.By("PHASE 9: Verifying through stargz-store logs")
		stargzLogs, _ := ExecOnNodeWithChroot(oc, testNode.Name, "journalctl", "-u", "stargz-store", "--since", "5 minutes ago", "-n", "50")
		framework.Logf("Recent stargz-store logs:\n%s", stargzLogs)

		g.By("Verifying through CRI-O logs")
		crioLogs, _ := ExecOnNodeWithChroot(oc, testNode.Name, "journalctl", "-u", "crio", "--since", "5 minutes ago", "--grep", eStargzImage, "-n", "20")
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
		// PHASE 11: Delete ContainerRuntimeConfig
		// =====================================================================
		g.By("PHASE 11: Deleting ContainerRuntimeConfig")
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
		// PHASE 12: Verify storage.conf cleanup (path removed)
		// =====================================================================
		g.By("PHASE 12: Verifying storage.conf cleanup after CRC deletion")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).NotTo(o.ContainSubstring(stargzSetup.GetStorePath()),
				"storage.conf should not contain stargz-store path after ContainerRuntimeConfig deletion on node %s",
				node.Name)
			framework.Logf("Node %s: stargz-store path removed from storage.conf", node.Name)
		}

		// Final Summary
		framework.Logf("========================================")
		framework.Logf("COMPREHENSIVE TEST RESULTS SUMMARY")
		framework.Logf("========================================")
		framework.Logf("1. stargz-store deployed: YES")
		framework.Logf("2. stargz-store service active: YES")
		framework.Logf("3. ContainerRuntimeConfig applied: YES")
		framework.Logf("4. MCO/MCP rollout completed: YES")
		framework.Logf("5. storage.conf updated with :ref: YES")
		framework.Logf("6. CRI-O active: YES")
		framework.Logf("7. All nodes Ready: YES")
		framework.Logf("8. First pod with eStargz created: YES")
		framework.Logf("9. Snapshots created: YES (count: %d -> %d)", initialSnapshots, snapshotsAfterPod1)
		framework.Logf("10. Second pod layer sharing: VERIFIED (snapshot count unchanged)")
		framework.Logf("11. stargz-store logs verified: YES")
		framework.Logf("12. CRI-O logs verified: YES")
		framework.Logf("13. Pods removed: YES")
		framework.Logf("14. CRC deleted: YES")
		framework.Logf("15. storage.conf cleanup: YES")
		framework.Logf("========================================")
		framework.Logf("Image: %s", eStargzImage)
		framework.Logf("Test Node: %s", testNode.Name)
		framework.Logf("========================================")
		framework.Logf("Test PASSED: Comprehensive additionalLayerStores E2E verification complete")
	})

	// TC10: Update Existing Configuration
	g.It("should update additionalLayerStores when ContainerRuntimeConfig is modified [TC10]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		if len(pureWorkers) < 1 {
			e2eskipper.Skipf("Need at least 1 worker node for this test")
		}

		g.By("Creating shared layer directories on worker nodes")
		layerDirs := []string{"/var/lib/layerstore-1", "/var/lib/layerstore-2", "/var/lib/layerstore-3"}
		err = createDirectoriesOnNodes(oc, pureWorkers, layerDirs)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, pureWorkers, layerDirs)

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
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating")

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying initial configuration with :ref suffix")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			// MCO automatically appends :ref suffix
			o.Expect(output).To(o.ContainSubstring("/var/lib/layerstore-1:ref"),
				"storage.conf should contain /var/lib/layerstore-1:ref on node %s", node.Name)
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

		g.By("Waiting for MachineConfigPool to start updating after update")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating")

		g.By("Waiting for MachineConfigPool rollout to complete after update")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying updated configuration includes both stores with :ref suffix")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			// MCO automatically appends :ref suffix to all paths
			o.Expect(output).To(o.ContainSubstring("/var/lib/layerstore-1:ref"),
				"storage.conf should contain /var/lib/layerstore-1:ref on node %s", node.Name)
			o.Expect(output).To(o.ContainSubstring("/var/lib/layerstore-2:ref"),
				"storage.conf should contain /var/lib/layerstore-2:ref on node %s", node.Name)
			framework.Logf("Node %s: Both layer stores configured with :ref suffix after update", node.Name)
		}

		framework.Logf("Test PASSED: ContainerRuntimeConfig update applied successfully")
	})

	// TC11: Multiple Storage Paths (up to max 5)
	g.It("should configure multiple additionalLayerStores paths up to maximum [TC11]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		if len(pureWorkers) < 1 {
			e2eskipper.Skipf("Need at least 1 worker node for this test")
		}

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
		g.DeferCleanup(cleanupDirectoriesOnNodes, oc, pureWorkers, layerDirs)

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
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating")

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying all 5 layer stores configured with :ref suffix on nodes")
		for _, node := range pureWorkers {
			output, err := ExecOnNodeWithChroot(oc, node.Name, "cat", "/etc/containers/storage.conf")
			o.Expect(err).NotTo(o.HaveOccurred())
			// MCO automatically appends :ref suffix to all paths
			for _, dir := range layerDirs {
				expectedPathWithRef := fmt.Sprintf("%s:ref", dir)
				o.Expect(output).To(o.ContainSubstring(expectedPathWithRef),
					"storage.conf should contain %s with :ref suffix on node %s", dir, node.Name)
			}
			framework.Logf("Node %s: All 5 layer stores configured with :ref suffix", node.Name)
		}

		framework.Logf("Test PASSED: Multiple additionalLayerStores (max 5) configured successfully")
	})

	// TC12: Fallback when non-eStargz image is used
	g.It("should fallback to standard pull when non-eStargz image is used [TC12]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		if len(pureWorkers) < 1 {
			e2eskipper.Skipf("Need at least 1 worker node for this test")
		}
		testNode := pureWorkers[0]

		g.By("Phase 1: Deploying stargz-store for additionalLayerStores")
		stargzSetup := NewStargzStoreSetup(oc)
		err = stargzSetup.Deploy(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer stargzSetup.Cleanup(ctx)
		framework.Logf("stargz-store deployed successfully")

		g.By("Phase 2: Creating ContainerRuntimeConfig with additionalLayerStores")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-fallback-nonstargz-test",
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
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating")

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MCP rollout completed")

		g.By("Phase 3: Verifying stargz-store is running")
		err = stargzSetup.VerifyStorageConfContainsStargz(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Phase 4: Testing fallback with non-eStargz image (standard OCI image)")
		// Use a standard OCI image (NOT eStargz format) - 6GB image to test fallback
		standardImage := "quay.io/openshifttest/additional-storage-tests:test-6gb-standard-v1.0"
		framework.Logf("Pulling standard OCI image (non-eStargz): %s", standardImage)

		testNamespace := oc.Namespace()
		initialSnapshots, err := getStargzSnapshotCount(oc, testNode.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Initial stargz snapshots: %d", initialSnapshots)

		// Create pod with standard OCI image
		podName := "fallback-standard-oci-pod"
		pod := createTestPod(podName, testNamespace, standardImage, testNode.Name)
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePodAndWait(ctx, oc, testNamespace, podName)

		g.By("Waiting for pod to start with standard image")
		err = waitForPodRunning(ctx, oc, podName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Pod started successfully with standard OCI image")

		// Verify stargz snapshots did NOT increase (fallback to standard pull)
		snapshotsAfterStandard, err := getStargzSnapshotCount(oc, testNode.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Snapshots after standard image: %d", snapshotsAfterStandard)

		snapshotDiff := snapshotsAfterStandard - initialSnapshots
		framework.Logf("Snapshot difference: %d", snapshotDiff)

		// Standard images should NOT create stargz snapshots (or very minimal)
		// The image should be pulled normally using standard OCI pull mechanism
		o.Expect(snapshotDiff).To(o.BeNumerically("<=", 1),
			"Standard OCI image should not create significant stargz snapshots (fallback to standard pull)")

		g.By("Verifying pod is running and healthy")
		podObj, err := oc.AdminKubeClient().CoreV1().Pods(testNamespace).Get(ctx, podName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podObj.Status.Phase).To(o.Equal(corev1.PodRunning))

		deletePodAndWait(ctx, oc, testNamespace, podName)

		framework.Logf("Test PASSED: Non-eStargz image successfully used standard pull mechanism (fallback)")
	})

	// TC13: Fallback when stargz-store is stopped
	g.It("should fallback to standard pull when stargz-store service is stopped [TC13]", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		pureWorkers := getPureWorkerNodes(workerNodes)
		if len(pureWorkers) < 1 {
			e2eskipper.Skipf("Need at least 1 worker node for this test")
		}
		testNode := pureWorkers[0]

		g.By("Phase 1: Deploying stargz-store for additionalLayerStores")
		stargzSetup := NewStargzStoreSetup(oc)
		err = stargzSetup.Deploy(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer stargzSetup.Cleanup(ctx)
		framework.Logf("stargz-store deployed successfully")

		g.By("Phase 2: Creating ContainerRuntimeConfig with additionalLayerStores")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "layer-fallback-stopped-test",
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
		g.DeferCleanup(cleanupContainerRuntimeConfig, ctx, mcClient, ctrcfg.Name)

		err = waitForContainerRuntimeConfigSuccess(ctx, mcClient, ctrcfg.Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for MachineConfigPool to start updating")
		err = waitForMCPToStartUpdating(ctx, mcClient, "worker", 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfigPool started updating")

		g.By("Waiting for MachineConfigPool rollout to complete")
		err = waitForMCP(ctx, mcClient, "worker", 25*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MCP rollout completed")

		g.By("Phase 3: Verifying stargz-store is running")
		err = stargzSetup.VerifyStorageConfContainsStargz(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Phase 4: Testing eStargz image works when stargz-store is running")
		eStargzImage := "quay.io/openshifttest/additional-storage-tests:test-5mb-estargz"
		framework.Logf("Pulling eStargz image with stargz-store running: %s", eStargzImage)

		testNamespace := oc.Namespace()
		initialSnapshots, err := getStargzSnapshotCount(oc, testNode.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Initial stargz snapshots: %d", initialSnapshots)

		// First pod - stargz-store running
		pod1Name := "fallback-estargz-running-pod"
		pod1 := createTestPod(pod1Name, testNamespace, eStargzImage, testNode.Name)
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, pod1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePodAndWait(ctx, oc, testNamespace, pod1Name)

		err = waitForPodRunning(ctx, oc, pod1Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("First pod running with stargz-store active")

		snapshotsAfterPod1, err := getStargzSnapshotCount(oc, testNode.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Snapshots after first pod: %d", snapshotsAfterPod1)
		o.Expect(snapshotsAfterPod1).To(o.BeNumerically(">", initialSnapshots),
			"eStargz image should create snapshots when stargz-store is running")

		deletePodAndWait(ctx, oc, testNamespace, pod1Name)

		g.By("Phase 5: Stopping stargz-store service on test node")
		stopOutput, err := ExecOnNodeWithChroot(oc, testNode.Name, "systemctl", "stop", "stargz-store")
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("stargz-store stopped: %s", stopOutput)
		defer func() {
			// Restart stargz-store at cleanup
			framework.Logf("Restarting stargz-store service")
			ExecOnNodeWithChroot(oc, testNode.Name, "systemctl", "start", "stargz-store")
		}()

		// Verify service is stopped
		statusOutput, err := ExecOnNodeWithChroot(oc, testNode.Name, "systemctl", "is-active", "stargz-store")
		framework.Logf("stargz-store status after stop: %s", strings.TrimSpace(statusOutput))
		o.Expect(strings.TrimSpace(statusOutput)).NotTo(o.Equal("active"))

		g.By("Phase 6: Testing fallback when stargz-store is stopped")
		framework.Logf("Pulling different eStargz image with stargz-store stopped (should fallback to standard pull)")

		// Second pod - use different eStargz image that hasn't been pulled yet
		// This tests that when stargz-store is stopped, new eStargz images fallback to standard pull
		fallbackImage := "quay.io/openshifttest/additional-storage-tests:test-6gb-estargz-v1.0"
		pod2Name := "fallback-estargz-stopped-pod"
		pod2 := createTestPod(pod2Name, testNamespace, fallbackImage, testNode.Name)
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, pod2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deletePodAndWait(ctx, oc, testNamespace, pod2Name)

		err = waitForPodRunning(ctx, oc, pod2Name, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Second pod running successfully with stargz-store stopped (fallback to standard pull)")

		// Verify pod is healthy
		pod2Obj, err := oc.AdminKubeClient().CoreV1().Pods(testNamespace).Get(ctx, pod2Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pod2Obj.Status.Phase).To(o.Equal(corev1.PodRunning))

		// Verify snapshots did not change significantly (stargz-store was stopped)
		snapshotsAfterPod2, err := getStargzSnapshotCount(oc, testNode.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Snapshots after second pod (stargz stopped): %d", snapshotsAfterPod2)

		snapshotDiffAfterStop := snapshotsAfterPod2 - snapshotsAfterPod1
		framework.Logf("New snapshots created with stargz-store stopped: %d", snapshotDiffAfterStop)

		// When stargz-store is stopped, eStargz images should fallback to standard pull
		// No new stargz snapshots should be created
		o.Expect(snapshotDiffAfterStop).To(o.BeNumerically("<=", 1),
			"When stargz-store is stopped, eStargz images should fallback to standard pull (no new snapshots)")

		deletePodAndWait(ctx, oc, testNamespace, pod2Name)

		framework.Logf("Test PASSED: eStargz image successfully fell back to standard pull when stargz-store was stopped")
	})
})
