// +build linux

package node

import (
	"fmt"
	"net"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	networkinformers "github.com/openshift/client-go/network/informers/externalversions"
	"github.com/openshift/origin/pkg/network/common"
	"github.com/vishvananda/netlink"
)

type egressIPWatcher struct {
	sync.Mutex

	tracker *common.EgressIPTracker

	oc            *ovsController
	localIP       string
	masqueradeBit uint32

	iptables     *NodeIPTables
	iptablesMark map[string]string

	vxlanMonitor *egressVXLANMonitor

	localEgressLink netlink.Link
	localEgressNet  *net.IPNet

	testModeChan chan string
}

func newEgressIPWatcher(oc *ovsController, localIP string, masqueradeBit *int32) *egressIPWatcher {
	eip := &egressIPWatcher{
		oc:      oc,
		localIP: localIP,

		iptablesMark: make(map[string]string),
	}
	if masqueradeBit != nil {
		eip.masqueradeBit = 1 << uint32(*masqueradeBit)
	}

	eip.tracker = common.NewEgressIPTracker(eip)
	return eip
}

func (eip *egressIPWatcher) Start(networkInformers networkinformers.SharedInformerFactory, iptables *NodeIPTables) error {
	var err error
	if eip.localEgressLink, eip.localEgressNet, err = GetLinkDetails(eip.localIP); err != nil {
		// Not expected, should already be caught by node.New()
		return nil
	}

	eip.iptables = iptables

	updates := make(chan *egressVXLANNode)
	eip.vxlanMonitor = newEgressVXLANMonitor(eip.oc.ovs, eip.tracker, updates)
	go eip.watchVXLAN(updates)

	eip.tracker.Start(networkInformers.Network().V1().HostSubnets(), networkInformers.Network().V1().NetNamespaces())
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

func (eip *egressIPWatcher) ClaimEgressIP(vnid uint32, egressIP, nodeIP string) {
	if nodeIP == eip.localIP {
		mark := getMarkForVNID(vnid, eip.masqueradeBit)
		eip.iptablesMark[egressIP] = mark
		if err := eip.assignEgressIP(egressIP, mark); err != nil {
			utilruntime.HandleError(fmt.Errorf("Error assigning Egress IP %q: %v", egressIP, err))
		}
	} else if eip.vxlanMonitor != nil {
		eip.vxlanMonitor.AddNode(nodeIP)
	}
}

func (eip *egressIPWatcher) ReleaseEgressIP(egressIP, nodeIP string) {
	if nodeIP == eip.localIP {
		mark := eip.iptablesMark[egressIP]
		delete(eip.iptablesMark, egressIP)
		if err := eip.releaseEgressIP(egressIP, mark); err != nil {
			utilruntime.HandleError(fmt.Errorf("Error releasing Egress IP %q: %v", egressIP, err))
		}
	} else if eip.vxlanMonitor != nil {
		eip.vxlanMonitor.RemoveNode(nodeIP)
	}
}

func (eip *egressIPWatcher) UpdateEgressCIDRs() {
}

func (eip *egressIPWatcher) SetNamespaceEgressNormal(vnid uint32) {
	if err := eip.oc.SetNamespaceEgressNormal(vnid); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error updating Namespace egress rules for VNID %d: %v", vnid, err))
	}
}

func (eip *egressIPWatcher) SetNamespaceEgressDropped(vnid uint32) {
	if err := eip.oc.SetNamespaceEgressDropped(vnid); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error updating Namespace egress rules for VNID %d: %v", vnid, err))
	}
}

func (eip *egressIPWatcher) SetNamespaceEgressViaEgressIP(vnid uint32, egressIP, nodeIP string) {
	mark := eip.iptablesMark[egressIP]
	if err := eip.oc.SetNamespaceEgressViaEgressIP(vnid, nodeIP, mark); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error updating Namespace egress rules for VNID %d: %v", vnid, err))
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
	// Use arping to try to update other hosts ARP caches, in case this IP was
	// previously active on another node. (Based on code from "ifup".)
	go func() {
		out, err := exec.Command("/sbin/arping", "-q", "-A", "-c", "1", "-I", eip.localEgressLink.Attrs().Name, egressIP).CombinedOutput()
		if err != nil {
			glog.Warningf("Failed to send ARP claim for egress IP %q: %v (%s)", egressIP, err, string(out))
			return
		}
		time.Sleep(2 * time.Second)
		_ = exec.Command("/sbin/arping", "-q", "-U", "-c", "1", "-I", eip.localEgressLink.Attrs().Name, egressIP).Run()
	}()

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

func (eip *egressIPWatcher) watchVXLAN(updates chan *egressVXLANNode) {
	for node := range updates {
		eip.tracker.SetNodeOffline(node.nodeIP, node.offline)
	}
}
