package common

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	networkinformers "github.com/openshift/origin/pkg/network/generated/informers/internalversion/network/internalversion"
	"github.com/openshift/origin/pkg/util/netutils"
)

type nodeEgress struct {
	nodeIP       string
	sdnIP        string
	requestedIPs sets.String
	offline      bool
}

type namespaceEgress struct {
	vnid         uint32
	requestedIPs []string

	activeEgressIP string
}

type egressIPInfo struct {
	ip string

	nodes      []*nodeEgress
	namespaces []*namespaceEgress

	assignedNodeIP string
	assignedVNID   uint32
}

type EgressIPWatcher interface {
	ClaimEgressIP(vnid uint32, egressIP, nodeIP string)
	ReleaseEgressIP(egressIP, nodeIP string)

	SetNamespaceEgressNormal(vnid uint32)
	SetNamespaceEgressDropped(vnid uint32)
	SetNamespaceEgressViaEgressIP(vnid uint32, egressIP, nodeIP string)
}

type EgressIPTracker struct {
	sync.Mutex

	watcher EgressIPWatcher

	nodesByNodeIP    map[string]*nodeEgress
	namespacesByVNID map[uint32]*namespaceEgress
	egressIPs        map[string]*egressIPInfo

	changedEgressIPs  map[*egressIPInfo]bool
	changedNamespaces map[*namespaceEgress]bool
}

func NewEgressIPTracker(watcher EgressIPWatcher) *EgressIPTracker {
	return &EgressIPTracker{
		watcher: watcher,

		nodesByNodeIP:    make(map[string]*nodeEgress),
		namespacesByVNID: make(map[uint32]*namespaceEgress),
		egressIPs:        make(map[string]*egressIPInfo),

		changedEgressIPs:  make(map[*egressIPInfo]bool),
		changedNamespaces: make(map[*namespaceEgress]bool),
	}
}

func (eit *EgressIPTracker) Start(hostSubnetInformer networkinformers.HostSubnetInformer, netNamespaceInformer networkinformers.NetNamespaceInformer) {
	eit.watchHostSubnets(hostSubnetInformer)
	eit.watchNetNamespaces(netNamespaceInformer)
}

func (eit *EgressIPTracker) ensureEgressIPInfo(egressIP string) *egressIPInfo {
	eg := eit.egressIPs[egressIP]
	if eg == nil {
		eg = &egressIPInfo{ip: egressIP}
		eit.egressIPs[egressIP] = eg
	}
	return eg
}

func (eit *EgressIPTracker) egressIPChanged(eg *egressIPInfo) {
	eit.changedEgressIPs[eg] = true
	for _, ns := range eg.namespaces {
		eit.changedNamespaces[ns] = true
	}
}

func (eit *EgressIPTracker) addNodeEgressIP(node *nodeEgress, egressIP string) {
	eg := eit.ensureEgressIPInfo(egressIP)
	eg.nodes = append(eg.nodes, node)

	eit.egressIPChanged(eg)
}

func (eit *EgressIPTracker) deleteNodeEgressIP(node *nodeEgress, egressIP string) {
	eg := eit.egressIPs[egressIP]
	if eg == nil {
		return
	}

	for i := range eg.nodes {
		if eg.nodes[i] == node {
			eit.egressIPChanged(eg)
			eg.nodes = append(eg.nodes[:i], eg.nodes[i+1:]...)
			return
		}
	}
}

func (eit *EgressIPTracker) addNamespaceEgressIP(ns *namespaceEgress, egressIP string) {
	eg := eit.ensureEgressIPInfo(egressIP)
	eg.namespaces = append(eg.namespaces, ns)

	eit.egressIPChanged(eg)
}

func (eit *EgressIPTracker) deleteNamespaceEgressIP(ns *namespaceEgress, egressIP string) {
	eg := eit.egressIPs[egressIP]
	if eg == nil {
		return
	}

	for i := range eg.namespaces {
		if eg.namespaces[i] == ns {
			eit.egressIPChanged(eg)
			eg.namespaces = append(eg.namespaces[:i], eg.namespaces[i+1:]...)
			return
		}
	}
}

func (eit *EgressIPTracker) watchHostSubnets(hostSubnetInformer networkinformers.HostSubnetInformer) {
	funcs := InformerFuncs(&networkapi.HostSubnet{}, eit.handleAddOrUpdateHostSubnet, eit.handleDeleteHostSubnet)
	hostSubnetInformer.Informer().AddEventHandler(funcs)
}

func (eit *EgressIPTracker) handleAddOrUpdateHostSubnet(obj, _ interface{}, eventType watch.EventType) {
	hs := obj.(*networkapi.HostSubnet)
	glog.V(5).Infof("Watch %s event for HostSubnet %q", eventType, hs.Name)

	eit.UpdateHostSubnetEgress(hs)
}

