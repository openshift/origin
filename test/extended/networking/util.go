package networking

import (
	"fmt"
	"os"
	"time"

	testexutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type NodeType int

const (
	// Initial pod start can be delayed O(minutes) by slow docker pulls
	// TODO: Make this 30 seconds once #4566 is resolved.
	podStartTimeout = 5 * time.Minute

	// How often to poll pods and nodes.
	poll = 5 * time.Second

	// How wide to print pod names, by default. Useful for aligning printing to
	// quickly scan through output.
	podPrintWidth = 55

	// Indicator for same or different node
	SAME_NODE      NodeType = iota
	DIFFERENT_NODE NodeType = iota
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

func launchWebserverService(f *e2e.Framework, serviceName string, nodeName string) (serviceAddr string) {
	e2e.LaunchWebserverPod(f, serviceName, nodeName)
	// FIXME: make e2e.LaunchWebserverPod() set the label when creating the pod
	podClient := f.Client.Pods(f.Namespace.Name)
	pod, err := podClient.Get(serviceName)
	expectNoError(err)
	pod.ObjectMeta.Labels = make(map[string]string)
	pod.ObjectMeta.Labels["name"] = "web"
	podClient.Update(pod)

	servicePort := 8080
	service := &api.Service{
		ObjectMeta: api.ObjectMeta{
			Name: serviceName,
		},
		Spec: api.ServiceSpec{
			Type: api.ServiceTypeClusterIP,
			Ports: []api.ServicePort{
				{
					Protocol: api.ProtocolTCP,
					Port:     int32(servicePort),
				},
			},
			Selector: map[string]string{
				"name": "web",
			},
		},
	}
	serviceClient := f.Client.Services(f.Namespace.Name)
	_, err = serviceClient.Create(service)
	expectNoError(err)
	expectNoError(f.WaitForAnEndpoint(serviceName))
	createdService, err := serviceClient.Get(serviceName)
	expectNoError(err)
	serviceAddr = fmt.Sprintf("%s:%d", createdService.Spec.ClusterIP, servicePort)
	e2e.Logf("Target service IP:port is %s", serviceAddr)
	return
}

func checkConnectivityToHost(f *e2e.Framework, nodeName string, podName string, host string, timeout int) error {
	contName := fmt.Sprintf("%s-container", podName)
	pod := &api.Pod{
		TypeMeta: unversioned.TypeMeta{
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

func pluginIsolatesNamespaces() bool {
	return os.Getenv("OPENSHIFT_NETWORK_ISOLATION") == "true"
}

func makeNamespaceGlobal(ns *api.Namespace) {
	client, err := testutil.GetClusterAdminClient(testexutil.KubeConfigPath())
	expectNoError(err)
	netns, err := client.NetNamespaces().Get(ns.Name)
	expectNoError(err)
	netns.NetID = 0
	_, err = client.NetNamespaces().Update(netns)
	expectNoError(err)
}

func checkPodIsolation(f1, f2 *e2e.Framework, nodeType NodeType) error {
	nodes := e2e.GetReadySchedulableNodesOrDie(f1.Client)
	var serverNode, clientNode *api.Node
	serverNode = &nodes.Items[0]
	if nodeType == DIFFERENT_NODE {
		if len(nodes.Items) == 1 {
			e2e.Skipf("Only one node is available in this environment")
		}
		clientNode = &nodes.Items[1]
	} else {
		clientNode = serverNode
	}

	podName := "isolation-webserver"
	defer f1.Client.Pods(f1.Namespace.Name).Delete(podName, nil)
	ip := e2e.LaunchWebserverPod(f1, podName, serverNode.Name)

	return checkConnectivityToHost(f2, clientNode.Name, "isolation-wget", ip, 10)
}

func checkServiceConnectivity(serverFramework, clientFramework *e2e.Framework, nodeType NodeType) error {
	nodes := e2e.GetReadySchedulableNodesOrDie(serverFramework.Client)
	var serverNode, clientNode *api.Node
	serverNode = &nodes.Items[0]
	if nodeType == DIFFERENT_NODE {
		if len(nodes.Items) == 1 {
			e2e.Skipf("Only one node is available in this environment")
		}
		clientNode = &nodes.Items[1]
	} else {
		clientNode = serverNode
	}

	podName := api.SimpleNameGenerator.GenerateName("service-")
	defer serverFramework.Client.Pods(serverFramework.Namespace.Name).Delete(podName, nil)
	defer serverFramework.Client.Services(serverFramework.Namespace.Name).Delete(podName)
	ip := launchWebserverService(serverFramework, podName, serverNode.Name)

	return checkConnectivityToHost(clientFramework, clientNode.Name, "service-wget", ip, 10)
}

func InSingleTenantContext(body func()) {
	Context("when using a single-tenant plugin", func() {
		BeforeEach(func() {
			if pluginIsolatesNamespaces() {
				e2e.Skipf("Not a single-tenant plugin.")
			}
		})

		body()
	})
}

func InMultiTenantContext(body func()) {
	Context("when using a multi-tenant plugin", func() {
		BeforeEach(func() {
			if !pluginIsolatesNamespaces() {
				e2e.Skipf("Not a multi-tenant plugin.")
			}
		})

		body()
	})
}
