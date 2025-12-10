package machine_config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	osconfigv1 "github.com/openshift/api/config/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
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
	currentConfigAnnotationKey      = "machineconfiguration.openshift.io/currentConfig"
	desiredConfigAnnotationKey      = "machineconfiguration.openshift.io/desiredConfig"
	stateAnnotationKey              = "machineconfiguration.openshift.io/state"
)

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

// `IsSingleNode` returns true if the cluster is using single-node topology and false otherwise
func IsSingleNode(oc *exutil.CLI) bool {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Error determining cluster infrastructure.")
	return infra.Status.ControlPlaneTopology == osconfigv1.SingleReplicaTopologyMode
}

// `skipOnTwoNodeTopology` skips the test if the cluster is using two-node topology, including
// both standard and arbiter cases.
func skipOnTwoNodeTopology(oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if infra.Status.ControlPlaneTopology == osconfigv1.DualReplicaTopologyMode || infra.Status.ControlPlaneTopology == osconfigv1.HighlyAvailableArbiterMode {
		e2eskipper.Skipf("This test does not apply to two-node topologies")
	}
}

// `IsTwoNode` returns true if the cluster is using two-node topology and false otherwise
func IsTwoNode(oc *exutil.CLI) bool {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Error determining cluster infrastructure.")
	return infra.Status.ControlPlaneTopology == osconfigv1.DualReplicaTopologyMode
}

// `IsTwoNodeArbiter` returns true if the cluster is using two-node arbiter topology and false otherwise
func IsTwoNodeArbiter(oc *exutil.CLI) bool {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Error determining cluster infrastructure.")
	return infra.Status.ControlPlaneTopology == osconfigv1.HighlyAvailableArbiterMode &&
		infra.Status.InfrastructureTopology == osconfigv1.HighlyAvailableTopologyMode
}

// `IsDisconnected` returns true if the cluster is a Disconnected cluster and false otherwise
func IsDisconnected(oc *exutil.CLI, nodeName string) bool {
	networkStatus, _ := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-machine-config-operator", "sh", "-c", "curl -s --connect-timeout 5 http://fedoraproject.org/static/hotspot.txt &>/dev/null && echo \"Connected\" || echo \"Disconnected\"")
	if networkStatus == "Connected" {
		return false
	}
	return true
}

// `IsMetal` returns true if the cluster is hosted on a Metal Platform and false otherwise
func IsMetal(oc *exutil.CLI) bool {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return infra.Status.Platform == osconfigv1.BareMetalPlatformType
}

// skipOnMetal skips the test if the cluster is using Metal PLatform
func skipOnMetal(oc *exutil.CLI) {
	if IsMetal(oc) {
		e2eskipper.Skipf("This test does not apply to metal")
	}
}

// `GetRolesToTest` gets the MCPs in a cluster with nodes associated to it. This allows a more robust way to determine
// the roles to use when selecting nodes and testing their MCP associations in an MCN.
func GetRolesToTest(oc *exutil.CLI, machineConfigClient *machineconfigclient.Clientset) []string {
	// Get MCPs
	mcps, mcpErr := machineConfigClient.MachineconfigurationV1().MachineConfigPools().List(context.TODO(), metav1.ListOptions{})
	o.Expect(mcpErr).NotTo(o.HaveOccurred(), "Error getting MCPs.")

	// For any MCP with machines, add the MCP name as a role to test.
	var rolesToTest []string
	for _, mcp := range mcps.Items {
		if mcp.Status.MachineCount > 0 {
			rolesToTest = append(rolesToTest, mcp.Name)
		}
	}

	return rolesToTest
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
	defer func() {
		// Restore machineSet to original boot image as the machineset may be used by other test variants, regardless of success/fail
		err = oc.Run("patch").Args(mapiMachinesetQualifiedName, machineSet.Name, "-p", fmt.Sprintf(`{"spec":{"template":{"spec":{"providerSpec":{"value":%s}}}}}`, originalProviderSpecPatch), "-n", mapiNamespace, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Restored build name in the machineset %s from \"%s\" to \"%s\"", machineSet.Name, newBootImage, originalBootImage)
	}()
	// Ensure boot image controller is not progressing
	framework.Logf("Waiting until the boot image controller is not progressing...")
	WaitForBootImageControllerToComplete(oc)

	o.Eventually(func() bool {
		// Fetch the providerSpec of the machineset under test again
		providerSpec, err := oc.Run("get").Args(mapiMachinesetQualifiedName, machineSet.Name, "-o", "template", "--template=`{{.spec.template.spec.providerSpec.value}}`", "-n", mapiNamespace).Output()
		if err != nil {
			return false
		}

		// Verify that the machineset has the expected boot image values
		if updateExpected {
			return !strings.Contains(providerSpec, newBootImage)
		} else {
			return strings.Contains(providerSpec, newBootImage)
		}
	}, 10*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out verifying MachineSet '%v'", machineSet.Name)
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
	case osconfigv1.VSpherePlatformType:
		return generateVSphereProviderSpecPatch(machineSet)
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

	// Modify the boot image to an older known AMI value
	// See: https://issues.redhat.com/browse/OCPBUGS-57426
	originalBootImage := *providerSpec.AMI.ID
	newBootImage := "ami-000145e5a91e9ac22"
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

	// Modify the boot image to a older known value.
	// See: https://issues.redhat.com/browse/OCPBUGS-57426
	originalBootImage := providerSpec.Disks[0].Image
	newBootImage := "projects/rhcos-cloud/global/images/rhcos-410-84-202210040010-0-gcp-x86-64"
	newProviderSpec := providerSpec.DeepCopy()
	for idx := range newProviderSpec.Disks {
		if newProviderSpec.Disks[idx].Boot {
			newProviderSpec.Disks[idx].Image = newBootImage
		}
	}
	newProviderSpecPatch, err := marshalProviderSpec(newProviderSpec)
	o.Expect(err).NotTo(o.HaveOccurred())
	originalProviderSpecPatch, err := marshalProviderSpec(providerSpec)
	o.Expect(err).NotTo(o.HaveOccurred())

	return newProviderSpecPatch, originalProviderSpecPatch, newBootImage, originalBootImage
}

// generateVSphereProviderSpecPatch generates a fake update patch for the VSphere MachineSet
func generateVSphereProviderSpecPatch(machineSet machinev1beta1.MachineSet) (string, string, string, string) {
	providerSpec := new(machinev1beta1.VSphereMachineProviderSpec)
	err := unmarshalProviderSpec(&machineSet, providerSpec)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Modify the boot image to a "fake" value
	originalBootImage := providerSpec.Template
	newBootImage := "fake-update"
	newProviderSpec := providerSpec.DeepCopy()
	newProviderSpec.Template = newBootImage
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
	}, 5*time.Minute, 5*time.Second).MustPassRepeatedly(3).Should(o.BeTrue())
}

