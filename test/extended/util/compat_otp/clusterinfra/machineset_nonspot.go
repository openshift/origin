package clusterinfra

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"github.com/tidwall/sjson"

	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// MachineSetNonSpotDescription to create machineset without spot machines
type MachineSetNonSpotDescription struct {
	Name     string
	Replicas int
}

// CreateMachineSet create a new machineset
func (ms *MachineSetNonSpotDescription) CreateMachineSet(oc *exutil.CLI) {
	g.By("Creating a new MachineSets having dedicated machines ...")
	machinesetName := GetRandomMachineSetName(oc)
	machineSetJSON, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, machinesetName, "-n", MachineAPINamespace, "-o=json").OutputToFile("machineset.json")
	o.Expect(err).NotTo(o.HaveOccurred())

	bytes, _ := ioutil.ReadFile(machineSetJSON)
	machinesetjsonWithName, _ := sjson.Set(string(bytes), "metadata.name", ms.Name)
	machinesetjsonWithSelector, _ := sjson.Set(machinesetjsonWithName, "spec.selector.matchLabels.machine\\.openshift\\.io/cluster-api-machineset", ms.Name)
	machinesetjsonWithTemplateLabel, _ := sjson.Set(machinesetjsonWithSelector, "spec.template.metadata.labels.machine\\.openshift\\.io/cluster-api-machineset", ms.Name)
	machinesetjsonWithReplicas, _ := sjson.Set(machinesetjsonWithTemplateLabel, "spec.replicas", ms.Replicas)
	//Removing spot option if present , nothing happens if it is not present
	machinesetjsonNonSpot := strings.ReplaceAll(machinesetjsonWithReplicas, "\"spotVMOptions\": {},", "") //azure
	machinesetjsonNonSpot = strings.ReplaceAll(machinesetjsonNonSpot, "\"spotMarketOptions\": {},", "")   //aws
	machinesetjsonNonSpot = strings.ReplaceAll(machinesetjsonNonSpot, "\"preemptible: true\",", "")       //gcp

	err = ioutil.WriteFile(machineSetJSON, []byte(machinesetjsonNonSpot), 0o644)
	o.Expect(err).NotTo(o.HaveOccurred())
	if err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", machineSetJSON).Execute(); err != nil {
		ms.DeleteMachineSet(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	g.By("Checking machine status ...")
	FailedStatus := WaitForMachineFailedToSkip(oc, ms.Name)
	e2e.Logf("FailedStatus: %v\n", FailedStatus)
	if FailedStatus == nil {
		ms.DeleteMachineSet(oc)
		g.Skip("Something wrong invalid configuration for machines , not worth to continue")

	}
	if FailedStatus.Error() != "timed out waiting for the condition" {

		e2e.Logf("Check machineset yaml , machine is in failed status ...")
		ms.DeleteMachineSet(oc)
		g.Skip(" Failed due to invalid configuration for machines, not worth to continue")
	}
	ms.DeleteMachinesetIfDedicatedMachinesAreNotRunning(oc, ms.Replicas, ms.Name)

}

