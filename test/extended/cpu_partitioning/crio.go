package cpu_partitioning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	ocpv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
)

// This collection script queries CRIO for all it's containers, that data is then filtered down to only a subset
// of information needed for validating workload partitioning and the PIDs are mapped to their corresponding host
// CPUSet via the taskset command.
const collectionScript = `
#!/bin/bash

container_ids=$(sudo crictl ps -q)
container_data=$(sudo crictl inspect $container_ids)

# Filter down CRIO data to relevant information only
workload_containers=$(echo $container_data | jq -rs '[ 
	.[] | select(.info.runtimeSpec.annotations["target.workload.openshift.io/management"]) | 
	{
		cpuSet: (.info.runtimeSpec.linux.resources.cpu.cpus // ""),
        cpuShare: .info.runtimeSpec.linux.resources.cpu.shares,
		annotations: .info.runtimeSpec.annotations,
        podNamespace: .info.runtimeSpec.annotations["io.kubernetes.pod.namespace"],
        podName: .info.runtimeSpec.annotations["io.kubernetes.pod.name"],
		name: .status.metadata.name,
		pid: .info.pid, 
		hostname: .info.runtimeSpec.hostname 
	}
	]')

# Get the host CPUSet for each PID, if during regular e2e tests a POD comes up and is done
# in between us recording a CRIO process and checking taskset, we record -1 for CPUSet that PID and filter it out later.
workload_host_cpu_set=$(echo $workload_containers | jq -r '.[]|.pid' | \
while read i; do (taskset -cp $i 2>/dev/null || echo "pid $i's no longer runs: -1"); done | awk '{printf "{\"hostCPUSet\": \"%s\"}", $6}' | jq -rs)

# Map PIDs and the CRIO data into the final results.
final_results=$(echo "$workload_containers $workload_host_cpu_set" | jq -rs '.[0] as $file1 | .[1] as $file2 | map($file1[] + $file2[]) | unique')

# Filter out results that we failed to get a CPUSet for due to failure with taskset.
echo "$final_results" | jq -rc '[.[] | select(.hostCPUSet != "-1")]'
`

const jsonKeyCPUShare = "cpushares"

// Collection data for each container in a node
type crioContainerData struct {
	PodNamespace string            `json:"podNamespace"`
	PodName      string            `json:"podName"`
	Name         string            `json:"name"`
	CPUSet       string            `json:"cpuSet"`
	CPUShare     int               `json:"cpuShare"`
	Annotations  map[string]string `json:"annotations"`
	HostCPUSet   string            `json:"hostCPUSet"`
	Hostname     string            `json:"hostname"`
	Pid          int               `json:"pid"`
}

// Container struct for master and worker node CPUSets
type nodeCPUSets struct {
	Master cpuset.CPUSet
	Worker cpuset.CPUSet
}

// getAnnotationCPUShare Looks through the annotations to find the workload partitioning annotation and
// returns the cpushares or error if no such annotation exists.
func (c *crioContainerData) getAnnotationCPUShare() (int, error) {
	anno := fmt.Sprintf("%s/%s", WorkloadAnnotationPrefix, c.Name)
	cpuShare, ok := c.Annotations[anno]
	if !ok {
		return 0, fmt.Errorf("workload annotation of [%s] was expected but not found", anno)
	}

	// workload cpushare annotations have a json string that will contain `{ "cpushares": <share int> }`
	// we parse and return that value here.
	temp := map[string]int{}
	err := json.Unmarshal([]byte(cpuShare), &temp)
	if err != nil {
		return 0, fmt.Errorf("err parsing cpushare json annotation: %w", err)
	}

	value, ok := temp[jsonKeyCPUShare]
	if !ok {
		return 0, fmt.Errorf("err cpushare annotation was incorrect expected format {\"cpushares\": <int> }")
	}

	return value, nil
}

