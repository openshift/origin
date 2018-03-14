package util

import (
	"fmt"
	"io"
	"strings"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	networktypedclient "github.com/openshift/origin/pkg/network/generated/internalclientset/typed/network/internalversion"
	osclientcmd "github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/openshift/origin/pkg/util/netutils"
)

const (
	NetworkDiagNamespacePrefix       = "network-diag-ns"
	NetworkDiagGlobalNamespacePrefix = "network-diag-global-ns"
	NetworkDiagPodNamePrefix         = "network-diag-pod"
	NetworkDiagSCCNamePrefix         = "network-diag-privileged"
	NetworkDiagSecretName            = "network-diag-secret"

	NetworkDiagTestPodNamePrefix      = "network-diag-test-pod"
	NetworkDiagTestServiceNamePrefix  = "network-diag-test-service"
	NetworkDiagContainerMountPath     = "/host"
	NetworkDiagDefaultLogDir          = "/tmp/openshift/"
	NetworkDiagNodeLogDirPrefix       = "/nodes"
	NetworkDiagMasterLogDirPrefix     = "/master"
	NetworkDiagPodLogDirPrefix        = "/pods"
	NetworkDiagDefaultTestPodProtocol = string(kapi.ProtocolTCP)
	NetworkDiagDefaultTestPodPort     = 8080
)

func trimRegistryPath(image string) string {
	// Image format could be: [<dns-name>/]openshift/origin-deployer[:<tag>]
	// Return image without registry dns: openshift/origin-deployer[:<tag>]
	tokens := strings.Split(image, "/")
	sz := len(tokens)
	trimmedImage := image
	if sz >= 2 {
		trimmedImage = fmt.Sprintf("%s/%s", tokens[sz-2], tokens[sz-1])
	}
	return trimmedImage
}

func GetNetworkDiagDefaultPodImage() string {
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = variable.DefaultImagePrefix + ":${version}"
	image := imageTemplate.ExpandOrDie("")
	return trimRegistryPath(image)
}

func GetNetworkDiagDefaultTestPodImage() string {
	imageTemplate := variable.NewDefaultImageTemplate()
	image := imageTemplate.ExpandOrDie("deployer")
	return trimRegistryPath(image)
}

func GetOpenShiftNetworkPlugin(clusterNetworkClient networktypedclient.ClusterNetworksGetter) (string, bool, error) {
	cn, err := clusterNetworkClient.ClusterNetworks().Get(networkapi.ClusterNetworkDefault, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return cn.PluginName, network.IsOpenShiftNetworkPlugin(cn.PluginName), nil
}

func GetNodes(kubeClient kclientset.Interface) ([]kapi.Node, error) {
	nodeList, err := kubeClient.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Listing nodes in the cluster failed. Error: %s", err)
	}
	return nodeList.Items, nil
}

func GetSchedulableNodes(kubeClient kclientset.Interface) ([]kapi.Node, error) {
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

func GetLocalNode(kubeClient kclientset.Interface) (string, string, error) {
	nodeList, err := kubeClient.Core().Nodes().List(metav1.ListOptions{})
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
func GetLocalAndNonLocalDiagnosticPods(kubeClient kclientset.Interface) ([]kapi.Pod, []kapi.Pod, error) {
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
		if vnidMap[pod.Namespace] == network.GlobalVNID {
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
	if vnidMap[ns1] == network.GlobalVNID || vnidMap[ns2] == network.GlobalVNID {
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
	client, err := factory.ClientSet()
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
		Executor:  &kubecmd.DefaultRemoteExecutor{},
		PodClient: client.Core(),
		Config:    config,
		Command:   command,
	}
	err = execOptions.Validate()
	if err != nil {
		return err
	}
	return execOptions.Run()
}

func getSDNRunningPods(kubeClient kclientset.Interface) ([]kapi.Pod, error) {
	podList, err := kubeClient.Core().Pods(metav1.NamespaceAll).List(metav1.ListOptions{})
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
