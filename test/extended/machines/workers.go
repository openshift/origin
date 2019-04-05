package operators

import (
	"bytes"
	"fmt"
	"text/tabwriter"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	machineLabelSelectorWorker = "machine.openshift.io/cluster-api-machine-role=worker"
	machineAPINamespace        = "openshift-machine-api"
	nodeLabelSelectorWorker    = "node-role.kubernetes.io/worker"

	// time after purge of machine to wait for replacement and ready node
	// TODO: tighten this further based on node lifecycle controller [appears to be ~5m30s]
	machineRepairWait = 7 * time.Minute
)

// machineClient returns a client for machines scoped to the proper namespace
func machineClient(dc dynamic.Interface) dynamic.ResourceInterface {
	machineClient := dc.Resource(schema.GroupVersionResource{Group: "machine.openshift.io", Resource: "machines", Version: "v1beta1"})
	return machineClient.Namespace(machineAPINamespace)
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

// mapNodeNameToMachineName returns a tuple (map node to machine by name, true if a match is found for every node)
func mapNodeNameToMachineName(nodes []corev1.Node, machines []objx.Map) (map[string]string, bool) {
	result := map[string]string{}
	for i := range nodes {
		for j := range machines {
			if nodes[i].Name == nodeNameFromNodeRef(machines[j]) {
				result[nodes[i].Name] = machineName(machines[j])
				break
			}
		}
	}
	return result, len(nodes) == len(result)
}

// mapMachineNameToNodeName returns a tuple (map node to machine by name, true if a match is found for every node)
func mapMachineNameToNodeName(machines []objx.Map, nodes []corev1.Node) (map[string]string, bool) {
	result := map[string]string{}
	for i := range machines {
		for j := range nodes {
			if nodes[j].Name == nodeNameFromNodeRef(machines[i]) {
				result[machineName(machines[i])] = nodes[j].Name
				break
			}
		}
	}
	return result, len(machines) == len(result)
}

var _ = g.Describe("[Feature:Machines][Disruptive] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("recover from deleted worker machines", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking for the openshift machine api operator")
		// TODO: skip if platform != aws
		skipUnlessMachineAPIOperator(c.CoreV1().Namespaces())

		g.By("validating node and machine invariants")
		// fetch machines
		machines, err := listMachines(dc, machineLabelSelectorWorker)
		if err != nil {
			e2e.Failf("unable to fetch worker machines: %v", err)
		}
		numMachineWorkers := len(machines)
		if numMachineWorkers == 0 {
			e2e.Failf("cluster should have worker machines")
		}

		// fetch nodes
		nodes, err := c.CoreV1().Nodes().List(metav1.ListOptions{
			LabelSelector: nodeLabelSelectorWorker,
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		// map node -> machine
		nodeToMachine, nodeMatch := mapNodeNameToMachineName(nodes.Items, machines)
		if !nodeMatch {
			e2e.Failf("unable to map every node to machine.  nodeToMachine: %v, nodeName: %v", nodeToMachine, nodeNames(nodes.Items))
		}
		machineToNode, machineMatch := mapMachineNameToNodeName(machines, nodes.Items)
		if !machineMatch {
			e2e.Failf("unable to map every machine to node.  machineToNode: %v, machineNames: %v", machineToNode, machineNames(machines))
		}

		g.By("deleting all worker nodes")
		for _, machine := range machines {
			machineName := machine.Get("metadata.name").String()
			if err := deleteMachine(dc, machineName); err != nil {
				e2e.Failf("Unable to delete machine %s/%s with error: %v", machineAPINamespace, machineName, err)
			}
		}

		g.By("waiting for cluster to replace and recover workers")
		if pollErr := wait.PollImmediate(3*time.Second, machineRepairWait, func() (bool, error) {
			machines, err = listMachines(dc, machineLabelSelectorWorker)
			if err != nil {
				return false, nil
			}
			if numMachineWorkers != len(machines) {
				e2e.Logf("Waiting for %v machines, but only found: %v", numMachineWorkers, len(machines))
				return false, nil
			}
			nodes, err = c.CoreV1().Nodes().List(metav1.ListOptions{
				LabelSelector: nodeLabelSelectorWorker,
			})
			if err != nil {
				return false, nil
			}
			// map both data sets for easy comparison now
			nodeToMachine, nodeMatch = mapNodeNameToMachineName(nodes.Items, machines)
			machineToNode, machineMatch = mapMachineNameToNodeName(machines, nodes.Items)
			if !nodeMatch {
				e2e.Logf("unable to map every node to machine.  nodeToMachine: %v\n, \tnodeName: %v", nodeToMachine, nodeNames(nodes.Items))
				return false, nil
			}
			if !machineMatch {
				e2e.Logf("unable to map every machine to node.  machineToNode: %v\n, \tmachineNames: %v", machineToNode, machineNames(machines))
				return false, nil
			}
			return true, nil
		}); pollErr != nil {
			buf := &bytes.Buffer{}
			w := tabwriter.NewWriter(buf, 0, 4, 1, ' ', 0)
			fmt.Fprintf(w, "NAMESPACE\tNAME\tNODE NAME\n")
			for _, machine := range machines {
				ns := machine.Get("metadata.namespace").String()
				name := machine.Get("metadata.name").String()
				nodeName := nodeNameFromNodeRef(machine)
				fmt.Fprintf(w, "%s\t%s\t%s\n",
					ns,
					name,
					nodeName,
				)
			}
			w.Flush()
			e2e.Logf("Machines:\n%s", buf.String())
			e2e.Logf("Machines to nodes:\n%v", machineToNode)
			e2e.Logf("Node to machines:\n%v", nodeToMachine)
			e2e.Failf("Worker machines were not replaced as expected: %v", pollErr)
		}

		// TODO: ensure all nodes are ready
		// TODO: ensure no pods pending
	})
})
