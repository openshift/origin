package machine_config

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	osconfigv1 "github.com/openshift/api/config/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	v1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	opv1 "github.com/openshift/api/operator/v1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	mcopclient "github.com/openshift/client-go/operator/clientset/versioned"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

const (
	mcoNamespace                    = "openshift-machine-config-operator"
	mapiNamespace                   = "openshift-machine-api"
	mapiMachinesetQualifiedName     = "machinesets.machine.openshift.io"
	cmName                          = "coreos-bootimages"
	mapiMasterMachineLabelSelector  = "machine.openshift.io/cluster-api-machine-role=master"
	mapiMachineSetArchAnnotationKey = "capacity.cluster-autoscaler.kubernetes.io/labels"
)

// TODO: add error message returns for `.NotTo(o.HaveOccurred())` cases.

// skipUnlessTargetPlatform skips the test if it is running on the target platform
func skipUnlessTargetPlatform(oc *exutil.CLI, platformType osconfigv1.PlatformType) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if infra.Status.PlatformStatus.Type != platformType {
		e2eskipper.Skipf("This test only applies to %s platform", platformType)
	}
}

// skipUnlessFunctionalMachineAPI skips the test if the cluster is using Machine API
func skipUnlessFunctionalMachineAPI(oc *exutil.CLI) {
	machineClient, err := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).ToNot(o.HaveOccurred())
	machines, err := machineClient.MachineV1beta1().Machines(mapiNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: mapiMasterMachineLabelSelector})
	// the machine API can be unavailable resulting in a 404 or an empty list
	if err != nil {
		if !apierrors.IsNotFound(err) {
			o.Expect(err).ToNot(o.HaveOccurred())
		}
		e2eskipper.Skipf("haven't found machines resources on the cluster, this test can be run on a platform that supports functional MachineAPI")
		return
	}
	if len(machines.Items) == 0 {
		e2eskipper.Skipf("got an empty list of machines resources from the cluster, this test can be run on a platform that supports functional MachineAPI")
		return
	}

	// we expect just a single machine to be in the Running state
	for _, machine := range machines.Items {
		phase := ptr.Deref(machine.Status.Phase, "")
		if phase == "Running" {
			return
		}
	}
	e2eskipper.Skipf("haven't found a machine in running state, this test can be run on a platform that supports functional MachineAPI")
}

// skipOnSingleNodeTopology skips the test if the cluster is using single-node topology
func skipOnSingleNodeTopology(oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if infra.Status.ControlPlaneTopology == osconfigv1.SingleReplicaTopologyMode {
		e2eskipper.Skipf("This test does not apply to single-node topologies")
	}
}

