package plugin

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/util/ovs"

	kapi "k8s.io/kubernetes/pkg/api"
	kapiunversioned "k8s.io/kubernetes/pkg/api/unversioned"
)

func setup(t *testing.T) (ovs.Interface, *ovsController, []string) {
	ovsif := ovs.NewFake(BR)
	oc := NewOVSController(ovsif, 0)
	err := oc.SetupOVS("10.128.0.0/14", "172.30.0.0/16", "10.128.0.0/23", "10.128.0.1")
	if err != nil {
		t.Fatalf("Unexpected error setting up OVS: %v", err)
	}

	origFlows, err := ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}

	return ovsif, oc, origFlows
}

type flowChangeKind string

const (
	flowAdded   flowChangeKind = "added"
	flowRemoved flowChangeKind = "removed"
)

type flowChange struct {
	kind    flowChangeKind
	match   []string
	noMatch []string
}

// assertFlowChanges asserts that origFlows and newFlows differ in the ways described by
// changes, which consists of a series of flows that have been removed from origFlows or
// added to newFlows. There must be exactly 1 matching flow that contains all of the
// strings in match and none of the strings in noMatch.
func assertFlowChanges(origFlows, newFlows []string, changes ...flowChange) error {
	// copy to avoid modifying originals
	dup := make([]string, 0, len(origFlows))
	origFlows = append(dup, origFlows...)
	dup = make([]string, 0, len(newFlows))
	newFlows = append(dup, newFlows...)

	for _, change := range changes {
		var modFlows *[]string
		if change.kind == flowAdded {
			modFlows = &newFlows
		} else {
			modFlows = &origFlows
		}

		matchIndex := -1
		for i, flow := range *modFlows {
			matches := true
			for _, match := range change.match {
				if !strings.Contains(flow, match) {
					matches = false
					break
				}
			}
			for _, nonmatch := range change.noMatch {
				if strings.Contains(flow, nonmatch) {
					matches = false
					break
				}
			}
			if matches {
				if matchIndex == -1 {
					matchIndex = i
				} else {
					return fmt.Errorf("multiple %s flows matching %#v", string(change.kind), change.match)
				}
			}
		}
		if matchIndex == -1 {
			return fmt.Errorf("no %s flow matching %#v", string(change.kind), change.match)
		}
		*modFlows = append((*modFlows)[:matchIndex], (*modFlows)[matchIndex+1:]...)
	}

	if !reflect.DeepEqual(origFlows, newFlows) {
		return fmt.Errorf("unexpected additional changes to flows")
	}
	return nil
}

func TestOVSHostSubnet(t *testing.T) {
	ovsif, oc, origFlows := setup(t)

	hs := osapi.HostSubnet{
		TypeMeta: kapiunversioned.TypeMeta{
			Kind: "HostSubnet",
		},
		ObjectMeta: kapi.ObjectMeta{
			Name: "node2",
		},
		Host:   "node2",
		HostIP: "192.168.1.2",
		Subnet: "10.129.0.0/23",
	}
	err := oc.AddHostSubnetRules(&hs)
	if err != nil {
		t.Fatalf("Unexpected error adding HostSubnet rules: %v", err)
	}

	flows, err := ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:  flowAdded,
			match: []string{"table=10", "tun_src=192.168.1.2"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=50", "arp", "nw_dst=10.129.0.0/23", "192.168.1.2->tun_dst"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=90", "ip", "nw_dst=10.129.0.0/23", "192.168.1.2->tun_dst"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %v\nNew: %v", err, origFlows, flows)
	}

	err = oc.DeleteHostSubnetRules(&hs)
	if err != nil {
		t.Fatalf("Unexpected error deleting HostSubnet rules: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows) // no changes

	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %v\nNew: %v", err, origFlows, flows)
	}
}

func TestOVSService(t *testing.T) {
	ovsif, oc, origFlows := setup(t)

	svc := kapi.Service{
		TypeMeta: kapiunversioned.TypeMeta{
			Kind: "Service",
		},
		ObjectMeta: kapi.ObjectMeta{
			Name: "service",
		},
		Spec: kapi.ServiceSpec{
			ClusterIP: "172.30.99.99",
			Ports: []kapi.ServicePort{
				{Protocol: kapi.ProtocolTCP, Port: 80},
				{Protocol: kapi.ProtocolTCP, Port: 443},
			},
		},
	}
	err := oc.AddServiceRules(&svc, 42)
	if err != nil {
		t.Fatalf("Unexpected error adding service rules: %v", err)
	}

	flows, err := ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows,
		flowChange{
			kind:    flowAdded,
			match:   []string{"table=60", "ip_frag", "42->NXM_NX_REG1"},
			noMatch: []string{"tcp"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=60", "nw_dst=172.30.99.99", "tcp_dst=80", "42->NXM_NX_REG1"},
		},
		flowChange{
			kind:  flowAdded,
			match: []string{"table=60", "nw_dst=172.30.99.99", "tcp_dst=443", "42->NXM_NX_REG1"},
		},
	)
	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %v\nNew: %v", err, origFlows, flows)
	}

	err = oc.DeleteServiceRules(&svc)
	if err != nil {
		t.Fatalf("Unexpected error deleting service rules: %v", err)
	}
	flows, err = ovsif.DumpFlows()
	if err != nil {
		t.Fatalf("Unexpected error dumping flows: %v", err)
	}
	err = assertFlowChanges(origFlows, flows) // no changes

	if err != nil {
		t.Fatalf("Unexpected flow changes: %v\nOrig: %v\nNew: %v", err, origFlows, flows)
	}
}
