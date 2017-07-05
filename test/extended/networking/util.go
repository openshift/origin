package networking

import (
	"fmt"
	"os"
	"time"

	testexutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	kapiv1pod "k8s.io/kubernetes/pkg/api/v1/pod"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/client/retry"
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
func podReady(pod *kapiv1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == kapiv1.PodReady && cond.Status == kapiv1.ConditionTrue {
			return true
		}
	}
	return false
}

type podCondition func(pod *kapiv1.Pod) (bool, error)

func waitForPodCondition(c kclientset.Interface, ns, podName, desc string, timeout time.Duration, condition podCondition) error {
	e2e.Logf("Waiting up to %[1]v for pod %-[2]*[3]s status to be %[4]s", timeout, podPrintWidth, podName, desc)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(poll) {
		pod, err := c.CoreV1().Pods(ns).Get(podName, metav1.GetOptions{})
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
func waitForPodSuccessInNamespace(c kclientset.Interface, podName string, contName string, namespace string) error {
	return waitForPodCondition(c, namespace, podName, "success or failure", podStartTimeout, func(pod *kapiv1.Pod) (bool, error) {
		// Cannot use pod.Status.Phase == api.PodSucceeded/api.PodFailed due to #2632
		ci, ok := kapiv1pod.GetContainerStatus(pod.Status.ContainerStatuses, contName)
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

func waitForEndpoint(c kclientset.Interface, ns, name string) error {
	for t := time.Now(); time.Since(t) < 3*time.Minute; time.Sleep(poll) {
		endpoint, err := c.Core().Endpoints(ns).Get(name, metav1.GetOptions{})
		if kapierrs.IsNotFound(err) {
			e2e.Logf("Endpoint %s/%s is not ready yet", ns, name)
			continue
		}
		Expect(err).NotTo(HaveOccurred())
		if len(endpoint.Subsets) == 0 || len(endpoint.Subsets[0].Addresses) == 0 {
			e2e.Logf("Endpoint %s/%s is not ready yet", ns, name)
			continue
		} else {
			return nil
		}
	}
	return fmt.Errorf("Failed to get endpoints for %s/%s", ns, name)
}

func launchWebserverService(f *e2e.Framework, serviceName string, nodeName string) (serviceAddr string) {
	e2e.LaunchWebserverPod(f, serviceName, nodeName)

	// FIXME: make e2e.LaunchWebserverPod() set the label when creating the pod
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		podClient := f.ClientSet.CoreV1().Pods(f.Namespace.Name)
		pod, err := podClient.Get(serviceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod.ObjectMeta.Labels == nil {
			pod.ObjectMeta.Labels = make(map[string]string)
		}
		pod.ObjectMeta.Labels["name"] = "web"
		_, err = podClient.Update(pod)
		return err
	})
	expectNoError(err)

	servicePort := 8080
	service := &kapiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: kapiv1.ServiceSpec{
			Type: kapiv1.ServiceTypeClusterIP,
			Ports: []kapiv1.ServicePort{
				{
					Protocol: kapiv1.ProtocolTCP,
					Port:     int32(servicePort),
				},
			},
			Selector: map[string]string{
				"name": "web",
			},
		},
	}
	serviceClient := f.ClientSet.CoreV1().Services(f.Namespace.Name)
	_, err = serviceClient.Create(service)
	expectNoError(err)
	expectNoError(waitForEndpoint(f.ClientSet, f.Namespace.Name, serviceName))
	createdService, err := serviceClient.Get(serviceName, metav1.GetOptions{})
	expectNoError(err)
	serviceAddr = fmt.Sprintf("%s:%d", createdService.Spec.ClusterIP, servicePort)
	e2e.Logf("Target service IP:port is %s", serviceAddr)
	return
}

func checkConnectivityToHost(f *e2e.Framework, nodeName string, podName string, host string, timeout int) error {
	contName := fmt.Sprintf("%s-container", podName)
	pod := &kapiv1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: kapiv1.PodSpec{
			Containers: []kapiv1.Container{
				{
					Name:    contName,
					Image:   "gcr.io/google_containers/busybox",
					Command: []string{"wget", fmt.Sprintf("--timeout=%d", timeout), "-s", host},
				},
			},
			NodeName:      nodeName,
			RestartPolicy: kapiv1.RestartPolicyNever,
		},
	}
	podClient := f.ClientSet.CoreV1().Pods(f.Namespace.Name)
	_, err := podClient.Create(pod)
	expectNoError(err)
	defer podClient.Delete(podName, nil)
	return waitForPodSuccessInNamespace(f.ClientSet, podName, contName, f.Namespace.Name)
}

func pluginIsolatesNamespaces() bool {
	return os.Getenv("NETWORKING_E2E_ISOLATION") == "true"
}

func pluginImplementsNetworkPolicy() bool {
	return os.Getenv("NETWORKING_E2E_NETWORKPOLICY") == "true"
}

func makeNamespaceGlobal(ns *kapiv1.Namespace) {
	client, err := testutil.GetClusterAdminClient(testexutil.KubeConfigPath())
	expectNoError(err)
	netns, err := client.NetNamespaces().Get(ns.Name, metav1.GetOptions{})
	expectNoError(err)
	netns.NetID = 0
	_, err = client.NetNamespaces().Update(netns)
	expectNoError(err)
}

func checkPodIsolation(f1, f2 *e2e.Framework, nodeType NodeType) error {
	nodes := e2e.GetReadySchedulableNodesOrDie(f1.ClientSet)
	var serverNode, clientNode *kapiv1.Node
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
	defer f1.ClientSet.CoreV1().Pods(f1.Namespace.Name).Delete(podName, nil)
	ip := e2e.LaunchWebserverPod(f1, podName, serverNode.Name)

	return checkConnectivityToHost(f2, clientNode.Name, "isolation-wget", ip, 10)
}

func checkServiceConnectivity(serverFramework, clientFramework *e2e.Framework, nodeType NodeType) error {
	nodes := e2e.GetReadySchedulableNodesOrDie(serverFramework.ClientSet)
	var serverNode, clientNode *kapiv1.Node
	serverNode = &nodes.Items[0]
	if nodeType == DIFFERENT_NODE {
		if len(nodes.Items) == 1 {
			e2e.Skipf("Only one node is available in this environment")
		}
		clientNode = &nodes.Items[1]
	} else {
		clientNode = serverNode
	}

	podName := kapiv1.SimpleNameGenerator.GenerateName("service-")
	defer serverFramework.ClientSet.CoreV1().Pods(serverFramework.Namespace.Name).Delete(podName, nil)
	defer serverFramework.ClientSet.CoreV1().Services(serverFramework.Namespace.Name).Delete(podName, nil)
	ip := launchWebserverService(serverFramework, podName, serverNode.Name)

	return checkConnectivityToHost(clientFramework, clientNode.Name, "service-wget", ip, 10)
}

func InSingleTenantContext(body func()) {
	Context("when using a plugin that does not isolate namespaces by default", func() {
		BeforeEach(func() {
			if pluginIsolatesNamespaces() {
				e2e.Skipf("This plugin isolates namespaces by default.")
			}
		})

		body()
	})
}

func InMultiTenantContext(body func()) {
	Context("when using a plugin that isolates namespaces by default", func() {
		BeforeEach(func() {
			if !pluginIsolatesNamespaces() {
				e2e.Skipf("This plugin does not isolate namespaces by default.")
			}
		})

		body()
	})
}

func InNetworkPolicyContext(body func()) {
	Context("when using a plugin that implements NetworkPolicy", func() {
		BeforeEach(func() {
			if !pluginImplementsNetworkPolicy() {
				e2e.Skipf("This plugin does not implement NetworkPolicy.")
			}
		})

		body()
	})
}
