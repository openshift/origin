package node

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

// syncFsDropInPath is the storage.conf drop-in file injected by the MachineConfig.
// Placed at 99-* so it takes lexicographic precedence over the base storage.conf.
const (
	syncFsDropInPath    = "/etc/containers/storage.conf.d/99-syncfs.conf"
	syncFsDropInContent = "[storage.options.overlay]\nsync = \"filesystem\"\n"
)

// CRI-O overlay.sync=filesystem E2E test suite.
//
// overlay.sync=filesystem is an opt-in containers-storage/CRI-O durability option that
// calls syncfs() before committing layer metadata to disk, so that an unclean node
// shutdown mid-pull cannot leave the image store in a corrupted or partially-committed
// state.
//
// This suite verifies, end-to-end on an OpenShift cluster, that:
//   - The option can be enabled via a MachineConfig storage.conf drop-in and is picked
//     up by CRI-O without disrupting the node.
//   - Normal image pulls and pod startup continue to work with the option enabled.
//   - Concurrent image pulls do not deadlock or corrupt the store while the option is
//     enabled.
//   - Disabling the option again (removing the MachineConfig) is safe: previously
//     pulled images and CRI-O itself remain intact and functional afterward.

var _ = g.Describe("[sig-node][Jira:Node/CRI-O][apigroup:machineconfiguration.openshift.io][Serial][Disruptive][Suite:openshift/machine-config-operator/disruptive] CRI-O overlay.sync=filesystem", func() {
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
		g.By("Enabling overlay.sync=filesystem via a MachineConfig storage.conf drop-in")
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

	// Verify the MachineConfig storage.conf drop-in is deployed and CRI-O is active after the configuration change
	// Verifies that overlay.sync=filesystem can be enabled on a node via a
	// MachineConfig storage.conf drop-in and that CRI-O accepts the new
	// configuration without disruption.
	g.It("should accept overlay.sync=filesystem configuration via a MachineConfig drop-in", func(ctx context.Context) {
		applyMachineConfig(ctx)

		g.By("Step 1: Verifying the storage.conf drop-in was deployed to the node")
		dropIn, err := ExecOnNodeWithChroot(ctx, oc, testNode, "cat", syncFsDropInPath)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(dropIn).To(o.ContainSubstring(`sync = "filesystem"`),
			"storage.conf drop-in should contain sync = \"filesystem\"")
		framework.Logf("Drop-in file contents:\n%s", dropIn)

		g.By("Step 2: Verifying CRI-O is active after the configuration change")
		crioStatus, err := ExecOnNodeWithChroot(ctx, oc, testNode, "systemctl", "is-active", "crio")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(crioStatus)).To(o.Equal("active"),
			"CRI-O should remain active after enabling sync=filesystem")
		framework.Logf("Configuration check passed: overlay.sync=filesystem is applied and CRI-O is active")
	})

	// Verifies that with overlay.sync=filesystem enabled, a fresh image pull
	// succeeds and a pod using that image reaches the Running phase.
	g.It("should allow image pulls and pod startup with overlay.sync=filesystem enabled", func(ctx context.Context) {
		applyMachineConfig(ctx)

		testNamespace := oc.Namespace()
		testImage := image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.63.0")

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
			image.LocationFor("registry.k8s.io/e2e-test-images/busybox:1.37.0-1"),
			image.LocationFor("registry.k8s.io/e2e-test-images/nginx:1.15-4"),
			image.LocationFor("registry.k8s.io/e2e-test-images/sample-device-plugin:1.7"),
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
		testImage := image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.63.0")

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

		g.By("Step 3: Verifying the storage.conf drop-in has been removed from the node")
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

// newSyncFsMachineConfig returns a MachineConfig that injects a storage.conf drop-in
// enabling overlay.sync=filesystem for the specified MachineConfigPool role.
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
