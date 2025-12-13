package operators

import (
	"bytes"
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	g "github.com/onsi/ginkgo/v2"
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
	machineAPINamespace     = "openshift-machine-api"
	nodeLabelSelectorWorker = "node-role.kubernetes.io/worker"
	machineLabelRole        = "machine.openshift.io/cluster-api-machine-role"

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
	obj, err := mc.List(context.Background(), metav1.ListOptions{
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
	return mc.Delete(context.Background(), machineName, metav1.DeleteOptions{})
}

// machineName returns the machine name
func machineName(item objx.Map) string {
	return item.Get("metadata.name").String()
}

func creationTimestamp(item objx.Map) time.Time {
	creationString := item.Get("metadata.creationTimestamp").String()
	creation, err := time.Parse(time.RFC3339, creationString)
	if err != nil {
		// This should never happen as we always read creation timestamps
		// set by the Kube API server which sets timestamps to the RFC3339 format.
		panic(err)
	}
	return creation
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

func isNodeReady(node corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

var _ = g.Describe("[sig-cluster-lifecycle][Feature:Machines][Disruptive] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("recover from deleted worker machines [apigroup:machine.openshift.io]", g.Label("Size:L"), func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking for the openshift machine api operator")
		// TODO: skip if platform != aws
		skipUnlessMachineAPIOperator(dc, c.CoreV1().Namespaces())

		g.By("validating node and machine invariants")
		// fetch all machines
		allMachines, err := listMachines(dc, "")
		if err != nil {
			e2e.Failf("unable to fetch worker machines: %v", err)
		}

		if len(allMachines) == 0 {
			e2e.Failf("cluster should have machines")
		}

		// fetch worker nodes
		workerNodes, err := c.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: nodeLabelSelectorWorker,
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(workerNodes.Items) == 0 {
			e2e.Failf("cluster should have nodes")
		}
		// map node -> machine
		workerNodeToMachine, nodeMatch := mapNodeNameToMachine(workerNodes.Items, allMachines)
		if !nodeMatch {
			e2e.Failf("unable to map every node to machine.  workerNodeToMachine: %v, nodeName: %v", workerNodeToMachine, nodeNames(workerNodes.Items))
		}
		numMachineWorkers := len(workerNodeToMachine)

		g.By("deleting all worker nodes")
		for _, machine := range workerNodeToMachine {
			machineName := machine.Get("metadata.name").String()
			if err := deleteMachine(dc, machineName); err != nil {
				e2e.Failf("Unable to delete machine %s/%s with error: %v", machineAPINamespace, machineName, err)
			}
		}
		g.By("waiting for cluster to replace and recover workers")
		if pollErr := wait.PollImmediate(3*time.Second, machineRepairWait, func() (bool, error) {
			allMachines, err = listMachines(dc, "")
			if err != nil {
				return false, nil
			}
			workerNodes, err = c.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
				LabelSelector: nodeLabelSelectorWorker,
			})
			if err != nil {
				e2e.Logf("Waiting for workerNodes to show up, but getting error: %v", err)
				return false, nil
			}
			// map both data sets for easy comparison now
			workerNodeToMachine, nodeMatch = mapNodeNameToMachine(workerNodes.Items, allMachines)
			if numMachineWorkers != len(workerNodeToMachine) {
				e2e.Logf("Waiting for %v machines, but only found: %v", numMachineWorkers, len(workerNodeToMachine))
				return false, nil
			}

			if !nodeMatch {
				e2e.Logf("unable to map every node to machine.  workerNodeToMachine: %v\n, \tnodeName: %v", workerNodeToMachine, nodeNames(workerNodes.Items))
				return false, nil
			}
			for _, node := range workerNodes.Items {
				if !isNodeReady(node) {
					e2e.Logf("node %q is not ready: %v", node.Name, node.Status)
					return false, nil
				}
			}

			return true, nil
		}); pollErr != nil {
			buf := &bytes.Buffer{}
			w := tabwriter.NewWriter(buf, 0, 4, 1, ' ', 0)
			fmt.Fprintf(w, "NAMESPACE\tNAME\tNODE NAME\n")
			for _, machine := range workerNodeToMachine {
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
			e2e.Logf("Worker Machines:\n%s", buf.String())
			e2e.Logf("Worker node to machines:\n%v", workerNodeToMachine)
			e2e.Failf("Worker machines were not replaced as expected: %v", pollErr)
		}
		// TODO: ensure no pods pending
	})
})