var _ = g.Describe("[sig-node][apigroup:config.openshift.io] CPU Partitioning node validation", func() {

	var (
		oc                      = exutil.NewCLIWithoutNamespace("cpu-partitioning").AsAdmin()
		ctx                     = context.Background()
		isClusterCPUPartitioned = false
	)

	g.BeforeEach(func() {
		isClusterCPUPartitioned = getCpuPartitionedStatus(oc) == ocpv1.CPUPartitioningAllNodes
	})

	g.It("should have correct cpuset and cpushare set in crio containers", func() {

		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// HyperShift clusters do not run the PerformanceProfile controller nor do they support
		// MachineConfigs, Workload partitioning feature uses those components to configure CPUSets.
		if *controlPlaneTopology == ocpv1.ExternalTopologyMode {
			g.Skip("Clusters with external control plane topology do not run PerformanceProfile Controller")
		}

		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred(), "error listing cluster nodes")

		nodeRoleCPUSet, err := getExpectedCPUSetConstraints(ctx, oc, isClusterCPUPartitioned)
		o.Expect(err).ToNot(o.HaveOccurred(), "error getting node CPUSets")

		for _, node := range nodes.Items {
			// Collect the container data from the node
			crioData, err := collectContainerInfo(ctx, oc.AdminConfig(), oc.AsAdmin().KubeFramework().ClientSet, node)
			o.Expect(err).ToNot(o.HaveOccurred(), fmt.Sprintf("error getting crio container data from node %s", node.Name))

			// When the cluster is CPU Partitioned there should always be pods with workload annotations. However,
			// we do not enforce this check on non partitioned clusters since we don't expect nodes beyond masters to have pods
			// that contain workload annotations. In the event that they do, we allow the logic to continue to validate
			// CPUSets.
			if isClusterCPUPartitioned {
				o.Expect(crioData).ToNot(o.BeEmpty(), "error getting crio container data, container data should not be empty")
			}

			// CPU Core counts are numbers 0-(n-1) we iterate and create a CPUSet to compare.
			// Note: CPU Value should be whole core counts, if situations come up where that's not the case
			// we will need to revisit this approach.
			totalCoreCount := []int{}
			for x := 0; x < int(node.Status.Capacity.Cpu().Value()); x++ {
				totalCoreCount = append(totalCoreCount, x)
			}
			parsedFullNodeCPUSet := cpuset.New(totalCoreCount...)

			expectedCPUSet := cpuset.CPUSet{}

			if _, isMaster := node.Labels[nodeMasterLabel]; isMaster {
				expectedCPUSet = nodeRoleCPUSet.Master
			} else if _, isWorker := node.Labels[nodeWorkerLabel]; isWorker {
				expectedCPUSet = nodeRoleCPUSet.Worker
			}

			for _, containerInfo := range crioData {
				parsedContainer, err := cpuset.Parse(containerInfo.CPUSet)
				o.Expect(err).ToNot(o.HaveOccurred(), fmt.Sprintf("error parsing crio container cpuset %s", containerInfo.CPUSet))
				parsedHost, err := cpuset.Parse(containerInfo.HostCPUSet)
				o.Expect(err).ToNot(o.HaveOccurred(), fmt.Sprintf("error parsing host cpuset %s", containerInfo.HostCPUSet))

				o.Expect(parsedContainer.Equals(expectedCPUSet)).To(o.BeTrue(), "cpusets do not match between container and desired")

				// Empty CPUSets mean wide open, so we check the processes are not being limited.
				// If the expected CPUSet is not empty, we make sure the host is respecting it.
				if expectedCPUSet.IsEmpty() {
					o.Expect(parsedHost.Equals(parsedFullNodeCPUSet)).To(o.BeTrue(), fmt.Sprintf("expected host CPUSet: %s got: %s", parsedHost, parsedFullNodeCPUSet))
				} else {
					o.Expect(parsedContainer.Equals(parsedHost)).To(o.BeTrue(), fmt.Sprintf("expected: %s got: %s", parsedContainer, parsedHost))
				}

				// If we are in a CPU Partitioned cluster, containers MUST be annotated with the correct CPU Share at the CRIO level
				// and the desired annotation cpu shares must equal the crio config cpu shares
				if isClusterCPUPartitioned {
					share, err := containerInfo.getAnnotationCPUShare()
					o.Expect(err).ToNot(o.HaveOccurred(), "failed to get cpushares annotation json", err)
					o.Expect(share).To(o.Equal(containerInfo.CPUShare), "cpushares do not match between crio config and desired")
				}
			}
		}
	})
})

