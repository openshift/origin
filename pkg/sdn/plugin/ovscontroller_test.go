package plugin

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/util/ovs"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
)

func setup(t *testing.T) (ovs.Interface, *ovsController, []string) {
	ovsif := ovs.NewFake(BR)
	oc := NewOVSController(ovsif, 0)
	err := oc.SetupOVS("10.128.0.0/14", "172.30.0.0/16", "10.128.0.0/23", "10.128.0.1")
	if err != nil {
		t.Fatalf("Unexpected error setting up OVS: %v", err)
	}

	origFlows, err := ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}

	return ovsif, oc, origFlows
}

type flowChangeKind string

const (
	flowAdded   flowChangeKind = "added"
	flowRemoved flowChangeKind = "removed"
)

type flowChange struct {
	kind    flowChangeKind
	match   []string
	noMatch []string
}

// assertFlowChanges asserts that origFlows and newFlows differ in the ways described by
// changes, which consists of a series of flows that have been removed from origFlows or
// added to newFlows. There must be exactly 1 matching flow that contains all of the
// strings in match and none of the strings in noMatch.
func assertFlowChanges(origFlows, newFlows []string, changes ...flowChange) error {
	// copy to avoid modifying originals
	dup := make([]string, 0, len(origFlows))
	origFlows = append(dup, origFlows...)
	dup = make([]string, 0, len(newFlows))
	newFlows = append(dup, newFlows...)

	for _, change := range changes {
		var modFlows *[]string
		if change.kind == flowAdded {
			modFlows = &newFlows
		} else {
			modFlows = &origFlows
		}

		matchIndex := -1
		for i, flow := range *modFlows {
			matches := true
			for _, match := range change.match {
				if !strings.Contains(flow, match) {
					matches = false
					break
				}
			}
			for _, nonmatch := range change.noMatch {
				if strings.Contains(flow, nonmatch) {
					matches = false
					break
				}
			}
			if matches {
				if matchIndex == -1 {
					matchIndex = i
				} else {
					return fmt.Errorf("multiple %s flows matching %#v", string(change.kind), change.match)
				}
			}
		}
		if matchIndex == -1 {
			return fmt.Errorf("no %s flow matching %#v", string(change.kind), change.match)
		}
		*modFlows = append((*modFlows)[:matchIndex], (*modFlows)[matchIndex+1:]...)
	}

	if !reflect.DeepEqual(origFlows, newFlows) {
		return fmt.Errorf("unexpected additional changes to flows")
	}
	return nil
}

