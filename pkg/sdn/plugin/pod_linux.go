// +build linux

package plugin

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/plugin/cniserver"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kcontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/dockertools"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	kubehostport "k8s.io/kubernetes/pkg/kubelet/network/hostport"
	kbandwidth "k8s.io/kubernetes/pkg/util/bandwidth"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/ip"
	"github.com/containernetworking/cni/pkg/ipam"
	"github.com/containernetworking/cni/pkg/ns"
	cnitypes "github.com/containernetworking/cni/pkg/types"

	"github.com/vishvananda/netlink"
)

const (
	sdnScript   = "openshift-sdn-ovs"
	setUpCmd    = "setup"
	tearDownCmd = "teardown"
	updateCmd   = "update"

	podInterfaceName = knetwork.DefaultInterfaceName
)

type PodConfig struct {
	vnid             uint32
	ingressBandwidth string
	egressBandwidth  string
	wantMacvlan      bool
}

func getBandwidth(pod *kapi.Pod) (string, string, error) {
	ingress, egress, err := kbandwidth.ExtractPodBandwidthResources(pod.Annotations)
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

func wantsMacvlan(pod *kapi.Pod) (bool, error) {
	privileged := false
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
			privileged = true
			break
		}
	}

	val, ok := pod.Annotations[sdnapi.AssignMacvlanAnnotation]
	if !ok || val != "true" {
		return false, nil
	}
	if !privileged {
		return false, fmt.Errorf("pod has %q annotation but is not privileged", sdnapi.AssignMacvlanAnnotation)
	}

	return true, nil
}

// Create and return a PodConfig describing which openshift-sdn specific pod attributes
// to configure
func (m *podManager) getPodConfig(req *cniserver.PodRequest) (*PodConfig, *kapi.Pod, error) {
	var err error

	config := &PodConfig{}
	if m.multitenant {
		config.vnid, err = m.vnids.GetVNID(req.PodNamespace)
		if err != nil {
			return nil, nil, err
		}
	}

	pod, err := m.kClient.Pods(req.PodNamespace).Get(req.PodName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read pod %s/%s: %v", req.PodNamespace, req.PodName, err)
	}

	config.wantMacvlan, err = wantsMacvlan(pod)
	if err != nil {
		return nil, nil, err
	}

	config.ingressBandwidth, config.egressBandwidth, err = getBandwidth(pod)
	if err != nil {
		return nil, nil, err
	}

	return config, pod, nil
}

// For a given container, returns host veth name, container veth MAC, and pod IP
func getVethInfo(netns, containerIfname string) (string, string, string, error) {
	var (
		peerIfindex int
		contVeth    netlink.Link
		err         error
		podIP       string
	)

	containerNs, err := ns.GetNS(netns)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get container netns: %v", err)
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
		return "", "", "", fmt.Errorf("failed to inspect container interface: %v", err)
	}

	hostVeth, err := netlink.LinkByIndex(peerIfindex)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get host veth: %v", err)
	}

	return hostVeth.Attrs().Name, contVeth.Attrs().HardwareAddr.String(), podIP, nil
}

