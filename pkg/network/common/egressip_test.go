package common

import (
	"fmt"
	"strings"
	"testing"

	ktypes "k8s.io/apimachinery/pkg/types"

	networkapi "github.com/openshift/api/network/v1"
)

type testEIPWatcher struct {
	changes []string
}

func (w *testEIPWatcher) ClaimEgressIP(vnid uint32, egressIP, nodeIP string) {
	w.changes = append(w.changes, fmt.Sprintf("claim %s on %s for namespace %d", egressIP, nodeIP, vnid))
}

func (w *testEIPWatcher) ReleaseEgressIP(egressIP, nodeIP string) {
	w.changes = append(w.changes, fmt.Sprintf("release %s on %s", egressIP, nodeIP))
}

func (w *testEIPWatcher) SetNamespaceEgressNormal(vnid uint32) {
	w.changes = append(w.changes, fmt.Sprintf("namespace %d normal", int(vnid)))
}

func (w *testEIPWatcher) SetNamespaceEgressDropped(vnid uint32) {
	w.changes = append(w.changes, fmt.Sprintf("namespace %d dropped", int(vnid)))
}

func (w *testEIPWatcher) SetNamespaceEgressViaEgressIP(vnid uint32, egressIP, nodeIP string) {
	w.changes = append(w.changes, fmt.Sprintf("namespace %d via %s on %s", int(vnid), egressIP, nodeIP))
}

func (w *testEIPWatcher) UpdateEgressCIDRs() {
	w.changes = append(w.changes, "update egress CIDRs")
}

func (w *testEIPWatcher) assertChanges(expected ...string) error {
	changed := w.changes
	w.changes = []string{}
	missing := []string{}

	for len(expected) > 0 {
		exp := expected[0]
		expected = expected[1:]
		for i, ch := range changed {
			if ch == exp {
				changed = append(changed[:i], changed[i+1:]...)
				exp = ""
				break
			}
		}
		if exp != "" {
			missing = append(missing, exp)
		}
	}

	if len(changed) > 0 && len(missing) > 0 {
		return fmt.Errorf("unexpected changes %#v, missing changes %#v", changed, missing)
	} else if len(changed) > 0 {
		return fmt.Errorf("unexpected changes %#v", changed)
	} else if len(missing) > 0 {
		return fmt.Errorf("missing changes %#v", missing)
	} else {
		return nil
	}
}

func (w *testEIPWatcher) assertNoChanges() error {
	return w.assertChanges()
}

func (w *testEIPWatcher) flushChanges() {
	w.changes = []string{}
}

func (w *testEIPWatcher) assertUpdateEgressCIDRsNotification() error {
	for _, change := range w.changes {
		if change == "update egress CIDRs" {
			w.flushChanges()
			return nil
		}
	}
	return fmt.Errorf("expected change \"update egress CIDRs\", got %#v", w.changes)
}

func setupEgressIPTracker(t *testing.T) (*EgressIPTracker, *testEIPWatcher) {
	watcher := &testEIPWatcher{}
	return NewEgressIPTracker(watcher), watcher
}

func updateHostSubnetEgress(eit *EgressIPTracker, hs *networkapi.HostSubnet) {
	if hs.Host == "" {
		hs.Host = "node-" + hs.HostIP[strings.LastIndex(hs.HostIP, ".")+1:]
	}
	hs.Name = hs.Host
	hs.UID = ktypes.UID(hs.Name)
	eit.UpdateHostSubnetEgress(hs)
}

func updateNetNamespaceEgress(eit *EgressIPTracker, ns *networkapi.NetNamespace) {
	if ns.NetName == "" {
		ns.NetName = fmt.Sprintf("ns-%d", ns.NetID)
	}
	ns.Name = ns.NetName
	ns.UID = ktypes.UID(ns.Name)
	eit.UpdateNetNamespaceEgress(ns)
}

