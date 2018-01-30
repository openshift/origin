// +build linux

package node

import (
	"fmt"
	"net"
	"reflect"
	"sort"
	"strings"
	"testing"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/util/ovs"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/containernetworking/plugins/pkg/utils/hwaddr"
)

func setupOVSController(t *testing.T) (ovs.Interface, *ovsController, []string) {
	ovsif := ovs.NewFake(Br0)
	oc := NewOVSController(ovsif, 0, true, "172.17.0.4")
	oc.tunMAC = "c6:ac:2c:13:48:4b"
	err := oc.SetupOVS([]string{"10.128.0.0/14"}, "172.30.0.0/16", "10.128.0.0/23", "10.128.0.1")
	if err != nil {
		t.Fatalf("Unexpected error setting up OVS: %v", err)
	}

	origFlows, err := ovsif.DumpFlows("")
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
	ovsif, oc, origFlows := setupOVSController(t)

	hs := networkapi.HostSubnet{
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

	flows, err := ovsif.DumpFlows("")
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
			match: []string{"table=50", "arp", "arp_tpa=10.129.0.0/23", "192.168.1.2->tun_dst"},
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
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows) // no changes

	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}
}

func TestOVSService(t *testing.T) {
	ovsif, oc, origFlows := setupOVSController(t)

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

	flows, err := ovsif.DumpFlows("")
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
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows) // no changes

	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}
}

const (
	sandboxID string = "bcb5d8d287fcf97458c48ad643b101079e3bc265a94e097e7407440716112f69"
)

func TestOVSPod(t *testing.T) {
	ovsif, oc, origFlows := setupOVSController(t)

	// Add
	ofport, err := oc.SetUpPod(sandboxID, "veth1", net.ParseIP("10.128.0.2"), 42)
	if err != nil {
		t.Fatalf("Unexpected error adding pod rules: %v", err)
	}

	flows, err := ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=20", fmt.Sprintf("in_port=%d", ofport), "arp", "10.128.0.2", "00:00:0a:80:00:02/00:00:ff:ff:ff:ff"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=20", fmt.Sprintf("in_port=%d", ofport), "ip", "10.128.0.2", "42->NXM_NX_REG0"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=25", "ip", "10.128.0.2", "42->NXM_NX_REG0"},
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
	err = oc.UpdatePod(sandboxID, 43)
	if err != nil {
		t.Fatalf("Unexpected error updating pod rules: %v", err)
	}

	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=20", fmt.Sprintf("in_port=%d", ofport), "arp", "10.128.0.2", "00:00:0a:80:00:02/00:00:ff:ff:ff:ff"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=20", fmt.Sprintf("in_port=%d", ofport), "ip", "10.128.0.2", "43->NXM_NX_REG0"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=25", "ip", "10.128.0.2", "43->NXM_NX_REG0"},
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
	err = oc.TearDownPod(sandboxID)
	if err != nil {
		t.Fatalf("Unexpected error deleting pod rules: %v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows) // no changes

	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}
}

func TestGetPodDetails(t *testing.T) {
	type testcase struct {
		sandboxID string
		ip        string
		errStr    string
	}

	testcases := []testcase{
		{
			sandboxID: sandboxID,
			ip:        "10.130.0.2",
		},
	}

	for _, tc := range testcases {
		_, oc, _ := setupOVSController(t)
		tcOFPort, err := oc.SetUpPod(tc.sandboxID, "veth1", net.ParseIP(tc.ip), 42)
		if err != nil {
			t.Fatalf("Unexpected error adding pod rules: %v", err)
		}

		ofport, ip, err := oc.getPodDetailsBySandboxID(tc.sandboxID)
		if err != nil {
			if tc.errStr != "" {
				if !strings.Contains(err.Error(), tc.errStr) {
					t.Fatalf("unexpected error %v (expected %q)", err, tc.errStr)
				}
			} else {
				t.Fatalf("unexpected failure %v", err)
			}
		} else if tc.errStr != "" {
			t.Fatalf("expected error %q", tc.errStr)
		}
		if ofport != tcOFPort {
			t.Fatalf("unexpected ofport %d (expected %d)", ofport, tcOFPort)
		}
		if ip.String() != tc.ip {
			t.Fatalf("unexpected ip %q (expected %q)", ip.String(), tc.ip)
		}
	}
}

func TestOVSMulticast(t *testing.T) {
	ovsif, oc, origFlows := setupOVSController(t)

	// local flows
	err := oc.UpdateLocalMulticastFlows(99, true, []int{4, 5, 6})
	if err != nil {
		t.Fatalf("Unexpected error adding multicast flows: %v", err)
	}
	flows, err := ovsif.DumpFlows("")
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
	flows, err = ovsif.DumpFlows("")
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
	flows, err = ovsif.DumpFlows("")
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
	flows, err = ovsif.DumpFlows("")
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
	flows, err = ovsif.DumpFlows("")
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
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows) // no changes
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %#v\nNew: %#v", err, origFlows, flows)
	}
}

