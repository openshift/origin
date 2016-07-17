package network

import (
	"errors"
	"fmt"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcontainer "k8s.io/kubernetes/pkg/kubelet/container"
	kexec "k8s.io/kubernetes/pkg/util/exec"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/diagnostics/types"
	"github.com/openshift/origin/pkg/sdn/api"
	sdnplugin "github.com/openshift/origin/pkg/sdn/plugin"
	"github.com/openshift/origin/pkg/util/netutils"
)

const (
	CheckPodNetworkName = "CheckPodNetwork"
)

// CheckPodNetwork is a Diagnostic to check communication between pods in the cluster.
type CheckPodNetwork struct {
	KubeClient *kclient.Client
	OSClient   *osclient.Client
}

// Name is part of the Diagnostic interface and just returns name.
func (d CheckPodNetwork) Name() string {
	return CheckPodNetworkName
}

// Description is part of the Diagnostic interface and just returns the diagnostic description.
func (d CheckPodNetwork) Description() string {
	return "Check pod to pod communication in the cluster. In case of ovs-subnet network plugin, all pods should be able to communicate with each other and in case of multitenant network plugin, pods in non-global projects should be isolated and pods in global projects should be able to access any pod in the cluster and vice versa."
}

// CanRun is part of the Diagnostic interface; it determines if the conditions are right to run this diagnostic.
func (d CheckPodNetwork) CanRun() (bool, error) {
	if d.KubeClient == nil {
		return false, errors.New("must have kube client")
	} else if d.OSClient == nil {
		return false, errors.New("must have openshift client")
	}
	return true, nil
}

// Check is part of the Diagnostic interface; it runs the actual diagnostic logic
func (d CheckPodNetwork) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(CheckPodNetworkName)

	pluginName, ok, err := getOpenShiftNetworkPlugin(d.OSClient)
	if err != nil {
		r.Error("DPodNet1001", err, fmt.Sprintf("Checking network plugin failed. Error: %s", err))
		return r
	}
	if !ok {
		r.Warn("DPodNet1002", nil, fmt.Sprintf("Skipping pod connectivity test. Reason: Not using openshift network plugin."))
		return r
	}

	localPods, nonlocalPods, err := getLocalAndNonLocalPods(d.KubeClient)
	if err != nil {
		r.Error("DPodNet1003", err, fmt.Sprintf("Getting local and nonlocal pods failed. Error: %s", err))
		return r
	}

	vnidMap := map[string]uint32{}
	if sdnplugin.IsOpenShiftMultitenantNetworkPlugin(pluginName) {
		netnsList, err := d.OSClient.NetNamespaces().List(kapi.ListOptions{})
		if err != nil {
			r.Error("DPodNet1004", err, fmt.Sprintf("Getting all network namespaces failed. Error: %s", err))
			return r
		}

		for _, netns := range netnsList.Items {
			vnidMap[netns.NetName] = netns.NetID
		}
	}

	localGlobalPods, localNonGlobalPods := getGlobalAndNonGlobalPods(localPods, vnidMap)
	nonlocalGlobalPods, nonlocalNonGlobalPods := getGlobalAndNonGlobalPods(nonlocalPods, vnidMap)

	checkSameNodePodToPodConnection(localGlobalPods, localNonGlobalPods, vnidMap, r)
	checkDifferentNodePodToPodConnection(localGlobalPods, localNonGlobalPods, nonlocalGlobalPods, nonlocalNonGlobalPods, vnidMap, r)
	return r
}

func getGlobalAndNonGlobalPods(pods []kapi.Pod, vmap map[string]uint32) ([]kapi.Pod, []kapi.Pod) {
	globalPods := []kapi.Pod{}
	nonGlobalPods := []kapi.Pod{}

	for _, pod := range pods {
		if len(vmap) > 0 && vmap[pod.Namespace] == api.GlobalVNID {
			globalPods = append(globalPods, pod)
		} else {
			nonGlobalPods = append(nonGlobalPods, pod)
		}
	}
	return globalPods, nonGlobalPods
}

func getOpenShiftNetworkPlugin(osClient *osclient.Client) (string, bool, error) {
	cn, err := osClient.ClusterNetwork().Get(api.ClusterNetworkDefault)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return cn.PluginName, sdnplugin.IsOpenShiftNetworkPlugin(cn.PluginName), nil
}

func getLocalAndNonLocalPods(kubeClient *kclient.Client) ([]kapi.Pod, []kapi.Pod, error) {
	pods, err := getPods(kubeClient)
	if err != nil {
		return nil, nil, err
	}

	localIP, err := getLocalNodeIP(kubeClient)
	if err != nil {
		return nil, nil, err
	}

	localPods := []kapi.Pod{}
	nonlocalPods := []kapi.Pod{}
	for _, pod := range pods {
		if pod.Status.HostIP == localIP {
			localPods = append(localPods, pod)
		} else {
			nonlocalPods = append(nonlocalPods, pod)
		}
	}
	return localPods, nonlocalPods, nil
}

func getLocalNodeIP(kubeClient *kclient.Client) (string, error) {
	nodeList, err := kubeClient.Nodes().List(kapi.ListOptions{})
	if err != nil {
		return "", err
	}

	_, hostIPs, err := netutils.GetHostIPNetworks(nil)
	if err != nil {
		return "", err
	}
	for _, node := range nodeList.Items {
		if len(node.Status.Addresses) == 0 {
			continue
		}
		for _, ip := range hostIPs {
			for _, addr := range node.Status.Addresses {
				if addr.Type == kapi.NodeInternalIP && ip.String() == addr.Address {
					return addr.Address, nil
				}
			}
		}
	}
	return "", fmt.Errorf("unable to find local node IP")
}