// getRandomMachineSet picks a random machineset present on the cluster
func getRandomMachineSet(machineClient *machineclient.Clientset) machinev1beta1.MachineSet {
	machineSets, err := machineClient.MachineV1beta1().MachineSets("openshift-machine-api").List(context.TODO(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	machineSetUnderTest := machineSets.Items[rnd.Intn(len(machineSets.Items))]
	return machineSetUnderTest
}

// verifyMachineSetUpdate verifies that the the boot image values of a MachineSet are reconciled correctly
func verifyMachineSetUpdate(oc *exutil.CLI, machineSet machinev1beta1.MachineSet, updateExpected bool) {

	newProviderSpecPatch, originalProviderSpecPatch, newBootImage, originalBootImage := createFakeUpdatePatch(oc, machineSet)
	err := oc.Run("patch").Args(mapiMachinesetQualifiedName, machineSet.Name, "-p", fmt.Sprintf(`{"spec":{"template":{"spec":{"providerSpec":{"value":%s}}}}}`, newProviderSpecPatch), "-n", mapiNamespace, "--type=merge").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Ensure boot image controller is not progressing
	framework.Logf("Waiting until the boot image controller is not progressing...")
	WaitForBootImageControllerToComplete(oc)

	// Fetch the providerSpec of the machineset under test again
	providerSpecDisks, err := oc.Run("get").Args(mapiMachinesetQualifiedName, machineSet.Name, "-o", "template", "--template=`{{.spec.template.spec.providerSpec.value}}`", "-n", mapiNamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Verify that the machineset has the expected boot image values
	if updateExpected {
		o.Expect(providerSpecDisks).ShouldNot(o.ContainSubstring(newBootImage))
	} else {
		o.Expect(providerSpecDisks).Should(o.ContainSubstring(newBootImage))
		// Restore machineSet to original boot image in this case, as the machineset may be used by other test variants

		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("patch").Args(mapiMachinesetQualifiedName, machineSet.Name, "-p", fmt.Sprintf(`{"spec":{"template":{"spec":{"providerSpec":{"value":%s}}}}}`, originalProviderSpecPatch), "-n", mapiNamespace, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Restored build name in the machineset %s from \"%s\" to \"%s\"", machineSet.Name, originalBootImage, newBootImage)
	}
}

// unmarshalProviderSpec unmarshals the machineset's provider spec into
// a ProviderSpec object. Returns an error if providerSpec field is nil,
// or the unmarshal fails
func unmarshalProviderSpec(ms *machinev1beta1.MachineSet, providerSpec interface{}) error {
	if ms.Spec.Template.Spec.ProviderSpec.Value == nil {
		return fmt.Errorf("providerSpec field was empty")
	}
	if err := yaml.Unmarshal(ms.Spec.Template.Spec.ProviderSpec.Value.Raw, &providerSpec); err != nil {
		return fmt.Errorf("unmarshal into providerSpec failed %w", err)
	}
	return nil
}

// marshalProviderSpec marshals the ProviderSpec object into a MachineSet object.
// Returns an error if ProviderSpec or MachineSet is nil, or if the marshal fails
func marshalProviderSpec(providerSpec interface{}) (string, error) {
	if providerSpec == nil {
		return "", fmt.Errorf("ProviderSpec object was nil")
	}
	rawBytes, err := json.Marshal(providerSpec)
	if err != nil {
		return "", fmt.Errorf("marshalling providerSpec failed: %w", err)
	}
	return string(rawBytes), nil
}

// createFakeUpdatePatch creates an update patch for the MachineSet object based on the platform
func createFakeUpdatePatch(oc *exutil.CLI, machineSet machinev1beta1.MachineSet) (string, string, string, string) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	switch infra.Status.PlatformStatus.Type {
	case osconfigv1.AWSPlatformType:
		return generateAWSProviderSpecPatch(machineSet)
	case osconfigv1.GCPPlatformType:
		return generateGCPProviderSpecPatch(machineSet)
	default:
		exutil.FatalErr(fmt.Errorf("unexpected platform type; should not be here"))
		return "", "", "", ""
	}
}

// generateAWSProviderSpecPatch generates a fake update patch for the AWS MachineSet
func generateAWSProviderSpecPatch(machineSet machinev1beta1.MachineSet) (string, string, string, string) {
	providerSpec := new(machinev1beta1.AWSMachineProviderConfig)
	err := unmarshalProviderSpec(&machineSet, providerSpec)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Modify the boot image to a "fake" value
	originalBootImage := *providerSpec.AMI.ID
	newBootImage := originalBootImage + "-fake-update"
	newProviderSpec := providerSpec.DeepCopy()
	newProviderSpec.AMI.ID = &newBootImage

	newProviderSpecPatch, err := marshalProviderSpec(newProviderSpec)
	o.Expect(err).NotTo(o.HaveOccurred())
	originalProviderSpecPatch, err := marshalProviderSpec(providerSpec)
	o.Expect(err).NotTo(o.HaveOccurred())

	return newProviderSpecPatch, originalProviderSpecPatch, newBootImage, originalBootImage

}

// generateGCPProviderSpecPatch generates a fake update patch for the GCP MachineSet
func generateGCPProviderSpecPatch(machineSet machinev1beta1.MachineSet) (string, string, string, string) {
	providerSpec := new(machinev1beta1.GCPMachineProviderSpec)
	err := unmarshalProviderSpec(&machineSet, providerSpec)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Modify the boot image to a "fake" value
	originalBootImage := providerSpec.Disks[0].Image
	newBootImage := "projects/centos-cloud/global/images/family/centos-stream-9"
	newProviderSpec := providerSpec.DeepCopy()
	for idx := range newProviderSpec.Disks {
		newProviderSpec.Disks[idx].Image = newBootImage
	}
	newProviderSpecPatch, err := marshalProviderSpec(newProviderSpec)
	o.Expect(err).NotTo(o.HaveOccurred())
	originalProviderSpecPatch, err := marshalProviderSpec(providerSpec)
	o.Expect(err).NotTo(o.HaveOccurred())

	return newProviderSpecPatch, originalProviderSpecPatch, newBootImage, originalBootImage
}

// WaitForBootImageControllerToComplete waits until the boot image controller is no longer progressing
func WaitForBootImageControllerToComplete(oc *exutil.CLI) {
	machineConfigurationClient, err := mcopclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	// This has a MustPassRepeatedly(3) to ensure there isn't a false positive by checking the
	// controller status too quickly after applying the fixture/labels.
	o.Eventually(func() bool {
		mcop, err := machineConfigurationClient.OperatorV1().MachineConfigurations().Get(context.TODO(), "cluster", metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab machineconfiguration object, error :%v", err)
			return false
		}
		return IsMachineConfigurationConditionFalse(mcop.Status.Conditions, opv1.MachineConfigurationBootImageUpdateProgressing)
	}, 3*time.Minute, 5*time.Second).MustPassRepeatedly(3).Should(o.BeTrue())
}

// IsMachineConfigPoolConditionTrue returns true when the conditionType is present and set to `ConditionTrue`
func IsMachineConfigPoolConditionTrue(conditions []mcfgv1.MachineConfigPoolCondition, conditionType mcfgv1.MachineConfigPoolConditionType) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// IsMachineConfigurationConditionFalse returns false when the conditionType is present and set to `ConditionFalse`
func IsMachineConfigurationConditionFalse(conditions []metav1.Condition, conditionType string) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == metav1.ConditionFalse
		}
	}
	return false
}

