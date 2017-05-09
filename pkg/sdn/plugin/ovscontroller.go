package plugin

import (
	"fmt"
	"sort"
	"strings"

	"github.com/golang/glog"

	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/util/ovs"

	kapi "k8s.io/kubernetes/pkg/api"
)

type ovsController struct {
	ovs      ovs.Interface
	pluginId int
}

const (
	BR    = "br0"
	TUN   = "tun0"
	VXLAN = "vxlan0"

	// rule versioning; increment each time flow rules change
	VERSION = 3

	VERSION_TABLE = 253
)

func NewOVSController(ovsif ovs.Interface, pluginId int) *ovsController {
	return &ovsController{ovs: ovsif, pluginId: pluginId}
}

func (oc *ovsController) getVersionNote() string {
	if VERSION > 254 {
		panic("Version too large!")
	}
	return fmt.Sprintf("note:%02X.%02X", oc.pluginId, VERSION)
}

func (oc *ovsController) AlreadySetUp() bool {
	flows, err := oc.ovs.DumpFlows()
	if err != nil {
		return false
	}
	expectedVersionNote := oc.getVersionNote()
	for _, flow := range flows {
		if strings.HasSuffix(flow, expectedVersionNote) && strings.Contains(flow, fmt.Sprintf("table=%d", VERSION_TABLE)) {
			return true
		}
	}
	return false
}

func (oc *ovsController) SetupOVS(clusterNetworkCIDR, serviceNetworkCIDR, localSubnetCIDR, localSubnetGateway string) error {
	err := oc.ovs.AddBridge("fail-mode=secure", "protocols=OpenFlow13")
	if err != nil {
		return err
	}
	err = oc.ovs.SetFrags("nx-match")
	if err != nil {
		return err
	}
	_ = oc.ovs.DeletePort(VXLAN)
	_, err = oc.ovs.AddPort(VXLAN, 1, "type=vxlan", `options:remote_ip="flow"`, `options:key="flow"`)
	if err != nil {
		return err
	}
	_ = oc.ovs.DeletePort(TUN)
	_, err = oc.ovs.AddPort(TUN, 2, "type=internal")
	if err != nil {
		return err
	}

	otx := oc.ovs.NewTransaction()
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

	// Table 110: outbound multicast filtering, updated by UpdateLocalMulticastFlows()
	// eg, "table=110, priority=100, reg0=${tenant_id}, actions=goto_table:111
	otx.AddFlow("table=110, priority=0, actions=drop")

	// Table 111: multicast delivery from local pods to the VXLAN; only one rule, updated by UpdateVXLANMulticastRules()
	// eg, "table=111, priority=100, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:${remote_node_ip_1}->tun_dst,output:1,set_field:${remote_node_ip_2}->tun_dst,output:1,goto_table:120"
	otx.AddFlow("table=111, priority=100, actions=goto_table:120")

	// Table 120: multicast delivery to local pods (either from VXLAN or local pods); updated by UpdateLocalMulticastFlows()
	// eg, "table=120, priority=100, reg0=${tenant_id}, actions=output:${ovs_port_1},output:${ovs_port_2}"
	otx.AddFlow("table=120, priority=0, actions=drop")

	// Table 253: rule version note
	otx.AddFlow("table=%d, actions=%s", VERSION_TABLE, oc.getVersionNote())

	err = otx.EndTransaction()
	if err != nil {
		return err
	}

	return nil
}

func (oc *ovsController) NewTransaction() ovs.Transaction {
	return oc.ovs.NewTransaction()
}

func (oc *ovsController) ensureOvsPort(hostVeth string) (int, error) {
	return oc.ovs.AddPort(hostVeth, -1)
}

func (oc *ovsController) setupPodFlows(ofport int, podIP, podMAC string, vnid uint32) error {
	otx := oc.ovs.NewTransaction()

	// ARP/IP traffic from container
	otx.AddFlow("table=20, priority=100, in_port=%d, arp, nw_src=%s, arp_sha=%s, actions=load:%d->NXM_NX_REG0[], goto_table:21", ofport, podIP, podMAC, vnid)
	otx.AddFlow("table=20, priority=100, in_port=%d, ip, nw_src=%s, actions=load:%d->NXM_NX_REG0[], goto_table:21", ofport, podIP, vnid)

	// ARP request/response to container (not isolated)
	otx.AddFlow("table=40, priority=100, arp, nw_dst=%s, actions=output:%d", podIP, ofport)

	// IP traffic to container
	otx.AddFlow("table=70, priority=100, ip, nw_dst=%s, actions=load:%d->NXM_NX_REG1[], load:%d->NXM_NX_REG2[], goto_table:80", podIP, vnid, ofport)

	return otx.EndTransaction()
}

func (oc *ovsController) cleanupPodFlows(podIP string) error {
	otx := oc.ovs.NewTransaction()
	otx.DeleteFlows("ip, nw_dst=%s", podIP)
	otx.DeleteFlows("ip, nw_src=%s", podIP)
	otx.DeleteFlows("arp, nw_dst=%s", podIP)
	otx.DeleteFlows("arp, nw_src=%s", podIP)
	return otx.EndTransaction()
}