// Adds a macvlan interface to a container for use with the egress router feature
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

	podNs, err := ns.GetNS(netns)
	if err != nil {
		return fmt.Errorf("could not open netns %q", netns)
	}
	defer podNs.Close()

	err = netlink.LinkAdd(&netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{
			MTU:         defIface.Attrs().MTU,
			Name:        "macvlan0",
			ParentIndex: defIface.Attrs().Index,
			Namespace:   netlink.NsFd(podNs.Fd()),
		},
		Mode: netlink.MACVLAN_MODE_PRIVATE,
	})
	if err != nil {
		return fmt.Errorf("failed to create macvlan interface: %v", err)
	}
	return podNs.Do(func(netns ns.NetNS) error {
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

// Run CNI IPAM for the container, either allocating an IP address and returning
// it (for ADD) or releasing the lease and cleaning up (for DEL)
func (m *podManager) runIPAM(netnsPath string, action cniserver.CNICommand, id string) (*cnitypes.Result, error) {
	args := &invoke.Args{
		Command:     string(action),
		ContainerID: id,
		NetNS:       netnsPath,
		IfName:      podInterfaceName,
		Path:        "/opt/cni/bin",
	}

	if action == cniserver.CNI_ADD {
		result, err := invoke.ExecPluginWithResult("/opt/cni/bin/host-local", m.ipamConfig, args)
		if err != nil {
			return nil, fmt.Errorf("failed to run CNI IPAM ADD: %v", err)
		}

		if result.IP4 == nil {
			return nil, fmt.Errorf("failed to obtain IP address from CNI IPAM")
		}

		return result, nil
	} else if action == cniserver.CNI_DEL {
		err := invoke.ExecPluginWithoutResult("/opt/cni/bin/host-local", m.ipamConfig, args)
		if err != nil {
			return nil, fmt.Errorf("failed to run CNI IPAM DEL: %v", err)
		}
		return nil, nil
	}

	return nil, fmt.Errorf("invalid IPAM action %v", action)
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

func vnidToString(vnid uint32) string {
	return strconv.FormatUint(uint64(vnid), 10)
}

// Set up all networking (host/container veth, OVS flows, IPAM, loopback, etc)
func (m *podManager) setup(req *cniserver.PodRequest) (*cnitypes.Result, *kubehostport.RunningPod, error) {
	podConfig, pod, err := m.getPodConfig(req)
	if err != nil {
		return nil, nil, err
	}

	ipamResult, err := m.runIPAM(req.Netns, cniserver.CNI_ADD, req.ContainerId)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to run IPAM for %v: %v", req.ContainerId, err)
	}
	podIP := ipamResult.IP4.IP.IP

	// Release any IPAM allocations and hostports if the setup failed
	var success bool
	defer func() {
		if !success {
			m.runIPAM(req.Netns, cniserver.CNI_DEL, req.ContainerId)
			if err := m.hostportHandler.SyncHostports(TUN, m.getRunningPods()); err != nil {
				glog.Warningf("failed syncing hostports: %v", err)
			}
		}
	}()

	// Open any hostports the pod wants
	newPod := &kubehostport.RunningPod{Pod: pod, IP: podIP}
	if err := m.hostportHandler.OpenPodHostportsAndSync(newPod, TUN, m.getRunningPods()); err != nil {
		return nil, nil, err
	}

	var hostVeth, contVeth netlink.Link
	err = ns.WithNetNSPath(req.Netns, func(hostNS ns.NetNS) error {
		hostVeth, contVeth, err = ip.SetupVeth(podInterfaceName, int(m.mtu), hostNS)
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
		ipamResult.IP4.Gateway = nil
		if err = ipam.ConfigureIface(podInterfaceName, ipamResult); err != nil {
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
		return nil, nil, err
	}

	if podConfig.wantMacvlan {
		if err := addMacvlan(req.Netns); err != nil {
			return nil, nil, err
		}
	}

	contVethMac := contVeth.Attrs().HardwareAddr.String()
	vnidStr := vnidToString(podConfig.vnid)
	out, err := exec.Command(sdnScript, setUpCmd, hostVeth.Attrs().Name, contVethMac, podIP.String(), vnidStr, podConfig.ingressBandwidth, podConfig.egressBandwidth).CombinedOutput()
	glog.V(5).Infof("SetUpPod network plugin output: %s, %v", string(out), err)

	if isScriptError(err) {
		return nil, nil, fmt.Errorf("error running network setup script:\nhostVethName %s, contVethMac %s, podIP %s, podConfig %#v\n %s", hostVeth.Attrs().Name, contVethMac, podIP.String(), podConfig, getScriptError(out))
	} else if err != nil {
		return nil, nil, err
	}

	success = true
	return ipamResult, newPod, nil
}

func (m *podManager) getContainerNetnsPath(id string) (string, error) {
	runtime, ok := m.host.GetRuntime().(*dockertools.DockerManager)
	if !ok {
		return "", fmt.Errorf("openshift-sdn execution called on non-docker runtime")
	}
	return runtime.GetNetNS(kcontainer.DockerID(id).ContainerID())
}

// Update OVS flows when something (like the pod's namespace VNID) changes
func (m *podManager) update(req *cniserver.PodRequest) error {
	// Updates may come at startup and thus we may not have the pod's
	// netns from kubelet (since kubelet doesn't have UPDATE actions).
	// Read the missing netns from the pod's file.
	if req.Netns == "" {
		netns, err := m.getContainerNetnsPath(req.ContainerId)
		if err != nil {
			return err
		}
		req.Netns = netns
	}

	podConfig, _, err := m.getPodConfig(req)
	if err != nil {
		return err
	}

	hostVethName, contVethMac, podIP, err := getVethInfo(req.Netns, podInterfaceName)
	if err != nil {
		return err
	}

	vnidStr := vnidToString(podConfig.vnid)
	out, err := exec.Command(sdnScript, updateCmd, hostVethName, contVethMac, podIP, vnidStr, podConfig.ingressBandwidth, podConfig.egressBandwidth).CombinedOutput()
	glog.V(5).Infof("UpdatePod network plugin output: %s, %v", string(out), err)

	if isScriptError(err) {
		return fmt.Errorf("error running network update script: %s", getScriptError(out))
	} else if err != nil {
		return err
	}

	return nil
}

// Clean up all pod networking (clear OVS flows, release IPAM lease, remove host/container veth)
func (m *podManager) teardown(req *cniserver.PodRequest) error {
	if err := ns.IsNSorErr(req.Netns); err != nil {
		if _, ok := err.(ns.NSPathNotExistErr); ok {
			glog.V(3).Infof("teardown called on already-destroyed pod %s/%s", req.PodNamespace, req.PodName)
			return nil
		}
	}

	hostVethName, contVethMac, podIP, err := getVethInfo(req.Netns, podInterfaceName)
	if err != nil {
		return err
	}

	// The script's teardown functionality doesn't need the VNID
	out, err := exec.Command(sdnScript, tearDownCmd, hostVethName, contVethMac, podIP, "-1").CombinedOutput()
	glog.V(5).Infof("TearDownPod network plugin output: %s, %v", string(out), err)

	if isScriptError(err) {
		return fmt.Errorf("error running network teardown script: %s", getScriptError(out))
	} else if err != nil {
		return err
	}

	if _, err := m.runIPAM(req.Netns, cniserver.CNI_DEL, req.ContainerId); err != nil {
		return err
	}

	if err := m.hostportHandler.SyncHostports(TUN, m.getRunningPods()); err != nil {
		return err
	}

	return nil
}
