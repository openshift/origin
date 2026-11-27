package node

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

// syncFsDropInPath is the CRI-O config drop-in injected by the MachineConfig
// (99-* so it wins over the base crio.conf). Uses CRI-O's own storage_option
// (crio.conf.d), not a containers/storage storage.conf.d drop-in: openshift/
// cri-o's vendored storage fork predates storage.conf.d support, so that
// drop-in would be silently ignored.
const (
	syncFsDropInPath    = "/etc/crio/crio.conf.d/99-sync-filesystem.conf"
	syncFsDropInContent = "[crio]\nstorage_option = [\n  \"overlay.sync=filesystem\",\n]\n"

	// syncFsCrashMarkerPath is written (and synced) by the crash script only
	// once the raced pull has observably started writing layers to the store;
	// Step 5 of the durability spec requires it after the reboot, proving the
	// panic was synchronized with storage writes. Syncing only this one small
	// file leaves CRI-O's concurrent layer writes unflushed, preserving the
	// power-loss semantics.
	syncFsCrashMarkerPath = "/var/tmp/syncfs-crash-synchronized"
)

// CRI-O overlay.sync=filesystem E2E test suite.
//
// overlay.sync=filesystem is an opt-in containers-storage/CRI-O durability option that
// calls syncfs() before committing layer metadata to disk, so that an unclean node
// shutdown mid-pull cannot leave the image store in a corrupted or partially-committed
// state.
//
// This suite verifies, end-to-end on an OpenShift cluster, that:
//   - The option can be enabled via a MachineConfig crio.conf.d drop-in and is picked
//     up by CRI-O without disrupting the node.
//   - Normal image pulls and pod startup continue to work with the option enabled.
//   - Concurrent image pulls do not deadlock or corrupt the store while the option is
//     enabled.
//   - Disabling the option again (removing the MachineConfig) is safe: previously
//     pulled images and CRI-O itself remain intact and functional afterward.

