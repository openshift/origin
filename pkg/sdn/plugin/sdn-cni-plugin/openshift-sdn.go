// +build linux

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	sdnplugin "github.com/openshift/origin/pkg/sdn/plugin"
	"github.com/openshift/origin/pkg/sdn/plugin/api"
	"github.com/openshift/origin/pkg/util/netutils"

	oclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	osapi "github.com/openshift/origin/pkg/sdn/api"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	kbandwidth "k8s.io/kubernetes/pkg/util/bandwidth"
	utilwait "k8s.io/kubernetes/pkg/util/wait"

	"github.com/containernetworking/cni/pkg/ip"
	"github.com/containernetworking/cni/pkg/ipam"
	"github.com/containernetworking/cni/pkg/ns"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"

	"github.com/vishvananda/netlink"

	// Must be imported to register API types like ClusterNetwork
	_ "github.com/openshift/origin/pkg/api/install"
)

const (
	sdnScript   = "openshift-sdn-ovs"
	setUpCmd    = "setup"
	tearDownCmd = "teardown"
	statusCmd   = "status"
	updateCmd   = "update"

	AssignMacVlanAnnotation string = "pod.network.openshift.io/assign-macvlan"

	interfaceName = knetwork.DefaultInterfaceName
)

type PodConfig struct {
	masterKubeConfig string
	clusterNetwork   string
	nodeNetwork      string
	mtu              uint32
	vnid             string
	ingressBandwidth string
	egressBandwidth  string
	wantMacvlan      bool
	podStatusPath    string
}

type podInfo struct {
	podStatusPath string
	Vnid          uint32
	Privileged    bool
	Annotations   map[string]string
}

func gatherCniArgs(cniArgs string) (map[string]string, error) {
	mapArgs := make(map[string]string)
	for _, arg := range strings.Split(cniArgs, ";") {
		parts := strings.Split(arg, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid CNI_ARG '%s'", arg)
		}
		mapArgs[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return mapArgs, nil
}

func getPodNamespaceAndName(cniArgs string) (string, string, error) {
	mapArgs, err := gatherCniArgs(cniArgs)
	if err != nil {
		return "", "", err
	}

	namespace, namespaceOk := mapArgs["K8S_POD_NAMESPACE"]
	name, nameOk := mapArgs["K8S_POD_NAME"]
	if !namespaceOk || !nameOk {
		return "", "", fmt.Errorf("missing pod namespace or name")
	}

	return namespace, name, nil
}

func readPodInfo(originClient *oclient.Client, kubeClient *kclient.Client, cniArgs string, pluginName string) (*podInfo, error) {
	namespace, name, err := getPodNamespaceAndName(cniArgs)
	if err != nil {
		return nil, err
	}

	info := &podInfo{}

	if sdnplugin.IsOpenShiftMultitenantNetworkPlugin(pluginName) {
		netNamespace, err := originClient.NetNamespaces().Get(namespace)
		if err != nil {
			return nil, err
		}
		info.Vnid = netNamespace.NetID
	}

	// FIXME: does this ensure the returned pod lives on this node?
	pod, err := kubeClient.Pods(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("failed to read pod %s/%s: %v", namespace, name, err)
	}
	info.podStatusPath = path.Join(sdnplugin.PodStatusPath(pod))

	for _, container := range pod.Spec.Containers {
		if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
			info.Privileged = true
			break
		}
	}

	info.Annotations = pod.Annotations
	return info, nil
}

func getBandwidth(pi *podInfo) (string, string, error) {
	ingress, egress, err := kbandwidth.ExtractPodBandwidthResources(pi.Annotations)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse pod bandwidth: %v", err)
	}
	var ingressStr, egressStr string
	if ingress != nil {
		ingressStr = fmt.Sprintf("%d", ingress.Value())
	}
	if egress != nil {
		egressStr = fmt.Sprintf("%d", egress.Value())
	}
	return ingressStr, egressStr, nil
}

func wantsMacvlan(pi *podInfo) (bool, error) {
	val, found := pi.Annotations[AssignMacVlanAnnotation]
	if !found || val != "true" {
		return false, nil
	}
	if pi.Privileged {
		return true, nil
	}
	return false, fmt.Errorf("pod has %q annotation but is not privileged", AssignMacVlanAnnotation)
}

func loadNetConf(bytes []byte) (*api.CNINetConfig, error) {
	n := &api.CNINetConfig{}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}
	return n, nil
}

