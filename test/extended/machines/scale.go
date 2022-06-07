package operators

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ghodss/yaml"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machineclientset "github.com/openshift/client-go/machine/clientset/versioned"
	machinev1beta1clientset "github.com/openshift/client-go/machine/clientset/versioned/typed/machine/v1beta1"
	bmhelper "github.com/openshift/origin/test/extended/baremetal"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/scale"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/utils/pointer"
)

const (
	machineAPIGroup                = "machine.openshift.io"
	machineSetOwningLabel          = "machine.openshift.io/cluster-api-machineset"
	machineSetDeleteNodeAnnotaiton = "machine.openshift.io/cluster-api-delete-machine"
	scalingTime                    = 15 * time.Minute
)

// machineSetClient returns a client for machines scoped to the proper namespace
func machineSetClient(mc machineclientset.Interface) machinev1beta1clientset.MachineSetInterface {
	return mc.MachineV1beta1().MachineSets(machineAPINamespace)
}

// listWorkerMachineSets list all worker machineSets
func listWorkerMachineSets(mc machineclientset.Interface) ([]machinev1beta1.MachineSet, error) {
	c := machineSetClient(mc)
	machineSets, err := c.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	out := []machinev1beta1.MachineSet{}
	for _, ms := range machineSets.Items {
		labels := ms.Spec.Template.ObjectMeta.Labels
		e2e.Logf("Labels %v", labels)

		if val, ok := labels[machineLabelRole]; ok {
			if val == "worker" {
				out = append(out, ms)
				continue
			}
		}
	}
	return out, nil
}