// WaitForMachineConfigurationStatus waits until the MCO syncs the operator status to the latest spec
func WaitForMachineConfigurationStatusUpdate(oc *exutil.CLI) {
	machineConfigurationClient, err := mcopclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	// This has a MustPassRepeatedly(3) to ensure there isn't a false positive by checking the
	// status too quickly after applying the fixture.
	o.Eventually(func() bool {
		mcop, err := machineConfigurationClient.OperatorV1().MachineConfigurations().Get(context.TODO(), "cluster", metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab machineconfiguration object, error :%v", err)
			return false
		}
		return mcop.Generation == mcop.Status.ObservedGeneration
	}, 3*time.Minute, 1*time.Second).MustPassRepeatedly(3).Should(o.BeTrue())
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

// Applies a boot image fixture and waits for the MCO to reconcile the status
func ApplyMachineConfigurationFixture(oc *exutil.CLI, fixture string) {
	err := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Ensure status accounts for the fixture that was applied
	WaitForMachineConfigurationStatusUpdate(oc)
}

// `ValidateMCNForNodeInPool` validates the MCN of a node in a given pool. It does the following:
//  1. Get node from desired pool
//  2. Get the MCN for the node
//  3. Validate the MCN against the node properties
//     - Check that `mcn.Spec.Pool.Name` matches provided `poolName`
//     - Check that `mcn.Name` matches the node name
//     - Check that `mcn.Spec.ConfigVersion.Desired` matches the node desired config version
//     - Check that `nmcn.Status.ConfigVersion.Current` matches the node current config version
//     - Check that `mcn.Status.ConfigVersion.Desired` matches the node desired config version
func ValidateMCNForNodeInPool(oc *exutil.CLI, clientSet *machineconfigclient.Clientset, node corev1.Node, poolName string) error {
	// Get node's desired and current config versions
	nodeCurrentConfig := node.Annotations[currentConfigAnnotationKey]
	nodeDesiredConfig := node.Annotations[desiredConfigAnnotationKey]

	// Get node MCN
	framework.Logf("Getting MCN for node '%v'.", node.Name)
	mcn, mcnErr := clientSet.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
	if mcnErr != nil {
		framework.Logf("Could not get MCN for node '%v'.", node.Name)
		return mcnErr
	}

	// Check MCN pool name value for default MCPs
	framework.Logf("Checking MCN pool name for node '%v' matches pool association '%v'.", node.Name, poolName)
	if mcn.Spec.Pool.Name != poolName {
		framework.Logf("MCN pool name '%v' does not match node MCP association '%v'.", mcn.Spec.Pool.Name, poolName)
		return fmt.Errorf("MCN pool name does not match node MCP association")
	}

	// Check MCN name matches node name
	framework.Logf("Checking MCN name matches node name '%v'.", node.Name)
	if mcn.Name != node.Name {
		framework.Logf("MCN name '%v' does not match node name '%v'.", mcn.Name, node.Name)
		return fmt.Errorf("MCN name does not match node name")
	}

	// Check desired config version in MCN spec matches desired config on node
	framework.Logf("Checking node '%v' desired config version '%v' matches desired config version in MCN spec.", node.Name, nodeDesiredConfig)
	if mcn.Spec.ConfigVersion.Desired != nodeDesiredConfig {
		framework.Logf("MCN spec desired config version '%v' does not match node desired config version '%v'.", mcn.Spec.ConfigVersion.Desired, nodeDesiredConfig)
		return fmt.Errorf("MCN spec desired config version does not match node desired config version")
	}

	// Check desired config version in MCN spec matches desired config on node
	framework.Logf("Checking node '%v' current config version '%v' matches current version in MCN status.", node.Name, nodeCurrentConfig)
	if mcn.Status.ConfigVersion.Current != nodeCurrentConfig {
		framework.Logf("MCN status current config version '%v' does not match node current config version '%v'.", mcn.Status.ConfigVersion.Current, nodeCurrentConfig)
		return fmt.Errorf("MCN status current config version does not match node current config version")
	}

	// Check desired config version in MCN spec matches desired config on node
	framework.Logf("Checking node '%v' desired config version '%v' matches desired version in MCN status.", node.Name, nodeDesiredConfig)
	if mcn.Status.ConfigVersion.Desired != nodeDesiredConfig {
		framework.Logf("MCN status desired config version '%v' does not match node desired config version '%v'.", mcn.Status.ConfigVersion.Desired, nodeDesiredConfig)
		return fmt.Errorf("MCN status desired config version does not match node desired config version")
	}

	return nil
}

// `GetRandomNode` gets a random node from with a given role and checks whether the node is ready. If no
// nodes are ready, it will wait for up to 5 minutes for a node to become available.
func GetRandomNode(oc *exutil.CLI, role string) corev1.Node {
	if node := getRandomNode(oc, role); isNodeReady(node) {
		return node
	}

	// If no nodes are ready, wait for up to 5 minutes for one to be ready
	waitPeriod := time.Minute * 5
	framework.Logf("No ready nodes found with role '%s', waiting up to %s for a ready node to become available", role, waitPeriod)
	var targetNode corev1.Node
	o.Eventually(func() bool {
		if node := getRandomNode(oc, role); isNodeReady(node) {
			targetNode = node
			return true
		}

		return false
	}, 5*time.Minute, 2*time.Second).Should(o.BeTrue())

	return targetNode
}

// `getRandomNode` gets a random node with a given role
func getRandomNode(oc *exutil.CLI, role string) corev1.Node {
	nodes, err := GetNodesByRole(oc, role)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(nodes).ShouldNot(o.BeEmpty())

	// Disable gosec here to avoid throwing
	// G404: Use of weak random number generator (math/rand instead of crypto/rand)
	// #nosec
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	return nodes[rnd.Intn(len(nodes))]
}

// `GetNodesByRole` gets all nodes labeled with the desired role
func GetNodesByRole(oc *exutil.CLI, role string) ([]corev1.Node, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{fmt.Sprintf("node-role.kubernetes.io/%s", role): ""}).String(),
	}
	nodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

// `GetAllNodes` gets all nodes from a cluster
func GetAllNodes(oc *exutil.CLI) ([]corev1.Node, error) {
	nodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

// `isNodeReady` determines if a given node is ready
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
	if !checkMCDState(node, "Done") {
		return false
	}

	return true
}

// `isNodeKubeletReady` determines if a given node's kubelet is ready
func isNodeKubeletReady(node corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Reason == "KubeletReady" && condition.Status == "True" && condition.Type == "Ready" {
			return true
		}
	}

	return false
}

