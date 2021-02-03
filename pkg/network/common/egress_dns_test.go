package common

import (
	"net"
	"time"

	"testing"

	networkv1 "github.com/openshift/api/network/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

func newEgressNetworkPolicy(dnsName string, namespace string) networkv1.EgressNetworkPolicy {
	return networkv1.EgressNetworkPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name:      "enp",
			Namespace: namespace,
			UID:       ktypes.UID(namespace + "-enp"),
		},
		Spec: networkv1.EgressNetworkPolicySpec{
			Egress: []networkv1.EgressNetworkPolicyRule{
				{
					Type: networkv1.EgressNetworkPolicyRuleAllow,
					To: networkv1.EgressNetworkPolicyPeer{
						DNSName: dnsName,
					},
				},
			},
		},
	}
}

func TestSync(t *testing.T) {

	startTime := time.Now().Add(150 * time.Millisecond)
	DNSReplies := []fakeDNSReply{
		{
			name:          "domain1.com",
			ttl:           1 * time.Second,
			ips:           []net.IP{net.ParseIP("1.1.1.1")},
			delay:         50 * time.Millisecond,
			nextQueryTime: startTime.Add(100 * time.Millisecond),
		},
		{
			name:          "domain2.com",
			ttl:           1 * time.Second,
			ips:           []net.IP{net.ParseIP("1.2.3.4")},
			delay:         3500 * time.Millisecond,
			nextQueryTime: startTime.Add(150 * time.Millisecond),
		},
		{
			name:          "domain1.com",
			ttl:           1 * time.Second,
			ips:           []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("1.1.1.2")},
			delay:         50 * time.Millisecond,
			nextQueryTime: startTime.Add(200 * time.Millisecond),
		},
	}

	dnsInfo := NewFakeDNS(DNSReplies)
	egressDNS := EgressDNS{
		dns:                dnsInfo,
		dnsNamesToPolicies: map[string]sets.String{},
		namespaces:         map[ktypes.UID]string{},
		added:              make(chan bool),
		Updates:            make(chan EgressDNSUpdates),
		stopCh:             make(chan struct{}),
	}

	egressDNS.Add(newEgressNetworkPolicy("domain1.com", "fake-ns-1"))
	egressDNS.Add(newEgressNetworkPolicy("domain2.com", "fake-ns-2"))
	egressDNS.Add(newEgressNetworkPolicy("domain1.com", "fake-ns-3"))

	go egressDNS.Sync()
	update := <-egressDNS.Updates
	if len(update) != 2 {
		t.Errorf("Expected exactly two elements in the update: %v", update)
		// Exit the function to avoid a nil pointer dereference
		return
	}

	u0 := update[0]
	u1 := update[1]
	if !((u0.Namespace == "fake-ns-1" && u1.Namespace == "fake-ns-3") ||
		(u0.Namespace == "fake-ns-3" && u1.Namespace == "fake-ns-1")) {
		t.Errorf("Expecting an update for fake-ns-1 and fake-ns-3. Got: %v", update)
	}

	// If the queries were made asynchronously, the EgressDNSUpdatesi with
	// fake-ns-2 would make it to the updates channel after fake-ns-1 and
	// fake-ns-3. This is actual proof of BZ#1850060
	update = <-egressDNS.Updates
	if len(update) != 1 {
		t.Errorf("Expected exactly one element in the update: %v", update)
		// Exit the function to avoid a nil pointer dereference
		return
	}
	u0 = update[0]
	if u0.Namespace != "fake-ns-2" {
		t.Errorf("Expecting an update for fake-ns-1 and fake-ns-2. Got: %v", update)
	}

	// This should arrive right before the update for fake-ns-2

	update = <-egressDNS.Updates
	if len(update) != 2 {
		t.Errorf("Expected exactly two element in the update: %v", update)
		// Exit the function to avoid a nil pointer dereference
		return
	}

	u0 = update[0]
	u1 = update[1]
	if !((u0.Namespace == "fake-ns-1" && u1.Namespace == "fake-ns-3") ||
		(u0.Namespace == "fake-ns-3" && u1.Namespace == "fake-ns-1")) {
		t.Errorf("Expecting an update for fake-ns-1 and fake-ns-3. Got: %v", update)
	}

	egressDNS.Stop()

}
