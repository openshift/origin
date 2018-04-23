package ovs

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"
)

func normalSetup() *fakeexec.FakeExec {
	return &fakeexec.FakeExec{
		LookPathFunc: func(prog string) (string, error) {
			if prog == "ovs-ofctl" || prog == "ovs-vsctl" {
				return "/sbin/" + prog, nil
			} else {
				return "", fmt.Errorf("%s not found", prog)
			}
		},
	}
}

func missingSetup() *fakeexec.FakeExec {
	return &fakeexec.FakeExec{
		LookPathFunc: func(prog string) (string, error) {
			return "", fmt.Errorf("%s not found", prog)
		},
	}
}

func addTestResult(t *testing.T, fexec *fakeexec.FakeExec, command string, output string, err error) *fakeexec.FakeCmd {
	fcmd := &fakeexec.FakeCmd{
		CombinedOutputScript: []fakeexec.FakeCombinedOutputAction{
			func() ([]byte, error) { return []byte(output), err },
		},
	}
	fexec.CommandScript = append(fexec.CommandScript,
		func(cmd string, args ...string) exec.Cmd {
			execCommand := strings.Join(append([]string{cmd}, args...), " ")
			if execCommand != command {
				t.Fatalf("Unexpected command: wanted %q got %q", command, execCommand)
			}
			return fakeexec.InitFakeCmd(fcmd, cmd, args...)
		})

	return fcmd
}

func ensureTestResults(t *testing.T, fexec *fakeexec.FakeExec) {
	if fexec.CommandCalls != len(fexec.CommandScript) {
		t.Fatalf("Only used %d of %d expected commands", fexec.CommandCalls, len(fexec.CommandScript))
	}
}

func ensureInputFlows(t *testing.T, fakeCmd *fakeexec.FakeCmd, flows []string) {
	allFlows := strings.Join(flows, "\n")

	var fakeCmdFlows string
	if fakeCmd != nil {
		data, err := ioutil.ReadAll(fakeCmd.Stdin)
		if err != nil {
			t.Fatalf(err.Error())
		}
		fakeCmdFlows = string(data)
	}
	if strings.Compare(allFlows, fakeCmdFlows) != 0 {
		t.Fatalf("Expected input flows: %q but got %q", allFlows, fakeCmdFlows)
	}
}

func TestTransactionSuccess(t *testing.T) {
	fexec := normalSetup()

	ovsif, err := New(fexec, "br0", "")
	if err != nil {
		t.Fatalf("Unexpected error from ovs.New(): %v", err)
	}

	// Test Empty transaction
	otx := ovsif.NewTransaction()
	if err = otx.Commit(); err != nil {
		t.Fatalf("Unexpected error from command: %v", err)
	}
	ensureTestResults(t, fexec)
	ensureInputFlows(t, nil, []string{})

	// Test Successful transaction
	fakeCmd := addTestResult(t, fexec, "ovs-ofctl -O OpenFlow13 bundle br0 -", "", nil)
	otx = ovsif.NewTransaction()
	otx.AddFlow("flow1")
	otx.AddFlow("flow2")
	if err = otx.Commit(); err != nil {
		t.Fatalf("Unexpected error from command: %v", err)
	}
	ensureTestResults(t, fexec)
	expectedInputFlows := []string{
		"flow add flow1",
		"flow add flow2",
	}
	ensureInputFlows(t, fakeCmd, expectedInputFlows)

	// Test reuse transaction object
	if err = otx.Commit(); err != nil {
		t.Fatalf("Unexpected error from command: %v", err)
	}
	ensureTestResults(t, fexec)

	// Test Failed transaction
	fakeCmd = addTestResult(t, fexec, "ovs-ofctl -O OpenFlow13 bundle br0 -", "", fmt.Errorf("Something bad happened"))
	otx = ovsif.NewTransaction()
	otx.AddFlow("flow1")
	otx.DeleteFlows("flow2")
	if err = otx.Commit(); err == nil {
		t.Fatalf("Failed to get expected error")
	}
	ensureTestResults(t, fexec)
	expectedInputFlows = []string{
		"flow add flow1",
		"flow delete flow2",
	}
	ensureInputFlows(t, fakeCmd, expectedInputFlows)
}

func TestDumpFlows(t *testing.T) {
	fexec := normalSetup()
	addTestResult(t, fexec, "ovs-ofctl -O OpenFlow13 dump-flows br0 ", `OFPST_FLOW reply (OF1.3) (xid=0x2):
 cookie=0x0, duration=13271.779s, table=0, n_packets=0, n_bytes=0, priority=100,ip,nw_dst=192.168.1.0/24 actions=set_field:0a:7b:e6:19:11:cf->eth_dst,output:2
 cookie=0x0, duration=13271.776s, table=0, n_packets=1, n_bytes=42, priority=100,arp,arp_tpa=192.168.1.0/24 actions=set_field:10.19.17.34->tun_dst,output:1
 cookie=0x3, duration=13267.277s, table=0, n_packets=788539827, n_bytes=506520926762, priority=100,ip,nw_dst=192.168.2.2 actions=output:3
 cookie=0x0, duration=13284.668s, table=0, n_packets=505, n_bytes=21210, priority=100,arp,arp_tpa=192.168.2.1 actions=output:2
 cookie=0x0, duration=13284.666s, table=0, n_packets=0, n_bytes=0, priority=100,ip,nw_dst=192.168.2.1 actions=output:2
 cookie=0x3, duration=13267.276s, table=0, n_packets=506, n_bytes=21252, priority=100,arp,arp_tpa=192.168.2.2 actions=output:3
 cookie=0x0, duration=13284.67s, table=0, n_packets=782815611, n_bytes=179416494325, priority=50 actions=output:2
`, nil)

	ovsif, err := New(fexec, "br0", "")
	if err != nil {
		t.Fatalf("Unexpected error from ovs.New(): %v", err)
	}
	flows, err := ovsif.DumpFlows("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if strings.Contains(flows[0], "OFPST") {
		t.Fatalf("DumpFlows() did not filter results correctly")
	}
	if len(flows) != 7 {
		t.Fatalf("Unexpected number of flows (%d)", len(flows))
	}

	ensureTestResults(t, fexec)
}

