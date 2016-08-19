package netid

import (
	"errors"

	"k8s.io/kubernetes/pkg/registry/service/allocator"
)

// Interface manages the allocation of netids out of a range.
// Interface should be threadsafe.
type Interface interface {
	Allocate(uint32) error
	AllocateNext() (uint32, error)
	Release(uint32) error
	Has(uint32) bool
}

var (
	ErrFull       = errors.New("range is full")
	ErrNotInRange = errors.New("provided netid is not in the valid range")
	ErrAllocated  = errors.New("provided netid is already allocated")
)

type Allocator struct {
	netIDRange *NetIDRange
	alloc      allocator.Interface
}

// Allocator implements allocator Interface
var _ Interface = &Allocator{}

// New creates a Allocator over a netid Range, calling allocatorFactory to construct the backing store.
func New(r *NetIDRange, allocatorFactory allocator.AllocatorFactory) *Allocator {
	return &Allocator{
		netIDRange: r,
		alloc:      allocatorFactory(int(r.Size), r.String()),
	}
}

// Helper that wraps New, for creating a range backed by an in-memory store.
func NewInMemory(r *NetIDRange) *Allocator {
	return New(r, func(max int, rangeSpec string) allocator.Interface {
		return allocator.NewAllocationMap(max, rangeSpec)
	})
}

// Free returns the count of netid left in the range.
func (r *Allocator) Free() int {
	return r.alloc.Free()
}

// Allocate attempts to reserve the provided netid. ErrNotInRange or
// ErrAllocated will be returned if the netid is not valid for this range
// or has already been reserved.
func (r *Allocator) Allocate(id uint32) error {
	ok, offset := r.netIDRange.Contains(id)
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

// AllocateNext reserves one of the netids from the pool. ErrFull may
// be returned if there are no netids left.
func (r *Allocator) AllocateNext() (uint32, error) {
	offset, ok, err := r.alloc.AllocateNext()
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrFull
	}
	return r.netIDRange.Base + uint32(offset), nil
}

// Release releases the netid back to the pool. Releasing an
// unallocated netid or a netid out of the range is a no-op and
// returns no error.
func (r *Allocator) Release(id uint32) error {
	ok, offset := r.netIDRange.Contains(id)
	if !ok {
		return nil
	}
	return r.alloc.Release(int(offset))
}

// Has returns true if the provided netid is already allocated and a call
// to Allocate(netid) would fail with ErrAllocated.
func (r *Allocator) Has(id uint32) bool {
	ok, offset := r.netIDRange.Contains(id)
	if !ok {
		return false
	}
	return r.alloc.Has(int(offset))
}
