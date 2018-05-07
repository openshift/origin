package node

import (
	"fmt"
	"net"
	"sync"
	"syscall"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
	networkinformers "github.com/openshift/origin/pkg/network/generated/informers/internalversion"

	"github.com/vishvananda/netlink"
)

type nodeEgress struct {
	nodeIP       string
	requestedIPs sets.String
}

type namespaceEgress struct {
	vnid         uint32
	requestedIPs []string
}

type egressIPInfo struct {
	ip string

	nodes      []*nodeEgress
	namespaces []*namespaceEgress

	assignedNodeIP       string
	assignedIPTablesMark string
}

type egressIPWatcher struct {
	sync.Mutex

	oc            *ovsController
	localIP       string
	masqueradeBit uint32

	networkInformers networkinformers.SharedInformerFactory
	iptables         *NodeIPTables

	nodesByNodeIP    map[string]*nodeEgress
	namespacesByVNID map[uint32]*namespaceEgress
	egressIPs        map[string]*egressIPInfo

	changedEgressIPs  []*egressIPInfo
	changedNamespaces []*namespaceEgress

	localEgressLink netlink.Link
	localEgressNet  *net.IPNet

	testModeChan chan string
}

func newEgressIPWatcher(oc *ovsController, localIP string, masqueradeBit *int32) *egressIPWatcher {
	eip := &egressIPWatcher{
		oc:      oc,
		localIP: localIP,

		nodesByNodeIP:    make(map[string]*nodeEgress),
		namespacesByVNID: make(map[uint32]*namespaceEgress),
		egressIPs:        make(map[string]*egressIPInfo),
	}
	if masqueradeBit != nil {
		eip.masqueradeBit = 1 << uint32(*masqueradeBit)
	}
	return eip
}

func (eip *egressIPWatcher) Start(networkInformers networkinformers.SharedInformerFactory, iptables *NodeIPTables) error {
	var err error
	if eip.localEgressLink, eip.localEgressNet, err = GetLinkDetails(eip.localIP); err != nil {
		// Not expected, should already be caught by node.New()
		return nil
	}

	eip.networkInformers = networkInformers
	eip.iptables = iptables

	eip.watchHostSubnets()
	eip.watchNetNamespaces()
	return nil
}

// Convert vnid to a hex value that is not 0, does not have masqueradeBit set, and isn't
// the same value as would be returned for any other valid vnid.
func getMarkForVNID(vnid, masqueradeBit uint32) string {
	if vnid == 0 {
		vnid = 0xff000000
	}
	if (vnid & masqueradeBit) != 0 {
		vnid = (vnid | 0x01000000) ^ masqueradeBit
	}
	return fmt.Sprintf("0x%08x", vnid)
}

func (eip *egressIPWatcher) ensureEgressIPInfo(egressIP string) *egressIPInfo {
	eg := eip.egressIPs[egressIP]
	if eg == nil {
		eg = &egressIPInfo{ip: egressIP}
		eip.egressIPs[egressIP] = eg
	}
	return eg
}

func (eip *egressIPWatcher) egressIPChanged(eg *egressIPInfo) {
	eip.changedEgressIPs = append(eip.changedEgressIPs, eg)
	for _, ns := range eg.namespaces {
		eip.changedNamespaces = append(eip.changedNamespaces, ns)
	}
}

func (eip *egressIPWatcher) addNode(egressIP string, node *nodeEgress) {
	eg := eip.ensureEgressIPInfo(egressIP)
	if len(eg.nodes) != 0 {
		utilruntime.HandleError(fmt.Errorf("Multiple nodes claiming EgressIP %q (nodes %q, %q)", eg.ip, node.nodeIP, eg.nodes[0].nodeIP))
	}
	eg.nodes = append(eg.nodes, node)

	eip.egressIPChanged(eg)
}

func (eip *egressIPWatcher) deleteNode(egressIP string, node *nodeEgress) {
	eg := eip.egressIPs[egressIP]
	if eg == nil {
		return
	}

	for i := range eg.nodes {
		if eg.nodes[i] == node {
			eip.egressIPChanged(eg)
			eg.nodes = append(eg.nodes[:i], eg.nodes[i+1:]...)
			return
		}
	}
}