// IsClusterOperatorConditionTrue returns true when the conditionType is present and set to `configv1.ConditionTrue`
func IsClusterOperatorConditionTrue(conditions []osconfigv1.ClusterOperatorStatusCondition, conditionType osconfigv1.ClusterStatusConditionType) bool {
	return IsClusterOperatorConditionPresentAndEqual(conditions, conditionType, osconfigv1.ConditionTrue)
}

// IsClusterOperatorConditionFalse returns true when the conditionType is present and set to `configv1.ConditionFalse`
func IsClusterOperatorConditionFalse(conditions []osconfigv1.ClusterOperatorStatusCondition, conditionType osconfigv1.ClusterStatusConditionType) bool {
	return IsClusterOperatorConditionPresentAndEqual(conditions, conditionType, osconfigv1.ConditionFalse)
}

// IsClusterOperatorConditionPresentAndEqual returns true when conditionType is present and equal to status.
func IsClusterOperatorConditionPresentAndEqual(conditions []osconfigv1.ClusterOperatorStatusCondition, conditionType osconfigv1.ClusterStatusConditionType, status osconfigv1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}

// FindClusterOperatorStatusCondition finds the conditionType in conditions.
func FindClusterOperatorStatusCondition(conditions []osconfigv1.ClusterOperatorStatusCondition, conditionType osconfigv1.ClusterStatusConditionType) *osconfigv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// WaitForOneMasterNodeToBeReady waits until atleast one master node has completed an update
func WaitForOneMasterNodeToBeReady(oc *exutil.CLI) error {
	machineConfigClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Eventually(func() bool {
		mcp, err := machineConfigClient.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), "master", metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab machineconfigpools, error :%v", err)
			return false
		}
		// Check if the pool has atleast one updated node(mid-upgrade), or if the pool has completed the upgrade to the new config(the additional check for spec==status here is
		// to ensure we are not checking an older "Updated" condition and the MCP fields haven't caught up yet
		if (IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdating) && mcp.Status.UpdatedMachineCount > 0) ||
			(IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdated) && (mcp.Spec.Configuration.Name == mcp.Status.Configuration.Name)) {
			return true
		}
		framework.Logf("Waiting for atleast one ready control-plane node")
		return false
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue())
	return nil
}

