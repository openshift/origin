package node

import (
	"testing"

	"github.com/openshift/origin/pkg/util/ovs"
)

func setPacketCounts(ovsif ovs.Interface, nodeIP string, sent, received int64) {
	otx := ovsif.NewTransaction()
	otx.DeleteFlows("table=10, tun_src=%s", nodeIP)
	otx.DeleteFlows("table=100, dummy=%s", nodeIP)
	if received >= 0 {
		otx.AddFlow("table=10, n_packets=%d, tun_src=%s, actions=goto_table:30", received, nodeIP)
	}
	if sent >= 0 {
		otx.AddFlow("table=100, n_packets=%d, dummy=%s, actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", sent, nodeIP, nodeIP)
	}
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
	setPacketCounts(ovsif, "192.168.1.1", 0, 0)
	setPacketCounts(ovsif, "192.168.1.2", -1, 0)
	setPacketCounts(ovsif, "192.168.1.3", 0, 0)
	setPacketCounts(ovsif, "192.168.1.4", -1, 0)
	setPacketCounts(ovsif, "192.168.1.5", 0, 0)

	updates := make(chan *egressVXLANNode, 10)
	evm := newEgressVXLANMonitor(ovsif, updates)
	evm.pollInterval = 0

	evm.AddNode("192.168.1.1", "")
	evm.AddNode("192.168.1.3", "")
	evm.AddNode("192.168.1.5", "")

	// Everything should be fine at startup
	retry := evm.check(false)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Initial check showed updated node %#v", update)
	}
	if retry {
		t.Fatalf("Initial check requested retry")
	}

	// Send and receive some traffic
	setPacketCounts(ovsif, "192.168.1.1", 10, 10)
	setPacketCounts(ovsif, "192.168.1.2", -1, 20)
	setPacketCounts(ovsif, "192.168.1.3", 10, 30)
	setPacketCounts(ovsif, "192.168.1.4", -1, 40)
	setPacketCounts(ovsif, "192.168.1.5", 70, 50)

	retry = evm.check(false)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed updated node %#v", update)
	}
	if retry {
		t.Fatalf("Check erroneously requested retry")
	}

	// Send some more traffic to .3 but don't receive any. Receive some more
	// traffic from 5 but don't send any.
	setPacketCounts(ovsif, "192.168.1.3", 20, 30)
	setPacketCounts(ovsif, "192.168.1.5", 70, 100)

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

	// Since we're only doing retries, it should ignore this
	setPacketCounts(ovsif, "192.168.1.1", 20, 10)

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

	setPacketCounts(ovsif, "192.168.1.1", 20, 20)
	retry = evm.check(false)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed updated node %#v", update)
	}
	if retry {
		t.Fatalf("Check erroneously requested retry")
	}

	// Have .1 lag a bit but then catch up
	setPacketCounts(ovsif, "192.168.1.1", 30, 20)
	retry = evm.check(false)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed updated node %#v", update)
	}
	if !retry {
		t.Fatalf("Check erroneously failed to request retry")
	}

	setPacketCounts(ovsif, "192.168.1.1", 30, 30)
	retry = evm.check(true)
	if update := peekUpdate(updates); update != nil {
		t.Fatalf("Check erroneously showed updated node %#v", update)
	}
	if retry {
		t.Fatalf("Check erroneously requested retry")
	}

	// Now bring back the failed node
	setPacketCounts(ovsif, "192.168.1.3", 50, 40)
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
}
