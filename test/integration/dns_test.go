// +build integration,!no-etcd

package integration

import (
	"net"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/miekg/dns"
	testutil "github.com/openshift/origin/test/util"
)

func TestDNS(t *testing.T) {
	masterConfig, clientFile, err := testutil.StartTestAllInOne()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var masterIP net.IP

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
				t.Fatalf("expected an A record with an IP: %#v", a)
			}
			masterIP = a.A
		} else {
			t.Fatalf("expected an A record: %#v", in)
		}
		t.Log(in)
		close(stop)
	}, 50*time.Millisecond, stop)

	client, err := testutil.GetClusterAdminKubeClient(clientFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := client.Services(kapi.NamespaceDefault).Create(&kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name: "headless",
		},
		Spec: kapi.ServiceSpec{
			PortalIP: kapi.PortalIPNone,
			Port:     443,
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := client.Endpoints(kapi.NamespaceDefault).Create(&kapi.Endpoints{
		ObjectMeta: kapi.ObjectMeta{
			Name: "headless",
		},
		Endpoints: []kapi.Endpoint{
			{
				IP:   "172.0.0.1",
				Port: 2345,
			},
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	headlessIP := net.ParseIP("172.0.0.1")

	// verify recursive DNS lookup is visible when expected
	tests := []struct {
		dnsQuestionName   string
		recursionExpected bool
		expect            *net.IP
	}{
		{
			dnsQuestionName:   "foo.kubernetes.default.local.",
			recursionExpected: false,
			expect:            &masterIP,
		},
		{
			dnsQuestionName:   "openshift.default.local.",
			recursionExpected: false,
			expect:            &masterIP,
		},
		{
			dnsQuestionName:   "headless.default.local.",
			recursionExpected: false,
			expect:            &headlessIP,
		},
		{
			dnsQuestionName:   "www.google.com.",
			recursionExpected: true,
		},
	}
	for _, tc := range tests {
		m1 := &dns.Msg{
			MsgHdr:   dns.MsgHdr{Id: dns.Id(), RecursionDesired: true},
			Question: []dns.Question{{tc.dnsQuestionName, dns.TypeA, dns.ClassINET}},
		}
		in, err := dns.Exchange(m1, masterConfig.DNSConfig.BindAddress)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !tc.recursionExpected && len(in.Answer) != 1 {
			t.Fatalf("did not resolve or unexpected forward resolution: %#v", in)
		} else if tc.recursionExpected && len(in.Answer) == 0 {
			t.Fatalf("expected forward resolution: %#v", in)
		}
		if a, ok := in.Answer[0].(*dns.A); ok {
			if a.A == nil {
				t.Errorf("expected an A record with an IP: %#v", a)
			} else {
				if tc.expect != nil && tc.expect.String() != a.A.String() {
					t.Errorf("A record has a different IP than the test case: %v / %v", a.A, *tc.expect)
				}
			}
		} else {
			t.Errorf("expected an A record: %#v", in)
		}
		t.Log(in)
	}
}
