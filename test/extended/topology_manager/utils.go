package topologymanager

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/yaml"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
)

const (
	strictCheckEnvVar           = "TOPOLOGY_MANAGER_TEST_STRICT"
	roleWorkerEnvVar            = "ROLE_WORKER"
	resourceNameEnvVar          = "RESOURCE_NAME"
	sriovNetworkNamespaceEnvVar = "SRIOV_NETWORK_NAMESPACE"
	sriovNetworkEnvVar          = "SRIOV_NETWORK"
	ipFamilyEnvVar              = "IP_FAMILY"

	defaultRoleWorker   = "worker"
	defaultResourceName = "openshift.io/intelnics"
	// no default for sriovNetworkNamespace: use the e2e test framework default
	defaultSriovNetwork = "sriov-network"
	defaultIPFamily     = "v4"

	namespaceMachineConfigOperator = "openshift-machine-config-operator"
	containerMachineConfigDaemon   = "machine-config-daemon"
)

const (
	labelRole     = "node-role.kubernetes.io"
	labelHostname = "kubernetes.io/hostname"
)

const (
	filePathKubeletConfig = "/etc/kubernetes/kubelet.conf"
)

func getValueFromEnv(name, fallback, desc string) string {
	val := fallback
	if envVal, ok := os.LookupEnv(name); ok {
		val = envVal
	}
	e2e.Logf("%s: %q", desc, val)
	return val
}

func expectNonZeroNodes(nodes []corev1.Node, message string) {
	if _, ok := os.LookupEnv(strictCheckEnvVar); ok {
		o.Expect(nodes).ToNot(o.BeEmpty(), message)
	}
	if len(nodes) < 1 {
		g.Skip(message)
	}
}

func findNodeWithMultiNuma(nodes []corev1.Node, c clientset.Interface, oc *exutil.CLI) (*corev1.Node, int) {
	for _, node := range nodes {
		numaNodes, err := getNumaNodeCountFromNode(c, oc, &node)
		if err != nil {
			e2e.Logf("error getting the NUMA node count from %q: %v", node.Name, err)
			continue
		}
		if numaNodes > 1 {
			return &node, numaNodes
		}
	}
	return nil, 0
}

func filterNodeWithTopologyManagerPolicy(workerNodes []corev1.Node, client clientset.Interface, oc *exutil.CLI, policy string) []corev1.Node {
	ocRaw := (*oc).WithoutNamespace()

	var topoMgrNodes []corev1.Node

	for _, node := range workerNodes {
		kubeletConfig, err := getKubeletConfig(client, ocRaw, &node)
		e2e.ExpectNoError(err)

		e2e.Logf("kubelet %s CPU Manager policy: %q", node.Name, kubeletConfig.CPUManagerPolicy)
		if kubeletConfig.TopologyManagerPolicy != policy {
			e2e.Logf("kubelet %s Topology Manager policy: %q", node.Name, kubeletConfig.TopologyManagerPolicy)
			continue
		}
		topoMgrNodes = append(topoMgrNodes, node)
	}
	return topoMgrNodes
}

func getNodeByRole(c clientset.Interface, role string) ([]corev1.Node, error) {
	selector, err := labels.Parse(fmt.Sprintf("%s/%s=", labelRole, role))
	if err != nil {
		return nil, err
	}

	nodes := &corev1.NodeList{}
	if nodes, err = c.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: selector.String()}); err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

func getMachineConfigDaemonByNode(c clientset.Interface, node *corev1.Node) (*corev1.Pod, error) {
	listOptions := metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node.Name}).String(),
		LabelSelector: labels.SelectorFromSet(labels.Set{"k8s-app": "machine-config-daemon"}).String(),
	}

	mcds, err := c.CoreV1().Pods(namespaceMachineConfigOperator).List(listOptions)
	if err != nil {
		return nil, err
	}

	if len(mcds.Items) < 1 {
		return nil, fmt.Errorf("failed to get machine-config-daemon pod for the node %q", node.Name)
	}
	return &mcds.Items[0], nil
}

const (
	sysFSNumaNodePath = "/sys/devices/system/node"
)

func getContainerRshArgs(pod *corev1.Pod, cnt *corev1.Container) []string {
	return []string{
		"-n", pod.Namespace,
		"-c", cnt.Name,
		pod.Name,
	}
}

