package machine_config

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	mcfgv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	mcClient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/yaml"
)

// This test is [Serial] because it modifies the state of the images present on Node in each test.
var _ = g.Describe("[sig-mco][OCPFeatureGate:PinnedImages][OCPFeatureGate:MachineConfigNodes][Serial]", func() {
	defer g.GinkgoRecover()
	var (
		MCOPinnedImageBaseDir       = exutil.FixturePath("testdata", "machine_config", "pinnedimage")
		MCOMachineConfigPoolBaseDir = exutil.FixturePath("testdata", "machine_config", "machineconfigpool")
		MCOKubeletConfigBaseDir     = exutil.FixturePath("testdata", "machine_config", "kubeletconfig")

		pinnedImageSetFixture              = filepath.Join(MCOPinnedImageBaseDir, "pis.yaml")
		customMCPFixture                   = filepath.Join(MCOMachineConfigPoolBaseDir, "customMCP.yaml")
		customMCPpinnedImageSetFixture     = filepath.Join(MCOPinnedImageBaseDir, "customMCPpis.yaml")
		customGCMCPpinnedImageSetFixture   = filepath.Join(MCOPinnedImageBaseDir, "customGCMCPpis.yaml")
		customGcKCFixture                  = filepath.Join(MCOKubeletConfigBaseDir, "gcKC.yaml")
		invalidPinnedImageSetFixture       = filepath.Join(MCOPinnedImageBaseDir, "invalidPis.yaml")
		customInvalidPinnedImageSetFixture = filepath.Join(MCOPinnedImageBaseDir, "customInvalidPis.yaml")

		oc = exutil.NewCLIWithoutNamespace("machine-config")

		busyboxImage = "quay.io/openshifttest/busybox@sha256:c5439d7db88ab5423999530349d327b04279ad3161d7596d2126dfb5b02bfd1f"
	)

	g.BeforeEach(func(ctx context.Context) {
		//skip these tests on hypershift platforms
		if ok, _ := exutil.IsHypershift(ctx, oc.AdminConfigClient()); ok {
			g.Skip("MachineConfigNodes is not supported on hypershift. Skipping tests.")
		}
	})

	// Ensure each test pins a separate image, since we are not deleting them after each

	g.It("All Nodes in a custom Pool should have the PinnedImages even after Garbage Collection [apigroup:machineconfiguration.openshift.io]", func() {
		// skip this test on single node platforms
		skipOnSingleNodeTopology(oc)

		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Get KubeClient")

		// Create custom MCP
		err = oc.Run("apply").Args("-f", customMCPFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deleteMCP(oc, "custom")

		// Add node to pool
		nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/worker": ""}).String()})
		o.Expect(err).NotTo(o.HaveOccurred(), "Get all nodes")
		o.Expect(nodes.Items).NotTo(o.BeEmpty())
		err = oc.AsAdmin().Run("label").Args("node", nodes.Items[0].Name, "node-role.kubernetes.io/custom=").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Label node")
		defer deletePinnedImage(oc, nodes.Items[0].Name, busyboxImage)
		defer waitTillNodeReadyWithConfig(kubeClient, nodes.Items[0].Name)
		defer unlabelNode(oc, nodes.Items[0].Name)

		GCPISTest(oc, kubeClient, customGCMCPpinnedImageSetFixture, true, nodes.Items[0].Name, customGcKCFixture)
	})

	g.It("All Nodes in a Custom Pool should have the PinnedImages in PIS [apigroup:machineconfiguration.openshift.io]", func() {
		// skip this test on single node platforms
		skipOnSingleNodeTopology(oc)

		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Get KubeClient")

		// Create custom MCP
		err = oc.Run("apply").Args("-f", customMCPFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deleteMCP(oc, "custom")

		// Add node to pool
		nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/worker": ""}).String()})
		o.Expect(err).NotTo(o.HaveOccurred(), "Get all nodes")
		o.Expect(nodes.Items).NotTo(o.BeEmpty())
		err = oc.AsAdmin().Run("label").Args("node", nodes.Items[0].Name, "node-role.kubernetes.io/custom=").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Label node")
		defer deletePinnedImage(oc, nodes.Items[0].Name, busyboxImage)
		defer waitTillNodeReadyWithConfig(kubeClient, nodes.Items[0].Name)
		defer unlabelNode(oc, nodes.Items[0].Name)

		SimplePISTest(oc, kubeClient, customMCPpinnedImageSetFixture, true)
	})

	g.It("All Nodes in a standard Pool should have the PinnedImages PIS [apigroup:machineconfiguration.openshift.io]", func() {
		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Get KubeClient")

		SimplePISTest(oc, kubeClient, pinnedImageSetFixture, true)
	})

	g.It("Invalid PIS leads to degraded MCN in a standard Pool [apigroup:machineconfiguration.openshift.io]", func() {
		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Get KubeClient")

		SimplePISTest(oc, kubeClient, invalidPinnedImageSetFixture, false)
	})

	g.It("Invalid PIS leads to degraded MCN in a custom Pool [apigroup:machineconfiguration.openshift.io]", func() {
		// skip this test on single node platforms
		skipOnSingleNodeTopology(oc)

		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Get KubeClient")

		// Create custom MCP
		err = oc.Run("apply").Args("-f", customMCPFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer deleteMCP(oc, "custom")

		// Add node to pool
		nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/worker": ""}).String()})
		o.Expect(err).NotTo(o.HaveOccurred(), "Get all nodes")
		o.Expect(nodes.Items).NotTo(o.BeEmpty())
		err = oc.AsAdmin().Run("label").Args("node", nodes.Items[0].Name, "node-role.kubernetes.io/custom=").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Label node")
		defer deletePinnedImage(oc, nodes.Items[0].Name, busyboxImage)
		defer waitTillNodeReadyWithConfig(kubeClient, nodes.Items[0].Name)
		defer unlabelNode(oc, nodes.Items[0].Name)

		SimplePISTest(oc, kubeClient, customInvalidPinnedImageSetFixture, false)
	})

})

func GCPISTest(oc *exutil.CLI, kubeClient *kubernetes.Clientset, fixture string, success bool, nodeName, customGcKCFixture string) {
	clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())

	pis, err := getPISFromFixture(fixture)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "Applied PIS")
	defer deletePIS(oc, pis.Name)

	gcImage := "quay.io/openshifttest/alpine@sha256:dc1536cbff0ba235d4219462aeccd4caceab9def96ae8064257d049166890083"
	_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-machine-config-operator", "crictl", "pull", gcImage)
	o.Expect(err).NotTo(o.HaveOccurred(), "Pulling Alpine Image")
	waitTillImageDownload(oc, nodeName, gcImage)

	// Apply KC to Pool
	err = oc.Run("apply").Args("-f", customGcKCFixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	defer deleteKC(oc, "custom-gc-config")

	waitForReboot(kubeClient, nodeName)
	waitTillImageGC(oc, nodeName, gcImage)

	waitTime := time.Minute * 20
	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
	defer cancel()

	err = waitForPISStatusX(ctx, oc, kubeClient, clientSet, pis.Name, success)
	o.Expect(err).NotTo(o.HaveOccurred(), "Checking status of PIS")
}

func SimplePISTest(oc *exutil.CLI, kubeClient *kubernetes.Clientset, fixture string, success bool) {
	clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())

	pis, err := getPISFromFixture(fixture)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "Applied PIS")
	defer deletePIS(oc, pis.Name)

	waitTime := time.Minute * 20
	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
	defer cancel()

	err = waitForPISStatusX(ctx, oc, kubeClient, clientSet, pis.Name, success)
	o.Expect(err).NotTo(o.HaveOccurred(), "Checking status of PIS")
}