func (eit *EgressIPTracker) handleDeleteHostSubnet(obj interface{}) {
	hs := obj.(*networkapi.HostSubnet)
	glog.V(5).Infof("Watch %s event for HostSubnet %q", watch.Deleted, hs.Name)

	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		HostIP: hs.HostIP,
	})
}

func (eit *EgressIPTracker) UpdateHostSubnetEgress(hs *networkapi.HostSubnet) {
	eit.Lock()
	defer eit.Unlock()

	sdnIP := ""
	if hs.Subnet != "" {
		_, cidr, err := net.ParseCIDR(hs.Subnet)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("could not parse HostSubnet %q CIDR: %v", hs.Name, err))
		}
		sdnIP = netutils.GenerateDefaultGateway(cidr).String()
	}

	node := eit.nodesByNodeIP[hs.HostIP]
	if node == nil {
		if len(hs.EgressIPs) == 0 {
			return
		}
		node = &nodeEgress{
			nodeIP:       hs.HostIP,
			sdnIP:        sdnIP,
			requestedIPs: sets.NewString(),
		}
		eit.nodesByNodeIP[hs.HostIP] = node
	} else if len(hs.EgressIPs) == 0 {
		delete(eit.nodesByNodeIP, hs.HostIP)
	}
	oldRequestedIPs := node.requestedIPs
	node.requestedIPs = sets.NewString(hs.EgressIPs...)

	// Process new and removed EgressIPs
	for _, ip := range node.requestedIPs.Difference(oldRequestedIPs).UnsortedList() {
		eit.addNodeEgressIP(node, ip)
	}
	for _, ip := range oldRequestedIPs.Difference(node.requestedIPs).UnsortedList() {
		eit.deleteNodeEgressIP(node, ip)
	}

	eit.syncEgressIPs()
}

func (eit *EgressIPTracker) watchNetNamespaces(netNamespaceInformer networkinformers.NetNamespaceInformer) {
	funcs := InformerFuncs(&networkapi.NetNamespace{}, eit.handleAddOrUpdateNetNamespace, eit.handleDeleteNetNamespace)
	netNamespaceInformer.Informer().AddEventHandler(funcs)
}

func (eit *EgressIPTracker) handleAddOrUpdateNetNamespace(obj, _ interface{}, eventType watch.EventType) {
	netns := obj.(*networkapi.NetNamespace)
	glog.V(5).Infof("Watch %s event for NetNamespace %q", eventType, netns.Name)

	eit.UpdateNetNamespaceEgress(netns)
}

func (eit *EgressIPTracker) handleDeleteNetNamespace(obj interface{}) {
	netns := obj.(*networkapi.NetNamespace)
	glog.V(5).Infof("Watch %s event for NetNamespace %q", watch.Deleted, netns.Name)

	eit.DeleteNetNamespaceEgress(netns.NetID)
}

func (eit *EgressIPTracker) UpdateNetNamespaceEgress(netns *networkapi.NetNamespace) {
	eit.Lock()
	defer eit.Unlock()

	ns := eit.namespacesByVNID[netns.NetID]
	if ns == nil {
		if len(netns.EgressIPs) == 0 {
			return
		}
		ns = &namespaceEgress{vnid: netns.NetID}
		eit.namespacesByVNID[netns.NetID] = ns
	} else if len(netns.EgressIPs) == 0 {
		delete(eit.namespacesByVNID, netns.NetID)
	}

	oldRequestedIPs := sets.NewString(ns.requestedIPs...)
	newRequestedIPs := sets.NewString(netns.EgressIPs...)
	ns.requestedIPs = netns.EgressIPs

	// Process new and removed EgressIPs
	for _, ip := range newRequestedIPs.Difference(oldRequestedIPs).UnsortedList() {
		eit.addNamespaceEgressIP(ns, ip)
	}
	for _, ip := range oldRequestedIPs.Difference(newRequestedIPs).UnsortedList() {
		eit.deleteNamespaceEgressIP(ns, ip)
	}

	// Even IPs that weren't added/removed need to be considered "changed", to
	// ensure we correctly process reorderings, duplicates added/removed, etc.
	for _, ip := range newRequestedIPs.Intersection(oldRequestedIPs).UnsortedList() {
		eit.egressIPChanged(eit.egressIPs[ip])
	}

	eit.syncEgressIPs()
}

func (eit *EgressIPTracker) DeleteNetNamespaceEgress(vnid uint32) {
	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetID: vnid,
	})
}