func getExpectedCPUSetConstraints(ctx context.Context, oc *exutil.CLI, isClusterPartitioned bool) (*nodeCPUSets, error) {
	// Default behavior is that CPUSets are empty, which means wide open to the host.
	// There should be no CPUSet constraint if no PerformanceProfile is present.
	nodeRoleCPUSet := &nodeCPUSets{
		Master: cpuset.New(),
		Worker: cpuset.New(),
	}

	if !isClusterPartitioned {
		return nodeRoleCPUSet, nil
	}

	// Performance profiles dictate constrained CPUSets, we gather them here to compare.
	// Note: We're using a dynamic client here to avoid importing the PerformanceProfile
	// for a simple query and keep this code change small. If we end up needing more interaction
	// with PerformanceProfiles, then we should import the package and update this call.
	performanceProfiles, err := oc.AdminDynamicClient().
		Resource(schema.GroupVersionResource{
			Resource: "performanceprofiles",
			Group:    "performance.openshift.io",
			Version:  "v2"}).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing performance profiles: %w", err)
	}

	for _, profile := range performanceProfiles.Items {

		selector, found, err := unstructured.NestedStringMap(profile.Object, "spec", "nodeSelector")
		if err != nil {
			return nil, fmt.Errorf("error getting nodeSelector from PerformanceProfile: %w", err)
		}
		if !found {
			return nil, fmt.Errorf("expected spec.nodeSelector to be found in PerformanceProfile(%s)", profile.GetName())
		}

		reservedCPUSet, found, err := unstructured.NestedString(profile.Object, "spec", "cpu", "reserved")
		if err != nil {
			return nil, fmt.Errorf("error getting reservedCPUSet from PerformanceProfile: %w", err)
		}
		if !found {
			return nil, fmt.Errorf("expected spec.reserved to be found in PerformanceProfile(%s)", profile.GetName())
		}

		parsedReservedCPUSet, err := cpuset.Parse(reservedCPUSet)
		if err != nil {
			return nil, fmt.Errorf("error parsing reserved cpuset: %w", err)
		}

		// Single Node may include both worker and master labels for node selection,
		// we match on Master first to account for that, since Master get's preference when both
		// labels are present.
		if _, ok := selector[nodeMasterLabel]; ok {
			nodeRoleCPUSet.Master = parsedReservedCPUSet
		} else if _, ok := selector[nodeWorkerLabel]; ok {
			nodeRoleCPUSet.Worker = parsedReservedCPUSet
		}
	}

	return nodeRoleCPUSet, nil
}

// Execute collection script on machine-config-daemon of every node. Marshal the results to get a list of containers
// running in the cluster.
func collectContainerInfo(ctx context.Context, cfg *rest.Config, c kubernetes.Interface, node corev1.Node) ([]crioContainerData, error) {

	listOptions := metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node.Name}).String(),
		LabelSelector: labels.SelectorFromSet(labels.Set{"k8s-app": "machine-config-daemon"}).String(),
	}

	pods, err := c.CoreV1().Pods(namespaceMachineConfigOperator).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	if len(pods.Items) < 1 {
		return nil, fmt.Errorf("failed to get machine-config-daemon pod for the node %q", node.Name)
	}

	pod := &pods.Items[0]

	var out []byte
	getInfo := []string{
		"chroot",
		"/rootfs",
		"/bin/bash",
		"-c", collectionScript}

	if err := wait.PollWithContext(ctx, 15*time.Second, time.Minute, func(context.Context) (done bool, err error) {
		out, err = execCommandOnPod(cfg, c, pod, getInfo)
		if err != nil {
			return false, err
		}

		return len(out) != 0, nil
	}); err != nil {
		return nil, err
	}

	info := []crioContainerData{}
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("error parsing container output to json: %w", err)
	}

	return info, nil
}

func execCommandOnPod(cfg *rest.Config, c kubernetes.Interface, pod *corev1.Pod, command []string) ([]byte, error) {
	var outputBuf bytes.Buffer
	var errorBuf bytes.Buffer

	req := c.CoreV1().RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: pod.Spec.Containers[0].Name,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return nil, err
	}

	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: &outputBuf,
		Stderr: &errorBuf,
		Tty:    false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run command %v: output %q; error %q; %w", command, outputBuf.String(), errorBuf.String(), err)
	}

	return outputBuf.Bytes(), nil
}