func getPods(kubeClient *kclient.Client) ([]kapi.Pod, error) {
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

func checkDifferentNodePodToPodConnection(localGlobalPods, localNonGlobalPods, nonlocalGlobalPods, nonlocalNonGlobalPods []kapi.Pod, vmap map[string]uint32, r types.DiagnosticResult) {
	// Applicable to flat and multitenant networks
	if len(localNonGlobalPods) > 0 && len(nonlocalNonGlobalPods) > 0 {
		checkPodToPodConnection(localNonGlobalPods[0], nonlocalNonGlobalPods[0], vmap, r)
	} else {
		r.Warn("DPodNet1005", nil, fmt.Sprintf("Skipping pod connectivity test for non-global projects on different nodes. Reason: Couldn't find 2 non-global pods."))
	}
	if len(vmap) == 0 {
		return
	}

	// Applicable to multitenant network
	if len(localGlobalPods) > 0 && len(nonlocalGlobalPods) > 0 {
		checkPodToPodConnection(localGlobalPods[0], nonlocalGlobalPods[0], vmap, r)
	} else {
		r.Warn("DPodNet1006", nil, fmt.Sprintf("Skipping pod connectivity test for global projects on different nodes. Reason: Couldn't find 2 global pods."))
	}

	if len(localGlobalPods) > 0 && len(nonlocalNonGlobalPods) > 0 {
		checkPodToPodConnection(localGlobalPods[0], nonlocalNonGlobalPods[0], vmap, r)
	} else {
		r.Warn("DPodNet1007", nil, fmt.Sprintf("Skipping pod connectivity test between global to non-global projects on different nodes. Reason: Couldn't find global and/or non-global pod."))
	}
}

func checkSameNodePodToPodConnection(globalPods, nonGlobalPods []kapi.Pod, vmap map[string]uint32, r types.DiagnosticResult) {
	// Applicable to flat and multitenant networks
	if len(nonGlobalPods) >= 2 {
		checkPodToPodConnection(nonGlobalPods[0], nonGlobalPods[1], vmap, r)
	} else {
		r.Warn("DPodNet1008", nil, fmt.Sprintf("Skipping pod connectivity test for non-global projects on the same node. Reason: Couldn't find 2 non-global pods."))
	}
	if len(vmap) == 0 {
		return
	}

	// Applicable to multitenant network
	if len(globalPods) >= 2 {
		checkPodToPodConnection(globalPods[0], globalPods[1], vmap, r)
	} else {
		r.Warn("DPodNet1009", nil, fmt.Sprintf("Skipping pod connectivity test for global projects on the same node. Reason: Couldn't find 2 global pods."))
	}

	if len(globalPods) > 0 && len(nonGlobalPods) > 0 {
		checkPodToPodConnection(globalPods[0], nonGlobalPods[0], vmap, r)
	} else {
		r.Warn("DPodNet1010", nil, fmt.Sprintf("Skipping pod connectivity test between global to non-global projects on the same node. Reason: Couldn't find global and/or non-global pod."))
	}
}

// Determine expected connection status for the given pods
// true indicates success and false means failure
func expConnStatus(ns1, ns2 string, vmap map[string]uint32) bool {
	// Check if sdn is flat network
	if len(vmap) == 0 {
		return true
	} // else multitenant

	// Check if one of the pods belongs to global network
	if vmap[ns1] == api.GlobalVNID || vmap[ns2] == api.GlobalVNID {
		return true
	}

	// Check if both the pods are sharing the network
	if vmap[ns1] == vmap[ns2] {
		return true
	}

	// Isolated network
	return false
}

func printPod(pod kapi.Pod) string {
	return fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
}

// checkPodToPodConnection verifies connection from fromPod to toPod.
// Connection check from toPod to fromPod will be done by the node of toPod.
func checkPodToPodConnection(fromPod, toPod kapi.Pod, vmap map[string]uint32, r types.DiagnosticResult) {
	if len(fromPod.Status.ContainerStatuses) <= 0 {
		r.Error("DPodNet1011", nil, fmt.Sprintf("ContainerID not found for pod %q", printPod(fromPod)))
		return
	}

	success := expConnStatus(fromPod.Namespace, toPod.Namespace, vmap)

	kexecer := kexec.New()
	containerID := kcontainer.ParseContainerID(fromPod.Status.ContainerStatuses[0].ContainerID).ID
	pid, err := kexecer.Command("docker", "inspect", "-f", "{{.State.Pid}}", containerID).CombinedOutput()
	if err != nil {
		r.Error("DPodNet1012", err, fmt.Sprintf("Fetching pid for pod %q, container %q failed. Error: %s", printPod(fromPod), containerID, err))
		return
	}

	out, err := kexecer.Command("nsenter", "-n", "-t", strings.Trim(fmt.Sprintf("%s", pid), "\n"), "--", "ping", "-c1", "-W1", toPod.Status.PodIP).CombinedOutput()
	if success && err != nil {
		r.Error("DPodNet1013", err, fmt.Sprintf("Connectivity from pod %q to pod %q failed. Error: %s, Out: %s", printPod(fromPod), printPod(toPod), err, string(out)))
	} else if !success && err == nil {
		msg := fmt.Sprintf("Unexpected connectivity from pod %q to pod %q.", printPod(fromPod), printPod(toPod))
		r.Error("DPodNet1014", fmt.Errorf("%s", msg), msg)
	}
}
