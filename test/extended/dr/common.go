package dr

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2essh "k8s.io/kubernetes/test/e2e/framework/ssh"

	"github.com/openshift/origin/test/e2e/upgrade"
	exutil "github.com/openshift/origin/test/extended/util"

	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"
)

const (
	operatorWait      = 15 * time.Minute
	defaultSSHTimeout = 5 * time.Minute
)

func runCommandAndRetry(command string) string {
	const (
		maxRetries = 10
		pause      = 10
	)
	var (
		retryCount = 0
		out        []byte
		err        error
	)
	e2elog.Logf("command '%s'", command)
	for retryCount = 0; retryCount <= maxRetries; retryCount++ {
		out, err = exec.Command("bash", "-c", command).CombinedOutput()
		e2elog.Logf("output:\n%s", out)
		if err == nil {
			break
		}
		e2elog.Logf("%v", err)
		time.Sleep(time.Second * pause)
	}
	o.Expect(retryCount).NotTo(o.Equal(maxRetries + 1))
	return string(out)
}

func masterNodes(oc *exutil.CLI) []*corev1.Node {
	masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master",
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	var nodes []*corev1.Node
	for i := range masterNodes.Items {
		node := &masterNodes.Items[i]
		nodes = append(nodes, node)
	}
	return nodes
}

func clusterNodes(oc *exutil.CLI) (masters, workers []*corev1.Node) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
			masters = append(masters, node)
		} else {
			workers = append(workers, node)
		}
	}
	return
}

func waitForMastersToUpdate(oc *exutil.CLI, mcps dynamic.NamespaceableResourceInterface) {
	e2elog.Logf("Waiting for MachineConfig master to finish rolling out")
	err := wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
		return upgrade.IsPoolUpdated(mcps, "master")
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForOperatorsToSettle(coc dynamic.NamespaceableResourceInterface) {
	var lastErr error
	// gate on all clusteroperators being ready
	available := make(map[string]struct{})
	lastErr = nil
	var lastCOs []objx.Map
	wait.PollImmediate(30*time.Second, operatorWait, func() (bool, error) {
		obj, err := coc.List(context.Background(), metav1.ListOptions{})
		if err != nil {
			lastErr = err
			e2elog.Logf("Unable to check for cluster operators: %v", err)
			return false, nil
		}
		cv := objx.Map(obj.UnstructuredContent())
		lastErr = nil
		items := objects(cv.Get("items"))
		lastCOs = items

		if len(items) == 0 {
			return false, nil
		}

		var unavailable []objx.Map
		var unavailableNames []string
		for _, co := range items {
			if condition(co, "Available").Get("status").String() != "True" {
				ns := co.Get("metadata.namespace").String()
				name := co.Get("metadata.name").String()
				unavailableNames = append(unavailableNames, fmt.Sprintf("%s/%s", ns, name))
				unavailable = append(unavailable, co)
				break
			}
			if condition(co, "Progressing").Get("status").String() != "False" {
				ns := co.Get("metadata.namespace").String()
				name := co.Get("metadata.name").String()
				unavailableNames = append(unavailableNames, fmt.Sprintf("%s/%s", ns, name))
				unavailable = append(unavailable, co)
				break
			}
			if condition(co, "Degraded").Get("status").String() != "False" {
				ns := co.Get("metadata.namespace").String()
				name := co.Get("metadata.name").String()
				unavailableNames = append(unavailableNames, fmt.Sprintf("%s/%s", ns, name))
				unavailable = append(unavailable, co)
				break
			}
		}
		if len(unavailable) > 0 {
			e2elog.Logf("Operators still doing work: %s", strings.Join(unavailableNames, ", "))
			return false, nil
		}
		return true, nil
	})

	o.Expect(lastErr).NotTo(o.HaveOccurred())
	var unavailable []string
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 4, 1, ' ', 0)
	fmt.Fprintf(w, "NAMESPACE\tNAME\tPROGRESSING\tAVAILABLE\tVERSION\tMESSAGE\n")
	for _, co := range lastCOs {
		ns := co.Get("metadata.namespace").String()
		name := co.Get("metadata.name").String()
		if condition(co, "Available").Get("status").String() != "True" {
			unavailable = append(unavailable, fmt.Sprintf("%s/%s", ns, name))
		} else {
			available[fmt.Sprintf("%s/%s", ns, name)] = struct{}{}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			ns,
			name,
			condition(co, "Progressing").Get("status").String(),
			condition(co, "Available").Get("status").String(),
			co.Get("status.version").String(),
			condition(co, "Degraded").Get("message").String(),
		)
	}
	w.Flush()
	e2elog.Logf("ClusterOperators:\n%s", buf.String())
	if len(unavailable) > 0 {
		e2e.Failf("Some cluster operators never became available %s", strings.Join(unavailable, ", "))
	}
	// Check at least one core operator is available
	if len(available) == 0 {
		e2e.Failf("There must be at least one cluster operator")
	}
}

