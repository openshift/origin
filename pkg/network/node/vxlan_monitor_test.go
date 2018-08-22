// +build linux

package node

import (
	"testing"

	"github.com/openshift/origin/pkg/util/ovs"
)

func packetsIn(ovsif ovs.Interface, counts map[string]int, nodeIP string) {
	counts[nodeIP] += 1
	otx := ovsif.NewTransaction()
	otx.DeleteFlows("table=10, tun_src=%s", nodeIP)
	otx.AddFlow("table=10, n_packets=%d, tun_src=%s, actions=goto_table:30", counts[nodeIP], nodeIP)
	err := otx.Commit()
	if err != nil {
		panic("can't happen: " + err.Error())
	}
}

func packetsOut(ovsif ovs.Interface, counts map[uint32]int, nodeIP string, vnid uint32) {
	counts[vnid] += 1
	otx := ovsif.NewTransaction()
	otx.DeleteFlows("table=100, dummy=%s, reg0=%d", nodeIP, vnid)
	otx.AddFlow("table=100, n_packets=%d, dummy=%s, reg0=%d, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", counts[vnid], nodeIP, vnid, nodeIP)
	err := otx.Commit()
	if err != nil {
		panic("can't happen: " + err.Error())
	}
}

func peekUpdate(updates chan *egressVXLANNode) *egressVXLANNode {
	select {
	case update := <-updates:
		return update
	default:
		return nil
	}
}

func TestEgressVXLANMonitor(t *testing.T) {
	ovsif := ovs.NewFake(Br0)
	ovsif.AddBridge()

	inCounts := make(map[string]int)
	outCounts := make(map[uint32]int)

	packetsIn(ovsif, inCounts, "192.168.1.1")
	packetsOut(ovsif, outCounts, "192.168.1.1", 0x41)
	packetsIn(ovsif, inCounts, "192.168.1.2")
	packetsIn(ovsif, inCounts, "192.168.1.3")
	packetsOut(ovsif, outCounts, "192.168.1.3", 0x43)
	packetsIn(ovsif, inCounts, "192.168.1.4")
	packetsIn(ovsif, inCounts, "192.168.1.5")
	packetsOut(ovsif, outCounts, "192.168.1.5", 0x45)
	packetsOut(ovsif, outCounts, "192.168.1.5", 0x46)
	packetsOut(ovsif, outCounts, "192.168.1.5", 0x47)

	updates := make(chan *egressVXLANNode, 10)
	evm := newEgressVXLANMonitor(ovsif, nil, updates)
	evm.pollInterval = 0

	evm.AddNode("192.168.1.1")
	evm.AddNode("192.168.1.3")
	evm.AddNode("192.168.1.5")

	// Everything should be fine at startup
	retry := evm.check(false)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Initial check showed updated node %#v", update)
	}
	if retry {
		t.Fatalf("Initial check requested retry")
	}

	// Send and receive some traffic
	packetsOut(ovsif, outCounts, "192.168.1.1", 0x41)
	packetsIn(ovsif, inCounts, "192.168.1.1")

	packetsIn(ovsif, inCounts, "192.168.1.2")

	packetsOut(ovsif, outCounts, "192.168.1.3", 0x43)
	packetsIn(ovsif, inCounts, "192.168.1.3")

	packetsIn(ovsif, inCounts, "192.168.1.4")

	packetsOut(ovsif, outCounts, "192.168.1.5", 0x45)
	packetsIn(ovsif, inCounts, "192.168.1.5")

	retry = evm.check(false)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed updated node %#v", update)
	}
	if retry {
		t.Fatalf("Check erroneously requested retry")
	}

	// Send some more traffic to .3 but don't receive any; this should cause
	// .3 to be noticed as "maybe offline", causing retries. OTOH, receiving
	// traffic on .5 without having sent any should have no effect.
	packetsOut(ovsif, outCounts, "192.168.1.3", 0x43)
	packetsIn(ovsif, inCounts, "192.168.1.5")

	retry = evm.check(false)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed updated node %#v", update)
	}
	if !retry {
		t.Fatalf("Check erroneously failed to request retry")
	}
	retry = evm.check(true)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed updated node %#v", update)
	}
	if !retry {
		t.Fatalf("Check erroneously failed to request retry")
	}

	// Since we're only doing retries right now, it should ignore this
	packetsOut(ovsif, outCounts, "192.168.1.1", 0x41)

	retry = evm.check(true)
	if update := peekUpdate(updates); update == nil {
		t.Fatalf("Check failed to fail after maxRetries")
	} else if update.nodeIP != "192.168.1.3" || !update.offline {
		t.Fatalf("Unexpected update node %#v", update)
	}
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed additional updated node %#v", update)
	}
	if retry {
		t.Fatalf("Check erroneously requested retry")
	}

	// If we update .1 now before the next full check, then the monitor should never
	// notice that it was briefly out of sync.
	packetsIn(ovsif, inCounts, "192.168.1.1")
	retry = evm.check(false)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed updated node %#v", update)
	}
	if retry {
		t.Fatalf("Check erroneously requested retry")
	}

	// Have .1 lag a bit but then catch up
	packetsOut(ovsif, outCounts, "192.168.1.1", 0x41)
	retry = evm.check(false)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed updated node %#v", update)
	}
	if !retry {
		t.Fatalf("Check erroneously failed to request retry")
	}

	packetsIn(ovsif, inCounts, "192.168.1.1")
	retry = evm.check(true)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed updated node %#v", update)
	}
	if retry {
		t.Fatalf("Check erroneously requested retry")
	}

	// Now bring back the failed node
	packetsOut(ovsif, outCounts, "192.168.1.3", 0x43)
	packetsIn(ovsif, inCounts, "192.168.1.3")
	retry = evm.check(false)
	if update := peekUpdate(updates); update == nil {
		t.Fatalf("Node failed to recover")
	} else if update.nodeIP != "192.168.1.3" || update.offline {
		t.Fatalf("Unexpected updated node %#v", update)
	}
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed additional updated node %#v", update)
	}
	if retry {
		t.Fatalf("Check erroneously requested retry")
	}

	// When a node hosts multiple egress IPs, we should notice it failing if *any*
	// IP fails
	packetsOut(ovsif, outCounts, "192.168.1.5", 0x46)
	retry = evm.check(false)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed updated node %#v", update)
	}
	if !retry {
		t.Fatalf("Check erroneously failed to request retry")
	}
	retry = evm.check(true)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed updated node %#v", update)
	}
	if !retry {
		t.Fatalf("Check erroneously failed to request retry")
	}
	retry = evm.check(true)
	if update := peekUpdate(updates); update == nil {
		t.Fatalf("Check failed to fail after maxRetries")
	} else if update.nodeIP != "192.168.1.5" || !update.offline {
		t.Fatalf("Unexpected update node %#v", update)
	}
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed additional updated node %#v", update)
	}
	if retry {
		t.Fatalf("Check erroneously requested retry")
	}

	packetsIn(ovsif, inCounts, "192.168.1.5")
	retry = evm.check(false)
	if update := peekUpdate(updates); update == nil {
		t.Fatalf("Node failed to recover")
	} else if update.nodeIP != "192.168.1.5" || update.offline {
		t.Fatalf("Unexpected updated node %#v", update)
	}
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed additional updated node %#v", update)
	}
	if retry {
		t.Fatalf("Check erroneously requested retry")
	}
}