var enp1 = networkapi.EgressNetworkPolicy{
	TypeMeta: metav1.TypeMeta{
		Kind: "EgressNetworkPolicy",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "enp1",
	},
	Spec: networkapi.EgressNetworkPolicySpec{
		Egress: []networkapi.EgressNetworkPolicyRule{
			{
				Type: networkapi.EgressNetworkPolicyRuleAllow,
				To: networkapi.EgressNetworkPolicyPeer{
					CIDRSelector: "192.168.0.0/16",
				},
			},
			{
				Type: networkapi.EgressNetworkPolicyRuleDeny,
				To: networkapi.EgressNetworkPolicyPeer{
					CIDRSelector: "192.168.1.0/24",
				},
			},
			{
				Type: networkapi.EgressNetworkPolicyRuleAllow,
				To: networkapi.EgressNetworkPolicyPeer{
					CIDRSelector: "192.168.1.1/32",
				},
			},
		},
	},
}

var enp2 = networkapi.EgressNetworkPolicy{
	TypeMeta: metav1.TypeMeta{
		Kind: "EgressNetworkPolicy",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "enp2",
	},
	Spec: networkapi.EgressNetworkPolicySpec{
		Egress: []networkapi.EgressNetworkPolicyRule{
			{
				Type: networkapi.EgressNetworkPolicyRuleAllow,
				To: networkapi.EgressNetworkPolicyPeer{
					CIDRSelector: "192.168.1.0/24",
				},
			},
			{
				Type: networkapi.EgressNetworkPolicyRuleAllow,
				To: networkapi.EgressNetworkPolicyPeer{
					CIDRSelector: "192.168.2.0/24",
				},
			},
			{
				Type: networkapi.EgressNetworkPolicyRuleDeny,
				To: networkapi.EgressNetworkPolicyPeer{
					// "/32" is wrong but accepted for backward-compatibility
					CIDRSelector: "0.0.0.0/32",
				},
			},
		},
	},
}

var enpDenyAll = networkapi.EgressNetworkPolicy{
	TypeMeta: metav1.TypeMeta{
		Kind: "EgressNetworkPolicy",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "enpDenyAll",
	},
	Spec: networkapi.EgressNetworkPolicySpec{
		Egress: []networkapi.EgressNetworkPolicyRule{
			{
				Type: networkapi.EgressNetworkPolicyRuleDeny,
				To: networkapi.EgressNetworkPolicyPeer{
					CIDRSelector: "0.0.0.0/0",
				},
			},
		},
	},
}

type enpFlowAddition struct {
	policy *networkapi.EgressNetworkPolicy
	vnid   int
}

