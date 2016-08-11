package plugin

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	osapi "github.com/openshift/origin/pkg/sdn/api"
)

func TestMasterVNIDMap(t *testing.T) {
	vmap := newMasterVNIDMap()

	// empty vmap
	checkCurrentVNIDs(t, vmap, 0, 0)

	// assign vnids
	_, _, err := vmap.allocateNetID(kapi.NamespaceDefault)
	checkNoErr(t, err)
	_, exists, err := vmap.allocateNetID("alpha")
	checkNoErr(t, err)
	if exists {
		t.Fatalf("expected netid not to exists")
	}
	_, exists, err = vmap.allocateNetID("alpha")
	checkNoErr(t, err)
	if !exists {
		t.Fatalf("expected allocated netid to exists")
	}
	_, _, err = vmap.allocateNetID("bravo")
	checkNoErr(t, err)
	_, _, err = vmap.allocateNetID("charlie")
	checkNoErr(t, err)
	checkCurrentVNIDs(t, vmap, 4, 3)

	// update vnids
	_, err = vmap.updateNetID("alpha", osapi.JoinPodNetwork, "bravo")
	checkNoErr(t, err)
	_, err = vmap.updateNetID("alpha", osapi.JoinPodNetwork, "bogus")
	checkErr(t, err)
	_, err = vmap.updateNetID("bogus", osapi.JoinPodNetwork, "alpha")
	checkErr(t, err)
	checkCurrentVNIDs(t, vmap, 4, 2)

	_, err = vmap.updateNetID("alpha", osapi.GlobalPodNetwork, "")
	checkNoErr(t, err)
	_, err = vmap.updateNetID("charlie", osapi.GlobalPodNetwork, "")
	checkNoErr(t, err)
	_, err = vmap.updateNetID("bogus", osapi.GlobalPodNetwork, "")
	checkErr(t, err)
	checkCurrentVNIDs(t, vmap, 4, 1)

	_, err = vmap.updateNetID("alpha", osapi.IsolatePodNetwork, "")
	checkNoErr(t, err)
	_, err = vmap.updateNetID("bravo", osapi.IsolatePodNetwork, "")
	checkNoErr(t, err)
	_, err = vmap.updateNetID("bogus", osapi.IsolatePodNetwork, "")
	checkErr(t, err)
	checkCurrentVNIDs(t, vmap, 4, 2)

	// release vnids
	checkNoErr(t, vmap.releaseNetID("alpha"))
	checkNoErr(t, vmap.releaseNetID("bravo"))
	checkNoErr(t, vmap.releaseNetID("charlie"))
	checkNoErr(t, vmap.releaseNetID(kapi.NamespaceDefault))
	checkErr(t, vmap.releaseNetID("bogus"))
	checkCurrentVNIDs(t, vmap, 0, 0)
}

func checkNoErr(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func checkErr(t *testing.T, err error) {
	if err == nil {
		t.Fatalf("Expected error is missing!")
	}
}

func checkCurrentVNIDs(t *testing.T, vmap *masterVNIDMap, expectedMapCount, expectedAllocatorCount int) {
	if len(vmap.ids) != expectedMapCount {
		t.Fatalf("Wrong number of VNIDs: %d vs %d", len(vmap.ids), expectedMapCount)
	}

	// Check bitmap allocator
	expected_free := int(osapi.MaxVNID-osapi.MinVNID) + 1 - expectedAllocatorCount
	if vmap.netIDManager.Free() != expected_free {
		t.Fatalf("Allocator mismatch: %d vs %d", vmap.netIDManager.Free(), expected_free)
	}
	for _, id := range vmap.ids {
		if id == osapi.GlobalVNID {
			continue
		}
		if !vmap.netIDManager.Has(id) {
			t.Fatalf("Missing VNID: %d", id)
		}
	}
}
