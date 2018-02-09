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
	nodeIP string

	// requestedIPs are the EgressIPs listed on the node's HostSubnet
	requestedIPs sets.String
	// assignedIPs are the IPs actually in use on the node
	assignedIPs sets.String
}

type namespaceEgress struct {
	vnid uint32

	// requestedIP is the egress IP it wants (NetNamespace.EgressIPs[0])
	requestedIP string
	// assignedIP is an egress IP actually in use on nodeIP
	assignedIP string
	nodeIP     string
}

type egressIPWatcher struct {
	sync.Mutex

	oc            *ovsController
	localIP       string
	masqueradeBit uint32

	networkInformers networkinformers.SharedInformerFactory
	iptables         *NodeIPTables

	// from HostSubnets
	nodesByNodeIP   map[string]*nodeEgress
	nodesByEgressIP map[string]*nodeEgress

	// From NetNamespaces
	namespacesByVNID     map[uint32]*namespaceEgress
	namespacesByEgressIP map[string]*namespaceEgress

	localEgressLink netlink.Link
	localEgressNet  *net.IPNet

	testModeChan chan string
}

func newEgressIPWatcher(oc *ovsController, localIP string, masqueradeBit *int32) *egressIPWatcher {
	eip := &egressIPWatcher{
		oc:      oc,
		localIP: localIP,

		nodesByNodeIP:   make(map[string]*nodeEgress),
		nodesByEgressIP: make(map[string]*nodeEgress),

		namespacesByVNID:     make(map[uint32]*namespaceEgress),
		namespacesByEgressIP: make(map[string]*namespaceEgress),
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
			assignedIPs:  sets.NewString(),
		}
		eip.nodesByNodeIP[nodeIP] = node
	} else if len(nodeEgressIPs) == 0 {
		delete(eip.nodesByNodeIP, nodeIP)
	}
	oldRequestedIPs := node.requestedIPs
	node.requestedIPs = sets.NewString(nodeEgressIPs...)

	// Process new EgressIPs
	for _, ip := range node.requestedIPs.Difference(oldRequestedIPs).UnsortedList() {
		if oldNode := eip.nodesByEgressIP[ip]; oldNode != nil {
			utilruntime.HandleError(fmt.Errorf("Multiple nodes claiming EgressIP %q (nodes %q, %q)", ip, node.nodeIP, oldNode.nodeIP))
			continue
		}

		eip.nodesByEgressIP[ip] = node
		eip.maybeAddEgressIP(ip)
	}

	// Process removed EgressIPs
	for _, ip := range oldRequestedIPs.Difference(node.requestedIPs).UnsortedList() {
		if oldNode := eip.nodesByEgressIP[ip]; oldNode != node {
			// User removed a duplicate EgressIP
			continue
		}

		eip.deleteEgressIP(ip)
		delete(eip.nodesByEgressIP, ip)
	}
}

func (eip *egressIPWatcher) maybeAddEgressIP(egressIP string) {
	node := eip.nodesByEgressIP[egressIP]
	ns := eip.namespacesByEgressIP[egressIP]
	if ns == nil {
		return
	}

	mark := getMarkForVNID(ns.vnid, eip.masqueradeBit)
	nodeIP := ""

	if node != nil && !node.assignedIPs.Has(egressIP) {
		node.assignedIPs.Insert(egressIP)
		nodeIP = node.nodeIP
		if node.nodeIP == eip.localIP {
			if err := eip.assignEgressIP(egressIP, mark); err != nil {
				utilruntime.HandleError(fmt.Errorf("Error assigning Egress IP %q: %v", egressIP, err))
				nodeIP = ""
			}
		}
	}

	if ns.assignedIP != egressIP || ns.nodeIP != nodeIP {
		ns.assignedIP = egressIP
		ns.nodeIP = nodeIP

		err := eip.oc.UpdateNamespaceEgressRules(ns.vnid, ns.nodeIP, mark)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error updating Namespace egress rules: %v", err))
		}
	}
}

func (eip *egressIPWatcher) deleteEgressIP(egressIP string) {
	node := eip.nodesByEgressIP[egressIP]
	ns := eip.namespacesByEgressIP[egressIP]
	if node == nil || ns == nil {
		return
	}

	mark := getMarkForVNID(ns.vnid, eip.masqueradeBit)
	if node.nodeIP == eip.localIP {
		if err := eip.releaseEgressIP(egressIP, mark); err != nil {
			utilruntime.HandleError(fmt.Errorf("Error releasing Egress IP %q: %v", egressIP, err))
		}
	}

	if ns.assignedIP == egressIP {
		ns.assignedIP = ""
		ns.nodeIP = ""
	}

	var err error
	if ns.requestedIP == "" {
		// Namespace no longer wants EgressIP
		err = eip.oc.UpdateNamespaceEgressRules(ns.vnid, "", "")
	} else {
		// Namespace still wants EgressIP but no node provides it
		err = eip.oc.UpdateNamespaceEgressRules(ns.vnid, "", mark)
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error updating Namespace egress rules: %v", err))
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
		ns = &namespaceEgress{vnid: vnid}
		eip.namespacesByVNID[vnid] = ns
	}
	if ns.requestedIP == egressIP {
		return
	}
	if oldNS := eip.namespacesByEgressIP[egressIP]; oldNS != nil {
		utilruntime.HandleError(fmt.Errorf("Multiple NetNamespaces claiming EgressIP %q (NetIDs %d, %d)", egressIP, ns.vnid, oldNS.vnid))
		return
	}

	if ns.assignedIP != "" {
		eip.deleteEgressIP(egressIP)
		delete(eip.namespacesByEgressIP, egressIP)
		ns.assignedIP = ""
		ns.nodeIP = ""
	}
	ns.requestedIP = egressIP
	eip.namespacesByEgressIP[egressIP] = ns
	eip.maybeAddEgressIP(egressIP)
}

func (eip *egressIPWatcher) deleteNamespaceEgress(vnid uint32) {
	eip.Lock()
	defer eip.Unlock()

	ns := eip.namespacesByVNID[vnid]
	if ns == nil {
		return
	}

	if ns.assignedIP != "" {
		ns.requestedIP = ""
		egressIP := ns.assignedIP
		eip.deleteEgressIP(egressIP)
		delete(eip.namespacesByEgressIP, egressIP)
	}
	delete(eip.namespacesByVNID, vnid)
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