var _ = g.Describe("[sig-node][Jira:Node/CRI-O][apigroup:machineconfiguration.openshift.io][Serial][Disruptive][Suite:openshift/disruptive-longrunning] CRI-O overlay.sync=filesystem", ote.Informing(), func() {
	defer g.GinkgoRecover()

	var (
		oc        = exutil.NewCLI("syncfs-overlay")
		mcClient  *machineconfigclient.Clientset
		testNode  string
		mcpName   string
		mcName    string
		mcpConfig *CustomMCPConfig
	)

	g.BeforeEach(func(ctx context.Context) {
		SkipOnMicroShift(oc)
		SkipOnHyperShift(ctx, oc)
		EnsureNodesReady(ctx, oc)

		var err error
		mcClient, err = machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		testNode = GetFirstReadyWorkerNode(oc)
		framework.Logf("Test node: %s", testNode)

		mcpName = "syncfs-test"
		mcName = "99-syncfs-overlay-" + mcpName

		mcpConfig, err = CreateCustomMCPForNode(ctx, oc, mcClient, mcpName, testNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Custom MCP %q created for node %s", mcpName, testNode)
	})

	g.AfterEach(func() {
		cleanupCtx := context.Background()
		// deleteSyncFsMachineConfig tolerates not-found, so this is safe even if
		// the rollback test already deleted the MC as part of its own verification.
		if err := deleteSyncFsMachineConfig(cleanupCtx, mcClient, mcName); err != nil {
			framework.Logf("Warning: failed to delete MachineConfig %q during cleanup: %v", mcName, err)
		} else {
			if waitErr := WaitForMCP(cleanupCtx, mcClient, mcpName, 15*time.Minute); waitErr != nil {
				framework.Logf("Warning: MCP %q did not settle after MC removal: %v", mcpName, waitErr)
			}
		}
		if mcpConfig != nil {
			if err := CleanupCustomMCP(cleanupCtx, mcpConfig); err != nil {
				framework.Logf("Warning: failed to cleanup MCP %q: %v", mcpName, err)
			}
		}
	})

	// applyMachineConfig creates the syncfs MachineConfig and waits for the MCP
	// to roll out on the test node. Called at the start of each It that requires
	// overlay.sync=filesystem to be active on the node.
	applyMachineConfig := func(ctx context.Context) {
		g.By("Enabling overlay.sync=filesystem via a MachineConfig crio.conf.d drop-in")
		mc := newSyncFsMachineConfig(mcName, mcpName)
		_, err := mcClient.MachineconfigurationV1().MachineConfigs().Create(ctx, mc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfig %q created; waiting for MCP %q rollout", mcName, mcpName)

		err = WaitForMCPUpdating(ctx, mcClient, mcpName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = WaitForMCP(ctx, mcClient, mcpName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MCP %q rollout complete", mcpName)
	}

	// Verifies the durability guarantee overlay.sync=filesystem provides: the
	// store survives a real, unflushed hard crash (kernel panic via sysrq, not a
	// process kill) while a pull is still in flight. The panic is fired only once
	// layer writes are observably underway (see Step 2) -- crashing after the pull
	// completes would prove nothing, since the data is committed either way.
	g.It("should not corrupt the store when a real power-loss lands mid-pull", func(ctx context.Context) {
		applyMachineConfig(ctx)

		testImage := image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.66.1")

		g.By("Step 1: Verifying overlay.sync=filesystem was actually picked up by CRI-O, then removing any pre-existing copy so the race below starts from a genuinely fresh pull")
		// Sanity-check the config so a silent misconfiguration fails here with a
		// clear message instead of as a confusing failure below.
		dropIn, err := ExecOnNodeWithChroot(ctx, oc, testNode, "cat", syncFsDropInPath)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(dropIn).To(o.ContainSubstring("overlay.sync=filesystem"),
			"crio.conf.d drop-in should contain overlay.sync=filesystem")
		crioConfig, err := ExecOnNodeWithChroot(ctx, oc, testNode, "crio", "config")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(crioConfig).To(o.ContainSubstring("overlay.sync=filesystem"),
			"crio config should report overlay.sync=filesystem as an active storage_option, not just as a dropped-in file")

		existingID, listErr := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "images", "-q", testImage)
		o.Expect(listErr).NotTo(o.HaveOccurred(), "listing pre-existing test image should succeed")
		if strings.TrimSpace(existingID) != "" {
			framework.Logf("Removing pre-existing image to force a fresh pull")
			_, err := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "rmi", strings.TrimSpace(existingID))
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Step 2: Starting the pull and triggering a real, unflushed hard crash (kernel panic via sysrq) only once layer writes are observably in flight")
		crashTime := time.Now()
		// The script backgrounds the pull and waits (polling every 50ms) until
		// it has observable evidence the pull is writing layers -- a new
		// directory under .../storage/overlay -- and only then triggers the
		// panic. The image is removed beforehand, so a successful pull must
		// create new entries there; if no growth is observed within 30s, the
		// marker is not written and Step 5 fails the test, so this spec cannot
		// pass without a crash landing during layer writes.
		raceScript := fmt.Sprintf(`
img=%s
store=/var/lib/containers/storage/overlay
marker=%s
rm -f "$marker"
before=$(ls "$store" 2>/dev/null | wc -l)
crictl pull "$img" &
pullPid=$!
observed=0
for i in $(seq 1 600); do
	now=$(ls "$store" 2>/dev/null | wc -l)
	if [ "$now" -gt "$before" ]; then observed=1; break; fi
	if ! kill -0 "$pullPid" 2>/dev/null; then break; fi
	sleep 0.05
done
if [ "$observed" -eq 1 ]; then
	echo "layer-writes-observed" > "$marker"
	sync "$marker"
fi
echo c > /proc/sysrq-trigger
`, shellQuote(testImage), shellQuote(syncFsCrashMarkerPath))
		_, _ = ExecOnNodeWithChroot(ctx, oc, testNode, "bash", "-c", raceScript)

		g.By("Step 3: Waiting for the node to go NotReady, then recover and become Ready again")
		notReady := e2enode.WaitForNodeToBeNotReady(ctx, oc.AdminKubeClient(), testNode, 3*time.Minute)
		o.Expect(notReady).To(o.BeTrue(), "node should go NotReady after the kernel panic")
		ready := e2enode.WaitForNodeToBeReady(ctx, oc.AdminKubeClient(), testNode, 10*time.Minute)
		o.Expect(ready).To(o.BeTrue(), "node should recover and become Ready again after the panic")
		framework.Logf("Node %s recovered %v after the crash", testNode, time.Since(crashTime))

		g.By("Step 4: Verifying CRI-O detected the unclean shutdown on restart")
		crioLog, err := ExecOnNodeWithChroot(ctx, oc, testNode, "journalctl", "-u", "crio", "--no-pager", "-b")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(crioLog).To(o.ContainSubstring("unclean shutdown"),
			"CRI-O should detect and log the unclean shutdown on restart")

		g.By("Step 5: Verifying the crash was actually synchronized with in-flight storage writes")
		marker, err := ExecOnNodeWithChroot(ctx, oc, testNode, "cat", syncFsCrashMarkerPath)
		o.Expect(err).NotTo(o.HaveOccurred(),
			"crash-synchronization marker must exist: without it the panic may have fired before the pull reached storage")
		o.Expect(marker).To(o.ContainSubstring("layer-writes-observed"),
			"the marker must record that layer writes were observed before the panic")

		g.By("Step 6a: Verifying the raced pull either never committed, or committed fully intact -- never partial or corrupted")
		imageIDAfter, err := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "images", "-q", testImage)
		o.Expect(err).NotTo(o.HaveOccurred())
		imageIDAfter = strings.TrimSpace(imageIDAfter)
		if imageIDAfter == "" {
			// Depending on exactly how early the crash landed, the pull may
			// not have committed anything yet (nothing for CRI-O to flag as
			// incomplete) or may have left a partial layer behind (which
			// CRI-O discards and logs on restart). Both are fine -- Step 7
			// below is what actually proves the store is still healthy.
			if strings.Contains(crioLog, "Found incomplete layer") {
				framework.Logf("Pull did not commit before the crash landed; CRI-O found and discarded the incomplete layer")
			} else {
				framework.Logf("Pull did not commit before the crash landed; no incomplete layer was logged (crash likely landed before any layer commit began)")
			}
		} else {
			framework.Logf("Pull won the race and committed before the crash: %s", imageIDAfter)
			_, err = ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "inspecti", testImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "if the image is present, its metadata must be fully intact and inspectable")

			// inspecti only proves metadata; a truncated layer file would go
			// unnoticed, and the recovery pull below would reuse the existing
			// layer ID without reading its contents. Running a container from
			// the surviving image forces the layers to be read (overlay mount
			// + exec), proving the contents survived intact.
			g.By("Step 6b: Starting a pod from the surviving image to verify its layer contents are actually usable")
			crashSurvivorPod := createSyncFsPod("syncfs-crash-survivor-pod", oc.Namespace(), testImage, testNode)
			g.DeferCleanup(func() {
				_ = e2epod.DeletePodWithWaitByName(context.Background(), oc.AdminKubeClient(), crashSurvivorPod.Name, crashSurvivorPod.Namespace)
			})
			_, err = oc.AdminKubeClient().CoreV1().Pods(crashSurvivorPod.Namespace).Create(ctx, crashSurvivorPod, metav1.CreateOptions{})
			o.Expect(errorWithoutDetails(err)).NotTo(o.HaveOccurred(), "pod creation from the surviving image should succeed")
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.AdminKubeClient(), crashSurvivorPod)
			o.Expect(errorWithoutDetails(err)).NotTo(o.HaveOccurred(),
				"a container started from the surviving image must run: intact metadata with truncated layer contents is exactly the corruption this test exists to detect")
			framework.Logf("Surviving image is not just inspectable but runnable: layer contents survived the crash intact")
		}

		g.By("Step 7: Verifying the store itself is healthy: a pull of the same image now succeeds cleanly")
		_, err = ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", testImage)
		o.Expect(errorWithoutDetails(err)).NotTo(o.HaveOccurred(), "the store should not be corrupted: a pull after the crash should succeed")
		finalID, err := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "images", "-q", testImage)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(finalID)).NotTo(o.BeEmpty())
		_, err = ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "inspecti", testImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "image should be fully inspectable after a clean post-crash pull")

		framework.Logf("Durability check passed: store recovered cleanly from a real power-loss landing mid-pull with overlay.sync=filesystem enabled")
	})

	// Verifies that with overlay.sync=filesystem enabled, a fresh image pull
	// succeeds and a pod using that image reaches the Running phase.
	g.It("should allow image pulls and pod startup with overlay.sync=filesystem enabled", func(ctx context.Context) {
		applyMachineConfig(ctx)

		testNamespace := oc.Namespace()
		testImage := image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.66.1")

		g.By("Step 1: Pulling a test image with overlay.sync=filesystem enabled")
		// Remove any cached copy to force a real pull through CRI-O with sync=filesystem.
		if existingID, _ := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "images", "-q", testImage); strings.TrimSpace(existingID) != "" {
			framework.Logf("Removing pre-existing image to force a fresh pull")
			_, err := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "rmi", strings.TrimSpace(existingID))
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		_, err := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", testImage)
		o.Expect(errorWithoutDetails(err)).NotTo(o.HaveOccurred(), "crictl pull should succeed with sync=filesystem")
		framework.Logf("Image pulled successfully with sync=filesystem enabled")

		g.By("Step 2: Starting a pod using the image pulled under overlay.sync=filesystem")
		testPod := createSyncFsPod("syncfs-functional-pod", testNamespace, testImage, testNode)
		g.DeferCleanup(func() {
			_ = e2epod.DeletePodWithWaitByName(context.Background(), oc.AdminKubeClient(), testPod.Name, testNamespace)
		})
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, testPod, metav1.CreateOptions{})
		o.Expect(errorWithoutDetails(err)).NotTo(o.HaveOccurred(), "pod creation should succeed")
		err = e2epod.WaitForPodRunningInNamespace(ctx, oc.AdminKubeClient(), testPod)
		o.Expect(errorWithoutDetails(err)).NotTo(o.HaveOccurred(), "pod should reach Running")
		framework.Logf("Functional check passed: pod is Running using an image pulled under overlay.sync=filesystem")
	})

	// Verifies that pulling multiple images in parallel with overlay.sync=filesystem
	// enabled does not cause deadlocks or leave CRI-O in a degraded state.
	g.It("should handle concurrent image pulls without deadlock with overlay.sync=filesystem enabled", func(ctx context.Context) {
		applyMachineConfig(ctx)

		g.By("Step 1: Pulling multiple images concurrently to verify no deadlock under overlay.sync=filesystem")
		concurrentImages := []string{
			image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.55"),
			image.LocationFor("registry.k8s.io/e2e-test-images/busybox:1.37.0-2"),
			image.LocationFor("registry.k8s.io/e2e-test-images/nginx:1.27.0-2"),
			image.LocationFor("registry.k8s.io/e2e-test-images/sample-device-plugin:1.8.0"),
		}

		// Remove cached copies so every concurrent pull writes fresh layers
		// (exercising the syncfs path under concurrency) instead of resolving
		// to already-present layer IDs. Safe for other workloads: crictl rmi
		// refuses to remove images still in use by any container.
		for _, img := range concurrentImages {
			existingID, err := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "images", "-q", img)
			o.Expect(err).NotTo(o.HaveOccurred(), "listing cached copies of %s should succeed", img)
			if strings.TrimSpace(existingID) != "" {
				framework.Logf("Removing cached copy of %s to force a fresh pull", img)
				_, err := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "rmi", strings.TrimSpace(existingID))
				o.Expect(err).NotTo(o.HaveOccurred(),
					"cached test image %s must be removable for this test to force fresh concurrent layer writes", img)
			}
		}

		// Pull all 4 images in parallel inside a single debug session using shell job control.
		// wait $p1 && ... propagates exit codes so any pull failure fails the command.
		pullScript := fmt.Sprintf(
			"crictl pull %s & p1=$!; crictl pull %s & p2=$!; crictl pull %s & p3=$!; crictl pull %s & p4=$!; wait $p1 && wait $p2 && wait $p3 && wait $p4",
			shellQuote(concurrentImages[0]),
			shellQuote(concurrentImages[1]),
			shellQuote(concurrentImages[2]),
			shellQuote(concurrentImages[3]),
		)
		_, err := ExecOnNodeWithChroot(ctx, oc, testNode, "bash", "-c", pullScript)
		o.Expect(errorWithoutDetails(err)).NotTo(o.HaveOccurred(),
			"all concurrent crictl pulls should complete without deadlock")

		g.By("Step 2: Verifying CRI-O is still active after concurrent pulls")
		crioStatus, err := ExecOnNodeWithChroot(ctx, oc, testNode, "systemctl", "is-active", "crio")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(crioStatus)).To(o.Equal("active"),
			"CRI-O must remain active after concurrent pulls")
		framework.Logf("Concurrency check passed: %d parallel pulls completed, CRI-O remained active", len(concurrentImages))
	})

	// Verifies that disabling overlay.sync=filesystem (by removing the MachineConfig)
	// is safe: images already present on the node survive the rollback and new pods
	// can start without any intervention.
	g.It("should preserve previously-pulled images and allow new pods after overlay.sync=filesystem is disabled", func(ctx context.Context) {
		applyMachineConfig(ctx)

		testNamespace := oc.Namespace()
		testImage := image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.66.1")

		g.By("Step 1: Pulling a test image while overlay.sync=filesystem is enabled")
		_, err := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", testImage)
		o.Expect(errorWithoutDetails(err)).NotTo(o.HaveOccurred())

		// Record image IDs present on the node before the rollback.
		imagesBefore, err := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "images", "-q")
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Image IDs on node before rollback:\n%s", strings.TrimSpace(imagesBefore))

		g.By("Step 2: Disabling overlay.sync=filesystem by removing the MachineConfig")
		// Deleting the MC triggers MCO to remove the drop-in file and restart CRI-O.
		err = deleteSyncFsMachineConfig(ctx, mcClient, mcName)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("MachineConfig %q deleted; waiting for MCP %q rollback", mcName, mcpName)

		err = WaitForMCPUpdating(ctx, mcClient, mcpName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = WaitForMCP(ctx, mcClient, mcpName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Rollback MCP rollout complete")

		g.By("Step 3: Verifying the crio.conf.d drop-in has been removed from the node")
		_, dropInErr := ExecOnNodeWithChroot(ctx, oc, testNode, "cat", syncFsDropInPath)
		o.Expect(dropInErr).To(o.HaveOccurred(),
			"drop-in file should not exist after MachineConfig deletion")

		g.By("Step 4: Verifying CRI-O is active again without overlay.sync=filesystem")
		crioStatus, err := ExecOnNodeWithChroot(ctx, oc, testNode, "systemctl", "is-active", "crio")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(crioStatus)).To(o.Equal("active"),
			"CRI-O must remain active after the rollback")

		g.By("Step 5: Verifying all previously-pulled images are still present after rollback")
		imagesAfter, err := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "images", "-q")
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, id := range strings.Fields(imagesBefore) {
			o.Expect(imagesAfter).To(o.ContainSubstring(id),
				"image %s should survive disabling sync=filesystem", id)
		}
		framework.Logf("All %d image IDs still present after rollback", len(strings.Fields(imagesBefore)))

		g.By("Step 6: Verifying a new pod can start after overlay.sync=filesystem is disabled")
		rollbackPod := createSyncFsPod("syncfs-rollback-pod", testNamespace, testImage, testNode)
		g.DeferCleanup(func() {
			_ = e2epod.DeletePodWithWaitByName(context.Background(), oc.AdminKubeClient(), rollbackPod.Name, testNamespace)
		})
		_, err = oc.AdminKubeClient().CoreV1().Pods(testNamespace).Create(ctx, rollbackPod, metav1.CreateOptions{})
		o.Expect(errorWithoutDetails(err)).NotTo(o.HaveOccurred())
		err = e2epod.WaitForPodRunningInNamespace(ctx, oc.AdminKubeClient(), rollbackPod)
		o.Expect(errorWithoutDetails(err)).NotTo(o.HaveOccurred(),
			"pod should reach Running after sync=filesystem is disabled")
		framework.Logf("Rollback check passed: images intact, CRI-O healthy, new pod Running")
	})
})

