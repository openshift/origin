package clusterinfra

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"github.com/tidwall/sjson"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	MachineAPINamespace = "openshift-machine-api"
	//MapiMachineset means the fullname of mapi machineset
	MapiMachineset = "machinesets.machine.openshift.io"
	//MapiMachine means the fullname of mapi machine
	MapiMachine = "machines.machine.openshift.io"
	//MapiMHC means the fullname of mapi machinehealthcheck
	MapiMHC = "machinehealthchecks.machine.openshift.io"
)

// MachineSetDescription define fields to create machineset
type MachineSetDescription struct {
	Name     string
	Replicas int
}

// CreateMachineSet create a new machineset
func (ms *MachineSetDescription) CreateMachineSet(oc *exutil.CLI) {
	e2e.Logf("Creating a new MachineSets ...")
	machinesetName := GetRandomMachineSetName(oc)
	machineSetJSON, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, machinesetName, "-n", MachineAPINamespace, "-o=json").OutputToFile("machineset.json")
	o.Expect(err).NotTo(o.HaveOccurred())

	bytes, _ := ioutil.ReadFile(machineSetJSON)
	value1, _ := sjson.Set(string(bytes), "metadata.name", ms.Name)
	value2, _ := sjson.Set(value1, "spec.selector.matchLabels.machine\\.openshift\\.io/cluster-api-machineset", ms.Name)
	value3, _ := sjson.Set(value2, "spec.template.metadata.labels.machine\\.openshift\\.io/cluster-api-machineset", ms.Name)
	value4, _ := sjson.Set(value3, "spec.replicas", ms.Replicas)
	// Adding taints to machineset so that pods without toleration can not schedule to the nodes we provision
	value5, _ := sjson.Set(value4, "spec.template.spec.taints.0", map[string]interface{}{"effect": "NoSchedule", "key": "mapi", "value": "mapi_test"})
	err = ioutil.WriteFile(machineSetJSON, []byte(value5), 0644)
	o.Expect(err).NotTo(o.HaveOccurred())

	if err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", machineSetJSON).Execute(); err != nil {
		ms.DeleteMachineSet(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		WaitForMachinesRunning(oc, ms.Replicas, ms.Name)
	}
}

