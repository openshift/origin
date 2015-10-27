package multitenant

import (
	"encoding/hex"
	"fmt"
	log "github.com/golang/glog"
	"net"
	"os/exec"
	"strings"
	"syscall"

	"github.com/openshift/openshift-sdn/pkg/netutils"
	"github.com/openshift/openshift-sdn/pkg/ovssubnet/api"
)

type FlowController struct {
}

func NewFlowController() *FlowController {
	return &FlowController{}
}

func (c *FlowController) Setup(localSubnetCIDR, clusterNetworkCIDR, servicesNetworkCIDR string, mtu uint) error {
	_, ipnet, err := net.ParseCIDR(localSubnetCIDR)
	localSubnetMaskLength, _ := ipnet.Mask.Size()
	localSubnetGateway := netutils.GenerateDefaultGateway(ipnet).String()
	out, err := exec.Command("openshift-sdn-multitenant-setup.sh", localSubnetGateway, localSubnetCIDR, fmt.Sprint(localSubnetMaskLength), clusterNetworkCIDR, servicesNetworkCIDR, fmt.Sprint(mtu)).CombinedOutput()
	log.Infof("Output of setup script:\n%s", out)
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			status := exitErr.ProcessState.Sys().(syscall.WaitStatus)
			if status.Exited() && status.ExitStatus() == 140 {
				// valid, do nothing, its just a benevolent restart
				return nil
			}
		}
		log.Errorf("Error executing setup script. \n\tOutput: %s\n\tError: %v\n", out, err)
		return err
	}
	return nil
}

func (c *FlowController) AddOFRules(nodeIP, nodeSubnetCIDR, localIP string) error {
	if nodeIP == localIP {
		return nil
	}

	cookie := generateCookie(nodeIP)
	iprule := fmt.Sprintf("table=7,cookie=0x%s,priority=100,ip,nw_dst=%s,actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", cookie, nodeSubnetCIDR, nodeIP)
	arprule := fmt.Sprintf("table=8,cookie=0x%s,priority=100,arp,nw_dst=%s,actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", cookie, nodeSubnetCIDR, nodeIP)
	o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", iprule).CombinedOutput()
	log.Infof("Output of adding %s: %s (%v)", iprule, o, e)
	o, e = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", arprule).CombinedOutput()
	log.Infof("Output of adding %s: %s (%v)", arprule, o, e)
	return e
}

func (c *FlowController) DelOFRules(nodeIP, localIP string) error {
	if nodeIP == localIP {
		return nil
	}

	log.Infof("Calling del rules for %s", nodeIP)
	cookie := generateCookie(nodeIP)
	iprule := fmt.Sprintf("table=7,cookie=0x%s/0xffffffff", cookie)
	arprule := fmt.Sprintf("table=8,cookie=0x%s/0xffffffff", cookie)
	o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", iprule).CombinedOutput()
	log.Infof("Output of deleting local ip rules %s (%v)", o, e)
	o, e = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", arprule).CombinedOutput()
	log.Infof("Output of deleting local arp rules %s (%v)", o, e)
	return e
}

func generateCookie(ip string) string {
	return hex.EncodeToString(net.ParseIP(ip).To4())
}

func (c *FlowController) AddServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error {
	rule := generateAddServiceRule(netID, IP, protocol, port)
	o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", rule).CombinedOutput()
	log.Infof("Output of adding %s: %s (%v)", rule, o, e)
	return e
}

func (c *FlowController) DelServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error {
	rule := generateDelServiceRule(IP, protocol, port)
	o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", rule).CombinedOutput()
	log.Infof("Output of deleting %s: %s (%v)", rule, o, e)
	return e
}

func generateBaseServiceRule(IP string, protocol api.ServiceProtocol, port uint) string {
	return fmt.Sprintf("table=4,%s,nw_dst=%s,tp_dst=%d", strings.ToLower(string(protocol)), IP, port)
}

func generateAddServiceRule(netID uint, IP string, protocol api.ServiceProtocol, port uint) string {
	baseRule := generateBaseServiceRule(IP, protocol, port)
	if netID == 0 {
		return fmt.Sprintf("%s,priority=200,actions=output:2", baseRule)
	} else {
		return fmt.Sprintf("%s,priority=200,reg0=%d,actions=output:2", baseRule, netID)
	}
}

func generateDelServiceRule(IP string, protocol api.ServiceProtocol, port uint) string {
	return generateBaseServiceRule(IP, protocol, port)
}

func (c *FlowController) UpdatePod(namespace, podName, containerID string, netID uint) error {
	out, err := exec.Command("openshift-ovs-multitenant", "update", namespace, podName, containerID, fmt.Sprint(netID)).CombinedOutput()
	log.V(5).Infof("UpdatePod output: %s, error: %v", out, err)
	return err
}