// `checkMCDState` determines whether the MCD state matches the provided desired state
func checkMCDState(node corev1.Node, desiredState string) bool {
	state := node.Annotations[stateAnnotationKey]
	return state == desiredState
}

// `WaitForMCPToBeReady` waits up to 5 minutes for a pool to be in an updated state with a specified number of ready machines
func WaitForMCPToBeReady(oc *exutil.CLI, machineConfigClient *machineconfigclient.Clientset, poolName string, readyMachineCount int32) {
	o.Eventually(func() bool {
		mcp, err := machineConfigClient.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), poolName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab MCP '%v', error :%v", poolName, err)
			return false
		}
		// Check if the pool is in an updated state with the correct number of ready machines
		if IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdated) && mcp.Status.UpdatedMachineCount == readyMachineCount {
			framework.Logf("MCP '%v' has the desired %v ready machines.", poolName, mcp.Status.UpdatedMachineCount)
			return true
		}
		// Log details of what is outstanding for the pool to be considered ready
		if mcp.Status.UpdatedMachineCount == readyMachineCount {
			framework.Logf("MCP '%v' has the desired %v ready machines, but is not in an 'Updated' state.", poolName, mcp.Status.UpdatedMachineCount)
		} else {
			framework.Logf("MCP '%v' has %v ready machines. Waiting for the desired ready machine count of %v.", poolName, mcp.Status.UpdatedMachineCount, readyMachineCount)
		}
		return false
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for MCP '%v' to be in 'Updated' state with %v ready machines.", poolName, readyMachineCount)
}

// `CleanupCustomMCP` cleans up a custom MCP through the following steps:
//  1. Remove the custom MCP role label from the node
//  2. Wait for the custom MCP to be updated with no ready machines
//  3. Optionally, if a MC has been provided, delete it; if none has been provided, skip to step 4
//  4. Wait for the node to have a current config version equal to the config version of the worker MCP
//  5. Remove the custom MCP
func CleanupCustomMCP(oc *exutil.CLI, clientSet *machineconfigclient.Clientset, customMCPName string, nodeName string, mcName *string) error {
	// Unlabel node
	framework.Logf("Removing label node-role.kubernetes.io/%v from node %v", customMCPName, nodeName)
	unlabelErr := oc.Run("label").Args(fmt.Sprintf("node/%s", nodeName), fmt.Sprintf("node-role.kubernetes.io/%s-", customMCPName)).Execute()
	if unlabelErr != nil {
		return fmt.Errorf("could not remove label 'node-role.kubernetes.io/%v' from node '%v'; err: %v", customMCPName, nodeName, unlabelErr)
	}

	// Wait for custom MCP to report no ready nodes
	framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", customMCPName, 0)
	WaitForMCPToBeReady(oc, clientSet, customMCPName, 0)

	// Delete the MC, if one was provided
	if mcName != nil {
		deleteMCErr := oc.Run("delete").Args("machineconfig", *mcName).Execute()
		if deleteMCErr != nil {
			return fmt.Errorf("could delete MachineConfig '%v'; err: %v", mcName, deleteMCErr)

		}
	}

	// Wait for node to have a current config version equal to the worker MCP's config version
	workerMcp, workerMcpErr := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), worker, metav1.GetOptions{})
	if workerMcpErr != nil {
		return fmt.Errorf("could not get worker MCP; err: %v", workerMcpErr)
	}
	workerMcpConfig := workerMcp.Spec.Configuration.Name
	framework.Logf("Waiting for %v node to be updated with %v config version.", nodeName, workerMcpConfig)
	WaitForNodeCurrentConfig(oc, nodeName, workerMcpConfig)

	// Delete custom MCP
	framework.Logf("Deleting MCP %v", customMCPName)
	deleteMCPErr := oc.Run("delete").Args("mcp", customMCPName).Execute()
	if deleteMCPErr != nil {
		return fmt.Errorf("error deleting MCP '%v': %v", customMCPName, deleteMCPErr)
	}

	return nil
}

// `WaitForNodeCurrentConfig` waits up to 5 minutes for a input node to have a current config equal to the `config` parameter
func WaitForNodeCurrentConfig(oc *exutil.CLI, nodeName string, config string) {
	o.Eventually(func() bool {
		node, nodeErr := oc.AsAdmin().KubeClient().CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if nodeErr != nil {
			framework.Logf("Failed to get node '%v', error :%v", nodeName, nodeErr)
			return false
		}

		// Check if the node's current config matches the input config version
		nodeCurrentConfig := node.Annotations[currentConfigAnnotationKey]
		if nodeCurrentConfig == config {
			framework.Logf("Node '%v' has successfully updated and has a current config version of '%v'.", nodeName, nodeCurrentConfig)
			return true
		}
		framework.Logf("Node '%v' has a current config version of '%v'. Waiting for the node's current config version to be '%v'.", nodeName, nodeCurrentConfig, config)
		return false
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for node '%v' to have a current config version of '%v'.", nodeName, config)
}

// `GetUpdatingNode` returns the updating node, determined by the node targetting a new desired
// config, when the corresponding MCP starts updating
func GetUpdatingNode(oc *exutil.CLI, mcpName, originalConfigVersion string) corev1.Node {
	// Wait for the MCP to start updating
	o.Expect(WaitForMCPConditionStatus(oc, mcpName, mcfgv1.MachineConfigPoolUpdating, corev1.ConditionTrue, 3*time.Minute, 2*time.Second)).NotTo(o.HaveOccurred(), "Waiting for 'Updating' status change failed.")

	// Get first updating node & return it
	var updatingNode corev1.Node
	o.Eventually(func() bool {
		framework.Logf("Trying to get updating node in '%v' MCP.", mcpName)

		// Get nodes in MCP
		nodes, nodeErr := GetNodesByRole(oc, mcpName)
		o.Expect(nodeErr).NotTo(o.HaveOccurred(), "Error getting nodes from %v MCP.", mcpName)
		o.Expect(nodes).ShouldNot(o.BeEmpty(), "No nodes found for %v MCP.", mcpName)

		// Loop through nodes to see which is targetting a new desired config version
		for _, node := range nodes {
			if node.Annotations[desiredConfigAnnotationKey] != originalConfigVersion {
				updatingNode = node
				return true
			}
		}

		return false
	}, 30*time.Second, 1*time.Second).Should(o.BeTrue())

	return updatingNode
}

// `WaitForMCPConditionStatus` waits up to the desired timeout for the desired MCP condition to match the desired status (ex. wait until "Updating" is "True")
func WaitForMCPConditionStatus(oc *exutil.CLI, mcpName string, conditionType mcfgv1.MachineConfigPoolConditionType, status corev1.ConditionStatus, timeout time.Duration, interval time.Duration) error {
	framework.Logf("Waiting up to %v for MCP '%s' condition '%s' to be '%s'.", timeout, mcpName, conditionType, status)
	machineConfigClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Eventually(func() bool {
		framework.Logf("Waiting for '%v' MCP's '%v' condition to be '%v'.", mcpName, conditionType, status)

		// Get MCP
		mcp, mcpErr := machineConfigClient.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), mcpName, metav1.GetOptions{})
		if mcpErr != nil {
			framework.Logf("Failed to grab MCP '%v', error :%v", mcpName, err)
			return false
		}

		// Loop through conditions to get check for desired condition type/status combination
		conditions := mcp.Status.Conditions
		for _, condition := range conditions {
			if condition.Type == conditionType {
				framework.Logf("MCP '%s' condition '%s' status is '%s'", mcp.Name, conditionType, condition.Status)
				return condition.Status == status
			}
		}

		return false
	}, timeout, interval).Should(o.BeTrue())
	return nil
}

