// +build linux

package plugin

import (
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/plugin/cniserver"
	"github.com/openshift/origin/pkg/util/ipcmd"
	"github.com/openshift/origin/pkg/util/ovs"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kcontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/dockertools"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	kubehostport "k8s.io/kubernetes/pkg/kubelet/network/hostport"
	kbandwidth "k8s.io/kubernetes/pkg/util/bandwidth"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	ksets "k8s.io/kubernetes/pkg/util/sets"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/ip"
	"github.com/containernetworking/cni/pkg/ipam"
	"github.com/containernetworking/cni/pkg/ns"
	cnitypes "github.com/containernetworking/cni/pkg/types"

	"github.com/vishvananda/netlink"
)

const (
	podInterfaceName = knetwork.DefaultInterfaceName
)

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

// Adds a macvlan interface to a container, if requested, for use with the egress router feature
func maybeAddMacvlan(pod *kapi.Pod, netns string) error {
	val, ok := pod.Annotations[sdnapi.AssignMacvlanAnnotation]
	if !ok || val != "true" {
		return nil
	}

	privileged := false
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext != nil && container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
			privileged = true
			break
		}
	}
	if !privileged {
		return fmt.Errorf("pod has %q annotation but is not privileged", sdnapi.AssignMacvlanAnnotation)
	}

	// Find interface with the default route
	var defIface netlink.Link
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

func ensureOvsPort(ovsif *ovs.Interface, hostVeth string) (int, error) {
	return ovsif.AddPort(hostVeth, -1)
}

func setupPodFlows(ovsif *ovs.Interface, ofport int, podIP, podMac string, vnid uint32) error {
	otx := ovsif.NewTransaction()

	// ARP/IP traffic from container
	otx.AddFlow("table=20, priority=100, in_port=%d, arp, nw_src=%s, arp_sha=%s, actions=load:%d->NXM_NX_REG0[], goto_table:21", ofport, podIP, podMac, vnid)
	otx.AddFlow("table=20, priority=100, in_port=%d, ip, nw_src=%s, actions=load:%d->NXM_NX_REG0[], goto_table:21", ofport, podIP, vnid)

	// ARP request/response to container (not isolated)
	otx.AddFlow("table=40, priority=100, arp, nw_dst=%s, actions=output:%d", podIP, ofport)

	// IP traffic to container
	otx.AddFlow("table=70, priority=100, ip, nw_dst=%s, actions=load:%d->NXM_NX_REG1[], load:%d->NXM_NX_REG2[], goto_table:80", podIP, vnid, ofport)

	return otx.EndTransaction()
}

func setupPodBandwidth(ovsif *ovs.Interface, pod *kapi.Pod, hostVeth string) error {
	podIngress, podEgress, err := kbandwidth.ExtractPodBandwidthResources(pod.Annotations)
	if err != nil {
		return fmt.Errorf("failed to parse pod bandwidth: %v", err)
	}
	if podIngress == nil && podEgress == nil {
		return nil
	}

	var ovsIngress, ovsEgress int64
	// note pod ingress == OVS egress and vice versa, and OVS ingress is in Kbps
	if podIngress != nil {
		ovsEgress = podIngress.Value()
	}
	if podEgress != nil {
		ovsIngress = podEgress.Value() / 1024
	}

	if ovsEgress > 0 {
		// FIXME: doesn't seem possible to do this with the netlink library?
		itx := ipcmd.NewTransaction(kexec.New(), hostVeth)
		itx.SetLink("qlen", "1000")
		err = itx.EndTransaction()
		if err != nil {
			return err
		}

		qos, err := ovsif.Create("qos", "type=linux-htb", fmt.Sprintf("other-config:max-rate=%d", ovsEgress))
		if err != nil {
			return err
		}
		err = ovsif.Set("port", hostVeth, fmt.Sprintf("qos=%s", qos))
		if err != nil {
			return err
		}
	}
	if ovsIngress > 0 {
		err := ovsif.Set("interface", hostVeth, fmt.Sprintf("ingress_policing_rate=%d", ovsIngress))
		if err != nil {
			return err
		}
	}

	return nil
}

func cleanupPodFlows(ovsif *ovs.Interface, podIP string) error {
	otx := ovsif.NewTransaction()
	otx.DeleteFlows("ip, nw_dst=%s", podIP)
	otx.DeleteFlows("ip, nw_src=%s", podIP)
	otx.DeleteFlows("arp, nw_dst=%s", podIP)
	otx.DeleteFlows("arp, nw_src=%s", podIP)
	return otx.EndTransaction()
}

