package mcsallocator

import (
	"errors"
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/service/allocator"

	"github.com/openshift/origin/pkg/security/mcs"
)

// Interface manages the allocation of ports out of a range. Interface
// should be threadsafe.
type Interface interface {
	Allocate(*mcs.Label) error
	AllocateNext() (*mcs.Label, error)
	Release(*mcs.Label) error
}

var (
	ErrFull            = errors.New("range is full")
	ErrNotInRange      = errors.New("provided label is not in the valid range")
	ErrAllocated       = errors.New("provided label is already allocated")
	ErrMismatchedRange = errors.New("the provided label does not match the current label range")
)

type Allocator struct {
	r     *mcs.Range
	alloc allocator.Interface
}

// Allocator implements Interface and Snapshottable
var _ Interface = &Allocator{}

// New creates a Allocator over a UID range, calling factory to construct the backing store.
func New(r *mcs.Range, factory allocator.AllocatorFactory) *Allocator {
	return &Allocator{
		r:     r,
		alloc: factory(int(r.Size()), r.String()),
	}
}

// Free returns the count of port left in the range.
func (r *Allocator) Free() int {
	return r.alloc.Free()
}

// Allocate attempts to reserve the provided label. ErrNotInRange or
// ErrAllocated will be returned if the label is not valid for this range
// or has already been reserved.  ErrFull will be returned if there
// are no labels left.
func (r *Allocator) Allocate(label *mcs.Label) error {
	ok, offset := r.contains(label)
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

// AllocateNext reserves one of the labels from the pool. ErrFull may
// be returned if there are no labels left.
func (r *Allocator) AllocateNext() (*mcs.Label, error) {
	offset, ok, err := r.alloc.AllocateNext()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrFull
	}
	label, ok := r.r.LabelAt(uint64(offset))
	if !ok {
		return nil, ErrNotInRange
	}
	return label, nil
}

// Release releases the port back to the pool. Releasing an
// unallocated port or a port out of the range is a no-op and
// returns no error.
func (r *Allocator) Release(label *mcs.Label) error {
	ok, offset := r.contains(label)
	if !ok {
		// TODO: log a warning
		return nil
	}

	return r.alloc.Release(int(offset))
}

// Has returns true if the provided port is already allocated and a call
// to Allocate(label) would fail with ErrAllocated.
func (r *Allocator) Has(label *mcs.Label) bool {
	ok, offset := r.contains(label)
	if !ok {
		return false
	}

	return r.alloc.Has(int(offset))
}

// Snapshot saves the current state of the pool.
func (r *Allocator) Snapshot(dst *api.RangeAllocation) error {
	snapshottable, ok := r.alloc.(allocator.Snapshottable)
	if !ok {
		return fmt.Errorf("not a snapshottable allocator")
	}
	rangeString, data := snapshottable.Snapshot()
	dst.Range = rangeString
	dst.Data = data
	return nil
}

// Restore restores the pool to the previously captured state. ErrMismatchedNetwork
// is returned if the provided port range doesn't exactly match the previous range.
func (r *Allocator) Restore(into *mcs.Range, data []byte) error {
	if into.String() != r.r.String() {
		return ErrMismatchedRange
	}
	snapshottable, ok := r.alloc.(allocator.Snapshottable)
	if !ok {
		return fmt.Errorf("not a snapshottable allocator")
	}
	return snapshottable.Restore(into.String(), data)
}

// contains returns true and the offset if the label is in the range (and aligned), and false
// and nil otherwise.
func (r *Allocator) contains(label *mcs.Label) (bool, uint64) {
	return r.r.Offset(label)
}