func getClients(masterKubeConfig string) (*oclient.Client, *kclient.Client, error) {
	// Create OpenShift and Kubernetes clients to read the ClusterNetwork and PodSpec
	overrides := &configapi.ClientConnectionOverrides{}
	configapi.SetProtobufClientDefaults(overrides)

	originClient, _, err := configapi.GetOpenShiftClient(masterKubeConfig, overrides)
	if err != nil {
		return nil, nil, err
	}

	kubeClient, _, err := configapi.GetKubeClient(masterKubeConfig, overrides)
	if err != nil {
		return nil, nil, err
	}
	return originClient, kubeClient, nil
}

func getPodConfig(args *skel.CmdArgs) (*PodConfig, error) {
	n, err := loadNetConf(args.StdinData)
	if err != nil {
		return nil, err
	}

	originClient, kubeClient, err := getClients(n.MasterKubeConfig)
	if err != nil {
		return nil, err
	}

	// Grab the node's network configuration
	cn, err := originClient.ClusterNetwork().Get(osapi.ClusterNetworkDefault)
	if err != nil {
		return nil, err
	} else if _, _, err := net.ParseCIDR(cn.Network); err != nil {
		return nil, err
	}

	hostSubnet, err := originClient.HostSubnets().Get(n.NodeName)
	if err != nil {
		return nil, err
	} else if _, _, err := net.ParseCIDR(hostSubnet.Subnet); err != nil {
		return nil, err
	}

	podInfo, err := readPodInfo(originClient, kubeClient, args.Args, cn.PluginName)
	if err != nil {
		return nil, err
	}

	ingress, egress, err := getBandwidth(podInfo)
	if err != nil {
		return nil, err
	}

	macvlan, err := wantsMacvlan(podInfo)
	if err != nil {
		return nil, err
	}

	return &PodConfig{
		masterKubeConfig: n.MasterKubeConfig,
		clusterNetwork:   cn.Network,
		nodeNetwork:      hostSubnet.Subnet,
		mtu:              n.MTU,
		vnid:             strconv.FormatUint(uint64(podInfo.Vnid), 10),
		ingressBandwidth: ingress,
		egressBandwidth:  egress,
		wantMacvlan:      macvlan,
		podStatusPath:    podInfo.podStatusPath,
	}, nil
}

// Returns host veth, container veth MAC, and pod IP
func getVethInfo(netns, containerIfname string) (netlink.Link, string, string, error) {
	var (
		peerIfindex int
		contVeth    netlink.Link
		err         error
		podIP       string
	)

	containerNs, err := ns.GetNS(netns)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get container netns: %v", err)
	}
	defer containerNs.Close()

	err = containerNs.Do(func(ns.NetNS) error {
		contVeth, err = netlink.LinkByName(containerIfname)
		if err != nil {
			return err
		}
		peerIfindex = contVeth.Attrs().ParentIndex

		addrs, err := netlink.AddrList(contVeth, syscall.AF_INET)
		if err != nil {
			return fmt.Errorf("failed to get container IP addresses: %v", err)
		}
		if len(addrs) == 0 {
			return fmt.Errorf("container had no addresses")
		}
		podIP = addrs[0].IP.String()

		return nil
	})
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to inspect container interface: %v", err)
	}

	hostVeth, err := netlink.LinkByIndex(peerIfindex)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get host veth: %v", err)
	}

	return hostVeth, contVeth.Attrs().HardwareAddr.String(), podIP, nil
}

func addMacvlan(netns string) error {
	var defIface netlink.Link
	var err error

	// Find interface with the default route
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to read routes: %v", err)
	}

	for _, r := range routes {
		if r.Dst == nil {
			defIface, err = netlink.LinkByIndex(r.LinkIndex)
			if err != nil {
				return fmt.Errorf("failed to get default route interface: %v", err)
			}
		}
	}
	if defIface == nil {
		return fmt.Errorf("failed to find default route interface")
	}

	containerNs, err := ns.GetNS(netns)
	if err != nil {
		return fmt.Errorf("failed to get container netns: %v", err)
	}
	defer containerNs.Close()

	return containerNs.Do(func(ns.NetNS) error {
		err := netlink.LinkAdd(&netlink.Macvlan{
			LinkAttrs: netlink.LinkAttrs{
				MTU:         defIface.Attrs().MTU,
				Name:        "macvlan0",
				ParentIndex: defIface.Attrs().Index,
			},
			Mode: netlink.MACVLAN_MODE_PRIVATE,
		})
		if err != nil {
			return fmt.Errorf("failed to create macvlan interface: %v", err)
		}
		l, err := netlink.LinkByName("macvlan0")
		if err != nil {
			return fmt.Errorf("failed to find macvlan interface: %v", err)
		}
		err = netlink.LinkSetUp(l)
		if err != nil {
			return fmt.Errorf("failed to set macvlan interface up: %v", err)
		}
		return nil
	})
}