// DeleteMachineSet delete a machineset
func (ms *MachineSetDescription) DeleteMachineSet(oc *exutil.CLI) error {
	e2e.Logf("Deleting a MachineSets ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args(MapiMachineset, ms.Name, "-n", MachineAPINamespace).Execute()
}

// ListAllMachineNames list all machines
func ListAllMachineNames(oc *exutil.CLI) []string {
	e2e.Logf("Listing all Machines ...")
	machineNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-o=jsonpath={.items[*].metadata.name}", "-n", MachineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(machineNames, " ")
}

// ListWorkerMachineSetNames list all linux worker machineSets
func ListWorkerMachineSetNames(oc *exutil.CLI) []string {
	e2e.Logf("Listing all MachineSets ...")
	machineSetNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, "-o=jsonpath={.items[*].metadata.name}", "-n", MachineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if machineSetNames == "" {
		g.Skip("Skip this test scenario because there are no machinesets in this cluster")
	}
	workerMachineSetNames := strings.Split(machineSetNames, " ")
	var linuxWorkerMachineSetNames []string
	for _, workerMachineSetName := range workerMachineSetNames {
		machineSetLabels, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, workerMachineSetName, "-o=jsonpath={.spec.template.metadata.labels}", "-n", MachineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(machineSetLabels, `"machine.openshift.io/os-id":"Windows"`) {
			linuxWorkerMachineSetNames = append(linuxWorkerMachineSetNames, workerMachineSetName)
		}
	}
	e2e.Logf("linuxWorkerMachineSetNames: %s", linuxWorkerMachineSetNames)
	return linuxWorkerMachineSetNames
}

// ListWorkerMachineNames list all worker machines
func ListWorkerMachineNames(oc *exutil.CLI) []string {
	e2e.Logf("Listing all Machines ...")
	machineNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-o=jsonpath={.items[*].metadata.name}", "-l", "machine.openshift.io/cluster-api-machine-type=worker", "-n", MachineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(machineNames, " ")
}

// ListMasterMachineNames list all master machines
func ListMasterMachineNames(oc *exutil.CLI) []string {
	e2e.Logf("Listing all Machines ...")
	machineNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-o=jsonpath={.items[*].metadata.name}", "-l", "machine.openshift.io/cluster-api-machine-type=master", "-n", MachineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(machineNames, " ")
}

// ListNonOutpostWorkerNodes lists all public nodes in the aws outposts mixed cluster
func ListNonOutpostWorkerNodes(oc *exutil.CLI) []string {
	e2e.Logf("Listing all regular nodes ...")
	var nodeNames []string
	var regularNodes []string
	nodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "node-role.kubernetes.io/worker", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if nodes == "" {
		g.Skip("Skip this test scenario because there are no worker nodes in this cluster")
	}
	nodeNames = strings.Split(nodes, " ")
	for _, node := range nodeNames {
		nodeLabels, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", node, "-o=jsonpath={.metadata.labels}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(nodeLabels, "topology.ebs.csi.aws.com/outpost-id") {
			regularNodes = append(regularNodes, node)
		}
	}
	return regularNodes
}

// GetMachineNamesFromMachineSet get all Machines in a Machineset
func GetMachineNamesFromMachineSet(oc *exutil.CLI, machineSetName string) []string {
	e2e.Logf("Getting all Machines in a Machineset ...")
	machineNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-o=jsonpath={.items[*].metadata.name}", "-l", "machine.openshift.io/cluster-api-machineset="+machineSetName, "-n", MachineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Split(machineNames, " ")
}

// GetNodeNamesFromMachineSet get all Nodes in a Machineset
func GetNodeNamesFromMachineSet(oc *exutil.CLI, machineSetName string) []string {
	e2e.Logf("Getting all Nodes in a Machineset ...")
	nodeNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-o=jsonpath={.items[*].status.nodeRef.name}", "-l", "machine.openshift.io/cluster-api-machineset="+machineSetName, "-n", MachineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if nodeNames == "" {
		return []string{}
	}
	return strings.Split(nodeNames, " ")
}

// GetNodeNameFromMachine get node name for a machine
func GetNodeNameFromMachine(oc *exutil.CLI, machineName string) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, machineName, "-o=jsonpath={.status.nodeRef.name}", "-n", MachineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return nodeName
}

// GetLatestMachineFromMachineSet returns the new created machine by a given machineSet.
func GetLatestMachineFromMachineSet(oc *exutil.CLI, machineSet string) string {
	machines := GetMachineNamesFromMachineSet(oc, machineSet)
	if len(machines) == 0 {
		e2e.Logf("Unable to get the latest machine for machineset %s", machineSet)
		return ""
	}

	var machine string
	newest := time.Date(2020, 0, 1, 12, 0, 0, 0, time.UTC)
	for key := range machines {
		machineCreationTime, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, machines[key], "-o=jsonpath={.metadata.creationTimestamp}", "-n", MachineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		parsedMachineCreationTime, err := time.Parse(time.RFC3339, machineCreationTime)
		if err != nil {
			e2e.Logf("Error parsing time:", err)
			return ""
		}
		if parsedMachineCreationTime.After(newest) {
			newest = parsedMachineCreationTime
			machine = machines[key]
		}
	}
	return machine
}

// GetRandomMachineSetName get a random RHCOS MachineSet name, if it's aws outpost cluster, return a outpost machineset
func GetRandomMachineSetName(oc *exutil.CLI) string {
	e2e.Logf("Getting a random MachineSet ...")
	if IsAwsOutpostCluster(oc) {
		return GetOneOutpostMachineSet(oc)
	}
	allMachineSetNames := ListWorkerMachineSetNames(oc)
	var filteredMachineSetNames []string

	// Filter out MachineSet names containing 'rhel'
	for _, name := range allMachineSetNames {
		if !strings.Contains(name, "rhel") {
			filteredMachineSetNames = append(filteredMachineSetNames, name)
		}
	}

	// Check if there are any machine sets left after filtering
	if len(filteredMachineSetNames) == 0 {
		g.Skip("Skip this test scenario because there are no suitable machinesets in this cluster to copy")
	}

	// Return a random MachineSet name from the filtered list
	return filteredMachineSetNames[rand.Int31n(int32(len(filteredMachineSetNames)))]
}

