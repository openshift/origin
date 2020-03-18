package topologymanager

import (
	"fmt"
	"os"
	"path"
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

	g "github.com/onsi/ginkgo"
)

const (
	strictCheckEnvVar = "TOPOLOGY_MANAGER_TEST_STRICT"

	defaultRoleWorker        = "worker"
	defaultMachineConfigPool = "worker"

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

func getMachineConfigPoolName() string {
	mcpName := defaultMachineConfigPool
	if mn, ok := os.LookupEnv("MACHINE_CONFIG_POOL"); ok {
		mcpName = mn
	}
	e2e.Logf("machine config pool: %q", mcpName)
	return mcpName
}

func getRoleWorkerLabel() string {
	roleWorker := defaultRoleWorker
	if rw, ok := os.LookupEnv("ROLE_WORKER"); ok {
		roleWorker = rw
	}
	e2e.Logf("role worker: %q", roleWorker)
	return roleWorker
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

func getNumaNodeSysfsList(oc *exutil.CLI, pod *corev1.Pod, cnt *corev1.Container) (string, error) {
	initialArgs := []string{
		"-n", pod.Namespace,
		"-c", cnt.Name,
		pod.Name,
	}
	command := []string{
		"find",
		"/sys/devices/system/node",
		"-type", "d",
		"-name", "node*",
		"-print",
	}
	args := append(initialArgs, command...)
	return oc.AsAdmin().Run("rsh").Args(args...).Output()
}

func getNumaNodeCount(oc *exutil.CLI, pod *corev1.Pod, cnt *corev1.Container) (int, error) {
	out, err := getNumaNodeSysfsList(oc, pod, cnt)
	if err != nil {
		return 0, err
	}
	nodes := strings.Split(out, "\n")
	e2e.Logf("out=%q nodes=%v", out, nodes)
	// the first entry find returns is the top level dire. We will have at least 1 NUMA node, so this is safe
	nodeNum := len(nodes) - 1
	e2e.Logf("pod %q cnt %q NUMA nodes %d", pod.Name, cnt.Name, nodeNum)
	return nodeNum, nil
}

func getAllowedCpuListForContainer(oc *exutil.CLI, pod *corev1.Pod, cnt *corev1.Container) (string, error) {
	initialArgs := []string{
		"-n", pod.Namespace,
		"-c", cnt.Name,
		pod.Name,
	}
	command := []string{
		"grep",
		"Cpus_allowed_list",
		"/proc/self/status",
	}
	args := append(initialArgs, command...)
	return oc.AsAdmin().Run("rsh").Args(args...).Output()
}

func makeAllowedCpuListEnv(out string) string {
	pair := strings.SplitN(out, ":", 2)
	return fmt.Sprintf("CPULIST_ALLOWED=%s\n", strings.TrimSpace(pair[1]))
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

// CAUTION: this breaks completely if tests are run in parallel

var testCounter int = 0

func BeforeAll(fn func()) {
	g.BeforeEach(func() {
		if testCounter == 0 {
			fn()
		}
		testCounter++
	})
}

func AfterAll(fn func()) {
	g.AfterEach(func() {
		testCounter--
		if testCounter == 0 {
			fn()
		}
	})
}
