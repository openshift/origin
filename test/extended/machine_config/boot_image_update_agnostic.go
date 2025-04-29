package machine_config

import (
	"context"
	"fmt"
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
	ApplyMachineConfigurationFixture(oc, fixture)

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
	ApplyMachineConfigurationFixture(oc, fixture)

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
	ApplyMachineConfigurationFixture(oc, fixture)

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
	ApplyMachineConfigurationFixture(oc, fixture)

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

func ScaleUpMachineSetTest(oc *exutil.CLI, fixture string) {

	// This fixture applies a boot image update configuration that opts in any machineset with the label test=boot
	ApplyMachineConfigurationFixture(oc, fixture)

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

	mcdPods, err := getMCDPodSet(oc)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Scale-up this machineset to an extra replica
	err = oc.Run("scale").Args(mapiMachinesetQualifiedName, machineSetUnderTest.Name, "-n", mapiNamespace, fmt.Sprintf("--replicas=%d", *machineSetUnderTest.Spec.Replicas+1)).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	defer func() {
		// Scale-down the machineset at the end of test
		err = oc.Run("scale").Args(mapiMachinesetQualifiedName, machineSetUnderTest.Name, "-n", mapiNamespace, fmt.Sprintf("--replicas=%d", *machineSetUnderTest.Spec.Replicas)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}()

	// TODO:Verify that the new machine from the scaled up machineset was booted from the latest bootimage
	o.Eventually(func() bool {
		updatedMcdPods, err := getMCDPodSet(oc)
		if err != nil {
			return false

		}
		newPods := diffNewPods(mcdPods, updatedMcdPods)
		if len(newPods) == 0 {
			return false
		}
		for _, pod := range newPods {
			logs, err := oc.Run("|").Args("sh", "-c", fmt.Sprintf("oc logs -n openshift-machine-config-operator %s | head -n 50", pod)).Output()
			if err != nil {
				continue
			}
			if strings.Contains(logs, "expectedRHCOSVersion") {
				return true
			}
		}
		return false
	}, 15*time.Minute, 10*time.Second).Should(o.BeTrue())
}

func diffNewPods(before, after map[string]struct{}) []string {
	var newPods []string
	for pod := range after {
		if _, found := before[pod]; !found {
			newPods = append(newPods, pod)
		}
	}
	return newPods
}

func getMCDPodSet(oc *exutil.CLI) (map[string]struct{}, error) {
	out, err := oc.Run("get").Args(
		"pods",
		"-n", "openshift-machine-config-operator",
		"-l", "k8s-app=machine-config-daemon",
		"-o", "go-template={{range .items}}{{.metadata.name}}{{\"\\n\"}}{{end}}",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get pods: %v", err)
	}

	podSet := make(map[string]struct{})
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" {
			podSet[name] = struct{}{}
		}
	}
	return podSet, nil
}