// `WaitForMCNConditionStatus` waits up to a specified timeout for the desired MCN condition to match the desired status (ex. wait until "Updated" is "False")
func WaitForMCNConditionStatus(clientSet *machineconfigclient.Clientset, mcnName string, conditionType mcfgv1.StateProgress, status metav1.ConditionStatus,
	timeout time.Duration, interval time.Duration) (bool, error) {

	conditionMet := false
	var conditionErr error
	var workerNodeMCN *mcfgv1.MachineConfigNode
	if err := wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, func(_ context.Context) (bool, error) {
		framework.Logf("Waiting for MCN '%v' %v condition to be %v.", mcnName, conditionType, status)

		workerNodeMCN, conditionErr = clientSet.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), mcnName, metav1.GetOptions{})
		// Record if an error occurs when getting the MCN resource
		if conditionErr != nil {
			framework.Logf("Error getting MCN for node '%v': %v", mcnName, conditionErr)
			return false, nil
		}

		// Check if the MCN status is as desired
		conditionMet = CheckMCNConditionStatus(workerNodeMCN, conditionType, status)
		return conditionMet, nil
	}); err != nil {
		framework.Logf("The desired MCN condition was never met: %v", err)
		// Handle the situation where there were errors getting the MCN resource
		if conditionErr != nil {
			framework.Logf("An error occured waiting for MCN '%v' %v condition to be %v: %v", mcnName, conditionType, status, conditionErr)
			return conditionMet, fmt.Errorf("MCN '%v' %v condition was not %v: %v", mcnName, conditionType, status, conditionErr)
		}
		// Handle case when no errors occur grabbing the MCN, but we time out waiting for the condition to be in the desired state
		framework.Logf("A timeout occured waiting for MCN '%v' %v condition was not %v.", mcnName, conditionType, status)
		return conditionMet, nil
	}

	return conditionMet, conditionErr
}

// `CheckMCNConditionStatus` checks that an MCN condition matches the desired status (ex. confirm "Updated" is "False")
func CheckMCNConditionStatus(mcn *mcfgv1.MachineConfigNode, conditionType mcfgv1.StateProgress, status metav1.ConditionStatus) bool {
	conditionStatus := getMCNConditionStatus(mcn, conditionType)
	return conditionStatus == status
}

// `GetMCNCondition` returns the queried condition or nil if the condition does not exist
func GetMCNCondition(mcn *mcfgv1.MachineConfigNode, conditionType mcfgv1.StateProgress) *metav1.Condition {
	// Loop through conditions and return the status of the desired condition type
	conditions := mcn.Status.Conditions
	for _, condition := range conditions {
		if condition.Type == string(conditionType) {
			return &condition
		}
	}
	return nil
}

// `getMCNConditionStatus` returns the status of the desired condition type for MCN, or an empty string if the condition does not exist
func getMCNConditionStatus(mcn *mcfgv1.MachineConfigNode, conditionType mcfgv1.StateProgress) metav1.ConditionStatus {
	// Loop through conditions and return the status of the desired condition type
	condition := GetMCNCondition(mcn, conditionType)
	if condition == nil {
		return ""
	}

	framework.Logf("MCN '%s' %s condition status is %s", mcn.Name, conditionType, condition.Status)
	return condition.Status
}

// `ConfirmUpdatedMCNStatus` confirms that an MCN is in a fully updated state, which requires:
//  1. "Updated" = True
//  2. All other conditions = False
func ConfirmUpdatedMCNStatus(clientSet *machineconfigclient.Clientset, mcnName string) bool {
	// Get MCN
	workerNodeMCN, workerErr := clientSet.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), mcnName, metav1.GetOptions{})
	o.Expect(workerErr).NotTo(o.HaveOccurred())

	// Loop through conditions and return the status of the desired condition type
	conditions := workerNodeMCN.Status.Conditions
	for _, condition := range conditions {
		if condition.Type == string(mcfgv1.MachineConfigNodeUpdated) && condition.Status != metav1.ConditionTrue {
			framework.Logf("Node '%s' update is not complete; 'Updated' condition status is '%v'", mcnName, condition.Status)
			return false
		} else if condition.Type != string(mcfgv1.MachineConfigNodeUpdated) && condition.Status != metav1.ConditionFalse {
			framework.Logf("Node '%s' is updated but MCN is invalid; '%v' codition status is '%v'", mcnName, condition.Type, condition.Status)
			return false
		}
	}

	framework.Logf("Node '%s' update is complete and corresponding MCN is valid.", mcnName)
	return true
}

// `GetDegradedNode` gets a degraded node from a specified MCP
func GetDegradedNode(oc *exutil.CLI, mcpName string) (corev1.Node, error) {
	// Get nodes in desired pool
	nodes, nodeErr := GetNodesByRole(oc, mcpName)
	if nodeErr != nil {
		return corev1.Node{}, nodeErr
	} else if len(nodes) == 0 {
		return corev1.Node{}, fmt.Errorf("no nodes found in MCP '%v", mcpName)
	}

	// Get degraded node
	for _, node := range nodes {
		if checkMCDState(node, "Degraded") {
			return node, nil
		}
	}

	return corev1.Node{}, errors.New("no degraded node found")
}

