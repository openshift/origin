package plugin

import (
	"fmt"
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
	return fmt.Sprintf("note:%02X.%02X", VERSION, oc.pluginId)
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

	// Table 110: outbound multicast filtering, updated by updateLocalMulticastFlows() in pod.go
	// eg, "table=110, priority=100, reg0=${tenant_id}, actions=goto_table:111
	otx.AddFlow("table=110, priority=0, actions=drop")

	// Table 111: multicast delivery from local pods to the VXLAN; only one rule, updated by updateVXLANMulticastRules() in subnets.go
	// eg, "table=111, priority=100, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:${remote_node_ip_1}->tun_dst,output:1,set_field:${remote_node_ip_2}->tun_dst,output:1,goto_table:120"
	otx.AddFlow("table=111, priority=0, actions=drop")

	// Table 120: multicast delivery to local pods (either from VXLAN or local pods); updated by updateLocalMulticastFlows() in pod.go
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
