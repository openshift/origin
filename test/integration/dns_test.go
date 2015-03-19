// +build integration,!no-etcd

package integration

import (
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/miekg/dns"
	testutil "github.com/openshift/origin/test/util"
)

func TestDNS(t *testing.T) {
	masterConfig, _, err := testutil.StartTestAllInOne()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify service DNS entry is visible
	stop := make(chan struct{})
	util.Until(func() {
		m1 := &dns.Msg{
			MsgHdr:   dns.MsgHdr{Id: dns.Id(), RecursionDesired: true},
			Question: []dns.Question{{"kubernetes.default.local.", dns.TypeA, dns.ClassINET}},
		}
		in, err := dns.Exchange(m1, masterConfig.DNSConfig.BindAddress)
		if err != nil {
			t.Logf("unexpected error: %v", err)
			return
		}
		if len(in.Answer) != 1 {
			t.Logf("unexpected answer: %#v", in)
			return
		}
		if a, ok := in.Answer[0].(*dns.A); ok {
			if a.A == nil {
				t.Errorf("expected an A record with an IP: %#v", a)
			}
		} else {
			t.Errorf("expected an A record: %#v", in)
		}
		t.Log(in)
		close(stop)
	}, 50*time.Millisecond, stop)

	// verify recursive DNS lookup is visible
	m1 := &dns.Msg{
		MsgHdr:   dns.MsgHdr{Id: dns.Id(), RecursionDesired: true},
		Question: []dns.Question{{"foo.kubernetes.default.local.", dns.TypeA, dns.ClassINET}},
	}
	in, err := dns.Exchange(m1, masterConfig.DNSConfig.BindAddress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(in.Answer) != 1 {
		t.Fatalf("unexpected response: %#v", in)
	}
	if a, ok := in.Answer[0].(*dns.A); ok {
		if a.A == nil {
			t.Errorf("expected an A record with an IP: %#v", a)
		}
	} else {
		t.Errorf("expected an A record: %#v", in)
	}
	t.Log(in)

	// verify recursive DNS lookup is visible
	m1 = &dns.Msg{
		MsgHdr:   dns.MsgHdr{Id: dns.Id(), RecursionDesired: true},
		Question: []dns.Question{{"www.google.com.", dns.TypeA, dns.ClassINET}},
	}
	in, err = dns.Exchange(m1, masterConfig.DNSConfig.BindAddress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(in.Answer) == 0 {
		t.Fatalf("unexpected response: %#v", in)
	}
	if a, ok := in.Answer[0].(*dns.A); ok {
		if a.A == nil {
			t.Errorf("expected an A record with an IP: %#v", a)
		}
	} else {
		t.Errorf("expected an A record: %#v", in)
	}
	t.Log(in)
}
