package networking

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/test/e2e"

	exutil "github.com/openshift/origin/test/extended/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	// Initial pod start can be delayed O(minutes) by slow docker pulls
	// TODO: Make this 30 seconds once #4566 is resolved.
	podStartTimeout = 5 * time.Minute

	// How often to poll pods and nodes.
	poll = 5 * time.Second

	// How wide to print pod names, by default. Useful for aligning printing to
	// quickly scan through output.
	podPrintWidth = 55
)

func expectNoError(err error, explain ...interface{}) {
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), explain...)
}

// podReady returns whether pod has a condition of Ready with a status of true.
func podReady(pod *api.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == api.PodReady && cond.Status == api.ConditionTrue {
			return true
		}
	}
	return false
}

type podCondition func(pod *api.Pod) (bool, error)

func waitForPodCondition(c *client.Client, ns, podName, desc string, timeout time.Duration, condition podCondition) error {
	e2e.Logf("Waiting up to %[1]v for pod %-[2]*[3]s status to be %[4]s", timeout, podPrintWidth, podName, desc)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(poll) {
		pod, err := c.Pods(ns).Get(podName)
		if err != nil {
			// Aligning this text makes it much more readable
			e2e.Logf("Get pod %-[1]*[2]s in namespace '%[3]s' failed, ignoring for %[4]v. Error: %[5]v",
				podPrintWidth, podName, ns, poll, err)
			continue
		}
		done, err := condition(pod)
		if done {
			return err
		}
		e2e.Logf("Waiting for pod %-[1]*[2]s in namespace '%[3]s' status to be '%[4]s'"+
			"(found phase: %[5]q, readiness: %[6]t) (%[7]v elapsed)",
			podPrintWidth, podName, ns, desc, pod.Status.Phase, podReady(pod), time.Since(start))
	}
	return fmt.Errorf("gave up waiting for pod '%s' to be '%s' after %v", podName, desc, timeout)
}

// waitForPodSuccessInNamespace returns nil if the pod reached state success, or an error if it reached failure or ran too long.
func waitForPodSuccessInNamespace(c *client.Client, podName string, contName string, namespace string) error {
	return waitForPodCondition(c, namespace, podName, "success or failure", podStartTimeout, func(pod *api.Pod) (bool, error) {
		// Cannot use pod.Status.Phase == api.PodSucceeded/api.PodFailed due to #2632
		ci, ok := api.GetContainerStatus(pod.Status.ContainerStatuses, contName)
		if !ok {
			e2e.Logf("No Status.Info for container '%s' in pod '%s' yet", contName, podName)
		} else {
			if ci.State.Terminated != nil {
				if ci.State.Terminated.ExitCode == 0 {
					By("Saw pod success")
					return true, nil
				}
				return true, fmt.Errorf("pod '%s' terminated with failure: %+v", podName, ci.State.Terminated)
			}
			e2e.Logf("Nil State.Terminated for container '%s' in pod '%s' in namespace '%s' so far", contName, podName, namespace)
		}
		return false, nil
	})
}

func isNodeReadySetAsExpected(node *api.Node, wantReady bool) bool {
	// Check the node readiness condition (logging all).
	for i, cond := range node.Status.Conditions {
		e2e.Logf("Node %s condition %d/%d: type: %v, status: %v, reason: %q, message: %q, last transition time: %v",
			node.Name, i+1, len(node.Status.Conditions), cond.Type, cond.Status,
			cond.Reason, cond.Message, cond.LastTransitionTime)
		// Ensure that the condition type is readiness and the status
		// matches as desired.
		if cond.Type == api.NodeReady && (cond.Status == api.ConditionTrue) == wantReady {
			e2e.Logf("Successfully found node %s readiness to be %t", node.Name, wantReady)
			return true
		}
	}
	return false
}

func providerIs(providers ...string) bool {
	for _, provider := range providers {
		if strings.ToLower(provider) == strings.ToLower(exutil.TestContext.Provider) {
			return true
		}
	}
	return false
}

// Filters nodes in NodeList in place, removing nodes that do not
// satisfy the given condition
// TODO: consider merging with pkg/client/cache.NodeLister
func filterNodes(nodeList *api.NodeList, fn func(node api.Node) bool) {
	var l []api.Node

	for _, node := range nodeList.Items {
		if fn(node) {
			l = append(l, node)
		}
	}
	nodeList.Items = l
}

func getMultipleNodes(f *e2e.Framework) (nodes *api.NodeList) {
	nodes, err := f.Client.Nodes().List(labels.Everything(), fields.Everything())
	if err != nil {
		e2e.Failf("Failed to list nodes: %v", err)
	}
	// previous tests may have cause failures of some nodes. Let's skip
	// 'Not Ready' nodes, just in case (there is no need to fail the test).
	filterNodes(nodes, func(node api.Node) bool {
		return isNodeReadySetAsExpected(&node, true)
	})

	if len(nodes.Items) == 0 {
		e2e.Failf("No Ready nodes found.")
	}
	if len(nodes.Items) == 1 {
		// in general, the test requires two nodes. But for local development, often a one node cluster
		// is created, for simplicity and speed. (see issue #10012). We permit one-node test
		// only in some cases
		if !providerIs("local") {
			e2e.Failf(fmt.Sprintf("The test requires two Ready nodes on %s, but found just one.", exutil.TestContext.Provider))
		}
		e2e.Logf("Only one ready node is detected. The test has limited scope in such setting. " +
			"Rerun it with at least two nodes to get complete coverage.")
	}
	return
}

func launchWebserverPod(f *e2e.Framework, podName string, nodeName string) (ip string) {
	containerName := fmt.Sprintf("%s-container", podName)
	port := 8080
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name: podName,
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:  containerName,
					Image: "gcr.io/google_containers/porter:59ad46ed2c56ba50fa7f1dc176c07c37",
					Env:   []api.EnvVar{{Name: fmt.Sprintf("SERVE_PORT_%d", port), Value: "foo"}},
					Ports: []api.ContainerPort{{ContainerPort: port}},
				},
			},
			NodeName:      nodeName,
			RestartPolicy: api.RestartPolicyNever,
		},
	}
	podClient := f.Client.Pods(f.Namespace.Name)
	_, err := podClient.Create(pod)
	expectNoError(err)
	expectNoError(f.WaitForPodRunning(podName))
	createdPod, err := podClient.Get(podName)
	expectNoError(err)
	ip = fmt.Sprintf("%s:%d", createdPod.Status.PodIP, port)
	e2e.Logf("Target pod IP:port is %s", ip)
	return
}

func checkConnectivityToHost(f *e2e.Framework, nodeName string, podName string, host string, timeout int) error {
	contName := fmt.Sprintf("%s-container", podName)
	pod := &api.Pod{
		TypeMeta: api.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: api.ObjectMeta{
			Name: podName,
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:    contName,
					Image:   "gcr.io/google_containers/busybox",
					Command: []string{"wget", fmt.Sprintf("--timeout=%d", timeout), "-s", host},
				},
			},
			NodeName:      nodeName,
			RestartPolicy: api.RestartPolicyNever,
		},
	}
	podClient := f.Client.Pods(f.Namespace.Name)
	_, err := podClient.Create(pod)
	expectNoError(err)
	defer podClient.Delete(podName, nil)
	return waitForPodSuccessInNamespace(f.Client, podName, contName, f.Namespace.Name)
}
