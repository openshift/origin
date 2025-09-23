package operators

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	bmhelper "github.com/openshift/origin/test/extended/baremetal"
	"github.com/stretchr/objx"
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
)

const (
	machineAPIGroup                = "machine.openshift.io"
	machineSetOwningLabel          = "machine.openshift.io/cluster-api-machineset"
	machineSetDeleteNodeAnnotaiton = "machine.openshift.io/cluster-api-delete-machine"
	scalingTime                    = 15 * time.Minute
)

// machineSetClient returns a client for machines scoped to the proper namespace
func machineSetClient(dc dynamic.Interface) dynamic.ResourceInterface {
	machineSetClient := dc.Resource(schema.GroupVersionResource{Group: machineAPIGroup, Resource: "machinesets", Version: "v1beta1"})
	return machineSetClient.Namespace(machineAPINamespace)
}

// listWorkerMachineSets list all worker machineSets
func listWorkerMachineSets(dc dynamic.Interface) ([]objx.Map, error) {
	mc := machineSetClient(dc)
	obj, err := mc.List(context.Background(), metav1.ListOptions{})
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
	machines, err := getMachinesFromMachineSet(dc, machineSetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get machines from machineset: %v", err)
	}

	// fetch nodes
	allWorkerNodes, err := c.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
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
		node, err := c.CoreV1().Nodes().Get(context.Background(), machineToNodes[machineName], metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get worker nodes %q: %v", machineToNodes[machineName], err)
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// getMachinesFromMachineSet returns an array of machines owned by a given machineSet
func getMachinesFromMachineSet(dc dynamic.Interface, machineSetName string) ([]objx.Map, error) {
	machines, err := listMachines(dc, fmt.Sprintf("%s=%s", machineSetOwningLabel, machineSetName))
	if err != nil {
		return nil, fmt.Errorf("failed to list machines: %v", err)
	}
	return machines, nil
}

// getNewestMachineNameFromMachineSet returns the name of the newest machine from the give machineSet
func getNewestMachineNameFromMachineSet(dc dynamic.Interface, machineSetName string) (string, error) {
	machines, err := getMachinesFromMachineSet(dc, machineSetName)
	if err != nil {
		return "", fmt.Errorf("failed to get machines from machineset: %v", err)
	}

	// Sort slice by descending timestamp to bring the newest to the first element
	sort.Slice(machines, func(i, j int) bool {
		return creationTimestamp(machines[i]).After(creationTimestamp(machines[j]))
	})

	return machineName(machines[0]), nil
}

// markMachineForScaleDown marks the named machine as a priority in the next machineSet
// scale down operation.
func markMachineForScaleDown(dc dynamic.Interface, machineName string) error {
	mc := machineClient(dc)
	machine, err := mc.Get(context.Background(), machineName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get machine %q: %v", machineName, err)
	}

	annotations := machine.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[machineSetDeleteNodeAnnotaiton] = "true"
	machine.SetAnnotations(annotations)

	if _, err := mc.Update(context.Background(), machine, metav1.UpdateOptions{}); err != nil {
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
func scaleMachineSet(name string, replicas int) error {
	scaleClient, err := getScaleClient()
	if err != nil {
		return fmt.Errorf("error calling getScaleClient: %v", err)
	}

	scale, err := scaleClient.Scales(machineAPINamespace).Get(context.Background(), schema.GroupResource{Group: machineAPIGroup, Resource: "MachineSet"}, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error calling scaleClient.Scales get: %v", err)
	}

	scaleUpdate := scale.DeepCopy()
	scaleUpdate.Spec.Replicas = int32(replicas)
	_, err = scaleClient.Scales(machineAPINamespace).Update(context.Background(), schema.GroupResource{Group: machineAPIGroup, Resource: "MachineSet"}, scaleUpdate, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error calling scaleClient.Scales update while setting replicas to %d: %v", err, replicas)
	}
	return nil
}

func getOperatorsNotProgressing(c configclient.Interface) map[string]metav1.Time {
	operators, err := c.ConfigV1().ClusterOperators().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	result := map[string]metav1.Time{}
	for _, operator := range operators.Items {
		for _, condition := range operator.Status.Conditions {
			if condition.Type == configv1.OperatorProgressing && condition.Status == configv1.ConditionFalse {
				result[operator.Name] = condition.LastTransitionTime
			}
		}
	}
	return result
}

var _ = g.Describe("[sig-cluster-lifecycle][Feature:Machines][Serial] Managed cluster should", func() {

	var (
		c                       *kubernetes.Clientset
		configClient            configclient.Interface
		dc                      dynamic.Interface
		helper                  *bmhelper.BaremetalTestHelper
		operatorsNotProgressing map[string]metav1.Time
	)

	g.BeforeEach(func() {

		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err = e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err = dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		// For baremetal platforms, an extra worker must be previously
		// deployed to allow subsequent scaling operations
		helper = bmhelper.NewBaremetalTestHelper(dc)
		if helper.CanDeployExtraWorkers() {
			helper.Setup()
			helper.DeployExtraWorker(0)
		}

		configClient, err = configclient.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())
		operatorsNotProgressing = getOperatorsNotProgressing(configClient)
	})

	g.AfterEach(func() {
		helper.DeleteAllExtraWorkers()

		// No cluster operator should leave Progressing=False only up to cluster scaling
		// https://github.com/openshift/api/blob/61248d910ff74aef020492922d14e6dadaba598b/config/v1/types_cluster_operator.go#L163-L164
		operatorsNotProgressingAfter := getOperatorsNotProgressing(configClient)
		var violations []string
		for operator, t1 := range operatorsNotProgressing {
			t2, ok := operatorsNotProgressingAfter[operator]
			if !ok || t1.Unix() != t2.Unix() {
				violations = append(violations, operator)
			}
		}
		o.Expect(violations).To(o.BeEmpty(), "those cluster operators left Progressing=False while cluster was scaling: %v", violations)
	})

	// The 30m timeout is essentially required by the baremetal platform environment,
	// since an extra worker is created during the test setup: it takes approx 10 minutes for
	// provisioning the new host, while it could take approx another 10 minutes for deprovisioning
	// and deleting it. The extra timeout amount should be enough to cover future slower execution
	// environments.
	g.It("grow and decrease when scaling different machineSets simultaneously [Timeout:30m][apigroup:machine.openshift.io]", func() {
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

		g.By("checking for the openshift machine api operator")
		// TODO: skip if platform != aws
		skipUnlessMachineAPIOperator(dc, c.CoreV1().Namespaces())

		g.By("fetching worker machineSets")
		machineSets, err := listWorkerMachineSets(dc)
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

		g.By("marking the newest machine for scale down")
		// To minimise disruption to existing workloads, we scale down
		// the newly created machine in each machineset as it should
		// have the fewest running workloads.
		for _, machineSet := range machineSets {
			newestMachine, err := getNewestMachineNameFromMachineSet(dc, machineName(machineSet))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Eventually(func() error {
				return markMachineForScaleDown(dc, newestMachine)
			}, 3*time.Minute, 5*time.Second).Should(o.Succeed(), "Unable to mark Machine %q for scale down", newestMachine)
		}

		g.By("scaling the machinesets back to their original size")
		for _, machineSet := range machineSets {
			scaledReplicasMachineSet := initialReplicasMachineSets[machineName(machineSet)] + 1
			g.By(fmt.Sprintf("scaling %q from %d to %d replicas", machineName(machineSet), scaledReplicasMachineSet, initialReplicasMachineSets[machineName(machineSet)]))
			err = scaleMachineSet(machineName(machineSet), initialReplicasMachineSets[machineName(machineSet)])
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
							e2e.Logf("node/%s had unexpected condition %s: %#v", node.Name, condition.Reason, condition.Status)
							return false
						}
					case corev1.NodeMemoryPressure,
						corev1.NodeDiskPressure,
						corev1.NodePIDPressure,
						corev1.NodeNetworkUnavailable:
						if condition.Status != corev1.ConditionFalse {
							e2e.Logf("node/%s had unexpected condition %s: %#v", node.Name, condition.Reason, condition.Status)
							return false
						}

					default:
						e2e.Logf("node/%s had unhandled condition %s: %#v", node.Name, condition.Reason, condition.Status)

					}
				}
				e2e.Logf("node/%s conditions seem ok", node.Name)
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