// Gets a random node from a given pool. Checks for whether the node is ready
// and if no nodes are ready, it will poll for up to 5 minutes for a node to
// become available.
func GetRandomNode(oc *exutil.CLI, pool string) corev1.Node {
	if node := getRandomNode(oc, pool); isNodeReady(node) {
		return node
	}

	waitPeriod := time.Minute * 5
	framework.Logf("No ready nodes found for pool %s, waiting up to %s for a ready node to become available", pool, waitPeriod)

	var targetNode corev1.Node

	o.Eventually(func() bool {
		if node := getRandomNode(oc, pool); isNodeReady(node) {
			targetNode = node
			return true
		}

		return false
	}, 5*time.Minute, 2*time.Second).Should(o.BeTrue())

	return targetNode
}

// Gets a random node from a given pool.
func getRandomNode(oc *exutil.CLI, pool string) corev1.Node {
	nodes, err := GetNodesByRole(oc, pool)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(nodes).ShouldNot(o.BeEmpty())

	// Disable gosec here to avoid throwing
	// G404: Use of weak random number generator (math/rand instead of crypto/rand)
	// #nosec
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	return nodes[rnd.Intn(len(nodes))]
}

// GetNodesByRole gets all nodes labeled with role role
func GetNodesByRole(oc *exutil.CLI, role string) ([]corev1.Node, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{fmt.Sprintf("node-role.kubernetes.io/%s", role): ""}).String(),
	}
	nodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.TODO(), listOptions)
	o.Expect(err).NotTo(o.HaveOccurred())
	return nodes.Items, nil
}

// Determines if a given node is ready.
func isNodeReady(node corev1.Node) bool {
	// If the node is cordoned, it is not ready.
	if node.Spec.Unschedulable {
		return false
	}

	// If the nodes' kubelet is not ready, it is not ready.
	if !isNodeKubeletReady(node) {
		return false
	}

	// If the nodes' MCD is not done, it is not ready.
	if !isMCDDone(node) {
		return false
	}

	return true
}

// Determines if a given node's kubelet is ready.
func isNodeKubeletReady(node corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Reason == "KubeletReady" && condition.Status == "True" && condition.Type == "Ready" {
			return true
		}
	}

	return false
}

// Determines whether the MCD on a given node has reached the "Done" state.
func isMCDDone(node corev1.Node) bool {
	// TODO: Update to make use of generalized
	state := node.Annotations["machineconfiguration.openshift.io/state"]
	return state == "Done"
}

// `WaitForMCPToBeReady` waits for a pool to be in an updated state with a specified number of ready machines
func WaitForMCPToBeReady(oc *exutil.CLI, machineConfigClient *machineconfigclient.Clientset, poolName string, readyMachineCount int32) error {
	o.Eventually(func() bool {
		mcp, err := machineConfigClient.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), poolName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab machineconfigpool %v, error :%v", poolName, err)
			return false
		}
		// Check if the pool is in an updated state with the correct number of ready machines
		if IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdated) && mcp.Status.UpdatedMachineCount == readyMachineCount {
			return true
		}
		framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", poolName, readyMachineCount)
		return false
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue())
	return nil
}

