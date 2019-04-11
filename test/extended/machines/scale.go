package operators

import (
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/scale"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	machineAPIGroup       = "machine.openshift.io"
	machineSetOwningLabel = "machine.openshift.io/cluster-api-machineset"
	scalingTime           = 7 * time.Minute
)

// machineSetClient returns a client for machines scoped to the proper namespace
func machineSetClient(dc dynamic.Interface) dynamic.ResourceInterface {
	machineSetClient := dc.Resource(schema.GroupVersionResource{Group: machineAPIGroup, Resource: "machinesets", Version: "v1beta1"})
	return machineSetClient.Namespace(machineAPINamespace)
}

// listMachineSets list all machineSets scoped by selector
func listMachineSets(dc dynamic.Interface, labelSelector string) ([]objx.Map, error) {
	mc := machineSetClient(dc)
	obj, err := mc.List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	machineSets := objx.Map(obj.UnstructuredContent())
	items := objects(machineSets.Get("items"))
	return items, nil
}

func getMachineSetReplicaNumber(item objx.Map) int {
	return int(item.Get("Spec.replicas").Int32())
}

// getNodesFromMachineSet returns an array of nodes backed by machines owned by a given machineSet
func getNodesFromMachineSet(c *kubernetes.Clientset, dc dynamic.Interface, machineSetName string) ([]*corev1.Node, error) {
	machines, err := listMachines(dc, fmt.Sprintf("%s=%s", machineSetOwningLabel, machineSetName))
	if err != nil {
		return nil, fmt.Errorf("failed to list machines: %v", err)
	}

	// fetch nodes
	allWorkerNodes, err := c.CoreV1().Nodes().List(metav1.ListOptions{
		LabelSelector: nodeLabelSelectorWorker,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list worker nodes: %v", err)
	}

	machineToNodes, match := mapMachineNameToNodeName(machines, allWorkerNodes.Items)
	if !match {
		return nil, fmt.Errorf("not all machines have a node reference: %v", machineToNodes)
	}
	var nodes []*corev1.Node
	for machineName := range machineToNodes {
		node, err := c.CoreV1().Nodes().Get(machineToNodes[machineName], metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get worker nodes %q: %v", machineToNodes[machineName], err)
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func getScaleClient() (scale.ScalesGetter, error) {
	cfg, err := e2e.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting config: %v", err)
	}

	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(cfg)
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return nil, fmt.Errorf("error getting API resources: %v", err)
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	scaleKindResolver := scale.NewDiscoveryScaleKindResolver(discoveryClient)

	scaleClient, err := scale.NewForConfig(cfg, restMapper, dynamic.LegacyAPIPathResolverFunc, scaleKindResolver)
	if err != nil {
		return nil, fmt.Errorf("error creating scale client: %v", err)
	}
	return scaleClient, nil
}

// scaleMachineSet scales a machineSet with a given name to the given number of replicas
func scaleMachineSet(name string, replicas int) error {
	scaleClient, err := getScaleClient()
	if err != nil {
		return fmt.Errorf("error calling getScaleClient: %v", err)
	}

	scale, err := scaleClient.Scales(machineAPINamespace).Get(schema.GroupResource{Group: machineAPIGroup, Resource: "MachineSet"}, name)
	if err != nil {
		return fmt.Errorf("error calling scaleClient.Scales get: %v", err)
	}

	scaleUpdate := scale.DeepCopy()
	scaleUpdate.Spec.Replicas = int32(replicas)
	_, err = scaleClient.Scales(machineAPINamespace).Update(schema.GroupResource{Group: machineAPIGroup, Resource: "MachineSet"}, scaleUpdate)
	if err != nil {
		return fmt.Errorf("error calling scaleClient.Scales update: %v", err)
	}
	return nil
}

func isNodeReady(node corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

var _ = g.Describe("[Feature:Machines][Disruptive] Managed cluster should", func() {
	g.It("grow and decrease when scaling different machineSets simultaneously", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking initial cluster workers size")
		nodeList, err := c.CoreV1().Nodes().List(metav1.ListOptions{
			LabelSelector: nodeLabelSelectorWorker,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		initialNumberOfWorkers := len(nodeList.Items)
		expectedScaleOut := 3

		g.By("fetching machineSets")
		machineSets, err := listMachineSets(dc, "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(machineSets)).To(o.BeNumerically(">", 2))
		machineSet0 := machineSets[0]
		initialReplicasMachineSet0 := getMachineSetReplicaNumber(machineSet0)
		machineSet1 := machineSets[1]
		initialReplicasMachineSet1 := getMachineSetReplicaNumber(machineSet1)

		g.By(fmt.Sprintf("scaling %q from %d to %d replicas", machineName(machineSet0), initialReplicasMachineSet0, expectedScaleOut))
		err = scaleMachineSet(machineName(machineSet0), expectedScaleOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("scaling %q from %d to %d replicas", machineName(machineSet1), initialReplicasMachineSet1, expectedScaleOut))
		err = scaleMachineSet(machineName(machineSet1), expectedScaleOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		// expect new nodes to come up for machineSet0
		o.Eventually(func() bool {
			nodes, err := getNodesFromMachineSet(c, dc, machineName(machineSet0))
			if err != nil {
				e2e.Logf("Error getting nodes from machineSet: %v", err)
				return false
			}
			for i := range nodes {
				if !isNodeReady(*nodes[i]) {
					e2e.Logf("Node %q is not ready", nodes[i].Name)
					return false
				}
			}
			return len(nodes) == expectedScaleOut
		}, scalingTime, 5*time.Second).Should(o.BeTrue())

		// expect new nodes to come up for machineSet1
		o.Eventually(func() bool {
			nodes, err := getNodesFromMachineSet(c, dc, machineName(machineSet1))
			if err != nil {
				e2e.Logf("Error getting nodes from machineSet: %v", err)
				return false
			}
			for i := range nodes {
				if !isNodeReady(*nodes[i]) {
					e2e.Logf("Node %q is not ready", nodes[i].Name)
					return false
				}
			}
			return len(nodes) == expectedScaleOut
		}, scalingTime, 5*time.Second).Should(o.BeTrue())

		g.By(fmt.Sprintf("scaling %q from %d to %d replicas", machineName(machineSet0), expectedScaleOut, initialReplicasMachineSet0))
		err = scaleMachineSet(machineName(machineSet0), initialReplicasMachineSet0)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("scaling %q from %d to %d replicas", machineName(machineSet1), expectedScaleOut, initialReplicasMachineSet1))
		err = scaleMachineSet(machineName(machineSet1), initialReplicasMachineSet1)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("waiting for cluster to get back to original size. Final size should be %d worker nodes", initialNumberOfWorkers))
		o.Eventually(func() bool {
			nodeList, err := c.CoreV1().Nodes().List(metav1.ListOptions{
				LabelSelector: nodeLabelSelectorWorker,
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			return len(nodeList.Items) == initialNumberOfWorkers
		}, 1*time.Minute, 5*time.Second).Should(o.BeTrue())
	})
})
