package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

// This test suite validates that the kubelet TLS configuration can be upgraded
// from TLS 1.2 to TLS 1.3 via a KubeletConfig resource applied to the worker
// MachineConfigPool. It is disruptive because applying KubeletConfig triggers
// a rolling reboot of all worker nodes.
var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive] Kubelet TLS configuration", func() {
	defer g.GinkgoRecover()
	var (
		oc                = exutil.NewCLIWithoutNamespace("node-kubeletconfig-tls")
		kubeletConfigName = "worker-tls13-config"
	)

	// skipUnsupportedTopologies skips this test on cluster topologies that lack
	// dedicated worker nodes or where an MCP rolling update would be unsafe.
	skipUnsupportedTopologies := func() {
		skipOnSingleNodeTopology(oc)
		skipOnTwoNodeTopology(oc)

		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("Skipping test on External (Hypershift) topology - MachineConfig API not available")
		}
	}

	// This test verifies that KubeletConfig can change the kubelet TLS configuration from TLS 1.2 to TLS 1.3
	// by applying a KubeletConfig to the worker pool, waiting for the MCP rollout, and confirming the change.
	g.It("should upgrade kubelet TLS from 1.2 to 1.3 on worker pool [apigroup:machineconfiguration.openshift.io]", func(ctx context.Context) {
		skipUnsupportedTopologies()

		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting kube client: %v", err))

		mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating machine configuration client")

		workerPool := "worker"

		framework.Logf("1) Check all worker nodes are in Ready state")
		workerNodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/worker": ""}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Error listing worker nodes")
		o.Expect(len(workerNodes.Items)).To(o.BeNumerically(">", 0), "No worker nodes found in the cluster")

		for _, node := range workerNodes.Items {
			isReady := false
			for _, condition := range node.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" {
					isReady = true
					break
				}
			}
			o.Expect(isReady).To(o.BeTrue(), "Worker node %s is not in Ready state", node.Name)
		}
		framework.Logf("All %d worker nodes are in Ready state", len(workerNodes.Items))

		framework.Logf("2) Check default TLS configuration on worker nodes")
		testNode := workerNodes.Items[0].Name
		framework.Logf("Checking kubelet.conf on node '%s'", testNode)

		defaultTLSVersion, err := getKubeletTLSVersion(oc, testNode)
		o.Expect(err).NotTo(o.HaveOccurred(), "Error reading default TLS version from node %s", testNode)
		framework.Logf("Default TLS configuration - Version: '%s'", defaultTLSVersion)

		if defaultTLSVersion == "VersionTLS13" {
			g.Skip("Worker kubelet is already on TLS 1.3; nothing to upgrade (leaked KubeletConfig or product default changed)")
		}
		if defaultTLSVersion != "" && defaultTLSVersion != "VersionTLS12" {
			framework.Failf("Unexpected default TLS version %q, expected VersionTLS12 or empty (cluster default)", defaultTLSVersion)
		}

		framework.Logf("3) Creating KubeletConfig with TLS 1.3 on worker pool")
		kubeletConfig := &mcfgv1.KubeletConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: kubeletConfigName,
			},
			Spec: mcfgv1.KubeletConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pools.operator.machineconfiguration.openshift.io/worker": "",
					},
				},
				KubeletConfig: &runtime.RawExtension{
					Raw: []byte(`{"tlsMinVersion":"VersionTLS13"}`),
				},
			},
		}

		g.DeferCleanup(func() {
			framework.Logf("Cleanup: deleting KubeletConfig %s", kubeletConfigName)
			cleanupCtx := context.Background()
			deleteErr := mcClient.MachineconfigurationV1().KubeletConfigs().Delete(cleanupCtx, kubeletConfigName, metav1.DeleteOptions{})
			if apierrors.IsNotFound(deleteErr) {
				return
			}
			o.Expect(deleteErr).NotTo(o.HaveOccurred(), "Cleanup: failed to delete KubeletConfig %s", kubeletConfigName)

			framework.Logf("Cleanup: waiting for worker MCP to become ready after KubeletConfig deletion")
			waitErr := waitForMCP(cleanupCtx, mcClient, workerPool, 60*time.Minute)
			o.Expect(waitErr).NotTo(o.HaveOccurred(), "Cleanup: worker MCP did not become ready after KubeletConfig deletion")
		})

		_, err = mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, kubeletConfig, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating KubeletConfig with TLS 1.3")

		framework.Logf("4) Waiting for worker MachineConfigPool to complete rollout")
		framework.Logf("This will take some time as all worker nodes will be updated in a rolling fashion")

		framework.Logf("Waiting for MachineConfigPool %q to start updating", workerPool)
		o.Eventually(func() bool {
			mcp, getErr := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, workerPool, metav1.GetOptions{})
			if getErr != nil {
				framework.Logf("Error getting %s MCP: %v", workerPool, getErr)
				return false
			}
			for _, condition := range mcp.Status.Conditions {
				if condition.Type == "Updating" && condition.Status == corev1.ConditionTrue {
					return true
				}
			}
			return false
		}, 10*time.Minute, 15*time.Second).Should(o.BeTrue(),
			"Timed out waiting for MachineConfigPool %q to start updating", workerPool)

		err = waitForMCP(ctx, mcClient, workerPool, 60*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Error waiting for MachineConfigPool %q to become ready", workerPool)

		framework.Logf("Worker MachineConfigPool has completed rollout")

		framework.Logf("5) Verifying all worker nodes are in Ready state after rollout")
		workerNodes, err = kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/worker": ""}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Error listing worker nodes after update")

		for _, node := range workerNodes.Items {
			isReady := false
			for _, condition := range node.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" {
					isReady = true
					break
				}
			}
			o.Expect(isReady).To(o.BeTrue(), "Worker node %s is not in Ready state after rollout", node.Name)

			nodeState := node.Annotations["machineconfiguration.openshift.io/state"]
			o.Expect(nodeState).To(o.Equal("Done"), "Node %s is in state '%s', expected 'Done'", node.Name, nodeState)
		}
		framework.Logf("All %d worker nodes are in Ready state after rollout", len(workerNodes.Items))

		framework.Logf("6) Verifying TLS version upgraded to 1.3")

		expectedTLSVersion := "VersionTLS13"
		actualTLSVersion, err := getKubeletTLSVersion(oc, testNode)
		o.Expect(err).NotTo(o.HaveOccurred(), "Error reading TLS version from node %s after rollout", testNode)

		o.Expect(actualTLSVersion).To(o.Equal(expectedTLSVersion),
			"TLS version should be '%s', but got '%s'", expectedTLSVersion, actualTLSVersion)

		framework.Logf("Successfully verified kubelet TLS upgrade from %s to %s",
			defaultTLSVersion, actualTLSVersion)
	})
})

// getKubeletTLSVersion reads kubelet.conf from the specified node via a
// retrying debug pod in the openshift-machine-config-operator namespace
// (which carries the privileged pod-security label) and extracts the
// tlsMinVersion value. Returns ("", nil) when no tlsMinVersion line is
// found (i.e. the cluster default is in effect).
func getKubeletTLSVersion(oc *exutil.CLI, nodeName string) (string, error) {
	kubeletConf, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, nodeName, "openshift-machine-config-operator",
		"cat", "/etc/kubernetes/kubelet.conf",
	)
	if err != nil {
		return "", fmt.Errorf("failed to read kubelet.conf from node %s: %w", nodeName, err)
	}

	for _, line := range strings.Split(kubeletConf, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "tlsMinVersion:") {
			parts := strings.SplitN(trimmedLine, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", nil
}
