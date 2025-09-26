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

var (
	emptyImagePin = "localhost.localdomain/emptyimagepin"

	// these represent the expected rendered config prefixes for worker and custom MCP nodes
	workerConfigPrefix = "rendered-worker"
	customConfigPrefix = "rendered-custom"
)

// These tests are `Disruptive` because they result in disruptive actions in the cluster, including node reboots and degrades and pod creations and deletions.
var _ = g.Describe("[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:PinnedImages][Disruptive]", func() {
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

	// This test is also considered `Slow` because it takes longer than 5 minutes to run.
	g.It("[Slow]All Nodes in a custom Pool should have the PinnedImages even after Garbage Collection [apigroup:machineconfiguration.openshift.io]", func() {
		// Skip this test on single node and two-node platforms since custom MCPs are not supported
		// for clusters with only a master MCP
		skipOnSingleNodeTopology(oc)
		skipOnTwoNodeTopology(oc)

		// Get the MCP, PIS, and KubletConfig fixtures needed for this test
		pisFixture := customGCMCPpinnedImageSetFixture
		mcpFixture := customMCPFixture
		kcFixture := customGcKCFixture

		// Get the PIS from the PIS fixture
		pis, err := getPISFromFixture(pisFixture)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting PIS from fixture `%v`: %v", pisFixture, err))
		pisDiverged := false

		// Create kube clients & MC clientset for test
		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting kube client: %v", err))
		clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting client set: %v", err))

		// Create custom MCP
		defer deleteMCP(oc, "custom")
		err = oc.Run("apply").Args("-f", mcpFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error creating MCP `custom`: %v", pisFixture))

		// Add node to custom MCP & wait for the node to be ready in the MCP
		optedNodes, err := addWorkerNodesToCustomPool(oc, kubeClient, 1, "custom")
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error adding node to `custom` MCP: %v", err))
		defer waitTillNodeReadyWithConfig(kubeClient, optedNodes[0], workerConfigPrefix)
		defer unlabelNode(oc, optedNodes[0])
		framework.Logf("Waiting for `%v` node to be ready in `custom` MCP.", optedNodes[0])
		waitTillNodeReadyWithConfig(kubeClient, optedNodes[0], customConfigPrefix)

		// Handle disconnected metal cluster environment
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

		// Get images defined in PIS
		var pinnedImages []string
		for _, img := range pis.Spec.PinnedImages {
			pinnedImages = append(pinnedImages, string(img.Name))
		}

		// Apply PIS
		defer deletePinnedImages(oc, kubeClient, clientSet, optedNodes, pinnedImages, isMetalDisconnected)
		defer deletePIS(oc, pis.Name)
		err = applyPIS(oc, pisFixture, pis, pisDiverged)
		o.Expect(err).NotTo(o.HaveOccurred(), "Applied PIS")

		// Test the images applied in the PIS exist on the node after garbage collection.
		GCPISTest(oc, kubeClient, clientSet, true, optedNodes[0], kcFixture, gcImage, pis.Name, isMetalDisconnected)
	})

	g.It("All Nodes in a Custom Pool should have the PinnedImages in PIS [apigroup:machineconfiguration.openshift.io]", func() {
		// Skip this test on single node and two-node platforms since custom MCPs are not supported
		// for clusters with only a master MCP
		skipOnSingleNodeTopology(oc)
		skipOnTwoNodeTopology(oc)

		// Get the MCP & PIS fixtures needed for this test
		pisFixture := customMCPpinnedImageSetFixture
		mcpFixture := customMCPFixture

		// Get the PIS from the PIS fixture
		pis, err := getPISFromFixture(pisFixture)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting PIS from fixture `%v`: %v", pisFixture, err))
		pisDiverged := false

		// Create kube clients & MC clientset for test
		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting kube client: %v", err))
		clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting client set: %v", err))

		// Create custom MCP
		defer deleteMCP(oc, "custom")
		err = oc.Run("apply").Args("-f", mcpFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error creating MCP `custom`: %v", pisFixture))

		// Add node to custom MCP & wait for the node to be ready in the MCP
		optedNodes, err := addWorkerNodesToCustomPool(oc, kubeClient, 1, "custom")
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error adding node to `custom` MCP: %v", err))
		defer waitTillNodeReadyWithConfig(kubeClient, optedNodes[0], workerConfigPrefix)
		defer unlabelNode(oc, optedNodes[0])
		framework.Logf("Waiting for `%v` node to be ready in `custom` MCP.", optedNodes[0])
		waitTillNodeReadyWithConfig(kubeClient, optedNodes[0], customConfigPrefix)

		// Handle disconnected metal cluster environment
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

		// Get images defined in PIS
		var pinnedImages []string
		for _, img := range pis.Spec.PinnedImages {
			pinnedImages = append(pinnedImages, string(img.Name))
		}

		// Apply PIS
		defer deletePinnedImages(oc, kubeClient, clientSet, optedNodes, pinnedImages, isMetalDisconnected)
		defer deletePIS(oc, pis.Name)
		err = applyPIS(oc, pisFixture, pis, pisDiverged)
		o.Expect(err).NotTo(o.HaveOccurred(), "Applied PIS")

		// Test the PIS apply & corresponding MCN updates to ensure the PIS application is successful.
		SimplePISTest(oc, kubeClient, clientSet, true, pis.Name, isMetalDisconnected)
	})

	g.It("All Nodes in a standard Pool should have the PinnedImages PIS [apigroup:machineconfiguration.openshift.io]", func() {
		// Create kube client and client set for test
		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting kube client: %v", err))
		clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting client set: %v", err))

		// Get the PIS for this test. For clusters with only a master MCP with nodes, use a PIS
		// targeting `master`, otherwise use a PIS targeting `worker`.
		pisFixture := pinnedImageSetFixture
		mcpsToTest := GetRolesToTest(oc, clientSet)
		mcpToTest := "worker"
		if len(mcpsToTest) == 1 && mcpsToTest[0] == "master" {
			pisFixture = masterPinnedImageSetFixture
		}
		pis, err := getPISFromFixture(pisFixture)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting PIS from fixture `%v`: %v", pisFixture, err))
		pisDiverged := false

		// Get the nodes targeted by the PIS
		var optedNodes []string
		nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{fmt.Sprintf("node-role.kubernetes.io/%s", mcpToTest): ""}).String()})
		o.Expect(err).NotTo(o.HaveOccurred(), "Getting nodes from standard pool")
		for _, node := range nodes.Items {
			optedNodes = append(optedNodes, node.Name)
		}

		// Handle disconnected metal cluster environment
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

		// Get images defined in PIS
		var pinnedImages []string
		for _, img := range pis.Spec.PinnedImages {
			pinnedImages = append(pinnedImages, string(img.Name))
		}

		// Apply PIS
		defer deletePinnedImages(oc, kubeClient, clientSet, optedNodes, pinnedImages, isMetalDisconnected)
		defer deletePIS(oc, pis.Name)
		err = applyPIS(oc, pisFixture, pis, pisDiverged)
		o.Expect(err).NotTo(o.HaveOccurred(), "Applied PIS")

		// Test the PIS apply & corresponding MCN updates to ensure the PIS application is successful.
		SimplePISTest(oc, kubeClient, clientSet, true, pis.Name, isMetalDisconnected)
	})

	g.It("Invalid PIS leads to degraded MCN in a standard Pool [apigroup:machineconfiguration.openshift.io]", func() {
		// Create kube client and client set for test
		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting kube client: %v", err))
		clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting client set: %v", err))

		// Get the PIS for this test. For clusters with only a master MCP with nodes, use a PIS
		// targeting `master`, otherwise use a PIS targeting `worker`.
		pisFixture := invalidPinnedImageSetFixture
		mcpsToTest := GetRolesToTest(oc, clientSet)
		if len(mcpsToTest) == 1 && mcpsToTest[0] == "master" {
			pisFixture = masterInvalidPinnedImageSetFixture
		}
		pis, err := getPISFromFixture(pisFixture)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting PIS from fixture `%v`: %v", pisFixture, err))
		framework.Logf("Using PIS `%v` for this test.", pis.Name)

		// Apply PIS
		defer deletePIS(oc, pis.Name)
		err = oc.Run("apply").Args("-f", pisFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error applying PIS `%v`: %v", pisFixture, err))

		// Test the PIS apply & corresponding MCN updates to ensure the PIS application is not
		// successful & the PIS Degraded condition in the MCN is populated as `True`.
		SimplePISTest(oc, kubeClient, clientSet, false, pis.Name, false)
	})

	g.It("Invalid PIS leads to degraded MCN in a custom Pool [apigroup:machineconfiguration.openshift.io]", func() {
		// Skip this test on single node and two-node platforms since custom MCPs are not supported
		// for clusters with only a master MCP
		skipOnSingleNodeTopology(oc)
		skipOnTwoNodeTopology(oc)

		// Get the MCP & PIS fixtures needed for this test
		pisFixture := customInvalidPinnedImageSetFixture
		mcpFixture := customMCPFixture

		// Get the PIS from the PIS fixture
		pis, err := getPISFromFixture(pisFixture)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting PIS from fixture `%v`: %v", pisFixture, err))

		// Create kube client and client set for test
		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting kube client: %v", err))
		clientSet, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting client set: %v", err))

		// Create custom MCP
		defer deleteMCP(oc, "custom")
		err = oc.Run("apply").Args("-f", mcpFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error creating MCP `custom`: %v", pisFixture))

		// Add node to custom MCP & wait for the node to be ready in the MCP
		optedNodes, err := addWorkerNodesToCustomPool(oc, kubeClient, 1, "custom")
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error adding node to `custom` MCP: %v", err))
		defer waitTillNodeReadyWithConfig(kubeClient, optedNodes[0], workerConfigPrefix)
		defer unlabelNode(oc, optedNodes[0])
		framework.Logf("Waiting for `%v` node to be ready in `custom` MCP.", optedNodes[0])
		waitTillNodeReadyWithConfig(kubeClient, optedNodes[0], customConfigPrefix)

		// Apply PIS
		defer deletePIS(oc, pis.Name)
		err = oc.Run("apply").Args("-f", pisFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error applying PIS `%v`: %v", pisFixture, err))

		// Test the PIS apply & corresponding MCN updates to ensure the PIS application is not
		// successful & the PIS Degraded condition in the MCN is populated as `True`.
		SimplePISTest(oc, kubeClient, clientSet, false, pis.Name, false)
	})
})

// `applyPIS` runs the oc command necessary for applying a provided PIS.
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

// `addWorkerNodesToCustomPool` labels the desired number of worker nodes with the MCP role
// selector so that the nodes become part of the desired custom MCP
func addWorkerNodesToCustomPool(oc *exutil.CLI, kubeClient *kubernetes.Clientset, numberOfNodes int, customMCP string) ([]string, error) {
	// Get the worker nodes
	nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/worker": ""}).String()})
	if err != nil {
		return nil, err
	}
	// Return an error if there are less worker nodes in the cluster than the desired number of nodes to add to the custom MCP
	if len(nodes.Items) < numberOfNodes {
		return nil, fmt.Errorf("Node in Worker MCP %d < Number of nodes needed in %d MCP", len(nodes.Items), numberOfNodes)
	}

	// Label the nodes with the custom MCP role selector
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

// `GCPISTest` completes the body of a PIS test including the garbage collection step
func GCPISTest(oc *exutil.CLI, kubeClient *kubernetes.Clientset, clientSet *mcClient.Clientset, success bool, nodeName, customGcKCFixture, gcImage, pisName string, isMetalDisconnected bool) {
	// Apply KC to Pool
	defer deleteKC(oc, "custom-gc-config")
	err := oc.Run("apply").Args("-f", customGcKCFixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error applying garbage collection kubelet config: %s", err))

	// Wait for the node to reboot & for the garbage collection to complete
	waitForReboot(kubeClient, nodeName)
	waitTillImageGC(oc, nodeName, gcImage)

	// Set overall timeout for PIS application validaiton
	waitTime := time.Minute * 20
	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
	defer cancel()

	// Check that the PIS conditions are still met
	err = waitForPISStatusX(ctx, oc, kubeClient, clientSet, pisName, success, isMetalDisconnected)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error checking status of PIS `%s`: %v", pisName, err))
}

// `GCPISTest` completes the body of a PIS test, validating that the conditions are met for cases
// where the PIS was expected to be successful and cases where the PIS applied was invalid
func SimplePISTest(oc *exutil.CLI, kubeClient *kubernetes.Clientset, clientSet *mcClient.Clientset, success bool, pisName string, isMetalDisconnected bool) {
	// Set overall timeout for PIS application validaiton
	waitTime := time.Minute * 20
	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
	defer cancel()

	// Check that the PIS conditions are met, depending on the expected validity of the PIS
	err := waitForPISStatusX(ctx, oc, kubeClient, clientSet, pisName, success, isMetalDisconnected)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error checking status of PIS `%s`: %v", pisName, err))
}

// `detectXCondition` checks if a valid PIS has been successfully applied. For disconnected metal
// environments, this checks that the `localhost.localdomain/emptyimagepin` image spec is present
// on the node. For all other cluster types, this checks that the MCN conditions are populating as
// expected and inspects the node to confirm each desired image is pinned.
func detectXCondition(oc *exutil.CLI, node corev1.Node, mcn *mcfgv1.MachineConfigNode, appliedPIS *mcfgv1.PinnedImageSet, isMetalDisconnected bool) (bool, bool, error) {
	// Handle disconnected metal clusters
	if isMetalDisconnected {
		crictlStatus, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, node.Name, "openshift-machine-config-operator", "crictl", "inspecti", emptyImagePin)
		if err != nil {
			return false, false, fmt.Errorf("failed to execute `crictl inspecti %s` on node %s: %w", emptyImagePin, node.Name, err)
		}
		if !strings.Contains(crictlStatus, "imageSpec") {
			return false, false, fmt.Errorf("Image %s not present on node %s: %w", emptyImagePin, node.Name, err)
		}
		return true, false, nil
	}

	// Loop through the MCN conditions, ensuring the "PinnedImageSetsDegraded" condition is not
	// `True`` and that the "PinnedImageSetsProgressing" is `True`.
	for _, cond := range mcn.Status.Conditions {
		if mcfgv1.StateProgress(cond.Type) == mcfgv1.MachineConfigNodePinnedImageSetsDegraded && cond.Status == "True" {
			return false, true, fmt.Errorf("PIS degraded for MCN %s with reason: %s and message: %s", mcn.Name, cond.Reason, cond.Message)
		}

		if mcfgv1.StateProgress(cond.Type) == mcfgv1.MachineConfigNodePinnedImageSetsProgressing && cond.Status == "True" {
			return false, false, nil
		}
	}

	// Loop though the images defined in the PIS & ensure they are pinned on the associated nodes
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
}

// `waitForPISStatusX` defines the retry function to use to validate if the PIS application has
// completed as intended. The steps include:
//  1. Wait for the PIS object to create
//  2. Get the MCP targetted by the PIS
//  3. Get the nodes part of the MCP from step 2
//  4. Loop through the nodes to see if the desired conditions are met
//     - If the PIS is expected to be invalid, it checks that the degrade condition in the
//     corresponding MCN becomes "true"
//     - If the PIS is expected to be valid, it checks that the desired images are pinned on the
//     corresponding nodes and that the MCN conditions properly report the success
func waitForPISStatusX(ctx context.Context, oc *exutil.CLI, kubeClient *kubernetes.Clientset, clientSet *mcClient.Clientset, pisName string, success bool, isMetalDisconnected bool) error {
	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (done bool, err error) {
		// Wait for PIS object to get created
		appliedPIS, err := clientSet.MachineconfigurationV1().PinnedImageSets().Get(context.TODO(), pisName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Retrying PIS Status with non-fatal error: PIS Object not created yet: %s", err)
			return false, nil
		}

		// Get the MCP targeted by the PIS
		pool, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(ctx, appliedPIS.Labels["machineconfiguration.openshift.io/role"], metav1.GetOptions{})
		if err != nil {
			framework.Logf("Retrying PIS Status with non-fatal error: failed to get MCP mentioned in PIS: %s", err)
			return false, nil
		}

		// Get the nodes from the MCP
		nodes, err := getNodesForPool(ctx, oc, kubeClient, pool)
		if err != nil {
			framework.Logf("Retrying PIS Status with non-fatal error: failed to get Nodes from MCP %q mentioned in PIS: %s", pool.Name, err)
			return false, nil
		}

		// Loop through nodes to see if the conditions required to consider the pis apply "done" are met
		doneNodes := 0
		for _, node := range nodes.Items {
			if !success { // handle case when we are expecting the PIS application to fail, so the PIS degraded condition should become true
				framework.Logf("Waiting for PinnedImageSetsDegraded=True")
				conditionMet, err := WaitForMCNConditionStatus(clientSet, node.Name, mcfgv1.MachineConfigNodePinnedImageSetsDegraded, metav1.ConditionTrue, 2*time.Minute, 5*time.Second)
				o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for PinnedImageSetsDegraded=True: %v", err))
				o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect PinnedImageSetsDegraded=True.")
			} else { // handle cases where we are expecting the PIS application to succeed
				mcn, err := clientSet.MachineconfigurationV1().MachineConfigNodes().Get(ctx, node.Name, metav1.GetOptions{})
				if err != nil {
					return false, fmt.Errorf("failed to get mcn: %w", err)
				}
				toContinue, isFatal, err := detectXCondition(oc, node, mcn, appliedPIS, isMetalDisconnected)
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
			}
			// If we make it here, it means no fatal or non-fatal error has occured & the PIS application conditions were met
			doneNodes += 1
		}
		if doneNodes == len(nodes.Items) {
			return true, nil
		}

		return false, nil
	})
}

// `deletePinnedImages` loops through each node targeted by a PIS and removes each image defined in
// the provided array of images, which represents the images defined in the previously applied PIS.
func deletePinnedImages(oc *exutil.CLI, kubeClient *kubernetes.Clientset, clientSet *mcClient.Clientset, optedNodes []string, images []string, isMetalDisconnected bool) {
	for _, nodeName := range optedNodes {
		for _, img := range images {
			// Handle disconnected metal cluster envs
			if isMetalDisconnected {
				_, _ = exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-machine-config-operator", "podman", "rmi", emptyImagePin)
				break
			}
			_, _ = exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-machine-config-operator", "podman", "rmi", img)
		}
	}
}

// `waitTillImageDownload` waits for up to 10 minutes for the desired image to download onto a node
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

// `waitTillImageGC` waits for up to 10 minutes for the garbage collection of the desired image on the input node
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

// `waitForReboot` waits for up to 5 minutes for the input node to start a reboot and then up to 15
// minutes for the node to complete its reboot.
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

// `waitTillNodeReadyWithConfig` loops for up to 5 minutes to check whether the input node reaches
// the desired rendered config version. The config version is determined by checking if the config
// version prefix matches the stardard format of `rendered-<desired-mcp-name>`.
func waitTillNodeReadyWithConfig(kubeClient *kubernetes.Clientset, nodeName, currentConfigPrefix string) {
	o.Eventually(func() bool {
		node, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab Node '%s', error :%s", nodeName, err)
			return false
		}
		currentConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
		if strings.Contains(currentConfig, currentConfigPrefix) && node.Annotations["machineconfiguration.openshift.io/state"] == "Done" {
			framework.Logf("Node '%s' has current config `%v`", nodeName, currentConfig)
			return true
		}
		framework.Logf("Node '%s' has is not yet ready and has the current config `%v`", nodeName, currentConfig)
		return false
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%s' to have rendered-worker config.", nodeName)
}

// `unlabelNode` removes the `node-role.kubernetes.io/custom` label from the node with the input
// name. This triggers the node's removal from the custom MCP named `custom`.
func unlabelNode(oc *exutil.CLI, name string) error {
	return oc.AsAdmin().Run("label").Args("node", name, "node-role.kubernetes.io/custom-").Execute()
}

// `deleteKC` deletes the KubeletConfig with the input name
func deleteKC(oc *exutil.CLI, name string) error {
	return oc.Run("delete").Args("kubeletconfig", name).Execute()
}

// `deleteMCP` deletes the MachineConfigPool with the input name
func deleteMCP(oc *exutil.CLI, name string) error {
	return oc.Run("delete").Args("mcp", name).Execute()
}

// `deletePIS` deletes the PinnedImageSet with the input name
func deletePIS(oc *exutil.CLI, name string) error {
	return oc.Run("delete").Args("pinnedimageset", name).Execute()
}

// `getPISFromFixture` extracts the PinnedImageSet object as defined in the the provided fixture
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
