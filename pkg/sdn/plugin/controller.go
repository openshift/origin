package plugin

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"

	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/util/ipcmd"
	"github.com/openshift/origin/pkg/util/netutils"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/iptables"
	"k8s.io/kubernetes/pkg/util/sysctl"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
)

const (
	// rule versioning; increment each time flow rules change
	VERSION        = 3
	VERSION_TABLE  = "table=253"
	VERSION_ACTION = "actions=note:"

	BR    = "br0"
	TUN   = "tun0"
	VXLAN = "vxlan0"

	VXLAN_PORT = "4789"
)

func (plugin *OsdnNode) getPluginVersion() []string {
	if VERSION > 254 {
		panic("Version too large!")
	}
	version := fmt.Sprintf("%02X", VERSION)
	pluginId := ""
	switch plugin.policy.(type) {
	case *singleTenantPlugin:
		pluginId = "00"
	case *multiTenantPlugin:
		pluginId = "01"
	case *networkPolicyPlugin:
		pluginId = "02"
	default:
		panic("Not an OpenShift-SDN plugin")
	}
	return []string{pluginId, version}
}

func (plugin *OsdnNode) getLocalSubnet() (string, error) {
	var subnet *osapi.HostSubnet
	backoff := utilwait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   2,
		Steps:    8,
	}
	err := utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		subnet, err = plugin.osClient.HostSubnets().Get(plugin.hostName)
		if err == nil {
			return true, nil
		} else if kapierrors.IsNotFound(err) {
			glog.Warningf("Could not find an allocated subnet for node: %s, Waiting...", plugin.hostName)
			return false, nil
		} else {
			return false, err
		}
	})
	if err != nil {
		return "", fmt.Errorf("failed to get subnet for this host: %s, error: %v", plugin.hostName, err)
	}

	if err = plugin.networkInfo.validateNodeIP(subnet.HostIP); err != nil {
		return "", fmt.Errorf("failed to validate own HostSubnet: %v", err)
	}

	return subnet.Subnet, nil
}

