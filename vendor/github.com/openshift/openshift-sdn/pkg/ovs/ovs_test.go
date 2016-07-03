package ovs

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openshift/openshift-sdn/pkg/exec"
)

func normalSetup() {
	exec.SetTestMode()
	exec.AddTestProgram("/usr/bin/ovs-ofctl")
	exec.AddTestProgram("/usr/bin/ovs-vsctl")
}

func missingSetup() {
	exec.SetTestMode()
}

func TestTransactionSuccess(t *testing.T) {
	normalSetup()
	exec.AddTestResult("/usr/bin/ovs-ofctl -O OpenFlow13 add-flow br0 flow1", "", nil)
	exec.AddTestResult("/usr/bin/ovs-ofctl -O OpenFlow13 add-flow br0 flow2", "", nil)

	otx := NewTransaction("br0")
	otx.AddFlow("flow1")
	otx.AddFlow("flow2")
	err := otx.EndTransaction()
	if err != nil {
		t.Fatalf("Unexpected error from command: %v", err)
	}
}

func TestTransactionFailure(t *testing.T) {
	normalSetup()
	exec.AddTestResult("/usr/bin/ovs-ofctl -O OpenFlow13 add-flow br0 flow1", "", fmt.Errorf("Something bad happened"))

	otx := NewTransaction("br0")
	otx.AddFlow("flow1")
	otx.AddFlow("flow2")
	err := otx.EndTransaction()
	if err == nil {
		t.Fatalf("Failed to get expected error")
	}
}

func TestDumpFlows(t *testing.T) {
	normalSetup()
	exec.AddTestResult("/usr/bin/ovs-ofctl -O OpenFlow13 dump-flows br0", `OFPST_FLOW reply (OF1.3) (xid=0x2):
 cookie=0x0, duration=13271.779s, table=0, n_packets=0, n_bytes=0, priority=100,ip,nw_dst=192.168.1.0/24 actions=set_field:0a:7b:e6:19:11:cf->eth_dst,output:2
 cookie=0x0, duration=13271.776s, table=0, n_packets=1, n_bytes=42, priority=100,arp,arp_tpa=192.168.1.0/24 actions=set_field:10.19.17.34->tun_dst,output:1
 cookie=0x3, duration=13267.277s, table=0, n_packets=788539827, n_bytes=506520926762, priority=100,ip,nw_dst=192.168.2.2 actions=output:3
 cookie=0x0, duration=13284.668s, table=0, n_packets=505, n_bytes=21210, priority=100,arp,arp_tpa=192.168.2.1 actions=output:2
 cookie=0x0, duration=13284.666s, table=0, n_packets=0, n_bytes=0, priority=100,ip,nw_dst=192.168.2.1 actions=output:2
 cookie=0x3, duration=13267.276s, table=0, n_packets=506, n_bytes=21252, priority=100,arp,arp_tpa=192.168.2.2 actions=output:3
 cookie=0x0, duration=13284.67s, table=0, n_packets=782815611, n_bytes=179416494325, priority=50 actions=output:2
`, nil)

	otx := NewTransaction("br0")
	flows, err := otx.DumpFlows()
	otx.EndTransaction()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if strings.Contains(flows[0], "OFPST") {
		t.Fatalf("DumpFlows() did not filter results correctly")
	}
	if len(flows) != 7 {
		t.Fatalf("Unexpected number of flows (%d)", len(flows))
	}
}

func TestOVSMissing(t *testing.T) {
	missingSetup()
	otx := NewTransaction("br0")
	otx.AddFlow("flow1")
	otx.AddFlow("flow2")
	err := otx.EndTransaction()
	if err == nil {
		t.Fatalf("Unexpectedly did not get error")
	}
	if err.Error() != "OVS is not installed" {
		t.Fatalf("Got wrong error: %v", err)
	}
}
