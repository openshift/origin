package operators

import (
	"fmt"
	"strconv"
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

	"github.com/openshift/origin/test/extended/util/ibmcloud"
)

const (
	machineAPIGroup       = "machine.openshift.io"
	machineSetOwningLabel = "machine.openshift.io/cluster-api-machineset"
	scalingTime           = 12 * time.Minute
)

// machineSetClient returns a client for machines scoped to the proper namespace
func machineSetClient(dc dynamic.Interface) dynamic.ResourceInterface {
	machineSetClient := dc.Resource(schema.GroupVersionResource{Group: machineAPIGroup, Resource: "machinesets", Version: "v1beta1"})
	return machineSetClient.Namespace(machineAPINamespace)
}

// listWorkerMachineSets list all worker machineSets
func listWorkerMachineSets(dc dynamic.Interface) ([]objx.Map, error) {
	mc := machineSetClient(dc)
	obj, err := mc.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	machineSets := []objx.Map{}
	for _, ms := range objects(objx.Map(obj.UnstructuredContent()).Get("items")) {
		e2e.Logf("Labels %v", ms.Get("spec.template.metadata.labels"))
		labels := (*ms.Get("spec.template.metadata.labels")).Data().(map[string]interface{})
		if val, ok := labels[machineLabelRole]; ok {
			if val == "worker" {
				machineSets = append(machineSets, ms)
				continue
			}
		}
	}
	return machineSets, nil
}

func getMachineSetReplicaNumber(item objx.Map) int {
	replicas, _ := strconv.Atoi(item.Get("spec.replicas").String())
	return replicas
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

	e2e.Logf("Machines found %v, nodes found: %v", machines, allWorkerNodes.Items)
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

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("error discovering client: %v", err)
	}

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
		return fmt.Errorf("error calling scaleClient.Scales update while setting replicas to %d: %v", err, replicas)
	}
	return nil
}

var _ = g.Describe("[Feature:Machines][Serial] Managed cluster should", func() {
	g.It("grow and decrease when scaling different machineSets simultaneously", func() {
		if e2e.TestContext.Provider == ibmcloud.ProviderName {
			e2e.Skipf("IBM Cloud clusters do not contain machineset resources")
		}

		// expect new nodes to come up for machineSet
		verifyNodeScalingFunc := func(c *kubernetes.Clientset, dc dynamic.Interface, expectedScaleOut int, machineSet objx.Map) bool {
			nodes, err := getNodesFromMachineSet(c, dc, machineName(machineSet))
			if err != nil {
				e2e.Logf("Error getting nodes from machineSet: %v", err)
				return false
			}
			e2e.Logf("node count : %v, expectedCount %v", len(nodes), expectedScaleOut)
			notReady := false
			for i := range nodes {
				e2e.Logf("node: %v", nodes[i].Name)
				if !isNodeReady(*nodes[i]) {
					e2e.Logf("Node %q is not ready", nodes[i].Name)
					notReady = true
				}
			}
			return !notReady && len(nodes) == expectedScaleOut
		}

		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("fetching worker machineSets")
		machineSets, err := listWorkerMachineSets(dc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(machineSets) == 0 {
			e2e.Skipf("Expects at least one worker machineset. Found none!!!")
		}

		g.By("checking initial cluster workers size")
		nodeList, err := c.CoreV1().Nodes().List(metav1.ListOptions{
			LabelSelector: nodeLabelSelectorWorker,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		initialNumberOfWorkers := len(nodeList.Items)
		g.By(fmt.Sprintf("initial cluster workers size is %v", initialNumberOfWorkers))

		initialReplicasMachineSets := map[string]int{}

		for _, machineSet := range machineSets {
			initialReplicasMachineSet := getMachineSetReplicaNumber(machineSet)
			expectedScaleOut := initialReplicasMachineSet + 1
			initialReplicasMachineSets[machineName(machineSet)] = initialReplicasMachineSet
			g.By(fmt.Sprintf("scaling %q from %d to %d replicas", machineName(machineSet), initialReplicasMachineSet, expectedScaleOut))
			err = scaleMachineSet(machineName(machineSet), expectedScaleOut)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("checking scaled up worker nodes are ready")
		for _, machineSet := range machineSets {
			expectedScaleOut := initialReplicasMachineSets[machineName(machineSet)] + 1
			o.Eventually(func() bool {
				return verifyNodeScalingFunc(c, dc, expectedScaleOut, machineSet)
			}, scalingTime, 5*time.Second).Should(o.BeTrue())
		}

		for _, machineSet := range machineSets {
			scaledReplicasMachineSet := initialReplicasMachineSets[machineName(machineSet)] + 1
			g.By(fmt.Sprintf("scaling %q from %d to %d replicas", machineName(machineSet), scaledReplicasMachineSet, initialReplicasMachineSets[machineName(machineSet)]))
			err = scaleMachineSet(machineName(machineSet), initialReplicasMachineSets[machineName(machineSet)])
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By(fmt.Sprintf("waiting for cluster to get back to original size. Final size should be %d worker nodes", initialNumberOfWorkers))
		o.Eventually(func() bool {
			nodeList, err := c.CoreV1().Nodes().List(metav1.ListOptions{
				LabelSelector: nodeLabelSelectorWorker,
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("got %v nodes, expecting %v", len(nodeList.Items), initialNumberOfWorkers))
			return len(nodeList.Items) == initialNumberOfWorkers
			// Azure actuator takes something over 3 minutes to delete a machine.
			// The worst observable case to delete a machine was 5m15s however.
			// Also, there are two instances to be deleted.
			// Rounding to 7 minutes to accomodate for future new and slower cloud providers.
		}, 7*time.Minute, 5*time.Second).Should(o.BeTrue())
	})
})