func TestEgressIP(t *testing.T) {
	eit, w := setupEgressIPTracker(t)

	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP: "172.17.0.3",
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP: "172.17.0.4",
	})
	eit.DeleteNetNamespaceEgress(42)
	eit.DeleteNetNamespaceEgress(43)

	// No namespaces use egress yet, so should be no changes
	err := w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Assign NetNamespace.EgressIP first, then HostSubnet.EgressIP
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     42,
		EgressIPs: []string{"172.17.0.100"},
	})
	err = w.assertChanges(
		"namespace 42 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100"}, // Added .100
	})
	err = w.assertChanges(
		"claim 172.17.0.100 on 172.17.0.3 for namespace 42",
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Assign HostSubnet.EgressIP first, then NetNamespace.EgressIP
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100", "172.17.0.101"}, // Added .101
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.5",
		EgressIPs: []string{"172.17.0.105"},
	})
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     43,
		EgressIPs: []string{"172.17.0.105"},
	})
	err = w.assertChanges(
		"claim 172.17.0.105 on 172.17.0.5 for namespace 43",
		"namespace 43 via 172.17.0.105 on 172.17.0.5",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Change NetNamespace.EgressIP
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     43,
		EgressIPs: []string{"172.17.0.101"},
	})
	err = w.assertChanges(
		"release 172.17.0.105 on 172.17.0.5",
		"claim 172.17.0.101 on 172.17.0.3 for namespace 43",
		"namespace 43 via 172.17.0.101 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Assign another EgressIP...
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     44,
		EgressIPs: []string{"172.17.0.104"},
	})
	err = w.assertChanges(
		"namespace 44 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.4",
		EgressIPs: []string{"172.17.0.102", "172.17.0.104"}, // Added .102, .104
	})
	err = w.assertChanges(
		"claim 172.17.0.104 on 172.17.0.4 for namespace 44",
		"namespace 44 via 172.17.0.104 on 172.17.0.4",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Change Namespace EgressIP
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     44,
		EgressIPs: []string{"172.17.0.102"},
	})
	err = w.assertChanges(
		"release 172.17.0.104 on 172.17.0.4",
		"claim 172.17.0.102 on 172.17.0.4 for namespace 44",
		"namespace 44 via 172.17.0.102 on 172.17.0.4",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Assign HostSubnet.EgressIP first, then NetNamespace.EgressIP
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.4",
		EgressIPs: []string{"172.17.0.102", "172.17.0.103"}, // Added .103, Dropped .104
	})
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     45,
		EgressIPs: []string{"172.17.0.103"},
	})
	err = w.assertChanges(
		"claim 172.17.0.103 on 172.17.0.4 for namespace 45",
		"namespace 45 via 172.17.0.103 on 172.17.0.4",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Drop namespace EgressIP
	eit.DeleteNetNamespaceEgress(44)
	err = w.assertChanges(
		"release 172.17.0.102 on 172.17.0.4",
		"namespace 44 normal",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Add namespace EgressIP back again after having removed it...
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     44,
		EgressIPs: []string{"172.17.0.102"},
	})
	err = w.assertChanges(
		"claim 172.17.0.102 on 172.17.0.4 for namespace 44",
		"namespace 44 via 172.17.0.102 on 172.17.0.4",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Drop node EgressIPs
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100"}, // Dropped .101
	})
	err = w.assertChanges(
		"release 172.17.0.101 on 172.17.0.3",
		"namespace 43 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.4",
		EgressIPs: []string{"172.17.0.102"}, // Dropped .103
	})
	err = w.assertChanges(
		"release 172.17.0.103 on 172.17.0.4",
		"namespace 45 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Add them back, swapped
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100", "172.17.0.103"}, // Added .103
	})
	err = w.assertChanges(
		"claim 172.17.0.103 on 172.17.0.3 for namespace 45",
		"namespace 45 via 172.17.0.103 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.4",
		EgressIPs: []string{"172.17.0.101", "172.17.0.102"}, // Added .101
	})
	err = w.assertChanges(
		"claim 172.17.0.101 on 172.17.0.4 for namespace 43",
		"namespace 43 via 172.17.0.101 on 172.17.0.4",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestMultipleNamespaceEgressIPs(t *testing.T) {
	eit, w := setupEgressIPTracker(t)

	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     42,
		EgressIPs: []string{"172.17.0.100"},
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100"},
	})
	err := w.assertChanges(
		// after UpdateNamespaceEgress()
		"namespace 42 dropped",
		// after UpdateHostSubnetEgress()
		"claim 172.17.0.100 on 172.17.0.3 for namespace 42",
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Prepending a second, unavailable, namespace egress IP should have no effect
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     42,
		EgressIPs: []string{"172.17.0.101", "172.17.0.100"},
	})
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Now assigning that IP to a node should switch OVS to use that since it's first in the list
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.4",
		EgressIPs: []string{"172.17.0.101"},
	})
	err = w.assertChanges(
		"claim 172.17.0.101 on 172.17.0.4 for namespace 42",
		"namespace 42 via 172.17.0.101 on 172.17.0.4",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Swapping the order in the NetNamespace should swap back
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     42,
		EgressIPs: []string{"172.17.0.100", "172.17.0.101"},
	})
	err = w.assertChanges(
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the inactive egress IP from its node should have no effect
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.4",
		EgressIPs: []string{"172.17.0.200"},
	})
	err = w.assertChanges(
		"release 172.17.0.101 on 172.17.0.4",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the remaining egress IP should now kill the namespace
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{},
	})
	err = w.assertChanges(
		"release 172.17.0.100 on 172.17.0.3",
		"namespace 42 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Now add the egress IPs back...
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100"},
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.4",
		EgressIPs: []string{"172.17.0.101"},
	})
	err = w.assertChanges(
		"claim 172.17.0.100 on 172.17.0.3 for namespace 42",
		"claim 172.17.0.101 on 172.17.0.4 for namespace 42",
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Assigning either the used or the unused Egress IP to another namespace should
	// break this namespace
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     43,
		EgressIPs: []string{"172.17.0.100"},
	})
	err = w.assertChanges(
		"release 172.17.0.100 on 172.17.0.3",
		"namespace 42 dropped",
		"namespace 43 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	eit.DeleteNetNamespaceEgress(43)
	err = w.assertChanges(
		"claim 172.17.0.100 on 172.17.0.3 for namespace 42",
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
		"namespace 43 normal",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     44,
		EgressIPs: []string{"172.17.0.101"},
	})
	err = w.assertChanges(
		"release 172.17.0.101 on 172.17.0.4",
		"namespace 42 dropped",
		"namespace 44 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	eit.DeleteNetNamespaceEgress(44)
	err = w.assertChanges(
		"claim 172.17.0.101 on 172.17.0.4 for namespace 42",
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
		"namespace 44 normal",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestDuplicateNodeEgressIPs(t *testing.T) {
	eit, w := setupEgressIPTracker(t)

	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     42,
		EgressIPs: []string{"172.17.0.100"},
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100"},
	})
	err := w.assertChanges(
		// after UpdateNamespaceEgress()
		"namespace 42 dropped",
		// after UpdateHostSubnetEgress()
		"claim 172.17.0.100 on 172.17.0.3 for namespace 42",
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Adding the Egress IP to another node should not work and should cause the
	// namespace to start dropping traffic. (And in particular, should not result
	// in a ClaimEgressIP for the new IP.)
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.4",
		EgressIPs: []string{"172.17.0.100"},
	})
	err = w.assertChanges(
		"release 172.17.0.100 on 172.17.0.3",
		"namespace 42 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Auto-egress-IP assignment should ignore the IP while it is double-booked
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:      "172.17.0.5",
		EgressCIDRs: []string{"172.17.0.0/24"},
	})
	err = w.assertChanges(
		"update egress CIDRs",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
	allocation := eit.ReallocateEgressIPs()
	if node5ips, ok := allocation["node-5"]; !ok {
		t.Fatalf("Unexpected IP allocation: %#v", allocation)
	} else if len(node5ips) != 0 {
		t.Fatalf("Unexpected IP allocation: %#v", allocation)
	}
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:      "172.17.0.5",
		EgressCIDRs: []string{},
	})
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the duplicate node egressIP should restore traffic to the broken namespace
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.4",
		EgressIPs: []string{},
	})
	err = w.assertChanges(
		"claim 172.17.0.100 on 172.17.0.3 for namespace 42",
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// As above, but with a different node IP
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.5",
		EgressIPs: []string{"172.17.0.100"},
	})
	err = w.assertChanges(
		"release 172.17.0.100 on 172.17.0.3",
		"namespace 42 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the egress IP from the namespace and then adding it back should result
	// in it still being broken.
	eit.DeleteNetNamespaceEgress(42)
	err = w.assertChanges(
		"namespace 42 normal",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     42,
		EgressIPs: []string{"172.17.0.100"},
	})
	err = w.assertChanges(
		"namespace 42 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the original egress node should result in the "duplicate" egress node
	// now being used.
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{},
	})
	err = w.assertChanges(
		"claim 172.17.0.100 on 172.17.0.5 for namespace 42",
		"namespace 42 via 172.17.0.100 on 172.17.0.5",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestDuplicateNamespaceEgressIPs(t *testing.T) {
	eit, w := setupEgressIPTracker(t)

	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     42,
		EgressIPs: []string{"172.17.0.100"},
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100"},
	})
	err := w.assertChanges(
		// after UpdateNamespaceEgress()
		"namespace 42 dropped",
		// after UpdateHostSubnetEgress()
		"claim 172.17.0.100 on 172.17.0.3 for namespace 42",
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Adding the Egress IP to another namespace should not work and should cause both
	// namespaces to start dropping traffic.
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     43,
		EgressIPs: []string{"172.17.0.100"},
	})
	err = w.assertChanges(
		"release 172.17.0.100 on 172.17.0.3",
		"namespace 42 dropped",
		"namespace 43 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Auto-egress-IP assignment should ignore the IP while it is double-booked
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:      "172.17.0.5",
		EgressCIDRs: []string{"172.17.0.0/24"},
	})
	err = w.assertChanges(
		"update egress CIDRs",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
	allocation := eit.ReallocateEgressIPs()
	if node5ips, ok := allocation["node-5"]; !ok {
		t.Fatalf("Unexpected IP allocation: %#v", allocation)
	} else if len(node5ips) != 0 {
		t.Fatalf("Unexpected IP allocation: %#v", allocation)
	}
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:      "172.17.0.5",
		EgressCIDRs: []string{},
	})
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the duplicate should cause the original to start working again
	eit.DeleteNetNamespaceEgress(43)
	err = w.assertChanges(
		"claim 172.17.0.100 on 172.17.0.3 for namespace 42",
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
		"namespace 43 normal",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Add duplicate back, re-breaking it
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     43,
		EgressIPs: []string{"172.17.0.100"},
	})
	err = w.assertChanges(
		"release 172.17.0.100 on 172.17.0.3",
		"namespace 42 dropped",
		"namespace 43 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Now remove and re-add the Node EgressIP; the namespace should stay broken
	// whether the IP is assigned to a node or not.
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{},
	})
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100"},
	})
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the egress IP from the original namespace should result in it being
	// given to the "duplicate" namespace
	eit.DeleteNetNamespaceEgress(42)
	err = w.assertChanges(
		"claim 172.17.0.100 on 172.17.0.3 for namespace 43",
		"namespace 42 normal",
		"namespace 43 via 172.17.0.100 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestOfflineEgressIPs(t *testing.T) {
	eit, w := setupEgressIPTracker(t)

	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100"},
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.4",
		EgressIPs: []string{"172.17.0.101"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     42,
		EgressIPs: []string{"172.17.0.100", "172.17.0.101"},
	})
	err := w.assertChanges(
		"claim 172.17.0.100 on 172.17.0.3 for namespace 42",
		"claim 172.17.0.101 on 172.17.0.4 for namespace 42",
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// If the primary goes offline, fall back to the secondary
	eit.SetNodeOffline("172.17.0.3", true)
	err = w.assertChanges(
		"namespace 42 via 172.17.0.101 on 172.17.0.4",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// If the secondary also goes offline, then we lose
	eit.SetNodeOffline("172.17.0.4", true)
	err = w.assertChanges(
		"namespace 42 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// If the secondary comes back, use it
	eit.SetNodeOffline("172.17.0.4", false)
	err = w.assertChanges(
		"namespace 42 via 172.17.0.101 on 172.17.0.4",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// If the primary comes back, use it in preference to the secondary
	eit.SetNodeOffline("172.17.0.3", false)
	err = w.assertChanges(
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// If the secondary goes offline now, we don't care
	eit.SetNodeOffline("172.17.0.4", true)
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func updateAllocations(eit *EgressIPTracker, allocation map[string][]string) {
	for nodeName, egressIPs := range allocation {
		for _, node := range eit.nodesByNodeIP {
			if node.nodeName == nodeName {
				updateHostSubnetEgress(eit, &networkapi.HostSubnet{
					HostIP:      node.nodeIP,
					EgressIPs:   egressIPs,
					EgressCIDRs: node.requestedCIDRs.List(),
				})
				break
			}
		}
	}
}

func TestEgressCIDRAllocation(t *testing.T) {
	eit, w := setupEgressIPTracker(t)

	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:      "172.17.0.3",
		EgressIPs:   []string{},
		EgressCIDRs: []string{"172.17.0.100/32", "172.17.0.101/32", "172.17.0.102/32", "172.17.0.103/32", "172.17.1.0/24"},
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:      "172.17.0.4",
		EgressIPs:   []string{},
		EgressCIDRs: []string{"172.17.0.0/24"},
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:      "172.17.0.5",
		EgressIPs:   []string{},
		EgressCIDRs: []string{},
	})
	err := w.assertChanges(
		"update egress CIDRs",
		"update egress CIDRs",
		// no "update egress CIDRs" for node-5 since it has no EgressCIDRs
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Either of these could be assigned to either node, but they should be balanced
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     42,
		EgressIPs: []string{"172.17.0.100"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     43,
		EgressIPs: []string{"172.17.0.101"},
	})
	err = w.assertChanges(
		"namespace 42 dropped",
		"update egress CIDRs",
		"namespace 43 dropped",
		"update egress CIDRs",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	allocation := eit.ReallocateEgressIPs()
	node3ips := allocation["node-3"]
	node4ips := allocation["node-4"]
	if len(node3ips) != 1 || len(node4ips) != 1 {
		t.Fatalf("Bad IP allocation: %#v", allocation)
	}
	var n42, n43 string
	if node3ips[0] == "172.17.0.100" && node4ips[0] == "172.17.0.101" {
		n42 = "172.17.0.3"
		n43 = "172.17.0.4"
	} else if node3ips[0] == "172.17.0.101" && node4ips[0] == "172.17.0.100" {
		n42 = "172.17.0.4"
		n43 = "172.17.0.3"
	} else {
		t.Fatalf("Bad IP allocation: %#v", allocation)
	}

	updateAllocations(eit, allocation)
	err = w.assertChanges(
		fmt.Sprintf("claim 172.17.0.100 on %s for namespace 42", n42),
		fmt.Sprintf("namespace 42 via 172.17.0.100 on %s", n42),
		fmt.Sprintf("claim 172.17.0.101 on %s for namespace 43", n43),
		fmt.Sprintf("namespace 43 via 172.17.0.101 on %s", n43),
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// First can only be assigned to node3. Second *could* be assigned to either, but
	// must get assigned to node4 for balance
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     44,
		EgressIPs: []string{"172.17.1.1"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     45,
		EgressIPs: []string{"172.17.0.102"},
	})
	err = w.assertChanges(
		"namespace 44 dropped",
		"update egress CIDRs",
		"namespace 45 dropped",
		"update egress CIDRs",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	allocation = eit.ReallocateEgressIPs()
	updateAllocations(eit, allocation)
	err = w.assertChanges(
		"claim 172.17.1.1 on 172.17.0.3 for namespace 44",
		"namespace 44 via 172.17.1.1 on 172.17.0.3",
		"claim 172.17.0.102 on 172.17.0.4 for namespace 45",
		"namespace 45 via 172.17.0.102 on 172.17.0.4",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Manually assigning egress IPs to the node with no EgressCIDRs should have no
	// effect on automatic assignments (though it will result in a spurious "update
	// egress CIDRs" notification).
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:    "172.17.0.5",
		EgressIPs: []string{"172.17.2.100"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     50,
		EgressIPs: []string{"172.17.2.100"},
	})
	err = w.assertChanges(
		"claim 172.17.2.100 on 172.17.0.5 for namespace 50",
		"namespace 50 via 172.17.2.100 on 172.17.0.5",
		"update egress CIDRs",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
	allocation = eit.ReallocateEgressIPs()
	updateAllocations(eit, allocation)
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// First two can only be assigned to node4. Last must get assigned to node3 for balance
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     46,
		EgressIPs: []string{"172.17.0.200"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     47,
		EgressIPs: []string{"172.17.0.201"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     48,
		EgressIPs: []string{"172.17.0.103"},
	})
	err = w.assertChanges(
		"namespace 46 dropped",
		"update egress CIDRs",
		"namespace 47 dropped",
		"update egress CIDRs",
		"namespace 48 dropped",
		"update egress CIDRs",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	allocation = eit.ReallocateEgressIPs()
	updateAllocations(eit, allocation)
	err = w.assertChanges(
		"claim 172.17.0.200 on 172.17.0.4 for namespace 46",
		"namespace 46 via 172.17.0.200 on 172.17.0.4",
		"claim 172.17.0.201 on 172.17.0.4 for namespace 47",
		"namespace 47 via 172.17.0.201 on 172.17.0.4",
		"claim 172.17.0.103 on 172.17.0.3 for namespace 48",
		"namespace 48 via 172.17.0.103 on 172.17.0.3",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Dropping an Egress CIDR will drop the Egress IP(s) that came from that CIDR.
	// If we then reallocate, the dropped Egress IP(s) might be allocated to new nodes.
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:      "172.17.0.3",
		EgressIPs:   allocation["node-3"],
		EgressCIDRs: []string{"172.17.0.100/32", "172.17.0.101/32", "172.17.0.102/32", "172.17.1.0/24"}, // removed "172.17.0.103/32"
	})
	err = w.assertChanges(
		"update egress CIDRs",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
	allocation = eit.ReallocateEgressIPs()
	updateAllocations(eit, allocation)
	err = w.assertChanges(
		"release 172.17.0.103 on 172.17.0.3",
		"namespace 48 dropped",
		// Now that the egress IP has been unassigned, the tracker sees that it
		// could be assigned to a new node.
		"update egress CIDRs",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
	allocation = eit.ReallocateEgressIPs()
	updateAllocations(eit, allocation)
	err = w.assertChanges(
		"claim 172.17.0.103 on 172.17.0.4 for namespace 48",
		"namespace 48 via 172.17.0.103 on 172.17.0.4",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Changing/Removing the EgressIPs of a namespace should drop the old allocation and create a new one
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     46,
		EgressIPs: []string{"172.17.0.202"}, // was 172.17.0.200
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     44,
		EgressIPs: []string{}, // was 172.17.1.1
	})
	err = w.assertChanges(
		"release 172.17.0.200 on 172.17.0.4",
		"namespace 46 dropped",
		"update egress CIDRs",
		"release 172.17.1.1 on 172.17.0.3",
		"namespace 44 normal",
		"update egress CIDRs",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	allocation = eit.ReallocateEgressIPs()
	for _, nodeAllocation := range allocation {
		for _, ip := range nodeAllocation {
			if ip == "172.17.1.1" || ip == "172.17.0.200" {
				t.Fatalf("reallocation failed to drop unused egress IP %s: %#v", ip, allocation)
			}
		}
	}
	updateAllocations(eit, allocation)
	err = w.assertChanges(
		"claim 172.17.0.202 on 172.17.0.4 for namespace 46",
		"namespace 46 via 172.17.0.202 on 172.17.0.4",
		"update egress CIDRs",
		"update egress CIDRs",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// You can't mix multiple-egress-IP HA with auto-allocated-egress-IP HA
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     45,
		EgressIPs: []string{"172.17.0.102", "172.17.1.102"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     49,
		EgressIPs: []string{"172.17.0.109", "172.17.1.109"},
	})
	err = w.assertChanges(
		"update egress CIDRs",
		"update egress CIDRs",
		"namespace 49 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	allocation = eit.ReallocateEgressIPs()
	updateAllocations(eit, allocation)
	err = w.assertChanges(
		"release 172.17.0.102 on 172.17.0.4",
		"namespace 45 dropped",
		"update egress CIDRs",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestEgressNodeRenumbering(t *testing.T) {
	eit, w := setupEgressIPTracker(t)

	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		Host:      "alpha",
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100"},
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		Host:      "beta",
		HostIP:    "172.17.0.4",
		EgressIPs: []string{"172.17.0.101"},
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		Host:      "gamma",
		HostIP:    "172.17.0.5",
		EgressIPs: []string{"172.17.0.102"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     42,
		EgressIPs: []string{"172.17.0.100"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     43,
		EgressIPs: []string{"172.17.0.101"},
	})

	err := w.assertChanges(
		"claim 172.17.0.100 on 172.17.0.3 for namespace 42",
		"namespace 42 via 172.17.0.100 on 172.17.0.3",
		"claim 172.17.0.101 on 172.17.0.4 for namespace 43",
		"namespace 43 via 172.17.0.101 on 172.17.0.4",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Renumber one of the hosts
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		Host:      "beta",
		HostIP:    "172.17.0.6",
		EgressIPs: []string{"172.17.0.101"},
	})
	err = w.assertChanges(
		"release 172.17.0.101 on 172.17.0.4",
		"namespace 43 dropped",
		"claim 172.17.0.101 on 172.17.0.6 for namespace 43",
		"namespace 43 via 172.17.0.101 on 172.17.0.6",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestEgressCIDRAllocationOffline(t *testing.T) {
	eit, w := setupEgressIPTracker(t)

	// Create nodes...
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:      "172.17.0.3",
		EgressIPs:   []string{},
		EgressCIDRs: []string{"172.17.0.0/24", "172.17.1.0/24"},
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:      "172.17.0.4",
		EgressIPs:   []string{},
		EgressCIDRs: []string{"172.17.0.0/24"},
	})
	updateHostSubnetEgress(eit, &networkapi.HostSubnet{
		HostIP:      "172.17.0.5",
		EgressIPs:   []string{},
		EgressCIDRs: []string{"172.17.1.0/24"},
	})

	// Create namespaces
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     100,
		EgressIPs: []string{"172.17.0.100"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     101,
		EgressIPs: []string{"172.17.0.101"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     102,
		EgressIPs: []string{"172.17.0.102"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     200,
		EgressIPs: []string{"172.17.1.200"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     201,
		EgressIPs: []string{"172.17.1.201"},
	})
	updateNetNamespaceEgress(eit, &networkapi.NetNamespace{
		NetID:     202,
		EgressIPs: []string{"172.17.1.202"},
	})

	// In a perfect world, we'd get 2 IPs on each node, but depending on processing
	// order, this isn't guaranteed. Eg, if the three 172.17.0.x IPs get processed
	// first, we could get two of them on node-3 and one on node-4. Then the first two
	// 172.17.1.x IPs get assigned to node-5, and the last one could go to either
	// node-3 or node-5. Regardless of order, node-3 is guaranteed to get at least
	// two IPs since there's no way either node-4 or node-5 could be assigned a
	// third IP if node-3 still only had one.
	allocation := eit.ReallocateEgressIPs()
	node3ips := allocation["node-3"]
	node4ips := allocation["node-4"]
	node5ips := allocation["node-5"]
	if len(node3ips) < 2 || len(node4ips) == 0 || len(node5ips) == 0 ||
		len(node3ips)+len(node4ips)+len(node5ips) != 6 {
		t.Fatalf("Bad IP allocation: %#v", allocation)
	}
	updateAllocations(eit, allocation)

	w.flushChanges()

	// Now take node-3 offline
	eit.SetNodeOffline("172.17.0.3", true)
	err := w.assertUpdateEgressCIDRsNotification()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// First reallocation should empty out node-3
	allocation = eit.ReallocateEgressIPs()
	if node3ips, ok := allocation["node-3"]; !ok || len(node3ips) != 0 {
		t.Fatalf("Bad IP allocation: %#v", allocation)
	}
	updateAllocations(eit, allocation)

	err = w.assertUpdateEgressCIDRsNotification()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Next reallocation should reassign egress IPs to node-4 and node-5
	allocation = eit.ReallocateEgressIPs()
	node3ips = allocation["node-3"]
	node4ips = allocation["node-4"]
	node5ips = allocation["node-5"]
	if len(node3ips) != 0 || len(node4ips) != 3 || len(node5ips) != 3 {
		t.Fatalf("Bad IP allocation: %#v", allocation)
	}
	updateAllocations(eit, allocation)

	// Bring node-3 back
	eit.SetNodeOffline("172.17.0.3", false)
	err = w.assertUpdateEgressCIDRsNotification()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// First reallocation should remove some IPs from node-4 and node-5 but not add
	// them to node-3. As above, the "balanced" allocation we're aiming for may not
	// be perfect, but it has to be planning to assign at least 2 IPs to node-3.
	allocation = eit.ReallocateEgressIPs()
	node3ips = allocation["node-3"]
	node4ips = allocation["node-4"]
	node5ips = allocation["node-5"]
	if len(node3ips) != 0 || len(node4ips)+len(node5ips) > 4 {
		t.Fatalf("Bad IP allocation: %#v", allocation)
	}
	updateAllocations(eit, allocation)

	err = w.assertUpdateEgressCIDRsNotification()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Next reallocation should reassign the IPs that were removed in the previous
	// round. While the last reallocation was designed to aim for a next reallocation
	// where all of the removed IPs end up on node-3, it's possible 1 of them will
	// end up getting reallocated back to the same node it was removed from.
	allocation = eit.ReallocateEgressIPs()
	node3ips = allocation["node-3"]
	node4ips = allocation["node-4"]
	node5ips = allocation["node-5"]
	if len(node3ips) < 1 || len(node3ips) > 3 ||
		len(node4ips) < 1 || len(node4ips) > 3 ||
		len(node5ips) < 1 || len(node5ips) > 3 ||
		len(node3ips)+len(node4ips)+len(node5ips) != 6 {
		t.Fatalf("Bad IP allocation: %#v", allocation)
	}
	updateAllocations(eit, allocation)
}
