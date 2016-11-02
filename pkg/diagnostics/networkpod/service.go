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
	"github.com/openshift/origin/pkg/diagnostics/networkpod/util"
	"github.com/openshift/origin/pkg/diagnostics/types"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

const (
	CheckServiceNetworkName = "CheckServiceNetwork"
)

// CheckServiceNetwork is a Diagnostic to check communication between services in the cluster.
type CheckServiceNetwork struct {
	KubeClient *kclient.Client
	OSClient   *osclient.Client

	vnidMap map[string]uint32
	res     types.DiagnosticResult
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
	d.res = types.NewDiagnosticResult(CheckServiceNetworkName)

	pluginName, ok, err := util.GetOpenShiftNetworkPlugin(d.OSClient)
	if err != nil {
		d.res.Error("DSvcNet1001", err, fmt.Sprintf("Checking network plugin failed. Error: %s", err))
		return d.res
	}
	if !ok {
		d.res.Warn("DSvcNet1002", nil, "Skipping service connectivity test. Reason: Not using openshift network plugin.")
		return d.res
	}

	services, err := getAllServices(d.KubeClient)
	if err != nil {
		d.res.Error("DSvcNet1003", err, fmt.Sprintf("Getting all services failed. Error: %s", err))
		return d.res
	}
	if len(services) == 0 {
		d.res.Warn("DSvcNet1004", nil, "Skipping service connectivity test. Reason: No services found.")
		return d.res
	}

	localPods, _, err := util.GetLocalAndNonLocalDiagnosticPods(d.KubeClient)
	if err != nil {
		d.res.Error("DSvcNet1005", err, fmt.Sprintf("Getting local and nonlocal pods failed. Error: %s", err))
		return d.res
	}

	if sdnapi.IsOpenShiftMultitenantNetworkPlugin(pluginName) {
		netnsList, err := d.OSClient.NetNamespaces().List(kapi.ListOptions{})
		if err != nil {
			d.res.Error("DSvcNet1006", err, fmt.Sprintf("Getting all network namespaces failed. Error: %s", err))
			return d.res
		}

		d.vnidMap = map[string]uint32{}
		for _, netns := range netnsList.Items {
			d.vnidMap[netns.NetName] = netns.NetID
		}
	}

	localGlobalPods, localNonGlobalPods := util.GetGlobalAndNonGlobalPods(localPods, d.vnidMap)

	// Applicable to flat and multitenant networks
	if len(localGlobalPods) > 0 {
		d.checkConnection(localGlobalPods, services, "Skipping service connectivity test for global projects. Reason: Couldn't find a global pod.")
	}

	// Applicable to multitenant network
	isMultitenant := (d.vnidMap != nil)
	if isMultitenant {
		d.checkConnection(localNonGlobalPods, services, "Skipping service connectivity test for non-global projects. Reason: Couldn't find a non-global pod.")
	}
	return d.res
}

func (d CheckServiceNetwork) checkConnection(pods []kapi.Pod, services []kapi.Service, warnMsg string) {
	if len(pods) < 0 || len(services) < 0 {
		d.res.Warn("DSvcNet1007", nil, warnMsg)
		return
	}

	sameNamespace := false
	diffNamespace := false

	// Test pod to service connection between same and different namespaces
	for _, pod := range pods {
		for _, svc := range services {
			if sameNamespace && diffNamespace {
				return
			}
			if !sameNamespace && (pod.Namespace == svc.Namespace) {
				sameNamespace = true
				d.checkPodToServiceConnection(&pod, &svc)
			}
			if !diffNamespace && (pod.Namespace != svc.Namespace) {
				diffNamespace = true
				d.checkPodToServiceConnection(&pod, &svc)
			}
		}
	}

	if !sameNamespace {
		d.res.Warn("DSvcNet1012", nil, fmt.Sprintf("Same Namespace: %s", warnMsg))
	}
	if !diffNamespace {
		d.res.Warn("DSvcNet1013", nil, fmt.Sprintf("Different namespaces: %s", warnMsg))
	}
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

func printService(svc *kapi.Service) string {
	return fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
}

func (d CheckServiceNetwork) checkPodToServiceConnection(fromPod *kapi.Pod, toService *kapi.Service) {
	if len(fromPod.Status.ContainerStatuses) == 0 {
		d.res.Error("DSvcNet1008", nil, fmt.Sprintf("ContainerID not found for pod %q", util.PrintPod(fromPod)))
		return
	}

	success := util.ExpectedConnectionStatus(fromPod.Namespace, toService.Namespace, d.vnidMap)

	kexecer := kexec.New()
	containerID := kcontainer.ParseContainerID(fromPod.Status.ContainerStatuses[0].ContainerID).ID
	pid, err := kexecer.Command("docker", "inspect", "-f", "{{.State.Pid}}", containerID).CombinedOutput()
	if err != nil {
		d.res.Error("DSvcNet1009", err, fmt.Sprintf("Fetching pid for pod %q failed. Error: %s", util.PrintPod(fromPod), err))
		return
	}

	// In bash, redirecting to /dev/tcp/HOST/PORT or /dev/udp/HOST/PORT opens a connection
	// to that HOST:PORT. Use this to test connectivity to the service; we can't use ping
	// like in the pod connectivity check because only connections to the correct port
	// get redirected by the iptables rules.
	srvConCmd := fmt.Sprintf("echo -n '' > /dev/%s/%s/%d", strings.ToLower(string(toService.Spec.Ports[0].Protocol)), toService.Spec.ClusterIP, toService.Spec.Ports[0].Port)
	out, err := kexecer.Command("nsenter", "-n", "-t", strings.Trim(fmt.Sprintf("%s", pid), "\n"), "--", "timeout", "1", "bash", "-c", srvConCmd).CombinedOutput()
	if success && err != nil {
		d.res.Error("DSvcNet1010", err, fmt.Sprintf("Connectivity from pod %q to service %q failed. Error: %s, Out: %s", util.PrintPod(fromPod), printService(toService), err, string(out)))
	} else if !success && err == nil {
		msg := fmt.Sprintf("Unexpected connectivity from pod %q to service %q.", util.PrintPod(fromPod), printService(toService))
		d.res.Error("DSvcNet1011", fmt.Errorf("%s", msg), msg)
	}
}
