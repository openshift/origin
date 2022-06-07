package operators

import (
	"bytes"
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machineclientset "github.com/openshift/client-go/machine/clientset/versioned"
	machinev1beta1clientset "github.com/openshift/client-go/machine/clientset/versioned/typed/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
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
func machineClient(mc machineclientset.Interface) machinev1beta1clientset.MachineInterface {
	return mc.MachineV1beta1().Machines(machineAPINamespace)
}

// listMachines list all machines scoped by selector
func listMachines(mc machineclientset.Interface, labelSelector string) (*machinev1beta1.MachineList, error) {
	c := machineClient(mc)
	machines, err := c.List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	return machines, nil
}

// deleteMachine deletes the named machine
func deleteMachine(mc machineclientset.Interface, machineName string) error {
	c := machineClient(mc)
	return c.Delete(context.Background(), machineName, metav1.DeleteOptions{})
}

// nodeNames returns the names of nodes
func nodeNames(nodes []corev1.Node) sets.String {
	result := sets.NewString()
	for i := range nodes {
		result.Insert(nodes[i].Name)
	}
	return result
}

// mapNodeNameToMachine returns a tuple (map node to machine by name, true if a match is found for every node)
func mapNodeNameToMachine(nodes []corev1.Node, machines []machinev1beta1.Machine) (map[string]machinev1beta1.Machine, bool) {
	result := map[string]machinev1beta1.Machine{}
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
func mapMachineNameToNodeName(machines []machinev1beta1.Machine, nodes []corev1.Node) (map[string]string, bool) {
	result := map[string]string{}
	for i := range machines {
		for j := range nodes {
			if nodes[j].Name == nodeNameFromNodeRef(machines[i]) {
				result[machines[i].GetName()] = nodes[j].Name
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

	g.It("recover from deleted worker machines", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		mc, err := machineclientset.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking for the openshift machine api operator")
		// TODO: skip if platform != aws
		skipUnlessMachineAPIOperator(mc, c.CoreV1().Namespaces())

		g.By("validating node and machine invariants")
		// fetch all machines
		allMachines, err := listMachines(mc, "")
		if err != nil {
			e2e.Failf("unable to fetch worker machines: %v", err)
		}

		if len(allMachines.Items) == 0 {
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
		workerNodeToMachine, nodeMatch := mapNodeNameToMachine(workerNodes.Items, allMachines.Items)
		if !nodeMatch {
			e2e.Failf("unable to map every node to machine.  workerNodeToMachine: %v, nodeName: %v", workerNodeToMachine, nodeNames(workerNodes.Items))
		}
		numMachineWorkers := len(workerNodeToMachine)

		g.By("deleting all worker nodes")
		for _, machine := range workerNodeToMachine {
			machineName := machine.GetName()
			if err := deleteMachine(mc, machineName); err != nil {
				e2e.Failf("Unable to delete machine %s/%s with error: %v", machineAPINamespace, machineName, err)
			}
		}
		g.By("waiting for cluster to replace and recover workers")
		if pollErr := wait.PollImmediate(3*time.Second, machineRepairWait, func() (bool, error) {
			allMachines, err = listMachines(mc, "")
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
			workerNodeToMachine, nodeMatch = mapNodeNameToMachine(workerNodes.Items, allMachines.Items)
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
				ns := machine.GetNamespace()
				name := machine.GetName()
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
