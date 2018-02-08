package networking

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/network"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
	testexutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"

	corev1 "k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	kapiv1pod "k8s.io/kubernetes/pkg/api/v1/pod"
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
func podReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

type podCondition func(pod *corev1.Pod) (bool, error)

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
	return waitForPodCondition(c, namespace, podName, "success or failure", podStartTimeout, func(pod *corev1.Pod) (bool, error) {
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
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolTCP,
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

func checkConnectivityToHost(f *e2e.Framework, nodeName string, podName string, host string, timeout time.Duration) error {
	e2e.Logf("Creating an exec pod on node %v", nodeName)
	execPodName := e2e.CreateExecPodOrFail(f.ClientSet, f.Namespace.Name, fmt.Sprintf("execpod-sourceip-%s", nodeName), func(pod *corev1.Pod) {
		pod.Spec.NodeName = nodeName
	})
	defer func() {
		e2e.Logf("Cleaning up the exec pod")
		err := f.ClientSet.Core().Pods(f.Namespace.Name).Delete(execPodName, nil)
		Expect(err).NotTo(HaveOccurred())
	}()
	execPod, err := f.ClientSet.Core().Pods(f.Namespace.Name).Get(execPodName, metav1.GetOptions{})
	e2e.ExpectNoError(err)

	var stdout string
	e2e.Logf("Waiting up to %v to wget %s", timeout, host)
	cmd := fmt.Sprintf("wget -T 30 -qO- %s", host)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(2) {
		stdout, err = e2e.RunHostCmd(execPod.Namespace, execPod.Name, cmd)
		if err != nil {
			e2e.Logf("got err: %v, retry until timeout", err)
			continue
		}
		// Need to check output because wget -q might omit the error.
		if strings.TrimSpace(stdout) == "" {
			e2e.Logf("got empty stdout, retry until timeout")
			continue
		}
		break
	}
	if err == nil {
		return nil
	}
	savedErr := err

	// Debug
	debugPodName := e2e.CreateExecPodOrFail(f.ClientSet, f.Namespace.Name, fmt.Sprintf("debugpod-sourceip-%s", nodeName), func(pod *corev1.Pod) {
		pod.Spec.Containers[0].Image = "openshift/node"
		pod.Spec.NodeName = nodeName
		pod.Spec.HostNetwork = true
		privileged := true
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "ovs-socket",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/var/run/openvswitch/br0.mgmt",
					},
				},
			},
		}
		pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "ovs-socket",
				MountPath: "/var/run/openvswitch/br0.mgmt",
			},
		}
		pod.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{Privileged: &privileged}
	})
	defer func() {
		err := f.ClientSet.Core().Pods(f.Namespace.Name).Delete(debugPodName, nil)
		Expect(err).NotTo(HaveOccurred())
	}()
	debugPod, err := f.ClientSet.Core().Pods(f.Namespace.Name).Get(debugPodName, metav1.GetOptions{})
	e2e.ExpectNoError(err)

	stdout, err = e2e.RunHostCmd(debugPod.Namespace, debugPod.Name, "ovs-ofctl -O OpenFlow13 dump-flows br0")
	if err != nil {
		e2e.Logf("DEBUG: got error dumping OVS flows: %v", err)
	} else {
		e2e.Logf("DEBUG:\n%s\n", stdout)
	}
	stdout, err = e2e.RunHostCmd(debugPod.Namespace, debugPod.Name, "iptables-save")
	if err != nil {
		e2e.Logf("DEBUG: got error dumping iptables: %v", err)
	} else {
		e2e.Logf("DEBUG:\n%s\n", stdout)
	}
	stdout, err = e2e.RunHostCmd(debugPod.Namespace, debugPod.Name, "ss -ant")
	if err != nil {
		e2e.Logf("DEBUG: got error dumping sockets: %v", err)
	} else {
		e2e.Logf("DEBUG:\n%s\n", stdout)
	}
	return savedErr
}

var cachedNetworkPluginName *string

func networkPluginName() string {
	if cachedNetworkPluginName == nil {
		// We don't use testexutil.NewCLI() here because it can't be called from BeforeEach()
		out, err := exec.Command(
			"oc", "--config="+testexutil.KubeConfigPath(),
			"get", "clusternetwork", "default",
			"--template={{.pluginName}}",
		).CombinedOutput()
		pluginName := string(out)
		if err != nil {
			e2e.Logf("Could not check network plugin name: %v. Assuming a non-OpenShift plugin", err)
			pluginName = ""
		}
		cachedNetworkPluginName = &pluginName
	}
	return *cachedNetworkPluginName
}

func pluginIsolatesNamespaces() bool {
	if os.Getenv("NETWORKING_E2E_ISOLATION") == "true" {
		return true
	}
	// Assume that only the OpenShift SDN "multitenant" plugin isolates by default
	return networkPluginName() == network.MultiTenantPluginName
}