func TestOVSHostSubnet(t *testing.T) {
	ovsif, oc, origFlows := setup(t)

	hs := osapi.HostSubnet{
		TypeMeta: metav1.TypeMeta{
			Kind: "HostSubnet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
		},
		Host:   "node2",
		HostIP: "192.168.1.2",
		Subnet: "10.129.0.0/23",
	}
	err := oc.AddHostSubnetRules(&hs)
	if err != nil {
		t.Fatalf("Unexpected error adding HostSubnet rules: %v", err)
	}

	flows, err := ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=10", "tun_src=192.168.1.2"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=50", "arp", "nw_dst=10.129.0.0/23", "192.168.1.2->tun_dst"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=90", "ip", "nw_dst=10.129.0.0/23", "192.168.1.2->tun_dst"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	err = oc.DeleteHostSubnetRules(&hs)
	if err != nil {
		t.Fatalf("Unexpected error deleting HostSubnet rules: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows) // no changes

	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}
}

func TestOVSService(t *testing.T) {
	ovsif, oc, origFlows := setup(t)

	svc := kapi.Service{
		TypeMeta: metav1.TypeMeta{
			Kind: "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "service",
		},
		Spec: kapi.ServiceSpec{
			ClusterIP: "172.30.99.99",
			Ports: []kapi.ServicePort{
				{Protocol: kapi.ProtocolTCP, Port: 80},
				{Protocol: kapi.ProtocolTCP, Port: 443},
			},
		},
	}
	err := oc.AddServiceRules(&svc, 42)
	if err != nil {
		t.Fatalf("Unexpected error adding service rules: %v", err)
	}

	flows, err := ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:    flowAdded,
			match:   []string{"table=60", "ip_frag", "42->NXM_NX_REG1"},
			noMatch: []string{"tcp"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=60", "nw_dst=172.30.99.99", "tcp_dst=80", "42->NXM_NX_REG1"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=60", "nw_dst=172.30.99.99", "tcp_dst=443", "42->NXM_NX_REG1"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	err = oc.DeleteServiceRules(&svc)
	if err != nil {
		t.Fatalf("Unexpected error deleting service rules: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows) // no changes

	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}
}

func TestOVSPod(t *testing.T) {
	ovsif, oc, origFlows := setup(t)

	// Add
	ofport, err := oc.SetUpPod("veth1", "10.128.0.2", "11:22:33:44:55:66", 42)
	if err != nil {
		t.Fatalf("Unexpected error adding pod rules: %v", err)
	}

	flows, err := ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=20", fmt.Sprintf("in_port=%d", ofport), "arp", "10.128.0.2", "11:22:33:44:55:66"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=20", fmt.Sprintf("in_port=%d", ofport), "ip", "10.128.0.2", "42->NXM_NX_REG0"},
		},
		flowChange{
			kind:    flowAdded,
			match:   []string{"table=40", "arp", "10.128.0.2", fmt.Sprintf("output:%d", ofport)},
			noMatch: []string{"reg0=42"},
		},
		flowChange{
			kind:    flowAdded,
			match:   []string{"table=70", "ip", "10.128.0.2", "42->NXM_NX_REG1", fmt.Sprintf("%d->NXM_NX_REG2", ofport)},
			noMatch: []string{"reg0=42"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	// Update
	err = oc.UpdatePod("veth1", "10.128.0.2", "11:22:33:44:55:66", 43)
	if err != nil {
		t.Fatalf("Unexpected error adding pod rules: %v", err)
	}

	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=20", fmt.Sprintf("in_port=%d", ofport), "arp", "10.128.0.2", "11:22:33:44:55:66"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=20", fmt.Sprintf("in_port=%d", ofport), "ip", "10.128.0.2", "43->NXM_NX_REG0"},
		},
		flowChange{
			kind:    flowAdded,
			match:   []string{"table=40", "arp", "10.128.0.2", fmt.Sprintf("output:%d", ofport)},
			noMatch: []string{"reg0=43"},
		},
		flowChange{
			kind:    flowAdded,
			match:   []string{"table=70", "ip", "10.128.0.2", "43->NXM_NX_REG1", fmt.Sprintf("%d->NXM_NX_REG2", ofport)},
			noMatch: []string{"reg0=43"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	// Delete
	err = oc.TearDownPod("veth1", "10.128.0.2")
	if err != nil {
		t.Fatalf("Unexpected error deleting pod rules: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows) // no changes

	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}
}

func TestOVSMulticast(t *testing.T) {
	ovsif, oc, origFlows := setup(t)

	// local flows
	err := oc.UpdateLocalMulticastFlows(99, true, []int{4, 5, 6})
	if err != nil {
		t.Fatalf("Unexpected error adding multicast flows: %v", err)
	}
	flows, err := ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=110", "reg0=99", "goto_table:111"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=120", "reg0=99", "output:4,output:5,output:6"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	err = oc.UpdateLocalMulticastFlows(88, false, []int{7, 8})
	if err != nil {
		t.Fatalf("Unexpected error adding multicast flows: %v", err)
	}
	lastFlows := flows
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(lastFlows, flows) // no changes
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	err = oc.UpdateLocalMulticastFlows(99, false, []int{4, 5})
	if err != nil {
		t.Fatalf("Unexpected error adding multicast flows: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows) // no changes
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	// VXLAN
	err = oc.UpdateVXLANMulticastFlows([]string{"192.168.1.2", "192.168.1.5", "192.168.1.3"})
	if err != nil {
		t.Fatalf("Unexpected error adding multicast flows: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:    flowRemoved,
			match:   []string{"table=111", "goto_table:120"},
			noMatch: []string{"->tun_dst"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=111", "192.168.1.2->tun_dst", "192.168.1.3->tun_dst", "192.168.1.5->tun_dst"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	err = oc.UpdateVXLANMulticastFlows([]string{"192.168.1.5", "192.168.1.3"})
	if err != nil {
		t.Fatalf("Unexpected error adding multicast flows: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:    flowRemoved,
			match:   []string{"table=111", "goto_table:120"},
			noMatch: []string{"->tun_dst"},
		},
		flowChange{
			kind:    flowAdded,
			match:   []string{"table=111", "192.168.1.3->tun_dst", "192.168.1.5->tun_dst"},
			noMatch: []string{"192.168.1.2"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	err = oc.UpdateVXLANMulticastFlows([]string{})
	if err != nil {
		t.Fatalf("Unexpected error adding multicast flows: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows) // no changes
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}
}

var enp1 = osapi.EgressNetworkPolicy{
	TypeMeta: metav1.TypeMeta{
		Kind: "EgressNetworkPolicy",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "enp1",
	},
	Spec: osapi.EgressNetworkPolicySpec{
		Egress: []osapi.EgressNetworkPolicyRule{
			{
				Type: osapi.EgressNetworkPolicyRuleAllow,
				To: osapi.EgressNetworkPolicyPeer{
					CIDRSelector: "192.168.0.0/16",
				},
			},
			{
				Type: osapi.EgressNetworkPolicyRuleDeny,
				To: osapi.EgressNetworkPolicyPeer{
					CIDRSelector: "192.168.1.0/24",
				},
			},
			{
				Type: osapi.EgressNetworkPolicyRuleAllow,
				To: osapi.EgressNetworkPolicyPeer{
					CIDRSelector: "192.168.1.1/32",
				},
			},
		},
	},
}

var enp2 = osapi.EgressNetworkPolicy{
	TypeMeta: metav1.TypeMeta{
		Kind: "EgressNetworkPolicy",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "enp2",
	},
	Spec: osapi.EgressNetworkPolicySpec{
		Egress: []osapi.EgressNetworkPolicyRule{
			{
				Type: osapi.EgressNetworkPolicyRuleAllow,
				To: osapi.EgressNetworkPolicyPeer{
					CIDRSelector: "192.168.1.0/24",
				},
			},
			{
				Type: osapi.EgressNetworkPolicyRuleAllow,
				To: osapi.EgressNetworkPolicyPeer{
					CIDRSelector: "192.168.2.0/24",
				},
			},
			{
				Type: osapi.EgressNetworkPolicyRuleDeny,
				To: osapi.EgressNetworkPolicyPeer{
					// "/32" is wrong but accepted for backward-compatibility
					CIDRSelector: "0.0.0.0/32",
				},
			},
		},
	},
}

var enpDenyAll = osapi.EgressNetworkPolicy{
	TypeMeta: metav1.TypeMeta{
		Kind: "EgressNetworkPolicy",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "enpDenyAll",
	},
	Spec: osapi.EgressNetworkPolicySpec{
		Egress: []osapi.EgressNetworkPolicyRule{
			{
				Type: osapi.EgressNetworkPolicyRuleDeny,
				To: osapi.EgressNetworkPolicyPeer{
					CIDRSelector: "0.0.0.0/0",
				},
			},
		},
	},
}

type enpFlowAddition struct {
	policy *osapi.EgressNetworkPolicy
	vnid   int
}

func assertENPFlowAdditions(origFlows, newFlows []string, additions ...enpFlowAddition) error {
	changes := make([]flowChange, 0)
	for _, addition := range additions {
		for i, rule := range addition.policy.Spec.Egress {
			var change flowChange
			change.kind = flowAdded
			change.match = []string{
				"table=100",
				fmt.Sprintf("reg0=%d", addition.vnid),
				fmt.Sprintf("priority=%d", len(addition.policy.Spec.Egress)-i),
			}
			if rule.To.CIDRSelector == "0.0.0.0/0" || rule.To.CIDRSelector == "0.0.0.0/32" {
				change.noMatch = []string{"nw_dst"}
			} else {
				change.match = append(change.match, fmt.Sprintf("nw_dst=%s", rule.To.CIDRSelector))
			}
			if rule.Type == osapi.EgressNetworkPolicyRuleAllow {
				change.match = append(change.match, "actions=output")
			} else {
				change.match = append(change.match, "actions=drop")
			}
			changes = append(changes, change)
		}
	}

	return assertFlowChanges(origFlows, newFlows, changes...)
}

func TestOVSEgressNetworkPolicy(t *testing.T) {
	ovsif, oc, origFlows := setup(t)

	// SUCCESSFUL CASES

	// Set one EgressNetworkPolicy on VNID 42
	err := oc.UpdateEgressNetworkPolicyRules(
		[]osapi.EgressNetworkPolicy{enp1},
		42,
		[]string{"ns1"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err := ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertENPFlowAdditions(origFlows, flows,
		enpFlowAddition{
			vnid:   42,
			policy: &enp1,
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	// Set one EgressNetworkPolicy on VNID 43
	err = oc.UpdateEgressNetworkPolicyRules(
		[]osapi.EgressNetworkPolicy{enp2},
		43,
		[]string{"ns2"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertENPFlowAdditions(origFlows, flows,
		enpFlowAddition{
			vnid:   42,
			policy: &enp1,
		},
		enpFlowAddition{
			vnid:   43,
			policy: &enp2,
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	// Change VNID 42 from ENP1 to ENP2
	err = oc.UpdateEgressNetworkPolicyRules(
		[]osapi.EgressNetworkPolicy{enp2},
		42,
		[]string{"ns1"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertENPFlowAdditions(origFlows, flows,
		enpFlowAddition{
			vnid:   42,
			policy: &enp2,
		},
		enpFlowAddition{
			vnid:   43,
			policy: &enp2,
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	// Drop EgressNetworkPolicy from VNID 43
	err = oc.UpdateEgressNetworkPolicyRules(
		[]osapi.EgressNetworkPolicy{},
		43,
		[]string{"ns2"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertENPFlowAdditions(origFlows, flows,
		enpFlowAddition{
			vnid:   42,
			policy: &enp2,
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	// Set no EgressNetworkPolicy on VNID 0
	err = oc.UpdateEgressNetworkPolicyRules(
		[]osapi.EgressNetworkPolicy{},
		0,
		[]string{"default", "my-global-project"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertENPFlowAdditions(origFlows, flows,
		enpFlowAddition{
			vnid:   42,
			policy: &enp2,
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	// Set no EgressNetworkPolicy on a shared namespace
	err = oc.UpdateEgressNetworkPolicyRules(
		[]osapi.EgressNetworkPolicy{},
		44,
		[]string{"ns3", "ns4"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertENPFlowAdditions(origFlows, flows,
		enpFlowAddition{
			vnid:   42,
			policy: &enp2,
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	// ERROR CASES

	// Can't set non-empty ENP in default namespace
	err = oc.UpdateEgressNetworkPolicyRules(
		[]osapi.EgressNetworkPolicy{enp1},
		0,
		[]string{"default"},
		nil,
	)
	if err == nil {
		t.Fatalf("Unexpected lack of error updating egress network policy")
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertENPFlowAdditions(origFlows, flows,
		enpFlowAddition{
			vnid:   42,
			policy: &enp2,
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	// Can't set non-empty ENP in a shared namespace
	err = oc.UpdateEgressNetworkPolicyRules(
		[]osapi.EgressNetworkPolicy{enp1},
		45,
		[]string{"ns3", "ns4"},
		nil,
	)
	if err == nil {
		t.Fatalf("Unexpected lack of error updating egress network policy")
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertENPFlowAdditions(origFlows, flows,
		enpFlowAddition{
			vnid:   42,
			policy: &enp2,
		},
		enpFlowAddition{
			vnid:   45,
			policy: &enpDenyAll,
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	// Can't set multiple policies
	err = oc.UpdateEgressNetworkPolicyRules(
		[]osapi.EgressNetworkPolicy{enp1, enp2},
		46,
		[]string{"ns5"},
		nil,
	)
	if err == nil {
		t.Fatalf("Unexpected lack of error updating egress network policy")
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertENPFlowAdditions(origFlows, flows,
		enpFlowAddition{
			vnid:   42,
			policy: &enp2,
		},
		enpFlowAddition{
			vnid:   45,
			policy: &enpDenyAll,
		},
		enpFlowAddition{
			vnid:   46,
			policy: &enpDenyAll,
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	// CLEARING ERRORS

	err = oc.UpdateEgressNetworkPolicyRules(
		[]osapi.EgressNetworkPolicy{},
		45,
		[]string{"ns3", "ns4"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertENPFlowAdditions(origFlows, flows,
		enpFlowAddition{
			vnid:   42,
			policy: &enp2,
		},
		enpFlowAddition{
			vnid:   46,
			policy: &enpDenyAll,
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}

	err = oc.UpdateEgressNetworkPolicyRules(
		[]osapi.EgressNetworkPolicy{},
		46,
		[]string{"ns5"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertENPFlowAdditions(origFlows, flows,
		enpFlowAddition{
			vnid:   42,
			policy: &enp2,
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}
}
