package util

import (
	"strings"
	"time"

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
