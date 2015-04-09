// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

// etcd needs to be running on http://127.0.0.1:4001

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/miekg/dns"
	backendetcd "github.com/skynetservices/skydns/backends/etcd"
	"github.com/skynetservices/skydns/cache"
	"github.com/skynetservices/skydns/msg"
)

// Keep global port counter that increments with 10 for each
// new call to newTestServer. The dns server is started on port 'Port'.
var Port = 9400
var StrPort = "9400" // string equivalent of Port

func addService(t *testing.T, s *server, k string, ttl uint64, m *msg.Service) {
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := msg.PathWithWildcard(k)
	t.Logf("Adding path %s:", path)
	_, err = s.backend.(*backendetcd.Backend).Client().Create(path, string(b), ttl)
	if err != nil {
		// TODO(miek): allow for existing keys...
		t.Fatal(err)
	}
}

func delService(t *testing.T, s *server, k string) {
	path, _ := msg.PathWithWildcard(k)
	_, err := s.backend.(*backendetcd.Backend).Client().Delete(path, false)
	if err != nil {
		t.Fatal(err)
	}
}

func newTestServer(t *testing.T, c bool) *server {
	Port += 10
	StrPort = strconv.Itoa(Port)
	s := new(server)
	client := etcd.NewClient([]string{"http://127.0.0.1:4001"})
	client.SyncCluster()

	// TODO(miek): why don't I use NewServer??
	s.group = new(sync.WaitGroup)
	s.scache = cache.New(100, 0)
	s.rcache = cache.New(100, 0)
	if c {
		s.rcache = cache.New(100, 60) // 100 items, 60s ttl
	}
	s.config = new(Config)
	s.config.Domain = "skydns.test."
	s.config.DnsAddr = "127.0.0.1:" + StrPort
	s.config.Nameservers = []string{"8.8.4.4:53"}
	SetDefaults(s.config)
	s.config.Local = "104.server1.development.region1.skydns.test."
	s.config.Priority = 10
	s.config.RCacheTtl = RCacheTtl
	s.config.Ttl = 3600
	s.config.Ndots = 2

	s.dnsUDPclient = &dns.Client{Net: "udp", ReadTimeout: 2 * s.config.ReadTimeout, WriteTimeout: 2 * s.config.ReadTimeout, SingleInflight: true}
	s.dnsTCPclient = &dns.Client{Net: "tcp", ReadTimeout: 2 * s.config.ReadTimeout, WriteTimeout: 2 * s.config.ReadTimeout, SingleInflight: true}

	s.backend = backendetcd.NewBackend(client, &backendetcd.Config{
		Ttl:      s.config.Ttl,
		Priority: s.config.Priority,
	})

	go s.Run()
	// Yeah, yeah, should do a proper fix.
	time.Sleep(500 * time.Millisecond)
	return s
}