// CreateMachineSetBasedOnExisting creates a non-spot MachineSet based on an existing one
func (ms *MachineSetNonSpotDescription) CreateMachineSetBasedOnExisting(oc *exutil.CLI, existingMset string, waitForMachinesRunning bool) {
	e2e.Logf("Creating MachineSet/%s based on MachineSet/%s", ms.Name, existingMset)
	existingMsetJson, err := oc.
		AsAdmin().
		WithoutNamespace().
		Run("get").
		Args(MapiMachineset, existingMset, "-n", MachineAPINamespace, "-o=json").
		OutputToFile("machineset.json")
	o.Expect(err).NotTo(o.HaveOccurred())
	defer func() {
		_ = os.Remove(existingMsetJson)
	}()

	existingMsetJsonBytes, err := os.ReadFile(existingMsetJson)
	o.Expect(err).NotTo(o.HaveOccurred())
	existingMsetJsonStr, err := sjson.Set(string(existingMsetJsonBytes), "metadata.name", ms.Name)
	o.Expect(err).NotTo(o.HaveOccurred())
	existingMsetJsonStr, err = sjson.Set(existingMsetJsonStr, "spec.selector.matchLabels.machine\\.openshift\\.io/cluster-api-machineset", ms.Name)
	o.Expect(err).NotTo(o.HaveOccurred())
	existingMsetJsonStr, err = sjson.Set(existingMsetJsonStr, "spec.template.metadata.labels.machine\\.openshift\\.io/cluster-api-machineset", ms.Name)
	o.Expect(err).NotTo(o.HaveOccurred())
	existingMsetJsonStr, err = sjson.Set(existingMsetJsonStr, "spec.replicas", ms.Replicas)
	o.Expect(err).NotTo(o.HaveOccurred())
	// Disable spot options for Azure
	existingMsetJsonStr = strings.ReplaceAll(existingMsetJsonStr, "\"spotVMOptions\": {},", "")
	// Disable spot options for AWS
	existingMsetJsonStr = strings.ReplaceAll(existingMsetJsonStr, "\"spotMarketOptions\": {},", "")
	// Disable spot options for GCP
	existingMsetJsonStr = strings.ReplaceAll(existingMsetJsonStr, "\"preemptible: true\",", "")
	err = os.WriteFile(existingMsetJson, []byte(existingMsetJsonStr), 0644)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", existingMsetJson).Execute()
	if err != nil {
		errDeleteMset := ms.DeleteMachineSet(oc)
		e2e.Failf("Error creating/deleting machineset: %v", errors.NewAggregate([]error{err, errDeleteMset}))
	}
	if waitForMachinesRunning {
		WaitForMachinesRunning(oc, ms.Replicas, ms.Name)
	}
	return
}

// DeleteMachineSet delete a machineset
func (ms *MachineSetNonSpotDescription) DeleteMachineSet(oc *exutil.CLI) error {
	compat_otp.By("Deleting a MachineSet ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args(MapiMachineset, ms.Name, "-n", MachineAPINamespace).Execute()
}

// DeleteMachinesetIfDedicatedMachinesAreNotRunning check labeled machines are running if not delete machineset
func (ms *MachineSetNonSpotDescription) DeleteMachinesetIfDedicatedMachinesAreNotRunning(oc *exutil.CLI, machineNumber int, machineSetName string) {
	g.By("Waiting for the machines Running ...")
	pollErr := wait.Poll(60*time.Second, 920*time.Second, func() (bool, error) {
		msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, machineSetName, "-o=jsonpath={.status.readyReplicas}", "-n", MachineAPINamespace).Output()
		machinesRunning, _ := strconv.Atoi(msg)
		if machinesRunning != machineNumber {
			e2e.Logf("Expected %v  machine are not Running yet and waiting up to 1 minutes ...", machineNumber)
			return false, nil
		}
		e2e.Logf("Expected %v  machines are Running", machineNumber)
		return true, nil
	})
	if pollErr != nil {
		ms.DeleteMachineSet(oc)
		compat_otp.AssertWaitPollNoErr(pollErr, fmt.Sprintf("Expected %v  machines are not Running after waiting up to 12 minutes ...", machineNumber))
	}
	g.By("All machines are Running ...")
}

// WaitForDedicatedMachineFailedToSkip for machines if failed to help skip test early
func WaitForDedicatedMachineFailedToSkip(oc *exutil.CLI, machineSetName string) error {
	g.By("Wait for machine to go into Failed phase")
	err := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machineSetName, "-o=jsonpath={.items[0].status.phase}").Output()
		if output != "Failed" {
			g.By("machine is not in Failed phase and waiting up to 10 seconds ...")
			return false, nil
		}
		g.By("machine is in Failed phase")
		return true, nil
	})

	return err

}