// `RecoverFromDegraded` gets the degraded node in the desired MCP, forces the node to recover by updating its desired
// config to be its current config, and waits for the MCP to return to an Update=True state
func RecoverFromDegraded(oc *exutil.CLI, mcpName string) error {
	framework.Logf("Recovering %s pool from degraded state", mcpName)

	// Get nodes from degraded MCP & update the desired config of the degraded node to force a recovery update
	nodes, nodeErr := GetNodesByRole(oc, mcpName)
	o.Expect(nodeErr).NotTo(o.HaveOccurred())
	o.Expect(nodes).ShouldNot(o.BeEmpty())
	for _, node := range nodes {
		framework.Logf("Restoring desired config for node: %s", node.Name)
		if checkMCDState(node, "Done") {
			framework.Logf("Node %s is updated and does not need to be recovered", node.Name)
		} else {
			err := restoreDesiredConfig(oc, node)
			if err != nil {
				return fmt.Errorf("error restoring desired config in node %s. Error: %s", node.Name, err)
			}
		}
	}

	// Wait for MCP to not be in degraded status
	mcpErr := WaitForMCPConditionStatus(oc, mcpName, "Degraded", "False", 4*time.Minute, 5*time.Second)
	o.Expect(mcpErr).NotTo(o.HaveOccurred(), fmt.Sprintf("could not recover %v MCP from the degraded status.", mcpName))
	mcpErr = WaitForMCPConditionStatus(oc, mcpName, "Updated", "True", 7*time.Minute, 5*time.Second)
	o.Expect(mcpErr).NotTo(o.HaveOccurred(), fmt.Sprintf("%v MCP could not reach an updated state.", mcpName))
	return nil
}

// `restoreDesiredConfig` updates the value of a node's desiredConfig annotation to be equal to the value of its currentConfig (desiredConfig=currentConfig)
func restoreDesiredConfig(oc *exutil.CLI, node corev1.Node) error {
	// Get current config
	currentConfig := node.Annotations[currentConfigAnnotationKey]
	if currentConfig == "" {
		return fmt.Errorf("currentConfig annotation is empty for node %s", node.Name)
	}

	// Update desired config to be equal to current config
	framework.Logf("Node: %s is restoring desiredConfig value to match currentConfig value: %s", node.Name, currentConfig)
	configErr := oc.Run("patch").Args(fmt.Sprintf("node/%v", node.Name), "--patch", fmt.Sprintf(`{"metadata":{"annotations":{"machineconfiguration.openshift.io/desiredConfig":"%v"}}}`, currentConfig), "--type=merge").Execute()
	return configErr
}

// `WorkersCanBeScaled` checks whether the worker nodes in a cluster can be scaled.
// Cases where scaling worker nodes is NOT possible include:
//   - Baremetal platform
//   - MachineAPI is disabled
//   - Error getting list of MachineSets / no MachineSets exist
//   - All MachineSets have 0 worker nodes
func WorkersCanBeScaled(oc *exutil.CLI, machineClient *machineclient.Clientset) (bool, error) {
	framework.Logf("Checking if worker nodes can be scaled using machinesets.")

	// Check if platform is baremetal
	framework.Logf("Checking if cluster platform is baremetal.")
	if checkPlatform(oc) == "baremetal" {
		framework.Logf("Cluster platform is baremetal. Nodes cannot be scaled in baremetal test environments.")
		return false, nil
	}

	// Check if MachineAPI is enabled
	framework.Logf("Checking if MachineAPI is enabled.")
	if !isCapabilityEnabled(oc, "MachineAPI") {
		framework.Logf("MachineAPI capability is not enabled. Nodes cannot be scaled.")
		return false, nil
	}

	// Get MachineSets
	framework.Logf("Getting MachineSets.")
	machineSets, machineSetErr := machineClient.MachineV1beta1().MachineSets("openshift-machine-api").List(context.TODO(), metav1.ListOptions{})
	if machineSetErr != nil {
		framework.Logf("Error getting list of MachineSets.")
		return false, machineSetErr
	} else if len(machineSets.Items) == 0 {
		framework.Logf("No MachineSets configured. Nodes cannot be scaled.")
		return false, nil
	}

	// Check if all MachineSets have 0 replicas
	// Per openshift-tests-private repo:
	// "In some UPI/SNO/Compact clusters machineset resources exist, but they are all configured with 0 replicas
	// If all machinesets have 0 replicas, then it means that we need to skip the test case"
	machineSetsWithReplicas := 0
	for _, machineSet := range machineSets.Items {
		replicas := machineSet.Spec.Replicas
		machineSetsWithReplicas += int(*replicas)
	}
	if machineSetsWithReplicas == 0 {
		framework.Logf("All machinesets have 0 worker nodes. Nodes cannot be scaled.")
		return false, nil
	}

	return true, nil
}

// `checkPlatform` returns the cluster's platform
func checkPlatform(oc *exutil.CLI) string {
	output, err := oc.AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed determining cluster infrastructure.")
	return strings.ToLower(output)
}

// `isCapabilityEnabled` checks whether a desired capability is in the cluster's enabledCapabilities list
func isCapabilityEnabled(oc *exutil.CLI, desiredCapability osconfigv1.ClusterVersionCapability) bool {
	enabledCapabilities := getEnabledCapabilities(oc)
	enabled := false
	for _, enabledCapability := range enabledCapabilities {
		if enabledCapability == desiredCapability {
			enabled = true
			break
		}
	}
	framework.Logf("Capability '%s' is enabled: %v", desiredCapability, enabled)

	return enabled
}

// `getEnabledCapabilities` gets a cluster's enabled capability list
func getEnabledCapabilities(oc *exutil.CLI) []osconfigv1.ClusterVersionCapability {
	clusterversion, err := oc.AsAdmin().AdminConfigClient().ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Error getting clusterverion.")
	enabledCapabilities := clusterversion.Status.Capabilities.EnabledCapabilities

	return enabledCapabilities
}

// `ScaleMachineSet` scales the provided MachineSet by updating the replica to be the provided value
func ScaleMachineSet(oc *exutil.CLI, machineSetName string, replicaValue string) error {
	return oc.Run("scale").Args(fmt.Sprintf("--replicas=%v", replicaValue), "machinesets.machine.openshift.io", machineSetName, "-n", "openshift-machine-api").Execute()
}

// `GetMachinesByPhase` get machine by phase e.g. Running, Provisioning, Provisioned, Deleting etc.
func GetMachinesByPhase(machineClient *machineclient.Clientset, machineSetName string, desiredPhase string) (machinev1beta1.Machine, error) {
	desiredMachine := machinev1beta1.Machine{}
	err := fmt.Errorf("no %v machine found in %v MachineSet", desiredPhase, machineSetName)
	o.Eventually(func() bool {
		framework.Logf("Trying to get machine with phase %v from MachineSet '%v'.", desiredPhase, machineSetName)

		// Get machines in desired MachineSet
		machines, machinesErr := machineClient.MachineV1beta1().Machines(mapiNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("machine.openshift.io/cluster-api-machineset=%v", machineSetName)})
		o.Expect(machinesErr).NotTo(o.HaveOccurred())

		// Find machine in desired phase
		for _, machine := range machines.Items {
			machinePhase := ptr.Deref(machine.Status.Phase, "")
			if machinePhase == desiredPhase {
				desiredMachine = machine
				err = nil
				return true
			}
		}
		return false
	}, 8*time.Minute, 3*time.Second).Should(o.BeTrue())
	return desiredMachine, err
}