// GetCordonedNodes get cordoned nodes (if maxUnavailable > 1 ) otherwise return the 1st cordoned node
func GetCordonedNodes(oc *exutil.CLI, mcpName string) []corev1.Node {
	// Wait for the MCP to start updating
	o.Expect(waitForMCPConditionStatus(oc, mcpName, "Updating", "True")).NotTo(o.HaveOccurred(), "Waiting for 'Updating' status change failed.")

	// Get updating node
	var allUpdatingNodes []corev1.Node
	o.Eventually(func() bool {
		nodes, nodeErr := GetNodesByRole(oc, mcpName)
		o.Expect(nodeErr).NotTo(o.HaveOccurred(), "Error getting nodes from %v MCP.", mcpName)
		o.Expect(nodes).ShouldNot(o.BeEmpty(), "No nodes found for %v MCP.", mcpName)

		for _, node := range nodes {
			unschedulable := node.Spec.Unschedulable
			if unschedulable {
				allUpdatingNodes = append(allUpdatingNodes, node)
			}
		}

		return len(allUpdatingNodes) > 0
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue())

	return allUpdatingNodes
}

// `waitForMCPConditionStatus` waits until the desired MCP condition matches the desired status (ex. wait until "Updating" is "True")
func waitForMCPConditionStatus(oc *exutil.CLI, mcpName string, conditionType mcfgv1.MachineConfigPoolConditionType, status corev1.ConditionStatus) error {
	framework.Logf("Waiting for MCP %s condition %s to be %s.", mcpName, conditionType, status)

	machineConfigClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Eventually(func() bool {
		// Get MCP
		mcp, mcpErr := machineConfigClient.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), mcpName, metav1.GetOptions{})
		if mcpErr != nil {
			framework.Logf("Failed to grab MCP %v, error :%v", mcpName, err)
			return false
		}

		// Loop through conditions to get check for desired condition type/status combonation
		conditions := mcp.Status.Conditions
		for _, condition := range conditions {
			if condition.Type == conditionType {
				framework.Logf("MCP %s condition %s status is %s", mcp.Name, conditionType, condition.Status)
				return condition.Status == status
			}
		}

		framework.Logf("Waiting for %v MCP's %v condition to be %v.", mcp.Name, conditionType, status)
		return false
	}, 5*time.Minute, 3*time.Second).Should(o.BeTrue())
	return nil
}

// `waitForMCNConditionStatus` waits until the desired MCN condition matches the desired status (ex. wait until "Updated" is "False")
func waitForMCNConditionStatus(clientSet *machineconfigclient.Clientset, mcnName string, conditionType string, status metav1.ConditionStatus, timeout time.Duration, interval time.Duration) error {
	o.Eventually(func() bool {
		// Get MCN & desried condition status
		workerNodeMCN, workerErr := clientSet.MachineconfigurationV1alpha1().MachineConfigNodes().Get(context.TODO(), mcnName, metav1.GetOptions{})
		o.Expect(workerErr).NotTo(o.HaveOccurred())
		conditionStatus := getMCNConditionStatus(workerNodeMCN, conditionType)
		if conditionStatus == status {
			return true
		}

		framework.Logf("Waiting for MCN %v %v condition to be %v.", mcnName, conditionType, status)
		return false
	}, timeout, interval).Should(o.BeTrue())
	return nil
}

// `getMCNConditionStatus` returns the status of the desired condition type for MCN, or an empty string if the condition does not exist
func getMCNConditionStatus(mcn *v1alpha1.MachineConfigNode, conditionType string) metav1.ConditionStatus {
	// Loop through conditions and return the status of the desired condition type
	conditions := mcn.Status.Conditions
	for _, condition := range conditions {
		if condition.Type == conditionType {
			framework.Logf("MCN %s condition %s status is %s", mcn.Name, conditionType, condition.Status)
			return condition.Status
		}
	}
	return ""
}

// `confirmUpdatedMCNStatus` confirms that an MCN is in a fully updated state, which requires:
//  1. "Updated" = True
//  2. All other conditions = False
func confirmUpdatedMCNStatus(clientSet *machineconfigclient.Clientset, mcnName string) bool {
	// Get MCN
	workerNodeMCN, workerErr := clientSet.MachineconfigurationV1alpha1().MachineConfigNodes().Get(context.TODO(), mcnName, metav1.GetOptions{})
	o.Expect(workerErr).NotTo(o.HaveOccurred())

	// Loop through conditions and return the status of the desired condition type
	conditions := workerNodeMCN.Status.Conditions
	for _, condition := range conditions {
		if condition.Type == "Updated" && condition.Status != "True" {
			framework.Logf("Node %s update is not complete; 'Updated' condition status is %v", mcnName, condition.Status)
			return false
		} else if condition.Type != "Updated" && condition.Status != "False" {
			framework.Logf("Node %s is updated but MCN is invalid; '%v' codition status is %v", mcnName, condition.Type, condition.Status)
			return false
		}
	}

	framework.Logf("Node %s update is complete and MCN is valid.", mcnName)
	return true
}
