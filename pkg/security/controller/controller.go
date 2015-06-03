package controller

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/security/mcs"
	"github.com/openshift/origin/pkg/security/uid"
	"github.com/openshift/origin/pkg/security/uidallocator"
)

const (
	uidRangeAnnotation = "openshift.io/sa.scc.uid-range"
	mcsAnnotation      = "openshift.io/sa.scc.mcs"
)

type MCSAllocationFunc func(uid.Block) *mcs.Label

// DefaultMCSAllocation returns a label from the MCS range that matches the offset
// within the overall range. blockSize must be a positive integer representing the
// number of labels to jump past in the category space (if 1, range == label, if 2
// each range will have two labels).
func DefaultMCSAllocation(from *uid.Range, to *mcs.Range, blockSize int) MCSAllocationFunc {
	return func(block uid.Block) *mcs.Label {
		ok, offset := from.Offset(block)
		if !ok {
			return nil
		}
		if blockSize > 0 {
			offset = offset * uint32(blockSize)
		}
		label, _ := to.LabelAt(uint64(offset))
		return label
	}
}

type Allocation struct {
	uid    uidallocator.Interface
	mcs    MCSAllocationFunc
	client kclient.NamespaceInterface
}

// retryCount is the number of times to retry on a conflict when updating a namespace
const retryCount = 2

// Next processes a changed namespace and tries to allocate a uid range for it.  If it is
// successful, an mcs label corresponding to the relative position of the range is also
// set.
func (c *Allocation) Next(ns *kapi.Namespace) error {
	tx := &tx{}
	defer tx.Rollback()

	if _, ok := ns.Annotations[uidRangeAnnotation]; ok {
		return nil
	}

	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}

	// do uid allocation
	block, err := c.uid.AllocateNext()
	if err != nil {
		return err
	}
	tx.Add(func() error { return c.uid.Release(block) })
	ns.Annotations[uidRangeAnnotation] = block.String()
	if _, ok := ns.Annotations[mcsAnnotation]; !ok {
		if label := c.mcs(block); label != nil {
			ns.Annotations[mcsAnnotation] = label.String()
		}
	}

	// TODO: could use a client.GuaranteedUpdate/Merge function
	for i := 0; i < retryCount; i++ {
		_, err := c.client.Update(ns)
		if err == nil {
			// commit and exit
			tx.Commit()
			return nil
		}

		if errors.IsNotFound(err) {
			return nil
		}
		if !errors.IsConflict(err) {
			return err
		}
		newNs, err := c.client.Get(ns.Name)
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if changedAndSetAnnotations(ns, newNs) {
			return nil
		}

		// try again
		if newNs.Annotations == nil {
			newNs.Annotations = make(map[string]string)
		}
		newNs.Annotations[uidRangeAnnotation] = ns.Annotations[uidRangeAnnotation]
		newNs.Annotations[mcsAnnotation] = ns.Annotations[mcsAnnotation]
		ns = newNs
	}

	return fmt.Errorf("unable to allocate security info on %q after %d retries", ns.Name, retryCount)
}

func changedAndSetAnnotations(old, ns *kapi.Namespace) bool {
	if value, ok := ns.Annotations[uidRangeAnnotation]; ok && value != old.Annotations[uidRangeAnnotation] {
		return true
	}
	if value, ok := ns.Annotations[mcsAnnotation]; ok && value != old.Annotations[mcsAnnotation] {
		return true
	}
	return false
}

type tx struct {
	rollback []func() error
}

func (tx *tx) Add(fn func() error) {
	tx.rollback = append(tx.rollback, fn)
}

func (tx *tx) HasChanges() bool {
	return len(tx.rollback) > 0
}

func (tx *tx) Rollback() {
	for _, fn := range tx.rollback {
		if err := fn(); err != nil {
			util.HandleError(fmt.Errorf("unable to undo tx: %v", err))
		}
	}
}

func (tx *tx) Commit() {
	tx.rollback = nil
}
