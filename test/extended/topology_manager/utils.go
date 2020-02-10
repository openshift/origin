package topologymanager

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
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
)

const (
	strictCheckEnvVar = "TOPOLOGY_MANAGER_TEST_STRICT"

	defaultRoleWorker = "worker"

	namespaceMachineConfigOperator = "openshift-machine-config-operator"
	containerMachineConfigDaemon   = "machine-config-daemon"

	featureGateTopologyManager = "TopologyManager"
)

const (
	labelRole     = "node-role.kubernetes.io"
	labelHostname = "kubernetes.io/hostname"
)

const (
	filePathKubeletConfig = "/etc/kubernetes/kubelet.conf"
	filePathKubePodsSlice = "/sys/fs/cgroup/cpuset/kubepods.slice"
	filePathSysCPU        = "/sys/devices/system/cpu"
)

func getRoleWorkerLabel() string {
	roleWorker := defaultRoleWorker
	if rw, ok := os.LookupEnv("ROLE_WORKER"); ok {
		roleWorker = rw
	}
	e2e.Logf("role worker: %q", roleWorker)
	return roleWorker
}

func filterNodeWithTopologyManagerPolicy(workerNodes []corev1.Node, client clientset.Interface, oc *exutil.CLI, policy string) []corev1.Node {
	ocRaw := (*oc).WithoutNamespace()

	var topoMgrNodes []corev1.Node

	for _, node := range workerNodes {
		kubeletConfig, err := getKubeletConfig(client, ocRaw, &node)
		e2e.ExpectNoError(err)

		e2e.Logf("kubelet %s CPU Manager policy: %q", node.Name, kubeletConfig.CPUManagerPolicy)

		// verify topology manager feature gate
		if enabled, ok := kubeletConfig.FeatureGates[featureGateTopologyManager]; !ok || !enabled {
			e2e.Logf("kubelet %s Topology Manager FeatureGate not enabled", node.Name)
			continue
		}

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

// ExecCommandOnMachineConfigDaemon returns the output of the command execution on the machine-config-daemon pod that runs on the specified node
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

// GetKubeletConfig returns KubeletConfiguration loaded from the node /etc/kubernetes/kubelet.conf
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

func filterNodeByResource(nodes []corev1.Node, resourceName string) []corev1.Node {
	resource := corev1.ResourceName(resourceName)
	nodesWithResource := []corev1.Node{}
	for _, node := range nodes {
		for name, quantity := range node.Status.Allocatable {
			if name == resource && !quantity.IsZero() {
				nodesWithResource = append(nodesWithResource, node)
			}
		}
	}
	return nodesWithResource
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

func createPodsOnNodeSync(client clientset.Interface, namespace string, node *corev1.Node, testPods ...*corev1.Pod) []*corev1.Pod {
	var updatedPods []*corev1.Pod
	for _, testPod := range testPods {
		if node != nil {
			testPod.Spec.NodeSelector = map[string]string{
				labelHostname: node.Name,
			}
		}

		created, err := client.CoreV1().Pods(namespace).Create(testPod)
		e2e.ExpectNoError(err)

		err = waitForPhase(client, created.Namespace, created.Name, corev1.PodRunning, 5*time.Minute)

		updatedPod, err := client.CoreV1().Pods(created.Namespace).Get(created.Name, metav1.GetOptions{})
		e2e.ExpectNoError(err)

		updatedPods = append(updatedPods, updatedPod)
	}
	return updatedPods
}

func findNodeHostingPod(nodes []corev1.Node, pod *corev1.Pod) *corev1.Node {
	for _, node := range nodes {
		ipAddr, ok := findNodeInternalIpAddr(node)
		if !ok {
			continue
		}
		if ipAddr == pod.Status.HostIP {
			return &node
		}
	}
	return nil
}

func findNodeInternalIpAddr(node corev1.Node) (string, bool) {
	for _, nodeAddr := range node.Status.Addresses {
		if nodeAddr.Type == corev1.NodeInternalIP {
			return nodeAddr.Address, true
		}

	}
	e2e.Logf("node %q lacks IP address? %v", node.Name, node.Status.Addresses)
	return "", false
}

func waitForPhase(c clientset.Interface, namespace, name string, phase corev1.PodPhase, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		updatedPod, err := c.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if updatedPod.Status.Phase == phase {
			return true, nil
		}
		return false, nil
	})
}

func execCommandOnPod(oc *exutil.CLI, pod *corev1.Pod, command []string) (string, error) {
	initialArgs := []string{
		"-n", pod.Namespace,
		pod.Name,
	}
	args := append(initialArgs, command...)
	return oc.AsAdmin().Run("rsh").Args(args...).Output()
}

func getContainerCPUSet(c clientset.Interface, oc *exutil.CLI, node *corev1.Node, pod *corev1.Pod, containerIdx int) ([]string, error) {
	podDir := fmt.Sprintf("kubepods-pod%s.slice", strings.ReplaceAll(string(pod.UID), "-", "_"))

	containerID := strings.Trim(pod.Status.ContainerStatuses[containerIdx].ContainerID, "cri-o://")
	containerDir := fmt.Sprintf("crio-%s.scope", containerID)

	// we will use machine-config-daemon to get all information from the node, because it has
	// mounted node filesystem under /rootfs
	command := []string{
		"cat",
		path.Join("/rootfs", filePathKubePodsSlice, podDir, containerDir, "cpuset.cpus"),
	}
	cpuSet, err := execCommandOnMachineConfigDaemon(c, oc, node, command)
	if err != nil {
		return nil, err
	}

	results := []string{}
	for _, cpuRange := range strings.Split(string(cpuSet), ",") {
		if strings.Contains(cpuRange, "-") {
			seq := strings.Split(cpuRange, "-")
			if len(seq) != 2 {
				return nil, fmt.Errorf("incorrect CPU range: %q", cpuRange)
			}
			// we will iterate over runes, so we should specify [0] to get it from string
			for i := seq[0][0]; i <= seq[1][0]; i++ {
				results = append(results, strings.Trim(string(i), "\n"))
			}
			continue
		}
		results = append(results, strings.Trim(cpuRange, "\n"))
	}
	return results, nil
}

func getCPUSetNumaNodes(c clientset.Interface, oc *exutil.CLI, node *corev1.Node, cpuSet []string) ([]string, error) {
	numaNodes := []string{}
	for _, cpuID := range cpuSet {
		cpuPath := path.Join("/rootfs", filePathSysCPU, "cpu"+cpuID)
		cpuDirContent, err := execCommandOnMachineConfigDaemon(c, oc, node, []string{"ls", cpuPath})
		if err != nil {
			return nil, err
		}
		re := regexp.MustCompile(`node(\d+)`)
		match := re.FindStringSubmatch(string(cpuDirContent))
		if len(match) != 2 {
			return nil, fmt.Errorf("incorrect match for 'ls' command: %v", match)
		}
		numaNodes = append(numaNodes, match[1])
	}
	return numaNodes, nil
}
