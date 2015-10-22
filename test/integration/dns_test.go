// +build integration,etcd

package integration

import (
	"net"
	"testing"
	"time"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util"

	"github.com/miekg/dns"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestDNS(t *testing.T) {
	masterConfig, clientFile, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	localAddr := ""
	if ip, err := cmdutil.DefaultLocalIP4(); err == nil {
		localAddr = ip.String()
	} else if err == cmdutil.ErrorNoDefaultIP {
		localAddr = "127.0.0.1"
	} else if err != nil {
		t.Fatalf("Unable to find a local IP address: %v", err)
	}

	localIP := net.ParseIP(localAddr)
	var masterIP net.IP
	// verify service DNS entry is visible
	stop := make(chan struct{})
	util.Until(func() {
		m1 := &dns.Msg{
			MsgHdr:   dns.MsgHdr{Id: dns.Id(), RecursionDesired: false},
			Question: []dns.Question{{"kubernetes.default.svc.cluster.local.", dns.TypeA, dns.ClassINET}},
		}
		in, exchangeErr := dns.Exchange(m1, masterConfig.DNSConfig.BindAddress)
		if exchangeErr != nil {
			t.Logf("unexpected error: %v", exchangeErr)
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
	for {
		if _, err := client.Services(kapi.NamespaceDefault).Create(&kapi.Service{
			ObjectMeta: kapi.ObjectMeta{
				Name: "headless",
			},
			Spec: kapi.ServiceSpec{
				ClusterIP: kapi.ClusterIPNone,
				Ports:     []kapi.ServicePort{{Port: 443}},
			},
		}); err != nil {
			if errors.IsForbidden(err) {
				t.Logf("forbidden, sleeping: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := client.Endpoints(kapi.NamespaceDefault).Create(&kapi.Endpoints{
			ObjectMeta: kapi.ObjectMeta{
				Name: "headless",
			},
			Subsets: []kapi.EndpointSubset{{
				Addresses: []kapi.EndpointAddress{{IP: "172.0.0.1"}},
				Ports: []kapi.EndpointPort{
					{Port: 2345},
				},
			}},
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		break
	}
	headlessIP := net.ParseIP("172.0.0.1")

	if _, err := client.Services(kapi.NamespaceDefault).Create(&kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name: "headless2",
		},
		Spec: kapi.ServiceSpec{
			ClusterIP: kapi.ClusterIPNone,
			Ports:     []kapi.ServicePort{{Port: 443}},
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := client.Endpoints(kapi.NamespaceDefault).Create(&kapi.Endpoints{
		ObjectMeta: kapi.ObjectMeta{
			Name: "headless2",
		},
		Subsets: []kapi.EndpointSubset{{
			Addresses: []kapi.EndpointAddress{{IP: "172.0.0.2"}},
			Ports: []kapi.EndpointPort{
				{Port: 2345, Name: "other"},
				{Port: 2346, Name: "http"},
			},
		}},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	headless2IP := net.ParseIP("172.0.0.2")

	tests := []struct {
		dnsQuestionName   string
		recursionExpected bool
		retry             bool
		expect            []*net.IP
		srv               []*dns.SRV
	}{
		{ // wildcard resolution of a service works
			dnsQuestionName: "foo.kubernetes.default.svc.cluster.local.",
			expect:          []*net.IP{&masterIP},
		},
		{ // resolving endpoints of a service works
			dnsQuestionName: "_endpoints.kubernetes.default.svc.cluster.local.",
			expect:          []*net.IP{&localIP},
		},
		{ // openshift override works
			dnsQuestionName: "openshift.default.svc.cluster.local.",
			expect:          []*net.IP{&masterIP},
		},
		{ // headless service
			dnsQuestionName: "headless.default.svc.cluster.local.",
			expect:          []*net.IP{&headlessIP},
		},
		{ // specific port of a headless service
			dnsQuestionName: "unknown-port-2345.e1.headless.default.svc.cluster.local.",
			expect:          []*net.IP{&headlessIP},
		},
		{ // SRV record for that service
			dnsQuestionName: "headless.default.svc.cluster.local.",
			srv: []*dns.SRV{
				{
					Target: "unknown-port-2345.e1.headless.",
					Port:   2345,
				},
			},
		},
		{ // the SRV record resolves to the IP
			dnsQuestionName: "unknown-port-2345.e1.headless.default.svc.cluster.local.",
			expect:          []*net.IP{&headlessIP},
		},
		{ // headless 2 service
			dnsQuestionName: "headless2.default.svc.cluster.local.",
			expect:          []*net.IP{&headless2IP},
		},
		{ // SRV records for that service
			dnsQuestionName: "headless2.default.svc.cluster.local.",
			srv: []*dns.SRV{
				{
					Target: "http.e1.headless2.",
					Port:   2346,
				},
				{
					Target: "other.e1.headless2.",
					Port:   2345,
				},
			},
		},
		{ // the SRV record resolves to the IP
			dnsQuestionName: "other.e1.headless2.default.svc.cluster.local.",
			expect:          []*net.IP{&headless2IP},
		},
		{
			dnsQuestionName:   "www.google.com.",
			recursionExpected: false,
		},
	}
	for i, tc := range tests {
		qType := dns.TypeA
		if tc.srv != nil {
			qType = dns.TypeSRV
		}
		m1 := &dns.Msg{
			MsgHdr:   dns.MsgHdr{Id: dns.Id(), RecursionDesired: false},
			Question: []dns.Question{{tc.dnsQuestionName, qType, dns.ClassINET}},
		}
		ch := make(chan struct{})
		count := 0
		util.Until(func() {
			count++
			if count > 100 {
				t.Errorf("%d: failed after max iterations", i)
				close(ch)
				return
			}
			in, err := dns.Exchange(m1, masterConfig.DNSConfig.BindAddress)
			if err != nil {
				return
			}
			switch {
			case tc.srv != nil:
				if len(in.Answer) != len(tc.srv) {
					t.Logf("%d: incorrect number of answers: %#v", i, in)
					return
				}
			case tc.recursionExpected:
				if len(in.Answer) == 0 {
					t.Errorf("%d: expected forward resolution: %#v", i, in)
				}
				close(ch)
				return
			default:
				if len(in.Answer) != len(tc.expect) {
					t.Logf("%d: did not resolve or unexpected forward resolution: %#v", i, in)
					return
				}
			}
			for _, answer := range in.Answer {
				switch a := answer.(type) {
				case *dns.A:
					matches := false
					if a.A != nil {
						for _, expect := range tc.expect {
							if a.A.String() == expect.String() {
								matches = true
								break
							}
						}
					}
					if !matches {
						t.Errorf("A record does not match any expected answer: %v", a.A)
					}
				case *dns.SRV:
					matches := false
					for _, expect := range tc.srv {
						if expect.Port == a.Port && expect.Target == a.Target {
							matches = true
							break
						}
					}
					if !matches {
						t.Errorf("SRV record does not match any expected answer: %#v", a)
					}
				default:
					t.Errorf("expected an A or SRV record: %#v", in)
				}
			}
			t.Log(in)
			close(ch)
		}, 50*time.Millisecond, ch)
	}
}