func TestOVSMissing(t *testing.T) {
	fexec := missingSetup()
	ovsif, err := New(fexec, "br0", "")
	if err == nil || ovsif != nil {
		t.Fatalf("Unexpectedly did not get error")
	}
	if err.Error() != "OVS is not installed" {
		t.Fatalf("Got wrong error: %v", err)
	}
}

func TestAddPort(t *testing.T) {
	fexec := normalSetup()
	ovsif, err := New(fexec, "br0", "")
	if err != nil {
		t.Fatalf("Unexpected error from ovs.New(): %v", err)
	}

	addTestResult(t, fexec, "ovs-vsctl --timeout=30 --may-exist add-port br0 veth0", "", nil)
	addTestResult(t, fexec, "ovs-vsctl --timeout=30 get Interface veth0 ofport", "1\n", nil)
	port, err := ovsif.AddPort("veth0", -1)
	if err != nil {
		t.Fatalf("Unexpected error from command: %v", err)
	}
	if port != 1 {
		t.Fatalf("Unexpected port number %d", port)
	}
	ensureTestResults(t, fexec)

	addTestResult(t, fexec, "ovs-vsctl --timeout=30 --may-exist add-port br0 veth0 -- set Interface veth0 ofport_request=5", "", nil)
	addTestResult(t, fexec, "ovs-vsctl --timeout=30 get Interface veth0 ofport", "5\n", nil)
	port, err = ovsif.AddPort("veth0", 5)
	if err != nil {
		t.Fatalf("Unexpected error from command: %v", err)
	}
	if port != 5 {
		t.Fatalf("Unexpected port number %d", port)
	}
	ensureTestResults(t, fexec)

	addTestResult(t, fexec, "ovs-vsctl --timeout=30 --may-exist add-port br0 tun0 -- set Interface tun0 type=internal", "", nil)
	addTestResult(t, fexec, "ovs-vsctl --timeout=30 get Interface tun0 ofport", "1\n", nil)
	port, err = ovsif.AddPort("tun0", -1, "type=internal")
	if err != nil {
		t.Fatalf("Unexpected error from command: %v", err)
	}
	if port != 1 {
		t.Fatalf("Unexpected port number %d", port)
	}
	ensureTestResults(t, fexec)

	addTestResult(t, fexec, "ovs-vsctl --timeout=30 --may-exist add-port br0 tun0 -- set Interface tun0 ofport_request=5 type=internal", "", nil)
	addTestResult(t, fexec, "ovs-vsctl --timeout=30 get Interface tun0 ofport", "5\n", nil)
	port, err = ovsif.AddPort("tun0", 5, "type=internal")
	if err != nil {
		t.Fatalf("Unexpected error from command: %v", err)
	}
	if port != 5 {
		t.Fatalf("Unexpected port number %d", port)
	}
	ensureTestResults(t, fexec)

	addTestResult(t, fexec, "ovs-vsctl --timeout=30 --may-exist add-port br0 veth0 -- set Interface veth0 ofport_request=5", "", nil)
	addTestResult(t, fexec, "ovs-vsctl --timeout=30 get Interface veth0 ofport", "3\n", nil)
	_, err = ovsif.AddPort("veth0", 5)
	if err == nil {
		t.Fatalf("Unexpectedly failed to get error")
	}
	if err.Error() != "allocated ofport (3) did not match request (5)" {
		t.Fatalf("Got wrong error: %v", err)
	}
	ensureTestResults(t, fexec)

	addTestResult(t, fexec, "ovs-vsctl --timeout=30 --may-exist add-port br0 veth0 -- set Interface veth0 ofport_request=5", "", nil)
	addTestResult(t, fexec, "ovs-vsctl --timeout=30 get Interface veth0 ofport", "-1\n", nil)
	addTestResult(t, fexec, "ovs-vsctl --timeout=30 get Interface veth0 error", "could not open network device veth0 (No such device)\n", nil)
	_, err = ovsif.AddPort("veth0", 5)
	if err == nil {
		t.Fatalf("Unexpectedly failed to get error")
	}
	if err.Error() != "error on port veth0: could not open network device veth0 (No such device)" {
		t.Fatalf("Got wrong error: %v", err)
	}
	ensureTestResults(t, fexec)
}

func TestOVSVersion(t *testing.T) {
	fexec := normalSetup()
	defer ensureTestResults(t, fexec)

	addTestResult(t, fexec, "ovs-vsctl --timeout=30 --version", "2.5.0", nil)
	if _, err := New(fexec, "br0", "2.5.0"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	addTestResult(t, fexec, "ovs-vsctl --timeout=30 --version", "2.4.0", nil)
	if _, err := New(fexec, "br0", "2.5.0"); err == nil {
		t.Fatalf("Unexpectedly did not get error")
	}

	addTestResult(t, fexec, "ovs-vsctl --timeout=30 --version", "3.2.0", nil)
	if _, err := New(fexec, "br0", "2.5.0"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}
