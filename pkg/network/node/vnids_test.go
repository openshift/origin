// +build linux

package node

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/network"
)

func TestNodeVNIDMap(t *testing.T) {
	vmap := newNodeVNIDMap(nil, nil)

	// empty vmap

	checkNotExists(t, vmap, "alpha")
	checkNamespaces(t, vmap, 1, []string{})
	checkAllocatedVNIDs(t, vmap, []uint32{})

	// set vnids, non-overlapping

	vmap.setVNID("alpha", 1, false)
	vmap.setVNID("bravo", 2, false)
	vmap.setVNID("charlie", 3, false)
	vmap.setVNID("delta", 4, false)

	checkExists(t, vmap, "alpha", 1)
	checkExists(t, vmap, "bravo", 2)
	checkExists(t, vmap, "charlie", 3)
	checkExists(t, vmap, "delta", 4)
	checkNotExists(t, vmap, "echo")

	checkNamespaces(t, vmap, 1, []string{"alpha"})
	checkNamespaces(t, vmap, 2, []string{"bravo"})
	checkNamespaces(t, vmap, 3, []string{"charlie"})
	checkNamespaces(t, vmap, 4, []string{"delta"})

	checkAllocatedVNIDs(t, vmap, []uint32{1, 2, 3, 4})

	// unset vnids

	id, err := vmap.unsetVNID("alpha")
	if id != 1 || err != nil {
		t.Fatalf("Unexpected failure: %d, %v", id, err)
	}
	id, err = vmap.unsetVNID("charlie")
	if id != 3 || err != nil {
		t.Fatalf("Unexpected failure: %d, %v", id, err)
	}

	checkNotExists(t, vmap, "alpha")
	checkExists(t, vmap, "bravo", 2)
	checkNotExists(t, vmap, "charlie")
	checkExists(t, vmap, "delta", 4)

	id, err = vmap.unsetVNID("alpha")
	if err == nil {
		t.Fatalf("Unexpected success: %d", id)
	}
	id, err = vmap.unsetVNID("echo")
	if err == nil {
		t.Fatalf("Unexpected success: %d", id)
	}

	checkNamespaces(t, vmap, 1, []string{})
	checkNamespaces(t, vmap, 2, []string{"bravo"})
	checkNamespaces(t, vmap, 3, []string{})
	checkNamespaces(t, vmap, 4, []string{"delta"})

	checkAllocatedVNIDs(t, vmap, []uint32{2, 4})

	// change vnids

	vmap.setVNID("bravo", 1, false)
	vmap.setVNID("delta", 2, false)

	checkExists(t, vmap, "bravo", 1)
	checkExists(t, vmap, "delta", 2)

	checkNamespaces(t, vmap, 1, []string{"bravo"})
	checkNamespaces(t, vmap, 2, []string{"delta"})
	checkNamespaces(t, vmap, 3, []string{})
	checkNamespaces(t, vmap, 4, []string{})

	checkAllocatedVNIDs(t, vmap, []uint32{1, 2})

	// overlapping vnids

	vmap.setVNID("echo", 3, false)
	vmap.setVNID("foxtrot", 5, false)
	vmap.setVNID("golf", 1, false)
	vmap.setVNID("hotel", 1, false)
	vmap.setVNID("india", 1, false)
	vmap.setVNID("juliet", 3, false)

	checkExists(t, vmap, "bravo", 1)
	checkExists(t, vmap, "delta", 2)
	checkExists(t, vmap, "echo", 3)
	checkExists(t, vmap, "foxtrot", 5)
	checkExists(t, vmap, "golf", 1)
	checkExists(t, vmap, "hotel", 1)
	checkExists(t, vmap, "india", 1)
	checkExists(t, vmap, "juliet", 3)

	checkNamespaces(t, vmap, 1, []string{"bravo", "golf", "hotel", "india"})
	checkNamespaces(t, vmap, 2, []string{"delta"})
	checkNamespaces(t, vmap, 3, []string{"echo", "juliet"})
	checkNamespaces(t, vmap, 4, []string{})
	checkNamespaces(t, vmap, 5, []string{"foxtrot"})

	checkAllocatedVNIDs(t, vmap, []uint32{1, 2, 3, 5})

	// deleting with overlapping vnids

	id, err = vmap.unsetVNID("golf")
	if err != nil {
		t.Fatalf("Unexpected failure: %d, %v", id, err)
	}
	id, err = vmap.unsetVNID("echo")
	if err != nil {
		t.Fatalf("Unexpected failure: %d, %v", id, err)
	}
	id, err = vmap.unsetVNID("juliet")
	if err != nil {
		t.Fatalf("Unexpected failure: %d, %v", id, err)
	}

	checkExists(t, vmap, "bravo", 1)
	checkExists(t, vmap, "delta", 2)
	checkNotExists(t, vmap, "echo")
	checkExists(t, vmap, "foxtrot", 5)
	checkNotExists(t, vmap, "golf")
	checkExists(t, vmap, "hotel", 1)
	checkExists(t, vmap, "india", 1)
	checkNotExists(t, vmap, "juliet")

	checkNamespaces(t, vmap, 1, []string{"bravo", "hotel", "india"})
	checkNamespaces(t, vmap, 2, []string{"delta"})
	checkNamespaces(t, vmap, 3, []string{})
	checkNamespaces(t, vmap, 4, []string{})
	checkNamespaces(t, vmap, 5, []string{"foxtrot"})

	checkAllocatedVNIDs(t, vmap, []uint32{1, 2, 5})
}

func checkExists(t *testing.T, vmap *nodeVNIDMap, name string, expected uint32) {
	id, err := vmap.getVNID(name)
	if id != expected || err != nil {
		t.Fatalf("Unexpected failure: %d, %v", id, err)
	}
}

func checkNotExists(t *testing.T, vmap *nodeVNIDMap, name string) {
	id, err := vmap.getVNID(name)
	if err == nil {
		t.Fatalf("Unexpected success: %d", id)
	}
}

func checkNamespaces(t *testing.T, vmap *nodeVNIDMap, vnid uint32, match []string) {
	namespaces := vmap.GetNamespaces(vnid)
	if len(namespaces) != len(match) {
		t.Fatalf("Wrong number of namespaces: %v vs %v", namespaces, match)
	}
	for _, m := range match {
		found := false
		for _, n := range namespaces {
			if n == m {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Missing namespace: %s", m)
		}
	}
}

func checkAllocatedVNIDs(t *testing.T, vmap *nodeVNIDMap, match []uint32) {
	ids := []uint32{}
	idSet := sets.Int{}
	for _, id := range vmap.ids {
		if id != network.GlobalVNID {
			if !idSet.Has(int(id)) {
				ids = append(ids, id)
				idSet.Insert(int(id))
			}
		}
	}
	if len(ids) != len(match) {
		t.Fatalf("Wrong number of VNIDs: %d vs %d", len(ids), len(match))
	}

	for _, m := range match {
		found := false
		for _, n := range ids {
			if n == m {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Missing VNID: %d", m)
		}
	}
}