func newTestServerDNSSEC(t *testing.T, cache bool) *server {
	var err error
	s := newTestServer(t, cache)
	s.config.PubKey = newDNSKEY("skydns.test. IN DNSKEY 256 3 5 AwEAAaXfO+DOBMJsQ5H4TfiabwSpqE4cGL0Qlvh5hrQumrjr9eNSdIOjIHJJKCe56qBU5mH+iBlXP29SVf6UiiMjIrAPDVhClLeWFe0PC+XlWseAyRgiLHdQ8r95+AfkhO5aZgnCwYf9FGGSaT0+CRYN+PyDbXBTLK5FN+j5b6bb7z+d")
	s.config.KeyTag = s.config.PubKey.KeyTag()
	s.config.PrivKey, err = s.config.PubKey.ReadPrivateKey(strings.NewReader(`Private-key-format: v1.3
Algorithm: 5 (RSASHA1)
Modulus: pd874M4EwmxDkfhN+JpvBKmoThwYvRCW+HmGtC6auOv141J0g6MgckkoJ7nqoFTmYf6IGVc/b1JV/pSKIyMisA8NWEKUt5YV7Q8L5eVax4DJGCIsd1Dyv3n4B+SE7lpmCcLBh/0UYZJpPT4JFg34/INtcFMsrkU36PlvptvvP50=
PublicExponent: AQAB
PrivateExponent: C6e08GXphbPPx6j36ZkIZf552gs1XcuVoB4B7hU8P/Qske2QTFOhCwbC8I+qwdtVWNtmuskbpvnVGw9a6X8lh7Z09RIgzO/pI1qau7kyZcuObDOjPw42exmjqISFPIlS1wKA8tw+yVzvZ19vwRk1q6Rne+C1romaUOTkpA6UXsE=
Prime1: 2mgJ0yr+9vz85abrWBWnB8Gfa1jOw/ccEg8ZToM9GLWI34Qoa0D8Dxm8VJjr1tixXY5zHoWEqRXciTtY3omQDQ==
Prime2: wmxLpp9rTzU4OREEVwF43b/TxSUBlUq6W83n2XP8YrCm1nS480w4HCUuXfON1ncGYHUuq+v4rF+6UVI3PZT50Q==
Exponent1: wkdTngUcIiau67YMmSFBoFOq9Lldy9HvpVzK/R0e5vDsnS8ZKTb4QJJ7BaG2ADpno7pISvkoJaRttaEWD3a8rQ==
Exponent2: YrC8OglEXIGkV3tm2494vf9ozPL6+cBkFsPPg9dXbvVCyyuW0pGHDeplvfUqs4nZp87z8PsoUL+LAUqdldnwcQ==
Coefficient: mMFr4+rDY5V24HZU3Oa5NEb55iQ56ZNa182GnNhWqX7UqWjcUUGjnkCy40BqeFAQ7lp52xKHvP5Zon56mwuQRw==
`), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDNSForward(t *testing.T) {
	s := newTestServer(t, false)
	defer s.Stop()

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("www.example.com.", dns.TypeA)
	resp, _, err := c.Exchange(m, "127.0.0.1:"+StrPort)
	if err != nil {
		// try twice
		resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(resp.Answer) == 0 || resp.Rcode != dns.RcodeSuccess {
		t.Fatal("Answer expected to have A records or rcode not equal to RcodeSuccess")
	}
	// TCP
	c.Net = "tcp"
	resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Answer) == 0 || resp.Rcode != dns.RcodeSuccess {
		t.Fatal("Answer expected to have A records or rcode not equal to RcodeSuccess")
	}
}
func TestDNSTtlRRset(t *testing.T) {
	s := newTestServerDNSSEC(t, false)
	defer s.Stop()

	ttl := uint32(60)
	for _, serv := range services {
		addService(t, s, serv.Key, uint64(ttl), serv)
		defer delService(t, s, serv.Key)
		ttl += 60
	}
	c := new(dns.Client)
	tc := dnsTestCases[9]
	t.Logf("%v\n", tc)
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)
	if tc.dnssec == true {
		m.SetEdns0(4096, true)
	}
	resp, _, err := c.Exchange(m, "127.0.0.1:"+StrPort)
	if err != nil {
		t.Fatalf("failing: %s: %s\n", m.String(), err.Error())
	}
	t.Logf("%s\n", resp)
	ttl = 360
	for i, a := range resp.Answer {
		if a.Header().Ttl != ttl {
			t.Errorf("Answer %d should have a Header TTL of %d, but has %d", i, ttl, a.Header().Ttl)
		}
	}
}

type rrSet []dns.RR

func (p rrSet) Len() int           { return len(p) }
func (p rrSet) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p rrSet) Less(i, j int) bool { return p[i].String() < p[j].String() }

