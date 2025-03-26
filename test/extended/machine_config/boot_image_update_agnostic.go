package machine_config

import (
	"context"
	"strings"
	"time"

	osconfigv1 "github.com/openshift/api/config/v1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"

	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

func AllMachineSetTest(oc *exutil.CLI, fixture string) {
	// This fixture applies a boot image update configuration that opts in all machinesets
	err := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Step through all machinesets and verify boot images are reconciled correctly.
	machineClient, err := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())

	machineSets, err := machineClient.MachineV1beta1().MachineSets("openshift-machine-api").List(context.TODO(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, ms := range machineSets.Items {
		verifyMachineSetUpdate(oc, ms, true)
	}
}

func PartialMachineSetTest(oc *exutil.CLI, fixture string) {

	// This fixture applies a boot image update configuration that opts in any machineset with the label test=boot
	err := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Ensure status accounts for the fixture that was applied
	WaitForMachineConfigurationStatusUpdate(oc)

	// Pick a random machineset to test
	machineClient, err := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	machineSetUnderTest := getRandomMachineSet(machineClient)
	framework.Logf("MachineSet under test: %s", machineSetUnderTest.Name)

	// Label this machineset with the test=boot label
	err = oc.Run("label").Args(mapiMachinesetQualifiedName, machineSetUnderTest.Name, "-n", mapiNamespace, "test=boot").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	defer func() {
		// Unlabel the machineset at the end of test
		err = oc.Run("label").Args(mapiMachinesetQualifiedName, machineSetUnderTest.Name, "-n", mapiNamespace, "test-").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}()
	// Step through all machinesets and verify that only the opted in machineset's boot images are reconciled.
	machineSets, err := machineClient.MachineV1beta1().MachineSets("openshift-machine-api").List(context.TODO(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, ms := range machineSets.Items {
		verifyMachineSetUpdate(oc, ms, machineSetUnderTest.Name == ms.Name)
	}

}

func NoneMachineSetTest(oc *exutil.CLI, fixture string) {
	// This fixture applies a boot image update configuration that opts in no machinesets, i.e. feature is disabled.
	err := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Ensure status accounts for the fixture that was applied
	WaitForMachineConfigurationStatusUpdate(oc)

	// Step through all machinesets and verify boot images are reconciled correctly.
	machineClient, err := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())

	machineSets, err := machineClient.MachineV1beta1().MachineSets("openshift-machine-api").List(context.TODO(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, ms := range machineSets.Items {
		verifyMachineSetUpdate(oc, ms, false)
	}
}

func DegradeOnOwnerRefTest(oc *exutil.CLI, fixture string) {
	e2eskipper.Skipf("This test is temporarily disabled until boot image skew enforcement is implemented")
	// This fixture applies a boot image update configuration that opts in all machinesets
	err := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Ensure status accounts for the fixture that was applied
	WaitForMachineConfigurationStatusUpdate(oc)

	// Pick a random machineset to test
	machineClient, err := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	machineSetUnderTest := getRandomMachineSet(machineClient)
	framework.Logf("MachineSet under test: %s", machineSetUnderTest.Name)

	// Add an owner reference to this machineset
	err = oc.Run("patch").Args(mapiMachinesetQualifiedName, machineSetUnderTest.Name, "-p", `{"metadata": {"ownerReferences": [{"apiVersion": "test", "kind": "test", "name": "test", "uid": "test"}]}}`, "-n", mapiNamespace, "--type=merge").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Verify that the MCO cluster operator has degraded with the correct reason
	o.Eventually(func() bool {
		co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.TODO(), "machine-config", metav1.GetOptions{})
		if err != nil {
			framework.Logf("failed to grab cluster operator, error :%v", err)
			return false
		}
		if IsClusterOperatorConditionFalse(co.Status.Conditions, osconfigv1.OperatorDegraded) {
			framework.Logf("Cluster Operator has not degraded")
			return false
		}
		degradedCondition := FindClusterOperatorStatusCondition(co.Status.Conditions, osconfigv1.OperatorDegraded)
		if !strings.Contains(degradedCondition.Message, "error syncing MAPI MachineSet "+machineSetUnderTest.Name+": unexpected OwnerReference: test/test") {
			framework.Logf("Cluster Operator degrade condition does not have the correct message")
			return false
		}
		return true
	}, 2*time.Minute, 5*time.Second).Should(o.BeTrue())
	framework.Logf("Succesfully verified that the cluster operator is degraded")

	// Remove the owner reference from this machineset
	err = oc.Run("patch").Args(mapiMachinesetQualifiedName, machineSetUnderTest.Name, "-p", `{"metadata": {"ownerReferences": []}}`, "-n", mapiNamespace, "--type=merge").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Verify that the MCO cluster operator is no longer degraded
	o.Eventually(func() bool {
		co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.TODO(), "machine-config", metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab cluster operator, error :%v", err)
			return false
		}
		if IsClusterOperatorConditionTrue(co.Status.Conditions, osconfigv1.OperatorDegraded) {
			framework.Logf("Cluster Operator is still degraded")
			return false
		}
		return true
	}, 2*time.Minute, 5*time.Second).Should(o.BeTrue())
	framework.Logf("Succesfully verified that the cluster operator is no longer degraded")
}

func EnsureConfigMapStampTest(oc *exutil.CLI, fixture string) {
	// Update boot image configmap stamps with a "fake" value, wait for it to be updated back by the operator.
	err := oc.Run("patch").Args("configmap", cmName, "-p", `{"data": {"MCOVersionHash": "fake-value"}}`, "-n", mcoNamespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.Run("patch").Args("configmap", cmName, "-p", `{"data": {"MCOReleaseImageVersion": "fake-value"}}`, "-n", mcoNamespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Ensure atleast one master node is ready
	WaitForOneMasterNodeToBeReady(oc)

	// Verify that the configmap has been updated back to the correct value
	o.Eventually(func() bool {
		cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(mcoNamespace).Get(context.TODO(), cmName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("failed to grab configmap, error :%v", err)
			return false
		}
		if cm.Data["MCOVersionHash"] == "fake-value" {
			framework.Logf("MCOVersionHash has not been restored to the original value")
			return false
		}
		if cm.Data["MCOReleaseImageVersion"] == "fake-value" {
			framework.Logf("MCOReleaseImageVersion has not been restored to the original value")
			return false
		}
		return true
	}, 2*time.Minute, 5*time.Second).Should(o.BeTrue())
	framework.Logf("Succesfully verified that the configmap has been correctly stamped")
}