func getEnvironmentVariables(oc *exutil.CLI, pod *corev1.Pod, cnt *corev1.Container) (string, error) {
	initialArgs := getContainerRshArgs(pod, cnt)
	command := []string{"env"}
	args := append(initialArgs, command...)
	out, err := oc.AsAdmin().Run("rsh").Args(args...).Output()
	e2e.Logf("environment for pod %q container %q: %q", pod.Name, cnt.Name, out)
	return out, err
}

func getNumaNodeSysfsList(oc *exutil.CLI, pod *corev1.Pod, cnt *corev1.Container) (string, error) {
	initialArgs := getContainerRshArgs(pod, cnt)
	command := []string{"cat", "/sys/devices/system/node/online"}
	args := append(initialArgs, command...)
	out, err := oc.AsAdmin().Run("rsh").Args(args...).Output()
	e2e.Logf("NUMA nodes seen by pod %q container %q: %q", pod.Name, cnt.Name, out)
	return out, err
}

func getNumaNodeCountFromContainer(oc *exutil.CLI, pod *corev1.Pod, cnt *corev1.Container) (int, error) {
	out, err := getNumaNodeSysfsList(oc, pod, cnt)
	if err != nil {
		return 0, err
	}
	nodeNum, err := parseSysfsNodeOnline(out)
	if err != nil {
		return 0, err
	}
	e2e.Logf("NUMA nodes for pod %q container %q: count=%d", pod.Name, cnt.Name, nodeNum)
	return nodeNum, nil
}

func getAllowedCpuListForContainer(oc *exutil.CLI, pod *corev1.Pod, cnt *corev1.Container) (string, error) {
	initialArgs := getContainerRshArgs(pod, cnt)
	command := []string{
		"grep",
		"Cpus_allowed_list",
		"/proc/self/status",
	}
	args := append(initialArgs, command...)
	out, err := oc.AsAdmin().Run("rsh").Args(args...).Output()
	e2e.Logf("Allowed CPU list for pod %q container %q: %q", pod.Name, cnt.Name, out)
	return out, err
}

func makeAllowedCpuListEnv(out string) string {
	pair := strings.SplitN(out, ":", 2)
	return fmt.Sprintf("CPULIST_ALLOWED=%s\n", strings.TrimSpace(pair[1]))
}

// execCommandOnMachineConfigDaemon returns the output of the command execution on the machine-config-daemon pod that runs on the specified node
func execCommandOnMachineConfigDaemon(c clientset.Interface, oc *exutil.CLI, node *corev1.Node, command []string) (string, error) {
	mcd, err := getMachineConfigDaemonByNode(c, node)
	if err != nil {
		return "", err
	}

	initialArgs := []string{
		"-n", namespaceMachineConfigOperator,
		"-c", containerMachineConfigDaemon,
		"--request-timeout", "30",
		mcd.Name,
	}
	args := append(initialArgs, command...)
	return oc.AsAdmin().Run("rsh").Args(args...).Output()
}

// getKubeletConfig returns KubeletConfiguration loaded from the node /etc/kubernetes/kubelet.conf
func getKubeletConfig(c clientset.Interface, oc *exutil.CLI, node *corev1.Node) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
	command := []string{"cat", path.Join("/rootfs", filePathKubeletConfig)}
	kubeletData, err := execCommandOnMachineConfigDaemon(c, oc, node, command)
	if err != nil {
		return nil, err
	}

	e2e.Logf("command output: %s", kubeletData)
	kubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{}
	if err := yaml.Unmarshal([]byte(kubeletData), kubeletConfig); err != nil {
		return nil, err
	}
	return kubeletConfig, err
}

func parseSysfsNodeOnline(data string) (int, error) {
	/*
	    The file content is expected to be:
	   "0\n" in one-node case
	   "0-K\n" in N-node case where K=N-1
	*/
	info := strings.TrimSpace(data)
	pair := strings.SplitN(info, "-", 2)
	if len(pair) != 2 {
		return 1, nil
	}
	out, err := strconv.Atoi(pair[1])
	if err != nil {
		return 0, err
	}
	return out + 1, nil
}

