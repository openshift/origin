package clusterinfra

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"github.com/openshift/origin/test/extended/util/compat_otp/architecture"
	"github.com/tidwall/sjson"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// CreateMachineSetByArch create a new machineset by arch
func (ms *MachineSetDescription) CreateMachineSetByArch(oc *exutil.CLI, arch architecture.Architecture) {
	e2e.Logf("Creating a new MachineSets ...")
	machinesetName := GetRandomMachineSetNameByArch(oc, arch)
	machineSetJSON, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, machinesetName, "-n", MachineAPINamespace, "-o=json").OutputToFile("machineset.json")
	o.Expect(err).NotTo(o.HaveOccurred())

	bytes, _ := ioutil.ReadFile(machineSetJSON)
	machinesetjsonWithName, _ := sjson.Set(string(bytes), "metadata.name", ms.Name)
	machinesetjsonWithSelector, _ := sjson.Set(machinesetjsonWithName, "spec.selector.matchLabels.machine\\.openshift\\.io/cluster-api-machineset", ms.Name)
	machinesetjsonWithTemplateLabel, _ := sjson.Set(machinesetjsonWithSelector, "spec.template.metadata.labels.machine\\.openshift\\.io/cluster-api-machineset", ms.Name)
	machinesetjsonWithReplicas, _ := sjson.Set(machinesetjsonWithTemplateLabel, "spec.replicas", ms.Replicas)
	// Adding taints to machineset so that pods without toleration can not schedule to the nodes we provision
	machinesetjsonWithTaints, _ := sjson.Set(machinesetjsonWithReplicas, "spec.template.spec.taints.0", map[string]interface{}{"effect": "NoSchedule", "key": "mapi", "value": "mapi_test"})
	err = ioutil.WriteFile(machineSetJSON, []byte(machinesetjsonWithTaints), 0644)
	o.Expect(err).NotTo(o.HaveOccurred())

	if err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", machineSetJSON).Execute(); err != nil {
		ms.DeleteMachineSet(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		WaitForMachinesRunning(oc, ms.Replicas, ms.Name)
	}
}

// ListWorkerMachineSetNamesByArch list all linux worker machineSets by arch
func ListWorkerMachineSetNamesByArch(oc *exutil.CLI, arch architecture.Architecture) []string {
	e2e.Logf("Listing all MachineSets by arch ...")
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
		machineSetAnnotation, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, workerMachineSetName, "-o=jsonpath={.metadata.annotations.capacity\\.cluster-autoscaler\\.kubernetes\\.io/labels}", "-n", MachineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(machineSetAnnotation, architecture.NodeArchitectureLabel+"="+arch.String()) && !strings.Contains(machineSetLabels, `"machine.openshift.io/os-id":"Windows"`) {
			linuxWorkerMachineSetNames = append(linuxWorkerMachineSetNames, workerMachineSetName)
		}
	}
	e2e.Logf("linuxWorkerMachineSetNames: %s", linuxWorkerMachineSetNames)
	return linuxWorkerMachineSetNames
}

// GetRandomMachineSetNameByArch get a random MachineSet name by arch
func GetRandomMachineSetNameByArch(oc *exutil.CLI, arch architecture.Architecture) string {
	e2e.Logf("Getting a random MachineSet by arch ...")
	machinesetNames := ListWorkerMachineSetNamesByArch(oc, arch)
	if len(machinesetNames) == 0 {
		g.Skip(fmt.Sprintf("Skip this test scenario because there are no linux/%s machinesets in this cluster", arch))
	}
	return machinesetNames[rand.Int31n(int32(len(machinesetNames)))]
}

// GetArchitectureFromMachineSet get the architecuture of a machineset
func GetArchitectureFromMachineSet(oc *exutil.CLI, machineSetName string) (architecture.Architecture, error) {
	nodeNames := GetNodeNamesFromMachineSet(oc, machineSetName)
	if len(nodeNames) == 0 {
		e2e.Logf("no nodes associated with %s. Using the capacity annotation", machineSetName)
		machineSetAnnotationCapacity, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			compat_otp.MapiMachineset, machineSetName,
			"-o=jsonpath={.metadata.annotations.capacity\\.cluster-autoscaler\\.kubernetes\\.io/labels}",
			"-n", compat_otp.MachineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		capacityLabels := mapFromCommaSeparatedKV(machineSetAnnotationCapacity)
		e2e.Logf("capacityLabels: %s", capacityLabels)
		for k, v := range capacityLabels {
			if strings.Contains(k, architecture.NodeArchitectureLabel) {
				return architecture.FromString(v), nil
			}
		}
		return architecture.UNKNOWN, fmt.Errorf(
			"error getting the machineSet's nodes and unable to infer the architecture from the %s's capacity annotations",
			machineSetName)
	}
	arch, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeNames[0],
		"-o=jsonpath={.status.nodeInfo.architecture}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return architecture.FromString(arch), nil
}

// mapFromCommaSeparatedKV convert a comma separated string of key=value pairs into a map
func mapFromCommaSeparatedKV(list string) map[string]string {
	merged := make(map[string]string)
	for _, kv := range strings.Split(list, ",") {
		kv := strings.Split(kv, "=")
		if len(kv) != 2 {
			// ignore invalid key=value pairs
			continue
		}
		merged[kv[0]] = kv[1]
	}
	return merged
}

// GetInstanceTypeByProviderAndArch get intance types for this provider and architecture
func GetInstanceTypeValuesByProviderAndArch(cloudProvider PlatformType, arch architecture.Architecture) []string {
	e2e.Logf("Getting instance type by provider and arch ...")
	instanceTypesMap := map[PlatformType]map[architecture.Architecture][]string{
		AWS: {
			architecture.AMD64: {
				"m5.xlarge",
				"m6i.xlarge",
			},
			architecture.ARM64: {
				"m6gd.xlarge",
				"m6g.xlarge",
			},
		},
		GCP: {
			architecture.AMD64: {
				"n2-standard-4",
				"n2d-standard-4",
			},
			architecture.ARM64: {
				"t2a-standard-4",
				"t2a-standard-8",
			},
		},
		Azure: {
			architecture.AMD64: {
				"Standard_D4s_v3",
				"Standard_D8s_v3",
			},
			architecture.ARM64: {
				"Standard_D4ps_v5",
				"Standard_D8ps_v5",
			},
		},
	}
	return instanceTypesMap[cloudProvider][arch]
}

// GetInstanceTypeKeyByProvider get intance type key for this provider
func GetInstanceTypeKeyByProvider(cloudProvider PlatformType) string {
	e2e.Logf("Getting instance type key by provider ...")
	instanceTypeKey := map[PlatformType]string{
		AWS:   "instanceType",
		GCP:   "machineType",
		Azure: "vmSize",
	}
	return instanceTypeKey[cloudProvider]
}