// getNodesFromMachineSet returns an array of nodes backed by machines owned by a given machineSet
func getNodesFromMachineSet(c *kubernetes.Clientset, mc machineclientset.Interface, machineSetName string, logger BufferedLogger) ([]*corev1.Node, error) {
	machines, err := getMachinesFromMachineSet(mc, machineSetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get machines from machineset: %v", err)
	}

	if err := printMachines(machines.Items, logger); err != nil {
		return nil, fmt.Errorf("failed to print machines to log: %v", err)
	}

	// fetch nodes
	allWorkerNodes, err := c.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: nodeLabelSelectorWorker,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list worker nodes: %v", err)
	}

	if err := printNodes(allWorkerNodes.Items, logger); err != nil {
		return nil, fmt.Errorf("failed to print nodes to log: %v", err)
	}

	machineToNodes, match := mapMachineNameToNodeName(machines.Items, allWorkerNodes.Items)
	if !match {
		return nil, fmt.Errorf("not all machines have a node reference: %v", machineToNodes)
	}
	var nodes []*corev1.Node
	for machineName := range machineToNodes {
		node, err := c.CoreV1().Nodes().Get(context.Background(), machineToNodes[machineName], metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get worker nodes %q: %v", machineToNodes[machineName], err)
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func printMachines(machines []machinev1beta1.Machine, logger BufferedLogger) error {
	for _, machine := range machines {
		// Clear fields that pollute the output.
		machine.ObjectMeta.ManagedFields = nil

		machineYAML, err := yaml.Marshal(machine)
		if err != nil {
			return fmt.Errorf("could not convert machine to yaml: %v", err)
		}

		logger.Logf("Found Machine %s in phase %s:\n%s", machine.GetName(), pointer.StringDeref(machine.Status.Phase, ""), string(machineYAML))
	}

	return nil
}

func printNodes(nodes []corev1.Node, logger BufferedLogger) error {
	for _, node := range nodes {
		// Clear fields that pollute the output.
		node.ObjectMeta.ManagedFields = nil

		nodeYAML, err := yaml.Marshal(node)
		if err != nil {
			return fmt.Errorf("could not convert machine to yaml: %v", err)
		}

		logger.Logf("Found Node %s:\n%s", node.GetName(), string(nodeYAML))
	}

	return nil
}

// getMachinesFromMachineSet returns an array of machines owned by a given machineSet
func getMachinesFromMachineSet(mc machineclientset.Interface, machineSetName string) (*machinev1beta1.MachineList, error) {
	machines, err := listMachines(mc, fmt.Sprintf("%s=%s", machineSetOwningLabel, machineSetName))
	if err != nil {
		return nil, fmt.Errorf("failed to list machines: %v", err)
	}
	return machines, nil
}

// getNewestMachineNameFromMachineSet returns the name of the newest machine from the give machineSet
func getNewestMachineNameFromMachineSet(mc machineclientset.Interface, machineSetName string) (string, error) {
	machines, err := getMachinesFromMachineSet(mc, machineSetName)
	if err != nil {
		return "", fmt.Errorf("failed to get machines from machineset: %v", err)
	}

	// Sort slice by descending timestamp to bring the newest to the first element
	sort.Slice(machines.Items, func(i, j int) bool {
		return machines.Items[i].GetCreationTimestamp().After(machines.Items[j].GetCreationTimestamp().Time)
	})

	return machines.Items[0].GetName(), nil
}

// markMachineForScaleDown marks the named machine as a priority in the next machineSet
// scale down operation.
func markMachineForScaleDown(mc machineclientset.Interface, machineName string) error {
	c := machineClient(mc)
	machine, err := c.Get(context.Background(), machineName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get machine %q: %v", machineName, err)
	}

	annotations := machine.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[machineSetDeleteNodeAnnotaiton] = "true"
	machine.SetAnnotations(annotations)

	if _, err := c.Update(context.Background(), machine, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update machine %q: %v", machineName, err)
	}
	return nil
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
func scaleMachineSet(name string, replicas int32) error {
	scaleClient, err := getScaleClient()
	if err != nil {
		return fmt.Errorf("error calling getScaleClient: %v", err)
	}

	scale, err := scaleClient.Scales(machineAPINamespace).Get(context.Background(), schema.GroupResource{Group: machineAPIGroup, Resource: "MachineSet"}, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error calling scaleClient.Scales get: %v", err)
	}

	scaleUpdate := scale.DeepCopy()
	scaleUpdate.Spec.Replicas = replicas
	_, err = scaleClient.Scales(machineAPINamespace).Update(context.Background(), schema.GroupResource{Group: machineAPIGroup, Resource: "MachineSet"}, scaleUpdate, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error calling scaleClient.Scales update while setting replicas to %d: %v", err, replicas)
	}
	return nil
}

var _ = g.Describe("[sig-cluster-lifecycle][Feature:Machines][Serial] Managed cluster should", func() {

	var (
		c             *kubernetes.Clientset
		dc            dynamic.Interface
		mc            machineclientset.Interface
		helper        *bmhelper.BaremetalTestHelper
		machineLogger BufferedLogger
	)

	g.BeforeEach(func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err = e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		mc, err = machineclientset.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err = dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		machineLogger = NewBufferedLogger(g.GinkgoWriter)

		// For baremetal platforms, an extra worker must be previously
		// deployed to allow subsequent scaling operations
		helper = bmhelper.NewBaremetalTestHelper(dc)
		if helper.CanDeployExtraWorkers() {
			helper.Setup()
			helper.DeployExtraWorker(0)
		}
	})

	g.AfterEach(func() {
		machineLogger.Flush()

		helper.DeleteAllExtraWorkers()
	})

	// The 30m timeout is essentially required by the baremetal platform environment,
	// since an extra worker is created during the test setup: it takes approx 10 minutes for
	// provisioning the new host, while it could take approx another 10 minutes for deprovisioning
	// and deleting it. The extra timeout amount should be enough to cover future slower execution
	// environments.
	g.It("grow and decrease when scaling different machineSets simultaneously [Timeout:30m]", func() {
		// expect new nodes to come up for machineSet
		verifyNodeScalingFunc := func(c *kubernetes.Clientset, mc machineclientset.Interface, expectedScaleOut int32, machineSet machinev1beta1.MachineSet) bool {
			// This function is called repeatedly, we only want to keep the output of the last iteration.
			machineLogger.Reset()

			nodes, err := getNodesFromMachineSet(c, mc, machineSet.GetName(), machineLogger)
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
			return !notReady && len(nodes) == int(expectedScaleOut)
		}

		g.By("checking for the openshift machine api operator")
		// TODO: skip if platform != aws
		skipUnlessMachineAPIOperator(mc, c.CoreV1().Namespaces())

		g.By("fetching worker machineSets")
		machineSets, err := listWorkerMachineSets(mc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(machineSets) == 0 {
			e2eskipper.Skipf("Expects at least one worker machineset. Found none!!!")
		}

		g.By("checking initial cluster workers size")
		nodeList, err := c.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: nodeLabelSelectorWorker,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		initialNumberOfWorkers := len(nodeList.Items)
		g.By(fmt.Sprintf("initial cluster workers size is %v", initialNumberOfWorkers))

		initialReplicasMachineSets := map[string]int32{}

		for _, machineSet := range machineSets {
			initialReplicasMachineSet := pointer.Int32Deref(machineSet.Spec.Replicas, 0)
			var expectedScaleOut int32 = initialReplicasMachineSet + 1
			initialReplicasMachineSets[machineSet.GetName()] = initialReplicasMachineSet
			g.By(fmt.Sprintf("scaling %q from %d to %d replicas", machineSet.GetName(), initialReplicasMachineSet, expectedScaleOut))
			err = scaleMachineSet(machineSet.GetName(), expectedScaleOut)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("checking scaled up worker nodes are ready")
		for _, machineSet := range machineSets {
			expectedScaleOut := initialReplicasMachineSets[machineSet.GetName()] + 1
			o.Eventually(func() bool {
				return verifyNodeScalingFunc(c, mc, expectedScaleOut, machineSet)
			}, scalingTime, 5*time.Second).Should(o.BeTrue())
		}

		g.By("marking the newest machine for scale down")
		// To minimise disruption to existing workloads, we scale down
		// the newly created machine in each machineset as it should
		// have the fewest running workloads.
		for _, machineSet := range machineSets {
			newestMachine, err := getNewestMachineNameFromMachineSet(mc, machineSet.GetName())
			o.Expect(err).NotTo(o.HaveOccurred())
			err = markMachineForScaleDown(mc, newestMachine)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("scaling the machinesets back to their original size")
		for _, machineSet := range machineSets {
			scaledReplicasMachineSet := initialReplicasMachineSets[machineSet.GetName()] + 1
			g.By(fmt.Sprintf("scaling %q from %d to %d replicas", machineSet.GetName(), scaledReplicasMachineSet, initialReplicasMachineSets[machineSet.GetName()]))
			err = scaleMachineSet(machineSet.GetName(), initialReplicasMachineSets[machineSet.GetName()])
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By(fmt.Sprintf("waiting for cluster to get back to original size. Final size should be %d worker nodes", initialNumberOfWorkers))
		o.Eventually(func() bool {
			nodeList, err := c.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
				LabelSelector: nodeLabelSelectorWorker,
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("got %v nodes, expecting %v", len(nodeList.Items), initialNumberOfWorkers))
			if len(nodeList.Items) != initialNumberOfWorkers {
				return false
			}

			g.By("ensure worker is healthy")
			for _, node := range nodeList.Items {
				for _, condition := range node.Status.Conditions {
					switch condition.Type {
					case corev1.NodeReady:
						if condition.Status != corev1.ConditionTrue {
							e2e.Logf("node/%s had unexpected condition %q == %v: %#v", node.Name, condition.Reason, condition.Status)
							return false
						}
					case corev1.NodeMemoryPressure,
						corev1.NodeDiskPressure,
						corev1.NodePIDPressure,
						corev1.NodeNetworkUnavailable:
						if condition.Status != corev1.ConditionFalse {
							e2e.Logf("node/%s had unexpected condition %q == %v: %#v", node.Name, condition.Reason, condition.Status)
							return false
						}

					default:
						e2e.Logf("node/%s had unhandled condition %q == %v: %#v", node.Name, condition.Reason, condition.Status)

					}
				}
				e2e.Logf("node/%s conditions seem ok")
			}

			return true
			// Azure actuator takes something over 3 minutes to delete a machine.
			// The worst observable case to delete a machine was 5m15s however.
			// Also, there are two instances to be deleted.
			// Rounding to 10 minutes to accommodate for future new and slower cloud providers.
			// https://bugzilla.redhat.com/show_bug.cgi?id=1812240
		}, 10*time.Minute, 5*time.Second).Should(o.BeTrue())
	})
})
