package controller

import (
	"fmt"
	"time"

	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/service"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/security"
	"github.com/openshift/origin/pkg/security/uid"
	"github.com/openshift/origin/pkg/security/uidallocator"
)

// Repair is a controller loop that periodically examines all UID allocations
// and logs any errors, and then sets the compacted and accurate list of both
//
// Can be run at infrequent intervals, and is best performed on startup of the master.
// Is level driven and idempotent - all claimed UIDs will be updated into the allocator
// map at the end of a single execution loop if no race is encountered.
//
type Repair struct {
	interval time.Duration
	client   client.NamespaceInterface
	alloc    service.RangeRegistry
	uidRange *uid.Range
}

// NewRepair creates a controller that periodically ensures that all UIDs labels that are allocated in the cluster
// are claimed.
func NewRepair(interval time.Duration, client client.NamespaceInterface, uidRange *uid.Range, alloc service.RangeRegistry) *Repair {
	return &Repair{
		interval: interval,
		client:   client,
		uidRange: uidRange,
		alloc:    alloc,
	}
}

// RunUntil starts the controller until the provided ch is closed.
func (c *Repair) RunUntil(ch chan struct{}) {
	util.Until(func() {
		if err := c.RunOnce(); err != nil {
			util.HandleError(err)
		}
	}, c.interval, ch)
}

// RunOnce verifies the state of allocations and returns an error if an unrecoverable problem occurs.
func (c *Repair) RunOnce() error {
	// TODO: (per smarterclayton) if Get() or List() is a weak consistency read,
	// or if they are executed against different leaders,
	// the ordering guarantee required to ensure no item is allocated twice is violated.
	// List must return a ResourceVersion higher than the etcd index Get,
	// and the release code must not release items that have allocated but not yet been created
	// See #8295

	latest, err := c.alloc.Get()
	if err != nil {
		return fmt.Errorf("unable to refresh the security allocation UID blocks: %v", err)
	}

	list, err := c.client.List(labels.Everything(), fields.Everything())
	if err != nil {
		return fmt.Errorf("unable to refresh the security allocation UID blocks: %v", err)
	}

	uids := uidallocator.NewInMemory(c.uidRange)

	for _, ns := range list.Items {
		value, ok := ns.Annotations[security.UIDRangeAnnotation]
		if !ok {
			continue
		}
		block, err := uid.ParseBlock(value)
		if err != nil {
			continue
		}
		switch err := uids.Allocate(block); err {
		case nil:
		case uidallocator.ErrFull:
			// TODO: send event
			return fmt.Errorf("the UID range %s is full; you must widen the range in order to allocate more UIDs", c.uidRange)
		default:
			return fmt.Errorf("unable to allocate UID block %s for namespace %s due to an unknown error, exiting: %v", block, ns.Name, err)
		}
	}

	err = uids.Snapshot(latest)
	if err != nil {
		return fmt.Errorf("unable to persist the updated namespace UID allocations: %v", err)
	}

	if err := c.alloc.CreateOrUpdate(latest); err != nil {
		return fmt.Errorf("unable to persist the updated namespace UID allocations: %v", err)
	}
	return nil
}
