package mcsallocator

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/registry/service/allocator"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/security/mcs"
)

func TestAllocate(t *testing.T) {
	ranger, _ := mcs.NewRange("s0:", 5, 2)
	r := New(ranger, func(max int, rangeSpec string) allocator.Interface {
		return allocator.NewContiguousAllocationMap(max, rangeSpec)
	})
	if f := r.Free(); f != 10 {
		t.Errorf("unexpected free %d", f)
	}
	found := sets.NewString()
	count := 0
	for r.Free() > 0 {
		label, err := r.AllocateNext()
		if err != nil {
			t.Fatalf("error @ %d: %v", count, err)
		}
		count++
		if !ranger.Contains(label) {
			t.Fatalf("allocated %s which is outside of %s", label, ranger)
		}
		if found.Has(label.String()) {
			t.Fatalf("allocated %s twice @ %d", label, count)
		}
		found.Insert(label.String())
	}
	if _, err := r.AllocateNext(); err != ErrFull {
		t.Fatal(err)
	}

	released, _ := ranger.LabelAt(3)
	if err := r.Release(released); err != nil {
		t.Fatal(err)
	}
	if f := r.Free(); f != 1 {
		t.Errorf("unexpected free %d", f)
	}
	label, err := r.AllocateNext()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(released, label) {
		t.Errorf("unexpected %s : %s", label, released)
	}

	if err := r.Release(released); err != nil {
		t.Fatal(err)
	}
	badLabel, _ := ranger.LabelAt(30)
	if err := r.Allocate(badLabel); err != ErrNotInRange {
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
