package ipcmd

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/util/exec"
)

func normalSetup() *exec.FakeExec {
	return &exec.FakeExec{
		LookPathFunc: func(prog string) (string, error) {
			if prog == "ip" {
				return "/sbin/ip", nil
			} else {
				return "", fmt.Errorf("%s not found", prog)
			}
		},
	}
}

func missingSetup() *exec.FakeExec {
	return &exec.FakeExec{
		LookPathFunc: func(prog string) (string, error) {
			return "", fmt.Errorf("%s not found", prog)
		},
	}
}

func addTestResult(t *testing.T, fexec *exec.FakeExec, command string, output string, err error) {
	fcmd := exec.FakeCmd{
		CombinedOutputScript: []exec.FakeCombinedOutputAction{
			func() ([]byte, error) { return []byte(output), err },
		},
	}
	fexec.CommandScript = append(fexec.CommandScript,
		func(cmd string, args ...string) exec.Cmd {
			execCommand := strings.Join(append([]string{cmd}, args...), " ")
			if execCommand != command {
				t.Fatalf("Unexpected command: wanted %q got %q", command, execCommand)
			}
			return exec.InitFakeCmd(&fcmd, cmd, args...)
		})
}

func ensureTestResults(t *testing.T, fexec *exec.FakeExec) {
	if fexec.CommandCalls != len(fexec.CommandScript) {
		t.Fatalf("Only used %d of %d expected commands", fexec.CommandCalls, len(fexec.CommandScript))
	}
}

func TestGetAddresses(t *testing.T) {
	fexec := normalSetup()
	addTestResult(t, fexec, "/sbin/ip addr show dev lo", `1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default 
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host 
       valid_lft forever preferred_lft forever
`, nil)
	itx := NewTransaction(fexec, "lo")
	addrs, err := itx.GetAddresses()
	if err != nil {
		t.Fatalf("Failed to get addresses for 'lo': %v", err)
	}
	if len(addrs) != 1 {
		t.Fatalf("'lo' has unexpected len(addrs) %d", len(addrs))
	}
	if addrs[0] != "127.0.0.1/8" {
		t.Fatalf("'lo' has unexpected address %s", addrs[0])
	}
	err = itx.EndTransaction()
	if err != nil {
		t.Fatalf("Transaction unexpectedly returned error: %v", err)
	}
	ensureTestResults(t, fexec)

	addTestResult(t, fexec, "/sbin/ip addr show dev eth0", `2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether aa:bb:cc:dd:ee:ff brd ff:ff:ff:ff:ff:ff
    inet 192.168.1.10/24 brd 192.168.1.255 scope global dynamic eth0
       valid_lft 81296sec preferred_lft 81296sec
    inet 192.168.1.152/24 brd 192.168.1.255 scope global dynamic eth0
       valid_lft 81296sec preferred_lft 81296sec
`, nil)
	itx = NewTransaction(fexec, "eth0")
	addrs, err = itx.GetAddresses()
	if err != nil {
		t.Fatalf("Failed to get addresses for 'eth0': %v", err)
	}
	if len(addrs) != 2 {
		t.Fatalf("'eth0' has unexpected len(addrs) %d", len(addrs))
	}
	if addrs[0] != "192.168.1.10/24" || addrs[1] != "192.168.1.152/24" {
		t.Fatalf("'eth0' has unexpected addresses %v", addrs)
	}
	err = itx.EndTransaction()
	if err != nil {
		t.Fatalf("Transaction unexpectedly returned error: %v", err)
	}
	ensureTestResults(t, fexec)

	addTestResult(t, fexec, "/sbin/ip addr show dev wlan0", "", fmt.Errorf("Device \"%s\" does not exist", "wlan0"))
	itx = NewTransaction(fexec, "wlan0")
	addrs, err = itx.GetAddresses()
	if err == nil {
		t.Fatalf("Allegedly got addresses for non-existent link: %v", addrs)
	}
	err = itx.EndTransaction()
	if err == nil {
		t.Fatalf("Transaction unexpectedly returned no error")
	}
	ensureTestResults(t, fexec)
}