func TestDNS(t *testing.T) {
	s := newTestServerDNSSEC(t, false)
	defer s.Stop()

	for _, serv := range services {
		addService(t, s, serv.Key, 0, serv)
		defer delService(t, s, serv.Key)
	}
	c := new(dns.Client)
	for _, tc := range dnsTestCases {
		m := new(dns.Msg)
		m.SetQuestion(tc.Qname, tc.Qtype)
		if tc.dnssec {
			m.SetEdns0(4096, true)
		}
		if tc.chaos {
			m.Question[0].Qclass = dns.ClassCHAOS
		}
		resp, _, err := c.Exchange(m, "127.0.0.1:"+StrPort)
		t.Logf("question: %s\n", m.Question[0].String())
		if err != nil {
			// try twice, be more resilent against remote lookups
			// timing out.
			resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
			if err != nil {
				t.Fatalf("failing: %s: %s\n", m.String(), err.Error())
			}
		}
		sort.Sort(rrSet(resp.Answer))
		sort.Sort(rrSet(resp.Ns))
		sort.Sort(rrSet(resp.Extra))
		t.Logf("%s\n", resp)
		if resp.Rcode != tc.Rcode {
			t.Fatalf("rcode is %q, expected %q", dns.RcodeToString[resp.Rcode], dns.RcodeToString[tc.Rcode])
		}
		if len(resp.Answer) != len(tc.Answer) {
			t.Fatalf("answer for %q contained %d results, %d expected", tc.Qname, len(resp.Answer), len(tc.Answer))
		}
		for i, a := range resp.Answer {
			if a.Header().Name != tc.Answer[i].Header().Name {
				t.Fatalf("answer %d should have a Header Name of %q, but has %q", i, tc.Answer[i].Header().Name, a.Header().Name)
			}
			if a.Header().Ttl != tc.Answer[i].Header().Ttl {
				t.Fatalf("Answer %d should have a Header TTL of %d, but has %d", i, tc.Answer[i].Header().Ttl, a.Header().Ttl)
			}
			if a.Header().Rrtype != tc.Answer[i].Header().Rrtype {
				t.Fatalf("answer %d should have a header response type of %d, but has %d", i, tc.Answer[i].Header().Rrtype, a.Header().Rrtype)
			}
			switch x := a.(type) {
			case *dns.SRV:
				if x.Priority != tc.Answer[i].(*dns.SRV).Priority {
					t.Fatalf("answer %d should have a Priority of %d, but has %d", i, tc.Answer[i].(*dns.SRV).Priority, x.Priority)
				}
				if x.Weight != tc.Answer[i].(*dns.SRV).Weight {
					t.Fatalf("answer %d should have a Weight of %d, but has %d", i, tc.Answer[i].(*dns.SRV).Weight, x.Weight)
				}
				if x.Port != tc.Answer[i].(*dns.SRV).Port {
					t.Fatalf("answer %d should have a Port of %d, but has %d", i, tc.Answer[i].(*dns.SRV).Port, x.Port)
				}
				if x.Target != tc.Answer[i].(*dns.SRV).Target {
					t.Fatalf("answer %d should have a Target of %q, but has %q", i, tc.Answer[i].(*dns.SRV).Target, x.Target)
				}
			case *dns.A:
				if x.A.String() != tc.Answer[i].(*dns.A).A.String() {
					t.Fatalf("answer %d should have a Address of %q, but has %q", i, tc.Answer[i].(*dns.A).A.String(), x.A.String())
				}
			case *dns.AAAA:
				if x.AAAA.String() != tc.Answer[i].(*dns.AAAA).AAAA.String() {
					t.Fatalf("answer %d should have a Address of %q, but has %q", i, tc.Answer[i].(*dns.AAAA).AAAA.String(), x.AAAA.String())
				}
			case *dns.TXT:
				for j, txt := range x.Txt {
					if txt != tc.Answer[i].(*dns.TXT).Txt[j] {
						t.Fatalf("answer %d should have a Txt of %q, but has %q", i, tc.Answer[i].(*dns.TXT).Txt[j], txt)
					}
				}
			case *dns.DNSKEY:
				tt := tc.Answer[i].(*dns.DNSKEY)
				if x.Flags != tt.Flags {
					t.Fatalf("DNSKEY flags should be %q, but is %q", x.Flags, tt.Flags)
				}
				if x.Protocol != tt.Protocol {
					t.Fatalf("DNSKEY protocol should be %q, but is %q", x.Protocol, tt.Protocol)
				}
				if x.Algorithm != tt.Algorithm {
					t.Fatalf("DNSKEY algorithm should be %q, but is %q", x.Algorithm, tt.Algorithm)
				}
			case *dns.RRSIG:
				tt := tc.Answer[i].(*dns.RRSIG)
				if x.TypeCovered != tt.TypeCovered {
					t.Fatalf("RRSIG type-covered should be %d, but is %d", x.TypeCovered, tt.TypeCovered)
				}
				if x.Algorithm != tt.Algorithm {
					t.Fatalf("RRSIG algorithm should be %d, but is %d", x.Algorithm, tt.Algorithm)
				}
				if x.Labels != tt.Labels {
					t.Fatalf("RRSIG label should be %d, but is %d", x.Labels, tt.Labels)
				}
				if x.OrigTtl != tt.OrigTtl {
					t.Fatalf("RRSIG orig-ttl should be %d, but is %d", x.OrigTtl, tt.OrigTtl)
				}
				if x.KeyTag != tt.KeyTag {
					t.Fatalf("RRSIG key-tag should be %d, but is %d", x.KeyTag, tt.KeyTag)
				}
				if x.SignerName != tt.SignerName {
					t.Fatalf("RRSIG signer-name should be %q, but is %q", x.SignerName, tt.SignerName)
				}
			case *dns.SOA:
				tt := tc.Answer[i].(*dns.SOA)
				if x.Ns != tt.Ns {
					t.Fatalf("SOA nameserver should be %q, but is %q", x.Ns, tt.Ns)
				}
			case *dns.PTR:
				tt := tc.Answer[i].(*dns.PTR)
				if x.Ptr != tt.Ptr {
					t.Fatalf("PTR ptr should be %q, but is %q", x.Ptr, tt.Ptr)
				}
			case *dns.CNAME:
				tt := tc.Answer[i].(*dns.CNAME)
				if x.Target != tt.Target {
					t.Fatalf("CNAME target should be %q, but is %q", x.Target, tt.Target)
				}
			}
		}
		if len(resp.Ns) != len(tc.Ns) {
			t.Fatalf("authority for %q contained %d results, %d expected", tc.Qname, len(resp.Ns), len(tc.Ns))
		}
		for i, n := range resp.Ns {
			switch x := n.(type) {
			case *dns.SOA:
				tt := tc.Ns[i].(*dns.SOA)
				if x.Ns != tt.Ns {
					t.Fatalf("SOA nameserver should be %q, but is %q", x.Ns, tt.Ns)
				}
			case *dns.NS:
				tt := tc.Ns[i].(*dns.NS)
				if x.Ns != tt.Ns {
					t.Fatalf("NS nameserver should be %q, but is %q", x.Ns, tt.Ns)
				}
			case *dns.NSEC3:
				tt := tc.Ns[i].(*dns.NSEC3)
				if x.NextDomain != tt.NextDomain {
					t.Fatalf("NSEC3 nextdomain should be %q, but is %q", x.NextDomain, tt.NextDomain)
				}
				if x.Hdr.Name != tt.Hdr.Name {
					t.Fatalf("NSEC3 ownername should be %q, but is %q", x.Hdr.Name, tt.Hdr.Name)
				}
				for j, y := range x.TypeBitMap {
					if y != tt.TypeBitMap[j] {
						t.Fatalf("NSEC3 bitmap should have %q, but is %q", dns.TypeToString[y], dns.TypeToString[tt.TypeBitMap[j]])
					}
				}
			}
		}
		if len(resp.Extra) != len(tc.Extra) {
			t.Fatalf("additional for %q contained %d results, %d expected", tc.Qname, len(resp.Extra), len(tc.Extra))
		}
		for i, e := range resp.Extra {
			switch x := e.(type) {
			case *dns.A:
				if x.A.String() != tc.Extra[i].(*dns.A).A.String() {
					t.Fatalf("extra %d should have a address of %q, but has %q", i, tc.Extra[i].(*dns.A).A.String(), x.A.String())
				}
			case *dns.AAAA:
				if x.AAAA.String() != tc.Extra[i].(*dns.AAAA).AAAA.String() {
					t.Fatalf("extra %d should have a address of %q, but has %q", i, tc.Extra[i].(*dns.AAAA).AAAA.String(), x.AAAA.String())
				}
			case *dns.CNAME:
				tt := tc.Extra[i].(*dns.CNAME)
				if x.Target != tt.Target {
					t.Fatalf("CNAME target should be %q, but is %q", x.Target, tt.Target)
				}
			}
		}
	}
}