func (oc *ovsController) SetUpPod(hostVeth, podIP, podMAC string, vnid uint32) (int, error) {
	ofport, err := oc.ensureOvsPort(hostVeth)
	if err != nil {
		return -1, err
	}
	return ofport, oc.setupPodFlows(ofport, podIP, podMAC, vnid)
}

func (oc *ovsController) SetPodBandwidth(hostVeth string, ingressBPS, egressBPS int64) error {
	// note pod ingress == OVS egress and vice versa

	qos, err := oc.ovs.Get("port", hostVeth, "qos")
	if err != nil {
		return err
	}
	if qos != "[]" {
		err = oc.ovs.Clear("port", hostVeth, "qos")
		if err != nil {
			return err
		}
		err = oc.ovs.Destroy("qos", qos)
		if err != nil {
			return err
		}
	}

	if ingressBPS > 0 {
		qos, err := oc.ovs.Create("qos", "type=linux-htb", fmt.Sprintf("other-config:max-rate=%d", ingressBPS))
		if err != nil {
			return err
		}
		err = oc.ovs.Set("port", hostVeth, fmt.Sprintf("qos=%s", qos))
		if err != nil {
			return err
		}
	}
	if egressBPS > 0 {
		// ingress_policing_rate is in Kbps
		err := oc.ovs.Set("interface", hostVeth, fmt.Sprintf("ingress_policing_rate=%d", egressBPS/1024))
		if err != nil {
			return err
		}
	}

	return nil
}

func (oc *ovsController) UpdatePod(hostVeth, podIP, podMAC string, vnid uint32) error {
	ofport, err := oc.ensureOvsPort(hostVeth)
	if err != nil {
		return err
	}
	err = oc.cleanupPodFlows(podIP)
	if err != nil {
		return err
	}
	return oc.setupPodFlows(ofport, podIP, podMAC, vnid)
}

func (oc *ovsController) TearDownPod(hostVeth, podIP string) error {
	err := oc.cleanupPodFlows(podIP)
	if err != nil {
		return err
	}
	_ = oc.SetPodBandwidth(hostVeth, -1, -1)
	return oc.ovs.DeletePort(hostVeth)
}

func policyNames(policies []osapi.EgressNetworkPolicy) string {
	names := make([]string, len(policies))
	for i, policy := range policies {
		names[i] = policy.Namespace + ":" + policy.Name
	}
	return strings.Join(names, ", ")
}

func (oc *ovsController) UpdateEgressNetworkPolicyRules(policies []osapi.EgressNetworkPolicy, vnid uint32, namespaces []string, egressDNS *EgressDNS) error {
	otx := oc.ovs.NewTransaction()
	var inputErr error

	if len(policies) == 0 {
		otx.DeleteFlows("table=100, reg0=%d", vnid)
	} else if vnid == 0 {
		inputErr = fmt.Errorf("EgressNetworkPolicy in global network namespace is not allowed (%s); ignoring", policyNames(policies))
	} else if len(namespaces) > 1 {
		// Rationale: In our current implementation, multiple namespaces share their network by using the same VNID.
		// Even though Egress network policy is defined per namespace, its implementation is based on VNIDs.
		// So in case of shared network namespaces, egress policy of one namespace will affect all other namespaces that are sharing the network which might not be desirable.
		inputErr = fmt.Errorf("EgressNetworkPolicy not allowed in shared NetNamespace (%s); dropping all traffic", strings.Join(namespaces, ", "))
		otx.DeleteFlows("table=100, reg0=%d", vnid)
		otx.AddFlow("table=100, reg0=%d, priority=1, actions=drop", vnid)
	} else if len(policies) > 1 {
		// Rationale: If we have allowed more than one policy, we could end up with different network restrictions depending
		// on the order of policies that were processed and also it doesn't give more expressive power than a single policy.
		inputErr = fmt.Errorf("multiple EgressNetworkPolicies in same network namespace (%s) is not allowed; dropping all traffic", policyNames(policies))
		otx.DeleteFlows("table=100, reg0=%d", vnid)
		otx.AddFlow("table=100, reg0=%d, priority=1, actions=drop", vnid)
	} else /* vnid != 0 && len(policies) == 1 */ {
		// Temporarily drop all outgoing traffic, to avoid race conditions while modifying the other rules
		otx.AddFlow("table=100, reg0=%d, cookie=1, priority=65535, actions=drop", vnid)
		otx.DeleteFlows("table=100, reg0=%d, cookie=0/1", vnid)

		dnsFound := false
		for i, rule := range policies[0].Spec.Egress {
			priority := len(policies[0].Spec.Egress) - i

			var action string
			if rule.Type == osapi.EgressNetworkPolicyRuleAllow {
				action = "output:2"
			} else {
				action = "drop"
			}

			var selectors []string
			if len(rule.To.CIDRSelector) > 0 {
				selectors = append(selectors, rule.To.CIDRSelector)
			} else if len(rule.To.DNSName) > 0 {
				dnsFound = true
				ips := egressDNS.GetIPs(policies[0], rule.To.DNSName)
				for _, ip := range ips {
					selectors = append(selectors, ip.String())
				}
			}

			for _, selector := range selectors {
				var dst string
				if selector == "0.0.0.0/0" {
					dst = ""
				} else if selector == "0.0.0.0/32" {
					glog.Warningf("Correcting CIDRSelector '0.0.0.0/32' to '0.0.0.0/0' in EgressNetworkPolicy %s:%s", policies[0].Namespace, policies[0].Name)
					dst = ""
				} else {
					dst = fmt.Sprintf(", nw_dst=%s", selector)
				}

				otx.AddFlow("table=100, reg0=%d, priority=%d, ip%s, actions=%s", vnid, priority, dst, action)
			}
		}

		if dnsFound {
			if err := CheckDNSResolver(); err != nil {
				inputErr = fmt.Errorf("DNS resolver failed: %v, dropping all traffic for namespace: %q", err, namespaces[0])
				otx.DeleteFlows("table=100, reg0=%d", vnid)
				otx.AddFlow("table=100, reg0=%d, priority=1, actions=drop", vnid)
			}
		}
		otx.DeleteFlows("table=100, reg0=%d, cookie=1/1", vnid)
	}

	txErr := otx.EndTransaction()
	if inputErr != nil {
		return inputErr
	} else {
		return txErr
	}
}

