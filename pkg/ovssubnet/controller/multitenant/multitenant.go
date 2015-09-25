package multitenant

import (
	"encoding/hex"
	"fmt"
	log "github.com/golang/glog"
	"net"
	"os/exec"
	"strings"
	"syscall"

	"github.com/openshift/openshift-sdn/pkg/firewalld"
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
				err = nil
			}
		}
	}
	if err != nil {
		log.Errorf("Error executing setup script. \n\tOutput: %s\n\tError: %v\n", out, err)
		return err
	}

	fw := firewalld.New()
	err = c.SetupIptables(fw, clusterNetworkCIDR)
	if err != nil {
		log.Errorf("Error setting up iptables: %v\n", err)
		return err
	}

	fw.AddReloadFunc(func() {
		err = c.SetupIptables(fw, clusterNetworkCIDR)
		if err != nil {
			log.Errorf("Error reloading iptables: %v\n", err)
		}
	})

	return nil
}

type FirewallRule struct {
	ipv      string
	table    string
	chain    string
	priority int
	args     []string
}

func (c *FlowController) SetupIptables(fw *firewalld.Interface, clusterNetworkCIDR string) error {
	if fw.IsRunning() {
		rules := []FirewallRule{
			{firewalld.IPv4, "nat", "POSTROUTING", 0, []string{"-s", clusterNetworkCIDR, "!", "-d", clusterNetworkCIDR, "-j", "MASQUERADE"}},
			{firewalld.IPv4, "filter", "INPUT", 0, []string{"-p", "udp", "-m", "multiport", "--dports", "4789", "-m", "comment", "--comment", "001 vxlan incoming", "-j", "ACCEPT"}},
			{firewalld.IPv4, "filter", "INPUT", 0, []string{"-i", "tun0", "-m", "comment", "--comment", "traffic from docker for internet", "-j", "ACCEPT"}},
			{firewalld.IPv4, "filter", "FORWARD", 0, []string{"-d", clusterNetworkCIDR, "-j", "ACCEPT"}},
			{firewalld.IPv4, "filter", "FORWARD", 0, []string{"-s", clusterNetworkCIDR, "-j", "ACCEPT"}},
		}

		for _, rule := range rules {
			err := fw.EnsureRule(rule.ipv, rule.table, rule.chain, rule.priority, rule.args)
			if err != nil {
				return err
			}
		}
	} else {
		exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING", "-s", clusterNetworkCIDR, "!", "-d", clusterNetworkCIDR, "-j", "MASQUERADE").Run()
		err := exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", clusterNetworkCIDR, "!", "-d", clusterNetworkCIDR, "-j", "MASQUERADE").Run()
		if err != nil {
			return err
		}
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
	rule := generateServiceRule(netID, IP, protocol, port)
	o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", rule).CombinedOutput()
	log.Infof("Output of adding %s: %s (%v)", rule, o, e)
	return e
}

func (c *FlowController) DelServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error {
	rule := generateServiceRule(netID, IP, protocol, port)
	o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", rule).CombinedOutput()
	log.Infof("Output of deleting %s: %s (%v)", rule, o, e)
	return e
}

func generateServiceRule(netID uint, IP string, protocol api.ServiceProtocol, port uint) string {
	if netID == 0 {
		return fmt.Sprintf("table=4,priority=200,%s,nw_dst=%s,tp_dst=%d,actions=output:2", strings.ToLower(string(protocol)), IP, port)
	} else {
		return fmt.Sprintf("table=4,priority=200,reg0=%d,%s,nw_dst=%s,tp_dst=%d,actions=output:2", netID, strings.ToLower(string(protocol)), IP, port)
	}
}