func getNumaNodeCountFromNode(c clientset.Interface, oc *exutil.CLI, node *corev1.Node) (int, error) {
	command := []string{"cat", "/sys/devices/system/node/online"}
	out, err := execCommandOnMachineConfigDaemon(c, oc, node, command)
	if err != nil {
		return 0, err
	}

	e2e.Logf("command output: %s", out)
	nodeNum, err := parseSysfsNodeOnline(out)
	if err != nil {
		return 0, err
	}
	e2e.Logf("node %q NUMA nodes %d", node.Name, nodeNum)
	return nodeNum, nil
}

func makeBusyboxPod(namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: "test-",
			Labels: map[string]string{
				"test": "",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "test",
					Image:   "busybox",
					Command: []string{"sleep", "10h"},
				},
			},
		},
	}
}

func setNodeForPods(pods []*corev1.Pod, node *corev1.Node) {
	for _, pod := range pods {
		pod.Spec.NodeSelector = map[string]string{
			labelHostname: node.Name,
		}
	}

}

func createPods(client clientset.Interface, namespace string, testPods ...*corev1.Pod) []*corev1.Pod {
	updatedPods := make([]*corev1.Pod, len(testPods), len(testPods))
	var wg sync.WaitGroup

	for i := 0; i < len(testPods); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			created, err := client.CoreV1().Pods(namespace).Create(testPods[idx])
			e2e.ExpectNoError(err)

			err = waitForPhase(client, created.Namespace, created.Name, corev1.PodRunning, 5*time.Minute)
			e2e.ExpectNoError(err)

			updatedPods[idx], err = client.CoreV1().Pods(created.Namespace).Get(created.Name, metav1.GetOptions{})
			e2e.ExpectNoError(err)
		}(i)
	}
	wg.Wait()

	return updatedPods
}

func waitForPhase(c clientset.Interface, namespace, name string, phase corev1.PodPhase, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		updatedPod, err := c.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		e2e.Logf("pod %q phase %s", updatedPod.Name, updatedPod.Status.Phase)
		if updatedPod.Status.Phase == phase {
			return true, nil
		}
		return false, nil
	})
}

// GetSriovNicIPs returns the list of ip addresses related to the given
// interface name for the given pod.
func getSriovNicIPs(pod *corev1.Pod, ifcName string) ([]string, error) {
	// Needed for parsing of podinfo
	type Network struct {
		Interface string
		Ips       []string
	}

	var nets []Network
	err := json.Unmarshal([]byte(pod.ObjectMeta.Annotations["k8s.v1.cni.cncf.io/networks-status"]), &nets)
	if err != nil {
		return nil, err
	}
	for _, net := range nets {
		if net.Interface != ifcName {
			continue
		}
		return net.Ips, nil
	}
	return nil, nil
}

func getPodsIPAddrs(pods []*corev1.Pod, iface string) (map[string][]string, error) {
	ipOutput := make(map[string][]string)
	for _, pod := range pods {
		ips, err := getSriovNicIPs(pod, iface)
		if err != nil {
			return nil, err
		}
		ipOutput[pod.Name] = ips
		e2e.Logf("pod %q IP addresses %v", pod.Name, ips)
	}
	return ipOutput, nil
}

func pingAddrFromPod(oc *exutil.CLI, pod *corev1.Pod, cnt *corev1.Container, addr string) error {
	initialArgs := getContainerRshArgs(pod, cnt)
	command := []string{
		"ping",
		"-c",
		"3",
		addr,
	}
	args := append(initialArgs, command...)
	out, err := oc.AsAdmin().Run("rsh").Args(args...).Output()
	e2e.Logf("`%s` output for pod %q container %q: %q", strings.Join(command, " "), pod.Name, cnt.Name, out)
	return err
}

func findFirstIPForFamily(ips []string, family string) (string, error) {
	for _, ip := range ips {
		addr := net.ParseIP(ip)
		if addr == nil {
			return "", fmt.Errorf("cannot parse IP %q", ip)
		}
		if family == "v6" && addr.To16() != nil {
			return ip, nil
		}
		if family == "v4" && addr.To4() != nil {
			return ip, nil
		}
	}
	return "", fmt.Errorf("IP address for family %q not found in %v", family, ips)
}
