package netid

import (
	"strconv"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

func TestAllocate(t *testing.T) {
	nr, err := NewNetIDRange(201, 300)
	if err != nil {
		t.Fatal(err)
	}
	r := NewInMemory(nr)
	if f := r.Free(); f != 100 {
		t.Errorf("unexpected free %d", f)
	}

	// Test AllocateNext()
	found := sets.NewString()
	count := 0
	for r.Free() > 0 {
		netid, err := r.AllocateNext()
		if err != nil {
			t.Fatalf("error @ %d: %v", count, err)
		}
		count++
		if ok, _ := nr.Contains(netid); !ok {
			t.Fatalf("allocated %d which is outside of %v", netid, nr)
		}
		netidString := strconv.Itoa(int(netid))
		if found.Has(netidString) {
			t.Fatalf("allocated %d twice @ %d", netid, count)
		}
		found.Insert(netidString)
	}
	if count != 100 {
		t.Fatal("failed to allocate all netids in the given range")
	}
	if _, err := r.AllocateNext(); err != ErrFull {
		t.Fatal(err)
	}

	// Test Release()
	released := uint32(210)
	if err := r.Release(released); err != nil {
		t.Fatal(err)
	}
	if f := r.Free(); f != 1 {
		t.Errorf("unexpected free %d", f)
	}
	netid, err := r.AllocateNext()
	if err != nil {
		t.Fatal(err)
	}
	if released != netid {
		t.Errorf("unexpected %d : %d", netid, released)
	}

	// Test Allocate()
	if err := r.Release(released); err != nil {
		t.Fatal(err)
	}
	if err := r.Allocate(1); err != ErrNotInRange {
		t.Fatal(err)
	}
	if err := r.Allocate(201); err != ErrAllocated {
		t.Fatal(err)
	}
	if err := r.Allocate(301); err != ErrNotInRange {
		t.Fatal(err)
	}
	if err := r.Allocate(500); err != ErrNotInRange {
		t.Fatal(err)
	}
	if f := r.Free(); f != 1 {
		t.Errorf("unexpected free %d", f)
	}
	if err := r.Allocate(released); err != nil {
		t.Fatal(err)
	}
	if f := r.Free(); f != 0 {
		t.Errorf("unexpected free %d", f)
	}
}