func cleanupPodBandwidth(ovsif *ovs.Interface, hostVeth string) error {
	qos, err := ovsif.Get("port", hostVeth, "qos")
	if err != nil || qos == "[]" {
		return err
	}
	err = ovsif.Clear("port", hostVeth, "qos")
	if err != nil {
		return err
	}
	return ovsif.Destroy("qos", qos)
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
		return nil, fmt.Errorf("failed to retrieve pods from runtime: %v", err)
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
func (m *podManager) setup(req *cniserver.PodRequest) (*cnitypes.Result, *runningPod, error) {
	pod, err := m.kClient.Pods(req.PodNamespace).Get(req.PodName)
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
	newPod := &kubehostport.ActivePod{Pod: pod, IP: podIP}
	if err := m.hostportHandler.OpenPodHostportsAndSync(newPod, TUN, m.getRunningPods()); err != nil {
		return nil, nil, err
	}

	var hostVethName, contVethMac string
	err = ns.WithNetNSPath(req.Netns, func(hostNS ns.NetNS) error {
		hostVeth, contVeth, err := ip.SetupVeth(podInterfaceName, int(m.mtu), hostNS)
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

		hostVethName = hostVeth.Attrs().Name
		contVethMac = contVeth.Attrs().HardwareAddr.String()
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	vnid, err := m.policy.GetVNID(req.PodNamespace)
	if err != nil {
		return nil, nil, err
	}

	if err := maybeAddMacvlan(pod, req.Netns); err != nil {
		return nil, nil, err
	}

	ofport, err := ensureOvsPort(m.ovs, hostVethName)
	if err != nil {
		return nil, nil, err
	}
	if err := setupPodFlows(m.ovs, ofport, podIP.String(), contVethMac, vnid); err != nil {
		return nil, nil, err
	}
	if err := setupPodBandwidth(m.ovs, pod, hostVethName); err != nil {
		return nil, nil, err
	}

	m.policy.RefVNID(vnid)
	success = true
	return ipamResult, &runningPod{activePod: newPod, vnid: vnid, ofport: ofport}, nil
}

func (m *podManager) getContainerNetnsPath(id string) (string, error) {
	runtime, ok := m.host.GetRuntime().(*dockertools.DockerManager)
	if !ok {
		return "", fmt.Errorf("openshift-sdn execution called on non-docker runtime")
	}
	return runtime.GetNetNS(kcontainer.DockerID(id).ContainerID())
}

// Update OVS flows when something (like the pod's namespace VNID) changes
func (m *podManager) update(req *cniserver.PodRequest) (uint32, error) {
	// Updates may come at startup and thus we may not have the pod's
	// netns from kubelet (since kubelet doesn't have UPDATE actions).
	// Read the missing netns from the pod's file.
	if req.Netns == "" {
		netns, err := m.getContainerNetnsPath(req.ContainerId)
		if err != nil {
			return 0, err
		}
		req.Netns = netns
	}

	pod, err := m.kClient.Pods(req.PodNamespace).Get(req.PodName)
	if err != nil {
		return 0, err
	}

	hostVethName, contVethMac, podIP, err := getVethInfo(req.Netns, podInterfaceName)
	if err != nil {
		return 0, err
	}
	vnid, err := m.policy.GetVNID(req.PodNamespace)
	if err != nil {
		return 0, err
	}

	ofport, err := ensureOvsPort(m.ovs, hostVethName)
	if err != nil {
		return 0, err
	}
	if err := cleanupPodFlows(m.ovs, podIP); err != nil {
		return 0, err
	}
	if err := setupPodFlows(m.ovs, ofport, podIP, contVethMac, vnid); err != nil {
		return 0, err
	}
	if err := cleanupPodBandwidth(m.ovs, hostVethName); err != nil {
		return 0, err
	}
	if err := setupPodBandwidth(m.ovs, pod, hostVethName); err != nil {
		return 0, err
	}

	return vnid, nil
}

// Clean up all pod networking (clear OVS flows, release IPAM lease, remove host/container veth)
func (m *podManager) teardown(req *cniserver.PodRequest) error {
	errList := []error{}

	netnsValid := true
	if err := ns.IsNSorErr(req.Netns); err != nil {
		if _, ok := err.(ns.NSPathNotExistErr); ok {
			glog.V(3).Infof("teardown called on already-destroyed pod %s/%s; only cleaning up IPAM", req.PodNamespace, req.PodName)
			netnsValid = false
		}
	}

	if netnsValid {
		hostVethName, _, podIP, err := getVethInfo(req.Netns, podInterfaceName)
		if err != nil {
			return err
		}

		if err := cleanupPodFlows(m.ovs, podIP); err != nil {
			errList = append(errList, err)
		}
		if err := cleanupPodBandwidth(m.ovs, hostVethName); err != nil {
			errList = append(errList, err)
		}
		if err := m.ovs.DeletePort(hostVethName); err != nil {
			errList = append(errList, err)
		}

		if vnid, err := m.policy.GetVNID(req.PodNamespace); err == nil {
			m.policy.UnrefVNID(vnid)
		}
	}

	if err := m.ipamDel(req.ContainerId); err != nil {
		errList = append(errList, err)
	}

	if err := m.hostportHandler.SyncHostports(TUN, m.getRunningPods()); err != nil {
		errList = append(errList, err)
	}

	return kerrors.NewAggregate(errList)
}
