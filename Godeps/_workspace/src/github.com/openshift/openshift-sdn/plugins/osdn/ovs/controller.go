package ovs

import (
	"encoding/hex"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/openshift/openshift-sdn/pkg/ipcmd"
	"github.com/openshift/openshift-sdn/pkg/netutils"
	"github.com/openshift/openshift-sdn/pkg/ovs"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"

	"k8s.io/kubernetes/pkg/util/sysctl"
)

const (
	BR       = "br0"
	LBR      = "lbr0"
	TUN      = "tun0"
	VLINUXBR = "vlinuxbr"
	VOVSBR   = "vovsbr"
	VXLAN    = "vxlan0"
)

type FlowController struct {
	multitenant bool
}

func NewFlowController(multitenant bool) *FlowController {
	return &FlowController{multitenant}
}

func alreadySetUp(multitenant bool, localSubnetGatewayCIDR string) bool {
	var found bool

	itx := ipcmd.NewTransaction(LBR)
	addrs, err := itx.GetAddresses()
	itx.EndTransaction()
	if err != nil {
		return false
	}
	found = false
	for _, addr := range addrs {
		if addr == localSubnetGatewayCIDR {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	otx := ovs.NewTransaction(BR)
	flows, err := otx.DumpFlows()
	otx.EndTransaction()
	if err != nil {
		return false
	}
	found = false
	for _, flow := range flows {
		if !strings.Contains(flow, "table=3") {
			continue
		}
		if (multitenant && strings.Contains(flow, "NXM_NX_TUN_ID")) ||
			(!multitenant && strings.Contains(flow, "goto_table:9")) {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	return true
}

func deleteLocalSubnetRoute(device, localSubnetCIDR string) {
	const (
		timeInterval = 100 * time.Millisecond
		maxIntervals = 20
	)

	for i := 0; i < maxIntervals; i++ {
		itx := ipcmd.NewTransaction(device)
		routes, err := itx.GetRoutes()
		if err != nil {
			glog.Errorf("Could not get routes for dev %s: %v", device, err)
			return
		}
		for _, route := range routes {
			if strings.Contains(route, localSubnetCIDR) {
				itx.DeleteRoute(localSubnetCIDR)
				err = itx.EndTransaction()
				if err != nil {
					glog.Errorf("Could not delete subnet route %s from dev %s: %v", localSubnetCIDR, device, err)
				}
				return
			}
		}

		time.Sleep(timeInterval)
	}

	glog.Errorf("Timed out looking for %s route for dev %s; if it appears later it will not be deleted.", localSubnetCIDR, device)
}

func (c *FlowController) Setup(localSubnetCIDR, clusterNetworkCIDR, servicesNetworkCIDR string, mtu uint) (bool, error) {
	_, ipnet, err := net.ParseCIDR(localSubnetCIDR)
	localSubnetMaskLength, _ := ipnet.Mask.Size()
	localSubnetGateway := netutils.GenerateDefaultGateway(ipnet).String()

	glog.V(5).Infof("[SDN setup] node pod subnet %s gateway %s", ipnet.String(), localSubnetGateway)

	gwCIDR := fmt.Sprintf("%s/%d", localSubnetGateway, localSubnetMaskLength)
	if alreadySetUp(c.multitenant, gwCIDR) {
		glog.V(5).Infof("[SDN setup] no SDN setup required")
		return false, nil
	}
	glog.V(5).Infof("[SDN setup] full SDN setup required")

	mtuStr := fmt.Sprint(mtu)

	itx := ipcmd.NewTransaction(LBR)
	itx.SetLink("down")
	itx.IgnoreError()
	itx.DeleteLink()
	itx.IgnoreError()
	itx.AddLink("type", "bridge")
	itx.AddAddress(gwCIDR)
	itx.SetLink("up")
	err = itx.EndTransaction()
	if err != nil {
		glog.Errorf("Failed to configure docker bridge: %v", err)
		return false, err
	}
	defer deleteLocalSubnetRoute(LBR, localSubnetCIDR)

	glog.V(5).Infof("[SDN setup] docker setup %s mtu %s", LBR, mtuStr)
	out, err := exec.Command("openshift-sdn-docker-setup.sh", LBR, mtuStr).CombinedOutput()
	if err != nil {
		glog.Errorf("Failed to configure docker networking: %v\n%s", err, out)
		return false, err
	} else {
		glog.V(5).Infof("[SDN setup] docker setup success:\n%s", out)
	}

	config := fmt.Sprintf("export OPENSHIFT_CLUSTER_SUBNET=%s", clusterNetworkCIDR)
	err = ioutil.WriteFile("/run/openshift-sdn/config.env", []byte(config), 0644)
	if err != nil {
		return false, err
	}

	itx = ipcmd.NewTransaction(VLINUXBR)
	itx.DeleteLink()
	itx.IgnoreError()
	itx.AddLink("mtu", mtuStr, "type", "veth", "peer", "name", VOVSBR, "mtu", mtuStr)
	itx.SetLink("up")
	itx.SetLink("txqueuelen", "0")
	err = itx.EndTransaction()
	if err != nil {
		return false, err
	}

	itx = ipcmd.NewTransaction(VOVSBR)
	itx.SetLink("up")
	itx.SetLink("txqueuelen", "0")
	err = itx.EndTransaction()
	if err != nil {
		return false, err
	}

	itx = ipcmd.NewTransaction(LBR)
	itx.AddSlave(VLINUXBR)
	err = itx.EndTransaction()
	if err != nil {
		return false, err
	}

	otx := ovs.NewTransaction(BR)
	otx.AddBridge("fail-mode=secure", "protocols=OpenFlow13")
	otx.AddPort(VXLAN, 1, "type=vxlan", `options:remote_ip="flow"`, `options:key="flow"`)
	otx.AddPort(TUN, 2, "type=internal")
	otx.AddPort(VOVSBR, 3)

	// Table 0; VXLAN filtering; the first rule sends un-tunnelled packets
	// to table 1. Additional per-node rules are filled in by controller.go
	otx.AddFlow("table=0, tun_src=0.0.0.0, actions=goto_table:1")
	// eg, "table=0, tun_src=${remote_node}, actions=goto_table:1"

	// Table 1; learn MAC addresses and continue with table 2
	otx.AddFlow("table=1, actions=learn(table=9, priority=200, hard_timeout=900, NXM_OF_ETH_DST[]=NXM_OF_ETH_SRC[], load:NXM_NX_TUN_IPV4_SRC[]->NXM_NX_TUN_IPV4_DST[], output:NXM_OF_IN_PORT[]), goto_table:2")

	// Table 2; initial dispatch
	otx.AddFlow("table=2, priority=200, arp, actions=goto_table:9")
	otx.AddFlow("table=2, priority=100, in_port=1, actions=goto_table:3") // vxlan0
	otx.AddFlow("table=2, priority=100, in_port=2, actions=goto_table:6") // tun0
	otx.AddFlow("table=2, priority=100, in_port=3, actions=goto_table:6") // vovsbr
	otx.AddFlow("table=2, priority=0, actions=goto_table:4")              // container

	// Table 3; incoming from vxlan
	otx.AddFlow("table=3, priority=200, ip, nw_dst=%s, actions=output:2", localSubnetGateway)
	if c.multitenant {
		otx.AddFlow("table=3, priority=100, ip, nw_dst=%s, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[], goto_table:6", localSubnetCIDR)
	} else {
		otx.AddFlow("table=3, priority=100, ip, nw_dst=%s, actions=goto_table:9", localSubnetCIDR)
	}

	// Table 4; incoming from container; filled in by openshift-sdn-ovs
	// eg, single-tenant: "table=4, priority=100, in_port=${ovs_port}, ip, nw_src=${ipaddr}, goto_table:6"
	//     multitenant:   "table=4, priority=100, in_port=${ovs_port}, ip, nw_src=${ipaddr}, actions=load:${tenant_id}->NXM_NX_REG0[], goto_table:5"

	// Table 5; service isolation; mostly filled in by AddServiceOFRules()
	if c.multitenant {
		otx.AddFlow("table=5, priority=200, reg0=0, ip, nw_dst=%s, actions=output:2", servicesNetworkCIDR)
		// eg, "table=5, priority=200, ${service_proto}, nw_dst=${service_ip}, tp_dst=${service_port}, actions=output:2"
		otx.AddFlow("table=5, priority=100, ip, nw_dst=%s, actions=drop", servicesNetworkCIDR)
		otx.AddFlow("table=5, priority=0, actions=goto_table:6")
	}

	// Table 6; general routing
	otx.AddFlow("table=6, priority=200, ip, nw_dst=%s, actions=output:2", localSubnetGateway)
	if c.multitenant {
		otx.AddFlow("table=6, priority=175, ip, reg0=0, nw_dst=%s, actions=goto_table:9", localSubnetCIDR)
		otx.AddFlow("table=6, priority=150, ip, nw_dst=%s, actions=goto_table:7", localSubnetCIDR)
	} else {
		otx.AddFlow("table=6, priority=150, ip, nw_dst=%s, actions=goto_table:9", localSubnetCIDR)
	}
	otx.AddFlow("table=6, priority=100, ip, nw_dst=%s, actions=goto_table:8", clusterNetworkCIDR)
	otx.AddFlow("table=6, priority=0, ip, actions=output:2")

	// Table 7; to local container with isolation; filled in by openshift-sdn-ovs
	// eg, "table=7, priority=100, ip, nw_dst=${ipaddr}, reg0=${tenant_id}, actions=output:${ovs_port}"

	// Table 8; to remote container; filled in by AddOFRules()
	// eg, "table=8, priority=100, ip, nw_dst=${remote_subnet_cidr}, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31], set_field:${remote_node_ip}->tun_dst,output:1"

	// Table 9; MAC dispatch / ARP, filled in by Table 1's learn() rule
	// and with per-node vxlan ARP rules by AddOFRules()
	otx.AddFlow("table=9, priority=0, arp, actions=flood")
	// eg, "table=9, priority=100, arp, nw_dst=${remote_subnet_cidr}, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31], set_field:${remote_node_ip}->tun_dst,output:1"

	err = otx.EndTransaction()
	if err != nil {
		return false, err
	}

	itx = ipcmd.NewTransaction(TUN)
	itx.AddAddress(gwCIDR)
	defer deleteLocalSubnetRoute(TUN, localSubnetCIDR)
	itx.SetLink("up")
	itx.AddRoute(clusterNetworkCIDR, "proto", "kernel", "scope", "link")
	itx.AddRoute(servicesNetworkCIDR)
	err = itx.EndTransaction()
	if err != nil {
		return false, err
	}

	// Clean up docker0 since docker won't
	itx = ipcmd.NewTransaction("docker0")
	itx.SetLink("down")
	itx.IgnoreError()
	itx.DeleteLink()
	itx.IgnoreError()
	_ = itx.EndTransaction()

	// Disable iptables for linux bridges (and in particular lbr0), ignoring errors.
	// (This has to have been performed in advance for docker-in-docker deployments,
	// since this will fail there).
	_, _ = exec.Command("modprobe", "br_netfilter").CombinedOutput()
	err = sysctl.SetSysctl("net/bridge/bridge-nf-call-iptables", 0)
	if err != nil {
		glog.Warningf("Could not set net.bridge.bridge-nf-call-iptables sysctl: %s", err)
	} else {
		glog.V(5).Infof("[SDN setup] set net.bridge.bridge-nf-call-iptables to 0")
	}

	// Enable IP forwarding for ipv4 packets
	err = sysctl.SetSysctl("net/ipv4/ip_forward", 1)
	if err != nil {
		return false, fmt.Errorf("Could not enable IPv4 forwarding: %s", err)
	}
	err = sysctl.SetSysctl(fmt.Sprintf("net/ipv4/conf/%s/forwarding", TUN), 1)
	if err != nil {
		return false, fmt.Errorf("Could not enable IPv4 forwarding on %s: %s", TUN, err)
	}

	return true, nil
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
	otx := ovs.NewTransaction(BR)

	otx.AddFlow("table=0,cookie=0x%s,tun_src=%s,actions=goto_table:1", cookie, nodeIP)
	otx.AddFlow("table=8,cookie=0x%s,priority=100,ip,nw_dst=%s,actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", cookie, nodeSubnetCIDR, nodeIP)
	otx.AddFlow("table=9,cookie=0x%s,priority=100,arp,nw_dst=%s,actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", cookie, nodeSubnetCIDR, nodeIP)

	err := otx.EndTransaction()
	if err != nil {
		glog.Errorf("Error adding OVS flows: %v", err)
	}
	return err
}

func (c *FlowController) DelOFRules(nodeIP, localIP string) error {
	if nodeIP == localIP {
		return nil
	}

	glog.V(5).Infof("DelOFRules for %s", nodeIP)

	otx := ovs.NewTransaction(BR)
	otx.DeleteFlows("cookie=0x%s/0xffffffff", generateCookie(nodeIP))
	err := otx.EndTransaction()
	if err != nil {
		glog.Errorf("Error deleting OVS flows: %v", err)
	}
	return err
}

func generateCookie(ip string) string {
	return hex.EncodeToString(net.ParseIP(ip).To4())
}

func (c *FlowController) AddServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error {
	if !c.multitenant {
		return nil
	}

	glog.V(5).Infof("AddServiceOFRules for %s/%s/%d", IP, string(protocol), port)

	otx := ovs.NewTransaction(BR)
	otx.AddFlow(generateAddServiceRule(netID, IP, protocol, port))
	err := otx.EndTransaction()
	if err != nil {
		glog.Errorf("Error adding OVS flow: %v", err)
	}
	return err
}

func (c *FlowController) DelServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error {
	if !c.multitenant {
		return nil
	}

	glog.V(5).Infof("DelServiceOFRules for %s/%s/%d", IP, string(protocol), port)

	otx := ovs.NewTransaction(BR)
	otx.DeleteFlows(generateDelServiceRule(IP, protocol, port))
	err := otx.EndTransaction()
	if err != nil {
		glog.Errorf("Error deleting OVS flow: %v", err)
	}
	return err
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