func detectXCondition(oc *exutil.CLI, node corev1.Node, mcn *mcfgv1alpha1.MachineConfigNode, appliedPIS *mcfgv1.PinnedImageSet, detectingSuccess bool) (bool, bool, error) {
	if detectingSuccess {
		for _, cond := range mcn.Status.Conditions {
			if mcfgv1alpha1.StateProgress(cond.Type) == mcfgv1alpha1.MachineConfigNodePinnedImageSetsDegraded && cond.Status == "True" {
				return false, true, fmt.Errorf("PIS degraded for MCN %s with reason: %s and message: %s", mcn.Name, cond.Reason, cond.Message)
			}

			if mcfgv1alpha1.StateProgress(cond.Type) == mcfgv1alpha1.MachineConfigNodePinnedImageSetsProgressing && cond.Status == "True" {
				return false, false, nil
			}
		}
		for _, img := range appliedPIS.Spec.PinnedImages {
			crictlStatus, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, node.Name, "openshift-machine-config-operator", "crictl", "inspecti", string(img.Name))
			if err != nil {
				return false, false, fmt.Errorf("failed to execute `crictl inspecti %s` on node %s: %w", img.Name, node.Name, err)
			}
			if !strings.Contains(crictlStatus, "imageSpec") {
				return false, false, fmt.Errorf("Image %s not present on node %s: %w", img.Name, node.Name, err)
			}
		}
		return true, false, nil
	} else {
		for _, cond := range mcn.Status.Conditions {
			if mcfgv1alpha1.StateProgress(cond.Type) == mcfgv1alpha1.MachineConfigNodePinnedImageSetsDegraded && cond.Status == "True" {
				continue
			}
			if mcfgv1alpha1.StateProgress(cond.Type) == mcfgv1alpha1.MachineConfigNodePinnedImageSetsProgressing && cond.Status == "True" {
				return false, false, nil
			}
		}
		return true, false, nil
	}
}