func (eip *egressIPWatcher) addNamespace(egressIP string, ns *namespaceEgress) {
	eg := eip.ensureEgressIPInfo(egressIP)
	if len(eg.namespaces) != 0 {
		utilruntime.HandleError(fmt.Errorf("Multiple namespaces claiming EgressIP %q (NetIDs %d, %d)", eg.ip, ns.vnid, eg.namespaces[0].vnid))
	}
	eg.namespaces = append(eg.namespaces, ns)

	eip.egressIPChanged(eg)
}

func (eip *egressIPWatcher) deleteNamespace(egressIP string, ns *namespaceEgress) {
	eg := eip.egressIPs[egressIP]
	if eg == nil {
		return
	}

	for i := range eg.namespaces {
		if eg.namespaces[i] == ns {
			eip.egressIPChanged(eg)
			eg.namespaces = append(eg.namespaces[:i], eg.namespaces[i+1:]...)
			return
		}
	}
}

func (eip *egressIPWatcher) watchHostSubnets() {
	funcs := common.InformerFuncs(&networkapi.HostSubnet{}, eip.handleAddOrUpdateHostSubnet, eip.handleDeleteHostSubnet)
	eip.networkInformers.Network().InternalVersion().HostSubnets().Informer().AddEventHandler(funcs)
}

func (eip *egressIPWatcher) handleAddOrUpdateHostSubnet(obj, _ interface{}, eventType watch.EventType) {
	hs := obj.(*networkapi.HostSubnet)
	glog.V(5).Infof("Watch %s event for HostSubnet %q", eventType, hs.Name)

	eip.updateNodeEgress(hs.HostIP, hs.EgressIPs)
}

func (eip *egressIPWatcher) handleDeleteHostSubnet(obj interface{}) {
	hs := obj.(*networkapi.HostSubnet)
	glog.V(5).Infof("Watch %s event for HostSubnet %q", watch.Deleted, hs.Name)

	eip.updateNodeEgress(hs.HostIP, nil)
}

func (eip *egressIPWatcher) updateNodeEgress(nodeIP string, nodeEgressIPs []string) {
	eip.Lock()
	defer eip.Unlock()

	node := eip.nodesByNodeIP[nodeIP]
	if node == nil {
		if len(nodeEgressIPs) == 0 {
			return
		}
		node = &nodeEgress{
			nodeIP:       nodeIP,
			requestedIPs: sets.NewString(),
		}
		eip.nodesByNodeIP[nodeIP] = node
	} else if len(nodeEgressIPs) == 0 {
		delete(eip.nodesByNodeIP, nodeIP)
	}
	oldRequestedIPs := node.requestedIPs
	node.requestedIPs = sets.NewString(nodeEgressIPs...)

	// Process new and removed EgressIPs
	for _, ip := range node.requestedIPs.Difference(oldRequestedIPs).UnsortedList() {
		eip.addNode(ip, node)
	}
	for _, ip := range oldRequestedIPs.Difference(node.requestedIPs).UnsortedList() {
		eip.deleteNode(ip, node)
	}

	eip.syncEgressIPs()
}

func (eip *egressIPWatcher) watchNetNamespaces() {
	funcs := common.InformerFuncs(&networkapi.NetNamespace{}, eip.handleAddOrUpdateNetNamespace, eip.handleDeleteNetNamespace)
	eip.networkInformers.Network().InternalVersion().NetNamespaces().Informer().AddEventHandler(funcs)
}

func (eip *egressIPWatcher) handleAddOrUpdateNetNamespace(obj, _ interface{}, eventType watch.EventType) {
	netns := obj.(*networkapi.NetNamespace)
	glog.V(5).Infof("Watch %s event for NetNamespace %q", eventType, netns.Name)

	if len(netns.EgressIPs) != 0 {
		if len(netns.EgressIPs) > 1 {
			glog.Warningf("Ignoring extra EgressIPs (%v) in NetNamespace %q", netns.EgressIPs[1:], netns.Name)
		}
	}
	eip.updateNamespaceEgress(netns.NetID, netns.EgressIPs)
}

