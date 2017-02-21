package ovs

import (
	"strings"
	"testing"
)

func TestFakePorts(t *testing.T) {
	ovsif := NewFake("br0")

	_, err := ovsif.AddPort("tun0", 1)
	if err == nil {
		t.Fatalf("unexpected lack of error adding port on non-existent bridge")
	}

	err = ovsif.AddBridge()
	if err != nil {
		t.Fatalf("unexpected error adding bridge: %v", err)
	}
	ofport, err := ovsif.AddPort("tun0", 17)
	if err != nil {
		t.Fatalf("unexpected error adding port: %v", err)
	}
	if ofport != 17 {
		t.Fatalf("unexpected ofport %d returned from AddPort", ofport)
	}
	ofport, err = ovsif.GetOFPort("tun0")
	if ofport != 17 {
		t.Fatalf("unexpected ofport %d returned from GetOFPort", ofport)
	}
	err = ovsif.DeletePort("tun0")
	if err != nil {
		t.Fatalf("unexpected error deleting port: %v", err)
	}
	_, err = ovsif.GetOFPort("tun0")
	if err == nil {
		t.Fatalf("unexpected lack of error getting non-existent port")
	}
}

type flowTest struct {
	input string
	match ovsFlow
}

func TestParseFlows(t *testing.T) {
	parseTests := []flowTest{
		{
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: ovsFlow{
				table:    0,
				priority: 200,
				fields: []ovsField{
					{name: "in_port", value: "1"},
					{name: "arp", value: ""},
					{name: "nw_src", value: "10.128.0.0/14"},
					{name: "nw_dst", value: "10.128.0.0/23"},
				},
				actions: "move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			},
		},
		{
			input: "table=10, priority=0, actions=drop",
			match: ovsFlow{
				table:    10,
				priority: 0,
				fields:   []ovsField{},
				actions:  "drop",
			},
		},
		{
			input: "table=50, priority=100, arp, nw_dst=10.128.0.0/23, actions=load:0x1234->NXM_NX_TUN_ID[0..31],set_field:172.17.0.2->tun_dst,match:1",
			match: ovsFlow{
				table:    50,
				priority: 100,
				fields: []ovsField{
					{name: "arp", value: ""},
					{name: "nw_dst", value: "10.128.0.0/23"},
				},
				actions: "load:0x1234->NXM_NX_TUN_ID[0..31],set_field:172.17.0.2->tun_dst,match:1",
			},
		},
		{
			input: "table=21, priority=100, ip, actions=ct(commit,table=30)",
			match: ovsFlow{
				table:    21,
				priority: 100,
				fields: []ovsField{
					{name: "ip", value: ""},
				},
				actions: "ct(commit,table=30)",
			},
		},
		{
			// everything after actions is considered part of actions; this would be a syntax error if we actually parsed actions
			input: "table=10, priority=0, actions=drop, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23",
			match: ovsFlow{
				table:    10,
				priority: 0,
				fields:   []ovsField{},
				actions:  "drop, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23",
			},
		},
	}

	for i, test := range parseTests {
		parsed, err := parseFlow(parseForAdd, test.input)
		if err != nil {
			t.Fatalf("unexpected error from parseFlow: %v", err)
		}
		if !flowMatches(parsed, &test.match, true) {
			t.Fatalf("parsed flow %d (%#v) does not match expected output (%#v)", i, parsed, &test.match)
		}
	}
}

func TestParseFlowsDefaults(t *testing.T) {
	parseTests := []flowTest{
		{
			// Default table is 0
			input: "actions=drop",
			match: ovsFlow{
				table:    0,
				priority: 32768,
				fields:   []ovsField{},
				actions:  "drop",
			},
		},
	}

	for i, test := range parseTests {
		parsed, err := parseFlow(parseForAdd, test.input)
		if err != nil {
			t.Fatalf("unexpected error from parseFlow: %v", err)
		}
		if !flowMatches(parsed, &test.match, true) {
			t.Fatalf("parsed flow %d (%#v) does not match expected output (%#v)", i, parsed, &test.match)
		}
	}
}

func TestParseFlowsBad(t *testing.T) {
	parseTests := []flowTest{
		{
			// table is empty
			input: "table=, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// table is non-numeric
			input: "table=foo, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// table out of range
			input: "table=-1, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// table out of range
			input: "table=1000, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// priority is empty
			input: "table=0, priority=, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// priority is non-numeric
			input: "table=0, priority=high, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// priority is out of range
			input: "table=0, priority=-1, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// priority is out of range
			input: "table=0, priority=200000, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// field value is empty
			input: "table=0, priority=200, in_port=, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// actions is empty
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=",
		},
		{
			// actions is empty
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions",
		},
		{
			// nw_src/nw_dst without arp/ip
			input: "table=0, priority=200, in_port=1, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
	}

	for i, test := range parseTests {
		_, err := parseFlow(parseForAdd, test.input)
		if err == nil {
			t.Fatalf("unexpected lack of error from parseFlow on %d %q", i, test.input)
		}
	}
}

func TestFlowMatchesBad(t *testing.T) {
	parseTests := []flowTest{
		{
			// Table is wrong
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: ovsFlow{
				table:    10,
				priority: 200,
				fields: []ovsField{
					{name: "in_port", value: "1"},
					{name: "arp", value: ""},
					{name: "nw_src", value: "10.128.0.0/14"},
					{name: "nw_dst", value: "10.128.0.0/23"},
				},
				actions: "move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			},
		},
		{
			// field value is incorrect
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: ovsFlow{
				table:    0,
				priority: 200,
				fields: []ovsField{
					{name: "in_port", value: "2"},
					{name: "arp", value: ""},
					{name: "nw_src", value: "10.128.0.0/14"},
					{name: "nw_dst", value: "10.128.0.0/23"},
				},
				actions: "move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			},
		},
		{
			// present field is matched against empty field
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: ovsFlow{
				table:    0,
				priority: 200,
				fields: []ovsField{
					{name: "in_port", value: ""},
					{name: "arp", value: ""},
					{name: "nw_src", value: "10.128.0.0/14"},
					{name: "nw_dst", value: "10.128.0.0/23"},
				},
				actions: "move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			},
		},
		{
			// empty field is matched against present field
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: ovsFlow{
				table:    0,
				priority: 200,
				fields: []ovsField{
					{name: "in_port", value: "1"},
					{name: "arp", value: "jean"},
					{name: "nw_src", value: "10.128.0.0/14"},
					{name: "nw_dst", value: "10.128.0.0/23"},
				},
				actions: "move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			},
		},
		{
			// match field is not present in input
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: ovsFlow{
				table:    0,
				priority: 200,
				fields: []ovsField{
					{name: "in_port", value: "1"},
					{name: "arp", value: ""},
					{name: "nw_src", value: "10.128.0.0/14"},
					{name: "nw_dst", value: "10.128.0.0/23"},
					{name: "today", value: "wednesday"},
				},
				actions: "move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			},
		},
	}

	for i, test := range parseTests {
		parsed, err := parseFlow(parseForAdd, test.input)
		if err != nil {
			t.Fatalf("unexpected error from parseFlow: %v", err)
		}
		if flowMatches(parsed, &test.match, false) {
			t.Fatalf("parsed flow %d (%#v) unexpectedly matches output (%#v)", i, parsed, &test.match)
		}
	}
}

func TestFakeDumpFlows(t *testing.T) {
	ovsif := NewFake("br0")
	err := ovsif.AddBridge()
	if err != nil {
		t.Fatalf("unexpected error adding bridge: %v", err)
	}

	clusterNetworkCIDR := "10.128.0.0/14"
	localSubnetCIDR := "10.129.0.0/23"
	localSubnetGateway := "10.129.0.1"
	serviceNetworkCIDR := "172.30.0.0/16"

	otx := ovsif.NewTransaction()
	// All the base flows from (the current version of) controller.go, randomly reordered
	otx.AddFlow("table=50, priority=0, actions=drop")
	otx.AddFlow("table=80, priority=0, actions=drop")
	otx.AddFlow("table=30, priority=100, ip, nw_dst=%s, actions=goto_table:60", serviceNetworkCIDR)
	otx.AddFlow("table=40, priority=0, actions=drop")
	otx.AddFlow("table=30, priority=25, ip, nw_dst=224.0.0.0/4, actions=goto_table:110")
	otx.AddFlow("table=30, priority=200, arp, nw_dst=%s, actions=goto_table:40", localSubnetCIDR)
	otx.AddFlow("table=0, priority=200, in_port=2, ip, actions=goto_table:30")
	otx.AddFlow("table=0, priority=200, in_port=1, ip, nw_src=%s, nw_dst=%s, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10", clusterNetworkCIDR, localSubnetCIDR)
	otx.AddFlow("table=120, priority=0, actions=drop")
	otx.AddFlow("table=70, priority=0, actions=drop")
	otx.AddFlow("table=90, priority=0, actions=drop")
	otx.AddFlow("table=111, priority=0, actions=drop")
	otx.AddFlow("table=30, priority=200, ip, nw_dst=%s, actions=goto_table:70", localSubnetCIDR)
	otx.AddFlow("table=100, priority=0, actions=output:2")
	otx.AddFlow("table=0, priority=100, arp, actions=goto_table:20")
	otx.AddFlow("table=0, priority=200, in_port=2, arp, nw_src=%s, nw_dst=%s, actions=goto_table:30", localSubnetGateway, clusterNetworkCIDR)
	otx.AddFlow("table=20, priority=0, actions=drop")
	otx.AddFlow("table=60, priority=200, reg0=0, actions=output:2")
	otx.AddFlow("table=30, priority=0, arp, actions=drop")
	otx.AddFlow("table=30, priority=100, ip, nw_dst=%s, actions=goto_table:90", clusterNetworkCIDR)
	otx.AddFlow("table=0, priority=200, in_port=1, ip, nw_src=%s, nw_dst=224.0.0.0/4, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10", clusterNetworkCIDR)
	otx.AddFlow("table=0, priority=150, in_port=2, actions=drop")
	otx.AddFlow("table=0, priority=150, in_port=1, actions=drop")
	otx.AddFlow("table=110, priority=0, actions=drop")
	otx.AddFlow("table=60, priority=0, actions=drop")
	otx.AddFlow("table=10, priority=0, actions=drop")
	otx.AddFlow("table=30, priority=300, arp, nw_dst=%s, actions=output:2", localSubnetGateway)
	otx.AddFlow("table=30, priority=100, arp, nw_dst=%s, actions=goto_table:50", clusterNetworkCIDR)
	otx.AddFlow("table=0, priority=250, in_port=2, ip, nw_dst=224.0.0.0/4, actions=drop")
	otx.AddFlow("table=80, priority=300, ip, nw_src=%s/32, actions=output:NXM_NX_REG2[]", localSubnetGateway)
	otx.AddFlow("table=21, priority=0, actions=goto_table:30")
	otx.AddFlow("table=30, priority=0, ip, actions=goto_table:100")
	otx.AddFlow("table=0, priority=0, actions=drop")
	otx.AddFlow("table=0, priority=100, ip, actions=goto_table:20")
	otx.AddFlow("table=0, priority=200, in_port=1, arp, nw_src=%s, nw_dst=%s, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10", clusterNetworkCIDR, localSubnetCIDR)
	otx.AddFlow("table=30, priority=50, in_port=1, ip, nw_dst=224.0.0.0/4, actions=goto_table:120")
	otx.AddFlow("table=30, priority=300, ip, nw_dst=%s, actions=output:2", localSubnetGateway)

	err = otx.EndTransaction()
	if err != nil {
		t.Fatalf("unexpected error from AddFlow: %v", err)
	}

	dumpedFlows, err := ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("unexpected error from DumpFlows: %v", err)
	}

	// fake DumpFlows sorts first by table, then by priority (decreasing), then by creation time
	cmpFlows := []string{
		" cookie=0, table=0, priority=250, in_port=2, ip, nw_dst=224.0.0.0/4, actions=drop",
		" cookie=0, table=0, priority=200, in_port=2, ip, actions=goto_table:30",
		" cookie=0, table=0, priority=200, in_port=1, ip, nw_src=10.128.0.0/14, nw_dst=10.129.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		" cookie=0, table=0, priority=200, in_port=2, arp, nw_src=10.129.0.1, nw_dst=10.128.0.0/14, actions=goto_table:30",
		" cookie=0, table=0, priority=200, in_port=1, ip, nw_src=10.128.0.0/14, nw_dst=224.0.0.0/4, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		" cookie=0, table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.129.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		" cookie=0, table=0, priority=150, in_port=2, actions=drop",
		" cookie=0, table=0, priority=150, in_port=1, actions=drop",
		" cookie=0, table=0, priority=100, arp, actions=goto_table:20",
		" cookie=0, table=0, priority=100, ip, actions=goto_table:20",
		" cookie=0, table=0, priority=0, actions=drop",
		" cookie=0, table=10, priority=0, actions=drop",
		" cookie=0, table=20, priority=0, actions=drop",
		" cookie=0, table=21, priority=0, actions=goto_table:30",
		" cookie=0, table=30, priority=300, arp, nw_dst=10.129.0.1, actions=output:2",
		" cookie=0, table=30, priority=300, ip, nw_dst=10.129.0.1, actions=output:2",
		" cookie=0, table=30, priority=200, arp, nw_dst=10.129.0.0/23, actions=goto_table:40",
		" cookie=0, table=30, priority=200, ip, nw_dst=10.129.0.0/23, actions=goto_table:70",
		" cookie=0, table=30, priority=100, ip, nw_dst=172.30.0.0/16, actions=goto_table:60",
		" cookie=0, table=30, priority=100, ip, nw_dst=10.128.0.0/14, actions=goto_table:90",
		" cookie=0, table=30, priority=100, arp, nw_dst=10.128.0.0/14, actions=goto_table:50",
		" cookie=0, table=30, priority=50, in_port=1, ip, nw_dst=224.0.0.0/4, actions=goto_table:120",
		" cookie=0, table=30, priority=25, ip, nw_dst=224.0.0.0/4, actions=goto_table:110",
		" cookie=0, table=30, priority=0, arp, actions=drop",
		" cookie=0, table=30, priority=0, ip, actions=goto_table:100",
		" cookie=0, table=40, priority=0, actions=drop",
		" cookie=0, table=50, priority=0, actions=drop",
		" cookie=0, table=60, priority=200, reg0=0, actions=output:2",
		" cookie=0, table=60, priority=0, actions=drop",
		" cookie=0, table=70, priority=0, actions=drop",
		" cookie=0, table=80, priority=300, ip, nw_src=10.129.0.1/32, actions=output:NXM_NX_REG2[]",
		" cookie=0, table=80, priority=0, actions=drop",
		" cookie=0, table=90, priority=0, actions=drop",
		" cookie=0, table=100, priority=0, actions=output:2",
		" cookie=0, table=110, priority=0, actions=drop",
		" cookie=0, table=111, priority=0, actions=drop",
		" cookie=0, table=120, priority=0, actions=drop",
	}

	if len(dumpedFlows) != len(cmpFlows) {
		t.Fatalf("wrong number of flows returned (expected %d, got %d)", len(cmpFlows), len(dumpedFlows))
	}
	for i := range cmpFlows {
		if dumpedFlows[i] != cmpFlows[i] {
			t.Fatalf("mismatch at %d (expected %q, got %q)", i, cmpFlows[i], dumpedFlows[i])
		}
	}
}

func matchActions(flows []string, actions ...string) bool {
	if len(flows) != len(actions) {
		return false
	}
	for i := range flows {
		if !strings.HasSuffix(flows[i], "actions="+actions[i]) {
			return false
		}
	}
	return true
}

func TestFlowMatchesMasked(t *testing.T) {
	ovsif := NewFake("br0")
	err := ovsif.AddBridge()
	if err != nil {
		t.Fatalf("unexpected error adding bridge: %v", err)
	}

	otx := ovsif.NewTransaction()
	otx.AddFlow("table=100, priority=100, reg0=1, actions=one")
	otx.AddFlow("table=100, priority=200, reg0=2, actions=two")
	otx.AddFlow("table=100, priority=300, reg0=3, cookie=1, actions=three")
	otx.AddFlow("table=100, priority=400, reg0=4, cookie=0xe, actions=four")
	err = otx.EndTransaction()
	if err != nil {
		t.Fatalf("unexpected error from AddFlow: %v", err)
	}
	flows, err := ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("unexpected error from DumpFlows: %v", err)
	}
	if !matchActions(flows, "four", "three", "two", "one") {
		t.Fatalf("unexpected output from DumpFlows: %#v", flows)
	}

	otx = ovsif.NewTransaction()
	otx.DeleteFlows("table=100, cookie=0/0xFFFF")
	err = otx.EndTransaction()
	if err != nil {
		t.Fatalf("unexpected error from AddFlow: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("unexpected error from DumpFlows: %v", err)
	}
	if !matchActions(flows, "four", "three") {
		t.Fatalf("unexpected output from DumpFlows: %#v", flows)
	}

	otx = ovsif.NewTransaction()
	otx.DeleteFlows("table=100, cookie=2/2")
	err = otx.EndTransaction()
	if err != nil {
		t.Fatalf("unexpected error from AddFlow: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("unexpected error from DumpFlows: %v", err)
	}
	if !matchActions(flows, "three") {
		t.Fatalf("unexpected output from DumpFlows: %#v", flows)
	}
}
