package vnidallocator

import (
	"errors"
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/service/allocator"

	"github.com/openshift/origin/pkg/sdn/registry/netnamespace/vnid"
)

// Interface manages the allocation of VNIDs out of a range.
// Interface should be threadsafe.
type Interface interface {
	Allocate(uint) error
	AllocateNext() (uint, error)
	Release(uint) error

	Has(uint) bool
}

var (
	ErrFull            = errors.New("range is full")
	ErrNotInRange      = errors.New("provided VNID is not in the valid range")
	ErrAllocated       = errors.New("provided VNID is already allocated")
	ErrMismatchedRange = errors.New("the provided VNID range does not match the current VNID range")
)

type Allocator struct {
	vnidRange vnid.VNIDRange

	alloc allocator.Interface
}

// Allocator implements Interface and Snapshottable
var _ Interface = &Allocator{}

// NewAllocatorCustom creates a Allocator over a VNID Range, calling allocatorFactory to construct the backing store.
func NewAllocatorCustom(vr vnid.VNIDRange, allocatorFactory allocator.AllocatorFactory) *Allocator {
	return &Allocator{
		vnidRange: vr,
		alloc:     allocatorFactory(int(vr.Size), vr.String()),
	}
}

// Helper that wraps NewAllocatorCustom, for creating a range backed by an in-memory store.
func NewInMemoryAllocator(vr vnid.VNIDRange) *Allocator {
	return NewAllocatorCustom(vr, allocator.NewContiguousAllocationInterface)
}

// Free returns the count of VNID left in the range.
func (r *Allocator) Free() int {
	return r.alloc.Free()
}

// Allocate attempts to reserve the provided VNID. ErrNotInRange or
// ErrAllocated will be returned if the VNID is not valid for this range
// or has already been reserved.
func (r *Allocator) Allocate(id uint) error {
	ok, offset := r.contains(id)
	if !ok {
		return ErrNotInRange
	}

	allocated, err := r.alloc.Allocate(int(offset))
	if err != nil {
		return err
	}
	if !allocated {
		return ErrAllocated
	}
	return nil
}

// AllocateNext reserves one of the VNIDs from the pool. ErrFull may
// be returned if there are no VNIDs left.
func (r *Allocator) AllocateNext() (uint, error) {
	offset, ok, err := r.alloc.AllocateNext()
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrFull
	}
	return r.vnidRange.Base + uint(offset), nil
}

// Release releases the VNID back to the pool. Releasing an
// unallocated VNID or a VNID out of the range is a no-op and
// returns no error.
func (r *Allocator) Release(id uint) error {
	ok, offset := r.contains(id)
	if !ok {
		return nil
	}
	return r.alloc.Release(int(offset))
}

// Has returns true if the provided VNID is already allocated and a call
// to Allocate(VNID) would fail with ErrAllocated.
func (r *Allocator) Has(id uint) bool {
	ok, offset := r.contains(id)
	if !ok {
		return false
	}
	return r.alloc.Has(int(offset))
}

// Snapshot saves the current state of the pool.
func (r *Allocator) Snapshot(dst *kapi.RangeAllocation) error {
	snapshottable, ok := r.alloc.(allocator.Snapshottable)
	if !ok {
		return fmt.Errorf("not a snapshottable allocator")
	}
	rangeString, data := snapshottable.Snapshot()
	dst.Range = rangeString
	dst.Data = data
	return nil
}

// Restore restores the pool to the previously captured state. ErrMismatchedRange
// is returned if the provided VNID range doesn't exactly match the previous range.
func (r *Allocator) Restore(vr vnid.VNIDRange, data []byte) error {
	if vr.String() != r.vnidRange.String() {
		return ErrMismatchedRange
	}
	snapshottable, ok := r.alloc.(allocator.Snapshottable)
	if !ok {
		return fmt.Errorf("not a snapshottable allocator")
	}
	return snapshottable.Restore(vr.String(), data)
}

// contains returns true and the offset if the VNID is in the range,
// and false and 0 otherwise.
func (r *Allocator) contains(id uint) (bool, uint) {
	if !r.vnidRange.Contains(id) {
		return false, 0
	}

	offset := id - r.vnidRange.Base
	return true, offset
}
