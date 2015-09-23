package lbr

import (
	"encoding/hex"
	"fmt"
	log "github.com/golang/glog"
	"net"
	"os/exec"

	"github.com/openshift/openshift-sdn/ovssubnet/api"
	"github.com/openshift/openshift-sdn/pkg/netutils"
)

type FlowController struct {
}

func NewFlowController() *FlowController {
	return &FlowController{}
}

func (c *FlowController) Setup(localSubnet, containerNetwork, servicesNetwork string, mtu uint) error {
	_, ipnet, err := net.ParseCIDR(localSubnet)
	subnetMaskLength, _ := ipnet.Mask.Size()
	out, err := exec.Command("openshift-sdn-simple-setup-node.sh", netutils.GenerateDefaultGateway(ipnet).String(), ipnet.String(), containerNetwork, fmt.Sprint(subnetMaskLength), fmt.Sprint(mtu)).CombinedOutput()
	log.Infof("Output of setup script:\n%s", out)
	if err != nil {
		log.Errorf("Error executing setup script. \n\tOutput: %s\n\tError: %v\n", out, err)
	}
	_, err = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0").CombinedOutput()
	return err
}

func (c *FlowController) AddOFRules(nodeIP, subnet, localIP string) error {
	cookie := generateCookie(nodeIP)
	if nodeIP == localIP {
		// self, so add the input rules
		iprule := fmt.Sprintf("table=0,cookie=0x%s,priority=200,ip,in_port=10,nw_dst=%s,actions=output:9", cookie, subnet)
		arprule := fmt.Sprintf("table=0,cookie=0x%s,priority=200,arp,in_port=10,nw_dst=%s,actions=output:9", cookie, subnet)
		o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", iprule).CombinedOutput()
		log.Infof("Output of adding %s: %s (%v)", iprule, o, e)
		o, e = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", arprule).CombinedOutput()
		log.Infof("Output of adding %s: %s (%v)", arprule, o, e)
		return e
	} else {
		iprule := fmt.Sprintf("table=0,cookie=0x%s,priority=200,ip,in_port=9,nw_dst=%s,actions=set_field:%s->tun_dst,output:10", cookie, subnet, nodeIP)
		arprule := fmt.Sprintf("table=0,cookie=0x%s,priority=200,arp,in_port=9,nw_dst=%s,actions=set_field:%s->tun_dst,output:10", cookie, subnet, nodeIP)
		o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", iprule).CombinedOutput()
		log.Infof("Output of adding %s: %s (%v)", iprule, o, e)
		o, e = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", arprule).CombinedOutput()
		log.Infof("Output of adding %s: %s (%v)", arprule, o, e)
		return e
	}
	return nil
}

func (c *FlowController) DelOFRules(node, localIP string) error {
	log.Infof("Calling del rules for %s.", node)
	cookie := generateCookie(node)
	if node == localIP {
		iprule := fmt.Sprintf("table=0,cookie=0x%s/0xffffffff,ip,in_port=10", cookie)
		arprule := fmt.Sprintf("table=0,cookie=0x%s/0xffffffff,arp,in_port=10", cookie)
		o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", iprule).CombinedOutput()
		log.Infof("Output of deleting local ip rules: %s (%v)", o, e)
		o, e = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", arprule).CombinedOutput()
		log.Infof("Output of deleting local arp rules: %s (%v)", o, e)
		return e
	} else {
		iprule := fmt.Sprintf("table=0,cookie=0x%s/0xffffffff,ip,in_port=9", cookie)
		arprule := fmt.Sprintf("table=0,cookie=0x%s/0xffffffff,arp,in_port=9", cookie)
		o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", iprule).CombinedOutput()
		log.Infof("Output of deleting %s: %s (%v)", iprule, o, e)
		o, e = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", arprule).CombinedOutput()
		log.Infof("Output of deleting %s: %s (%v)", arprule, o, e)
		return e
	}
	return nil
}

func generateCookie(ip string) string {
	return hex.EncodeToString(net.ParseIP(ip).To4())
}

func (c *FlowController) AddServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error {
	return nil
}

func (c *FlowController) DelServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error {
	return nil
}
