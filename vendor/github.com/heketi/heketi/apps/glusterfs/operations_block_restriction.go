//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"fmt"

	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
)

// VolumeSetBlockRestrictionOperation implements the operation functions
// that change the block restriction state of a volume.
type VolumeSetBlockRestrictionOperation struct {
	OperationManager
	noRetriesOperation
	vol         *VolumeEntry
	restriction api.BlockRestriction
}

// NewVolumeSetBlockRestrictionOperation returns a new
// VolumeSetBlockRestrictionOperation populated with the given params.
func NewVolumeSetBlockRestrictionOperation(
	vol *VolumeEntry, db wdb.DB,
	restriction api.BlockRestriction) *VolumeSetBlockRestrictionOperation {

	return &VolumeSetBlockRestrictionOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		vol:         vol,
		restriction: restriction,
	}
}

func (ro *VolumeSetBlockRestrictionOperation) Label() string {
	return "Set Volume's Block Volumes Restriction State"
}

func (ro *VolumeSetBlockRestrictionOperation) ResourceUrl() string {
	return fmt.Sprintf("/volumes/%v", ro.vol.Info.Id)
}

// Build sets the state when adding restrictions.
func (ro *VolumeSetBlockRestrictionOperation) Build() error {
	if !ro.vol.Info.Block {
		return fmt.Errorf(
			"Block restrictions can only be set on block hosting volumes")
	}
	// do a "pre flight check" of unlock stuff
	if e := ro.checkCanUnlock(ro.vol, ro.restriction); e != nil {
		return e
	}
	if ro.restriction == api.Unrestricted {
		// relaxing the restriction on a volume is performed
		// only in finalize, after any sanity checks are done.
		return nil
	}
	return ro.db.Update(func(tx *bolt.Tx) error {
		v, err := NewVolumeEntryFromId(tx, ro.vol.Info.Id)
		if err != nil {
			return err
		}
		v.Info.BlockInfo.Restriction = ro.restriction
		return v.Save(tx)
	})
}

// Exec creates new bricks and volume on the underlying glusterfs storage system.
func (ro *VolumeSetBlockRestrictionOperation) Exec(executor executors.Executor) error {
	// currently does nothing. should do gluster-block sanity checks in the
	// future
	return nil
}

// Finalize marks our new volume and brick db entries as no longer pending.
func (ro *VolumeSetBlockRestrictionOperation) Finalize() error {
	if ro.restriction != api.Unrestricted {
		// change was already made by build
		return nil
	}
	return ro.db.Update(func(tx *bolt.Tx) error {
		v, err := NewVolumeEntryFromId(tx, ro.vol.Info.Id)
		if err != nil {
			return err
		}
		if e := ro.checkCanUnlock(v, ro.restriction); e != nil {
			return e
		}
		if e := ro.fixReservedSize(v); e != nil {
			return e
		}
		v.Info.BlockInfo.Restriction = ro.restriction
		return v.Save(tx)
	})
}

// Rollback does nothing for this operation type.
func (ro *VolumeSetBlockRestrictionOperation) Rollback(executor executors.Executor) error {
	return nil
}

func (ro *VolumeSetBlockRestrictionOperation) checkCanUnlock(
	v *VolumeEntry, r api.BlockRestriction) error {

	switch v.Info.BlockInfo.Restriction {
	case r:
		// current state same as desired state is always OK
		return nil
	case api.Locked:
		// unlocking an admin locked volume is OK
		return nil
	case api.LockedByUpdate:
		if v.Info.BlockInfo.ReservedSize > 0 {
			// reserved size was already set
			return nil
		}
		rSize := v.Info.Size - ReduceRawSize(v.Info.Size)
		if v.Info.BlockInfo.FreeSize >= rSize {
			// there is enough free space to reserve (some of) it
			return nil
		}
		return fmt.Errorf(
			"Can not unlock volume. %vGiB free space is required, but found %vGiB",
			rSize, v.Info.BlockInfo.FreeSize)
	case api.Unrestricted:
		// the volume is already Unrestricted. making it more restricted
		// is OK
		return nil
	default:
		return fmt.Errorf("Unexpected restriction state: %v",
			v.Info.BlockInfo.Restriction)
	}
}

func (ro *VolumeSetBlockRestrictionOperation) fixReservedSize(
	v *VolumeEntry) error {

	if v.Info.BlockInfo.Restriction != api.LockedByUpdate {
		return nil
	}
	if v.Info.BlockInfo.ReservedSize > 0 {
		// reserved size was already set
		return nil
	}
	rSize := v.Info.Size - ReduceRawSize(v.Info.Size)
	if err := v.ModifyFreeSize(-rSize); err != nil {
		return err
	}
	if err := v.ModifyReservedSize(rSize); err != nil {
		return err
	}
	return nil
}
