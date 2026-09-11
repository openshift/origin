package operators

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

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

		// Get number of nodes with machine annotation ("machine.openshift.io/machine")
		machineNodes := 0
		for _, node := range nodeItems {
			if _, ok := node.Annotations["machine.openshift.io/machine"]; ok {
				machineNodes++
			}
		}

		g.By("ensure number of Machines and Nodes are equal")
		o.Expect(machineNodes).To(o.Equal(len(machineItems)))
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

		bootTimelinesByNode := make(map[string][]bootTimelineEntry)
		errs := make([]error, 0)

		allNodeLogs := &bytes.Buffer{}
		// List nodes, set actual and expected number of reboots
		for _, node := range nodes.Items {
			nodeBoots, nodeReboots, nodeLogs, err := getNumberOfBootsForNode(oc.KubeClient(), oc.Namespace(), node.Name)
			allNodeLogs.WriteString(nodeLogs + "\n\n")
			if err != nil {
				errs = append(errs, err)
				continue
			}
			allTimelineEvents := []bootTimelineEntry{}
			allTimelineEvents = append(allTimelineEvents, nodeBoots...)
			allTimelineEvents = append(allTimelineEvents, nodeReboots...)
			sort.Sort(sort.Reverse(byTime(allTimelineEvents)))
			bootTimelinesByNode[node.Name] = allTimelineEvents

			e2e.Logf("timeline events for %q\n%v", node.Name, allTimelineEvents)
		}
		for nodeName, timelineEvents := range bootTimelinesByNode {
			// every reboot (except maybe the first), should have a rebootRequest before it.
			// we reversed the sort so we can step backwards in time like this
			for i, timelineEvent := range timelineEvents {
				// boots should be the events, reboots should be the odds
				expectedReboot := (i % 2) == 1
				expectedBoot := !expectedReboot
				isActualBoot := timelineEvent.action == "Boot"
				isActualReboot := timelineEvent.action == "RebootRequest"

				switch {
				case expectedBoot && !isActualBoot:
					errs = append(errs, fmt.Errorf("expected boot for node/%v, got %v", nodeName, timelineEvents))
				case expectedBoot && isActualBoot:
				case !expectedBoot && !isActualBoot:
				case !expectedBoot && isActualBoot:
					errs = append(errs, fmt.Errorf("unexpected boot for node/%v, got %v", nodeName, timelineEvents))
				}

				switch {
				case expectedReboot && !isActualReboot:
					errs = append(errs, fmt.Errorf("expected reboot for node/%v, got %v", nodeName, timelineEvents))
				case expectedReboot && isActualReboot:
				case !expectedReboot && !isActualReboot:
				case !expectedReboot && isActualReboot:
					errs = append(errs, fmt.Errorf("unexpected reboot for node/%v, got %v", nodeName, timelineEvents))
				}
			}
		}

		// Use gomega's WithTransform to compare actual to expected - and check that errs is empty
		o.Expect(utilerrors.NewAggregate(errs)).NotTo(o.HaveOccurred())
	})
})

func getNumberOfBootsForNode(kubeClient kubernetes.Interface, namespaceName, nodeName string) ([]bootTimelineEntry, []bootTimelineEntry, string, error) {
	ctx := context.Background()

	nodeLogs := &bytes.Buffer{}
	desiredPod := displayRebootsPod.DeepCopy()
	desiredPod.Namespace = namespaceName
	desiredPod.Spec.NodeName = nodeName

	fmt.Fprintf(nodeLogs, "checking node/%v\n", nodeName)

	// Run journalctl to collect a list of boots
	actualPod, err := kubeClient.CoreV1().Pods(namespaceName).Create(context.Background(), desiredPod, metav1.CreateOptions{})
	if err != nil {
		return nil, nil, nodeLogs.String(), fmt.Errorf("failed to create pod: %w", err)
	}

	if err := pod.WaitForPodSuccessInNamespace(ctx, kubeClient, actualPod.Name, actualPod.Namespace); err != nil {
		return nil, nil, nodeLogs.String(), fmt.Errorf("failed waiting for pod to succeed: %w", err)
	}

	// Fetch container logs with list of found boots
	containerListBootsLogs, err := pod.GetPodLogs(ctx, kubeClient, actualPod.Namespace, actualPod.Name, "list-boots")
	if err != nil {
		return nil, nil, nodeLogs.String(), fmt.Errorf("failed reading pod/logs for list-boots container: --namespace=%v pods/%v: %w", actualPod.Namespace, actualPod.Name, err)
	}
	e2e.Logf("node/%v list-boots %v", nodeName, containerListBootsLogs)

	// Fetch container logs with list of recorded reboots
	containerRebootsLogs, err := pod.GetPodLogs(ctx, kubeClient, actualPod.Namespace, actualPod.Name, "reboots")
	if err != nil {
		return nil, nil, nodeLogs.String(), fmt.Errorf("failed reading pod/logs for reboots container: --namespace=%v pods/%v: %w", actualPod.Namespace, actualPod.Name, err)
	}
	e2e.Logf("node/%v reboot-requests %v", nodeName, containerRebootsLogs)

	bootInstances, bootDiagnostics, err := parseBootInstances(containerListBootsLogs)
	for _, diagnostic := range bootDiagnostics {
		e2e.Logf("node/%v list-boots diagnostic: %s", nodeName, diagnostic)
	}
	if err != nil {
		return nil, nil, nodeLogs.String(), fmt.Errorf("failed to parse boots from --namespace=%v pods/%v err: %v pod logs: %v", actualPod.Namespace, actualPod.Name, err, containerListBootsLogs)
	}
	rebootInstances, rebootDiagnostics := parseRebootInstances(containerRebootsLogs)
	for _, diagnostic := range rebootDiagnostics {
		e2e.Logf("node/%v reboot-requests diagnostic: %s", nodeName, diagnostic)
	}

	return bootInstances, rebootInstances, nodeLogs.String(), nil
}

