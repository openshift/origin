package compat_otp

import exutil "github.com/openshift/origin/test/extended/util"

import (
	"context"
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// GetFirstLinuxWorkerNode returns the first linux worker node in the cluster
func GetFirstLinuxWorkerNode(oc *exutil.CLI) (string, error) {
	var (
		workerNode string
		err        error
	)
	workerNode, err = getFirstNodeByOsID(oc, "worker", "rhcos")
	if len(workerNode) == 0 {
		workerNode, err = getFirstNodeByOsID(oc, "worker", "rhel")
	}
	return workerNode, err
}

// GetAllNodesbyOSType returns a list of the names of all linux/windows nodes in the cluster have both linux and windows node
func GetAllNodesbyOSType(oc *exutil.CLI, ostype string) ([]string, error) {
	var nodesArray []string
	nodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "kubernetes.io/os="+ostype, "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	nodesStr := strings.Trim(nodes, "'")
	//If split an empty string to string array, the default length string array is 1
	//So need to check if string is empty.
	if len(nodesStr) == 0 {
		return nodesArray, err
	}
	nodesArray = strings.Split(nodesStr, " ")
	return nodesArray, err
}

// GetAllNodes returns a list of the names of all nodes in the cluster
func GetAllNodes(oc *exutil.CLI) ([]string, error) {
	nodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	return strings.Split(strings.Trim(nodes, "'"), " "), err
}

// GetFirstWorkerNode returns a first worker node
func GetFirstWorkerNode(oc *exutil.CLI) (string, error) {
	workerNodes, err := GetClusterNodesBy(oc, "worker")
	return workerNodes[0], err
}

// GetFirstMasterNode returns a first master node
func GetFirstMasterNode(oc *exutil.CLI) (string, error) {
	masterNodes, err := GetClusterNodesBy(oc, "master")
	return masterNodes[0], err
}

// GetClusterNodesBy returns the cluster nodes by role
func GetClusterNodesBy(oc *exutil.CLI, role string) ([]string, error) {
	nodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "node-role.kubernetes.io/"+role, "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	return strings.Split(strings.Trim(nodes, "'"), " "), err
}

// DebugNodeWithChroot creates a debugging session of the node with chroot
func DebugNodeWithChroot(oc *exutil.CLI, nodeName string, cmd ...string) (string, error) {
	stdOut, stdErr, err := debugNode(oc, nodeName, []string{}, true, true, cmd...)
	return strings.Join([]string{stdOut, stdErr}, "\n"), err
}

// DebugNodeWithOptions launches debug container with options e.g. --image
func DebugNodeWithOptions(oc *exutil.CLI, nodeName string, options []string, cmd ...string) (string, error) {
	stdOut, stdErr, err := debugNode(oc, nodeName, options, false, true, cmd...)
	return strings.Join([]string{stdOut, stdErr}, "\n"), err
}

// DebugNodeWithOptionsAndChroot launches debug container using chroot and with options e.g. --image
func DebugNodeWithOptionsAndChroot(oc *exutil.CLI, nodeName string, options []string, cmd ...string) (string, error) {
	stdOut, stdErr, err := debugNode(oc, nodeName, options, true, true, cmd...)
	return strings.Join([]string{stdOut, stdErr}, "\n"), err
}

// DebugNodeRetryWithOptionsAndChroot launches debug container using chroot and with options
// And waitPoll to avoid "error: unable to create the debug pod" and do retry
func DebugNodeRetryWithOptionsAndChroot(oc *exutil.CLI, nodeName string, options []string, cmd ...string) (string, error) {
	var stdErr string
	var stdOut string
	var err error
	errWait := wait.Poll(3*time.Second, 30*time.Second, func() (bool, error) {
		stdOut, stdErr, err = debugNode(oc, nodeName, options, true, true, cmd...)
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	AssertWaitPollNoErr(errWait, fmt.Sprintf("Failed to debug node : %v", errWait))
	return strings.Join([]string{stdOut, stdErr}, "\n"), err
}

// DebugNodeWithOptionsAndChrootWithoutRecoverNsLabel launches debug container using chroot and with options e.g. --image
// WithoutRecoverNsLabel which will not recover the labels that added for debug node container adapt the podSecurity changed on 4.12+ test clusters
// "security.openshift.io/scc.podSecurityLabelSync=false" And "pod-security.kubernetes.io/enforce=privileged"
func DebugNodeWithOptionsAndChrootWithoutRecoverNsLabel(oc *exutil.CLI, nodeName string, options []string, cmd ...string) (stdOut string, stdErr string, err error) {
	return debugNode(oc, nodeName, options, true, false, cmd...)
}

// DebugNode creates a debugging session of the node
func DebugNode(oc *exutil.CLI, nodeName string, cmd ...string) (string, error) {
	stdOut, stdErr, err := debugNode(oc, nodeName, []string{}, false, true, cmd...)
	return strings.Join([]string{stdOut, stdErr}, "\n"), err
}

func debugNode(oc *exutil.CLI, nodeName string, cmdOptions []string, needChroot bool, recoverNsLabels bool, cmd ...string) (stdOut string, stdErr string, err error) {
	var (
		debugNodeNamespace string
		isNsPrivileged     bool
		cargs              []string
		outputError        error
	)
	cargs = []string{"node/" + nodeName}
	// Enhance for debug node namespace used logic
	// if "--to-namespace=" option is used, then uses the input options' namespace, otherwise use oc.Namespace()
	// if oc.Namespace() is empty, uses "default" namespace instead
	hasToNamespaceInCmdOptions, index := StringsSliceElementsHasPrefix(cmdOptions, "--to-namespace=", false)
	if hasToNamespaceInCmdOptions {
		debugNodeNamespace = strings.TrimPrefix(cmdOptions[index], "--to-namespace=")
	} else {
		debugNodeNamespace = oc.Namespace()
		if debugNodeNamespace == "" {
			debugNodeNamespace = "default"
		}
	}
	// Running oc debug node command in normal projects
	// (normal projects mean projects that are not clusters default projects like: "openshift-xxx" et al)
	// need extra configuration on 4.12+ ocp test clusters
	// https://github.com/openshift/oc/blob/master/pkg/helpers/cmd/errors.go#L24-L29
	if !strings.HasPrefix(debugNodeNamespace, "openshift-") {
		isNsPrivileged, outputError = IsNamespacePrivileged(oc, debugNodeNamespace)
		if outputError != nil {
			return "", "", outputError
		}
		if !isNsPrivileged {
			if recoverNsLabels {
				defer RecoverNamespaceRestricted(oc, debugNodeNamespace)
			}
			outputError = SetNamespacePrivileged(oc, debugNodeNamespace)
			if outputError != nil {
				return "", "", outputError
			}
		}
	}

	// For default nodeSelector enabled test clusters we need to add the extra annotation to avoid the debug pod's
	// nodeSelector overwritten by the scheduler
	if IsDefaultNodeSelectorEnabled(oc) && !IsWorkerNode(oc, nodeName) && !IsSpecifiedAnnotationKeyExist(oc, "ns/"+debugNodeNamespace, "", `openshift.io/node-selector`) {
		AddAnnotationsToSpecificResource(oc, "ns/"+debugNodeNamespace, "", `openshift.io/node-selector=`)
		defer RemoveAnnotationFromSpecificResource(oc, "ns/"+debugNodeNamespace, "", `openshift.io/node-selector`)
	}

	if len(cmdOptions) > 0 {
		cargs = append(cargs, cmdOptions...)
	}
	if !hasToNamespaceInCmdOptions {
		cargs = append(cargs, "--to-namespace="+debugNodeNamespace)
	}
	if needChroot {
		cargs = append(cargs, "--", "chroot", "/host")
	} else {
		cargs = append(cargs, "--")
	}
	cargs = append(cargs, cmd...)
	return determineExecCLI(oc).WithoutNamespace().Run("debug").Args(cargs...).Outputs()
}

// DeleteLabelFromNode delete the custom label from the node
func DeleteLabelFromNode(oc *exutil.CLI, node string, label string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("label").Args("node", node, label+"-").Output()
}

// AddLabelToNode add the custom label to the node
func AddLabelToNode(oc *exutil.CLI, node string, label string, value string) (string, error) {
	return oc.AsAdmin().WithoutNamespace().Run("label").Args("node", node, label+"="+value).Output()
}

// GetFirstCoreOsWorkerNode returns the first CoreOS worker node
func GetFirstCoreOsWorkerNode(oc *exutil.CLI) (string, error) {
	return getFirstNodeByOsID(oc, "worker", "rhcos")
}

// GetFirstRhelWorkerNode returns the first rhel worker node
func GetFirstRhelWorkerNode(oc *exutil.CLI) (string, error) {
	return getFirstNodeByOsID(oc, "worker", "rhel")
}

// getFirstNodeByOsID returns the cluster node by role and os id
func getFirstNodeByOsID(oc *exutil.CLI, role string, osID string) (string, error) {
	nodes, err := GetClusterNodesBy(oc, role)
	for _, node := range nodes {
		stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node/"+node, "-o", "jsonpath=\"{.metadata.labels.node\\.openshift\\.io/os_id}\"").Output()
		if strings.Trim(stdout, "\"") == osID {
			return node, err
		}
	}
	return "", err
}

// GetNodeHostname returns the cluster node hostname
func GetNodeHostname(oc *exutil.CLI, node string) (string, error) {
	hostname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", node, "-o", "jsonpath='{..kubernetes\\.io/hostname}'").Output()
	return strings.Trim(hostname, "'"), err
}

// GetClusterNodesByRoleInHostedCluster returns the cluster nodes by role
func GetClusterNodesByRoleInHostedCluster(oc *exutil.CLI, role string) ([]string, error) {
	nodes, err := oc.AsAdmin().AsGuestKubeconf().Run("get").Args("node", "-l", "node-role.kubernetes.io/"+role, "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	return strings.Split(strings.Trim(nodes, "'"), " "), err
}

// getFirstNodeByOsIDInHostedCluster returns the cluster node by role and os id
func getFirstNodeByOsIDInHostedCluster(oc *exutil.CLI, role string, osID string) (string, error) {
	nodes, err := GetClusterNodesByRoleInHostedCluster(oc, role)
	for _, node := range nodes {
		stdout, err := oc.AsAdmin().AsGuestKubeconf().Run("get").Args("node/"+node, "-o", "jsonpath=\"{.metadata.labels.node\\.openshift\\.io/os_id}\"").Output()
		if strings.Trim(stdout, "\"") == osID {
			return node, err
		}
	}
	return "", err
}

// GetFirstLinuxWorkerNodeInHostedCluster returns the first linux worker node in the cluster
func GetFirstLinuxWorkerNodeInHostedCluster(oc *exutil.CLI) (string, error) {
	var (
		workerNode string
		err        error
	)
	workerNode, err = getFirstNodeByOsIDInHostedCluster(oc, "worker", "rhcos")
	if len(workerNode) == 0 {
		workerNode, err = getFirstNodeByOsIDInHostedCluster(oc, "worker", "rhel")
	}
	return workerNode, err
}

// GetAllNodesByNodePoolNameInHostedCluster return all node names of specified nodepool in hosted cluster.
func GetAllNodesByNodePoolNameInHostedCluster(oc *exutil.CLI, nodePoolName string) ([]string, error) {
	nodes, err := oc.AsAdmin().AsGuestKubeconf().Run("get").Args("node", "-l", "hypershift.openshift.io/nodePool="+nodePoolName, "-ojsonpath='{.items[*].metadata.name}'").Output()
	return strings.Split(strings.Trim(nodes, "'"), " "), err
}

// GetFirstWorkerNodeByNodePoolNameInHostedCluster returns the first linux worker node in the cluster
func GetFirstWorkerNodeByNodePoolNameInHostedCluster(oc *exutil.CLI, nodePoolName string) (string, error) {
	workerNodes, err := GetAllNodesByNodePoolNameInHostedCluster(oc, nodePoolName)
	o.Expect(err).NotTo(o.HaveOccurred())
	return workerNodes[0], err
}

// GetSchedulableLinuxWorkerNodes returns a group of nodes that match the requirements:
// os: linux, role: worker, status: ready, schedulable
func GetSchedulableLinuxWorkerNodes(oc *exutil.CLI) ([]v1.Node, error) {
	var nodes, workers []v1.Node
	linuxNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: "kubernetes.io/os=linux"})
	// get schedulable linux worker nodes
	for _, node := range linuxNodes.Items {
		if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok && !node.Spec.Unschedulable {
			workers = append(workers, node)
		}
	}
	// get ready nodes
	for _, worker := range workers {
		for _, con := range worker.Status.Conditions {
			if con.Type == "Ready" && con.Status == "True" {
				nodes = append(nodes, worker)
				break
			}
		}
	}
	return nodes, err
}

// GetPodsNodesMap returns all the running pods in each node
func GetPodsNodesMap(oc *exutil.CLI, nodes []v1.Node) map[string][]v1.Pod {
	podsMap := make(map[string][]v1.Pod)
	projects, err := oc.AdminKubeClient().CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// get pod list in each node
	for _, project := range projects.Items {
		pods, err := oc.AdminKubeClient().CoreV1().Pods(project.Name).List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, pod := range pods.Items {
			if pod.Status.Phase != "Failed" && pod.Status.Phase != "Succeeded" {
				podsMap[pod.Spec.NodeName] = append(podsMap[pod.Spec.NodeName], pod)
			}
		}
	}

	var nodeNames []string
	for _, node := range nodes {
		nodeNames = append(nodeNames, node.Name)
	}
	contain := func(a []string, b string) bool {
		for _, c := range a {
			if c == b {
				return true
			}
		}
		return false
	}
	// if the key is not in nodes list, remove the element from the map
	for podmap := range podsMap {
		if !contain(nodeNames, podmap) {
			delete(podsMap, podmap)
		}
	}
	return podsMap
}

// NodeResources contains the resources of CPU and Memory in a node
type NodeResources struct {
	CPU    int64
	Memory int64
}

// GetRequestedResourcesNodesMap returns the total requested CPU and Memory in each node
func GetRequestedResourcesNodesMap(oc *exutil.CLI, nodes []v1.Node) map[string]NodeResources {
	rmap := make(map[string]NodeResources)
	podsMap := GetPodsNodesMap(oc, nodes)
	for nodeName := range podsMap {
		var totalRequestedCPU, totalRequestedMemory int64
		for _, pod := range podsMap[nodeName] {
			for _, container := range pod.Spec.Containers {
				totalRequestedCPU += container.Resources.Requests.Cpu().MilliValue()
				totalRequestedMemory += container.Resources.Requests.Memory().MilliValue()
			}
		}
		rmap[nodeName] = NodeResources{totalRequestedCPU, totalRequestedMemory}
	}
	return rmap
}

// GetAllocatableResourcesNodesMap returns the total allocatable CPU and Memory in each node
func GetAllocatableResourcesNodesMap(nodes []v1.Node) map[string]NodeResources {
	rmap := make(map[string]NodeResources)
	for _, node := range nodes {
		rmap[node.Name] = NodeResources{node.Status.Allocatable.Cpu().MilliValue(), node.Status.Allocatable.Memory().MilliValue()}
	}
	return rmap
}

// GetRemainingResourcesNodesMap returns the total remaning CPU and Memory in each node
func GetRemainingResourcesNodesMap(oc *exutil.CLI, nodes []v1.Node) map[string]NodeResources {
	rmap := make(map[string]NodeResources)
	requested := GetRequestedResourcesNodesMap(oc, nodes)
	allocatable := GetAllocatableResourcesNodesMap(nodes)

	for _, node := range nodes {
		rmap[node.Name] = NodeResources{allocatable[node.Name].CPU - requested[node.Name].CPU, allocatable[node.Name].Memory - requested[node.Name].Memory}
	}
	return rmap
}

// getNodesByRoleAndOsID returns list of nodes by role and OS ID
func getNodesByRoleAndOsID(oc *exutil.CLI, role string, osID string) ([]string, error) {
	var nodesList []string
	nodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "node-role.kubernetes.io/"+role+",node.openshift.io/os_id="+osID, "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	nodes = strings.Trim(nodes, "'")
	if len(nodes) != 0 {
		nodesList = strings.Split(nodes, " ")
	}
	return nodesList, err
}

// GetAllWorkerNodesByOSID returns list of worker nodes by OS ID
func GetAllWorkerNodesByOSID(oc *exutil.CLI, osID string) ([]string, error) {
	return getNodesByRoleAndOsID(oc, "worker", osID)
}

// GetNodeArchByName gets the node arch by its name
func GetNodeArchByName(oc *exutil.CLI, nodeName string) string {
	nodeArch, err := GetResourceSpecificLabelValue(oc, "node/"+nodeName, "", "kubernetes\\.io/arch")
	o.Expect(err).NotTo(o.HaveOccurred(), "Fail to get node/%s arch: %v\n", nodeName, err)
	e2e.Logf(`The node/%s arch is "%s"`, nodeName, nodeArch)
	return nodeArch
}

// GetNodeListByLabel gets the node list by label
func GetNodeListByLabel(oc *exutil.CLI, labelKey string) []string {
	output, err := determineExecCLI(oc).WithoutNamespace().Run("get").Args("node", "-l", labelKey, "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "Fail to get node with label %v, got error: %v\n", labelKey, err)
	nodeNameList := strings.Fields(output)
	return nodeNameList
}

// IsDefaultNodeSelectorEnabled judges whether the test cluster enabled the defaultNodeSelector
func IsDefaultNodeSelectorEnabled(oc *exutil.CLI) bool {
	defaultNodeSelector, getNodeSelectorErr := determineExecCLI(oc).WithoutNamespace().Run("get").Args("scheduler", "cluster", "-o=jsonpath={.spec.defaultNodeSelector}").Output()
	if getNodeSelectorErr != nil && strings.Contains(defaultNodeSelector, `the server doesn't have a resource type`) {
		e2e.Logf("WARNING: The scheduler API is not supported on the test cluster")
		return false
	}
	o.Expect(getNodeSelectorErr).NotTo(o.HaveOccurred(), "Fail to get cluster scheduler defaultNodeSelector got error: %v\n", getNodeSelectorErr)
	return !strings.EqualFold(defaultNodeSelector, "")
}

// IsWorkerNode judges whether the node has the worker role
func IsWorkerNode(oc *exutil.CLI, nodeName string) bool {
	isWorker, _ := StringsSliceContains(GetNodeListByLabel(oc, `node-role.kubernetes.io/worker`), nodeName)
	return isWorker
}

func WaitForNodeToDisappear(oc *exutil.CLI, nodeName string, timeout, interval time.Duration) {
	o.Eventually(func() bool {
		_, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true
		}
		o.Expect(err).ShouldNot(o.HaveOccurred(), fmt.Sprintf("Unexpected error: %s", errors.ReasonForError(err)))
		e2e.Logf("Still waiting for node %s to disappear", nodeName)
		return false
	}).WithTimeout(timeout).WithPolling(interval).Should(o.BeTrue())
}

// DebugNodeRetryWithOptionsAndChroot launches debug container using chroot and with options
// And waitPoll to avoid "error: unable to create the debug pod" and do retry
// Separate the Warning from Output: metadata.name: this is used in the Pod's hostname, which can result in surprising behavior; a DNS label is recommended:
// [must be no more than 63 characters]\ndevice name 'Warning: metadata.name: this is used in the Pod's hostname, which can result in surprising behavior;
// a DNS label is recommended: [must be no more than 63 characters]' longer than 127 characters\nerror: non-zero exit code from debug container
func DebugNodeRetryWithOptionsAndChrootWithStdErr(oc *exutil.CLI, nodeName string, options []string, cmd ...string) (string, string, error) {
	var stdErr string
	var stdOut string
	var err error
	errWait := wait.Poll(3*time.Second, 30*time.Second, func() (bool, error) {
		stdOut, stdErr, err = debugNode(oc, nodeName, options, true, true, cmd...)
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	AssertWaitPollNoErr(errWait, fmt.Sprintf("Failed to debug node : %v", errWait))
	return stdOut, stdErr, err
}
