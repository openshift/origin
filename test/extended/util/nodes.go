package util

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// GetClusterNodesByRole returns the cluster nodes by role
func GetClusterNodesByRole(oc *CLI, role string) ([]string, error) {
	nodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "node-role.kubernetes.io/"+role, "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	return strings.Split(strings.Trim(nodes, "'"), " "), err
}

// GetFirstCoreOsWorkerNode returns the first CoreOS worker node
func GetFirstCoreOsWorkerNode(oc *CLI) (string, error) {
	return getFirstNodeByOsID(oc, "worker", "rhcos")
}

// getFirstNodeByOsID returns the cluster node by role and os id
func getFirstNodeByOsID(oc *CLI, role string, osID string) (string, error) {
	nodes, err := GetClusterNodesByRole(oc, role)
	for _, node := range nodes {
		stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node/"+node, "-o", "jsonpath=\"{.metadata.labels.node\\.openshift\\.io/os_id}\"").Output()
		if strings.Trim(stdout, "\"") == osID {
			return node, err
		}
	}
	return "", err
}

// DebugNodeRetryWithOptionsAndChroot launches debug container using chroot with options
// and waitPoll to avoid "error: unable to create the debug pod" and do retry
func DebugNodeRetryWithOptionsAndChroot(oc *CLI, nodeName string, debugNodeNamespace string, cmd ...string) (string, error) {
	var (
		cargs  []string
		stdOut string
		err    error
	)
	cargs = []string{"node/" + nodeName, "-n" + debugNodeNamespace, "--", "chroot", "/host"}
	cargs = append(cargs, cmd...)
	wait.Poll(3*time.Second, 30*time.Second, func() (bool, error) {
		stdOut, _, err = oc.AsAdmin().WithoutNamespace().Run("debug").Args(cargs...).Outputs()
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	return stdOut, err
}

func GetClusterNodesBySelector(oc *CLI, selector string) ([]corev1.Node, error) {
	nodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: selector,
		})
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

func GetAllClusterNodes(oc *CLI) ([]corev1.Node, error) {
	return GetClusterNodesBySelector(oc, "")
}

func DebugSelectedNodesRetryWithOptionsAndChroot(oc *CLI, selector string, debugNodeNamespace string, cmd ...string) (map[string]string, error) {
	nodes, err := GetClusterNodesBySelector(oc, selector)
	if err != nil {
		return nil, err
	}
	stdOut := make(map[string]string, len(nodes))
	for _, node := range nodes {
		stdOut[node.Name], err = DebugNodeRetryWithOptionsAndChroot(oc, node.Name, debugNodeNamespace, cmd...)
		if err != nil {
			return stdOut, err
		}
	}
	return stdOut, nil
}

func DebugAllNodesRetryWithOptionsAndChroot(oc *CLI, debugNodeNamespace string, cmd ...string) (map[string]string, error) {
	return DebugSelectedNodesRetryWithOptionsAndChroot(oc, "", debugNodeNamespace, cmd...)
}