type bootTimelineEntry struct {
	action string
	time   time.Time
}

type byTime []bootTimelineEntry

func (a byTime) Len() int      { return len(a) }
func (a byTime) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byTime) Less(i, j int) bool {
	return a[i].time.Before(a[j].time)
}

func (e bootTimelineEntry) String() string {
	return fmt.Sprintf("%v - %v", e.time.Format(time.RFC3339), e.action)
}

func isBootHeader(fields []string) bool {
	return len(fields) == 7 &&
		fields[0] == "IDX" && fields[1] == "BOOT" && fields[2] == "ID" &&
		fields[3] == "FIRST" && fields[4] == "ENTRY" && fields[5] == "LAST" && fields[6] == "ENTRY"
}

func parseBootTimestamp(fields []string) (time.Time, error) {
	if len(fields) != 4 {
		return time.Time{}, fmt.Errorf("expected weekday, date, time, and timezone")
	}
	switch fields[0] {
	case "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun":
	default:
		return time.Time{}, fmt.Errorf("invalid weekday %q", fields[0])
	}
	return time.Parse("2006-01-02 15:04:05 UTC", strings.Join(fields[1:], " "))
}

func parseBootInstance(fields []string) (bootTimelineEntry, bool) {
	if len(fields) != 10 {
		return bootTimelineEntry{}, false
	}
	if _, err := strconv.Atoi(fields[0]); err != nil {
		return bootTimelineEntry{}, false
	}
	if len(fields[1]) != 32 {
		return bootTimelineEntry{}, false
	}
	if _, err := hex.DecodeString(fields[1]); err != nil {
		return bootTimelineEntry{}, false
	}

	bootTime, err := parseBootTimestamp(fields[2:6])
	if err != nil {
		return bootTimelineEntry{}, false
	}
	if _, err := parseBootTimestamp(fields[6:10]); err != nil {
		return bootTimelineEntry{}, false
	}

	return bootTimelineEntry{action: "Boot", time: bootTime}, true
}

func parseBootInstances(listBootsOutput string) ([]bootTimelineEntry, []string, error) {
	ret := []bootTimelineEntry{}
	var diagnostics []string

	for _, line := range strings.Split(listBootsOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if isBootHeader(fields) {
			continue
		}
		bootInstance, ok := parseBootInstance(fields)
		if !ok {
			diagnostics = append(diagnostics, line)
			continue
		}
		ret = append(ret, bootInstance)
	}

	if len(ret) == 0 {
		return nil, diagnostics, fmt.Errorf("no boot records found")
	}

	return ret, diagnostics, nil
}

func parseRebootTimestamp(value string) (time.Time, error) {
	var parsed time.Time
	var err error
	for _, layout := range []string{
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05-07:00", // for cs10, rhel10
	} {
		parsed, err = time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, err
}

func isSystemdLogindTag(value string) bool {
	const prefix = "systemd-logind["
	const suffix = "]:"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, suffix) {
		return false
	}
	pid := strings.TrimSuffix(strings.TrimPrefix(value, prefix), suffix)
	if pid == "" {
		return false
	}
	for _, char := range pid {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func parseRebootInstance(fields []string) (bootTimelineEntry, bool) {
	if len(fields) != 6 || !isSystemdLogindTag(fields[2]) || strings.Join(fields[3:], " ") != "System is rebooting." {
		return bootTimelineEntry{}, false
	}
	bootTime, err := parseRebootTimestamp(fields[0])
	if err != nil {
		return bootTimelineEntry{}, false
	}

	return bootTimelineEntry{action: "RebootRequest", time: bootTime}, true
}

func parseRebootInstances(rebootsOutput string) ([]bootTimelineEntry, []string) {
	ret := []bootTimelineEntry{}
	var diagnostics []string

	for _, line := range strings.Split(rebootsOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		rebootInstance, ok := parseRebootInstance(fields)
		if !ok {
			diagnostics = append(diagnostics, line)
			continue
		}
		ret = append(ret, rebootInstance)
	}

	return ret, diagnostics
}