// newSyncFsMachineConfig returns a MachineConfig that injects a crio.conf.d
// drop-in enabling overlay.sync=filesystem for the specified MachineConfigPool role.
func newSyncFsMachineConfig(name, mcpRole string) *machineconfigv1.MachineConfig {
	encoded := base64.StdEncoding.EncodeToString([]byte(syncFsDropInContent))
	ignJSON := fmt.Sprintf(
		`{"ignition":{"version":"3.4.0"},"storage":{"files":[{"path":%q,"mode":420,"overwrite":true,"contents":{"source":"data:text/plain;base64,%s"}}]}}`,
		syncFsDropInPath,
		encoded,
	)
	return &machineconfigv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				machineconfigv1.MachineConfigRoleLabelKey: mcpRole,
			},
		},
		Spec: machineconfigv1.MachineConfigSpec{
			Config: runtime.RawExtension{Raw: []byte(ignJSON)},
		},
	}
}

// deleteSyncFsMachineConfig deletes the named MachineConfig, tolerating not-found.
func deleteSyncFsMachineConfig(ctx context.Context, mcClient *machineconfigclient.Clientset, name string) error {
	err := mcClient.MachineconfigurationV1().MachineConfigs().Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// createSyncFsPod returns a minimal, security-hardened pod pinned to nodeName.
// RestartPolicy is Never so test cleanup is predictable.
func createSyncFsPod(name, namespace, img, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			NodeName:      nodeName,
			RestartPolicy: corev1.RestartPolicyNever,
			Tolerations: []corev1.Toleration{
				{Operator: corev1.TolerationOpExists},
			},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:    ptr.To[int64](1000),
				RunAsNonRoot: ptr.To(true),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "test",
					Image:           img,
					Command:         []string{"/agnhost", "pause"},
					ImagePullPolicy: corev1.PullIfNotPresent,
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						RunAsNonRoot:             ptr.To(true),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
		},
	}
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
// Needed when embedding image pull specs in a bash -c script.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