func getIPAMConfig(podConfig *PodConfig) ([]byte, error) {
	nodeNet, err := types.ParseCIDR(podConfig.nodeNetwork)
	if err != nil {
		return nil, fmt.Errorf("error parsing node network '%s': %v", podConfig.nodeNetwork, err)
	}
	clusterNet, err := types.ParseCIDR(podConfig.clusterNetwork)
	if err != nil {
		return nil, fmt.Errorf("error parsing cluster network '%s': %v", podConfig.clusterNetwork, err)
	}

	type hostLocalIPAM struct {
		Type   string        `json:"type"`
		Subnet types.IPNet   `json:"subnet"`
		Routes []types.Route `json:"routes"`
	}

	type cniNetworkConfig struct {
		Name string         `json:"name"`
		Type string         `json:"type"`
		IPAM *hostLocalIPAM `json:"ipam"`
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
				{Dst: net.IPNet{
					IP:   net.IPv4zero,
					Mask: net.IPMask(net.IPv4zero),
				},
					GW: netutils.GenerateDefaultGateway(nodeNet)},
				{Dst: *clusterNet},
			},
		},
	})
}

func isScriptError(err error) bool {
	_, ok := err.(*exec.ExitError)
	return ok
}

// Get the last command (which is prefixed with "+" because of "set -x") and its output
func getScriptError(output []byte) string {
	lines := strings.Split(string(output), "\n")
	for n := len(lines) - 1; n >= 0; n-- {
		if strings.HasPrefix(lines[n], "+") {
			return strings.Join(lines[n:], "\n")
		}
	}
	return string(output)
}

func updatePodIP(masterKubeConfig string, cniArgs string, podIP string) error {
	_, kubeClient, err := getClients(masterKubeConfig)
	if err != nil {
		return err
	}

	namespace, name, err := getPodNamespaceAndName(cniArgs)
	if err != nil {
		return err
	}

	resultErr := kclient.RetryOnConflict(kclient.DefaultBackoff, func() error {
		pod, err := kubeClient.Pods(namespace).Get(name)
		if err == nil {
			// Push updated PodIP to the apiserver
			pod.Status.PodIP = podIP
			_, err = kubeClient.Pods(namespace).UpdateStatus(pod)
		}
		return err
	})
	if resultErr != nil {
		return fmt.Errorf("failed to update pod %s/%s IP: %v", namespace, name, err)
	}

	return nil
}

func waitForPodStatus(path string) error {
	/* Wait about 22 seconds max */
	backoff := utilwait.Backoff{
		Steps:    8,
		Duration: 10 * time.Millisecond,
		Factor:   3.0,
		Jitter:   0.1,
	}

	// Wait for the node process to write out the pod file
	var contents []byte
	var err error
	err = utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		contents, err = ioutil.ReadFile(path)
		switch {
		case err == nil:
			return true, nil
		case os.IsNotExist(err):
			// Just wait longer
			return false, nil
		default:
			return false, err
		}
	})
	if err != nil {
		return err
	}
	os.Remove(path)

	if string(contents) != "ok" {
		return errors.New(string(contents))
	}
	return nil
}