// `UpdateDeleteMachineAnnotation` updates the provided MachineSet's `deletePolicy` to be true.
// This will ensure the create machine is the one deleted on cleanup.
func UpdateDeleteMachineAnnotation(oc *exutil.CLI, machineSetName string) error {
	return oc.Run("patch").Args(fmt.Sprintf("machines.machine.openshift.io/%v", machineSetName), "-n", "openshift-machine-api", "--patch", `{"metadata":{"annotations":{"machine.openshift.io/delete-machine":"true"}}}`, "--type=merge").Execute()
}

// `WaitForMachineInState` waits up to 7 minutes for the desired machine to be in the desired state
func WaitForMachineInState(machineClient *machineclient.Clientset, machineName string, desiredPhase string) error {
	o.Eventually(func() bool {
		// Get the desired machine
		machine, machineErr := machineClient.MachineV1beta1().Machines(mapiNamespace).Get(context.TODO(), machineName, metav1.GetOptions{})
		o.Expect(machineErr).NotTo(o.HaveOccurred())

		// Check if machine phase is desired phase
		machinePhase := ptr.Deref(machine.Status.Phase, "")
		framework.Logf("Machine '%v' is in %v phase.", machineName, machinePhase)
		return machinePhase == desiredPhase
	}, 10*time.Minute, 10*time.Second).Should(o.BeTrue())
	return nil
}

// `GetNodeInMachine` gets the node associated with a machine
func GetNodeInMachine(oc *exutil.CLI, machineName string) (corev1.Node, error) {
	// Get name of nodes associated with the desired machine
	nodeNames, nodeNamesErr := oc.Run("get").Args("nodes", "-o", fmt.Sprintf(`jsonpath='{.items[?(@.metadata.annotations.machine\.openshift\.io/machine=="openshift-machine-api/%v")].metadata.name}'`, machineName)).Output()
	if nodeNamesErr != nil { //error getting filtered node names
		return corev1.Node{}, nodeNamesErr
	} else if nodeNames == "" { //error when no nodes are found
		return corev1.Node{}, fmt.Errorf("no node is linked to Machine: %s", machineName)
	}

	// Determine the number of nodes in the Machine
	// Note: the format of `nodeNames` is the names of nodes seperated by a space (ex: "node-name-1 node-name-2"),
	// so the number of nodes is equal to one more than the number of spaces
	numberOfNodeNames := strings.Count(nodeNames, " ") + 1
	if numberOfNodeNames > 1 { //error when a machine has more than one node
		return corev1.Node{}, fmt.Errorf("more than one node is linked to Machine: %s; number of nodes: %d", machineName, numberOfNodeNames)
	}

	node, nodeErr := oc.AsAdmin().KubeClient().CoreV1().Nodes().Get(context.TODO(), strings.ReplaceAll(nodeNames, "'", ""), metav1.GetOptions{})
	if nodeErr != nil { //error getting filtered node names
		return corev1.Node{}, nodeErr
	}

	return *node, nil
}

// `GetNewReadyNodeInMachine` waits up to 4 minutes for the newly provisioned node in a desired machine node to be ready
func GetNewReadyNodeInMachine(oc *exutil.CLI, machineName string) (corev1.Node, error) {
	desiredNode := corev1.Node{}
	err := fmt.Errorf("no ready node in Machine: %s", machineName)
	o.Eventually(func() bool {
		// Get the desired node
		node, nodeErr := GetNodeInMachine(oc, machineName)
		o.Expect(nodeErr).NotTo(o.HaveOccurred())

		// Check if node is in desiredStatus
		framework.Logf("Checking if node '%v' is ready.", node.Name)
		if isNodeReady(node) {
			framework.Logf("Node '%v' is ready.", node.Name)
			desiredNode = node
			err = nil
			return true
		}

		return false
	}, 4*time.Minute, 5*time.Second).Should(o.BeTrue(), fmt.Sprintf("Node in machine %v never became ready.", machineName))
	return desiredNode, err
}

// `WaitForValidMCNProperties` waits for the MCN of a node to be valid. To be valid, the following must be true:
//   - MCN with name equivalent to node name exists (waits up to 20 sec)
//   - Pool name in MCN spec matches node MCP association (waits up to 1 min)
//   - Desired config version of node matches desired config version in MCN spec (waits up to 1 min)
//   - Current config version of node matches current config version in MCN status (waits up to 2 min)
//   - Desired config version of node matches desired config version in MCN status (waits up to 1 min)
func WaitForValidMCNProperties(clientSet *machineconfigclient.Clientset, node corev1.Node) error {
	nodeDesiredConfig := node.Annotations[desiredConfigAnnotationKey]
	nodeCurrentConfig := node.Annotations[currentConfigAnnotationKey]

	// Check MCN exists and that its name and node name match
	framework.Logf("Checking MCN exists and name matches node name '%v'.", node.Name)
	o.Eventually(func() bool {
		// Get the desired MCN
		newMCN, newMCNErr := clientSet.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
		if newMCNErr != nil {
			framework.Logf("Failed getting MCN '%v'.", node.Name)
			return false
		}

		// Check if MCN name matches node's name
		framework.Logf("Node name: %v. MCN name: %v.", node.Name, newMCN.Name)
		return node.Name == newMCN.Name
	}, 20*time.Second, 2*time.Second).Should(o.BeTrue(), fmt.Sprintf("Could not get MCN for node %v", node.Name))

	// Check pool name in MCN matches node MCP association
	// Note: pool name should be default value of `worker`
	framework.Logf("Waiting for node MCP to match pool name in MCN '%v' spec.", node.Name)
	nodeMCP := ""
	var ok bool
	if _, ok = node.Labels["node-role.kubernetes.io/worker"]; ok {
		nodeMCP = "worker"
	} else {
		return fmt.Errorf("node MCP association could be determined for node %v; node is not in default worker pool", node.Name)
	}
	o.Eventually(func() bool {
		// Get the desired MCN
		newMCN, newMCNErr := clientSet.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
		if newMCNErr != nil {
			framework.Logf("Failed getting MCN '%v'.", node.Name)
			return false
		}

		// Check if MCN pool name in spec matches node's MCP association
		framework.Logf("Node MCP association: %v. MCN spec pool name: %v.", nodeMCP, newMCN.Spec.Pool.Name)
		return newMCN.Spec.Pool.Name == nodeMCP
	}, 1*time.Minute, 5*time.Second).Should(o.BeTrue())

	// Check desired config version matches for node and MCN spec config version
	framework.Logf("Waiting for node desired config version to match desired config version in MCN '%v' spec.", node.Name)
	o.Eventually(func() bool {
		// Get the desired MCN
		newMCN, newMCNErr := clientSet.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
		if newMCNErr != nil {
			framework.Logf("Failed getting MCN '%v'.", node.Name)
			return false
		}

		// Check if MCN desired config version in spec matches node's desired config version
		framework.Logf("Node desired config version: %v. MCN spec desired config version: %v.", nodeDesiredConfig, newMCN.Spec.ConfigVersion.Desired)
		return newMCN.Spec.ConfigVersion.Desired == nodeDesiredConfig
	}, 1*time.Minute, 5*time.Second).Should(o.BeTrue())

	// Check current config version matches for node and MCN status config version
	framework.Logf("Waiting for node current config version to match current config version in MCN '%v' status.", node.Name)
	o.Eventually(func() bool {
		// Get the desired MCN
		newMCN, newMCNErr := clientSet.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
		if newMCNErr != nil {
			framework.Logf("Failed getting MCN '%v'.", node.Name)
			return false
		}

		// Check if MCN current config version in status matches node's current config version
		framework.Logf("Node current config version: %v. MCN status current config version: %v.", nodeCurrentConfig, newMCN.Status.ConfigVersion.Current)
		return newMCN.Status.ConfigVersion.Current == nodeCurrentConfig
	}, 2*time.Minute, 5*time.Second).Should(o.BeTrue())

	// Check desired config version matches for node and MCN status config version
	framework.Logf("Waiting for node desired config version to match desired config version in MCN '%v' status.", node.Name)
	o.Eventually(func() bool {
		// Get the desired MCN
		newMCN, newMCNErr := clientSet.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
		if newMCNErr != nil {
			framework.Logf("Failed getting MCN '%v'.", node.Name)
			return false
		}

		// Check if MCN desired config version in status matches node's desired config version
		framework.Logf("Node desired config version: %v. MCN status desired config version: %v.", nodeDesiredConfig, newMCN.Status.ConfigVersion.Desired)
		return newMCN.Status.ConfigVersion.Desired == nodeDesiredConfig
	}, 2*time.Minute, 5*time.Second).Should(o.BeTrue())
	return nil
}