type dnsTestCase struct {
	Qname  string
	Qtype  uint16
	dnssec bool
	chaos  bool
	Rcode  int
	Answer []dns.RR
	Ns     []dns.RR
	Extra  []dns.RR
}

var services = []*msg.Service{
	{Host: "server1", Port: 8080, Key: "100.server1.development.region1.skydns.test."},
	{Host: "server2", Port: 80, Key: "101.server2.production.region1.skydns.test."},
	{Host: "server4", Port: 80, Priority: 333, Key: "102.server4.development.region6.skydns.test."},
	{Host: "server3", Key: "103.server4.development.region2.skydns.test."},
	{Host: "172.16.1.1", Key: "a.ipaddr.skydns.test."},
	{Host: "172.16.1.2", Key: "b.ipaddr.skydns.test."},
	{Host: "ipaddr.skydns.test", Key: "1.backend.in.skydns.test."},
	{Host: "10.0.0.1", Key: "104.server1.development.region1.skydns.test."},
	{Host: "2001::8:8:8:8", Key: "105.server3.production.region2.skydns.test."},
	{Host: "104.server1.development.region1.skydns.test", Key: "1.cname.skydns.test."},
	{Host: "100.server1.development.region1.skydns.test", Key: "2.cname.skydns.test."},
	{Host: "www.miek.nl", Key: "external1.cname.skydns.test."},
	{Host: "www.miek.nl", Key: "ext1.cname2.skydns.test."},
	{Host: "www.miek.nl", Key: "ext2.cname2.skydns.test."},
	{Host: "wwwwwww.miek.nl", Key: "external2.cname.skydns.test."},
	{Host: "4.cname.skydns.test", Key: "3.cname.skydns.test."},
	{Host: "3.cname.skydns.test", Key: "4.cname.skydns.test."},
	{Host: "10.0.0.2", Key: "ttl.skydns.test.", Ttl: 360},
	{Host: "reverse.example.com", Key: "1.0.0.10.in-addr.arpa."}, // 10.0.0.1
	{Host: "server1", Weight: 130, Key: "100.server1.region5.skydns.test."},
	{Host: "server2", Weight: 80, Key: "101.server2.region5.skydns.test."},
	{Host: "server3", Weight: 150, Key: "103.server3.region5.skydns.test."},
	{Host: "server4", Priority: 30, Key: "104.server4.region5.skydns.test."},
	// nameserver
	{Host: "10.0.0.2", Key: "ns.dns.skydns.test."},
	{Host: "10.0.0.3", Key: "ns2.dns.skydns.test."},
	// txt
	{Text: "abc", Key: "a1.txt.skydns.test."},
	{Text: "abc abc", Key: "a2.txt.skydns.test."},
}

