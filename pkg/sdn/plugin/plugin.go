package plugin

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/util/netutils"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/dockertools"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	utilsets "k8s.io/kubernetes/pkg/util/sets"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/ipam"
	"github.com/containernetworking/cni/pkg/ns"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/vishvananda/netlink"
)

const (
	SingleTenantPluginName string = "redhat/openshift-ovs-subnet"
	MultiTenantPluginName  string = "redhat/openshift-ovs-multitenant"

	IngressBandwidthAnnotation string = "kubernetes.io/ingress-bandwidth"
	EgressBandwidthAnnotation  string = "kubernetes.io/egress-bandwidth"
	AssignMacVlanAnnotation    string = "pod.network.openshift.io/assign-macvlan"
)

func IsOpenShiftNetworkPlugin(pluginName string) bool {
	switch strings.ToLower(pluginName) {
	case SingleTenantPluginName, MultiTenantPluginName:
		return true
	}
	return false
}

func IsOpenShiftMultitenantNetworkPlugin(pluginName string) bool {
	if strings.ToLower(pluginName) == MultiTenantPluginName {
		return true
	}
	return false
}

//-----------------------------------------------

const (
	setUpCmd    = "setup"
	tearDownCmd = "teardown"
	statusCmd   = "status"
	updateCmd   = "update"

	containerIfname   = "eth0"
)

func (plugin *OsdnNode) getExecutable() string {
	return "openshift-sdn-ovs"
}

func (plugin *OsdnNode) Init(host knetwork.Host, _ componentconfig.HairpinMode, _ string, _ int) error {
	plugin.host = host
	return nil
}

func (plugin *OsdnNode) Name() string {
	if plugin.multitenant {
		return MultiTenantPluginName
	} else {
		return SingleTenantPluginName
	}
}

func (plugin *OsdnNode) Capabilities() utilsets.Int {
	return utilsets.NewInt(knetwork.NET_PLUGIN_CAPABILITY_SHAPING)
}

func (plugin *OsdnNode) getVNID(namespace string) (string, error) {
	if plugin.multitenant {
		vnid, err := plugin.vnids.WaitAndGetVNID(namespace)
		if err != nil {
			return "", err
		}
		return strconv.FormatUint(uint64(vnid), 10), nil
	}

	return "0", nil
}

var minRsrc = resource.MustParse("1k")
var maxRsrc = resource.MustParse("1P")

func parseAndValidateBandwidth(value string) (int64, error) {
	rsrc, err := resource.ParseQuantity(value)
	if err != nil {
		return -1, err
	}

	if rsrc.Value() < minRsrc.Value() {
		return -1, fmt.Errorf("resource value %d is unreasonably small (< %d)", rsrc.Value(), minRsrc.Value())
	}
	if rsrc.Value() > maxRsrc.Value() {
		return -1, fmt.Errorf("resource value %d is unreasonably large (> %d)", rsrc.Value(), maxRsrc.Value())
	}
	return rsrc.Value(), nil
}

func extractBandwidthResources(pod *kapi.Pod) (ingress, egress int64, err error) {
	str, found := pod.Annotations[IngressBandwidthAnnotation]
	if found {
		ingress, err = parseAndValidateBandwidth(str)
		if err != nil {
			return -1, -1, err
		}
	}
	str, found = pod.Annotations[EgressBandwidthAnnotation]
	if found {
		egress, err = parseAndValidateBandwidth(str)
		if err != nil {
			return -1, -1, err
		}
	}
	return ingress, egress, nil
}

func wantsMacvlan(pod *kapi.Pod) (bool, error) {
	val, found := pod.Annotations[AssignMacVlanAnnotation]
	if !found || val != "true" {
		return false, nil
	}
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
			return true, nil
		}
	}
	return false, fmt.Errorf("Pod has %q annotation but is not privileged", AssignMacVlanAnnotation)
}

func isScriptError(err error) bool {
	_, ok := err.(*exec.ExitError)
	return ok
}