func (eit *EgressIPTracker) egressIPActive(eg *egressIPInfo) (bool, error) {
	if len(eg.nodes) == 0 || len(eg.namespaces) == 0 {
		return false, nil
	}
	if len(eg.nodes) > 1 {
		return false, fmt.Errorf("Multiple nodes (%s, %s) claiming EgressIP %s", eg.nodes[0].nodeIP, eg.nodes[1].nodeIP, eg.ip)
	}
	if len(eg.namespaces) > 1 {
		return false, fmt.Errorf("Multiple namespaces (%d, %d) claiming EgressIP %s", eg.namespaces[0].vnid, eg.namespaces[1].vnid, eg.ip)
	}
	for _, ip := range eg.namespaces[0].requestedIPs {
		eg2 := eit.egressIPs[ip]
		if eg2 != eg && len(eg2.nodes) == 1 && eg2.nodes[0] == eg.nodes[0] {
			return false, fmt.Errorf("Multiple EgressIPs (%s, %s) for VNID %d on node %s", eg.ip, eg2.ip, eg.namespaces[0].vnid, eg.nodes[0].nodeIP)
		}
	}
	return true, nil
}

func (eit *EgressIPTracker) syncEgressIPs() {
	changedEgressIPs := eit.changedEgressIPs
	eit.changedEgressIPs = make(map[*egressIPInfo]bool)

	changedNamespaces := eit.changedNamespaces
	eit.changedNamespaces = make(map[*namespaceEgress]bool)

	for eg := range changedEgressIPs {
		active, err := eit.egressIPActive(eg)
		if err != nil {
			utilruntime.HandleError(err)
		}
		eit.syncEgressNodeState(eg, active)
	}

	for ns := range changedNamespaces {
		eit.syncEgressNamespaceState(ns)
	}
}

func (eit *EgressIPTracker) syncEgressNodeState(eg *egressIPInfo, active bool) {
	if active && eg.assignedNodeIP != eg.nodes[0].nodeIP {
		glog.V(4).Infof("Assigning egress IP %s to node %s", eg.ip, eg.nodes[0].nodeIP)
		eg.assignedNodeIP = eg.nodes[0].nodeIP
		eit.watcher.ClaimEgressIP(eg.namespaces[0].vnid, eg.ip, eg.assignedNodeIP)
	} else if !active && eg.assignedNodeIP != "" {
		glog.V(4).Infof("Removing egress IP %s from node %s", eg.ip, eg.assignedNodeIP)
		eit.watcher.ReleaseEgressIP(eg.ip, eg.assignedNodeIP)
		eg.assignedNodeIP = ""
	}
}

func (eit *EgressIPTracker) syncEgressNamespaceState(ns *namespaceEgress) {
	if len(ns.requestedIPs) == 0 {
		if ns.activeEgressIP != "" {
			ns.activeEgressIP = ""
			eit.watcher.SetNamespaceEgressNormal(ns.vnid)
		}
		return
	}

	var active *egressIPInfo
	for _, ip := range ns.requestedIPs {
		eg := eit.egressIPs[ip]
		if eg == nil {
			continue
		}
		if len(eg.namespaces) > 1 {
			active = nil
			glog.V(4).Infof("VNID %d gets no egress due to multiply-assigned egress IP %s", ns.vnid, eg.ip)
			break
		}
		if active == nil {
			if eg.assignedNodeIP == "" {
				glog.V(4).Infof("VNID %d cannot use unassigned egress IP %s", ns.vnid, eg.ip)
			} else if len(ns.requestedIPs) > 1 && eg.nodes[0].offline {
				glog.V(4).Infof("VNID %d cannot use egress IP %s on offline node %s", ns.vnid, eg.ip, eg.assignedNodeIP)
			} else {
				active = eg
			}
		}
	}

	if active != nil {
		if ns.activeEgressIP != active.ip {
			ns.activeEgressIP = active.ip
			eit.watcher.SetNamespaceEgressViaEgressIP(ns.vnid, active.ip, active.assignedNodeIP)
		}
	} else {
		if ns.activeEgressIP != "dropped" {
			ns.activeEgressIP = "dropped"
			eit.watcher.SetNamespaceEgressDropped(ns.vnid)
		}
	}
}

func (eit *EgressIPTracker) SetNodeOffline(nodeIP string, offline bool) {
	eit.Lock()
	defer eit.Unlock()

	node := eit.nodesByNodeIP[nodeIP]
	if node == nil {
		return
	}

	node.offline = offline
	for _, ip := range node.requestedIPs.UnsortedList() {
		eg := eit.egressIPs[ip]
		if eg != nil {
			eit.egressIPChanged(eg)
		}
	}
	eit.syncEgressIPs()
}

// Ping a node and return whether or not it is online. We do this by trying to open a TCP
// connection to the "discard" service (port 9); if the node is offline, the attempt will
// time out with no response (and we will return false). If the node is online then we
// presumably will get a "connection refused" error; the code below assumes that anything
// other than timing out indicates that the node is online.
func (eit *EgressIPTracker) Ping(ip string, timeout time.Duration) bool {
	eit.Lock()
	defer eit.Unlock()

	// If the caller used a public node IP, replace it with the SDN IP
	if node := eit.nodesByNodeIP[ip]; node != nil {
		ip = node.sdnIP
	}

	conn, err := net.DialTimeout("tcp", ip+":9", timeout)
	if conn != nil {
		conn.Close()
	}
	if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
		return false
	} else {
		return true
	}
}
