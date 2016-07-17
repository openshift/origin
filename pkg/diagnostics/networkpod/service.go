package network

import (
	"errors"
	"fmt"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcontainer "k8s.io/kubernetes/pkg/kubelet/container"
	kexec "k8s.io/kubernetes/pkg/util/exec"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/diagnostics/types"
	sdnplugin "github.com/openshift/origin/pkg/sdn/plugin"
)

const (
	CheckServiceNetworkName = "CheckServiceNetwork"
)

// CheckServiceNetwork is a Diagnostic to check communication between services in the cluster.
type CheckServiceNetwork struct {
	KubeClient *kclient.Client
	OSClient   *osclient.Client
}

// Name is part of the Diagnostic interface and just returns name.
func (d CheckServiceNetwork) Name() string {
	return CheckServiceNetworkName
}

// Description is part of the Diagnostic interface and just returns the diagnostic description.
func (d CheckServiceNetwork) Description() string {
	return "Check pod to service communication in the cluster. In case of ovs-subnet network plugin, all pods should be able to communicate with all services and in case of multitenant network plugin, services in non-global projects should be isolated and pods in global projects should be able to access any service in the cluster."
}

// CanRun is part of the Diagnostic interface; it determines if the conditions are right to run this diagnostic.
func (d CheckServiceNetwork) CanRun() (bool, error) {
	if d.KubeClient == nil {
		return false, errors.New("must have kube client")
	} else if d.OSClient == nil {
		return false, errors.New("must have openshift client")
	}
	return true, nil
}

// Check is part of the Diagnostic interface; it runs the actual diagnostic logic
func (d CheckServiceNetwork) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(CheckServiceNetworkName)

	pluginName, ok, err := getOpenShiftNetworkPlugin(d.OSClient)
	if err != nil {
		r.Error("DSvcNet1001", err, fmt.Sprintf("Checking network plugin failed. Error: %s", err))
		return r
	}
	if !ok {
		r.Warn("DSvcNet1002", nil, fmt.Sprintf("Skipping service connectivity test. Reason: Not using openshift network plugin."))
		return r
	}

	services, err := getAllServices(d.KubeClient)
	if err != nil {
		r.Error("DSvcNet1003", err, fmt.Sprintf("Getting all services failed. Error: %s", err))
		return r
	}
	if len(services) == 0 {
		r.Warn("DSvcNet1004", nil, fmt.Sprintf("Skipping service connectivity test. Reason: No services found."))
		return r
	}

	localPods, _, err := getLocalAndNonLocalPods(d.KubeClient)
	if err != nil {
		r.Error("DSvcNet1005", err, fmt.Sprintf("Getting local and nonlocal pods failed. Error: %s", err))
		return r
	}

	vnidMap := map[string]uint32{}
	if sdnplugin.IsOpenShiftMultitenantNetworkPlugin(pluginName) {
		netnsList, err := d.OSClient.NetNamespaces().List(kapi.ListOptions{})
		if err != nil {
			r.Error("DSvcNet1006", err, fmt.Sprintf("Getting all network namespaces failed. Error: %s", err))
			return r
		}

		for _, netns := range netnsList.Items {
			vnidMap[netns.NetName] = netns.NetID
		}
	}

	localGlobalPods, localNonGlobalPods := getGlobalAndNonGlobalPods(localPods, vnidMap)

	// Applicable to flat and multitenant networks
	if len(localNonGlobalPods) > 0 {
		checkPodToServiceConnection(localNonGlobalPods[0], services[0], vnidMap, r)
	} else {
		r.Warn("DSvcNet1007", nil, fmt.Sprintf("Skipping service connectivity test for non-global projects. Reason: Couldn't find a non-global pod."))
	}
	if len(vnidMap) == 0 {
		return r
	}

	// Applicable to multitenant network
	if len(localGlobalPods) > 0 {
		checkPodToServiceConnection(localGlobalPods[0], services[0], vnidMap, r)
	} else {
		r.Warn("DSvcNet1008", nil, fmt.Sprintf("Skipping service connectivity test for global projects. Reason: Couldn't find a global pod."))
	}
	return r
}

func getAllServices(kubeClient *kclient.Client) ([]kapi.Service, error) {
	filtered_srvs := []kapi.Service{}
	serviceList, err := kubeClient.Services(kapi.NamespaceAll).List(kapi.ListOptions{})
	if err != nil {
		return filtered_srvs, err
	}

	for _, srv := range serviceList.Items {
		if len(srv.Spec.ClusterIP) == 0 || srv.Spec.ClusterIP == kapi.ClusterIPNone {
			continue
		}
		filtered_srvs = append(filtered_srvs, srv)
	}
	return filtered_srvs, nil
}

func printService(svc kapi.Service) string {
	return fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
}

func checkPodToServiceConnection(fromPod kapi.Pod, toService kapi.Service, vmap map[string]uint32, r types.DiagnosticResult) {
	if len(fromPod.Status.ContainerStatuses) <= 0 {
		r.Error("DSvcNet1009", nil, fmt.Sprintf("ContainerID not found for pod %q", printPod(fromPod)))
		return
	}

	success := expConnStatus(fromPod.Namespace, toService.Namespace, vmap)

	kexecer := kexec.New()
	containerID := kcontainer.ParseContainerID(fromPod.Status.ContainerStatuses[0].ContainerID).ID
	pid, err := kexecer.Command("docker", "inspect", "-f", "{{.State.Pid}}", containerID).CombinedOutput()
	if err != nil {
		r.Error("DSvcNet1010", err, fmt.Sprintf("Fetching pid for pod %q failed. Error: %s", printPod(fromPod), err))
		return
	}

	// In bash, redirecting to /dev/tcp/HOST/PORT or /dev/udp/HOST/PORT opens a connection
	// to that HOST:PORT. Use this to test connectivity to the service; we can't use ping
	// like in the pod connectivity check because only connections to the correct port
	// get redirected by the iptables rules.
	srvConCmd := fmt.Sprintf("echo -n '' > /dev/%s/%s/%d", strings.ToLower(string(toService.Spec.Ports[0].Protocol)), toService.Spec.ClusterIP, toService.Spec.Ports[0].Port)
	out, err := kexecer.Command("nsenter", "-n", "-t", strings.Trim(fmt.Sprintf("%s", pid), "\n"), "--", "timeout", "1", "bash", "-c", srvConCmd).CombinedOutput()
	if success && err != nil {
		r.Error("DSvcNet1011", err, fmt.Sprintf("Connectivity from pod %q to service %q failed. Error: %s, Out: %s", printPod(fromPod), printService(toService), err, string(out)))
	} else if !success && err == nil {
		msg := fmt.Sprintf("Unexpected connectivity from pod %q to service %q.", printPod(fromPod), printService(toService))
		r.Error("DSvcNet1012", fmt.Errorf("%s", msg), msg)
	}
}
