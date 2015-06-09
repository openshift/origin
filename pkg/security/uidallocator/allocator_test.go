package uidallocator

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/service/allocator"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/security/uid"
)

func TestAllocate(t *testing.T) {
	ranger, _ := uid.NewRange(0, 9, 2)
	r := New(ranger, allocator.NewContiguousAllocationInterface)
	if f := r.Free(); f != 5 {
		t.Errorf("unexpected free %d", f)
	}
	found := util.NewStringSet()
	count := 0
	for r.Free() > 0 {
		block, err := r.AllocateNext()
		if err != nil {
			t.Fatalf("error @ %d: %v", count, err)
		}
		count++
		if !ranger.Contains(block) {
			t.Fatalf("allocated %s which is outside of %s", block, ranger)
		}
		if found.Has(block.String()) {
			t.Fatalf("allocated %s twice @ %d", block, count)
		}
		found.Insert(block.String())
	}
	if _, err := r.AllocateNext(); err != ErrFull {
		t.Fatal(err)
	}

	released := uid.Block{2, 3}
	if err := r.Release(released); err != nil {
		t.Fatal(err)
	}
	if f := r.Free(); f != 1 {
		t.Errorf("unexpected free %d", f)
	}
	block, err := r.AllocateNext()
	if err != nil {
		t.Fatal(err)
	}
	if released != block {
		t.Errorf("unexpected %s : %s", block, released)
	}

	if err := r.Release(released); err != nil {
		t.Fatal(err)
	}
	if err := r.Allocate(uid.Block{11, 11}); err != ErrNotInRange {
		t.Fatal(err)
	}
	if err := r.Allocate(uid.Block{8, 11}); err != ErrNotInRange {
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