func cmdAdd(args *skel.CmdArgs) error {
	mapArgs, err := gatherCniArgs(args.Args)
	if err != nil {
		return err
	}

	// CNI doesn't yet have an UPDATE command so we fake it through CNI_ARGS.
	// See https://github.com/containernetworking/cni/issues/89
	if action, ok := mapArgs["OPENSHIFT_ACTION"]; ok && action == "UPDATE" {
		return cmdUpdate(args)
	}

	podConfig, err := getPodConfig(args)
	if err != nil {
		return err
	}

	// Run IPAM so we can set up container veth
	ipamConfig, err := getIPAMConfig(podConfig)
	if err != nil {
		return err
	}

	os.Setenv("CNI_ARGS", "")
	result, err := ipam.ExecAdd("host-local", ipamConfig)
	if err != nil {
		return fmt.Errorf("failed to run CNI IPAM ADD: %v", err)
	}
	if result.IP4 == nil || result.IP4.IP.IP.To4() == nil {
		return fmt.Errorf("failed to obtain IP address from CNI IPAM")
	}

	// Release any IPAM allocations if the setup failed
	var success bool
	defer func() {
		if !success {
			ipam.ExecDel("host-local", ipamConfig)
		}
	}()

	var hostVeth, contVeth netlink.Link
	err = ns.WithNetNSPath(args.Netns, func(hostNS ns.NetNS) error {
		hostVeth, contVeth, err = ip.SetupVeth(interfaceName, int(podConfig.mtu), hostNS)
		if err != nil {
			return fmt.Errorf("failed to create container veth: %v", err)
		}
		// refetch to get hardware address and other properties
		contVeth, err = netlink.LinkByIndex(contVeth.Attrs().Index)
		if err != nil {
			return fmt.Errorf("failed to fetch container veth: %v", err)
		}

		// Clear out gateway to prevent ConfigureIface from adding the cluster
		// subnet via the gateway
		result.IP4.Gateway = nil
		if err = ipam.ConfigureIface(interfaceName, result); err != nil {
			return fmt.Errorf("failed to configure container IPAM: %v", err)
		}

		lo, err := netlink.LinkByName("lo")
		if err == nil {
			err = netlink.LinkSetUp(lo)
		}
		if err != nil {
			return fmt.Errorf("failed to configure container loopback: %v", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if podConfig.wantMacvlan {
		if err := addMacvlan(args.Netns); err != nil {
			return err
		}
	}

	contVethMac := contVeth.Attrs().HardwareAddr.String()
	podIP := result.IP4.IP.IP.String()
	out, err := exec.Command(sdnScript, setUpCmd, hostVeth.Attrs().Name, contVethMac, podIP, podConfig.vnid, podConfig.ingressBandwidth, podConfig.egressBandwidth).CombinedOutput()
	if isScriptError(err) {
		return fmt.Errorf("error running network setup script:\nhostVethName %s, contVethMac %s, podIP %s, podConfig %#v\n %s", hostVeth.Attrs().Name, contVethMac, podIP, podConfig, out)
	} else if err != nil {
		return err
	}

	// Push PodIP to apiserver; node process needs it for HostPort handling
	updatePodIP(podConfig.masterKubeConfig, args.Args, podIP)

	// Wait for node process to set up any pod hostports
	if err := waitForPodStatus(podConfig.podStatusPath); err != nil {
		return err
	}

	if err := result.Print(); err != nil {
		return err
	}

	success = true
	return nil
}

func cmdUpdate(args *skel.CmdArgs) error {
	podConfig, err := getPodConfig(args)
	if err != nil {
		return err
	}

	hostVeth, contVethMac, podIP, err := getVethInfo(args.Netns, args.IfName)
	if err != nil {
		return err
	}

	out, err := exec.Command(sdnScript, updateCmd, hostVeth.Attrs().Name, contVethMac, podIP, podConfig.vnid, podConfig.ingressBandwidth, podConfig.egressBandwidth).CombinedOutput()

	if isScriptError(err) {
		return fmt.Errorf("error running network update script: %s", getScriptError(out))
	} else if err != nil {
		return err
	}

	return nil
}

func cmdDel(args *skel.CmdArgs) error {
	podConfig, err := getPodConfig(args)
	if err != nil {
		return err
	}

	hostVeth, contVethMac, podIP, err := getVethInfo(args.Netns, args.IfName)
	if err != nil {
		return err
	}

	// The script's teardown functionality doesn't need the VNID
	out, err := exec.Command(sdnScript, tearDownCmd, hostVeth.Attrs().Name, contVethMac, podIP, "-1").CombinedOutput()

	if isScriptError(err) {
		return fmt.Errorf("error running network teardown script: %s", getScriptError(out))
	} else if err != nil {
		return err
	}

	ipamConfig, err := getIPAMConfig(podConfig)
	if err != nil {
		return err
	}

	// Run IPAM to release the IP address lease
	os.Setenv("CNI_ARGS", "")
	if err := ipam.ExecDel("host-local", ipamConfig); err != nil {
		return fmt.Errorf("failed to run CNI IPAM DEL: %v", err)
	}

	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdDel)
}
