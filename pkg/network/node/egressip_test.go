package node

import (
	"fmt"
	"testing"
)

func assertNoNetlinkChanges(eip *egressIPWatcher) error {
	select {
	case change := <-eip.testModeChan:
		return fmt.Errorf("Unexpected netlink change %q", change)
	default:
		return nil
	}
}

func assertNetlinkChange(eip *egressIPWatcher, expected string) error {
	select {
	case change := <-eip.testModeChan:
		if change == expected {
			return nil
		}
		return fmt.Errorf("Unexpected netlink change %q (expected %q)", change, expected)
	default:
		return fmt.Errorf("Missing netlink change (expected %q)", expected)
	}
}

func TestEgressIP(t *testing.T) {
	ovsif, oc, origFlows := setupOVSController(t)
	if oc.localIP != "172.17.0.4" {
		panic("details of fake ovsController changed")
	}
	masqBit := int32(0)
	eip := newEgressIPWatcher(oc, "172.17.0.4", &masqBit)
	eip.testModeChan = make(chan string, 10)

	eip.updateNodeEgress("172.17.0.3", []string{})
	eip.updateNodeEgress("172.17.0.4", []string{})
	eip.deleteNamespaceEgress(42)
	eip.deleteNamespaceEgress(43)

	// No namespaces use egress yet, so should be no changes
	err := assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err := ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows) // no changes
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}

	// Assign NetNamespace.EgressIP first, then HostSubnet.EgressIP, with a remote EgressIP
	eip.updateNamespaceEgress(42, "172.17.0.100")
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=100", "reg0=42", "drop"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}

	eip.updateNodeEgress("172.17.0.3", []string{"172.17.0.100"})
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=100", "reg0=42", "172.17.0.3->tun_dst"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}
	origFlows = flows

	// Assign HostSubnet.EgressIP first, then NetNamespace.EgressIP, with a remote EgressIP
	eip.updateNodeEgress("172.17.0.3", []string{"172.17.0.101", "172.17.0.100"})
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}

	eip.updateNamespaceEgress(43, "172.17.0.101")
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=100", "reg0=43", "172.17.0.3->tun_dst"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}
	origFlows = flows

	// Assign NetNamespace.EgressIP first, then HostSubnet.EgressIP, with a local EgressIP
	eip.updateNamespaceEgress(44, "172.17.0.102")
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=100", "reg0=44", "drop"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}

	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.102"})
	err = assertNetlinkChange(eip, "claim 172.17.0.102")
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=100", "reg0=44", "0x0000002c->pkt_mark", "goto_table:101"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}
	origFlows = flows

	// Assign HostSubnet.EgressIP first, then NetNamespace.EgressIP, with a local EgressIP
	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.102", "172.17.0.103"})
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}

	eip.updateNamespaceEgress(45, "172.17.0.103")
	err = assertNetlinkChange(eip, "claim 172.17.0.103")
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=100", "reg0=45", "0x0100002c->pkt_mark", "goto_table:101"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}
	origFlows = flows

	// Drop namespace EgressIP
	eip.deleteNamespaceEgress(44)
	err = assertNetlinkChange(eip, "release 172.17.0.102")
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowRemoved,
			match: []string{"table=100", "reg0=44", "0x0000002c->pkt_mark", "goto_table:101"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}
	origFlows = flows

	// Drop remote node EgressIP
	eip.updateNodeEgress("172.17.0.3", []string{"172.17.0.100"})
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowRemoved,
			match: []string{"table=100", "reg0=43", "172.17.0.3->tun_dst"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=100", "reg0=43", "drop"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}
	origFlows = flows

	// Drop local node EgressIP
	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.102"})
	err = assertNetlinkChange(eip, "release 172.17.0.103")
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowRemoved,
			match: []string{"table=100", "reg0=45", "0x0100002c->pkt_mark", "goto_table:101"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=100", "reg0=45", "drop"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}
	origFlows = flows

	// Trying to assign node IP as egress IP should fail. (It will log an error but this test doesn't notice that.)
	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.4", "172.17.0.102"})
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	flows, err = ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v", err)
	}
}

func TestMarkForVNID(t *testing.T) {
	testcases := []struct {
		description   string
		vnid          uint32
		masqueradeBit uint32
		result        uint32
	}{
		{
			description:   "masqBit in VNID range, but not set in VNID",
			vnid:          0x000000aa,
			masqueradeBit: 0x00000001,
			result:        0x000000aa,
		},
		{
			description:   "masqBit in VNID range, and set in VNID",
			vnid:          0x000000ab,
			masqueradeBit: 0x00000001,
			result:        0x010000aa,
		},
		{
			description:   "masqBit in VNID range, VNID 0",
			vnid:          0x00000000,
			masqueradeBit: 0x00000001,
			result:        0xff000000,
		},
		{
			description:   "masqBit outside of VNID range",
			vnid:          0x000000aa,
			masqueradeBit: 0x80000000,
			result:        0x000000aa,
		},
		{
			description:   "masqBit outside of VNID range, VNID 0",
			vnid:          0x00000000,
			masqueradeBit: 0x80000000,
			result:        0x7f000000,
		},
		{
			description:   "masqBit == bit 24",
			vnid:          0x000000aa,
			masqueradeBit: 0x01000000,
			result:        0x000000aa,
		},
		{
			description:   "masqBit == bit 24, VNID 0",
			vnid:          0x00000000,
			masqueradeBit: 0x01000000,
			result:        0xfe000000,
		},
		{
			description:   "no masqBit, ordinary VNID",
			vnid:          0x000000aa,
			masqueradeBit: 0x00000000,
			result:        0x000000aa,
		},
		{
			description:   "no masqBit, VNID 0",
			vnid:          0x00000000,
			masqueradeBit: 0x00000000,
			result:        0xff000000,
		},
	}

	for _, tc := range testcases {
		result := getMarkForVNID(tc.vnid, tc.masqueradeBit)
		if result != fmt.Sprintf("0x%08x", tc.result) {
			t.Fatalf("test %q expected %08x got %s", tc.description, tc.result, result)
		}
	}
}