var dnsTestCases = []dnsTestCase{
	// Full Name Test
	{
		Qname: "100.server1.development.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("100.server1.development.region1.skydns.test. 3600 SRV 10 100 8080 server1.")},
	},
	// SOA Record Test
	{
		Qname: "skydns.test.", Qtype: dns.TypeSOA,
		Answer: []dns.RR{newSOA("skydns.test. 3600 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
	},
	// NS Record Test
	{
		Qname: "skydns.test.", Qtype: dns.TypeNS,
		Answer: []dns.RR{
			newNS("skydns.test. 3600 NS ns1.dns.skydns.test."),
			newNS("skydns.test. 3600 NS ns2.dns.skydns.test."),
		},
		Extra: []dns.RR{
			newA("ns.dns.skydns.test. 3600 A 10.0.0.2"),
			newA("ns2.dns.skydns.test. 3600 A 10.0.0.3"),
		},
	},
	// A Record For NS Record Test
	{
		Qname: "ns.dns.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{newA("ns.dns.skydns.test. 3600 A 10.0.0.2")},
	},
	// A Record Test
	{
		Qname: "104.server1.development.region1.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{newA("104.server1.development.region1.skydns.test. 3600 A 10.0.0.1")},
	},
	// Multiple A Record Test
	{
		Qname: "ipaddr.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			newA("ipaddr.skydns.test. 3600 A 172.16.1.1"),
			newA("ipaddr.skydns.test. 3600 A 172.16.1.2"),
		},
	},
	// A Record Test with SRV
	{
		Qname: "104.server1.development.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("104.server1.development.region1.skydns.test. 3600 SRV 10 100 0 104.server1.development.region1.skydns.test.")},
		Extra:  []dns.RR{newA("104.server1.development.region1.skydns.test. 3600 A 10.0.0.1")},
	},
	// AAAAA Record Test
	{
		Qname: "105.server3.production.region2.skydns.test.", Qtype: dns.TypeAAAA,
		Answer: []dns.RR{newAAAA("105.server3.production.region2.skydns.test. 3600 AAAA 2001::8:8:8:8")},
	},
	// Multi SRV with the same target, should be dedupped.
	{
		Qname: "*.cname2.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("*.cname2.skydns.test. 3600 IN SRV 10 100 0 www.miek.nl."),
		},
		Extra: []dns.RR{
			newA("a.miek.nl. 3600 IN A 176.58.119.54"),
			newAAAA("a.miek.nl. 3600 IN AAAA 2a01:7e00::f03c:91ff:feae:e74c"),
			newCNAME("www.miek.nl. 3600 IN CNAME a.miek.nl."),
		},
	},
	// TTL Test
	{
		// This test is referenced by number from DNSTtlRRset
		Qname: "ttl.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{newA("ttl.skydns.test. 360 A 10.0.0.2")},
	},
	// CNAME Test
	{
		Qname: "1.cname.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			newCNAME("1.cname.skydns.test. 3600 CNAME 104.server1.development.region1.skydns.test."),
			newA("104.server1.development.region1.skydns.test. 3600 A 10.0.0.1"),
		},
	},
	// Direct CNAME Test
	{
		Qname: "1.cname.skydns.test.", Qtype: dns.TypeCNAME,
		Answer: []dns.RR{
			newCNAME("1.cname.skydns.test. 3600 CNAME 104.server1.development.region1.skydns.test."),
		},
	},
	// CNAME (unresolvable internal name)
	{
		Qname: "2.cname.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{},
		Ns:     []dns.RR{newSOA("skydns.test. 60 SOA ns.dns.skydns.test. hostmaster.skydns.test. 1407441600 28800 7200 604800 60")},
	},
	// CNAME loop detection
	{
		Qname: "3.cname.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{},
		Ns:     []dns.RR{newSOA("skydns.test. 60 SOA ns.dns.skydns.test. hostmaster.skydns.test. 1407441600 28800 7200 604800 60")},
	},
	// CNAME (resolvable external name)
	{
		Qname: "external1.cname.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			newA("a.miek.nl. 60 IN A 176.58.119.54"),
			newCNAME("external1.cname.skydns.test. 60 IN CNAME www.miek.nl."),
			newCNAME("www.miek.nl. 60 IN CNAME a.miek.nl."),
		},
	},
	// CNAME (unresolvable external name)
	{
		Qname: "external2.cname.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{},
		Ns:     []dns.RR{newSOA("skydns.test. 60 SOA ns.dns.skydns.test. hostmaster.skydns.test. 1407441600 28800 7200 604800 60")},
	},
	// Priority Test
	{
		Qname: "region6.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("region6.skydns.test. 3600 SRV 333 100 80 server4.")},
	},
	// Subdomain Test
	{
		Qname: "region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("region1.skydns.test. 3600 SRV 10 33 0 104.server1.development.region1.skydns.test."),
			newSRV("region1.skydns.test. 3600 SRV 10 33 80 server2"),
			newSRV("region1.skydns.test. 3600 SRV 10 33 8080 server1.")},
		Extra: []dns.RR{newA("104.server1.development.region1.skydns.test. 3600 A 10.0.0.1")},
	},
	// Subdomain Weight Test
	{
		Qname: "region5.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("region5.skydns.test. 3600 SRV 10 22 0 server2."),
			newSRV("region5.skydns.test. 3600 SRV 10 36 0 server1."),
			newSRV("region5.skydns.test. 3600 SRV 10 41 0 server3."),
			newSRV("region5.skydns.test. 3600 SRV 30 100 0 server4.")},
	},
	// Wildcard Test
	{
		Qname: "*.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("*.region1.skydns.test. 3600 SRV 10 33 0 104.server1.development.region1.skydns.test."),
			newSRV("*.region1.skydns.test. 3600 SRV 10 33 80 server2"),
			newSRV("*.region1.skydns.test. 3600 SRV 10 33 8080 server1.")},
		Extra: []dns.RR{newA("104.server1.development.region1.skydns.test. 3600 A 10.0.0.1")},
	},
	// Wildcard Test
	{
		Qname: "production.*.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("production.*.skydns.test. 3600 IN SRV 10 50 0 105.server3.production.region2.skydns.test."),
			newSRV("production.*.skydns.test. 3600 IN SRV 10 50 80 server2.")},
		Extra: []dns.RR{newAAAA("105.server3.production.region2.skydns.test. 3600 IN AAAA 2001::8:8:8:8")},
	},
	// NXDOMAIN Test
	{
		Qname: "doesnotexist.skydns.test.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			newSOA("skydns.test. 3600 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0"),
		},
	},
	// NODATA Test
	{
		Qname: "104.server1.development.region1.skydns.test.", Qtype: dns.TypeTXT,
		Ns: []dns.RR{newSOA("skydns.test. 3600 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
	},
	// NODATA Test 2
	{
		Qname: "100.server1.development.region1.skydns.test.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Ns:    []dns.RR{newSOA("skydns.test. 3600 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
	},
	// CNAME Test that targets multiple A records (hits a directory in etcd)
	{
		Qname: "1.backend.in.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			newCNAME("1.backend.in.skydns.test. IN CNAME ipaddr.skydns.test."),
			newA("ipaddr.skydns.test. IN A 172.16.1.1"),
			newA("ipaddr.skydns.test. IN A 172.16.1.2"),
		},
	},
	// Query a etcd directory key
	{
		Qname: "backend.in.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			newCNAME("backend.in.skydns.test. IN CNAME ipaddr.skydns.test."),
			newA("ipaddr.skydns.test. IN A 172.16.1.1"),
			newA("ipaddr.skydns.test. IN A 172.16.1.2"),
		},
	},
	// Txt
	{
		Qname: "a1.txt.skydns.test.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			newTXT("a1.txt.skydns.test. IN TXT \"abc\""),
		},
	},
	{
		Qname: "a2.txt.skydns.test.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			newTXT("a2.txt.skydns.test. IN TXT \"abc abc\""),
		},
	},
	{
		Qname: "txt.skydns.test.", Qtype: dns.TypeTXT,
		Answer: []dns.RR{
			newTXT("txt.skydns.test. IN TXT \"abc abc\""),
			newTXT("txt.skydns.test. IN TXT \"abc\""),
		},
	},

	// DNSSEC

	// DNSKEY Test
	{
		dnssec: true,
		Qname:  "skydns.test.", Qtype: dns.TypeDNSKEY,
		Answer: []dns.RR{
			newDNSKEY("skydns.test. 3600 DNSKEY 256 3 5 deadbeaf"),
			newRRSIG("skydns.test. 3600 RRSIG DNSKEY 5 2 3600 0 0 51945 skydns.test. deadbeaf"),
		},
		Extra: []dns.RR{new(dns.OPT)},
	},
	// Signed Response Test
	{
		dnssec: true,
		Qname:  "104.server1.development.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newRRSIG("104.server1.development.region1.skydns.test. 3600 RRSIG SRV 5 6 3600 0 0 51945 skydns.test. deadbeaf"),
			newSRV("104.server1.development.region1.skydns.test. 3600 SRV 10 100 0 104.server1.development.region1.skydns.test.")},
		Extra: []dns.RR{
			newRRSIG("104.server1.developmen.region1.skydns.test. 3600 RRSIG A 5 6 3600 0 0 51945 skydns.test. deadbeaf"),
			newA("104.server1.development.region1.skydns.test. 3600 A 10.0.0.1"),
			new(dns.OPT),
		},
	},
	// Signed Response Test, ask twice to check cache
	{
		dnssec: true,
		Qname:  "104.server1.development.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newRRSIG("104.server1.development.region1.skydns.test. 3600 RRSIG SRV 5 6 3600 0 0 51945 skydns.test. deadbeaf"),
			newSRV("104.server1.development.region1.skydns.test. 3600 SRV 10 100 0 104.server1.development.region1.skydns.test.")},
		Extra: []dns.RR{
			newRRSIG("104.server1.developmen.region1.skydns.test. 3600 RRSIG A 5 6 3600 0 0 51945 skydns.test. deadbeaf"),
			newA("104.server1.development.region1.skydns.test. 3600 A 10.0.0.1"),
			new(dns.OPT),
		},
	},
	// NXDOMAIN Test
	{
		dnssec: true,
		Qname:  "doesnotexist.skydns.test.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			newNSEC3("44ohaq2njb0idnvolt9ggthvsk1e1uv8.skydns.test.	60 NSEC3 1 0 0 - 44OHAQ2NJB0IDNVOLT9GGTHVSK1E1UVA"),
			newRRSIG("44ohaq2njb0idnvolt9ggthvsk1e1uv8.skydns.test.	60 RRSIG NSEC3 5 3 3600 20140814205559 20140807175559 51945 skydns.test. deadbeef"),
			newNSEC3("ah4v7g5qoiri26armrb3bldqi1sng6a2.skydns.test.	60 NSEC3 1 0 0 - AH4V7G5QOIRI26ARMRB3BLDQI1SNG6A3 A AAAA SRV RRSIG"),
			newRRSIG("ah4v7g5qoiri26armrb3bldqi1sng6a2.skydns.test.	60 RRSIG NSEC3 5 3 3600 20140814205559 20140807175559 51945 skydns.test. deadbeef"),
			newNSEC3("lksd858f4cldl7emdord75k5jeks49p8.skydns.test.	60 NSEC3 1 0 0 - LKSD858F4CLDL7EMDORD75K5JEKS49PA"),
			newRRSIG("lksd858f4cldl7emdord75k5jeks49p8.skydns.test.	60 RRSIG NSEC3 5 3 3600 20140814205559 20140807175559 51945 skydns.test. deadbeef"),
			newRRSIG("skydns.test.	60 RRSIG SOA 5 2 3600 20140814205559 20140807175559 51945 skydns.test. deadbeaf"),
			newSOA("skydns.test. 3600 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0"),
		},
		Extra: []dns.RR{new(dns.OPT)},
	},
	// NXDOMAIN Test, cache test
	{
		dnssec: true,
		Qname:  "doesnotexist.skydns.test.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			newNSEC3("44ohaq2njb0idnvolt9ggthvsk1e1uv8.skydns.test.	60 NSEC3 1 0 0 - 44OHAQ2NJB0IDNVOLT9GGTHVSK1E1UVA"),
			newRRSIG("44ohaq2njb0idnvolt9ggthvsk1e1uv8.skydns.test.	60 RRSIG NSEC3 5 3 3600 20140814205559 20140807175559 51945 skydns.test. deadbeef"),
			newNSEC3("ah4v7g5qoiri26armrb3bldqi1sng6a2.skydns.test.	60 NSEC3 1 0 0 - AH4V7G5QOIRI26ARMRB3BLDQI1SNG6A3 A AAAA SRV RRSIG"),
			newRRSIG("ah4v7g5qoiri26armrb3bldqi1sng6a2.skydns.test.	60 RRSIG NSEC3 5 3 3600 20140814205559 20140807175559 51945 skydns.test. deadbeef"),
			newNSEC3("lksd858f4cldl7emdord75k5jeks49p8.skydns.test.	60 NSEC3 1 0 0 - LKSD858F4CLDL7EMDORD75K5JEKS49PA"),
			newRRSIG("lksd858f4cldl7emdord75k5jeks49p8.skydns.test.	60 RRSIG NSEC3 5 3 3600 20140814205559 20140807175559 51945 skydns.test. deadbeef"),
			newRRSIG("skydns.test.	60 RRSIG SOA 5 2 3600 20140814205559 20140807175559 51945 skydns.test. deadbeaf"),
			newSOA("skydns.test. 3600 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0"),
		},
		Extra: []dns.RR{new(dns.OPT)},
	},
	// NODATA Test
	{
		dnssec: true,
		Qname:  "104.server1.development.region1.skydns.test.", Qtype: dns.TypeTXT,
		Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			newNSEC3("E76CLEL5E7TQHRTFLTBVH0645NEKFJV9.skydns.test.	60 NSEC3 1 0 0 - E76CLEL5E7TQHRTFLTBVH0645NEKFJVA A AAAA SRV RRSIG"),
			newRRSIG("E76CLEL5E7TQHRTFLTBVH0645NEKFJV9.skydns.test.	60 RRSIG NSEC3 5 3 3600 20140814211641 20140807181641 51945 skydns.test. deadbeef"),
			newRRSIG("skydns.test.	60 RRSIG SOA 5 2 3600 20140814211641 20140807181641 51945 skydns.test. deadbeef"),
			newSOA("skydns.test.	60 SOA ns.dns.skydns.test. hostmaster.skydns.test. 1407445200 28800 7200 604800 60"),
		},
		Extra: []dns.RR{new(dns.OPT)},
	},
	// Reverse v4 local answer
	{
		Qname: "1.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Answer: []dns.RR{newPTR("1.0.0.10.in-addr.arpa. 3600 PTR reverse.example.com.")},
	},
	// Reverse v6 local answer

	// Reverse forwarding answer, TODO(miek) does not work
	//	{
	//		Qname: "1.0.16.172.in-addr.arpa.", Qtype: dns.TypePTR,
	//		Rcode: dns.RcodeNameError,
	//		Ns:    []dns.RR{newSOA("16.172.in-addr.arpa. 10800 SOA localhost. nobody.invalid. 0 0 0 0 0")},
	//	},

	// Reverse no answer

	// Local data query
	{
		Qname: "local.dns.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{newA("local.dns.skydns.test. 3600 A 10.0.0.1")},
	},
	// Author test
	{
		Qname: "skydns.test.", Qtype: dns.TypeTXT,
		chaos: true,
		Answer: []dns.RR{
			newTXT("skydns.test. 0 TXT \"Brian Ketelsen\""),
			newTXT("skydns.test. 0 TXT \"Erik St. Martin\""),
			newTXT("skydns.test. 0 TXT \"Michael Crosby\""),
			newTXT("skydns.test. 0 TXT \"Miek Gieben\""),
		},
	},
	// Author test 2
	{
		Qname: "authors.bind.", Qtype: dns.TypeTXT,
		chaos: true,
		Answer: []dns.RR{
			newTXT("authors.bind. 0 TXT \"Brian Ketelsen\""),
			newTXT("authors.bind. 0 TXT \"Erik St. Martin\""),
			newTXT("authors.bind. 0 TXT \"Michael Crosby\""),
			newTXT("authors.bind. 0 TXT \"Miek Gieben\""),
		},
	},
	// Author test, caps test
	{
		Qname: "AUTHOrs.BIND.", Qtype: dns.TypeTXT,
		chaos: true,
		Answer: []dns.RR{
			newTXT("AUTHOrs.BIND. 0 TXT \"Brian Ketelsen\""),
			newTXT("AUTHOrs.BIND. 0 TXT \"Erik St. Martin\""),
			newTXT("AUTHOrs.BIND. 0 TXT \"Michael Crosby\""),
			newTXT("AUTHOrs.BIND. 0 TXT \"Miek Gieben\""),
		},
	},
	// Author test 3, no answer.
	{
		Qname: "local.dns.skydns.test.", Qtype: dns.TypeA,
		Rcode: dns.RcodeServerFailure,
		chaos: true,
	},
	// HINFO Test, should be nodata for the apex
	{
		Qname: "skydns.test.", Qtype: dns.TypeHINFO,
		Ns: []dns.RR{newSOA("skydns.test. 3600 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
	},
}

func newA(rr string) *dns.A           { r, _ := dns.NewRR(rr); return r.(*dns.A) }
func newAAAA(rr string) *dns.AAAA     { r, _ := dns.NewRR(rr); return r.(*dns.AAAA) }
func newCNAME(rr string) *dns.CNAME   { r, _ := dns.NewRR(rr); return r.(*dns.CNAME) }
func newSRV(rr string) *dns.SRV       { r, _ := dns.NewRR(rr); return r.(*dns.SRV) }
func newSOA(rr string) *dns.SOA       { r, _ := dns.NewRR(rr); return r.(*dns.SOA) }
func newNS(rr string) *dns.NS         { r, _ := dns.NewRR(rr); return r.(*dns.NS) }
func newDNSKEY(rr string) *dns.DNSKEY { r, _ := dns.NewRR(rr); return r.(*dns.DNSKEY) }
func newRRSIG(rr string) *dns.RRSIG   { r, _ := dns.NewRR(rr); return r.(*dns.RRSIG) }
func newNSEC3(rr string) *dns.NSEC3   { r, _ := dns.NewRR(rr); return r.(*dns.NSEC3) }
func newPTR(rr string) *dns.PTR       { r, _ := dns.NewRR(rr); return r.(*dns.PTR) }
func newTXT(rr string) *dns.TXT       { r, _ := dns.NewRR(rr); return r.(*dns.TXT) }

func BenchmarkDNSSingleCache(b *testing.B) {
	b.StopTimer()
	t := new(testing.T)
	s := newTestServerDNSSEC(t, true)
	defer s.Stop()

	serv := services[0]
	addService(t, s, serv.Key, 0, serv)
	defer delService(t, s, serv.Key)

	c := new(dns.Client)
	tc := dnsTestCases[0]
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, "127.0.0.1:"+StrPort)
	}
}

