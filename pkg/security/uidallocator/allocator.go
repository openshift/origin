package uidallocator

import (
	"errors"
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/service/allocator"

	"github.com/openshift/origin/pkg/security/uid"
)

// Interface manages the allocation of ports out of a range. Interface
// should be threadsafe.
type Interface interface {
	Allocate(uid.Block) error
	AllocateNext() (uid.Block, error)
	Release(uid.Block) error
}

var (
	ErrFull            = errors.New("range is full")
	ErrNotInRange      = errors.New("provided UID range is not in the valid range")
	ErrAllocated       = errors.New("provided UID range is already allocated")
	ErrMismatchedRange = errors.New("the provided UID range does not match the current UID range")
)

type Allocator struct {
	r     *uid.Range
	alloc allocator.Interface
}

// Allocator implements Interface and Snapshottable
var _ Interface = &Allocator{}

// New creates a Allocator over a UID range, calling factory to construct the backing store.
func New(r *uid.Range, factory allocator.AllocatorFactory) *Allocator {
	return &Allocator{
		r:     r,
		alloc: factory(int(r.Size()), r.String()),
	}
}

// NewInMemory creates an in-memory Allocator
func NewInMemory(r *uid.Range) *Allocator {
	return New(r, allocator.NewContiguousAllocationInterface)
}

// Free returns the count of port left in the range.
func (r *Allocator) Free() int {
	return r.alloc.Free()
}

// Allocate attempts to reserve the provided block. ErrNotInRange or
// ErrAllocated will be returned if the block is not valid for this range
// or has already been reserved.  ErrFull will be returned if there
// are no blocks left.
func (r *Allocator) Allocate(block uid.Block) error {
	ok, offset := r.contains(block)
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

// AllocateNext reserves one of the ports from the pool. ErrFull may
// be returned if there are no ports left.
func (r *Allocator) AllocateNext() (uid.Block, error) {
	offset, ok, err := r.alloc.AllocateNext()
	if err != nil {
		return uid.Block{}, err
	}
	if !ok {
		return uid.Block{}, ErrFull
	}
	block, ok := r.r.BlockAt(uint32(offset))
	if !ok {
		return uid.Block{}, ErrNotInRange
	}
	return block, nil
}

// Release releases the port back to the pool. Releasing an
// unallocated port or a port out of the range is a no-op and
// returns no error.
func (r *Allocator) Release(block uid.Block) error {
	ok, offset := r.contains(block)
	if !ok {
		// TODO: log a warning
		return nil
	}

	return r.alloc.Release(int(offset))
}

// Has returns true if the provided port is already allocated and a call
// to Allocate(block) would fail with ErrAllocated.
func (r *Allocator) Has(block uid.Block) bool {
	ok, offset := r.contains(block)
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
func (r *Allocator) Restore(into *uid.Range, data []byte) error {
	if into.String() != r.r.String() {
		return ErrMismatchedRange
	}
	snapshottable, ok := r.alloc.(allocator.Snapshottable)
	if !ok {
		return fmt.Errorf("not a snapshottable allocator")
	}
	return snapshottable.Restore(into.String(), data)
}

// contains returns true and the offset if the block is in the range (and aligned), and false
// and nil otherwise.
func (r *Allocator) contains(block uid.Block) (bool, uint32) {
	return r.r.Offset(block)
}