func (eip *egressIPWatcher) handleDeleteNetNamespace(obj interface{}) {
	netns := obj.(*networkapi.NetNamespace)
	glog.V(5).Infof("Watch %s event for NetNamespace %q", watch.Deleted, netns.Name)

	eip.deleteNamespaceEgress(netns.NetID)
}

func (eip *egressIPWatcher) updateNamespaceEgress(vnid uint32, egressIPs []string) {
	eip.Lock()
	defer eip.Unlock()

	ns := eip.namespacesByVNID[vnid]
	if ns == nil {
		if len(egressIPs) == 0 {
			return
		}
		ns = &namespaceEgress{vnid: vnid}
		eip.namespacesByVNID[vnid] = ns
	} else if len(egressIPs) == 0 {
		delete(eip.namespacesByVNID, vnid)
	}

	oldRequestedIPs := sets.NewString(ns.requestedIPs...)
	newRequestedIPs := sets.NewString(egressIPs...)
	ns.requestedIPs = egressIPs

	// Process new and removed EgressIPs
	for _, ip := range newRequestedIPs.Difference(oldRequestedIPs).UnsortedList() {
		eip.addNamespace(ip, ns)
	}
	for _, ip := range oldRequestedIPs.Difference(newRequestedIPs).UnsortedList() {
		eip.deleteNamespace(ip, ns)
	}

	// Make sure we update OVS even if nothing was added or removed; the order might
	// have changed
	eip.changedNamespaces = append(eip.changedNamespaces, ns)

	eip.syncEgressIPs()
}

func (eip *egressIPWatcher) deleteNamespaceEgress(vnid uint32) {
	eip.updateNamespaceEgress(vnid, nil)
}

func (eip *egressIPWatcher) syncEgressIPs() {
	changedEgressIPs := make(map[*egressIPInfo]bool)
	for _, eg := range eip.changedEgressIPs {
		changedEgressIPs[eg] = true
	}
	eip.changedEgressIPs = eip.changedEgressIPs[:0]

	changedNamespaces := make(map[*namespaceEgress]bool)
	for _, ns := range eip.changedNamespaces {
		changedNamespaces[ns] = true
	}
	eip.changedNamespaces = eip.changedNamespaces[:0]

	for eg := range changedEgressIPs {
		eip.syncEgressNodeState(eg)
	}

	for ns := range changedNamespaces {
		err := eip.syncEgressNamespaceState(ns)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error updating Namespace egress rules for VNID %d: %v", ns.vnid, err))
		}
	}
}

func (eip *egressIPWatcher) syncEgressNodeState(eg *egressIPInfo) {
	// The egressIPInfo should have an assigned node IP if and only if the
	// egress IP is active (ie, it is assigned to exactly 1 node and exactly
	// 1 namespace).
	egressIPActive := (len(eg.nodes) == 1 && len(eg.namespaces) == 1)
	if egressIPActive && eg.assignedNodeIP != eg.nodes[0].nodeIP {
		glog.V(4).Infof("Assigning egress IP %s to node %s", eg.ip, eg.nodes[0].nodeIP)
		eg.assignedNodeIP = eg.nodes[0].nodeIP
		eg.assignedIPTablesMark = getMarkForVNID(eg.namespaces[0].vnid, eip.masqueradeBit)
		if eg.assignedNodeIP == eip.localIP {
			if err := eip.assignEgressIP(eg.ip, eg.assignedIPTablesMark); err != nil {
				utilruntime.HandleError(fmt.Errorf("Error assigning Egress IP %q: %v", eg.ip, err))
				eg.assignedNodeIP = ""
			}
		}
	} else if !egressIPActive && eg.assignedNodeIP != "" {
		glog.V(4).Infof("Removing egress IP %s from node %s", eg.ip, eg.assignedNodeIP)
		if eg.assignedNodeIP == eip.localIP {
			if err := eip.releaseEgressIP(eg.ip, eg.assignedIPTablesMark); err != nil {
				utilruntime.HandleError(fmt.Errorf("Error releasing Egress IP %q: %v", eg.ip, err))
			}
		}
		eg.assignedNodeIP = ""
		eg.assignedIPTablesMark = ""
	} else if !egressIPActive {
		glog.V(4).Infof("Egress IP %s is not assignable (%d namespaces, %d nodes)", eg.ip, len(eg.namespaces), len(eg.nodes))
	}
}