func BenchmarkDNSWildcardCache(b *testing.B) {
	b.StopTimer()
	t := new(testing.T)
	s := newTestServerDNSSEC(t, true)
	defer s.Stop()

	for _, serv := range services {
		m := &msg.Service{Host: serv.Host, Port: serv.Port}
		addService(t, s, serv.Key, 0, m)
		defer delService(t, s, serv.Key)
	}

	c := new(dns.Client)
	tc := dnsTestCases[8] // Wildcard Test
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, "127.0.0.1:"+StrPort)
	}
}

func BenchmarkDNSSECSingleCache(b *testing.B) {
	b.StopTimer()
	t := new(testing.T)
	s := newTestServerDNSSEC(t, true)
	defer s.Stop()

	serv := services[0]
	addService(t, s, serv.Key, 0, serv)
	defer delService(t, s, serv.Key)

	c := new(dns.Client)
	tc := dnsTestCases[0]
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)
	m.SetEdns0(4096, true)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, "127.0.0.1:"+StrPort)
	}
}

func BenchmarkDNSSingleNoCache(b *testing.B) {
	b.StopTimer()
	t := new(testing.T)
	s := newTestServerDNSSEC(t, false)
	defer s.Stop()

	serv := services[0]
	addService(t, s, serv.Key, 0, serv)
	defer delService(t, s, serv.Key)

	c := new(dns.Client)
	tc := dnsTestCases[0]
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, "127.0.0.1:"+StrPort)
	}
}

func BenchmarkDNSWildcardNoCache(b *testing.B) {
	b.StopTimer()
	t := new(testing.T)
	s := newTestServerDNSSEC(t, false)
	defer s.Stop()

	for _, serv := range services {
		m := &msg.Service{Host: serv.Host, Port: serv.Port}
		addService(t, s, serv.Key, 0, m)
		defer delService(t, s, serv.Key)
	}

	c := new(dns.Client)
	tc := dnsTestCases[8] // Wildcard Test
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, "127.0.0.1:"+StrPort)
	}
}

func BenchmarkDNSSECSingleNoCache(b *testing.B) {
	b.StopTimer()
	t := new(testing.T)
	s := newTestServerDNSSEC(t, false)
	defer s.Stop()

	serv := services[0]
	addService(t, s, serv.Key, 0, serv)
	defer delService(t, s, serv.Key)

	c := new(dns.Client)
	tc := dnsTestCases[0]
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)
	m.SetEdns0(4096, true)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, "127.0.0.1:"+StrPort)
	}
}