func (oc *ovsController) AddHostSubnetRules(subnet *osapi.HostSubnet) error {
	otx := oc.ovs.NewTransaction()

	otx.AddFlow("table=10, priority=100, tun_src=%s, actions=goto_table:30", subnet.HostIP)
	if vnid, ok := subnet.Annotations[osapi.FixedVNIDHostAnnotation]; ok {
		otx.AddFlow("table=50, priority=100, arp, nw_dst=%s, actions=load:%s->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", subnet.Subnet, vnid, subnet.HostIP)
		otx.AddFlow("table=90, priority=100, ip, nw_dst=%s, actions=load:%s->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", subnet.Subnet, vnid, subnet.HostIP)
	} else {
		otx.AddFlow("table=50, priority=100, arp, nw_dst=%s, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", subnet.Subnet, subnet.HostIP)
		otx.AddFlow("table=90, priority=100, ip, nw_dst=%s, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", subnet.Subnet, subnet.HostIP)
	}

	return otx.EndTransaction()
}

func (oc *ovsController) DeleteHostSubnetRules(subnet *osapi.HostSubnet) error {
	otx := oc.ovs.NewTransaction()
	otx.DeleteFlows("table=10, tun_src=%s", subnet.HostIP)
	otx.DeleteFlows("table=50, arp, nw_dst=%s", subnet.Subnet)
	otx.DeleteFlows("table=90, ip, nw_dst=%s", subnet.Subnet)
	return otx.EndTransaction()
}

func (oc *ovsController) AddServiceRules(service *kapi.Service, netID uint32) error {
	otx := oc.ovs.NewTransaction()

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

	return otx.EndTransaction()
}

func (oc *ovsController) DeleteServiceRules(service *kapi.Service) error {
	otx := oc.ovs.NewTransaction()
	otx.DeleteFlows(generateBaseServiceRule(service.Spec.ClusterIP))
	return otx.EndTransaction()
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

func (oc *ovsController) UpdateLocalMulticastFlows(vnid uint32, enabled bool, ofports []int) error {
	otx := oc.ovs.NewTransaction()

	if enabled {
		otx.AddFlow("table=110, reg0=%d, actions=goto_table:111", vnid)
	} else {
		otx.DeleteFlows("table=110, reg0=%d", vnid)
	}

	var actions []string
	if enabled && len(ofports) > 0 {
		actions = make([]string, len(ofports))
		for i, ofport := range ofports {
			actions[i] = fmt.Sprintf("output:%d", ofport)
		}
		sort.Strings(actions)
		otx.AddFlow("table=120, priority=100, reg0=%d, actions=%s", vnid, strings.Join(actions, ","))
	} else {
		otx.DeleteFlows("table=120, reg0=%d", vnid)
	}

	return otx.EndTransaction()
}

func (oc *ovsController) UpdateVXLANMulticastFlows(remoteIPs []string) error {
	otx := oc.ovs.NewTransaction()

	if len(remoteIPs) > 0 {
		actions := make([]string, len(remoteIPs))
		for i, ip := range remoteIPs {
			actions[i] = fmt.Sprintf("set_field:%s->tun_dst,output:1", ip)
		}
		sort.Strings(actions)
		otx.AddFlow("table=111, priority=100, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],%s,goto_table:120", strings.Join(actions, ","))
	} else {
		otx.AddFlow("table=111, priority=100, actions=goto_table:120")
	}

	return otx.EndTransaction()
}