func (eip *egressIPWatcher) syncEgressNamespaceState(ns *namespaceEgress) error {
	if len(ns.requestedIPs) == 0 {
		return eip.oc.SetNamespaceEgressNormal(ns.vnid)
	}

	var active *egressIPInfo
	for i, ip := range ns.requestedIPs {
		eg := eip.egressIPs[ip]
		if eg == nil {
			continue
		}
		if len(eg.namespaces) > 1 {
			active = nil
			glog.V(4).Infof("VNID %d gets no egress due to multiply-assigned egress IP %s", ns.vnid, eg.ip)
			break
		}
		if active == nil && i == 0 {
			if eg.assignedNodeIP == "" {
				glog.V(4).Infof("VNID %d cannot use unassigned egress IP %s", ns.vnid, eg.ip)
			} else {
				active = eg
			}
		}
	}

	if active != nil {
		return eip.oc.SetNamespaceEgressViaEgressIP(ns.vnid, active.assignedNodeIP, active.assignedIPTablesMark)
	} else {
		return eip.oc.SetNamespaceEgressDropped(ns.vnid)
	}
}

func (eip *egressIPWatcher) assignEgressIP(egressIP, mark string) error {
	if egressIP == eip.localIP {
		return fmt.Errorf("desired egress IP %q is the node IP", egressIP)
	}

	if eip.testModeChan != nil {
		eip.testModeChan <- fmt.Sprintf("claim %s", egressIP)
		return nil
	}

	localEgressIPMaskLen, _ := eip.localEgressNet.Mask.Size()
	egressIPNet := fmt.Sprintf("%s/%d", egressIP, localEgressIPMaskLen)
	addr, err := netlink.ParseAddr(egressIPNet)
	if err != nil {
		return fmt.Errorf("could not parse egress IP %q: %v", egressIPNet, err)
	}
	if !eip.localEgressNet.Contains(addr.IP) {
		return fmt.Errorf("egress IP %q is not in local network %s of interface %s", egressIP, eip.localEgressNet.String(), eip.localEgressLink.Attrs().Name)
	}
	err = netlink.AddrAdd(eip.localEgressLink, addr)
	if err != nil {
		if err == syscall.EEXIST {
			glog.V(2).Infof("Egress IP %q already exists on %s", egressIPNet, eip.localEgressLink.Attrs().Name)
		} else {
			return fmt.Errorf("could not add egress IP %q to %s: %v", egressIPNet, eip.localEgressLink.Attrs().Name, err)
		}
	}

	if err := eip.iptables.AddEgressIPRules(egressIP, mark); err != nil {
		return fmt.Errorf("could not add egress IP iptables rule: %v", err)
	}

	return nil
}

func (eip *egressIPWatcher) releaseEgressIP(egressIP, mark string) error {
	if egressIP == eip.localIP {
		return nil
	}

	if eip.testModeChan != nil {
		eip.testModeChan <- fmt.Sprintf("release %s", egressIP)
		return nil
	}

	localEgressIPMaskLen, _ := eip.localEgressNet.Mask.Size()
	egressIPNet := fmt.Sprintf("%s/%d", egressIP, localEgressIPMaskLen)
	addr, err := netlink.ParseAddr(egressIPNet)
	if err != nil {
		return fmt.Errorf("could not parse egress IP %q: %v", egressIPNet, err)
	}
	err = netlink.AddrDel(eip.localEgressLink, addr)
	if err != nil {
		if err == syscall.EADDRNOTAVAIL {
			glog.V(2).Infof("Could not delete egress IP %q from %s: no such address", egressIPNet, eip.localEgressLink.Attrs().Name)
		} else {
			return fmt.Errorf("could not delete egress IP %q from %s: %v", egressIPNet, eip.localEgressLink.Attrs().Name, err)
		}
	}

	if err := eip.iptables.DeleteEgressIPRules(egressIP, mark); err != nil {
		return fmt.Errorf("could not delete egress IP iptables rule: %v", err)
	}

	return nil
}
