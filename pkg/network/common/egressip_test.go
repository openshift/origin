package common

import (
	"fmt"
	"testing"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
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

func setupEgressIPTracker(t *testing.T) (*EgressIPTracker, *testEIPWatcher) {
	watcher := &testEIPWatcher{}
	return NewEgressIPTracker(watcher), watcher
}

func TestEgressIP(t *testing.T) {
	eit, w := setupEgressIPTracker(t)

	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:   "node-3",
		HostIP: "172.17.0.3",
	})
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:   "node-4",
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
	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-42",
		NetID:     42,
		EgressIPs: []string{"172.17.0.100"},
	})
	err = w.assertChanges(
		"namespace 42 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
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
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100", "172.17.0.101"}, // Added .101
	})
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-5",
		HostIP:    "172.17.0.5",
		EgressIPs: []string{"172.17.0.105"},
	})
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-43",
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
	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-43",
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
	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-44",
		NetID:     44,
		EgressIPs: []string{"172.17.0.104"},
	})
	err = w.assertChanges(
		"namespace 44 dropped",
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-4",
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
	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-44",
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
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-4",
		HostIP:    "172.17.0.4",
		EgressIPs: []string{"172.17.0.102", "172.17.0.103"}, // Added .103, Dropped .104
	})
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-45",
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
	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-44",
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
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
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

	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-4",
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
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
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

	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-4",
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

	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-42",
		NetID:     42,
		EgressIPs: []string{"172.17.0.100"},
	})
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
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
	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-42",
		NetID:     42,
		EgressIPs: []string{"172.17.0.101", "172.17.0.100"},
	})
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Now assigning that IP to a node should switch OVS to use that since it's first in the list
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-4",
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
	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-42",
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
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-4",
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
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
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
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100"},
	})
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-4",
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
	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-43",
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

	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-44",
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

	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-42",
		NetID:     42,
		EgressIPs: []string{"172.17.0.100"},
	})
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
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
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-4",
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

	// Removing the duplicate node egressIP should restore traffic to the broken namespace
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-4",
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
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-5",
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

	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-42",
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
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
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

	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-42",
		NetID:     42,
		EgressIPs: []string{"172.17.0.100"},
	})
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
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
	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-43",
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
	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-43",
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
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
		HostIP:    "172.17.0.3",
		EgressIPs: []string{},
	})
	err = w.assertNoChanges()
	if err != nil {
		t.Fatalf("%v", err)
	}

	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
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

	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-3",
		HostIP:    "172.17.0.3",
		EgressIPs: []string{"172.17.0.100"},
	})
	eit.UpdateHostSubnetEgress(&networkapi.HostSubnet{
		Host:      "node-4",
		HostIP:    "172.17.0.4",
		EgressIPs: []string{"172.17.0.101"},
	})
	eit.UpdateNetNamespaceEgress(&networkapi.NetNamespace{
		NetName:   "ns-42",
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
