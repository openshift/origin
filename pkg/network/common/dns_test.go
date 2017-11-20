package common

import (
	"fmt"
	"net"
	"strings"
	"testing"

	kexec "k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"
)

func addTestResult(t *testing.T, fexec *fakeexec.FakeExec, command string, output string, err error) {
	fcmd := fakeexec.FakeCmd{
		CombinedOutputScript: []fakeexec.FakeCombinedOutputAction{
			func() ([]byte, error) { return []byte(output), err },
		},
	}
	fexec.CommandScript = append(fexec.CommandScript,
		func(cmd string, args ...string) kexec.Cmd {
			execCommand := strings.Join(append([]string{cmd}, args...), " ")
			if execCommand != command {
				t.Fatalf("Unexpected command: wanted %q got %q", command, execCommand)
			}
			return fakeexec.InitFakeCmd(&fcmd, cmd, args...)
		})
}

func ensureTestResults(t *testing.T, fexec *fakeexec.FakeExec) {
	if fexec.CommandCalls != len(fexec.CommandScript) {
		t.Fatalf("Only used %d of %d expected commands", fexec.CommandCalls, len(fexec.CommandScript))
	}
}

func TestAddDNS(t *testing.T) {
	type dnsTest struct {
		testCase          string
		domainName        string
		dnsResolverOutput string
		ips               []net.IP
		ttl               float64
		expectFailure     bool
	}

	ip := net.ParseIP("10.11.12.13")
	tests := []dnsTest{
		{
			testCase:   "Test valid domain name",
			domainName: "example.com",
			dnsResolverOutput: "example.com.		600	IN	A	10.11.12.13",
			ips:           []net.IP{ip},
			ttl:           600,
			expectFailure: false,
		},
		{
			testCase:          "Test invalid domain name",
			domainName:        "sads@#$.com",
			dnsResolverOutput: "",
			expectFailure:     true,
		},
	}

	for _, test := range tests {
		fexec := &fakeexec.FakeExec{}
		dns := NewDNS(fexec)
		addTestResult(t, fexec, fmt.Sprintf("dig +nocmd +noall +answer +ttlid a %s", test.domainName), test.dnsResolverOutput, nil)

		err := dns.Add(test.domainName)
		if test.expectFailure && err == nil {
			t.Fatalf("Test case: %s failed, expected failure but got success", test.testCase)
		} else if !test.expectFailure && err != nil {
			t.Fatalf("Test case: %s failed, err: %v", test.testCase, err)
		}
		ensureTestResults(t, fexec)

		if test.expectFailure {
			if _, ok := dns.dnsMap[test.domainName]; ok {
				t.Fatalf("Test case: %s failed, unexpected domain %q found in dns map", test.testCase, test.domainName)
			}
		} else {
			d, ok := dns.dnsMap[test.domainName]
			if !ok {
				t.Fatalf("Test case: %s failed, domain %q not found in dns map", test.testCase, test.domainName)
			}
			if !ipsEqual(d.ips, test.ips) {
				t.Fatalf("Test case: %s failed, expected IPs: %v, got: %v for the domain %q", test.testCase, test.ips, d.ips, test.domainName)
			}
			if d.ttl.Seconds() != test.ttl {
				t.Fatalf("Test case: %s failed, expected TTL: %g, got: %g for the domain %q", test.testCase, test.ttl, d.ttl.Seconds(), test.domainName)
			}
			if d.nextQueryTime.IsZero() {
				t.Fatalf("Test case: %s failed, nextQueryTime for the domain %q is not set", test.testCase, test.domainName)
			}
		}
	}
}

func TestUpdateDNS(t *testing.T) {
	type dnsTest struct {
		testCase   string
		domainName string

		addResolverOutput string
		addIPs            []net.IP
		addTTL            float64

		updateResolverOutput string
		updateIPs            []net.IP
		updateTTL            float64

		expectFailure bool
	}

	addIP := net.ParseIP("10.11.12.13")
	updateIP := net.ParseIP("10.11.12.14")
	tests := []dnsTest{
		{
			testCase:   "Test dns update of valid domain",
			domainName: "example.com",
			addResolverOutput: "example.com.		600	IN	A	10.11.12.13",
			addIPs: []net.IP{addIP},
			addTTL: 600,
			updateResolverOutput: "example.com.		500	IN	A	10.11.12.14",
			updateIPs:     []net.IP{updateIP},
			updateTTL:     500,
			expectFailure: false,
		},
		{
			testCase:             "Test dns update of invalid domain",
			domainName:           "sads@#$.com",
			addResolverOutput:    "",
			updateResolverOutput: "",
			expectFailure:        true,
		},
	}

	for _, test := range tests {
		fexec := &fakeexec.FakeExec{}
		dns := NewDNS(fexec)
		addTestResult(t, fexec, fmt.Sprintf("dig +nocmd +noall +answer +ttlid a %s", test.domainName), test.addResolverOutput, nil)

		dns.Add(test.domainName)
		ensureTestResults(t, fexec)

		orig := dns.Get(test.domainName)
		addTestResult(t, fexec, fmt.Sprintf("dig +nocmd +noall +answer +ttlid a %s", test.domainName), test.updateResolverOutput, nil)

		err, _ := dns.updateOne(test.domainName)
		if test.expectFailure && err == nil {
			t.Fatalf("Test case: %s failed, expected failure but got success", test.testCase)
		} else if !test.expectFailure && err != nil {
			t.Fatalf("Test case: %s failed, err: %v", test.testCase, err)
		}

		ensureTestResults(t, fexec)
		updated := dns.Get(test.domainName)
		sz := dns.Size()

		if !test.expectFailure && sz != 1 {
			t.Fatalf("Test case: %s failed, expected dns map size: 1, got %d", test.testCase, sz)
		}
		if test.expectFailure && sz != 0 {
			t.Fatalf("Test case: %s failed, expected dns map size: 0, got %d", test.testCase, sz)
		}

		if !test.expectFailure {
			if !ipsEqual(orig.ips, test.addIPs) {
				t.Fatalf("Test case: %s failed, expected ips after add op: %v, got: %v", test.testCase, test.addIPs, orig.ips)
			}
			if orig.ttl.Seconds() != test.addTTL {
				t.Fatalf("Test case: %s failed, expected ttl after add op: %g, got: %g", test.testCase, test.addTTL, orig.ttl.Seconds())
			}
			if orig.nextQueryTime.IsZero() {
				t.Fatalf("Test case: %s failed, expected nextQueryTime to be set after add op", test.testCase)
			}

			if !ipsEqual(updated.ips, test.updateIPs) {
				t.Fatalf("Test case: %s failed, expected ips after update op: %v, got: %v", test.testCase, test.updateIPs, updated.ips)
			}
			if updated.ttl.Seconds() != test.updateTTL {
				t.Fatalf("Test case: %s failed, expected ttl after update op: %g, got: %g", test.testCase, test.updateTTL, updated.ttl.Seconds())
			}
			if updated.nextQueryTime.IsZero() {
				t.Fatalf("Test case: %s failed, expected nextQueryTime to be set after update op", test.testCase)
			}

			if orig.nextQueryTime == updated.nextQueryTime {
				t.Fatalf("Test case: %s failed, expected nextQueryTime to change, original nextQueryTime: %v, updated nextQueryTime: %v", test.testCase, orig.nextQueryTime, updated.nextQueryTime)
			}
		}
	}
}
