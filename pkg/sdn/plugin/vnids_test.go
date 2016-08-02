package plugin

import (
	"testing"
)

func TestVnidMap(t *testing.T) {
	vmap := newVnidMap()

	// empty vmap

	checkNotExists(t, vmap, "alpha")
	checkNamespaces(t, vmap, 1, []string{})
	checkAllocatedVNIDs(t, vmap, []uint32{})

	// set vnids, non-overlapping

	vmap.SetVNID("alpha", 1)
	vmap.SetVNID("bravo", 2)
	vmap.SetVNID("charlie", 3)
	vmap.SetVNID("delta", 4)

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

	id, err := vmap.UnsetVNID("alpha")
	if id != 1 || err != nil {
		t.Fatalf("Unexpected failure: %d, %v", id, err)
	}
	id, err = vmap.UnsetVNID("charlie")
	if id != 3 || err != nil {
		t.Fatalf("Unexpected failure: %d, %v", id, err)
	}

	checkNotExists(t, vmap, "alpha")
	checkExists(t, vmap, "bravo", 2)
	checkNotExists(t, vmap, "charlie")
	checkExists(t, vmap, "delta", 4)

	id, err = vmap.UnsetVNID("alpha")
	if err == nil {
		t.Fatalf("Unexpected success: %d", id)
	}
	id, err = vmap.UnsetVNID("echo")
	if err == nil {
		t.Fatalf("Unexpected success: %d", id)
	}

	checkNamespaces(t, vmap, 1, []string{})
	checkNamespaces(t, vmap, 2, []string{"bravo"})
	checkNamespaces(t, vmap, 3, []string{})
	checkNamespaces(t, vmap, 4, []string{"delta"})

	checkAllocatedVNIDs(t, vmap, []uint32{2, 4})

	// change vnids

	vmap.SetVNID("bravo", 1)
	vmap.SetVNID("delta", 2)

	checkExists(t, vmap, "bravo", 1)
	checkExists(t, vmap, "delta", 2)

	checkNamespaces(t, vmap, 1, []string{"bravo"})
	checkNamespaces(t, vmap, 2, []string{"delta"})
	checkNamespaces(t, vmap, 3, []string{})
	checkNamespaces(t, vmap, 4, []string{})

	checkAllocatedVNIDs(t, vmap, []uint32{1, 2})

	// overlapping vnids

	vmap.SetVNID("echo", 3)
	vmap.SetVNID("foxtrot", 5)
	vmap.SetVNID("golf", 1)
	vmap.SetVNID("hotel", 1)
	vmap.SetVNID("india", 1)
	vmap.SetVNID("juliet", 3)

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

	id, err = vmap.UnsetVNID("golf")
	if err != nil {
		t.Fatalf("Unexpected failure: %d, %v", id, err)
	}
	id, err = vmap.UnsetVNID("echo")
	if err != nil {
		t.Fatalf("Unexpected failure: %d, %v", id, err)
	}
	id, err = vmap.UnsetVNID("juliet")
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

func checkExists(t *testing.T, vmap *vnidMap, name string, expected uint32) {
	id, err := vmap.GetVNID(name)
	if id != expected || err != nil {
		t.Fatalf("Unexpected failure: %d, %v", id, err)
	}
	if !vmap.CheckVNID(id) {
		t.Fatalf("Unexpected failure")
	}
}

func checkNotExists(t *testing.T, vmap *vnidMap, name string) {
	id, err := vmap.GetVNID(name)
	if err == nil {
		t.Fatalf("Unexpected success: %d", id)
	}
}

func checkNamespaces(t *testing.T, vmap *vnidMap, vnid uint32, match []string) {
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

func checkAllocatedVNIDs(t *testing.T, vmap *vnidMap, match []uint32) {
	vnids := vmap.GetAllocatedVNIDs()
	if len(vnids) != len(match) {
		t.Fatalf("Wrong number of VNIDs: %v vs %v", vnids, match)
	}
	for _, m := range match {
		found := false
		for _, n := range vnids {
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
