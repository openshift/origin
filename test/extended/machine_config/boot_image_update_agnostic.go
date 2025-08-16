package machine_config

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	osconfigv1 "github.com/openshift/api/config/v1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"

	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	streammeta "github.com/coreos/stream-metadata-go/stream"
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

func UploadTovCentreTest(oc *exutil.CLI, fixture string) {

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

	// Modify coreos-bootimage cm to an older known version (transient application since CVO should revert immediately)
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(mcoNamespace).Get(context.TODO(), cmName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	var stream streammeta.Stream
	err = json.Unmarshal([]byte(cm.Data["stream"]), &stream)
	o.Expect(err).NotTo(o.HaveOccurred())
	vmware := stream.Architectures["x86_64"].Artifacts["vmware"]
	currentVmwareRelease := vmware.Release
	vmware.Release = "418.94.202501221327-0"
	vmware.Formats["ova"].Disk.Location = "https://rhcos.mirror.openshift.com/art/storage/prod/streams/4.18-9.4/builds/418.94.202501221327-0/x86_64/rhcos-418.94.202501221327-0-vmware.x86_64.ova"
	vmware.Formats["ova"].Disk.Sha256 = "6fd6e9fa2ff949154d54572c3f6a0c400f3e801457aa88b585b751a0955bda19"
	stream.Architectures["x86_64"].Artifacts["vmware"] = vmware
	updatedStreamBytes, err := json.MarshalIndent(stream, "", "  ")
	o.Expect(err).NotTo(o.HaveOccurred())
	escapedStreamBytes, err := json.Marshal(string(updatedStreamBytes))
	o.Expect(err).NotTo(o.HaveOccurred())
	patchPayload := fmt.Sprintf(`{"data":{"stream":%s}}`, string(escapedStreamBytes))
	err = oc.Run("patch").Args("configmap", cmName, "-n", mcoNamespace, "-p", patchPayload).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Ensure boot image controller is not progressing
	framework.Logf("Waiting until the boot image controller is not progressing...")
	WaitForBootImageControllerToComplete(oc)

	// Ensure MSBIC moves successfully from current bootimage -> known old bootimage -> current bootimage
	currentToOldLog := fmt.Sprintf("Existing RHCOS v%s does not match current RHCOS v%s. Starting reconciliation process.", currentVmwareRelease, vmware.Release)
	oldToCurrentLog := fmt.Sprintf("Existing RHCOS v%s does not match current RHCOS v%s. Starting reconciliation process.", vmware.Release, currentVmwareRelease)
	successfullyPatchedLog := fmt.Sprintf("Successfully patched machineset %s", machineSetUnderTest.Name)
	o.Eventually(func() bool {
		podNames, err := oc.Run("get").Args(
			"pods",
			"-n", "openshift-machine-config-operator",
			"-l", "k8s-app=machine-config-controller",
			"-o", "go-template={{range .items}}{{.metadata.name}}{{\"\\n\"}}{{end}}",
		).Output()
		if err != nil {
			return false
		}
		podNamesArr := strings.Split(podNames, "\n")
		if len(podNamesArr) == 0 {
			return false
		}
		mccPodName := podNamesArr[0]
		logs, err := oc.Run("logs").Args(mccPodName, "-n", "openshift-machine-config-operator", "--tail=50").Output()
		if err != nil {
			return false
		}
		return checkOrder(logs, currentToOldLog, successfullyPatchedLog, oldToCurrentLog, successfullyPatchedLog)
	}, 15*time.Minute, 10*time.Second).Should(o.BeTrue())

	// Scale-up the machineset to verify we have not accidentally broken the bootimage updates
	mcdPods, err := getMCDPodSet(oc)
	err = oc.Run("scale").Args(mapiMachinesetQualifiedName, machineSetUnderTest.Name, "-n", mapiNamespace, fmt.Sprintf("--replicas=%d", *machineSetUnderTest.Spec.Replicas+1)).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	defer func() {
		// Scale-down the machineset at the end of test
		err = oc.Run("scale").Args(mapiMachinesetQualifiedName, machineSetUnderTest.Name, "-n", mapiNamespace, fmt.Sprintf("--replicas=%d", *machineSetUnderTest.Spec.Replicas)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}()

	o.Eventually(func() bool {
		machineset, err := machineClient.MachineV1beta1().MachineSets(mapiNamespace).Get(context.TODO(), machineSetUnderTest.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return machineset.Status.AvailableReplicas == *machineSetUnderTest.Spec.Replicas+1
	}, 15*time.Minute, 10*time.Second).Should(o.BeTrue())

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
			logs, err := oc.Run("logs").Args(pod, "-n", "openshift-machine-config-operator").Output()
			if err != nil {
				continue
			}
			lines := strings.Split(logs, "\n")
			if len(lines) > 50 {
				lines = lines[:50]
			}
			first50 := strings.Join(lines, "\n")
			if strings.Contains(first50, currentVmwareRelease) {
				return true
			}
		}
		return false
	}, 15*time.Minute, 10*time.Second).Should(o.BeTrue())
}

// checkOrder returns true if all statements appear in the log in the given order.
func checkOrder(log string, statements ...string) bool {
	if len(statements) == 0 {
		return false
	}
	parts := make([]string, len(statements))
	for i, s := range statements {
		parts[i] = regexp.QuoteMeta(s) // escape regex metacharacters
	}
	pattern := "(?s)" + strings.Join(parts, ".*") // allow anything between statements
	return regexp.MustCompile(pattern).MatchString(log)
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