func TestGetRoutes(t *testing.T) {
	const (
		l1 = "default via 192.168.1.1  proto static  metric 1024 "
		l2 = "1.2.3.4 via 192.168.1.1  proto static  metric 10 "
		l3 = "192.168.1.0/24  proto kernel  scope link  src 192.168.1.15 "
	)
	fexec := normalSetup()
	addTestResult(t, fexec, "/sbin/ip route show dev wlp3s0", l1+"\n"+l2+"\n"+l3+"\n", nil)
	itx := NewTransaction(fexec, "wlp3s0")
	routes, err := itx.GetRoutes()
	if err != nil {
		t.Fatalf("Failed to get routes for 'wlp3s0': %v", err)
	}
	if len(routes) != 3 {
		t.Fatalf("'wlp3s0' has unexpected len(routes) %d", len(routes))
	}
	if routes[0] != l1 {
		t.Fatalf("Unexpected first route %s", routes[0])
	}
	if routes[1] != l2 {
		t.Fatalf("Unexpected second route %s", routes[1])
	}
	if routes[2] != l3 {
		t.Fatalf("Unexpected third route %s", routes[2])
	}
	err = itx.EndTransaction()
	if err != nil {
		t.Fatalf("Transaction unexpectedly returned error: %v", err)
	}
	ensureTestResults(t, fexec)

	addTestResult(t, fexec, "/sbin/ip route show dev wlan0", "", fmt.Errorf("Device \"%s\" does not exist", "wlan0"))
	itx = NewTransaction(fexec, "wlan0")
	routes, err = itx.GetRoutes()
	if err == nil {
		t.Fatalf("Allegedly got routes for non-existent link: %v", routes)
	}
	err = itx.EndTransaction()
	if err == nil {
		t.Fatalf("Transaction unexpectedly returned no error")
	}
	ensureTestResults(t, fexec)
}

func TestErrorHandling(t *testing.T) {
	fexec := normalSetup()
	addTestResult(t, fexec, "/sbin/ip link del dummy0", "", fmt.Errorf("Device \"%s\" does not exist", "dummy0"))
	itx := NewTransaction(fexec, "dummy0")
	itx.DeleteLink()
	err := itx.EndTransaction()
	if err == nil {
		t.Fatalf("Failed to get expected error")
	}
	ensureTestResults(t, fexec)

	addTestResult(t, fexec, "/sbin/ip link del dummy0", "", fmt.Errorf("Device \"%s\" does not exist", "dummy0"))
	addTestResult(t, fexec, "/sbin/ip link add dummy0 type dummy", "", nil)
	itx = NewTransaction(fexec, "dummy0")
	itx.DeleteLink()
	itx.IgnoreError()
	itx.AddLink("type", "dummy")
	err = itx.EndTransaction()
	if err != nil {
		t.Fatalf("Unexpectedly got error after IgnoreError(): %v", err)
	}
	ensureTestResults(t, fexec)

	addTestResult(t, fexec, "/sbin/ip link add dummy0 type dummy", "", fmt.Errorf("RTNETLINK answers: Operation not permitted"))
	// other commands do not get run due to previous error
	itx = NewTransaction(fexec, "dummy0")
	itx.AddLink("type", "dummy")
	itx.SetLink("up")
	itx.DeleteLink()
	err = itx.EndTransaction()
	if err == nil {
		t.Fatalf("Failed to get expected error")
	}
	ensureTestResults(t, fexec)
}

func TestIPMissing(t *testing.T) {
	fexec := missingSetup()
	itx := NewTransaction(fexec, "dummy0")
	itx.AddLink("type", "dummy")
	err := itx.EndTransaction()
	if err == nil {
		t.Fatalf("Unexpectedly did not get error")
	}
	if err.Error() != "ip is not installed" {
		t.Fatalf("Got wrong error: %v", err)
	}
}
