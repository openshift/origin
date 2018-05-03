package node

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Checks the "testModeChan" of eip and ensures that the expected netlink event(s)
// occurred, in some order (or rather, that the event(s) *would have occurred* if
// "testModeChan" wasn't set).
func assertNetlinkChange(eip *egressIPWatcher, expected ...string) error {
	actual := []string{}
	for range expected {
		select {
		case change := <-eip.testModeChan:
			actual = append(actual, change)
		default:
			break
		}
	}

	sort.Strings(expected)
	sort.Strings(actual)
	if reflect.DeepEqual(expected, actual) {
		return nil
	}
	return fmt.Errorf("Unexpected netlink changes: expected %#v, got %#v", expected, actual)
}

// Checks the "testModeChan" of eip and ensures that no netlink events have occurred
// since the last assertNetlinkChange() or assertNoNetlinkChanges() call.
func assertNoNetlinkChanges(eip *egressIPWatcher) error {
	select {
	case change := <-eip.testModeChan:
		return fmt.Errorf("Unexpected netlink change %q", change)
	default:
		return nil
	}
}

type egressTrafficType string

const (
	Normal  egressTrafficType = "normal"
	Dropped egressTrafficType = "dropped"
	Local   egressTrafficType = "local"
	Remote  egressTrafficType = "remote"
)

type egressOVSChange struct {
	vnid   uint32
	egress egressTrafficType
	remote string
}

// Takes the previous set of egress OVS flows, then fetches the current set and checks
// that the expected changes have occurred. Each namespace whose egress has changed should
// have an egressOVSChange struct describing the expected new state. On success, returns
// the new/current set of flows in flows.
func assertOVSChanges(eip *egressIPWatcher, flows *[]string, changes ...egressOVSChange) error {
	oldFlows := *flows
	newFlows, err := eip.oc.ovs.DumpFlows("table=100")
	if err != nil {
		return fmt.Errorf("unexpected error dumping OVS flows: %v", err)
	}

	flowChanges := []flowChange{}
	for _, change := range changes {
		vnidStr := fmt.Sprintf("reg0=%d", change.vnid)
		for _, flow := range *flows {
			if strings.Contains(flow, vnidStr) {
				flowChanges = append(flowChanges,
					flowChange{
						kind:  flowRemoved,
						match: []string{flow},
					},
				)
			}
		}

		switch change.egress {
		case Normal:
			break
		case Dropped:
			flowChanges = append(flowChanges,
				flowChange{
					kind:  flowAdded,
					match: []string{vnidStr, "drop"},
				},
			)
		case Local:
			flowChanges = append(flowChanges,
				flowChange{
					kind:  flowAdded,
					match: []string{vnidStr, fmt.Sprintf("%s->pkt_mark", getMarkForVNID(change.vnid, eip.masqueradeBit)), "goto_table:101"},
				},
			)
		case Remote:
			flowChanges = append(flowChanges,
				flowChange{
					kind:  flowAdded,
					match: []string{vnidStr, fmt.Sprintf("%s->tun_dst", change.remote)},
				},
			)
		}
	}
	err = assertFlowChanges(oldFlows, newFlows, flowChanges...)
	if err != nil {
		return fmt.Errorf("unexpected flow changes: %v\nOrig:\n%s\nNew:\n%s", err,
			strings.Join(oldFlows, "\n"), strings.Join(newFlows, "\n"))
	}

	*flows = newFlows
	return nil
}

// Checks that no OVS changes have occurred (relative to the provided old flows)
func assertNoOVSChanges(eip *egressIPWatcher, flows *[]string) error {
	return assertOVSChanges(eip, flows)
}

func setupEgressIPWatcher(t *testing.T) (*egressIPWatcher, []string) {
	_, oc, _ := setupOVSController(t)
	if oc.localIP != "172.17.0.4" {
		panic("details of fake ovsController changed")
	}
	masqBit := int32(0)
	eip := newEgressIPWatcher(oc, "172.17.0.4", &masqBit)
	eip.testModeChan = make(chan string, 10)

	flows, err := eip.oc.ovs.DumpFlows("table=100")
	if err != nil {
		t.Fatalf("unexpected error dumping OVS flows: %v", err)
	}

	return eip, flows
}

