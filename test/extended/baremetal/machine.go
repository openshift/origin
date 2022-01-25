package baremetal

import (
	"context"
	"fmt"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/scale"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"strconv"
)

func machineSetClient(dc dynamic.Interface) dynamic.ResourceInterface {
	machineClient := dc.Resource(schema.GroupVersionResource{Group: "machine.openshift.io", Resource: "machinesets", Version: "v1beta1"})
	return machineClient.Namespace(machineAPINamespace)
}

func getScaleClient() (scale.ScalesGetter, error) {
	cfg, err := e2e.LoadConfig()
	o.Expect(err).NotTo(o.HaveOccurred())

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	o.Expect(err).NotTo(o.HaveOccurred())

	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	o.Expect(err).NotTo(o.HaveOccurred())

	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	scaleKindResolver := scale.NewDiscoveryScaleKindResolver(discoveryClient)

	scaleClient, err := scale.NewForConfig(cfg, restMapper, dynamic.LegacyAPIPathResolverFunc, scaleKindResolver)
	o.Expect(err).NotTo(o.HaveOccurred())

	return scaleClient, nil
}

func getObjects(from *objx.Value) []objx.Map {
	var values []objx.Map
	switch {
	case from.IsObjxMapSlice():
		return from.ObjxMapSlice()
	case from.IsInterSlice():
		for _, i := range from.InterSlice() {
			if msi, ok := i.(map[string]interface{}); ok {
				values = append(values, msi)
			}
		}
	}
	return values
}

func getMachineSetReplicaNumber(item objx.Map) int {
	replicas, _ := strconv.Atoi(item.Get("spec.replicas").String())
	return replicas
}

// getMachineName returns the machine name from given the item
func getMachineName(item objx.Map) string {
	return item.Get("metadata.name").String()
}

// getWorkerMachineSet returns the machineset will be used for scaling.
func getWorkerMachineSet(dc dynamic.Interface) objx.Map {
	mc := machineSetClient(dc)
	obj, err := mc.List(context.TODO(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	var machineSets []objx.Map
	for _, ms := range getObjects(objx.Map(obj.UnstructuredContent()).Get("items")) {
		labels := (*ms.Get("spec.template.metadata.labels")).Data().(map[string]interface{})
		if val, ok := labels["machine.openshift.io/cluster-api-machine-role"]; ok {
			if val == "worker" {
				machineSets = append(machineSets, ms)
				continue
			}
		}
	}

	if len(machineSets) == 0 {
		e2eskipper.Skipf("no machineset found to be used for scaling.")
	}

	// In baremetal, we'd expect only one machineset.
	return machineSets[0]
}

func getWorkerMachineSetByName(dc dynamic.Interface, name string) objx.Map {
	mc := machineSetClient(dc)
	obj, err := mc.Get(context.TODO(), name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.UnstructuredContent()
}

// scaleMachineSet scales a machineSet with a given name to the given number of replicas
func scaleMachineSet(name string, replicas int) {
	scaleClient, err := getScaleClient()
	o.Expect(err).NotTo(o.HaveOccurred())

	s, err := scaleClient.Scales(machineAPINamespace).Get(context.TODO(), schema.GroupResource{Group: "machine.openshift.io", Resource: "MachineSet"}, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	scaleUpdate := s.DeepCopy()
	scaleUpdate.Spec.Replicas = int32(replicas)
	_, err = scaleClient.Scales(machineAPINamespace).Update(context.TODO(), schema.GroupResource{Group: "machine.openshift.io", Resource: "MachineSet"}, scaleUpdate, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// ScaleUpToExtraWorkers scales machine resources to provision
// extra workers up to the given scale number.
func ScaleUpToExtraWorkers(client dynamic.Interface, scale int) string {
	ms := getWorkerMachineSet(client)
	expectedScaleUp := getMachineSetReplicaNumber(ms) + scale
	machineSetName := getMachineName(ms)
	g.By(fmt.Sprintf("scale machineset %s to %v", machineSetName, expectedScaleUp))
	scaleMachineSet(getMachineName(ms), expectedScaleUp)
	return machineSetName
}

// ScaleDownFromExtraWorkers scales machine resources to deprovision extra workers
// down from the given scale number.
func ScaleDownFromExtraWorkers(dc dynamic.Interface, name string, scale int) {
	ms := getWorkerMachineSetByName(dc, name)
	expectedScaleDown := getMachineSetReplicaNumber(ms) - scale
	g.By(fmt.Sprintf("scale machineset %s to %v", name, expectedScaleDown))
	scaleMachineSet(getMachineName(ms), expectedScaleDown)
}