func (plugin *OsdnNode) alreadySetUp(localSubnetGatewayCIDR, clusterNetworkCIDR string) bool {
	var found bool

	exec := kexec.New()
	itx := ipcmd.NewTransaction(exec, TUN)
	addrs, err := itx.GetAddresses()
	itx.EndTransaction()
	if err != nil {
		return false
	}
	found = false
	for _, addr := range addrs {
		if strings.Contains(addr, localSubnetGatewayCIDR) {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	itx = ipcmd.NewTransaction(exec, TUN)
	routes, err := itx.GetRoutes()
	itx.EndTransaction()
	if err != nil {
		return false
	}
	found = false
	for _, route := range routes {
		if strings.Contains(route, clusterNetworkCIDR+" ") {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	flows, err := plugin.ovs.DumpFlows()
	if err != nil {
		return false
	}
	found = false
	for _, flow := range flows {
		if !strings.Contains(flow, VERSION_TABLE) {
			continue
		}
		idx := strings.Index(flow, VERSION_ACTION)
		if idx < 0 {
			continue
		}

		// OVS note action format hex bytes separated by '.'; first
		// byte is plugin type (multi-tenant/single-tenant) and second
		// byte is flow rule version
		expected := plugin.getPluginVersion()
		existing := strings.Split(flow[idx+len(VERSION_ACTION):], ".")
		if len(existing) >= 2 && existing[0] == expected[0] && existing[1] == expected[1] {
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
	backoff := utilwait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   1.25,
		Steps:    6,
	}
	err := utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		itx := ipcmd.NewTransaction(kexec.New(), device)
		routes, err := itx.GetRoutes()
		if err != nil {
			return false, fmt.Errorf("could not get routes: %v", err)
		}
		for _, route := range routes {
			if strings.Contains(route, localSubnetCIDR) {
				itx.DeleteRoute(localSubnetCIDR)
				err = itx.EndTransaction()
				if err != nil {
					return false, fmt.Errorf("could not delete route: %v", err)
				}
				return true, nil
			}
		}
		return false, nil
	})

	if err != nil {
		glog.Errorf("Error removing %s route from dev %s: %v; if the route appears later it will not be deleted.", localSubnetCIDR, device, err)
	}
}

func (plugin *OsdnNode) SetupSDN() (bool, error) {
	clusterNetworkCIDR := plugin.networkInfo.ClusterNetwork.String()
	serviceNetworkCIDR := plugin.networkInfo.ServiceNetwork.String()

	localSubnetCIDR := plugin.localSubnetCIDR
	_, ipnet, err := net.ParseCIDR(localSubnetCIDR)
	localSubnetMaskLength, _ := ipnet.Mask.Size()
	localSubnetGateway := netutils.GenerateDefaultGateway(ipnet).String()

	glog.V(5).Infof("[SDN setup] node pod subnet %s gateway %s", ipnet.String(), localSubnetGateway)

	exec := kexec.New()

	if plugin.clearLbr0IptablesRule {
		// Delete docker's left-over lbr0 rule; cannot do this from
		// NewNodePlugin (where docker is cleaned up) because we need
		// localSubnetCIDR which is only valid after plugin start
		ipt := iptables.New(exec, utildbus.New(), iptables.ProtocolIpv4)
		ipt.DeleteRule(iptables.TableNAT, iptables.ChainPostrouting, "-s", localSubnetCIDR, "!", "-o", "lbr0", "-j", "MASQUERADE")
	}

	gwCIDR := fmt.Sprintf("%s/%d", localSubnetGateway, localSubnetMaskLength)
	if plugin.alreadySetUp(gwCIDR, clusterNetworkCIDR) {
		glog.V(5).Infof("[SDN setup] no SDN setup required")
		return false, nil
	}
	glog.V(5).Infof("[SDN setup] full SDN setup required")

	if err := os.MkdirAll("/run/openshift-sdn", 0700); err != nil {
		return false, err
	}
	config := fmt.Sprintf("export OPENSHIFT_CLUSTER_SUBNET=%s", clusterNetworkCIDR)
	err = ioutil.WriteFile("/run/openshift-sdn/config.env", []byte(config), 0644)
	if err != nil {
		return false, err
	}

	err = plugin.ovs.AddBridge("fail-mode=secure", "protocols=OpenFlow13")
	if err != nil {
		return false, err
	}
	err = plugin.ovs.SetFrags("nx-match")
	if err != nil {
		return false, err
	}
	_ = plugin.ovs.DeletePort(VXLAN)
	_, err = plugin.ovs.AddPort(VXLAN, 1, "type=vxlan", `options:remote_ip="flow"`, `options:key="flow"`)
	if err != nil {
		return false, err
	}
	_ = plugin.ovs.DeletePort(TUN)
	_, err = plugin.ovs.AddPort(TUN, 2, "type=internal")
	if err != nil {
		return false, err
	}

	otx := plugin.ovs.NewTransaction()
	// Table 0: initial dispatch based on in_port
	// vxlan0
	otx.AddFlow("table=0, priority=200, in_port=1, arp, nw_src=%s, nw_dst=%s, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10", clusterNetworkCIDR, localSubnetCIDR)
	otx.AddFlow("table=0, priority=200, in_port=1, ip, nw_src=%s, nw_dst=%s, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10", clusterNetworkCIDR, localSubnetCIDR)
	otx.AddFlow("table=0, priority=200, in_port=1, ip, nw_src=%s, nw_dst=224.0.0.0/4, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10", clusterNetworkCIDR)
	otx.AddFlow("table=0, priority=150, in_port=1, actions=drop")
	// tun0
	otx.AddFlow("table=0, priority=250, in_port=2, ip, nw_dst=224.0.0.0/4, actions=drop")
	otx.AddFlow("table=0, priority=200, in_port=2, arp, nw_src=%s, nw_dst=%s, actions=goto_table:30", localSubnetGateway, clusterNetworkCIDR)
	otx.AddFlow("table=0, priority=200, in_port=2, ip, actions=goto_table:30")
	otx.AddFlow("table=0, priority=150, in_port=2, actions=drop")
	// else, from a container
	otx.AddFlow("table=0, priority=100, arp, actions=goto_table:20")
	otx.AddFlow("table=0, priority=100, ip, actions=goto_table:20")
	otx.AddFlow("table=0, priority=0, actions=drop")

	// Table 10: VXLAN ingress filtering; filled in by AddHostSubnetRules()
	// eg, "table=10, priority=100, tun_src=${remote_node_ip}, actions=goto_table:30"
	otx.AddFlow("table=10, priority=0, actions=drop")

	// Table 20: from OpenShift container; validate IP/MAC, assign tenant-id; filled in by setupPodFlows
	// eg, "table=20, priority=100, in_port=${ovs_port}, arp, nw_src=${ipaddr}, arp_sha=${macaddr}, actions=load:${tenant_id}->NXM_NX_REG0[], goto_table:21"
	//     "table=20, priority=100, in_port=${ovs_port}, ip, nw_src=${ipaddr}, actions=load:${tenant_id}->NXM_NX_REG0[], goto_table:21"
	// (${tenant_id} is always 0 for single-tenant)
	otx.AddFlow("table=20, priority=0, actions=drop")

	// Table 21: from OpenShift container; NetworkPolicy plugin uses this for connection tracking
	otx.AddFlow("table=21, priority=0, actions=goto_table:30")

	// Table 30: general routing
	otx.AddFlow("table=30, priority=300, arp, nw_dst=%s, actions=output:2", localSubnetGateway)
	otx.AddFlow("table=30, priority=200, arp, nw_dst=%s, actions=goto_table:40", localSubnetCIDR)
	otx.AddFlow("table=30, priority=100, arp, nw_dst=%s, actions=goto_table:50", clusterNetworkCIDR)
	otx.AddFlow("table=30, priority=300, ip, nw_dst=%s, actions=output:2", localSubnetGateway)
	otx.AddFlow("table=30, priority=100, ip, nw_dst=%s, actions=goto_table:60", serviceNetworkCIDR)
	otx.AddFlow("table=30, priority=200, ip, nw_dst=%s, actions=goto_table:70", localSubnetCIDR)
	otx.AddFlow("table=30, priority=100, ip, nw_dst=%s, actions=goto_table:90", clusterNetworkCIDR)

	// Multicast coming from the VXLAN
	otx.AddFlow("table=30, priority=50, in_port=1, ip, nw_dst=224.0.0.0/4, actions=goto_table:120")
	// Multicast coming from local pods
	otx.AddFlow("table=30, priority=25, ip, nw_dst=224.0.0.0/4, actions=goto_table:110")

	otx.AddFlow("table=30, priority=0, ip, actions=goto_table:100")
	otx.AddFlow("table=30, priority=0, arp, actions=drop")

	// Table 40: ARP to local container, filled in by setupPodFlows
	// eg, "table=40, priority=100, arp, nw_dst=${container_ip}, actions=output:${ovs_port}"
	otx.AddFlow("table=40, priority=0, actions=drop")

	// Table 50: ARP to remote container; filled in by AddHostSubnetRules()
	// eg, "table=50, priority=100, arp, nw_dst=${remote_subnet_cidr}, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31], set_field:${remote_node_ip}->tun_dst,output:1"
	otx.AddFlow("table=50, priority=0, actions=drop")

	// Table 60: IP to service: vnid/port mappings; filled in by AddServiceRules()
	otx.AddFlow("table=60, priority=200, reg0=0, actions=output:2")
	// eg, "table=60, priority=100, reg0=${tenant_id}, ${service_proto}, nw_dst=${service_ip}, tp_dst=${service_port}, actions=load:${tenant_id}->NXM_NX_REG1[], load:2->NXM_NX_REG2[], goto_table:80"
	otx.AddFlow("table=60, priority=0, actions=drop")

	// Table 70: IP to local container: vnid/port mappings; filled in by setupPodFlows
	// eg, "table=70, priority=100, ip, nw_dst=${ipaddr}, actions=load:${tenant_id}->NXM_NX_REG1[], load:${ovs_port}->NXM_NX_REG2[], goto_table:80"
	otx.AddFlow("table=70, priority=0, actions=drop")

	// Table 80: IP policy enforcement; mostly managed by the osdnPolicy
	otx.AddFlow("table=80, priority=300, ip, nw_src=%s/32, actions=output:NXM_NX_REG2[]", localSubnetGateway)
	// eg, "table=80, priority=100, reg0=${tenant_id}, reg1=${tenant_id}, actions=output:NXM_NX_REG2[]"
	otx.AddFlow("table=80, priority=0, actions=drop")

	// Table 90: IP to remote container; filled in by AddHostSubnetRules()
	// eg, "table=90, priority=100, ip, nw_dst=${remote_subnet_cidr}, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31], set_field:${remote_node_ip}->tun_dst,output:1"
	otx.AddFlow("table=90, priority=0, actions=drop")

	// Table 100: egress network policy dispatch; edited by UpdateEgressNetworkPolicy()
	// eg, "table=100, reg0=${tenant_id}, priority=2, ip, nw_dst=${external_cidr}, actions=drop
	otx.AddFlow("table=100, priority=0, actions=output:2")

	// Table 110: outbound multicast filtering, updated by updateLocalMulticastFlows() in pod.go
	// eg, "table=110, priority=100, reg0=${tenant_id}, actions=goto_table:111
	otx.AddFlow("table=110, priority=0, actions=drop")

	// Table 111: multicast delivery from local pods to the VXLAN; only one rule, updated by updateVXLANMulticastRules() in subnets.go
	// eg, "table=111, priority=100, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:${remote_node_ip_1}->tun_dst,output:1,set_field:${remote_node_ip_2}->tun_dst,output:1,goto_table:120"
	otx.AddFlow("table=111, priority=0, actions=drop")

	// Table 120: multicast delivery to local pods (either from VXLAN or local pods); updated by updateLocalMulticastFlows() in pod.go
	// eg, "table=120, priority=100, reg0=${tenant_id}, actions=output:${ovs_port_1},output:${ovs_port_2}"
	otx.AddFlow("table=120, priority=0, actions=drop")

	err = otx.EndTransaction()
	if err != nil {
		return false, err
	}

	itx := ipcmd.NewTransaction(exec, TUN)
	itx.AddAddress(gwCIDR)
	defer deleteLocalSubnetRoute(TUN, localSubnetCIDR)
	itx.SetLink("mtu", fmt.Sprint(plugin.mtu))
	itx.SetLink("up")
	itx.AddRoute(clusterNetworkCIDR, "proto", "kernel", "scope", "link")
	itx.AddRoute(serviceNetworkCIDR)
	err = itx.EndTransaction()
	if err != nil {
		return false, err
	}

	sysctl := sysctl.New()

	// Enable IP forwarding for ipv4 packets
	err = sysctl.SetSysctl("net/ipv4/ip_forward", 1)
	if err != nil {
		return false, fmt.Errorf("could not enable IPv4 forwarding: %s", err)
	}
	err = sysctl.SetSysctl(fmt.Sprintf("net/ipv4/conf/%s/forwarding", TUN), 1)
	if err != nil {
		return false, fmt.Errorf("could not enable IPv4 forwarding on %s: %s", TUN, err)
	}

	// Table 253: rule version; note action is hex bytes separated by '.'
	otx = plugin.ovs.NewTransaction()
	pluginVersion := plugin.getPluginVersion()
	otx.AddFlow("%s, %s%s.%s", VERSION_TABLE, VERSION_ACTION, pluginVersion[0], pluginVersion[1])
	err = otx.EndTransaction()
	if err != nil {
		return false, err
	}

	return true, nil
}

func policyNames(policies []osapi.EgressNetworkPolicy) string {
	names := make([]string, len(policies))
	for i, policy := range policies {
		names[i] = policy.Namespace + ":" + policy.Name
	}
	return strings.Join(names, ", ")
}

func (plugin *OsdnNode) updateEgressNetworkPolicyRules(vnid uint32) {
	otx := plugin.ovs.NewTransaction()

	policies := plugin.egressPolicies[vnid]
	namespaces := plugin.policy.GetNamespaces(vnid)
	if len(policies) == 0 {
		otx.DeleteFlows("table=100, reg0=%d", vnid)
	} else if vnid == 0 {
		glog.Errorf("EgressNetworkPolicy in global network namespace is not allowed (%s); ignoring", policyNames(policies))
	} else if len(namespaces) > 1 {
		// Rationale: In our current implementation, multiple namespaces share their network by using the same VNID.
		// Even though Egress network policy is defined per namespace, its implementation is based on VNIDs.
		// So in case of shared network namespaces, egress policy of one namespace will affect all other namespaces that are sharing the network which might not be desirable.
		glog.Errorf("EgressNetworkPolicy not allowed in shared NetNamespace (%s); dropping all traffic", strings.Join(namespaces, ", "))
		otx.DeleteFlows("table=100, reg0=%d", vnid)
		otx.AddFlow("table=100, reg0=%d, priority=1, actions=drop", vnid)
	} else if len(policies) > 1 {
		// Rationale: If we have allowed more than one policy, we could end up with different network restrictions depending
		// on the order of policies that were processed and also it doesn't give more expressive power than a single policy.
		glog.Errorf("multiple EgressNetworkPolicies in same network namespace (%s) is not allowed; dropping all traffic", policyNames(policies))
		otx.DeleteFlows("table=100, reg0=%d", vnid)
		otx.AddFlow("table=100, reg0=%d, priority=1, actions=drop", vnid)
	} else /* vnid != 0 && len(policies) == 1 */ {
		// Temporarily drop all outgoing traffic, to avoid race conditions while modifying the other rules
		otx.AddFlow("table=100, reg0=%d, cookie=1, priority=65535, actions=drop", vnid)
		otx.DeleteFlows("table=100, reg0=%d, cookie=0/1", vnid)

		for i, rule := range policies[0].Spec.Egress {
			priority := len(policies[0].Spec.Egress) - i

			var action string
			if rule.Type == osapi.EgressNetworkPolicyRuleAllow {
				action = "output:2"
			} else {
				action = "drop"
			}

			var dst string
			if rule.To.CIDRSelector == "0.0.0.0/0" {
				dst = ""
			} else {
				dst = fmt.Sprintf(", nw_dst=%s", rule.To.CIDRSelector)
			}

			otx.AddFlow("table=100, reg0=%d, priority=%d, ip%s, actions=%s", vnid, priority, dst, action)
		}
		otx.DeleteFlows("table=100, reg0=%d, cookie=1/1", vnid)
	}

	if err := otx.EndTransaction(); err != nil {
		glog.Errorf("Error updating OVS flows for EgressNetworkPolicy: %v", err)
	}
}

func (plugin *OsdnNode) AddHostSubnetRules(subnet *osapi.HostSubnet) {
	glog.Infof("AddHostSubnetRules for %s", hostSubnetToString(subnet))
	otx := plugin.ovs.NewTransaction()

	otx.AddFlow("table=10, priority=100, tun_src=%s, actions=goto_table:30", subnet.HostIP)
	if vnid, ok := subnet.Annotations[osapi.FixedVNIDHostAnnotation]; ok {
		otx.AddFlow("table=50, priority=100, arp, nw_dst=%s, actions=load:%s->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", subnet.Subnet, vnid, subnet.HostIP)
		otx.AddFlow("table=90, priority=100, ip, nw_dst=%s, actions=load:%s->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", subnet.Subnet, vnid, subnet.HostIP)
	} else {
		otx.AddFlow("table=50, priority=100, arp, nw_dst=%s, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", subnet.Subnet, subnet.HostIP)
		otx.AddFlow("table=90, priority=100, ip, nw_dst=%s, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", subnet.Subnet, subnet.HostIP)
	}

	if err := otx.EndTransaction(); err != nil {
		glog.Errorf("Error adding OVS flows for subnet %q: %v", subnet.Subnet, err)
	}
}

func (plugin *OsdnNode) DeleteHostSubnetRules(subnet *osapi.HostSubnet) {
	glog.Infof("DeleteHostSubnetRules for %s", hostSubnetToString(subnet))

	otx := plugin.ovs.NewTransaction()
	otx.DeleteFlows("table=10, tun_src=%s", subnet.HostIP)
	otx.DeleteFlows("table=50, arp, nw_dst=%s", subnet.Subnet)
	otx.DeleteFlows("table=90, ip, nw_dst=%s", subnet.Subnet)
	if err := otx.EndTransaction(); err != nil {
		glog.Errorf("Error deleting OVS flows for subnet %q: %v", subnet.Subnet, err)
	}
}

func (plugin *OsdnNode) AddServiceRules(service *kapi.Service, netID uint32) {
	glog.V(5).Infof("AddServiceRules for %v", service)

	otx := plugin.ovs.NewTransaction()
	action := fmt.Sprintf(", priority=100, actions=load:%d->NXM_NX_REG1[], load:2->NXM_NX_REG2[], goto_table:80", netID)

	// Add blanket rule allowing subsequent IP fragments
	otx.AddFlow(generateBaseServiceRule(service.Spec.ClusterIP) + ", ip_frag=later" + action)

	for _, port := range service.Spec.Ports {
		baseRule, err := generateBaseAddServiceRule(service.Spec.ClusterIP, port.Protocol, int(port.Port))
		if err != nil {
			glog.Errorf("Error creating OVS flow for service %v, netid %d: %v", service, netID, err)
		}
		otx.AddFlow(baseRule + action)
	}

	if err := otx.EndTransaction(); err != nil {
		glog.Errorf("Error adding OVS flows for service %v, netid %d: %v", service, netID, err)
	}
}

func (plugin *OsdnNode) DeleteServiceRules(service *kapi.Service) {
	glog.V(5).Infof("DeleteServiceRules for %v", service)

	otx := plugin.ovs.NewTransaction()
	otx.DeleteFlows(generateBaseServiceRule(service.Spec.ClusterIP))
	otx.EndTransaction()
}

func generateBaseServiceRule(IP string) string {
	return fmt.Sprintf("table=60, ip, nw_dst=%s", IP)
}

func generateBaseAddServiceRule(IP string, protocol kapi.Protocol, port int) (string, error) {
	var dst string
	if protocol == kapi.ProtocolUDP {
		dst = fmt.Sprintf(", udp, udp_dst=%d", port)
	} else if protocol == kapi.ProtocolTCP {
		dst = fmt.Sprintf(", tcp, tcp_dst=%d", port)
	} else {
		return "", fmt.Errorf("unhandled protocol %v", protocol)
	}
	return generateBaseServiceRule(IP) + dst, nil
}