func TestEgressIP(t *testing.T) {
	eip, flows := setupEgressIPWatcher(t)

	eip.updateNodeEgress("172.17.0.3", []string{})
	eip.updateNodeEgress("172.17.0.4", []string{})
	eip.deleteNamespaceEgress(42)
	eip.deleteNamespaceEgress(43)

	// No namespaces use egress yet, so should be no changes
	err := assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertNoOVSChanges(eip, &flows)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Assign NetNamespace.EgressIP first, then HostSubnet.EgressIP, with a remote EgressIP
	eip.updateNamespaceEgress(42, []string{"172.17.0.100"})
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 42, egress: Dropped})
	if err != nil {
		t.Fatalf("%v", err)
	}

	eip.updateNodeEgress("172.17.0.3", []string{"172.17.0.100"}) // Added .100
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 42, egress: Remote, remote: "172.17.0.3"})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Assign HostSubnet.EgressIP first, then NetNamespace.EgressIP, with a remote EgressIP
	eip.updateNodeEgress("172.17.0.3", []string{"172.17.0.101", "172.17.0.100"}) // Added .101
	eip.updateNodeEgress("172.17.0.5", []string{"172.17.0.105"})                 // Added .105
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertNoOVSChanges(eip, &flows)
	if err != nil {
		t.Fatalf("%v", err)
	}

	eip.updateNamespaceEgress(43, []string{"172.17.0.105"})
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 43, egress: Remote, remote: "172.17.0.5"})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Change NetNamespace.EgressIP
	eip.updateNamespaceEgress(43, []string{"172.17.0.101"})
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 43, egress: Remote, remote: "172.17.0.3"})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Assign NetNamespace.EgressIP first, then HostSubnet.EgressIP, with a local EgressIP
	eip.updateNamespaceEgress(44, []string{"172.17.0.104"})
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 44, egress: Dropped})
	if err != nil {
		t.Fatalf("%v", err)
	}

	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.102", "172.17.0.104"}) // Added .102, .104
	err = assertNetlinkChange(eip, "claim 172.17.0.104")
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 44, egress: Local})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Change Namespace EgressIP
	eip.updateNamespaceEgress(44, []string{"172.17.0.102"})
	err = assertNetlinkChange(eip, "release 172.17.0.104", "claim 172.17.0.102")
	if err != nil {
		t.Fatalf("%v", err)
	}
	// The iptables rules change, but not the OVS flow
	err = assertNoOVSChanges(eip, &flows)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Assign HostSubnet.EgressIP first, then NetNamespace.EgressIP, with a local EgressIP
	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.102", "172.17.0.103"}) // Added .103, Dropped .104
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertNoOVSChanges(eip, &flows)
	if err != nil {
		t.Fatalf("%v", err)
	}

	eip.updateNamespaceEgress(45, []string{"172.17.0.103"})
	err = assertNetlinkChange(eip, "claim 172.17.0.103")
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 45, egress: Local})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Drop namespace EgressIP
	eip.deleteNamespaceEgress(44)
	err = assertNetlinkChange(eip, "release 172.17.0.102")
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 44, egress: Normal})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Add namespace EgressIP back again after having removed it...
	eip.updateNamespaceEgress(44, []string{"172.17.0.102"})
	err = assertNetlinkChange(eip, "claim 172.17.0.102")
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 44, egress: Local})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Drop remote node EgressIP
	eip.updateNodeEgress("172.17.0.3", []string{"172.17.0.100"}) // Dropped .101
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 43, egress: Dropped})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Drop local node EgressIP
	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.102"}) // Dropped .103
	err = assertNetlinkChange(eip, "release 172.17.0.103")
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 45, egress: Dropped})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Add them back, swapped
	eip.updateNodeEgress("172.17.0.3", []string{"172.17.0.100", "172.17.0.103"}) // Added .103
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 45, egress: Remote, remote: "172.17.0.3"})
	if err != nil {
		t.Fatalf("%v", err)
	}

	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.101", "172.17.0.102"}) // Added .101
	err = assertNetlinkChange(eip, "claim 172.17.0.101")
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 43, egress: Local})
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestMultipleNamespaceEgressIPs(t *testing.T) {
	eip, flows := setupEgressIPWatcher(t)

	eip.updateNamespaceEgress(42, []string{"172.17.0.100"})
	eip.updateNodeEgress("172.17.0.3", []string{"172.17.0.100"})
	err := assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Remote, remote: "172.17.0.3"},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Prepending a second, unavailable, namespace egress IP should block the working
	// one, because we still only support using the first EgressIP
	eip.updateNamespaceEgress(42, []string{"172.17.0.101", "172.17.0.100"})
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Dropped},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Assigning that IP to a node should now make it work
	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.101"})
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Local},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Swapping the order in the NetNamespace should bring the original Egress IP back
	eip.updateNamespaceEgress(42, []string{"172.17.0.100", "172.17.0.101"})
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Remote, remote: "172.17.0.3"},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the inactive egress IP from its node should have no effect
	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.200"})
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Remote, remote: "172.17.0.3"},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the remaining egress IP should now kill the namespace
	eip.updateNodeEgress("172.17.0.3", nil)
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Dropped},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Now add the egress IPs back...
	eip.updateNodeEgress("172.17.0.3", []string{"172.17.0.100"})
	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.101"})
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Remote, remote: "172.17.0.3"},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Assigning either the used or the unused Egress IP to another namespace should
	// break this namespace
	eip.updateNamespaceEgress(43, []string{"172.17.0.100"})
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Dropped},
		egressOVSChange{vnid: 43, egress: Dropped},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	eip.deleteNamespaceEgress(43)
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Remote, remote: "172.17.0.3"},
		egressOVSChange{vnid: 43, egress: Normal},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	eip.updateNamespaceEgress(44, []string{"172.17.0.101"})
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Dropped},
		egressOVSChange{vnid: 44, egress: Dropped},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	eip.deleteNamespaceEgress(44)
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Remote, remote: "172.17.0.3"},
		egressOVSChange{vnid: 44, egress: Normal},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestNodeIPAsEgressIP(t *testing.T) {
	eip, flows := setupEgressIPWatcher(t)

	// Trying to assign node IP as egress IP should fail. (It will log an error but this test doesn't notice that.)
	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.4", "172.17.0.102"})
	err := assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertNoOVSChanges(eip, &flows)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestDuplicateNodeEgressIPs(t *testing.T) {
	eip, flows := setupEgressIPWatcher(t)

	eip.updateNamespaceEgress(42, []string{"172.17.0.100"})
	eip.updateNodeEgress("172.17.0.3", []string{"172.17.0.100"})
	err := assertOVSChanges(eip, &flows, egressOVSChange{vnid: 42, egress: Remote, remote: "172.17.0.3"})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Adding the Egress IP to another node should not work and should cause the
	// namespace to start dropping traffic. (And in particular, even though we're
	// adding the Egress IP to the local node, there should not be a netlink change.)
	eip.updateNodeEgress("172.17.0.4", []string{"172.17.0.100"})
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 42, egress: Dropped})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the duplicate node egressIP should restore traffic to the broken namespace
	eip.updateNodeEgress("172.17.0.4", []string{})
	err = assertNoNetlinkChanges(eip)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 42, egress: Remote, remote: "172.17.0.3"})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// As above, but with a remote node IP
	eip.updateNodeEgress("172.17.0.5", []string{"172.17.0.100"})
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 42, egress: Dropped})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the egress IP from the namespace and then adding it back should result
	// in it still being broken.
	eip.deleteNamespaceEgress(42)
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 42, egress: Normal})
	if err != nil {
		t.Fatalf("%v", err)
	}

	eip.updateNamespaceEgress(42, []string{"172.17.0.100"})
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 42, egress: Dropped})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the original egress node should result in the "duplicate" egress node
	// now being used.
	eip.updateNodeEgress("172.17.0.3", []string{})
	err = assertOVSChanges(eip, &flows, egressOVSChange{vnid: 42, egress: Remote, remote: "172.17.0.5"})
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestDuplicateNamespaceEgressIPs(t *testing.T) {
	eip, flows := setupEgressIPWatcher(t)

	eip.updateNamespaceEgress(42, []string{"172.17.0.100"})
	eip.updateNodeEgress("172.17.0.3", []string{"172.17.0.100"})
	err := assertOVSChanges(eip, &flows, egressOVSChange{vnid: 42, egress: Remote, remote: "172.17.0.3"})
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Adding the Egress IP to another namespace should not work and should cause both
	// namespaces to start dropping traffic.
	eip.updateNamespaceEgress(43, []string{"172.17.0.100"})
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Dropped},
		egressOVSChange{vnid: 43, egress: Dropped},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the duplicate should cause the original to start working again
	eip.deleteNamespaceEgress(43)
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Remote, remote: "172.17.0.3"},
		egressOVSChange{vnid: 43, egress: Normal},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Add duplicate back, re-breaking it
	eip.updateNamespaceEgress(43, []string{"172.17.0.100"})
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Dropped},
		egressOVSChange{vnid: 43, egress: Dropped},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Now remove and re-add the Node EgressIP; the namespace should stay broken
	// whether the IP is assigned to a node or not, but processing the change might
	// cause the rules to get deleted and then added back in the opposite order,
	// which assertNoOVSChanges() would complain about, so we have to use
	// assertOVSChanges() instead.
	eip.updateNodeEgress("172.17.0.3", []string{})
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Dropped},
		egressOVSChange{vnid: 43, egress: Dropped},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	eip.updateNodeEgress("172.17.0.3", []string{"172.17.0.100"})
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Dropped},
		egressOVSChange{vnid: 43, egress: Dropped},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Removing the egress IP from the original namespace should result in it being
	// given to the "duplicate" namespace
	eip.deleteNamespaceEgress(42)
	err = assertOVSChanges(eip, &flows,
		egressOVSChange{vnid: 42, egress: Normal},
		egressOVSChange{vnid: 43, egress: Remote, remote: "172.17.0.3"},
	)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestMarkForVNID(t *testing.T) {
	testcases := []struct {
		description   string
		vnid          uint32
		masqueradeBit uint32
		result        uint32
	}{
		{
			description:   "masqBit in VNID range, but not set in VNID",
			vnid:          0x000000aa,
			masqueradeBit: 0x00000001,
			result:        0x000000aa,
		},
		{
			description:   "masqBit in VNID range, and set in VNID",
			vnid:          0x000000ab,
			masqueradeBit: 0x00000001,
			result:        0x010000aa,
		},
		{
			description:   "masqBit in VNID range, VNID 0",
			vnid:          0x00000000,
			masqueradeBit: 0x00000001,
			result:        0xff000000,
		},
		{
			description:   "masqBit outside of VNID range",
			vnid:          0x000000aa,
			masqueradeBit: 0x80000000,
			result:        0x000000aa,
		},
		{
			description:   "masqBit outside of VNID range, VNID 0",
			vnid:          0x00000000,
			masqueradeBit: 0x80000000,
			result:        0x7f000000,
		},
		{
			description:   "masqBit == bit 24",
			vnid:          0x000000aa,
			masqueradeBit: 0x01000000,
			result:        0x000000aa,
		},
		{
			description:   "masqBit == bit 24, VNID 0",
			vnid:          0x00000000,
			masqueradeBit: 0x01000000,
			result:        0xfe000000,
		},
		{
			description:   "no masqBit, ordinary VNID",
			vnid:          0x000000aa,
			masqueradeBit: 0x00000000,
			result:        0x000000aa,
		},
		{
			description:   "no masqBit, VNID 0",
			vnid:          0x00000000,
			masqueradeBit: 0x00000000,
			result:        0xff000000,
		},
	}

	for _, tc := range testcases {
		result := getMarkForVNID(tc.vnid, tc.masqueradeBit)
		if result != fmt.Sprintf("0x%08x", tc.result) {
			t.Fatalf("test %q expected %08x got %s", tc.description, tc.result, result)
		}
	}
}
