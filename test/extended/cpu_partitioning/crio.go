package cpu_partitioning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	ocpv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/cpuset"
)

// This collection script queries CRI-O for all it's containers, that data is then filtered down to only a subset
// of information needed for validating workload partitioning and the PIDs are mapped to their corresponding host
// CPUSet via the taskset command.
const collectionScript = `
#!/bin/bash

crictl_version=$(crictl --version)
jq_flags=""

# TODO: Remove after successful nightly to 1.31 crictl
if [[ $crictl_version =~ "1.30" ]]; then
	jq_flags="-rs"
fi

container_ids=$(sudo crictl ps -q)
container_data=$(sudo crictl inspect $container_ids)

# Filter down CRIO data to relevant information only
workload_containers=$(echo $container_data | jq $jq_flags '[
	.[] | select(.info.runtimeSpec.annotations["target.workload.openshift.io/management"]) | 
	{
		cpuSet: (.info.runtimeSpec.linux.resources.cpu.cpus // ""),
        cpuShares: .info.runtimeSpec.linux.resources.cpu.shares,
        cpuQuota: .info.runtimeSpec.linux.resources.cpu.quota,
        cpuPeriod: .info.runtimeSpec.linux.resources.cpu.period,
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

// Collection data for each container in a node
type crioContainerData struct {
	PodNamespace string            `json:"podNamespace"`
	PodName      string            `json:"podName"`
	Name         string            `json:"name"`
	CPUSet       string            `json:"cpuSet"`
	CPUShares    int64             `json:"cpuShares"`
	CPUQuota     int64             `json:"cpuQuota"`
	CPUPeriod    int64             `json:"cpuPeriod"`
	Annotations  map[string]string `json:"annotations"`
	HostCPUSet   string            `json:"hostCPUSet"`
	Hostname     string            `json:"hostname"`
	Pid          int               `json:"pid"`
}

// Parse the workload cpu resource annotation
type crioCPUResource struct {
	// Specifies the number of CPU shares this Pod has access to.
	CPUShares int64 `json:"cpushares,omitempty"`
	// Specifies the CPU limit in millicores. This will be used to calculate the CPU quota.
	CPULimit int64 `json:"cpulimit,omitempty"`
}

// Container struct for master and worker node CPUSets
type nodeCPUSets struct {
	Master  cpuset.CPUSet
	Arbiter cpuset.CPUSet
	Worker  cpuset.CPUSet
}

// getAnnotationCPUShare Looks through the annotations to find the workload partitioning annotation and
// returns the crio cpu resources or error if no such annotation exists.
func (c *crioContainerData) getAnnotationCPUResources() (crioCPUResource, error) {
	annotationCPUResource := crioCPUResource{}
	anno := fmt.Sprintf("%s/%s", workloadAnnotationPrefix, c.Name)
	cpuResources, ok := c.Annotations[anno]
	if !ok {
		return annotationCPUResource, fmt.Errorf("workload annotation of [%s] was expected but not found", anno)
	}

	// workload annotations have a json string that will contain the cpushares and cpulimit
	// these values are passed down from kubernetes, we parse and return that value here to validate.
	err := json.Unmarshal([]byte(cpuResources), &annotationCPUResource)
	if err != nil {
		return annotationCPUResource, fmt.Errorf("err parsing cpushare json annotation: %w", err)
	}

	return annotationCPUResource, nil
}

var _ = g.Describe("[sig-node][apigroup:config.openshift.io] CPU Partitioning node validation", func() {

	var (
		oc                      = exutil.NewCLIWithoutNamespace("cpu-partitioning").AsAdmin()
		managedNamespace        = exutil.NewCLI("managed-namespace").SetManagedNamespace().AsAdmin()
		ctx                     = context.Background()
		isClusterCPUPartitioned = false
	)

	g.BeforeEach(func() {
		isClusterCPUPartitioned = getCpuPartitionedStatus(oc) == ocpv1.CPUPartitioningAllNodes
	})

	g.AfterEach(func() {
		o.Expect(cleanup(managedNamespace, managedNamespace.Namespace())).To(o.Succeed())
	})

	g.It("should have correct cpuset and cpushare set in crio containers", g.Label("Size:L"), func() {

		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// HyperShift clusters do not run the PerformanceProfile controller nor do they support
		// MachineConfigs, Workload partitioning feature uses those components to configure CPUSets.
		if *controlPlaneTopology == ocpv1.ExternalTopologyMode {
			g.Skip("Clusters with external control plane topology do not run PerformanceProfile Controller")
		}

		// Create deployment with limits to validate cpu limits are respected
		requests := corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("20m"),
			corev1.ResourceMemory: resource.MustParse("100Mi"),
		}
		limits := corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("30m"),
			corev1.ResourceMemory: resource.MustParse("100Mi"),
		}
		_, err = createManagedDeployment(managedNamespace, requests, limits)
		o.Expect(err).ToNot(o.HaveOccurred(), "error creating deployment with cpu limits")

		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred(), "error listing cluster nodes")

		nodeRoleCPUSet, err := getExpectedCPUSetConstraints(ctx, oc, isClusterCPUPartitioned)
		o.Expect(err).ToNot(o.HaveOccurred(), "error getting node CPUSets")

		for _, node := range nodes.Items {
			// Collect the container data from the node
			crioData, err := collectContainerInfo(ctx, oc, node, isClusterCPUPartitioned)
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
			} else if _, isArbiter := node.Labels[nodeArbiterLabel]; isArbiter {
				expectedCPUSet = nodeRoleCPUSet.Arbiter
			} else if _, isWorker := node.Labels[nodeWorkerLabel]; isWorker {
				expectedCPUSet = nodeRoleCPUSet.Worker
			}

			for _, containerInfo := range crioData {
				failLocation := fmt.Sprintf("node: %s pod: %s/%s container: %s", node.Name, containerInfo.PodNamespace, containerInfo.PodName, containerInfo.Name)
				parsedContainer, err := cpuset.Parse(containerInfo.CPUSet)
				o.Expect(err).ToNot(o.HaveOccurred(), fmt.Sprintf("%s | error parsing crio container cpuset %s", failLocation, containerInfo.CPUSet))
				parsedHost, err := cpuset.Parse(containerInfo.HostCPUSet)
				o.Expect(err).ToNot(o.HaveOccurred(), fmt.Sprintf("%s | error parsing host cpuset %s", failLocation, containerInfo.HostCPUSet))

				o.Expect(parsedContainer.Equals(expectedCPUSet)).To(o.BeTrue(), "cpusets do not match between container and desired")

				// Empty CPUSets mean wide open, so we check the processes are not being limited.
				// If the expected CPUSet is not empty, we make sure the host is respecting it.
				if expectedCPUSet.IsEmpty() {
					o.Expect(parsedHost.Equals(parsedFullNodeCPUSet)).To(o.BeTrue(), fmt.Sprintf("%s | expected container pid CPUset to be: %s but got: %s", failLocation, parsedFullNodeCPUSet, parsedHost))
				} else {
					o.Expect(parsedContainer.Equals(parsedHost)).To(o.BeTrue(), fmt.Sprintf("%s | expected container pid CPUset to be: %s got: %s", failLocation, parsedHost, parsedContainer))
				}

				// If we are in a CPU Partitioned cluster, containers MUST be annotated with the correct CPU Share at the CRIO level
				// and the desired annotation cpu shares must equal the crio config cpu shares
				if isClusterCPUPartitioned {
					resource, err := containerInfo.getAnnotationCPUResources()
					o.Expect(err).ToNot(o.HaveOccurred(), fmt.Sprintf("%s | failed to get container resource annotation json", failLocation), err)
					o.Expect(resource.CPUShares).To(o.Equal(containerInfo.CPUShares), fmt.Sprintf("%s | cpushares do not match between crio config and desired", failLocation))

					desiredCPUQuota := milliCPUToQuota(resource.CPULimit, containerInfo.CPUPeriod)
					o.Expect(desiredCPUQuota).To(o.Equal(containerInfo.CPUQuota), fmt.Sprintf("%s | cpuquota do not match between crio config and desired", failLocation))
				}
			}
		}
	})
})

func getExpectedCPUSetConstraints(ctx context.Context, oc *exutil.CLI, isClusterPartitioned bool) (*nodeCPUSets, error) {
	// Default behavior is that CPUSets are empty, which means wide open to the host.
	// There should be no CPUSet constraint if no PerformanceProfile is present.
	nodeRoleCPUSet := &nodeCPUSets{
		Master:  cpuset.New(),
		Arbiter: cpuset.New(),
		Worker:  cpuset.New(),
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
		} else if _, ok := selector[nodeArbiterLabel]; ok {
			nodeRoleCPUSet.Arbiter = parsedReservedCPUSet
		} else if _, ok := selector[nodeWorkerLabel]; ok {
			nodeRoleCPUSet.Worker = parsedReservedCPUSet
		}
	}

	return nodeRoleCPUSet, nil
}

// Execute collection script on machine-config-daemon of every node. Marshal the results to get a list of containers
// running in the cluster.
func collectContainerInfo(ctx context.Context, oc *exutil.CLI, node corev1.Node, isClusterCPUPartitioned bool) ([]crioContainerData, error) {

	listOptions := metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node.Name}).String(),
		LabelSelector: labels.SelectorFromSet(labels.Set{"k8s-app": "machine-config-daemon"}).String(),
	}

	pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods(namespaceMachineConfigOperator).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	if len(pods.Items) < 1 {
		return nil, fmt.Errorf("failed to get machine-config-daemon pod for the node %q", node.Name)
	}

	pod := &pods.Items[0]
	podName := pod.Name
	podNamespace := pod.Namespace
	containerName := pod.Spec.Containers[0].Name

	getInfo := []string{
		"chroot",
		"/rootfs",
		"/bin/bash",
		"-c", collectionScript}

	backoff := wait.Backoff{
		Duration: 2 * time.Second,
		Steps:    5,
		Factor:   2.0,
		Jitter:   0.1,
	}

	execOptions := e2epod.ExecOptions{
		Command:            getInfo,
		PodName:            podName,
		Namespace:          podNamespace,
		ContainerName:      containerName,
		Stdin:              nil,
		CaptureStdout:      true,
		CaptureStderr:      true,
		PreserveWhitespace: false,
	}

	info := []crioContainerData{}
	err = retry.OnError(backoff, shouldRetryExec,
		func() error {
			out, outStdErr, execErr := e2epod.ExecWithOptions(oc.KubeFramework(), execOptions)
			if execErr != nil {
				return execErr
			}
			if outStdErr != "" {
				return fmt.Errorf("err execing command %s", outStdErr)
			}
			if isClusterCPUPartitioned && len(out) == 0 {
				return fmt.Errorf("err execing command, script result was empty but expected values")
			}
			if err := json.Unmarshal([]byte(out), &info); err != nil {
				return fmt.Errorf("error parsing container output to json: %w", err)
			}
			return nil
		})

	return info, err
}

// Due to cluster activity, sometimes execing against a pod can be flakey.
// We retry if the below errors are received during node queries.
func shouldRetryExec(err error) bool {
	var parseErr *json.SyntaxError
	substringErrors := []string{
		"error dialing backend: dial tcp", // Some sort of disruption is happening causing the exec to time out, retry.
		"container not found",             // On occasion there might be situations where the machine-config-daemon isn't running yet.
		"err execing command",             // Error when running the command in the container
	}
	switch {
	// This typically means that something interrupted the stream of json coming back, so we need to do the query again.
	case errors.As(err, &parseErr):
		return true
	case substringSliceContains(substringErrors, err.Error()):
		return true
	default:
		return false
	}
}

func substringSliceContains(slice []string, s string) bool {
	for _, substring := range slice {
		if strings.Contains(s, substring) {
			return true
		}
	}
	return false
}