// Get the last command (which is prefixed with "+" because of "set -x") and its output
// (Unless the script ended with "echo ...; exit", in which case we just return the
// echoed text.)
func getScriptError(output []byte) string {
	lines := strings.Split(string(output), "\n")
	last := len(lines)
	for n := last - 1; n >= 0; n-- {
		if strings.HasPrefix(lines[n], "+ exit") {
			last = n
		} else if strings.HasPrefix(lines[n], "+ echo") {
			return strings.Join(lines[n+1:last], "\n")
		} else if strings.HasPrefix(lines[n], "+") {
			return strings.Join(lines[n:], "\n")
		}
	}
	return string(output)
}

func (plugin *OsdnNode) SetUpPod(namespace string, name string, id kubeletTypes.ContainerID) error {
	err := plugin.WaitForPodNetworkReady()
	if err != nil {
		return err
	}

	hostVethName, contMac, contIP, err := plugin.vethSetup(id)
	if err != nil {
		return err
	}

	ingressStr, egressStr, vnidStr, macvlanStr, err := plugin.getPodInfo(namespace, name)
	if err != nil {
		return err
	}

	out, err := exec.Command(plugin.getExecutable(), setUpCmd, hostVethName, contMac, contIP, vnidStr, ingressStr, egressStr, macvlanStr).CombinedOutput()
	glog.V(5).Infof("SetUpPod network plugin output: %s, %v", string(out), err)

	if isScriptError(err) {
		return fmt.Errorf("Error running network setup script: %s", getScriptError(out))
	} else {
		return err
	}
}

func (plugin *OsdnNode) TearDownPod(namespace string, name string, id kubeletTypes.ContainerID) error {
	hostVethName, contMac, contIP, err := plugin.vethInspect(id)
	if err != nil {
		return err
	}

	containerNs, err := plugin.getContainerNS(id)
	if err != nil {
		return fmt.Errorf("Failed to get container network namespace: %v", err)
	}
	defer containerNs.Close()

	if _, err := plugin.runIPAM(containerNs, "DEL", id); err != nil {
		return err
	}

	// The script's teardown functionality doesn't need the VNID
	out, err := exec.Command(plugin.getExecutable(), tearDownCmd, hostVethName, contMac, contIP, "-1", "", "", "false").CombinedOutput()
	glog.V(5).Infof("TearDownPod network plugin output: %s, %v", string(out), err)

	if isScriptError(err) {
		return fmt.Errorf("Error running network teardown script: %s", getScriptError(out))
	} else {
		return err
	}
}

func (plugin *OsdnNode) Status() error {
	return nil
}

func (plugin *OsdnNode) GetPodNetworkStatus(namespace string, name string, id kubeletTypes.ContainerID) (*knetwork.PodNetworkStatus, error) {
	_, _, contIP, err := plugin.vethInspect(id)
	if err != nil {
		return nil, err
	}

	ip, _, err := net.ParseCIDR(contIP)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse IP %s: %v", contIP, err)
	}

	return &knetwork.PodNetworkStatus{IP: ip}, nil
}

func (plugin *OsdnNode) UpdatePod(namespace string, name string, id kubeletTypes.ContainerID) error {
	hostVethName, contMac, contIP, err := plugin.vethInspect(id)
	if err != nil {
		return err
	}

	ingressStr, egressStr, vnidStr, macvlanStr, err := plugin.getPodInfo(namespace, name)
	if err != nil {
		return err
	}

	out, err := exec.Command(plugin.getExecutable(), updateCmd, hostVethName, contMac, contIP, vnidStr, ingressStr, egressStr, macvlanStr).CombinedOutput()
	glog.V(5).Infof("UpdatePod network plugin output: %s, %v", string(out), err)

	if isScriptError(err) {
		return fmt.Errorf("Error running network update script: %s", getScriptError(out))
	} else {
		return err
	}
}

func (plugin *OsdnNode) Event(name string, details map[string]interface{}) {
}