func waitForPISStatusX(ctx context.Context, oc *exutil.CLI, kubeClient *kubernetes.Clientset, clientSet *mcClient.Clientset, pisName string, success bool) error {
	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (done bool, err error) {
		// Wait for PIS object to get created
		appliedPIS, err := clientSet.MachineconfigurationV1().PinnedImageSets().Get(context.TODO(), pisName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("PIS Object not created yet: %w", err)
		}

		pool, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(ctx, appliedPIS.Labels["machineconfiguration.openshift.io/role"], metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get MCP mentioned in PIS: %w", err)
		}

		nodes, err := getNodesForPool(ctx, oc, kubeClient, pool)
		if err != nil {
			return false, fmt.Errorf("failed to get Nodes from MCP %q  mentioned in PIS: %w", pool.Name, err)
		}

		doneNodes := 0
		for _, node := range nodes.Items {
			mcn, err := clientSet.MachineconfigurationV1alpha1().MachineConfigNodes().Get(ctx, node.Name, metav1.GetOptions{})
			if err != nil {
				return false, fmt.Errorf("failed to get mcn: %w", err)
			}
			toContinue, isFatal, err := detectXCondition(oc, node, mcn, appliedPIS, success)
			if !toContinue {
				if isFatal {
					return true, err
				} else {
					if err != nil {
						framework.Logf("Retrying PIS Status with non-fatal error: %v", err)
					}
					return false, nil
				}
			}
			doneNodes += 1
		}
		if doneNodes == len(nodes.Items) {
			return true, nil
		}

		return false, nil
	})
}

func deletePinnedImage(oc *exutil.CLI, nodeName, imageName string) {
	_, _ = exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-machine-config-operator", "podman", "rmi", imageName)
}

func waitTillImageDownload(oc *exutil.CLI, nodeName, imageName string) {
	o.Eventually(func() bool {
		crictlstatus, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-machine-config-operator", "crictl", "inspecti", imageName)
		if err != nil {
			framework.Logf("Node %v doesn't have Image %v yet", nodeName, imageName)
			return false
		}
		if strings.Contains(crictlstatus, "imageSpec") {
			return true
		}
		return false
	}, 10*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%v' to download image '%v'.", nodeName, imageName)
}

func waitTillImageGC(oc *exutil.CLI, nodeName, imageName string) {
	o.Eventually(func() bool {
		_, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-machine-config-operator", "crictl", "inspecti", imageName)
		if err == nil {
			framework.Logf("Node %v no longer has Image %v", nodeName, imageName)
			return true
		}
		return false
	}, 10*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%v' to garbage collect '%v'.", nodeName, imageName)
}

func waitForReboot(kubeClient *kubernetes.Clientset, nodeName string) {
	o.Eventually(func() bool {
		node, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab Node '%v', error :%v", nodeName, err)
			return false
		}
		if node.Annotations["machineconfiguration.openshift.io/state"] == "Working" {
			framework.Logf("Node '%v' has entered reboot", nodeName)
			return true
		}
		return false
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%v' to start reboot.", nodeName)

	o.Eventually(func() bool {
		node, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab Node '%v', error :%v", nodeName, err)
			return false
		}
		if node.Annotations["machineconfiguration.openshift.io/state"] == "Done" && len(node.Spec.Taints) == 0 {
			framework.Logf("Node '%v' has finished reboot", nodeName)
			return true
		}
		return false
	}, 15*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%v' to finish reboot.", nodeName)
}

func waitTillNodeReadyWithConfig(kubeClient *kubernetes.Clientset, nodeName string) {
	o.Eventually(func() bool {
		node, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab Node '%v', error :%v", nodeName, err)
			return false
		}
		if strings.Contains(node.Annotations["machineconfiguration.openshift.io/currentConfig"], "rendered-worker") && node.Annotations["machineconfiguration.openshift.io/state"] == "Done" {
			framework.Logf("Node '%v' has rendered-worker config", nodeName)
			return true
		}
		return false
	}, 15*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%v' to have rendered-worker config.", nodeName)
}

func unlabelNode(oc *exutil.CLI, name string) error {
	return oc.AsAdmin().Run("label").Args("node", name, "node-role.kubernetes.io/custom-").Execute()
}

func deleteKC(oc *exutil.CLI, name string) error {
	return oc.Run("delete").Args("kubeletconfig", name).Execute()
}

func deleteMCP(oc *exutil.CLI, name string) error {
	return oc.Run("delete").Args("mcp", name).Execute()
}

func deletePIS(oc *exutil.CLI, name string) error {
	return oc.Run("delete").Args("pinnedimageset", name).Execute()
}

func getPISFromFixture(path string) (*mcfgv1.PinnedImageSet, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	ob := new(mcfgv1.PinnedImageSet)
	err = yaml.Unmarshal(data, ob)
	if err != nil {
		return nil, err
	}

	return ob, nil
}
