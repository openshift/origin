package integration

import (
	"fmt"
	"hash/fnv"
	"net"
	"strconv"
	"testing"
	"time"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	waitutil "k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/miekg/dns"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestDNS(t *testing.T) {
	masterConfig, clientFile, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

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
	waitutil.Until(func() {
		m1 := &dns.Msg{
			MsgHdr:   dns.MsgHdr{Id: dns.Id(), RecursionDesired: false},
			Question: []dns.Question{{Name: "kubernetes.default.svc.cluster.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET}},
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

	// Verify kubernetes service port is 53 and target port is the
	// configured masterConfig.DNSConfig.BindAddress port.
	_, dnsPortString, err := net.SplitHostPort(masterConfig.DNSConfig.BindAddress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dnsPort, err := strconv.Atoi(dnsPortString)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	kubernetesService, err := client.Core().Services(metav1.NamespaceDefault).Get("kubernetes", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, port := range kubernetesService.Spec.Ports {
		if port.Port == 53 && port.TargetPort.IntVal == int32(dnsPort) && port.Protocol == kapi.ProtocolTCP {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("did not find DNS port in kubernetes service: %#v", kubernetesService)
	}

	for {
		if _, err := client.Core().Services(metav1.NamespaceDefault).Create(&kapi.Service{
			ObjectMeta: metav1.ObjectMeta{
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
		if _, err := client.Core().Endpoints(metav1.NamespaceDefault).Create(&kapi.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name: "headless",
			},
			Subsets: []kapi.EndpointSubset{{
				Addresses: []kapi.EndpointAddress{{IP: "172.0.0.1"}},
				Ports: []kapi.EndpointPort{
					{Port: 2345, Name: "http"},
				},
			}},
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		break
	}
	headlessIP := net.ParseIP("172.0.0.1")
	headlessIPHash := getHash(headlessIP.String())

	if _, err := client.Core().Services(metav1.NamespaceDefault).Create(&kapi.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "headless2",
		},
		Spec: kapi.ServiceSpec{
			ClusterIP: kapi.ClusterIPNone,
			Ports:     []kapi.ServicePort{{Port: 443}},
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := client.Core().Endpoints(metav1.NamespaceDefault).Create(&kapi.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
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
	precannedIP := net.ParseIP("10.2.4.50")

	headless2IPHash := getHash(headless2IP.String())

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
		{ // pod by IP
			dnsQuestionName: "10-2-4-50.default.pod.cluster.local.",
			expect:          []*net.IP{&precannedIP},
		},
		{ // headless service
			dnsQuestionName: "headless.default.svc.cluster.local.",
			expect:          []*net.IP{&headlessIP},
		},
		{ // specific port of a headless service
			dnsQuestionName: "_http._tcp.headless.default.svc.cluster.local.",
			expect:          []*net.IP{&headlessIP},
		},
		{ // SRV record for that service
			dnsQuestionName: "headless.default.svc.cluster.local.",
			srv: []*dns.SRV{
				{
					Target: headlessIPHash + ".headless.default.svc.cluster.local.",
					Port:   0,
				},
			},
		},
		{ // SRV record for a port
			dnsQuestionName: "_http._tcp.headless2.default.svc.cluster.local.",
			srv: []*dns.SRV{
				{
					Target: headless2IPHash + ".headless2.default.svc.cluster.local.",
					Port:   2346,
				},
			},
		},
		{ // the SRV record resolves to the IP
			dnsQuestionName: "_http._tcp.headless.default.svc.cluster.local.",
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
					Target: headless2IPHash + ".headless2.default.svc.cluster.local.",
					Port:   0,
				},
			},
		},
		{ // the SRV record resolves to the IP
			dnsQuestionName: headless2IPHash + ".headless2.default.svc.cluster.local.",
			expect:          []*net.IP{&headless2IP},
		},
		{
			dnsQuestionName:   "www.google.com.",
			recursionExpected: true,
		},
	}
	for i, tc := range tests {
		qType := dns.TypeA
		if tc.srv != nil {
			qType = dns.TypeSRV
		}
		m1 := &dns.Msg{
			MsgHdr:   dns.MsgHdr{Id: dns.Id(), RecursionDesired: tc.recursionExpected},
			Question: []dns.Question{{Name: tc.dnsQuestionName, Qtype: qType, Qclass: dns.ClassINET}},
		}
		ch := make(chan struct{})
		count := 0
		failedLatency := 0
		waitutil.Until(func() {
			count++
			if count > 100 {
				t.Errorf("%d: failed after max iterations", i)
				close(ch)
				return
			}
			before := time.Now()
			in, err := dns.Exchange(m1, masterConfig.DNSConfig.BindAddress)
			if err != nil {
				return
			}
			after := time.Now()
			delta := after.Sub(before)
			if delta > 500*time.Millisecond {
				failedLatency++
				if failedLatency > 10 {
					t.Errorf("%d: failed after 10 requests took longer than 500ms", i)
					close(ch)
				}
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
						t.Errorf("%d: A record does not match any expected answer for %q: %v", i, tc.dnsQuestionName, a.A)
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
						t.Errorf("%d: SRV record does not match any expected answer %q: %#v", i, tc.dnsQuestionName, a)
					}
				default:
					t.Errorf("%d: expected an A or SRV record %q: %#v", i, tc.dnsQuestionName, in)
				}
			}
			t.Log(in)
			close(ch)
		}, 50*time.Millisecond, ch)
	}
}

// return a hash for the key name
func getHash(text string) string {
	h := fnv.New32a()
	h.Write([]byte(text))
	return fmt.Sprintf("%x", h.Sum32())
}
