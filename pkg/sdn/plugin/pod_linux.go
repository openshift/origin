// +build linux

package plugin

import (
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"path/filepath"
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
	ksets "k8s.io/kubernetes/pkg/util/sets"

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

func createIPAMArgs(netnsPath string, action cniserver.CNICommand, id string) *invoke.Args {
	return &invoke.Args{
		Command:     string(action),
		ContainerID: id,
		NetNS:       netnsPath,
		IfName:      podInterfaceName,
		Path:        "/opt/cni/bin",
	}
}

// Run CNI IPAM allocation for the container and return the allocated IP address
func (m *podManager) ipamAdd(netnsPath string, id string) (*cnitypes.Result, error) {
	if netnsPath == "" {
		return nil, fmt.Errorf("netns required for CNI_ADD")
	}

	args := createIPAMArgs(netnsPath, cniserver.CNI_ADD, id)
	result, err := invoke.ExecPluginWithResult("/opt/cni/bin/host-local", m.ipamConfig, args)
	if err != nil {
		return nil, fmt.Errorf("failed to run CNI IPAM ADD: %v", err)
	}

	if result.IP4 == nil {
		return nil, fmt.Errorf("failed to obtain IP address from CNI IPAM")
	}

	return result, nil
}

// Run CNI IPAM release for the container
func (m *podManager) ipamDel(id string) error {
	args := createIPAMArgs("", cniserver.CNI_DEL, id)
	err := invoke.ExecPluginWithoutResult("/opt/cni/bin/host-local", m.ipamConfig, args)
	if err != nil {
		return fmt.Errorf("failed to run CNI IPAM DEL: %v", err)
	}
	return nil
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

// podIsExited returns true if the pod is exited (all containers inside are exited).
func podIsExited(p *kcontainer.Pod) bool {
	for _, c := range p.Containers {
		if c.State != kcontainer.ContainerStateExited {
			return false
		}
	}
	return true
}

// getNonExitedPods returns a list of pods that have at least one running container.
func (m *podManager) getNonExitedPods() ([]*kcontainer.Pod, error) {
	ret := []*kcontainer.Pod{}
	pods, err := m.host.GetRuntime().GetPods(true)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve pods from runtime: %v", err)
	}
	for _, p := range pods {
		if podIsExited(p) {
			continue
		}
		ret = append(ret, p)
	}
	return ret, nil
}

// ipamGarbageCollection will release unused IPs from dead containers that
// the CNI plugin was never notified had died.  openshift-sdn uses the CNI
// host-local IPAM plugin, which stores allocated IPs in a file in
// /var/lib/cni/network. Each file in this directory has as its name the
// allocated IP address of the container, and as its contents the container ID.
// This routine looks for container IDs that are not reported as running by the
// container runtime, and releases each one's IPAM allocation.
func (m *podManager) ipamGarbageCollection() {
	glog.V(2).Infof("Starting IP garbage collection")

	const ipamDir string = "/var/lib/cni/networks/openshift-sdn"
	files, err := ioutil.ReadDir(ipamDir)
	if err != nil {
		glog.Errorf("Failed to list files in CNI host-local IPAM store %v: %v", ipamDir, err)
		return
	}

	// gather containerIDs for allocated ips
	ipContainerIdMap := make(map[string]string)
	for _, file := range files {
		// skip non checkpoint file
		if ip := net.ParseIP(file.Name()); ip == nil {
			continue
		}

		content, err := ioutil.ReadFile(filepath.Join(ipamDir, file.Name()))
		if err != nil {
			glog.Errorf("Failed to read file %v: %v", file, err)
		}
		ipContainerIdMap[file.Name()] = strings.TrimSpace(string(content))
	}

	// gather infra container IDs of current running Pods
	runningContainerIDs := ksets.String{}
	pods, err := m.getNonExitedPods()
	if err != nil {
		glog.Errorf("Failed to get pods: %v", err)
		return
	}
	for _, pod := range pods {
		containerID, err := m.host.GetRuntime().GetPodContainerID(pod)
		if err != nil {
			glog.Warningf("Failed to get infra containerID of %q/%q: %v", pod.Namespace, pod.Name, err)
			continue
		}

		runningContainerIDs.Insert(strings.TrimSpace(containerID.ID))
	}

	// release leaked ips
	for ip, containerID := range ipContainerIdMap {
		// if the container is not running, release IP
		if runningContainerIDs.Has(containerID) {
			continue
		}

		glog.V(2).Infof("Releasing IP %q allocated to %q.", ip, containerID)
		m.ipamDel(containerID)
	}
}

// Set up all networking (host/container veth, OVS flows, IPAM, loopback, etc)
func (m *podManager) setup(req *cniserver.PodRequest) (*cnitypes.Result, *kubehostport.RunningPod, error) {
	podConfig, pod, err := m.getPodConfig(req)
	if err != nil {
		return nil, nil, err
	}

	ipamResult, err := m.ipamAdd(req.Netns, req.ContainerId)
	if err != nil {
		// TODO: Remove this hack once we've figured out how to retrieve the netns
		// of an exited container. Currently, restarting docker will leak a bunch of
		// ips. This will exhaust available ip space unless we cleanup old ips. At the
		// same time we don't want to try GC'ing them periodically as that could lead
		// to a performance regression in starting pods. So on each setup failure, try
		// GC on the assumption that the kubelet is going to retry pod creation, and
		// when it does, there will be ips.
		m.ipamGarbageCollection()

		return nil, nil, fmt.Errorf("failed to run IPAM for %v: %v", req.ContainerId, err)
	}
	podIP := ipamResult.IP4.IP.IP

	// Release any IPAM allocations and hostports if the setup failed
	var success bool
	defer func() {
		if !success {
			m.ipamDel(req.ContainerId)
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
	netnsValid := true
	if err := ns.IsNSorErr(req.Netns); err != nil {
		if _, ok := err.(ns.NSPathNotExistErr); ok {
			glog.V(3).Infof("teardown called on already-destroyed pod %s/%s; only cleaning up IPAM", req.PodNamespace, req.PodName)
			netnsValid = false
		}
	}

	if netnsValid {
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
	}

	if err := m.ipamDel(req.ContainerId); err != nil {
		return err
	}

	if err := m.hostportHandler.SyncHostports(TUN, m.getRunningPods()); err != nil {
		return err
	}

	return nil
}