// Returns host docker host veth name, container MAC address, and container IP address
func (plugin *OsdnNode) vethSetup(id kubeletTypes.ContainerID) (string, string, string, error) {
	containerNs, err := plugin.getContainerNS(id)
	if err != nil {
		return "", "", "", fmt.Errorf("Failed to get container network namespace: %v", err)
	}
	defer containerNs.Close()

	hostVeth, contVeth, err := getVeths(containerNs)
	if err != nil {
		return "", "", "", err
	}

	// Clear docker addresses and routes from container veth
	err = containerNs.Do(func(ns.NetNS) error {
		addrs, err := netlink.AddrList(contVeth, syscall.AF_UNSPEC)
		if err != nil {
			return err
		}
		for _, a := range addrs {
			netlink.AddrDel(contVeth, &a)
		}

		routes, err := netlink.RouteList(contVeth, syscall.AF_UNSPEC)
		if err != nil {
			return err
		}
		for _, r := range routes {
			netlink.RouteDel(&r)
		}

		return nil
	})
	if err != nil {
		return "", "", "", fmt.Errorf("Failed to clear docker IPAM: %v", err)
	}

	// Remove the host veth interface from the docker bridge
	if err := netlink.LinkSetMasterByIndex(hostVeth, 0); err != nil {
		return "", "", "", fmt.Errorf("Failed to unparent host veth: %v", err)
	}

	ipStr, err := plugin.runIPAM(containerNs, "ADD", id)
	if err != nil {
		return "", "", "", err
	}

	return hostVeth.Attrs().Name, contVeth.Attrs().HardwareAddr.String(), ipStr, nil
}

// Returns host docker host veth name, container MAC address, and container IP address
func (plugin *OsdnNode) vethInspect(id kubeletTypes.ContainerID) (string, string, string, error) {
	containerNs, err := plugin.getContainerNS(id)
	if err != nil {
		return "", "", "", fmt.Errorf("Failed to get container network namespace: %v", err)
	}
	defer containerNs.Close()

	hostVeth, contVeth, err := getVeths(containerNs)
	if err != nil {
		return "", "", "", err
	}

	var containerIP string
	err = containerNs.Do(func(ns.NetNS) error {
		link, err := netlink.LinkByName(containerIfname)
		if err != nil {
			return fmt.Errorf("failed to get container interface: %v", err)
		}
		addrs, err := netlink.AddrList(link, syscall.AF_INET)
		if err != nil {
			return fmt.Errorf("failed to get container IP addresses: %v", err)
		}
		if len(addrs) == 0 {
			return fmt.Errorf("container had no addresses")
		}
		containerIP = strings.Split(addrs[0].String(), " ")[0]
		return nil
	})
	if err != nil {
		return "", "", "", fmt.Errorf("Failed to read container IP address: %v", err)
	}

	return hostVeth.Attrs().Name, contVeth.Attrs().HardwareAddr.String(), containerIP, nil
}

func (plugin *OsdnNode) getContainerNS(id kubeletTypes.ContainerID) (ns.NetNS, error) {
	runtime, ok := plugin.host.GetRuntime().(*dockertools.DockerManager)
	if !ok {
		return nil, fmt.Errorf("openshift-sdn execution called on non-docker runtime")
	}
	netns, err := runtime.GetNetNS(id)
	if err != nil {
		return nil, err
	}

	return ns.GetNS(netns)
}

func getVeths(containerNs ns.NetNS) (netlink.Link, netlink.Link, error) {
	var (
		peerIfindex int
		contVeth    netlink.Link
		err         error
	)

	err = containerNs.Do(func(ns.NetNS) error {
		contVeth, err = netlink.LinkByName(containerIfname)
		if err != nil {
			return err
		}
		peerIfindex = contVeth.Attrs().ParentIndex
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get eth0 veth peer index: %v", err)
	}

	hostVeth, err := netlink.LinkByIndex(peerIfindex)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get host veth: %v", err)
	}

	return hostVeth, contVeth, nil
}

