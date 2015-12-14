package ovs

import (
	"encoding/hex"
	"fmt"
	"github.com/golang/glog"
	"net"
	"os/exec"
	"strings"
	"syscall"

	"github.com/openshift/openshift-sdn/pkg/netutils"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"
)

type FlowController struct {
	multitenant bool
}

func NewFlowController(multitenant bool) *FlowController {
	return &FlowController{multitenant}
}

func (c *FlowController) Setup(localSubnetCIDR, clusterNetworkCIDR, servicesNetworkCIDR string, mtu uint) error {
	_, ipnet, err := net.ParseCIDR(localSubnetCIDR)
	localSubnetMaskLength, _ := ipnet.Mask.Size()
	localSubnetGateway := netutils.GenerateDefaultGateway(ipnet).String()
	out, err := exec.Command("openshift-sdn-ovs-setup.sh", localSubnetGateway, localSubnetCIDR, fmt.Sprint(localSubnetMaskLength), clusterNetworkCIDR, servicesNetworkCIDR, fmt.Sprint(mtu), fmt.Sprint(c.multitenant)).CombinedOutput()
	if err != nil {
		glog.Infof("Output of setup script:\n%s", out)
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			status := exitErr.ProcessState.Sys().(syscall.WaitStatus)
			if status.Exited() && status.ExitStatus() == 140 {
				// valid, do nothing, its just a benevolent restart
				return nil
			}
		}
		glog.Errorf("Error executing setup script: %v\n", err)
		return err
	} else {
		glog.V(5).Infof("Output of setup script:\n%s", out)
	}
	return nil
}

func (c *FlowController) GetName() string {
	if c.multitenant {
		return MultiTenantPluginName()
	} else {
		return SingleTenantPluginName()
	}
}

func (c *FlowController) AddOFRules(nodeIP, nodeSubnetCIDR, localIP string) error {
	if nodeIP == localIP {
		return nil
	}

	glog.V(5).Infof("AddOFRules for %s", nodeIP)
	cookie := generateCookie(nodeIP)

	inrule := fmt.Sprintf("table=0,cookie=0x%s,tun_src=%s,actions=goto_table:1", cookie, nodeIP)
	out, err := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", inrule).CombinedOutput()
	if err != nil {
		glog.Errorf("Error adding flow %q: %s (%v)", inrule, out, err)
		return err
	}

	iprule := fmt.Sprintf("table=8,cookie=0x%s,priority=100,ip,nw_dst=%s,actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", cookie, nodeSubnetCIDR, nodeIP)
	out, err = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", iprule).CombinedOutput()
	if err != nil {
		glog.Errorf("Error adding flow %q: %s (%v)", iprule, out, err)
		return err
	}

	arprule := fmt.Sprintf("table=9,cookie=0x%s,priority=100,arp,nw_dst=%s,actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", cookie, nodeSubnetCIDR, nodeIP)
	out, err = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", arprule).CombinedOutput()
	if err != nil {
		glog.Errorf("Error adding flow %q: %s (%v)", arprule, out, err)
		return err
	}
	return nil
}

func (c *FlowController) DelOFRules(nodeIP, localIP string) error {
	if nodeIP == localIP {
		return nil
	}

	glog.V(5).Infof("DelOFRules for %s", nodeIP)

	rule := fmt.Sprintf("cookie=0x%s/0xffffffff", generateCookie(nodeIP))
	out, err := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", rule).CombinedOutput()
	if err != nil {
		glog.Errorf("Error deleting flow %q: %s (%v)", rule, out, err)
		return err
	}
	return nil
}

func generateCookie(ip string) string {
	return hex.EncodeToString(net.ParseIP(ip).To4())
}

func (c *FlowController) AddServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error {
	if !c.multitenant {
		return nil
	}

	glog.V(5).Infof("AddServiceOFRules for %s/%s/%d", IP, string(protocol), port)

	rule := generateAddServiceRule(netID, IP, protocol, port)
	out, err := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", rule).CombinedOutput()
	if err != nil {
		glog.Errorf("Error adding flow %q: %s (%v)", rule, out, err)
		return err
	}
	return nil
}

func (c *FlowController) DelServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error {
	if !c.multitenant {
		return nil
	}

	glog.V(5).Infof("DelServiceOFRules for %s/%s/%d", IP, string(protocol), port)

	rule := generateDelServiceRule(IP, protocol, port)
	out, err := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", rule).CombinedOutput()
	if err != nil {
		glog.Errorf("Error deleting flow %q: %s (%v)", rule, out, err)
		return err
	}
	return nil
}

func generateBaseServiceRule(IP string, protocol api.ServiceProtocol, port uint) string {
	return fmt.Sprintf("table=5,%s,nw_dst=%s,tp_dst=%d", strings.ToLower(string(protocol)), IP, port)
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
