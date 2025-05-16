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

const emptyImagePin string = "localhost.localdomain/emptyimagepin"

// This test is [Serial] because it modifies the state of the images present on Node in each test.
var _ = g.Describe("[sig-mco][OCPFeatureGate:PinnedImages][OCPFeatureGate:MachineConfigNodes][Serial]", func() {
	defer g.GinkgoRecover()
	var (
		MCOPinnedImageBaseDir       = exutil.FixturePath("testdata", "machine_config", "pinnedimage")
		MCOMachineConfigPoolBaseDir = exutil.FixturePath("testdata", "machine_config", "machineconfigpool")
		MCOKubeletConfigBaseDir     = exutil.FixturePath("testdata", "machine_config", "kubeletconfig")

		pinnedImageSetFixture              = filepath.Join(MCOPinnedImageBaseDir, "pis.yaml")
		masterPinnedImageSetFixture        = filepath.Join(MCOPinnedImageBaseDir, "masterPis.yaml")
		customMCPFixture                   = filepath.Join(MCOMachineConfigPoolBaseDir, "customMCP.yaml")
		customMCPpinnedImageSetFixture     = filepath.Join(MCOPinnedImageBaseDir, "customMCPpis.yaml")
		customGCMCPpinnedImageSetFixture   = filepath.Join(MCOPinnedImageBaseDir, "customGCMCPpis.yaml")
		customGcKCFixture                  = filepath.Join(MCOKubeletConfigBaseDir, "gcKC.yaml")
		invalidPinnedImageSetFixture       = filepath.Join(MCOPinnedImageBaseDir, "invalidPis.yaml")
		masterInvalidPinnedImageSetFixture = filepath.Join(MCOPinnedImageBaseDir, "masterInvalidPis.yaml")
		customInvalidPinnedImageSetFixture = filepath.Join(MCOPinnedImageBaseDir, "customInvalidPis.yaml")

		oc = exutil.NewCLIWithoutNamespace("machine-config")

		alpineImage  = "quay.io/openshifttest/alpine@sha256:dc1536cbff0ba235d4219462aeccd4caceab9def96ae8064257d049166890083"
		emptyImageGc = "localhost.localdomain/emptyimagegc"
	)

	g.BeforeEach(func(ctx context.Context) {
		//skip these tests on hypershift platforms
		if ok, _ := exutil.IsHypershift(ctx, oc.AdminConfigClient()); ok {
			g.Skip("PinnedImages is not supported on hypershift. Skipping tests.")
		}
	})

	g.It("All Nodes in a custom Pool should have the PinnedImages even after Garbage Collection [apigroup:machineconfiguration.openshift.io]", func() {

		// skip this test on single node platforms
		skipOnSingleNodeTopology(oc)

		pisFixture := customGCMCPpinnedImageSetFixture
		mcpFixture := customMCPFixture
		kcFixture := customGcKCFixture

		pis, err := getPISFromFixture(pisFixture)
		o.Expect(err).NotTo(o.HaveOccurred())
		pisDiverged := false

		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Get KubeClient")

		clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		// Create custom MCP
		defer deleteMCP(oc, "custom")
		err = oc.Run("apply").Args("-f", mcpFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Add node to pool
		optedNodes, err := addWorkerNodesToCustomPool(oc, kubeClient, 1, "custom")
		o.Expect(err).NotTo(o.HaveOccurred(), "Label node")
		defer waitTillNodeReadyWithConfig(kubeClient, optedNodes[0])
		defer unlabelNode(oc, optedNodes[0])

		isMetalDisconnected := false
		gcImage := alpineImage
		if IsMetal(oc) && IsDisconnected(oc, optedNodes[0]) {
			isMetalDisconnected = true
			pinnedImage := emptyImagePin
			_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, optedNodes[0], "openshift-machine-config-operator", "podman", "import", "--change", "LABEL=build=p", "/dev/null", pinnedImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Creating Pinned Image")
			pinnedImage, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, optedNodes[0], "openshift-machine-config-operator", "podman", "inspect", "--format", "'{{ index .RepoDigests 0 }}'", pinnedImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Getting ImageDigestFormat of Pinned Image")
			pis.Spec.PinnedImages[0].Name = mcfgv1.ImageDigestFormat(strings.Trim(pinnedImage, "'"))
			pisDiverged = true

			gcImage = emptyImageGc
			_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, optedNodes[0], "openshift-machine-config-operator", "podman", "import", "--change", "LABEL=build=g", "/dev/null", gcImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Creating GC Image")
		} else {
			_, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, optedNodes[0], "openshift-machine-config-operator", "crictl", "pull", gcImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Pulling Alpine Image")
			waitTillImageDownload(oc, optedNodes[0], gcImage)
		}

		var pinnedImages []string
		for _, img := range pis.Spec.PinnedImages {
			pinnedImages = append(pinnedImages, string(img.Name))
		}

		// Apply PIS
		defer deletePinnedImages(oc, kubeClient, clientSet, optedNodes, pinnedImages, isMetalDisconnected)
		defer deletePIS(oc, pis.Name)
		err = applyPIS(oc, pisFixture, pis, pisDiverged)
		o.Expect(err).NotTo(o.HaveOccurred(), "Applied PIS")

		GCPISTest(oc, kubeClient, clientSet, true, optedNodes[0], kcFixture, gcImage, pis.Name, isMetalDisconnected)
	})

	g.It("All Nodes in a Custom Pool should have the PinnedImages in PIS [apigroup:machineconfiguration.openshift.io]", func() {

		// skip this test on single node platforms
		skipOnSingleNodeTopology(oc)

		pisFixture := customMCPpinnedImageSetFixture
		mcpFixture := customMCPFixture

		pis, err := getPISFromFixture(pisFixture)
		o.Expect(err).NotTo(o.HaveOccurred())
		pisDiverged := false

		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Get KubeClient")

		clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		// Create custom MCP
		defer deleteMCP(oc, "custom")
		err = oc.Run("apply").Args("-f", mcpFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Add node to pool
		optedNodes, err := addWorkerNodesToCustomPool(oc, kubeClient, 1, "custom")
		o.Expect(err).NotTo(o.HaveOccurred(), "Label node")
		defer waitTillNodeReadyWithConfig(kubeClient, optedNodes[0])
		defer unlabelNode(oc, optedNodes[0])

		isMetalDisconnected := false
		if IsMetal(oc) && IsDisconnected(oc, optedNodes[0]) {
			isMetalDisconnected = true
			pinnedImage := emptyImagePin
			_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, optedNodes[0], "openshift-machine-config-operator", "podman", "import", "--change", "LABEL=build=p", "/dev/null", pinnedImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Creating Pinned Image")
			pinnedImage, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, optedNodes[0], "openshift-machine-config-operator", "podman", "inspect", "--format", "'{{ index .RepoDigests 0 }}'", pinnedImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Getting ImageDigestFormat of Pinned Image")
			pis.Spec.PinnedImages[0].Name = mcfgv1.ImageDigestFormat(strings.Trim(pinnedImage, "'"))
			pisDiverged = true
		}

		var pinnedImages []string
		for _, img := range pis.Spec.PinnedImages {
			pinnedImages = append(pinnedImages, string(img.Name))
		}

		// Apply PIS
		defer deletePinnedImages(oc, kubeClient, clientSet, optedNodes, pinnedImages, isMetalDisconnected)
		defer deletePIS(oc, pis.Name)
		err = applyPIS(oc, pisFixture, pis, pisDiverged)
		o.Expect(err).NotTo(o.HaveOccurred(), "Applied PIS")

		SimplePISTest(oc, kubeClient, clientSet, true, pis.Name, isMetalDisconnected)
	})

	g.It("All Nodes in a standard Pool should have the PinnedImages PIS [apigroup:machineconfiguration.openshift.io]", func() {

		// Since the node in a SNO cluster is a part of the master MCP, the PIS for this test on a
		// single node topology should target `master`.
		pisFixture := pinnedImageSetFixture
		if IsSingleNode(oc) {
			pisFixture = masterPinnedImageSetFixture
		}

		pis, err := getPISFromFixture(pisFixture)
		o.Expect(err).NotTo(o.HaveOccurred())
		pisDiverged := false

		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Get KubeClient")

		clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		var optedNodes []string
		nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/worker": ""}).String()})
		o.Expect(err).NotTo(o.HaveOccurred(), "Getting nodes from standard pool")
		for _, node := range nodes.Items {
			optedNodes = append(optedNodes, node.Name)
		}

		isMetalDisconnected := false
		if IsMetal(oc) && IsDisconnected(oc, optedNodes[0]) {
			isMetalDisconnected = true
			pinnedImage := emptyImagePin
			for _, node := range optedNodes {
				_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, node, "openshift-machine-config-operator", "podman", "import", "--change", "LABEL=build=p", "/dev/null", pinnedImage)
				o.Expect(err).NotTo(o.HaveOccurred(), "Creating Pinned Image on Node %s", node)
			}
			pinnedImage, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, optedNodes[0], "openshift-machine-config-operator", "podman", "inspect", "--format", "'{{ index .RepoDigests 0 }}'", pinnedImage)
			o.Expect(err).NotTo(o.HaveOccurred(), "Getting ImageDigestFormat of Pinned Image")
			pis.Spec.PinnedImages[0].Name = mcfgv1.ImageDigestFormat(strings.Trim(pinnedImage, "'"))
			pisDiverged = true
		}

		var pinnedImages []string
		for _, img := range pis.Spec.PinnedImages {
			pinnedImages = append(pinnedImages, string(img.Name))
		}

		// Apply PIS
		defer deletePinnedImages(oc, kubeClient, clientSet, optedNodes, pinnedImages, isMetalDisconnected)
		defer deletePIS(oc, pis.Name)
		err = applyPIS(oc, pisFixture, pis, pisDiverged)
		o.Expect(err).NotTo(o.HaveOccurred(), "Applied PIS")

		SimplePISTest(oc, kubeClient, clientSet, true, pis.Name, isMetalDisconnected)
	})

	g.It("Invalid PIS leads to degraded MCN in a standard Pool [apigroup:machineconfiguration.openshift.io]", func() {

		// Since the node in a SNO cluster is a part of the master MCP, the PIS for this test on a
		// single node topology should target `master`.
		pisFixture := invalidPinnedImageSetFixture
		if IsSingleNode(oc) {
			pisFixture = masterInvalidPinnedImageSetFixture
		}

		pis, err := getPISFromFixture(pisFixture)
		o.Expect(err).NotTo(o.HaveOccurred())

		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Get KubeClient")

		clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		// Apply PIS
		defer deletePIS(oc, pis.Name)
		err = oc.Run("apply").Args("-f", pisFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Applied PIS")

		SimplePISTest(oc, kubeClient, clientSet, false, pis.Name, false)
	})

	g.It("Invalid PIS leads to degraded MCN in a custom Pool [apigroup:machineconfiguration.openshift.io]", func() {

		// skip this test on single node platforms
		skipOnSingleNodeTopology(oc)

		pisFixture := customInvalidPinnedImageSetFixture
		mcpFixture := customMCPFixture

		pis, err := getPISFromFixture(pisFixture)
		o.Expect(err).NotTo(o.HaveOccurred())

		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Get KubeClient")

		clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		// Create custom MCP
		defer deleteMCP(oc, "custom")
		err = oc.Run("apply").Args("-f", mcpFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Add node to pool
		optedNodes, err := addWorkerNodesToCustomPool(oc, kubeClient, 1, "custom")
		o.Expect(err).NotTo(o.HaveOccurred(), "Label node")
		defer waitTillNodeReadyWithConfig(kubeClient, optedNodes[0])
		defer unlabelNode(oc, optedNodes[0])

		// Apply PIS
		defer deletePIS(oc, pis.Name)
		err = oc.Run("apply").Args("-f", pisFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Applied PIS")

		SimplePISTest(oc, kubeClient, clientSet, false, pis.Name, false)
	})

})

func applyPIS(oc *exutil.CLI, pisFixture string, pis *mcfgv1.PinnedImageSet, pisDiverged bool) error {
	if pisDiverged {
		yamlData, err := yaml.Marshal(&pis)
		if err != nil {
			return err
		}
		err = oc.Run("apply").Args("-f", "-").InputString(string(yamlData)).Execute()
		if err != nil {
			return err
		}
	} else {
		err := oc.Run("apply").Args("-f", pisFixture).Execute()
		if err != nil {
			return err
		}
	}
	return nil
}

func addWorkerNodesToCustomPool(oc *exutil.CLI, kubeClient *kubernetes.Clientset, numberOfNodes int, customMCP string) ([]string, error) {
	nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/worker": ""}).String()})
	if err != nil {
		return nil, err
	}
	if len(nodes.Items) < numberOfNodes {
		return nil, fmt.Errorf("Node in Worker MCP %d < Number of nodes needed in %d MCP", len(nodes.Items), numberOfNodes)
	}
	var optedNodes []string
	for node_i := 0; node_i < numberOfNodes; node_i++ {
		err = oc.AsAdmin().Run("label").Args("node", nodes.Items[node_i].Name, fmt.Sprintf("node-role.kubernetes.io/%s=", customMCP)).Execute()
		if err != nil {
			return nil, err
		}
		optedNodes = append(optedNodes, nodes.Items[node_i].Name)
	}
	return optedNodes, nil
}

func GCPISTest(oc *exutil.CLI, kubeClient *kubernetes.Clientset, clientSet *mcClient.Clientset, success bool, nodeName, customGcKCFixture, gcImage, pisName string, isMetalDisconnected bool) {

	// Apply KC to Pool
	defer deleteKC(oc, "custom-gc-config")
	err := oc.Run("apply").Args("-f", customGcKCFixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	waitForReboot(kubeClient, nodeName)
	waitTillImageGC(oc, nodeName, gcImage)

	waitTime := time.Minute * 20
	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
	defer cancel()

	err = waitForPISStatusX(ctx, oc, kubeClient, clientSet, pisName, success, isMetalDisconnected)
	o.Expect(err).NotTo(o.HaveOccurred(), "Checking status of PIS")
}

func SimplePISTest(oc *exutil.CLI, kubeClient *kubernetes.Clientset, clientSet *mcClient.Clientset, success bool, pisName string, isMetalDisconnected bool) {

	waitTime := time.Minute * 20
	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
	defer cancel()

	err := waitForPISStatusX(ctx, oc, kubeClient, clientSet, pisName, success, isMetalDisconnected)
	o.Expect(err).NotTo(o.HaveOccurred(), "Checking status of PIS")
}

func detectXCondition(oc *exutil.CLI, node corev1.Node, mcn *mcfgv1.MachineConfigNode, appliedPIS *mcfgv1.PinnedImageSet, detectingSuccess bool, isMetalDisconnected bool) (bool, bool, error) {
	if detectingSuccess {
		for _, cond := range mcn.Status.Conditions {
			if mcfgv1.StateProgress(cond.Type) == mcfgv1.MachineConfigNodePinnedImageSetsDegraded && cond.Status == "True" {
				return false, true, fmt.Errorf("PIS degraded for MCN %s with reason: %s and message: %s", mcn.Name, cond.Reason, cond.Message)
			}

			if mcfgv1.StateProgress(cond.Type) == mcfgv1.MachineConfigNodePinnedImageSetsProgressing && cond.Status == "True" {
				return false, false, nil
			}
		}
		for _, img := range appliedPIS.Spec.PinnedImages {
			if isMetalDisconnected {
				crictlStatus, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, node.Name, "openshift-machine-config-operator", "crictl", "inspecti", emptyImagePin)
				if err != nil {
					return false, false, fmt.Errorf("failed to execute `crictl inspecti %s` on node %s: %w", img.Name, node.Name, err)
				}
				if !strings.Contains(crictlStatus, "imageSpec") {
					return false, false, fmt.Errorf("Image %s not present on node %s: %w", img.Name, node.Name, err)
				}
				break
			}
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
			if mcfgv1.StateProgress(cond.Type) == mcfgv1.MachineConfigNodePinnedImageSetsDegraded && cond.Status == "True" {
				continue
			}
			if mcfgv1.StateProgress(cond.Type) == mcfgv1.MachineConfigNodePinnedImageSetsProgressing && cond.Status == "True" {
				return false, false, nil
			}
		}
		return true, false, nil
	}
}

func waitForPISStatusX(ctx context.Context, oc *exutil.CLI, kubeClient *kubernetes.Clientset, clientSet *mcClient.Clientset, pisName string, success bool, isMetalDisconnected bool) error {
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
			mcn, err := clientSet.MachineconfigurationV1().MachineConfigNodes().Get(ctx, node.Name, metav1.GetOptions{})
			if err != nil {
				return false, fmt.Errorf("failed to get mcn: %w", err)
			}
			toContinue, isFatal, err := detectXCondition(oc, node, mcn, appliedPIS, success, isMetalDisconnected)
			if !toContinue {
				if isFatal {
					return true, err
				} else {
					if err != nil {
						framework.Logf("Retrying PIS Status with non-fatal error: %s", err)
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

func deletePinnedImages(oc *exutil.CLI, kubeClient *kubernetes.Clientset, clientSet *mcClient.Clientset, optedNodes []string, images []string, isMetalDisconnected bool) {
	for _, nodeName := range optedNodes {
		for _, img := range images {
			if isMetalDisconnected {
				_, _ = exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-machine-config-operator", "podman", "rmi", emptyImagePin)
				break
			}
			_, _ = exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-machine-config-operator", "podman", "rmi", img)
		}
	}
}

func waitTillImageDownload(oc *exutil.CLI, nodeName, imageName string) {
	o.Eventually(func() bool {
		crictlstatus, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-machine-config-operator", "crictl", "inspecti", imageName)
		if err != nil {
			framework.Logf("Node %s doesn't have Image %s yet", nodeName, imageName)
			return false
		}
		if strings.Contains(crictlstatus, "imageSpec") {
			return true
		}
		return false
	}, 10*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%s' to download image '%s'.", nodeName, imageName)
}

func waitTillImageGC(oc *exutil.CLI, nodeName, imageName string) {
	o.Eventually(func() bool {
		_, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-machine-config-operator", "crictl", "inspecti", imageName)
		if err == nil {
			framework.Logf("Node %s no longer has Image %s", nodeName, imageName)
			return true
		}
		return false
	}, 10*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%s' to garbage collect '%s'.", nodeName, imageName)
}

func waitForReboot(kubeClient *kubernetes.Clientset, nodeName string) {
	o.Eventually(func() bool {
		node, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab Node '%s', error :%s", nodeName, err)
			return false
		}
		if node.Annotations["machineconfiguration.openshift.io/state"] == "Working" {
			framework.Logf("Node '%s' has entered reboot", nodeName)
			return true
		}
		return false
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%s' to start reboot.", nodeName)

	o.Eventually(func() bool {
		node, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab Node '%s', error :%s", nodeName, err)
			return false
		}
		if node.Annotations["machineconfiguration.openshift.io/state"] == "Done" && len(node.Spec.Taints) == 0 {
			framework.Logf("Node '%s' has finished reboot", nodeName)
			return true
		}
		return false
	}, 15*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%s' to finish reboot.", nodeName)
}

func waitTillNodeReadyWithConfig(kubeClient *kubernetes.Clientset, nodeName string) {
	o.Eventually(func() bool {
		node, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab Node '%s', error :%s", nodeName, err)
			return false
		}
		if strings.Contains(node.Annotations["machineconfiguration.openshift.io/currentConfig"], "rendered-worker") && node.Annotations["machineconfiguration.openshift.io/state"] == "Done" {
			framework.Logf("Node '%s' has rendered-worker config", nodeName)
			return true
		}
		return false
	}, 15*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%s' to have rendered-worker config.", nodeName)
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
