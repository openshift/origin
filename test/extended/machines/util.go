package operators

import (
	"time"

	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// machineClient returns a client for machines scoped to the proper namespace
func machineClient(dc dynamic.Interface) dynamic.ResourceInterface {
	machineClient := dc.Resource(schema.GroupVersionResource{Group: "machine.openshift.io", Resource: "machines", Version: "v1beta1"})
	return machineClient.Namespace(machineAPINamespace)
}

// machinesetClient returns a client for machineseta scoped to the proper namespace
func machinesetClient(dc dynamic.Interface) dynamic.ResourceInterface {
	machinesetClient := dc.Resource(schema.GroupVersionResource{Group: "machine.openshift.io", Resource: "machinesets", Version: "v1beta1"})
	return machinesetClient.Namespace(machineAPINamespace)
}

// machineautoscalerClient returns a client for machineautoscalers scoped to the proper namespace
func machineautoscalerClient(dc dynamic.Interface) dynamic.ResourceInterface {
	machineautoscalerClient := dc.Resource(schema.GroupVersionResource{Group: "autoscaling.openshift.io", Resource: "machineautoscalers", Version: "v1beta1"})
	return machineautoscalerClient.Namespace(machineAPINamespace)
}

// listMachines list all machines scoped by selector
func listMachines(dc dynamic.Interface, labelSelector string) ([]objx.Map, error) {
	mc := machineClient(dc)
	obj, err := mc.List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	machines := objx.Map(obj.UnstructuredContent())
	items := objects(machines.Get("items"))
	return items, nil
}

// listMachineset list all machinesets 
func listMachinesets(dc dynamic.Interface) ([]objx.Map, error) {
	msc := machinesetClient(dc)
	obj, err := msc.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	machinesets := objx.Map(obj.UnstructuredContent())
	items := objects(machinesets.Get("items"))
	return items, nil
}

// listMachineautoscaler list all machineautoscalers 
func listMachineautoscalers(dc dynamic.Interface) ([]objx.Map, error) {
	mac := machineautoscalerClient(dc)
	obj, err := mac.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	machineautoscalers := objx.Map(obj.UnstructuredContent())
	items := objects(machineautoscalers.Get("items"))
	return items, nil
}

// deleteMachine deletes the named machine
func deleteMachine(dc dynamic.Interface, machineName string) error {
	mc := machineClient(dc)
	return mc.Delete(machineName, &metav1.DeleteOptions{})
}

// machineName returns the machine name
func machineName(item objx.Map) string {
	return item.Get("metadata.name").String()
}

// nodeNames returns the names of nodes
func nodeNames(nodes []corev1.Node) sets.String {
	result := sets.NewString()
	for i := range nodes {
		result.Insert(nodes[i].Name)
	}
	return result
}

// nodeNames returns the names of nodes
func machineNames(machines []objx.Map) sets.String {
	result := sets.NewString()
	for i := range machines {
		result.Insert(machineName(machines[i]))
	}
	return result
}

// mapNodeNameToMachine returns a tuple (map node to machine by name, true if a match is found for every node)
func mapNodeNameToMachine(nodes []corev1.Node, machines []objx.Map) (map[string]objx.Map, bool) {
	result := map[string]objx.Map{}
	for i := range nodes {
		for j := range machines {
			if nodes[i].Name == nodeNameFromNodeRef(machines[j]) {
				result[nodes[i].Name] = machines[j]
				break
			}
		}
	}
	return result, len(nodes) == len(result)
}

func isNodeReady(node corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func skipUnlessMachineAPIOperator(c coreclient.NamespaceInterface) {
	err := wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		_, err := c.Get("openshift-machine-api", metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		if errors.IsNotFound(err) {
			e2e.Skipf("The cluster machines are not managed by machine api operator")
		}
		e2e.Logf("Unable to check for machine api operator: %v", err)
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func objects(from *objx.Value) []objx.Map {
	var values []objx.Map
	switch {
	case from.IsObjxMapSlice():
		return from.ObjxMapSlice()
	case from.IsInterSlice():
		for _, i := range from.InterSlice() {
			if msi, ok := i.(map[string]interface{}); ok {
				values = append(values, objx.Map(msi))
			}
		}
	}
	return values
}

func nodeNameFromNodeRef(item objx.Map) string {
	return item.Get("status.nodeRef.name").String()
}