// GetMachineSetReplicas get MachineSet replicas
func GetMachineSetReplicas(oc *exutil.CLI, machineSetName string) int {
	e2e.Logf("Getting MachineSets replicas ...")
	replicasVal, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, machineSetName, "-o=jsonpath={.spec.replicas}", "-n", MachineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	replicas, _ := strconv.Atoi(replicasVal)
	return replicas
}

// ScaleMachineSet scale a MachineSet by replicas
func ScaleMachineSet(oc *exutil.CLI, machineSetName string, replicas int) {
	e2e.Logf("Scaling MachineSets ...")
	_, err := oc.AsAdmin().WithoutNamespace().Run("scale").Args("--replicas="+strconv.Itoa(replicas), MapiMachineset, machineSetName, "-n", MachineAPINamespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	WaitForMachinesRunning(oc, replicas, machineSetName)
}

// DeleteMachine delete a machine
func DeleteMachine(oc *exutil.CLI, machineName string) error {
	e2e.Logf("Deleting Machine ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args(MapiMachine, machineName, "-n", MachineAPINamespace).Execute()
}

// WaitForMachinesRunning check if all the machines are Running in a MachineSet
func WaitForMachinesRunning(oc *exutil.CLI, machineNumber int, machineSetName string) {
	e2e.Logf("Waiting for the machines Running ...")
	if machineNumber >= 1 {
		// Wait 180 seconds first, as it uses total 1200 seconds in wait.poll, it may not be enough for some platform(s)
		time.Sleep(180 * time.Second)
	}
	pollErr := wait.Poll(60*time.Second, 1200*time.Second, func() (bool, error) {
		msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, machineSetName, "-o=jsonpath={.status.readyReplicas}", "-n", MachineAPINamespace).Output()
		machinesRunning, _ := strconv.Atoi(msg)
		if machinesRunning != machineNumber {
			phase, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machineSetName, "-o=jsonpath={.items[*].status.phase}").Output()
			if strings.Contains(phase, "Failed") {
				output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machineSetName, "-o=yaml").Output()
				e2e.Logf("%v", output)
				if strings.Contains(output, "error launching instance: Instances in the pgcluster Placement Group") {
					e2e.Logf("%v", output)
					return false, fmt.Errorf("error launching instance in the pgcluster Placement Group")
				}
				return false, fmt.Errorf("Some machine go into Failed phase!")
			}
			if strings.Contains(phase, "Provisioning") {
				output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machineSetName, "-o=yaml").Output()
				if strings.Contains(output, "InsufficientInstanceCapacity") || strings.Contains(output, "InsufficientCapacityOnOutpost") {
					e2e.Logf("%v", output)
					return false, fmt.Errorf("InsufficientInstanceCapacity")
				}
				if strings.Contains(output, "InsufficientResources") {
					e2e.Logf("%v", output)
					return false, fmt.Errorf("InsufficientResources")
				}
			}
			e2e.Logf("Expected %v  machine are not Running yet and waiting up to 1 minutes ...", machineNumber)
			return false, nil
		}
		e2e.Logf("Expected %v  machines are Running", machineNumber)
		return true, nil
	})
	if pollErr != nil {
		if pollErr.Error() == "InsufficientInstanceCapacity" {
			g.Skip("InsufficientInstanceCapacity, skip this test")
		}
		if pollErr.Error() == "InsufficientResources" {
			g.Skip("InsufficientResources, skip this test")
		}
		if pollErr.Error() == "error launching instance in the pgcluster Placement Group" {
			g.Skip("launching instance in the pgcluster Placement Group Zone is not suppoted, skip this test")
		}
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machineSetName, "-o=yaml").Output()
		e2e.Logf("%v", output)
		e2e.Failf("Expected %v  machines are not Running after waiting up to 20 minutes ...", machineNumber)
	}
	e2e.Logf("All machines are Running ...")
	//add WaitForNodesReady here because we found sometimes the machine get Running but the node is still NotReady, it will take a little longer to be Ready
	if machineNumber >= 1 {
		WaitForNodesReady(oc, machineSetName)
	}
}