// `ScaleMachineSetDown` will determine whether a MachineSet needs to be scaled and, if so, will
// scale it. A MachineSet needs to be scaled if its desired replica value does not match its
// current replica value.
func ScaleMachineSetDown(oc *exutil.CLI, machineSet machinev1beta1.MachineSet, desiredReplicaValue int, cleanupCompleted bool) error {
	// Skip when cleanup is not needed
	if cleanupCompleted {
		return nil
	}

	// Check if MachineSet needs to be scaled
	if int(*machineSet.Spec.Replicas) == desiredReplicaValue {
		framework.Logf("MachineSet '%v' does not need to be scaled. Current replica value %v matches desired replica value of %v.", machineSet.Name, *machineSet.Spec.Replicas, desiredReplicaValue)
		return nil
	}

	// Scale MachineSet to desired replica value
	framework.Logf("Scaling MachineSet '%s' to replica value %v.", machineSet.Name, desiredReplicaValue)
	return ScaleMachineSet(oc, machineSet.Name, fmt.Sprintf("%d", desiredReplicaValue))
}

// `CleanupProvisionedMachine` scales down the replica count for a given MachineSet and checks whether the
// provisioned Machine provided is deleted.
func CleanupProvisionedMachine(oc *exutil.CLI, machineClient *machineclient.Clientset, machineSetName string, desiredReplicaValue int,
	machineName string, cleanupCompleted bool) error {
	// Skip when cleanup is not needed
	if cleanupCompleted {
		return nil
	}

	// Scale MachineSet to desired replica value
	framework.Logf("Scaling MachineSet '%s' to replica value %v.", machineSetName, desiredReplicaValue)
	scaleErr := ScaleMachineSet(oc, machineSetName, fmt.Sprintf("%d", desiredReplicaValue))
	if scaleErr != nil {
		return scaleErr
	}

	// Check that provisioned machine is deleted
	return WaitForMachineToBeDeleted(machineClient, machineName)
}

// `CleanupCreatedNode` scales down the replica count for a given MachineSet and checks whether the
// created Node provided is deleted.
func CleanupCreatedNode(oc *exutil.CLI, machineSetName string, desiredReplicaValue int, nodeName string, cleanupCompleted bool) error {
	// Skip when cleanup is not needed
	if cleanupCompleted {
		return nil
	}

	// Scale MachineSet to desired replica value
	framework.Logf("Scaling MachineSet '%s' to replica value %v.", machineSetName, desiredReplicaValue)
	scaleErr := ScaleMachineSet(oc, machineSetName, fmt.Sprintf("%d", desiredReplicaValue))
	if scaleErr != nil {
		return scaleErr
	}

	// Check that created node is deleted
	return WaitForNodeToBeDeleted(oc, nodeName)
}

// `WaitForNodeToBeDeleted` waits up to 10 minutes for a node to be deleted (no longer exist)
func WaitForNodeToBeDeleted(oc *exutil.CLI, nodeName string) error {
	o.Eventually(func() bool {
		framework.Logf("Checking if node '%v' is deleted.", nodeName)

		// Check if node still exists
		_, nodeErr := oc.AsAdmin().KubeClient().CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(nodeErr) {
			framework.Logf("Node '%v' has been deleted.", nodeName)
			return true
		}
		if nodeErr != nil {
			framework.Logf("Error trying to get node: %v.", nodeErr)
			return false
		}

		framework.Logf("Node '%v' still exists.", nodeName)
		return false
	}, 10*time.Minute, 5*time.Second).Should(o.BeTrue())
	return nil
}

// `WaitForMCNToBeDeleted` up waits to 4 minutes for a MCN to be deleted (no longer exist)
func WaitForMCNToBeDeleted(clientSet *machineconfigclient.Clientset, mcnName string) error {
	o.Eventually(func() bool {
		framework.Logf("Check if MCN '%v' is deleted.", mcnName)

		// Check if MCN still exists
		_, mcnErr := clientSet.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), mcnName, metav1.GetOptions{})
		if apierrors.IsNotFound(mcnErr) {
			framework.Logf("MCN '%v' has been deleted.", mcnName)
			return true
		}
		if mcnErr != nil {
			framework.Logf("Error trying to get MCN: '%v'.", mcnErr)
			return false
		}

		framework.Logf("MCN '%v' still exists.", mcnName)
		return false
	}, 4*time.Minute, 3*time.Second).Should(o.BeTrue())
	return nil
}