func pluginImplementsNetworkPolicy() bool {
	if os.Getenv("NETWORKING_E2E_NETWORKPOLICY") == "false" {
		return false
	}
	// Assume that all plugins except the OpenShift SDN "subnet" and "multitenant"
	// plugins implement NetworkPolicy
	return networkPluginName() != network.SingleTenantPluginName &&
		networkPluginName() != network.MultiTenantPluginName
}

func makeNamespaceGlobal(ns *corev1.Namespace) {
	clientConfig, err := testutil.GetClusterAdminClientConfig(testexutil.KubeConfigPath())
	networkClient := networkclient.NewForConfigOrDie(clientConfig)
	expectNoError(err)
	netns, err := networkClient.Network().NetNamespaces().Get(ns.Name, metav1.GetOptions{})
	expectNoError(err)
	netns.NetID = 0
	_, err = networkClient.Network().NetNamespaces().Update(netns)
	expectNoError(err)
}

func makeNamespaceScheduleToAllNodes(f *e2e.Framework) {
	// to avoid hassles dealing with selector limits, set the namespace label selector to empty
	// to allow targeting all nodes
	for {
		ns, err := f.ClientSet.CoreV1().Namespaces().Get(f.Namespace.Name, metav1.GetOptions{})
		expectNoError(err)
		ns.Annotations["openshift.io/node-selector"] = ""
		_, err = f.ClientSet.CoreV1().Namespaces().Update(ns)
		if err == nil {
			return
		}
		if kapierrs.IsConflict(err) {
			continue
		}
		expectNoError(err)
	}
}

// findAppropriateNodes tries to find a source and destination for a type of node connectivity
// test (same node, or different node).
func findAppropriateNodes(f *e2e.Framework, nodeType NodeType) (*corev1.Node, *corev1.Node) {
	nodes := e2e.GetReadySchedulableNodesOrDie(f.ClientSet)
	candidates := nodes.Items

	if len(candidates) == 0 {
		e2e.Failf("Unable to find any candidate nodes for e2e networking tests in \n%#v", nodes.Items)
	}

	// in general, avoiding masters is a good thing, so see if we can find nodes that aren't masters
	if len(candidates) > 1 {
		var withoutMasters []corev1.Node
		// look for anything that has the label value master or infra and try to skip it
		isAllowed := func(node *corev1.Node) bool {
			for _, value := range node.Labels {
				if value == "master" || value == "infra" {
					return false
				}
			}
			return true
		}
		for _, node := range candidates {
			if !isAllowed(&node) {
				continue
			}
			withoutMasters = append(withoutMasters, node)
		}
		if len(withoutMasters) >= 2 {
			candidates = withoutMasters
		}
	}

	var candidateNames, nodeNames []string
	for _, node := range candidates {
		candidateNames = append(candidateNames, node.Name)
	}
	for _, node := range nodes.Items {
		nodeNames = append(nodeNames, node.Name)
	}

	if nodeType == DIFFERENT_NODE {
		if len(candidates) <= 1 {
			e2e.Skipf("Only one node is available in this environment (%v out of %v)", candidateNames, nodeNames)
		}
		e2e.Logf("Using %s and %s for test (%v out of %v)", candidates[0].Name, candidates[1].Name, candidateNames, nodeNames)
		return &candidates[0], &candidates[1]
	}
	e2e.Logf("Using %s for test (%v out of %v)", candidates[0].Name, candidateNames, nodeNames)
	return &candidates[0], &candidates[0]
}

func checkPodIsolation(f1, f2 *e2e.Framework, nodeType NodeType) error {
	makeNamespaceScheduleToAllNodes(f1)
	makeNamespaceScheduleToAllNodes(f2)
	serverNode, clientNode := findAppropriateNodes(f1, nodeType)
	podName := "isolation-webserver"
	defer f1.ClientSet.CoreV1().Pods(f1.Namespace.Name).Delete(podName, nil)
	ip := e2e.LaunchWebserverPod(f1, podName, serverNode.Name)

	return checkConnectivityToHost(f2, clientNode.Name, "isolation-wget", ip, 10*time.Second)
}

func checkServiceConnectivity(serverFramework, clientFramework *e2e.Framework, nodeType NodeType) error {
	makeNamespaceScheduleToAllNodes(serverFramework)
	makeNamespaceScheduleToAllNodes(clientFramework)
	serverNode, clientNode := findAppropriateNodes(serverFramework, nodeType)
	podName := names.SimpleNameGenerator.GenerateName("service-")
	defer serverFramework.ClientSet.CoreV1().Pods(serverFramework.Namespace.Name).Delete(podName, nil)
	defer serverFramework.ClientSet.CoreV1().Services(serverFramework.Namespace.Name).Delete(podName, nil)
	ip := launchWebserverService(serverFramework, podName, serverNode.Name)

	return checkConnectivityToHost(clientFramework, clientNode.Name, "service-wget", ip, 10*time.Second)
}

func InNonIsolatingContext(body func()) {
	Context("when using a plugin that does not isolate namespaces by default", func() {
		BeforeEach(func() {
			if pluginIsolatesNamespaces() {
				e2e.Skipf("This plugin isolates namespaces by default.")
			}
		})

		body()
	})
}

func InIsolatingContext(body func()) {
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
