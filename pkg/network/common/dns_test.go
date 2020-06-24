package common

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestAddDNS(t *testing.T) {
	s, addr, err := runLocalUDPServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("unable to run test server: %v", err)
	}
	defer s.Shutdown()

	configFileName, err := createResolveConfFile(addr)
	if err != nil {
		t.Fatalf("unable to create test resolver: %v", err)
	}
	defer os.Remove(configFileName)

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
			testCase:          "Test valid domain name with resolver returning only A record",
			domainName:        "example.com",
			dnsResolverOutput: "example.com. 600 IN A 10.11.12.13",
			ips:               []net.IP{ip},
			ttl:               600,
			expectFailure:     false,
		},
		{
			testCase:          "Test valid domain name with resolver returning both CNAME and A records",
			domainName:        "example.com",
			dnsResolverOutput: "example.com. 200 IN CNAME foo.example.com.\nfoo.example.com. 600 IN A 10.11.12.13",
			ips:               []net.IP{ip},
			ttl:               200,
			expectFailure:     false,
		},
		{
			testCase:          "Test invalid domain name",
			domainName:        "sads@#$.com",
			dnsResolverOutput: "",
			expectFailure:     true,
		},
		{
			testCase:          "Test min TTL",
			domainName:        "example.com",
			dnsResolverOutput: "example.com. 0 IN A 10.11.12.13",
			ips:               []net.IP{ip},
			ttl:               1800,
			expectFailure:     false,
		},
	}

	for _, test := range tests {
		serverFn := dummyServer(test.dnsResolverOutput)
		dns.HandleFunc(test.domainName, serverFn)
		defer dns.HandleRemove(test.domainName)

		n, err := NewDNS(configFileName)
		if err != nil {
			t.Fatalf("Test case: %s failed, err: %v", test.testCase, err)
		}

		err = n.Add(test.domainName)
		if test.expectFailure && err == nil {
			t.Fatalf("Test case: %s failed, expected failure but got success", test.testCase)
		} else if !test.expectFailure && err != nil {
			t.Fatalf("Test case: %s failed, err: %v", test.testCase, err)
		}

		if test.expectFailure {
			if _, ok := n.dnsMap[test.domainName]; ok {
				t.Fatalf("Test case: %s failed, unexpected domain %q found in dns map", test.testCase, test.domainName)
			}
		} else {
			d, ok := n.dnsMap[test.domainName]
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
	s, addr, err := runLocalUDPServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("unable to run test server: %v", err)
	}
	defer s.Shutdown()

	configFileName, err := createResolveConfFile(addr)
	if err != nil {
		t.Fatalf("unable to create test resolver: %v", err)
	}
	defer os.Remove(configFileName)

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
			testCase:             "Test dns update of valid domain",
			domainName:           "example.com",
			addResolverOutput:    "example.com. 600 IN A 10.11.12.13",
			addIPs:               []net.IP{addIP},
			addTTL:               600,
			updateResolverOutput: "example.com. 500 IN A 10.11.12.14",
			updateIPs:            []net.IP{updateIP},
			updateTTL:            500,
			expectFailure:        false,
		},
		{
			testCase:             "Test dns update of invalid domain",
			domainName:           "sads@#$.com",
			addResolverOutput:    "",
			updateResolverOutput: "",
			expectFailure:        true,
		},
		{
			testCase:             "Test dns update min TTL",
			domainName:           "example.com",
			addResolverOutput:    "example.com. 5 IN A 10.11.12.13",
			addIPs:               []net.IP{addIP},
			addTTL:               5,
			updateResolverOutput: "example.com. 0 IN A 10.11.12.14",
			updateIPs:            []net.IP{updateIP},
			updateTTL:            1800,
			expectFailure:        false,
		},
	}

	for _, test := range tests {
		serverFn := dummyServer(test.addResolverOutput)
		dns.HandleFunc(test.domainName, serverFn)
		defer dns.HandleRemove(test.domainName)

		n, err := NewDNS(configFileName)
		if err != nil {
			t.Fatalf("Test case: %s failed, err: %v", test.testCase, err)
		}

		n.Add(test.domainName)

		orig := n.Get(test.domainName)

		dns.HandleRemove(test.domainName)
		serverFn = dummyServer(test.updateResolverOutput)
		dns.HandleFunc(test.domainName, serverFn)
		defer dns.HandleRemove(test.domainName)

		err, _ = n.updateOne(test.domainName)
		if test.expectFailure && err == nil {
			t.Fatalf("Test case: %s failed, expected failure but got success", test.testCase)
		} else if !test.expectFailure && err != nil {
			t.Fatalf("Test case: %s failed, err: %v", test.testCase, err)
		}

		updated := n.Get(test.domainName)
		sz := n.Size()

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

func dummyServer(output string) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, req *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(req)

		answers := strings.Split(output, "\n")
		m.Answer = make([]dns.RR, len(answers))
		for i, ans := range answers {
			mx, _ := dns.NewRR(ans)
			m.Answer[i] = mx
		}
		w.WriteMsg(m)
	}
}

func runLocalUDPServer(addr string) (*dns.Server, string, error) {
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, "", err
	}
	server := &dns.Server{PacketConn: pc, ReadTimeout: time.Hour, WriteTimeout: time.Hour}

	waitLock := sync.Mutex{}
	waitLock.Lock()
	server.NotifyStartedFunc = waitLock.Unlock

	// fin must be buffered so the goroutine below won't block
	// forever if fin is never read from.
	fin := make(chan error, 1)

	go func() {
		fin <- server.ActivateAndServe()
		pc.Close()
	}()

	waitLock.Lock()
	return server, pc.LocalAddr().String(), nil
}

func createResolveConfFile(addr string) (string, error) {
	configFile, err := ioutil.TempFile("/tmp/", "resolv")
	if err != nil {
		return "", fmt.Errorf("cannot create DNS resolver config file: %v", err)
	}

	data := fmt.Sprintf(`
nameserver %s
#nameserver 192.168.10.11

options rotate timeout:1 attempts:1`, addr)

	if _, err := configFile.WriteString(data); err != nil {
		return "", fmt.Errorf("unable to write data to resolver config file: %v", err)
	}

	return configFile.Name(), nil
}