// `WaitForMachineToBeDeleted` waits up to 10 minutes for a machine to be deleted (no longer exist)
func WaitForMachineToBeDeleted(machineClient *machineclient.Clientset, machineName string) error {
	o.Eventually(func() bool {
		framework.Logf("Checking if machine '%v' is deleted.", machineName)

		// Check if machine still exists
		_, machineErr := machineClient.MachineV1beta1().Machines(mapiNamespace).Get(context.TODO(), machineName, metav1.GetOptions{})
		if apierrors.IsNotFound(machineErr) {
			framework.Logf("Machine '%v' has been deleted.", machineName)
			return true
		}
		if machineErr != nil {
			framework.Logf("Error trying to get machine: %v.", machineErr)
			return false
		}

		framework.Logf("Machine '%v' still exists.", machineName)
		return false
	}, 10*time.Minute, 5*time.Second).Should(o.BeTrue())
	return nil
}

// ExecCmdOnNodeWithError behaves like ExecCmdOnNode, with the exception that
// any errors are returned to the caller for inspection. This allows one to
// execute a command that is expected to fail; e.g., stat /nonexistant/file.
func ExecCmdOnNodeWithError(oc *exutil.CLI, node corev1.Node, subArgs ...string) (string, error) {
	cmd, err := execCmdOnNode(oc, node, subArgs...)
	if err != nil {
		return "", err
	}

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ExecCmdOnNode finds a node's mcd, and oc rsh's into it to execute a command on the node
// all commands should use /rootfs as root
func ExecCmdOnNode(oc *exutil.CLI, node corev1.Node, subArgs ...string) string {
	cmd, err := execCmdOnNode(oc, node, subArgs...)
	o.Expect(err).NotTo(o.HaveOccurred(), "could not prepare to exec cmd %v on node %s: %s", subArgs, node.Name, err)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		// common err is that the mcd went down mid cmd. Re-try for good measure
		cmd, err = execCmdOnNode(oc, node, subArgs...)
		o.Expect(err).NotTo(o.HaveOccurred(), "could not prepare to exec cmd %v on node %s: %s", subArgs, node.Name, err)
		out, err = cmd.Output()

	}
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to exec cmd %v on node %s: %s", subArgs, node.Name, string(out))
	return string(out)
}

// ExecCmdOnNode finds a node's mcd, and oc rsh's into it to execute a command on the node
// all commands should use /rootfs as root
func execCmdOnNode(oc *exutil.CLI, node corev1.Node, subArgs ...string) (*exec.Cmd, error) {
	// Check for an oc binary in $PATH.
	path, err := exec.LookPath("oc")
	if err != nil {
		return nil, fmt.Errorf("could not locate oc command: %w", err)
	}

	mcd, err := mcdForNode(oc.AsAdmin().KubeClient(), &node)
	if err != nil {
		return nil, fmt.Errorf("could not get MCD for node %s: %w", node.Name, err)
	}

	mcdName := mcd.ObjectMeta.Name

	entryPoint := path
	args := []string{"rsh",
		"-n", "openshift-machine-config-operator",
		"-c", "machine-config-daemon",
		mcdName}
	args = append(args, subArgs...)

	cmd := exec.Command(entryPoint, args...)
	return cmd, nil
}

func mcdForNode(client kubernetes.Interface, node *corev1.Node) (*corev1.Pod, error) {
	// find the MCD pod that has spec.nodeNAME = node.Name and get its name:
	listOptions := metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node.Name}).String(),
	}
	listOptions.LabelSelector = labels.SelectorFromSet(labels.Set{"k8s-app": "machine-config-daemon"}).String()

	mcdList, err := client.CoreV1().Pods("openshift-machine-config-operator").List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}
	if len(mcdList.Items) != 1 {
		if len(mcdList.Items) == 0 {
			return nil, fmt.Errorf("failed to find MCD for node %s", node.Name)
		}
		return nil, fmt.Errorf("too many (%d) MCDs for node %s", len(mcdList.Items), node.Name)
	}
	return &mcdList.Items[0], nil
}

// Get nodes from a Pool
func getNodesForPool(ctx context.Context, oc *exutil.CLI, kubeClient *kubernetes.Clientset, pool *mcfgv1.MachineConfigPool) (*corev1.NodeList, error) {
	selector, err := metav1.LabelSelectorAsSelector(pool.Spec.NodeSelector)
	if err != nil {
		return nil, fmt.Errorf("invalid label selector: %w", err)
	}
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, fmt.Errorf("couldnt get nodes for mcp: %w", err)
	}
	return nodes, nil
}

// WaitForConfigAndPoolComplete is a helper function that gets a renderedConfig and waits for its pool to complete.
// The return value is the final rendered config.
func WaitForConfigAndPoolComplete(oc *exutil.CLI, pool, mcName string) string {
	config, err := WaitForRenderedConfig(oc, pool, mcName)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("%v: failed to render machine config %s from pool %s", err, mcName, pool))

	err = WaitForPoolComplete(oc, pool, config)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("%v: pool %s did not update to config %s", err, pool, config))
	return config
}

// WaitForRenderedConfig polls a MachineConfigPool until it has
// included the given mcName in its config, and returns the new
// rendered config name.
func WaitForRenderedConfig(oc *exutil.CLI, pool, mcName string) (string, error) {
	return WaitForRenderedConfigs(oc, pool, mcName)
}

// WaitForRenderedConfigs polls a MachineConfigPool until it has
// included the given mcNames in its config, and returns the new
// rendered config name.
func WaitForRenderedConfigs(oc *exutil.CLI, pool string, mcNames ...string) (string, error) {
	var renderedConfig string
	machineConfigClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	found := make(map[string]bool)
	o.Eventually(func() bool {
		// Set up the list
		for _, name := range mcNames {
			found[name] = false
		}

		// Update found based on the MCP
		mcp, err := machineConfigClient.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), pool, metav1.GetOptions{})
		if err != nil {
			return false
		}
		for _, mc := range mcp.Spec.Configuration.Source {
			if _, ok := found[mc.Name]; ok {
				found[mc.Name] = true
			}
		}

		// If any are still false, then they weren't included in the MCP
		for _, nameFound := range found {
			if !nameFound {
				return false
			}
		}

		// All the required names were found
		renderedConfig = mcp.Spec.Configuration.Name
		return true
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue())
	return renderedConfig, nil
}

// WaitForPoolComplete polls a pool until it has completed an update to target
func WaitForPoolComplete(oc *exutil.CLI, pool, target string) error {
	machineConfigClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Waiting for pool %s to complete %s", pool, target)
	o.Eventually(func() bool {
		mcp, err := machineConfigClient.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), pool, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab machineconfigpools, error :%v", err)
			return false
		}
		if mcp.Status.Configuration.Name != target {
			return false
		}
		if IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdated) {
			return true
		}
		return false
	}, 20*time.Minute, 10*time.Second).Should(o.BeTrue())
	return nil
}
