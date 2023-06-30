package operators

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/prometheus"
	"github.com/stretchr/objx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/pod"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	psapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/strings/slices"
)

var _ = g.Describe("[sig-cluster-lifecycle][Feature:Machines][Early] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have same number of Machines and Nodes [apigroup:machine.openshift.io]", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking for the openshift machine api operator")
		// TODO: skip if platform != aws
		skipUnlessMachineAPIOperator(dc, c.CoreV1().Namespaces())

		g.By("getting MachineSet list")
		machineSetClient := dc.Resource(schema.GroupVersionResource{Group: "machine.openshift.io", Resource: "machinesets", Version: "v1beta1"})
		msList, err := machineSetClient.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		machineSetList := objx.Map(msList.UnstructuredContent())
		machineSetItems := objects(machineSetList.Get("items"))

		if len(machineSetItems) == 0 {
			e2eskipper.Skipf("cluster does not have machineset resources")
		}

		g.By("getting Node list")
		nodeList, err := c.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeItems := nodeList.Items

		g.By("getting Machine list")
		machineClient := dc.Resource(schema.GroupVersionResource{Group: "machine.openshift.io", Resource: "machines", Version: "v1beta1"})
		obj, err := machineClient.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		machineList := objx.Map(obj.UnstructuredContent())
		machineItems := objects(machineList.Get("items"))

		g.By("ensure number of Machines and Nodes are equal")
		o.Expect(len(nodeItems)).To(o.Equal(len(machineItems)))
	})
})

var (
	//go:embed display_reboots_pod.yaml
	displayRebootsPodYaml []byte
	displayRebootsPod     = resourceread.ReadPodV1OrDie(displayRebootsPodYaml)
)

var _ = g.Describe("[sig-node] Managed cluster", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithPodSecurityLevel("managed-cluster-node", psapi.LevelPrivileged).AsAdmin()
	)

	var staticNodeNames []string
	g.It("record the number of nodes at the beginning of the tests [Early]", func() {
		nodeList, err := oc.KubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, node := range nodeList.Items {
			staticNodeNames = append(staticNodeNames, node.Name)
		}
	})

	// This test makes use of Prometheus metrics, which are not present in the absence of cluster-monitoring-operator, the owner for
	// the api groups tagged here.
	g.It("should report ready nodes the entire duration of the test run [Late][apigroup:monitoring.coreos.com]", func() {
		// we only consider samples since the beginning of the test
		testDuration := exutil.DurationSinceStartInSeconds().String()

		tests := map[string]bool{
			// static (nodes we collected before starting the tests) nodes should be reporting ready throughout the entire run, as long as they are older than 6m, and they still
			// exist in 1m (because prometheus doesn't support negative offsets, we have to shift the entire query left). Since
			// the late test might not catch a node not ready at the very end of the run anyway, we don't do anything special
			// to shift the test execution later, we just note that there's a scrape_interval+wait_interval gap here of up to
			// 1m30s and we can live with ith
			//
			// note:
			// we are only interested in examining the health of nodes collected at the beginning of a test suite
			// because some tests might add and remove nodes as part of their testing logic
			// nodes added dynamically naturally initially are not ready causing this query to fail
			fmt.Sprintf(`(min_over_time((max by (node) (kube_node_status_condition{condition="Ready",status="true",node=~"%s"} offset 1m) and (((max by (node) (kube_node_status_condition offset 1m))) and (0*max by (node) (kube_node_status_condition offset 7m)) and (0*max by (node) (kube_node_status_condition))))[%s:1s])) < 1`, strings.Join(staticNodeNames, "|"), testDuration): false,
		}
		err := prometheus.RunQueries(context.TODO(), oc.NewPrometheusClient(context.TODO()), tests, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should verify that nodes have no unexpected reboots [Late]", func() {
		ctx := context.Background()

		// List all nodes
		nodes, err := oc.KubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodes.Items).NotTo(o.HaveLen(0))

		expected := make(map[string]int)
		actual := make(map[string]int)
		errs := make([]error, 0)

		// List nodes, set actual and expected number of reboots
		for _, node := range nodes.Items {
			// Examine only the nodes which are available at the start of the test
			if !slices.Contains(staticNodeNames, node.Name) {
				continue
			}
			nodeBoots, nodeReboots, err := getNumberOfBootsForNode(oc.KubeClient(), oc.Namespace(), node.Name)
			if err != nil {
				errs = append(errs, err)
			}
			// actual - number of boots
			// expected - number of reboots recorded by systemd-login + 1 (first boot)
			actual[node.Name] = nodeBoots
			expected[node.Name] = nodeReboots + 1
		}
		// Use gomega's WithTransform to compare actual to expected - and check that errs is empty
		var emptyErrors = func(a interface{}) (interface{}, error) {
			if len(errs) > 0 {
				return a, fmt.Errorf("errors found: %v", errs)
			}
			return a, nil
		}
		o.Expect(actual).To(o.WithTransform(emptyErrors, o.Equal(expected)))
	})
})

func getNumberOfBootsForNode(kubeClient kubernetes.Interface, namespaceName, nodeName string) (int, int, error) {
	ctx := context.Background()

	desiredPod := displayRebootsPod.DeepCopy()
	desiredPod.Namespace = namespaceName
	desiredPod.Spec.NodeName = nodeName

	errs := []error{}
	for i := 0; i < 10; i++ {
		// Run journalctl to collect a list of boots
		actualPod, err := kubeClient.CoreV1().Pods(namespaceName).Create(context.Background(), desiredPod, metav1.CreateOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create pod: %w", err))
			continue
		}

		if err := pod.WaitForPodSuccessInNamespace(ctx, kubeClient, actualPod.Name, actualPod.Namespace); err != nil {
			errs = append(errs, fmt.Errorf("failed waiting for pod to succeed: %w", err))
			continue
		}

		// Fetch container logs with list of found boots
		containerListBootsLogs, err := pod.GetPodLogs(ctx, kubeClient, actualPod.Namespace, actualPod.Name, "list-boots")
		if err != nil {
			errs = append(errs, fmt.Errorf("failed reading pod/logs for list-boots container: --namespace=%v pods/%v: %w", actualPod.Namespace, actualPod.Name, err))
			continue
		}
		linesInListBootsContainerLogs := strings.Count(containerListBootsLogs, "\n") - 1
		if linesInListBootsContainerLogs < 1 {
			errs = append(errs, fmt.Errorf("failed to fetch boot list from --namespace=%v pods/%v pod logs: %v", actualPod.Namespace, actualPod.Name, containerListBootsLogs))
			continue
		}

		// Fetch container logs with list of recorded reboots
		containerRebootsLogs, err := pod.GetPodLogs(ctx, kubeClient, actualPod.Namespace, actualPod.Name, "reboots")
		if err != nil {
			errs = append(errs, fmt.Errorf("failed reading pod/logs for reboots container: --namespace=%v pods/%v: %w", actualPod.Namespace, actualPod.Name, err))
			continue
		}
		linesInRebootsContainerLogs := strings.Count(containerRebootsLogs, "\n") - 1
		if linesInListBootsContainerLogs < 1 {
			errs = append(errs, fmt.Errorf("failed to fetch list of reboots from --namespace=%v pods/%v pod logs: %v", actualPod.Namespace, actualPod.Name, containerListBootsLogs))
			continue
		}
		return linesInListBootsContainerLogs, linesInRebootsContainerLogs, nil
	}

	// report all the failures we saw. If we got here, we couldn't start the pod, finish the call, read the logs.
	return 0, 0, utilerrors.NewAggregate(errs)
}
