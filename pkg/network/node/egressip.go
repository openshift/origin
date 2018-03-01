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
	vnid        uint32
	requestedIP string
}

type egressIPInfo struct {
	ip string

	nodes      []*nodeEgress
	namespaces []*namespaceEgress

	assignedNodeIP       string
	assignedIPTablesMark string
	assignedVNID         uint32
	blockedVNIDs         map[uint32]bool
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

func (eg *egressIPInfo) addNode(node *nodeEgress) {
	if len(eg.nodes) != 0 {
		utilruntime.HandleError(fmt.Errorf("Multiple nodes claiming EgressIP %q (nodes %q, %q)", eg.ip, node.nodeIP, eg.nodes[0].nodeIP))
	}
	eg.nodes = append(eg.nodes, node)
}

func (eg *egressIPInfo) deleteNode(node *nodeEgress) {
	for i := range eg.nodes {
		if eg.nodes[i] == node {
			eg.nodes = append(eg.nodes[:i], eg.nodes[i+1:]...)
			return
		}
	}
}

func (eg *egressIPInfo) addNamespace(ns *namespaceEgress) {
	if len(eg.namespaces) != 0 {
		utilruntime.HandleError(fmt.Errorf("Multiple namespaces claiming EgressIP %q (NetIDs %d, %d)", eg.ip, ns.vnid, eg.namespaces[0].vnid))
	}
	eg.namespaces = append(eg.namespaces, ns)
}

func (eg *egressIPInfo) deleteNamespace(ns *namespaceEgress) {
	for i := range eg.namespaces {
		if eg.namespaces[i] == ns {
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

	// Process new EgressIPs
	for _, ip := range node.requestedIPs.Difference(oldRequestedIPs).UnsortedList() {
		eg := eip.ensureEgressIPInfo(ip)
		eg.addNode(node)
		eip.syncEgressIP(eg)
	}

	// Process removed EgressIPs
	for _, ip := range oldRequestedIPs.Difference(node.requestedIPs).UnsortedList() {
		eg := eip.egressIPs[ip]
		if eg == nil {
			continue
		}
		eg.deleteNode(node)
		eip.syncEgressIP(eg)
	}
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
		eip.updateNamespaceEgress(netns.NetID, netns.EgressIPs[0])
	} else {
		eip.deleteNamespaceEgress(netns.NetID)
	}
}

func (eip *egressIPWatcher) handleDeleteNetNamespace(obj interface{}) {
	netns := obj.(*networkapi.NetNamespace)
	glog.V(5).Infof("Watch %s event for NetNamespace %q", watch.Deleted, netns.Name)

	eip.deleteNamespaceEgress(netns.NetID)
}

func (eip *egressIPWatcher) updateNamespaceEgress(vnid uint32, egressIP string) {
	eip.Lock()
	defer eip.Unlock()

	ns := eip.namespacesByVNID[vnid]
	if ns == nil {
		if egressIP == "" {
			return
		}
		ns = &namespaceEgress{vnid: vnid}
		eip.namespacesByVNID[vnid] = ns
	} else if egressIP == "" {
		delete(eip.namespacesByVNID, vnid)
	}

	if ns.requestedIP == egressIP {
		return
	}

	if ns.requestedIP != "" {
		eg := eip.egressIPs[ns.requestedIP]
		if eg != nil {
			eg.deleteNamespace(ns)
			eip.syncEgressIP(eg)
		}
	}

	ns.requestedIP = egressIP
	if egressIP == "" {
		return
	}

	eg := eip.ensureEgressIPInfo(egressIP)
	eg.addNamespace(ns)
	eip.syncEgressIP(eg)
}

func (eip *egressIPWatcher) deleteNamespaceEgress(vnid uint32) {
	eip.updateNamespaceEgress(vnid, "")
}

func (eip *egressIPWatcher) syncEgressIP(eg *egressIPInfo) {
	assignedNodeIPChanged := eip.syncEgressIPTablesState(eg)
	eip.syncEgressOVSState(eg, assignedNodeIPChanged)
}

func (eip *egressIPWatcher) syncEgressIPTablesState(eg *egressIPInfo) bool {
	// The egressIPInfo should have an assigned node IP if and only if the
	// egress IP is active (ie, it is assigned to exactly 1 node and exactly
	// 1 namespace).
	egressIPActive := (len(eg.nodes) == 1 && len(eg.namespaces) == 1)
	assignedNodeIPChanged := false
	if egressIPActive && eg.assignedNodeIP != eg.nodes[0].nodeIP {
		eg.assignedNodeIP = eg.nodes[0].nodeIP
		eg.assignedIPTablesMark = getMarkForVNID(eg.namespaces[0].vnid, eip.masqueradeBit)
		assignedNodeIPChanged = true
		if eg.assignedNodeIP == eip.localIP {
			if err := eip.assignEgressIP(eg.ip, eg.assignedIPTablesMark); err != nil {
				utilruntime.HandleError(fmt.Errorf("Error assigning Egress IP %q: %v", eg.ip, err))
				eg.assignedNodeIP = ""
			}
		}
	} else if !egressIPActive && eg.assignedNodeIP != "" {
		if eg.assignedNodeIP == eip.localIP {
			if err := eip.releaseEgressIP(eg.ip, eg.assignedIPTablesMark); err != nil {
				utilruntime.HandleError(fmt.Errorf("Error releasing Egress IP %q: %v", eg.ip, err))
			}
		}
		eg.assignedNodeIP = ""
		eg.assignedIPTablesMark = ""
		assignedNodeIPChanged = true
	}
	return assignedNodeIPChanged
}

func (eip *egressIPWatcher) syncEgressOVSState(eg *egressIPInfo, assignedNodeIPChanged bool) {
	var blockedVNIDs map[uint32]bool

	// If multiple namespaces are assigned to the same EgressIP, we need to block
	// outgoing traffic from all of them.
	if len(eg.namespaces) > 1 {
		eg.assignedVNID = 0
		blockedVNIDs = make(map[uint32]bool)
		for _, ns := range eg.namespaces {
			blockedVNIDs[ns.vnid] = true
			if !eg.blockedVNIDs[ns.vnid] {
				err := eip.oc.SetNamespaceEgressDropped(ns.vnid)
				if err != nil {
					utilruntime.HandleError(fmt.Errorf("Error updating Namespace egress rules: %v", err))
				}
			}
		}
	}

	// If we have, or had, a single egress namespace, then update the OVS flows if
	// something has changed
	var err error
	if len(eg.namespaces) == 1 && (eg.assignedVNID != eg.namespaces[0].vnid || assignedNodeIPChanged) {
		eg.assignedVNID = eg.namespaces[0].vnid
		delete(eg.blockedVNIDs, eg.assignedVNID)
		err = eip.oc.SetNamespaceEgressViaEgressIP(eg.assignedVNID, eg.assignedNodeIP, getMarkForVNID(eg.assignedVNID, eip.masqueradeBit))
	} else if len(eg.namespaces) == 0 && eg.assignedVNID != 0 {
		err = eip.oc.SetNamespaceEgressNormal(eg.assignedVNID)
		eg.assignedVNID = 0
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error updating Namespace egress rules: %v", err))
	}

	// If we previously had blocked VNIDs, we need to unblock any that have been removed
	// from the duplicates list
	for vnid := range eg.blockedVNIDs {
		if !blockedVNIDs[vnid] {
			err := eip.oc.SetNamespaceEgressNormal(vnid)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("Error updating Namespace egress rules: %v", err))
			}
		}
	}
	eg.blockedVNIDs = blockedVNIDs
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
