package util

import (
	"fmt"
	"io"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"

	osclient "github.com/openshift/origin/pkg/client"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/sdn/api"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/util/netutils"
)

const (
	NetworkDiagNamespacePrefix       = "network-diag-ns"
	NetworkDiagGlobalNamespacePrefix = "network-diag-global-ns"
	NetworkDiagPodNamePrefix         = "network-diag-pod"
	NetworkDiagSCCNamePrefix         = "network-diag-privileged"
	NetworkDiagSecretName            = "network-diag-secret"

	NetworkDiagTestPodNamePrefix     = "network-diag-test-pod"
	NetworkDiagTestServiceNamePrefix = "network-diag-test-service"
	NetworkDiagContainerMountPath    = "/host"
	NetworkDiagDefaultLogDir         = "/tmp/openshift/"
	NetworkDiagNodeLogDirPrefix      = "/nodes"
	NetworkDiagMasterLogDirPrefix    = "/master"
	NetworkDiagPodLogDirPrefix       = "/pods"
)

func GetOpenShiftNetworkPlugin(osClient *osclient.Client) (string, bool, error) {
	cn, err := osClient.ClusterNetwork().Get(api.ClusterNetworkDefault)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return cn.PluginName, sdnapi.IsOpenShiftNetworkPlugin(cn.PluginName), nil
}

func GetNodes(kubeClient *kclient.Client) ([]kapi.Node, error) {
	nodeList, err := kubeClient.Nodes().List(kapi.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Listing nodes in the cluster failed. Error: %s", err)
	}
	return nodeList.Items, nil
}

func GetSchedulableNodes(kubeClient *kclient.Client) ([]kapi.Node, error) {
	filteredNodes := []kapi.Node{}
	nodes, err := GetNodes(kubeClient)
	if err != nil {
		return filteredNodes, err
	}

	for _, node := range nodes {
		// Skip if node is not schedulable
		if node.Spec.Unschedulable {
			continue
		}

		ready := kapi.ConditionUnknown
		// Get node ready status
		for _, condition := range node.Status.Conditions {
			if condition.Type == kapi.NodeReady {
				ready = condition.Status
				break
			}
		}

		// Skip if node is not ready
		if ready != kapi.ConditionTrue {
			continue
		}
		filteredNodes = append(filteredNodes, node)
	}
	return filteredNodes, nil
}

func GetLocalNode(kubeClient *kclient.Client) (string, string, error) {
	nodeList, err := kubeClient.Nodes().List(kapi.ListOptions{})
	if err != nil {
		return "", "", err
	}

	_, hostIPs, err := netutils.GetHostIPNetworks(nil)
	if err != nil {
		return "", "", err
	}
	for _, node := range nodeList.Items {
		if len(node.Status.Addresses) == 0 {
			continue
		}
		for _, ip := range hostIPs {
			for _, addr := range node.Status.Addresses {
				if addr.Type == kapi.NodeInternalIP && ip.String() == addr.Address {
					return node.Name, addr.Address, nil
				}
			}
		}
	}
	return "", "", fmt.Errorf("unable to find local node IP")
}

// Get local/non-local pods in network diagnostic namespaces
func GetLocalAndNonLocalDiagnosticPods(kubeClient *kclient.Client) ([]kapi.Pod, []kapi.Pod, error) {
	pods, err := getSDNRunningPods(kubeClient)
	if err != nil {
		return nil, nil, err
	}

	_, localIP, err := GetLocalNode(kubeClient)
	if err != nil {
		return nil, nil, err
	}

	localPods := []kapi.Pod{}
	nonlocalPods := []kapi.Pod{}
	for _, pod := range pods {
		if strings.HasPrefix(pod.Namespace, NetworkDiagNamespacePrefix) || strings.HasPrefix(pod.Namespace, NetworkDiagGlobalNamespacePrefix) {
			if pod.Status.HostIP == localIP {
				localPods = append(localPods, pod)
			} else {
				nonlocalPods = append(nonlocalPods, pod)
			}
		}
	}
	return localPods, nonlocalPods, nil
}

func PrintPod(pod *kapi.Pod) string {
	return fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
}

func GetGlobalAndNonGlobalPods(pods []kapi.Pod, vnidMap map[string]uint32) ([]kapi.Pod, []kapi.Pod) {
	if vnidMap == nil {
		return pods, nil
	}

	globalPods := []kapi.Pod{}
	nonGlobalPods := []kapi.Pod{}
	for _, pod := range pods {
		if vnidMap[pod.Namespace] == api.GlobalVNID {
			globalPods = append(globalPods, pod)
		} else {
			nonGlobalPods = append(nonGlobalPods, pod)
		}
	}
	return globalPods, nonGlobalPods
}

// Determine expected connection status for the given pods
// true indicates success and false means failure
func ExpectedConnectionStatus(ns1, ns2 string, vnidMap map[string]uint32) bool {
	// Check if sdn is flat network
	if vnidMap == nil {
		return true
	} // else multitenant

	// Check if one of the pods belongs to global network
	if vnidMap[ns1] == api.GlobalVNID || vnidMap[ns2] == api.GlobalVNID {
		return true
	}

	// Check if both the pods are sharing the network
	if vnidMap[ns1] == vnidMap[ns2] {
		return true
	}

	// Isolated network
	return false
}

// Execute() will run a command in a pod and streams the out/err
func Execute(factory *osclientcmd.Factory, command []string, pod *kapi.Pod, in io.Reader, out, errOut io.Writer) error {
	config, err := factory.ClientConfig()
	if err != nil {
		return err
	}
	client, err := factory.Client()
	if err != nil {
		return err
	}

	execOptions := &kubecmd.ExecOptions{
		StreamOptions: kubecmd.StreamOptions{
			Namespace:     pod.Namespace,
			PodName:       pod.Name,
			ContainerName: pod.Name,
			In:            in,
			Out:           out,
			Err:           errOut,
			Stdin:         in != nil,
		},
		Executor: &kubecmd.DefaultRemoteExecutor{},
		Client:   client,
		Config:   config,
		Command:  command,
	}
	err = execOptions.Validate()
	if err != nil {
		return err
	}
	return execOptions.Run()
}

func getSDNRunningPods(kubeClient *kclient.Client) ([]kapi.Pod, error) {
	podList, err := kubeClient.Pods(kapi.NamespaceAll).List(kapi.ListOptions{})
	if err != nil {
		return nil, err
	}

	filtered_pods := []kapi.Pod{}
	for _, pod := range podList.Items {
		// Skip pods that are not running
		if pod.Status.Phase != kapi.PodRunning {
			continue
		}

		// Skip pods with hostNetwork enabled
		if pod.Spec.SecurityContext.HostNetwork {
			continue
		}
		filtered_pods = append(filtered_pods, pod)
	}
	return filtered_pods, nil
}