func restartSDNPods(oc *exutil.CLI) {
	e2elog.Logf("Restarting SDN")

	pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-sdn").List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, pod := range pods.Items {
		e2elog.Logf("Deleting pod %s", pod.Name)
		err := oc.AdminKubeClient().CoreV1().Pods("openshift-sdn").Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	err = wait.Poll(10*time.Second, 5*time.Minute, func() (done bool, err error) {
		sdnDaemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets("openshift-sdn").Get(context.Background(), "sdn", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return sdnDaemonset.Status.NumberReady == sdnDaemonset.Status.NumberAvailable, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func objects(from *objx.Value) []objx.Map {
	var values []objx.Map
	switch {
	case from.IsObjxMapSlice():
		return from.ObjxMapSlice()
	case from.IsInterSlice():
		for _, i := range from.InterSlice() {
			if msi, ok := i.(map[string]interface{}); ok {
				values = append(values, objx.Map(msi))
			}
		}
	}
	return values
}

func condition(cv objx.Map, condition string) objx.Map {
	for _, obj := range objects(cv.Get("status.conditions")) {
		if obj.Get("type").String() == condition {
			return obj
		}
	}
	return objx.Map(nil)
}

func nodeConditionStatus(conditions []corev1.NodeCondition, conditionType corev1.NodeConditionType) corev1.ConditionStatus {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status
		}
	}
	return corev1.ConditionUnknown
}

func countReady(items []corev1.Node) int {
	ready := 0
	for _, item := range items {
		if nodeConditionStatus(item.Status.Conditions, corev1.NodeReady) == corev1.ConditionTrue {
			ready++
		}
	}
	return ready
}

func fetchFileContents(node *corev1.Node, path string) string {
	e2elog.Logf("Fetching %s file contents from %s", path, node.Name)
	out := execOnNodeWithOutputOrFail(node, fmt.Sprintf("cat %q", path))
	return out.Stdout
}

// execOnNodeWithOutputOrFail executes a command via ssh against a
// node in a poll loop to ensure reliable execution in a disrupted
// environment. The calling test will be failed if the command cannot
// be executed successfully before the provided timeout.
func execOnNodeWithOutputOrFail(node *corev1.Node, cmd string) *e2essh.Result {
	var out *e2essh.Result
	var err error
	waitErr := wait.PollImmediate(5*time.Second, defaultSSHTimeout, func() (bool, error) {
		out, err = e2essh.IssueSSHCommandWithResult(cmd, e2e.TestContext.Provider, node)
		// IssueSSHCommandWithResult logs output
		if err != nil {
			e2elog.Logf("Failed to exec cmd [%s] on node %s: %v", cmd, node.Name, err)
		}
		return err == nil, nil
	})
	o.Expect(waitErr).NotTo(o.HaveOccurred())
	return out
}

// execOnNodeOrFail executes a command via ssh against a node in a
// poll loop until success or timeout. The output is ignored. The
// calling test will be failed if the command cannot be executed
// successfully before the timeout.
func execOnNodeOrFail(node *corev1.Node, cmd string) {
	_ = execOnNodeWithOutputOrFail(node, cmd)
}

// checkSSH repeatedly attempts to establish an ssh connection to a
// node and fails the calling test if unable to establish the
// connection before the default timeout.
func checkSSH(node *corev1.Node) {
	_ = execOnNodeWithOutputOrFail(node, "true")
}

func ssh(cmd string, node *corev1.Node) (*e2essh.Result, error) {
	return e2essh.IssueSSHCommandWithResult(cmd, e2e.TestContext.Provider, node)
}