// WaitForNodesReady check if all the nodes are Ready in a MachineSet, then check if node has uninitialized taint, because healthy node should not has uninitialized taint
func WaitForNodesReady(oc *exutil.CLI, machineSetName string) {
	machineNumber := GetMachineSetReplicas(oc, machineSetName)
	if machineNumber >= 1 {
		e2e.Logf("Wait nodes ready then check nodes haven't uninitialized taints...")
		err := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 60*time.Second, false, func(cxt context.Context) (bool, error) {
			for _, nodeName := range GetNodeNamesFromMachineSet(oc, machineSetName) {
				readyStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.status.conditions[?(@.type==\"Ready\")].status}").Output()
				// If node NotFound，skip check this node
				if strings.Contains(readyStatus, "NotFound") {
					e2e.Logf("Node %s does not exist, skipping...", nodeName)
					continue
				}
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("node %s readyStatus: %s", nodeName, readyStatus)
				if readyStatus != "True" {
					return false, nil
				}
				taints, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.spec.taints}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(taints).ShouldNot(o.ContainSubstring("uninitialized"))
			}
			e2e.Logf("All nodes are ready and haven't uninitialized taints ...")
			return true, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "some nodes are not ready in 1 minutes")
	}
}

// WaitForMachineFailed check if all the machines are Failed in a MachineSet
func WaitForMachineFailed(oc *exutil.CLI, machineSetName string) {
	e2e.Logf("Wait for machines to go into Failed phase")
	err := wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
		machineNames := GetMachineNamesFromMachineSet(oc, machineSetName)
		if len(machineNames) == 0 {
			e2e.Logf("no machine for machineset %s", machineSetName)
			return false, nil
		}
		for _, machine := range machineNames {
			output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, machine, "-n", "openshift-machine-api", "-o=jsonpath={.status.phase}").Output()
			if output != "Failed" {
				e2e.Logf("machine %s is not in Failed phase and waiting up to 30 seconds ...", machine)
				return false, nil
			}
		}
		e2e.Logf("machines are in Failed phase")
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "Check machines phase failed")
}

// WaitForMachineProvisioned check if all the machines are Provisioned in a MachineSet
func WaitForMachineProvisioned(oc *exutil.CLI, machineSetName string) {
	e2e.Logf("Wait for machine to go into Provisioned phase")
	err := wait.Poll(60*time.Second, 300*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+machineSetName, "-o=jsonpath={.items[0].status.phase}").Output()
		if output != "Provisioned" {
			e2e.Logf("machine is not in Provisioned phase and waiting up to 60 seconds ...")
			return false, nil
		}
		e2e.Logf("machine is in Provisioned phase")
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "Check machine phase failed")
}