func assertENPFlowAdditions(origFlows, newFlows []string, additions ...enpFlowAddition) error {
	changes := make([]flowChange, 0)
	for _, addition := range additions {
		for i, rule := range addition.policy.Spec.Egress {
			var change flowChange
			change.kind = flowAdded
			change.match = []string{
				"table=101",
				fmt.Sprintf("reg0=%d", addition.vnid),
				fmt.Sprintf("priority=%d", len(addition.policy.Spec.Egress)-i),
			}
			if rule.To.CIDRSelector == "0.0.0.0/0" || rule.To.CIDRSelector == "0.0.0.0/32" {
				change.noMatch = []string{"nw_dst"}
			} else {
				change.match = append(change.match, fmt.Sprintf("nw_dst=%s", rule.To.CIDRSelector))
			}
			if rule.Type == networkapi.EgressNetworkPolicyRuleAllow {
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
	ovsif, oc, origFlows := setupOVSController(t)

	// SUCCESSFUL CASES

	// Set one EgressNetworkPolicy on VNID 42
	err := oc.UpdateEgressNetworkPolicyRules(
		[]networkapi.EgressNetworkPolicy{enp1},
		42,
		[]string{"ns1"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err := ovsif.DumpFlows("")
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
		[]networkapi.EgressNetworkPolicy{enp2},
		43,
		[]string{"ns2"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows("")
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
		[]networkapi.EgressNetworkPolicy{enp2},
		42,
		[]string{"ns1"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows("")
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
		[]networkapi.EgressNetworkPolicy{},
		43,
		[]string{"ns2"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows("")
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
		[]networkapi.EgressNetworkPolicy{},
		0,
		[]string{"default", "my-global-project"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows("")
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
		[]networkapi.EgressNetworkPolicy{},
		44,
		[]string{"ns3", "ns4"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows("")
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
		[]networkapi.EgressNetworkPolicy{enp1},
		0,
		[]string{"default"},
		nil,
	)
	if err == nil {
		t.Fatalf("Unexpected lack of error updating egress network policy")
	}
	flows, err = ovsif.DumpFlows("")
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
		[]networkapi.EgressNetworkPolicy{enp1},
		45,
		[]string{"ns3", "ns4"},
		nil,
	)
	if err == nil {
		t.Fatalf("Unexpected lack of error updating egress network policy")
	}
	flows, err = ovsif.DumpFlows("")
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
		[]networkapi.EgressNetworkPolicy{enp1, enp2},
		46,
		[]string{"ns5"},
		nil,
	)
	if err == nil {
		t.Fatalf("Unexpected lack of error updating egress network policy")
	}
	flows, err = ovsif.DumpFlows("")
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
		[]networkapi.EgressNetworkPolicy{},
		45,
		[]string{"ns3", "ns4"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows("")
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
		[]networkapi.EgressNetworkPolicy{},
		46,
		[]string{"ns5"},
		nil,
	)
	if err != nil {
		t.Fatalf("Unexpected error updating egress network policy: %v", err)
	}
	flows, err = ovsif.DumpFlows("")
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

func TestAlreadySetUp(t *testing.T) {
	testcases := []struct {
		flow    string
		success bool
	}{
		{
			// Good note
			flow:    fmt.Sprintf("cookie=0x0, duration=4.796s, table=253, n_packets=0, n_bytes=0, actions=note:00.%02x.00.00.00.00", ruleVersion),
			success: true,
		},
		{
			// Wrong version
			flow:    fmt.Sprintf("cookie=0x0, duration=4.796s, table=253, n_packets=0, n_bytes=0, actions=note:00.%02x.00.00.00.00", ruleVersion-1),
			success: false,
		},
		{
			// Wrong table
			flow:    fmt.Sprintf("cookie=0x0, duration=4.796s, table=10, n_packets=0, n_bytes=0, actions=note:00.%02x.00.00.00.00", ruleVersion),
			success: false,
		},
		{
			// No note
			flow:    "cookie=0x0, duration=4.796s, table=253, n_packets=0, n_bytes=0, actions=goto_table:50",
			success: false,
		},
	}

	for i, tc := range testcases {
		ovsif := ovs.NewFake(Br0)
		if err := ovsif.AddBridge("fail-mode=secure", "protocols=OpenFlow13"); err != nil {
			t.Fatalf("(%d) unexpected error from AddBridge: %v", i, err)
		}
		oc := NewOVSController(ovsif, 0, true, "172.17.0.4")

		otx := ovsif.NewTransaction()
		otx.AddFlow(tc.flow)
		if err := otx.EndTransaction(); err != nil {
			t.Fatalf("(%d) unexpected error from AddFlow: %v", i, err)
		}
		if success := oc.AlreadySetUp(); success != tc.success {
			t.Fatalf("(%d) unexpected setup value %v (expected %v)", i, success, tc.success)
		}
	}
}

func TestSyncVNIDRules(t *testing.T) {
	testcases := []struct {
		flows  []string
		unused []int
	}{
		{
			/* Both VNIDs have 1 pod and 1 service, so they stay */
			flows: []string{
				"table=60,priority=200,reg0=0 actions=output:2",
				"table=60,priority=100,ip,nw_dst=172.30.0.1,nw_frag=later actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,ip,nw_dst=172.30.156.103,nw_frag=later actions=load:0xcb81e9->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,ip,nw_dst=172.30.76.192,nw_frag=later actions=load:0x55fac->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.0.1,tp_dst=443 actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,udp,nw_dst=172.30.0.1,tp_dst=53 actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.0.1,tp_dst=53 actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.156.103,tp_dst=5454 actions=load:0xcb81e9->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.76.192,tp_dst=5454 actions=load:0x55fac->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.76.192,tp_dst=5455 actions=load:0x55fac->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=0 actions=drop",
				"table=70,priority=100,ip,nw_dst=10.129.0.2 actions=load:0x55fac->NXM_NX_REG1[],load:0x3->NXM_NX_REG2[],goto_table:80",
				"table=70,priority=100,ip,nw_dst=10.129.0.3 actions=load:0xcb81e9->NXM_NX_REG1[],load:0x4->NXM_NX_REG2[],goto_table:80",
				"table=70,priority=0 actions=drop",
				"table=80,priority=300,ip,nw_src=10.129.0.1 actions=output:NXM_NX_REG2[]",
				"table=80,priority=200,reg0=0 actions=output:NXM_NX_REG2[]",
				"table=80,priority=200,reg1=0 actions=output:NXM_NX_REG2[]",
				"table=80,priority=100,reg0=0x55fac,reg1=0x55fac actions=output:NXM_NX_REG2[]",
				"table=80,priority=100,reg0=0xcb81e9,reg1=0xcb81e9 actions=output:NXM_NX_REG2[]",
				"table=80,priority=0 actions=drop",
			},
			unused: []int{},
		},
		{
			/* 0xcb81e9 has just a pod, 0x55fac has just a service, both stay */
			flows: []string{
				"table=60,priority=200,reg0=0 actions=output:2",
				"table=60,priority=100,ip,nw_dst=172.30.0.1,nw_frag=later actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,ip,nw_dst=172.30.76.192,nw_frag=later actions=load:0x55fac->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.0.1,tp_dst=443 actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,udp,nw_dst=172.30.0.1,tp_dst=53 actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.0.1,tp_dst=53 actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.76.192,tp_dst=5454 actions=load:0x55fac->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.76.192,tp_dst=5455 actions=load:0x55fac->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=0 actions=drop",
				"table=70,priority=100,ip,nw_dst=10.129.0.3 actions=load:0xcb81e9->NXM_NX_REG1[],load:0x4->NXM_NX_REG2[],goto_table:80",
				"table=70,priority=0 actions=drop",
				"table=80,priority=300,ip,nw_src=10.129.0.1 actions=output:NXM_NX_REG2[]",
				"table=80,priority=200,reg0=0 actions=output:NXM_NX_REG2[]",
				"table=80,priority=200,reg1=0 actions=output:NXM_NX_REG2[]",
				"table=80,priority=100,reg0=0x55fac,reg1=0x55fac actions=output:NXM_NX_REG2[]",
				"table=80,priority=100,reg0=0xcb81e9,reg1=0xcb81e9 actions=output:NXM_NX_REG2[]",
				"table=80,priority=0 actions=drop",
			},
			unused: []int{},
		},
		{
			/* 0xcb81e9 gets GCed, 0x55fac stays */
			flows: []string{
				"table=60,priority=200,reg0=0 actions=output:2",
				"table=60,priority=100,ip,nw_dst=172.30.0.1,nw_frag=later actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,ip,nw_dst=172.30.76.192,nw_frag=later actions=load:0x55fac->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.0.1,tp_dst=443 actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,udp,nw_dst=172.30.0.1,tp_dst=53 actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.0.1,tp_dst=53 actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.76.192,tp_dst=5454 actions=load:0x55fac->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.76.192,tp_dst=5455 actions=load:0x55fac->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=0 actions=drop",
				"table=70,priority=100,ip,nw_dst=10.129.0.2 actions=load:0x55fac->NXM_NX_REG1[],load:0x3->NXM_NX_REG2[],goto_table:80",
				"table=70,priority=0 actions=drop",
				"table=80,priority=300,ip,nw_src=10.129.0.1 actions=output:NXM_NX_REG2[]",
				"table=80,priority=200,reg0=0 actions=output:NXM_NX_REG2[]",
				"table=80,priority=200,reg1=0 actions=output:NXM_NX_REG2[]",
				"table=80,priority=100,reg0=0x55fac,reg1=0x55fac actions=output:NXM_NX_REG2[]",
				"table=80,priority=100,reg0=0xcb81e9,reg1=0xcb81e9 actions=output:NXM_NX_REG2[]",
				"table=80,priority=0 actions=drop",
			},
			unused: []int{0xcb81e9},
		},
		{
			/* Both get GCed */
			flows: []string{
				"table=60,priority=200,reg0=0 actions=output:2",
				"table=60,priority=100,ip,nw_dst=172.30.0.1,nw_frag=later actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.0.1,tp_dst=443 actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,udp,nw_dst=172.30.0.1,tp_dst=53 actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=100,tcp,nw_dst=172.30.0.1,tp_dst=53 actions=load:0->NXM_NX_REG1[],load:0x2->NXM_NX_REG2[],goto_table:80",
				"table=60,priority=0 actions=drop",
				"table=70,priority=0 actions=drop",
				"table=80,priority=300,ip,nw_src=10.129.0.1 actions=output:NXM_NX_REG2[]",
				"table=80,priority=200,reg0=0 actions=output:NXM_NX_REG2[]",
				"table=80,priority=200,reg1=0 actions=output:NXM_NX_REG2[]",
				"table=80,priority=100,reg0=0x55fac,reg1=0x55fac actions=output:NXM_NX_REG2[]",
				"table=80,priority=100,reg0=0xcb81e9,reg1=0xcb81e9 actions=output:NXM_NX_REG2[]",
				"table=80,priority=0 actions=drop",
			},
			unused: []int{0x55fac, 0xcb81e9},
		},
	}

	for i, tc := range testcases {
		_, oc, _ := setupOVSController(t)

		otx := oc.NewTransaction()
		for _, flow := range tc.flows {
			otx.AddFlow(flow)
		}
		if err := otx.EndTransaction(); err != nil {
			t.Fatalf("(%d) unexpected error from AddFlow: %v", i, err)
		}

		unused := oc.FindUnusedVNIDs()
		sort.Ints(unused)
		if !reflect.DeepEqual(unused, tc.unused) {
			t.Fatalf("(%d) wrong result, expected %v, got %v", i, tc.unused, unused)
		}
	}
}

// Ensure that CNI's IP-addressed-based MAC addresses use the IP in the way we expect
func TestSetHWAddrByIP(t *testing.T) {
	ip := net.ParseIP("1.2.3.4")
	hwAddr, err := hwaddr.GenerateHardwareAddr4(ip, hwaddr.PrivateMACPrefix)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedHWAddr := net.HardwareAddr(append(hwaddr.PrivateMACPrefix, ip.To4()...))
	if !reflect.DeepEqual(hwAddr, expectedHWAddr) {
		t.Fatalf("hwaddr.GenerateHardwareAddr4 changed behavior! (%#v != %#v)", hwAddr, expectedHWAddr)
	}
}
