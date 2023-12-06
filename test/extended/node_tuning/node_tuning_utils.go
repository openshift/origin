package node_tuning

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const masterNodeRoleLabel = "node-role.kubernetes.io/master"

// isSNOCluster will check if OCP is a single node cluster
func isSNOCluster(oc *exutil.CLI) (bool, error) {
	infrastructureType, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	e2e.Logf("the cluster type is %v", infrastructureType.Status.ControlPlaneTopology)
	return infrastructureType.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode, nil
}

func getFirstMasterNode(ctx context.Context, nodeClient v1.NodeInterface) (*corev1.Node, error) {
	masterNodes, err := nodeClient.List(ctx, metav1.ListOptions{LabelSelector: masterNodeRoleLabel})
	if err != nil || len(masterNodes.Items) == 0 {
		e2e.Logf("failed to list master nodes %v", err)
		return nil, err
	}
	return &masterNodes.Items[0], err
}

func podLogsMatch(podName string, podLogs string, filter string) (bool, error) {
	regNTOPodLogs, err := regexp.Compile(".*" + filter + ".*")
	if err != nil {
		return false, err
	}
	isMatch := regNTOPodLogs.MatchString(podLogs)
	if isMatch {
		loglines := regNTOPodLogs.FindAllString(podLogs, -1)
		e2e.Logf("the [%v] matched in the logs of pod %v, full log is [%v]", filter, podName, loglines[0])
		return true, nil
	}
	e2e.Logf("the keywords [%s] of pod isn't found ...", filter)
	return false, nil
}

func getPodLogsLastLines(ctx context.Context, c clientset.Interface, namespace, podName, containerName string, lastlines int) (string, error) {
	return getPodLogsInternal(ctx, c, namespace, podName, containerName, false, nil, &lastlines)
}

// utility function for gomega Eventually
func getPodLogsInternal(ctx context.Context, c clientset.Interface, namespace, podName, containerName string, previous bool, sinceTime *metav1.Time, tailLines *int) (string, error) {
	request := c.CoreV1().RESTClient().Get().
		Resource("pods").
		Namespace(namespace).
		Name(podName).SubResource("log").
		Param("container", containerName).
		Param("previous", strconv.FormatBool(previous))
	if sinceTime != nil {
		request.Param("sinceTime", sinceTime.Format(time.RFC3339))
	}
	if tailLines != nil {
		request.Param("tailLines", strconv.Itoa(*tailLines))
	}
	logs, err := request.Do(ctx).Raw()
	if err != nil {
		return "", err
	}
	if strings.Contains(string(logs), "Internal Error") {
		return "", fmt.Errorf("fetched log contains \"Internal Error\": %q", string(logs))
	}
	return string(logs), err
}

func isPoolUpdated(dc dynamic.NamespaceableResourceInterface, name string) (poolUpToDate bool, poolIsUpdating bool) {
	pool, err := dc.Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		e2e.Logf("error getting pool %s: %v", name, err)
		return false, false
	}
	paused, found, err := unstructured.NestedBool(pool.Object, "spec", "paused")
	if err != nil || !found {
		return false, false
	}
	conditions, found, err := unstructured.NestedFieldNoCopy(pool.Object, "status", "conditions")
	if err != nil || !found {
		return false, false
	}
	original, ok := conditions.([]interface{})
	if !ok {
		return false, false
	}
	var updated, updating, degraded bool
	for _, obj := range original {
		o, ok := obj.(map[string]interface{})
		if !ok {
			return false, false
		}
		t, found, err := unstructured.NestedString(o, "type")
		if err != nil || !found {
			return false, false
		}
		s, found, err := unstructured.NestedString(o, "status")
		if err != nil || !found {
			return false, false
		}
		if t == "Updated" && s == "True" {
			updated = true
		}
		if t == "Updating" && s == "True" {
			updating = true
		}
		if t == "Degraded" && s == "True" {
			degraded = true
		}
	}
	if paused {
		e2e.Logf("pool %s is paused, treating as up-to-date (updated: %v, updating: %v, degraded: %v)", name, updated, updating, degraded)
		return true, updating
	}
	if updated && !updating && !degraded {
		return true, updating
	}
	e2e.Logf("pool %s is still reporting (updated: %v, updating: %v, degraded: %v)", name, updated, updating, degraded)
	return false, updating
}

func findCondition(conditions []configv1.ClusterOperatorStatusCondition, name configv1.ClusterStatusConditionType) *configv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if name == conditions[i].Type {
			return &conditions[i]
		}
	}
	return nil
}

// Check if the co status is available every 60 seconds, for a maximum of 5 tries.
// just in case the co status become degraded or processing during ocp upgrade scenario or node reboot/scale out.
func waitForClusterOperatorAvailable(oc *exutil.CLI, coName string) error {
	return wait.Poll(1*time.Minute, 5*time.Minute, func() (bool, error) {
		isCOAvailable, err := isCOAvailableState(oc, coName)
		if err != nil {
			return false, err
		}
		if isCOAvailable {
			e2e.Logf("the status of cluster operator %v keep on available state", coName)
			return true, nil
		}
		e2e.Logf("the status of co %v doesn't stay on expected state, will check again", coName)
		return false, nil
	})
}

// isCOAvailable returns true when the ClusterOperator's coName status conditions are as follows: Available true, Progressing false and Degraded false.
func isCOAvailableState(oc *exutil.CLI, coName string) (bool, error) {
	var (
		clusterOperators []configv1.ClusterOperator
		co               configv1.ClusterOperator
		isAvailable      bool
	)
	clusterOperatorsList, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		e2e.Logf("fail to get cluster operator list with error")
		return false, err
	}
	clusterOperators = clusterOperatorsList.Items
	for _, clusterOperator := range clusterOperators {
		if clusterOperator.Name == coName {
			co = clusterOperator
			e2e.Logf("co name is %v", co.Name)
			e2e.Logf("co.status.conditions of %v is %v", co.Name, co.Status.Conditions)
			break
		}
	}
	available := findCondition(co.Status.Conditions, configv1.OperatorAvailable)
	degraded := findCondition(co.Status.Conditions, configv1.OperatorDegraded)
	progressing := findCondition(co.Status.Conditions, configv1.OperatorProgressing)
	e2e.Logf("the status of co %v is available.Status [%v] degraded.Status [%v] and progressing.Status [%v]", coName, available.Status, degraded.Status, progressing.Status)
	if available.Status == configv1.ConditionTrue &&
		degraded.Status == configv1.ConditionFalse &&
		progressing.Status == configv1.ConditionFalse {
		isAvailable = true
	}
	e2e.Logf("co/%v status is %v", coName, co.Status)
	return isAvailable, nil
}

// Check the updated state of mcp master/worker every 60 seconds, for a maximum of 10 tries.
// just in case the mcp state become updating during ocp upgrade or node reboot/scale out or other test case change mcp
func waitForUpdatedMCP(mcps dynamic.NamespaceableResourceInterface, mcpName string) error {
	return wait.Poll(1*time.Minute, 10*time.Minute, func() (bool, error) {
		updated, updating := isPoolUpdated(mcps, mcpName)
		if updated && !updating {
			e2e.Logf("the status of mcp %v is updated state, not updating", mcpName)
			return true, nil
		}
		e2e.Logf("the status of mcp %v is updating or degraded state, will check again")
		e2e.Logf("the status of mcp is updated - [%v] updating - [%v]", updated, updating)
		return false, nil
	})
}