// WaitForMachinesDisapper check if all the machines are Dissappered in a MachineSet
func WaitForMachinesDisapper(oc *exutil.CLI, machineSetName string) {
	e2e.Logf("Waiting for the machines Dissapper ...")
	err := wait.Poll(60*time.Second, 1200*time.Second, func() (bool, error) {
		machineNames, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-o=jsonpath={.items[*].metadata.name}", "-l", "machine.openshift.io/cluster-api-machineset="+machineSetName, "-n", MachineAPINamespace).Output()
		if machineNames != "" {
			e2e.Logf(" Still have machines are not Disappered yet and waiting up to 1 minutes ...")
			return false, nil
		}
		e2e.Logf("All machines are Disappered")
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "Wait machine disappear failed.")
}

// WaitForMachinesRunningByLabel check if all the machines with the specific labels are Running
func WaitForMachinesRunningByLabel(oc *exutil.CLI, machineNumber int, labels string) []string {
	e2e.Logf("Waiting for the machines Running ...")
	err := wait.Poll(60*time.Second, 960*time.Second, func() (bool, error) {
		msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-l", labels, "-o=jsonpath={.items[*].status.phase}", "-n", MachineAPINamespace).Output()
		machinesRunning := strings.Count(msg, "Running")
		if machinesRunning == machineNumber {
			e2e.Logf("Expected %v machines are Running", machineNumber)
			return true, nil
		}
		e2e.Logf("Expected %v machine are not Running yet and waiting up to 1 minutes ...", machineNumber)
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "Wait machine running failed.")
	msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-l", labels, "-o=jsonpath={.items[*].metadata.name}", "-n", MachineAPINamespace).Output()
	return strings.Split(msg, " ")
}

// WaitForMachineRunningByField check if the machine is Running by field
func WaitForMachineRunningByField(oc *exutil.CLI, field string, fieldValue string, labels string) string {
	e2e.Logf("Waiting for the machine Running ...")
	var newMachineName string
	err := wait.Poll(60*time.Second, 960*time.Second, func() (bool, error) {
		msg, err2 := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-l", labels, "-o=jsonpath={.items[*].metadata.name}", "-n", MachineAPINamespace).Output()
		if err2 != nil {
			e2e.Logf("The server was unable to return a response and waiting up to 1 minutes ...")
			return false, nil
		}
		for _, machineName := range strings.Split(msg, " ") {
			phase, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, machineName, "-o=jsonpath={.status.phase}", "-n", MachineAPINamespace).Output()
			machineFieldValue, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, machineName, field, "-n", MachineAPINamespace).Output()
			if phase == "Running" && machineFieldValue == fieldValue {
				e2e.Logf("The machine with field %s = %s is Running %s", field, fieldValue, machineName)
				newMachineName = machineName
				return true, nil
			}
		}
		e2e.Logf("The machine with field %s = %s is not Running and waiting up to 1 minutes ...", field, fieldValue)
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "Wait machine Running failed.")
	return newMachineName
}

// WaitForMachineRunningBySuffix check if the machine is Running by suffix
func WaitForMachineRunningBySuffix(oc *exutil.CLI, machineNameSuffix string, labels string) string {
	e2e.Logf("Waiting for the machine Running ...")
	var newMachineName string
	err := wait.Poll(60*time.Second, 960*time.Second, func() (bool, error) {
		msg, err2 := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-l", labels, "-o=jsonpath={.items[*].metadata.name}", "-n", MachineAPINamespace).Output()
		if err2 != nil {
			e2e.Logf("The server was unable to return a response and waiting up to 1 minutes ...")
			return false, nil
		}
		for _, machineName := range strings.Split(msg, " ") {
			phase, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, machineName, "-o=jsonpath={.status.phase}", "-n", MachineAPINamespace).Output()
			if phase == "Running" && strings.HasSuffix(machineName, machineNameSuffix) {
				e2e.Logf("The machine with suffix %s is Running %s", machineNameSuffix, machineName)
				newMachineName = machineName
				return true, nil
			}
		}
		e2e.Logf("The machine with suffix %s is not Running and waiting up to 1 minutes ...", machineNameSuffix)
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "Wait machine Running failed.")
	return newMachineName
}

// WaitForMachineRunningByName check if the machine is Running by name
func WaitForMachineRunningByName(oc *exutil.CLI, machineName string) {
	e2e.Logf("Waiting for %s machine Running ...", machineName)
	err := wait.Poll(60*time.Second, 960*time.Second, func() (bool, error) {
		phase, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, machineName, "-o=jsonpath={.status.phase}", "-n", MachineAPINamespace).Output()
		if phase == "Running" {
			e2e.Logf("The machine %s is Running", machineName)
			return true, nil
		}
		e2e.Logf("The machine %s is not Running and waiting up to 1 minutes ...", machineName)
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "Wait machine Running failed.")
}

// WaitForMachineDisappearBySuffix check if the machine is disappear by machine suffix
func WaitForMachineDisappearBySuffix(oc *exutil.CLI, machineNameSuffix string, labels string) {
	e2e.Logf("Waiting for the machine disappear by suffix ...")
	err := wait.Poll(60*time.Second, 1800*time.Second, func() (bool, error) {
		msg, err2 := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-l", labels, "-o=jsonpath={.items[*].metadata.name}", "-n", MachineAPINamespace).Output()
		if err2 != nil {
			e2e.Logf("The server was unable to return a response and waiting up to 1 minutes ...")
			return false, nil
		}
		for _, machineName := range strings.Split(msg, " ") {
			if strings.HasSuffix(machineName, machineNameSuffix) {
				e2e.Logf("The machine %s is not disappear and waiting up to 1 minutes ...", machineName)
				return false, nil
			}
		}
		e2e.Logf("The machine with suffix %s is disappear", machineNameSuffix)
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "Wait machine disappear by suffix failed.")
}

// WaitForMachineDisappearBySuffixAndField check if the machine is disappear by machine suffix and field
func WaitForMachineDisappearBySuffixAndField(oc *exutil.CLI, machineNameSuffix string, field string, fieldValue string, labels string) {
	e2e.Logf("Waiting for the machine disappear by suffix and field...")
	err := wait.Poll(60*time.Second, 1800*time.Second, func() (bool, error) {
		msg, err2 := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-l", labels, "-o=jsonpath={.items[*].metadata.name}", "-n", MachineAPINamespace).Output()
		if err2 != nil {
			e2e.Logf("The server was unable to return a response and waiting up to 1 minutes ...")
			return false, nil
		}
		for _, machineName := range strings.Split(msg, " ") {
			machineFieldValue, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, machineName, field, "-n", MachineAPINamespace).Output()
			if strings.HasSuffix(machineName, machineNameSuffix) && machineFieldValue == fieldValue {
				e2e.Logf("The machine %s is not disappear and waiting up to 1 minutes ...", machineName)
				return false, nil
			}
		}
		e2e.Logf("The machine with suffix %s and %s = %s is disappear", machineNameSuffix, field, fieldValue)
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "Wait machine disappear by suffix and field failed.")
}

// WaitForMachineDisappearByName check if the machine is disappear by machine name
func WaitForMachineDisappearByName(oc *exutil.CLI, machineName string) {
	e2e.Logf("Waiting for the machine disappear by name ...")
	err := wait.Poll(60*time.Second, 1800*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, machineName, "-n", MachineAPINamespace).Output()
		if strings.Contains(output, "not found") {
			e2e.Logf("machine %s is disappear", machineName)
			return true, nil
		}
		e2e.Logf("machine %s is not disappear and waiting up to 1 minutes ...", machineName)
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "Wait machine disappear by name failed.")
}

// SkipConditionally check the total number of Running machines, if greater than zero, we think machines are managed by machine api operator.
func SkipConditionally(oc *exutil.CLI) {
	msg, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "--no-headers", "-n", MachineAPINamespace).Output()
	machinesRunning := strings.Count(msg, "Running")
	if machinesRunning == 0 {
		g.Skip("Expect at least one Running machine. Found none!!!")
	}
}

// Check if the cluster uses spot instances
func UseSpotInstanceWorkersCheck(oc *exutil.CLI) bool {
	machines, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machines.machine.openshift.io", "-o=jsonpath={.items[*].metadata.name}", "-n", "openshift-machine-api", "-l", "machine.openshift.io/interruptible-instance=").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if machines != "" {
		e2e.Logf("\nSpot instance workers are used\n")
		return true
	}
	return false
}

// GetOneOutpostMachineSet return one outpost machineset name
func GetOneOutpostMachineSet(oc *exutil.CLI) string {
	outpostMachines, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/worker", "-l", "topology.ebs.csi.aws.com/outpost-id", "-o=jsonpath={.items[*].metadata.annotations.machine\\.openshift\\.io\\/machine}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	oneOutpostMachine := strings.Split(outpostMachines, " ")[0]
	start := strings.Index(oneOutpostMachine, "openshift-machine-api/")
	suffix := strings.LastIndex(oneOutpostMachine, "-")
	oneOutpostMachineSet := oneOutpostMachine[start+22 : suffix]
	e2e.Logf("oneOutpostMachineSet: %s", oneOutpostMachineSet)
	return oneOutpostMachineSet
}
