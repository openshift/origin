package node

import (
	"context"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	"github.com/openshift/origin/test/extended/imagepolicy"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/ptr"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive] ContainerRuntimeConfig", nodeutils.SkipOnMicroShift, func() {
	var (
		oc = exutil.NewCLIWithoutNamespace("ctrcfg")
	)

	g.BeforeEach(func(ctx context.Context) {
		nodeutils.EnsureNodesReady(ctx, oc)
	})

	// Validates that ContainerRuntimeConfig pidsLimit setting is correctly applied
	// by MCO to a single worker node and that manual crio.conf edits are overwritten.
	//author: cmaurya@redhat.com
	g.It("[OTP] Verify pidsLimit and MCO overwrite behavior [OCP-45351]", func() {
		ctx := context.Background()
		ctrcfgName := "set-pids-limit"
		mcpName := "ctrcfg-pids"

		g.By("Get a ready worker node")
		workers, err := exutil.GetReadySchedulableWorkerNodes(ctx, oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get ready schedulable worker nodes")
		o.Expect(workers).NotTo(o.BeEmpty(), "No Ready worker nodes found")
		workerNode := workers[0].Name

		g.By("Make a manual change to crio.conf on worker node")
		_, err = nodeutils.ExecOnNodeWithChroot(ctx, oc, workerNode,
			"/bin/bash", "-c", `sed -i '/^\[crio\.runtime\]/a log_level = "debug"' /etc/crio/crio.conf`)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to edit crio.conf on node %s", workerNode)

		g.By("Verify the manual crio.conf edit took effect")
		editedConf, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, workerNode, "cat", "/etc/crio/crio.conf")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read crio.conf on node %s", workerNode)
		o.Expect(editedConf).To(o.ContainSubstring(`log_level = "debug"`),
			"sed edit did not apply: expected log_level = debug in crio.conf")

		mcpConfig := createSingleNodeMCP(ctx, oc, mcpName, workerNode)

		g.DeferCleanup(func() {
			g.By("Cleanup: delete ContainerRuntimeConfig")
			delErr := oc.MachineConfigurationClient().MachineconfigurationV1().ContainerRuntimeConfigs().Delete(
				ctx, ctrcfgName, metav1.DeleteOptions{})
			if !apierrors.IsNotFound(delErr) {
				o.Expect(delErr).NotTo(o.HaveOccurred(),
					"cleanup failed: could not delete ContainerRuntimeConfig %s", ctrcfgName)
			}
			cleanupSingleNodeMCP(ctx, mcpConfig)
		})

		initialSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, mcpName)

		g.By("Create ContainerRuntimeConfig with pidsLimit 2048")
		ctrcfg := &mcfgv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{Name: ctrcfgName},
			Spec: mcfgv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"machineconfiguration.openshift.io/pool": mcpName},
				},
				ContainerRuntimeConfig: &mcfgv1.ContainerRuntimeConfiguration{
					PidsLimit: ptr.To[int64](2048),
				},
			},
		}
		_, err = oc.MachineConfigurationClient().MachineconfigurationV1().ContainerRuntimeConfigs().Create(
			ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ContainerRuntimeConfig")

		g.By("Wait for custom MCP rollout to complete")
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, mcpName, initialSpec)
		e2e.Logf("Worker node rolled out successfully")

		g.By("Verify pidsLimit and conmon in crio config on worker node")
		var crioConfig string
		o.Eventually(func() error {
			var execErr error
			crioConfig, execErr = nodeutils.ExecOnNodeWithChroot(ctx, oc, workerNode,
				"/bin/bash", "-c", "crio config 2>/dev/null")
			return execErr
		}, 30*time.Second, 5*time.Second).Should(o.Succeed(), "failed to get crio config on node %s", workerNode)
		o.Expect(crioConfig).To(o.ContainSubstring("pids_limit = 2048"), "pidsLimit should be 2048")
		o.Expect(crioConfig).To(o.ContainSubstring(`conmon = ""`), "conmon should be empty")
		o.Expect(crioConfig).NotTo(o.ContainSubstring(`log_level = "debug"`),
			"manual crio.conf edit should be overwritten by MCO")
	})

	// Validates that setting overlaySize in ContainerRuntimeConfig is applied to
	// storage.conf on a single worker node and the overlay size is reflected inside a container.
	//author: cmaurya@redhat.com
	g.It("[OTP] Verify overlaySize is applied to node and container [OCP-46313]", func() {
		oc.SetupProject()
		ctx := context.Background()
		ctrcfgName := "ctrcfg-46313"
		mcpName := "ctrcfg-overlay"
		overlaySize := "9G"

		g.By("Get a ready worker node")
		workers, err := exutil.GetReadySchedulableWorkerNodes(ctx, oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get ready schedulable worker nodes")
		o.Expect(workers).NotTo(o.BeEmpty(), "No Ready worker nodes found")
		workerNode := workers[0].Name

		mcpConfig := createSingleNodeMCP(ctx, oc, mcpName, workerNode)

		g.DeferCleanup(func() {
			g.By("Cleanup: delete ContainerRuntimeConfig")
			delErr := oc.MachineConfigurationClient().MachineconfigurationV1().ContainerRuntimeConfigs().Delete(
				ctx, ctrcfgName, metav1.DeleteOptions{})
			if !apierrors.IsNotFound(delErr) {
				o.Expect(delErr).NotTo(o.HaveOccurred(),
					"cleanup failed: could not delete ContainerRuntimeConfig %s", ctrcfgName)
			}
			cleanupSingleNodeMCP(ctx, mcpConfig)
		})

		initialSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, mcpName)

		g.By("Create ContainerRuntimeConfig with overlaySize " + overlaySize)
		quantity := resource.MustParse(overlaySize)
		ctrcfg := &mcfgv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{Name: ctrcfgName},
			Spec: mcfgv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"machineconfiguration.openshift.io/pool": mcpName},
				},
				ContainerRuntimeConfig: &mcfgv1.ContainerRuntimeConfiguration{
					OverlaySize: &quantity,
				},
			},
		}
		_, err = oc.MachineConfigurationClient().MachineconfigurationV1().ContainerRuntimeConfigs().Create(
			ctx, ctrcfg, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ContainerRuntimeConfig")

		g.By("Wait for custom MCP rollout to complete")
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, mcpName, initialSpec)
		e2e.Logf("Worker node rolled out successfully")

		g.By("Check overlaySize takes effect in storage.conf on worker node")
		storageConf, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, workerNode,
			"/bin/bash", "-c", "head -n 7 /etc/containers/storage.conf | grep size")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read storage.conf on node %s", workerNode)
		e2e.Logf("storage.conf size line: %s", storageConf)
		o.Expect(storageConf).To(o.ContainSubstring(overlaySize),
			"storage.conf should contain size = %s", overlaySize)

		g.By("Create a pod on the target node to verify overlay size inside container")
		podName := "pod-46313"
		ns := oc.Namespace()
		err = oc.AsAdmin().WithoutNamespace().Run("run").Args(
			podName, "-n", ns,
			"--image=quay.io/openshifttest/hello-openshift@sha256:56c354e7885051b6bb4263f9faa58b2c292d44790599b7dde0e49e7c466cf339",
			"--restart=Never",
			"--overrides", `{"spec":{"nodeName":"`+workerNode+`","securityContext":{"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}},"containers":[{"name":"`+podName+`","image":"quay.io/openshifttest/hello-openshift@sha256:56c354e7885051b6bb4263f9faa58b2c292d44790599b7dde0e49e7c466cf339","command":["/bin/bash","-c","sleep 100000000"],"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}}]}}`,
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create pod")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", podName, "-n", ns, "--ignore-not-found").Execute()

		g.By("Wait for pod to be running")
		err = wait.Poll(5*time.Second, 5*time.Minute, func() (bool, error) {
			phase, pollErr := oc.AsAdmin().WithoutNamespace().Run("get").Args(
				"pod", podName, "-n", ns, "-o=jsonpath={.status.phase}").Output()
			if pollErr != nil {
				return false, nil
			}
			return phase == "Running", nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "pod did not reach Running state")

		g.By("Check overlay filesystem size inside the container")
		dfOutput, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args(
			"-n", ns, podName, "/bin/bash", "-c", "df -h / | grep overlay").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to exec df inside pod")
		e2e.Logf("overlay df output: %s", dfOutput)
		fields := strings.Fields(dfOutput)
		o.Expect(len(fields)).To(o.BeNumerically(">=", 2), "unexpected df output format: %s", dfOutput)
		actualSize := strings.Split(strings.TrimSuffix(fields[1], "G"), ".")[0] + "G"
		o.Expect(actualSize).To(o.Equal(overlaySize),
			"overlay filesystem should show %s, got: %s", overlaySize, actualSize)
	})
})

// createSingleNodeMCP creates a custom MachineConfigPool that targets exactly one worker node.
// It uses the shared helper from node_mcp_helpers.go and returns the config for cleanup.
func createSingleNodeMCP(ctx context.Context, oc *exutil.CLI, mcpName, workerNode string) *nodeutils.CustomMCPConfig {
	mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to create machine config client")

	mcpConfig, err := nodeutils.CreateCustomMCPForNode(ctx, oc, mcClient, mcpName, workerNode)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to create custom MCP")

	return mcpConfig
}

// cleanupSingleNodeMCP removes the node label, waits for the node to transition back to the
// worker pool config, and then deletes the custom MCP.
// It uses the shared helper from node_mcp_helpers.go.
func cleanupSingleNodeMCP(ctx context.Context, mcpConfig *nodeutils.CustomMCPConfig) {
	if mcpConfig == nil {
		return
	}

	err := nodeutils.CleanupCustomMCP(ctx, mcpConfig)
	if err != nil {
		e2e.Logf("WARNING: cleanup had errors: %v", err)
	}
}