func (plugin *OsdnNode) getPodInfo(namespace, name string) (string, string, string, string, error) {
	pod, err := plugin.registry.GetPod(plugin.hostName, namespace, name)
	if err != nil {
		return "", "", "", "", err
	}
	if pod == nil {
		return "", "", "", "", fmt.Errorf("failed to retrieve pod %s/%s", namespace, name)
	}
	ingress, egress, err := extractBandwidthResources(pod)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to parse pod %s/%s ingress/egress quantity: %v", namespace, name, err)
	}
	var ingressStr, egressStr string
	if ingress > 0 {
		ingressStr = fmt.Sprintf("%d", ingress)
	}
	if egress > 0 {
		egressStr = fmt.Sprintf("%d", egress)
	}

	vnidStr, err := plugin.getVNID(namespace)
	if err != nil {
		return "", "", "", "", err
	}

	macvlan, err := wantsMacvlan(pod)
	if err != nil {
		return "", "", "", "", err
	}

	return ingressStr, egressStr, vnidStr, fmt.Sprintf("%t", macvlan), nil
}

func getIPAMConfig(nodeNetwork string, clusterNet *net.IPNet) ([]byte, error) {
	nodeNet, err := types.ParseCIDR(nodeNetwork)
	if err != nil {
		return nil, fmt.Errorf("Error parsing node network '%s': %v", nodeNetwork, err)
	}
	defroute, err := types.ParseCIDR("0.0.0.0/0")
	if err != nil {
		return nil, fmt.Errorf("Error parsing default route: %v", err)
	}

	type hostLocalIPAM struct {
		Type   string        `json:"type,omitempty"`
		Subnet types.IPNet   `json:"subnet"`
		Routes []types.Route `json:"routes"`
	}

	type cniNetworkConfig struct {
		Name string         `json:"name,omitempty"`
		Type string         `json:"type,omitempty"`
		IPAM *hostLocalIPAM `json:"ipam,omitempty"`
	}

	return json.Marshal(&cniNetworkConfig{
		Name: "openshift-sdn",
		Type: "openshift-sdn",
		IPAM: &hostLocalIPAM{
			Type: "host-local",
			Subnet: types.IPNet{
				IP:   nodeNet.IP,
				Mask: nodeNet.Mask,
			},
			Routes: []types.Route{
				{Dst: *defroute, GW: netutils.GenerateDefaultGateway(nodeNet)},
				{Dst: *clusterNet},
			},
		},
	})
}

func (plugin *OsdnNode) runIPAM(containerNs ns.NetNS, action string, id kubeletTypes.ContainerID) (string, error) {
	if plugin.cniConfig == nil {
		ni, err := plugin.registry.GetNetworkInfo()
		if err != nil {
			return "", fmt.Errorf("Failed to get network info: %v", err)
		}

		config, err := getIPAMConfig(plugin.localSubnet.Subnet, ni.ClusterNetwork)
		if err != nil {
			return "", err
		}
		plugin.cniConfig = config
	}

	args := &invoke.Args{
		Command:     action,
		ContainerID: id.String(),
		NetNS:       containerNs.Path(),
		IfName:      containerIfname,
		Path:        "/opt/cni/bin",
	}

	// run the IPAM plugin and get back the config to apply
	var podIP string
	if action == "ADD" {
		result, err := invoke.ExecPluginWithResult("/opt/cni/bin/host-local", plugin.cniConfig, args)
		if err != nil {
			return "", fmt.Errorf("Failed to run CNI IPAM ADD: %v", err)
		}

		if result.IP4 == nil {
			return "", fmt.Errorf("Failed to obtain IP address from CNI IPAM")
		}

		err = containerNs.Do(func(ns.NetNS) error {
			return ipam.ConfigureIface(containerIfname, result)
		})
		if err != nil {
			return "", fmt.Errorf("Failed to configure container interface: %v", err)
		}

		podIP = result.IP4.IP.IP.String()
	} else if action == "DEL" {
		err := invoke.ExecPluginWithoutResult("/opt/cni/bin/host-local", plugin.cniConfig, args)
		if err != nil {
			return "", fmt.Errorf("Failed to run CNI IPAM DEL: %v", err)
		}
	} else {
		return "", fmt.Errorf("Invalid IPAM action %s", action)
	}

	return podIP, nil
}
