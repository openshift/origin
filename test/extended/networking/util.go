package networking

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	projectv1 "github.com/openshift/api/project/v1"
	networkclient "github.com/openshift/client-go/network/clientset/versioned/typed/network/v1"
	"github.com/openshift/library-go/pkg/network/networkutils"
	"k8s.io/kubernetes/test/e2e/framework/pod"

	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

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

func launchWebserverService(f *e2e.Framework, serviceName string, nodeName string) (serviceAddr string) {
	exutil.LaunchWebserverPod(f, serviceName, nodeName)

	// FIXME: make e2e.LaunchWebserverPod() set the label when creating the pod
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		podClient := f.ClientSet.CoreV1().Pods(f.Namespace.Name)
		pod, err := podClient.Get(context.Background(), serviceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod.ObjectMeta.Labels == nil {
			pod.ObjectMeta.Labels = make(map[string]string)
		}
		pod.ObjectMeta.Labels["name"] = "web"
		_, err = podClient.Update(context.Background(), pod, metav1.UpdateOptions{})
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
	_, err = serviceClient.Create(context.Background(), service, metav1.CreateOptions{})
	expectNoError(err)
	expectNoError(exutil.WaitForEndpoint(f.ClientSet, f.Namespace.Name, serviceName))
	createdService, err := serviceClient.Get(context.Background(), serviceName, metav1.GetOptions{})
	expectNoError(err)
	serviceAddr = fmt.Sprintf("%s:%d", createdService.Spec.ClusterIP, servicePort)
	e2e.Logf("Target service IP:port is %s", serviceAddr)
	return
}

func checkConnectivityToHost(f *e2e.Framework, nodeName string, podName string, host string, timeout time.Duration) error {
	e2e.Logf("Creating an exec pod on node %v", nodeName)
	execPod := pod.CreateExecPodOrFail(f.ClientSet, f.Namespace.Name, fmt.Sprintf("execpod-sourceip-%s", nodeName), func(pod *corev1.Pod) {
		pod.Spec.NodeName = nodeName
	})
	defer func() {
		e2e.Logf("Cleaning up the exec pod")
		err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(context.Background(), execPod.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	}()

	var stdout string
	e2e.Logf("Waiting up to %v to wget %s", timeout, host)
	cmd := fmt.Sprintf("wget -T 30 -qO- %s", host)
	var err error
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
	return err
}

var cachedNetworkPluginName *string

func networkPluginName() string {
	if cachedNetworkPluginName == nil {
		// We don't use exutil.NewCLI() here because it can't be called from BeforeEach()
		out, err := exec.Command(
			"oc", "--kubeconfig="+exutil.KubeConfigPath(),
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
	return networkPluginName() == networkutils.MultiTenantPluginName
}

func pluginImplementsNetworkPolicy() bool {
	switch {
	case os.Getenv("NETWORKING_E2E_NETWORKPOLICY") == "true":
		return true
	case networkPluginName() == networkutils.NetworkPolicyPluginName:
		return true
	default:
		// If we can't detect the plugin, we assume it doesn't support
		// NetworkPolicy, so the tests will work under kubenet
		return false
	}
}

func makeNamespaceGlobal(oc *exutil.CLI, ns *corev1.Namespace) {
	clientConfig := oc.AdminConfig()
	networkClient := networkclient.NewForConfigOrDie(clientConfig)
	netns, err := networkClient.NetNamespaces().Get(context.Background(), ns.Name, metav1.GetOptions{})
	expectNoError(err)
	netns.NetID = 0
	_, err = networkClient.NetNamespaces().Update(context.Background(), netns, metav1.UpdateOptions{})
	expectNoError(err)
}

func makeNamespaceScheduleToAllNodes(f *e2e.Framework) {
	// to avoid hassles dealing with selector limits, set the namespace label selector to empty
	// to allow targeting all nodes
	for {
		ns, err := f.ClientSet.CoreV1().Namespaces().Get(context.Background(), f.Namespace.Name, metav1.GetOptions{})
		expectNoError(err)
		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		ns.Annotations[projectv1.ProjectNodeSelector] = ""
		_, err = f.ClientSet.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
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
func findAppropriateNodes(f *e2e.Framework, nodeType NodeType) (*corev1.Node, *corev1.Node, error) {
	nodes, err := e2enode.GetReadySchedulableNodes(f.ClientSet)
	if err != nil {
		e2e.Logf("Unable to get schedulable nodes due to %v", err)
		return nil, nil, err
	}
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
			e2eskipper.Skipf("Only one node is available in this environment (%v out of %v)", candidateNames, nodeNames)
		}
		e2e.Logf("Using %s and %s for test (%v out of %v)", candidates[0].Name, candidates[1].Name, candidateNames, nodeNames)
		return &candidates[0], &candidates[1], nil
	}
	e2e.Logf("Using %s for test (%v out of %v)", candidates[0].Name, candidateNames, nodeNames)
	return &candidates[0], &candidates[0], nil
}

func checkPodIsolation(f1, f2 *e2e.Framework, nodeType NodeType) error {
	makeNamespaceScheduleToAllNodes(f1)
	makeNamespaceScheduleToAllNodes(f2)
	serverNode, clientNode, err := findAppropriateNodes(f1, nodeType)
	if err != nil {
		return err
	}
	podName := "isolation-webserver"
	defer f1.ClientSet.CoreV1().Pods(f1.Namespace.Name).Delete(context.Background(), podName, metav1.DeleteOptions{})
	ip := exutil.LaunchWebserverPod(f1, podName, serverNode.Name)

	return checkConnectivityToHost(f2, clientNode.Name, "isolation-wget", ip, 10*time.Second)
}

func checkServiceConnectivity(serverFramework, clientFramework *e2e.Framework, nodeType NodeType) error {
	makeNamespaceScheduleToAllNodes(serverFramework)
	makeNamespaceScheduleToAllNodes(clientFramework)
	serverNode, clientNode, err := findAppropriateNodes(serverFramework, nodeType)
	if err != nil {
		return err
	}
	podName := names.SimpleNameGenerator.GenerateName("service-")
	defer serverFramework.ClientSet.CoreV1().Pods(serverFramework.Namespace.Name).Delete(context.Background(), podName, metav1.DeleteOptions{})
	defer serverFramework.ClientSet.CoreV1().Services(serverFramework.Namespace.Name).Delete(context.Background(), podName, metav1.DeleteOptions{})
	ip := launchWebserverService(serverFramework, podName, serverNode.Name)

	return checkConnectivityToHost(clientFramework, clientNode.Name, "service-wget", ip, 10*time.Second)
}

func InNonIsolatingContext(body func()) {
	Context("when using a plugin that does not isolate namespaces by default", func() {
		BeforeEach(func() {
			if pluginIsolatesNamespaces() {
				e2eskipper.Skipf("This plugin isolates namespaces by default.")
			}
		})

		body()
	})
}

func InIsolatingContext(body func()) {
	Context("when using a plugin that isolates namespaces by default", func() {
		BeforeEach(func() {
			if !pluginIsolatesNamespaces() {
				e2eskipper.Skipf("This plugin does not isolate namespaces by default.")
			}
		})

		body()
	})
}

func InNetworkPolicyContext(body func()) {
	Context("when using a plugin that implements NetworkPolicy", func() {
		BeforeEach(func() {
			if !pluginImplementsNetworkPolicy() {
				e2eskipper.Skipf("This plugin does not implement NetworkPolicy.")
			}
		})

		body()
	})
}

func InPluginContext(plugins []string, body func()) {
	Context(fmt.Sprintf("when using one of the plugins '%s'", strings.Join(plugins, ", ")),
		func() {
			BeforeEach(func() {
				found := false
				for _, plugin := range plugins {
					if networkPluginName() == plugin {
						found = true
						break
					}
				}
				if !found {
					e2eskipper.Skipf("Not using one of the specified plugins")
				}
			})

			body()
		},
	)
}

func InOpenShiftSDNContext(body func()) {
	Context("when using openshift-sdn",
		func() {
			BeforeEach(func() {
				if networkPluginName() == "" {
					e2eskipper.Skipf("Not using openshift-sdn")
				}
			})

			body()
		},
	)
}
