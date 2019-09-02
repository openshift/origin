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

	"github.com/boltdb/bolt"
)

// VolumeCreateOperation implements the operation functions used to
// create a new volume.
type VolumeCreateOperation struct {
	OperationManager
	vol        *VolumeEntry
	maxRetries int
	reclaimed  ReclaimMap // gets set by Clean() call
}

// NewVolumeCreateOperation returns a new VolumeCreateOperation populated
// with the given volume entry and db connection and allocates a new
// pending operation entry.
func NewVolumeCreateOperation(
	vol *VolumeEntry, db wdb.DB) *VolumeCreateOperation {

	return &VolumeCreateOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		maxRetries: VOLUME_MAX_RETRIES,
		vol:        vol,
	}
}

// loadVolumeCreateOperation returns a VolumeCreateOperation populated
// from an existing pending operation entry in the db.
func loadVolumeCreateOperation(
	db wdb.DB, p *PendingOperationEntry) (*VolumeCreateOperation, error) {

	vols, err := volumesFromOp(db, p)
	if err != nil {
		return nil, err
	}
	if len(vols) != 1 {
		return nil, fmt.Errorf(
			"Incorrect number of volumes (%v) for create operation: %v",
			len(vols), p.Id)
	}

	return &VolumeCreateOperation{
		OperationManager: OperationManager{
			db: db,
			op: p,
		},
		maxRetries: VOLUME_MAX_RETRIES,
		vol:        vols[0],
	}, nil
}

func (vc *VolumeCreateOperation) Label() string {
	return "Create Volume"
}

func (vc *VolumeCreateOperation) ResourceUrl() string {
	return fmt.Sprintf("/volumes/%v", vc.vol.Info.Id)
}

func (vc *VolumeCreateOperation) MaxRetries() int {
	return vc.maxRetries
}

// Build allocates and saves new volume and brick entries (tagged as pending)
// in the db.
func (vc *VolumeCreateOperation) Build() error {
	return vc.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		brick_entries, err := vc.vol.createVolumeComponents(txdb)
		if err != nil {
			return err
		}
		for _, brick := range brick_entries {
			vc.op.RecordAddBrick(brick)
			if e := brick.Save(tx); e != nil {
				return e
			}
		}
		vc.op.RecordAddVolume(vc.vol)
		if e := vc.vol.Save(tx); e != nil {
			return e
		}
		if e := vc.op.Save(tx); e != nil {
			return e
		}
		return nil
	})
}

// Exec creates new bricks and volume on the underlying glusterfs storage system.
func (vc *VolumeCreateOperation) Exec(executor executors.Executor) error {
	brick_entries, err := bricksFromOp(vc.db, vc.op, vc.vol.Info.Gid)
	if err != nil {
		logger.LogError("Failed to get bricks from op: %v", err)
		return err
	}
	err = vc.vol.createVolumeExec(vc.db, executor, brick_entries)
	if err != nil {
		logger.LogError("Error executing create volume: %v", err)
		return OperationRetryError{err}
	}
	return nil
}

// Finalize marks our new volume and brick db entries as no longer pending.
func (vc *VolumeCreateOperation) Finalize() error {
	return vc.db.Update(func(tx *bolt.Tx) error {
		brick_entries, err := bricksFromOp(wdb.WrapTx(tx), vc.op, vc.vol.Info.Gid)
		if err != nil {
			logger.LogError("Failed to get bricks from op: %v", err)
			return err
		}
		for _, brick := range brick_entries {
			vc.op.FinalizeBrick(brick)
			if e := brick.Save(tx); e != nil {
				return e
			}
		}
		vc.op.FinalizeVolume(vc.vol)
		if e := vc.vol.Save(tx); e != nil {
			return e
		}

		vc.op.Delete(tx)
		return nil
	})
}

// Rollback removes any dangling volume and bricks from the underlying storage
// systems and removes the corresponding pending volume and brick entries from
// the db.
func (vc *VolumeCreateOperation) Rollback(executor executors.Executor) error {
	return rollbackViaClean(vc, executor)
}

func (vc *VolumeCreateOperation) Clean(executor executors.Executor) error {
	var err error
	logger.Info("Starting Clean for %v op:%v", vc.Label(), vc.op.Id)
	vc.reclaimed, err = removeVolumeWithOp(
		vc.db, executor, vc.op, vc.vol.Info.Id)
	return err
}

func (vc *VolumeCreateOperation) CleanDone() error {
	logger.Info("Clean is done for %v op:%v", vc.Label(), vc.op.Id)
	if vc.reclaimed == nil || len(vc.reclaimed) == 0 {
		return logger.LogError("brick reclaim map is empty (was Clean called?)")
	}
	var err error
	// set in-memory copy of volume to match (torn down) db state
	vc.vol, err = expungeVolumeWithOp(vc.db, vc.op, vc.vol.Info.Id, vc.reclaimed)
	return err
}

// VolumeExpandOperation implements the operation functions used to
// expand an existing volume.
type VolumeExpandOperation struct {
	OperationManager
	noRetriesOperation
	vol *VolumeEntry

	// modification values
	ExpandSize int
	reclaimed  ReclaimMap // gets set by Clean() call
}

// NewVolumeCreateOperation creates a new VolumeExpandOperation populated
// with the given volume entry, db connection and size (in GB) that the
// volume is to be expanded by.
func NewVolumeExpandOperation(
	vol *VolumeEntry, db wdb.DB, sizeGB int) *VolumeExpandOperation {

	return &VolumeExpandOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		vol:        vol,
		ExpandSize: sizeGB,
	}
}

// loadVolumeExpandOperation returns a VolumeExpandOperation populated
// from an existing pending operation entry in the db.
func loadVolumeExpandOperation(
	db wdb.DB, p *PendingOperationEntry) (*VolumeExpandOperation, error) {

	vols, err := volumesFromOp(db, p)
	if err != nil {
		return nil, err
	}
	if len(vols) != 1 {
		return nil, fmt.Errorf(
			"Incorrect number of volumes (%v) for create operation: %v",
			len(vols), p.Id)
	}

	return &VolumeExpandOperation{
		OperationManager: OperationManager{
			db: db,
			op: p,
		},
		vol: vols[0],
	}, nil
}

func (ve *VolumeExpandOperation) Label() string {
	return "Expand Volume"
}

func (ve *VolumeExpandOperation) ResourceUrl() string {
	return fmt.Sprintf("/volumes/%v", ve.vol.Info.Id)
}

// Build determines what new bricks needs to be created to satisfy the
// new volume size. It marks new bricks as pending in the db.
func (ve *VolumeExpandOperation) Build() error {
	return ve.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		brick_entries, err := ve.vol.expandVolumeComponents(
			txdb, ve.ExpandSize, false)
		if err != nil {
			return err
		}
		for _, brick := range brick_entries {
			ve.op.RecordAddBrick(brick)
			if e := brick.Save(tx); e != nil {
				return e
			}
		}
		ve.op.RecordExpandVolume(ve.vol, ve.ExpandSize)
		if e := ve.op.Save(tx); e != nil {
			return e
		}
		return nil
	})
}

// Exec creates new bricks on the underlying storage systems.
func (ve *VolumeExpandOperation) Exec(executor executors.Executor) error {
	brick_entries, err := bricksFromOp(ve.db, ve.op, ve.vol.Info.Gid)
	if err != nil {
		logger.LogError("Failed to get bricks from op: %v", err)
		return err
	}
	err = ve.vol.expandVolumeExec(ve.db, executor, brick_entries)
	if err != nil {
		logger.LogError("Error executing expand volume: %v", err)
	}
	return err
}

// Rollback cancels the volume expansion and remove pending brick entries
// from the db.
func (ve *VolumeExpandOperation) Rollback(executor executors.Executor) error {
	return rollbackViaClean(ve, executor)
}

// Finalize marks new bricks as no longer pending and updates the size
// of the existing volume entry.
func (ve *VolumeExpandOperation) Finalize() error {
	return ve.db.Update(func(tx *bolt.Tx) error {
		brick_entries, err := bricksFromOp(wdb.WrapTx(tx), ve.op, ve.vol.Info.Gid)
		if err != nil {
			logger.LogError("Failed to get bricks from op: %v", err)
			return err
		}
		sizeDelta, err := expandSizeFromOp(ve.op)
		if err != nil {
			logger.LogError("Failed to get expansion size from op: %v", err)
			return err
		}

		for _, brick := range brick_entries {
			ve.op.FinalizeBrick(brick)
			if e := brick.Save(tx); e != nil {
				return e
			}
		}
		ve.vol.Info.Size += sizeDelta
		if ve.vol.Info.Block == true {
			if e := ve.vol.AddRawCapacity(sizeDelta); e != nil {
				return e
			}
		}
		ve.op.FinalizeVolume(ve.vol)
		if e := ve.vol.Save(tx); e != nil {
			return e
		}

		ve.op.Delete(tx)
		return nil
	})
}

func (ve *VolumeExpandOperation) Clean(executor executors.Executor) error {
	logger.Info("Starting Clean for %v op:%v", ve.Label(), ve.op.Id)
	var (
		err  error
		bmap brickHostMap
	)
	err = ve.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		v, err := NewVolumeEntryFromId(tx, ve.vol.Info.Id)
		if err != nil {
			return err
		}
		bricks, err := bricksFromOp(txdb, ve.op, v.Info.Gid)
		if err != nil {
			return err
		}
		bmap, err = newBrickHostMap(txdb, bricks)
		return err
	})
	if err != nil {
		logger.LogError("Failed to get bricks from op: %v", err)
		return err
	}
	// nothing past this point needs a db reference
	ve.reclaimed, err = bmap.destroy(executor)
	if err != nil {
		logger.LogError("Failed to destroy bricks: %v", err)
		return err
	}
	return nil
}

func (ve *VolumeExpandOperation) CleanDone() error {
	logger.Info("Clean is done for %v op:%v", ve.Label(), ve.op.Id)
	// reminder: a volume's size is expanded during finalize and
	// thus retains the original size until op succeeds
	return ve.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		v, err := NewVolumeEntryFromId(tx, ve.vol.Info.Id)
		if err != nil {
			return err
		}
		bricks, err := bricksFromOp(txdb, ve.op, v.Info.Gid)
		for _, brick := range bricks {
			err := brick.removeAndFree(tx, v, ve.reclaimed[brick.Info.Id])
			if err != nil {
				return err
			}
		}
		if err := v.Save(tx); err != nil {
			return err
		}
		return ve.op.Delete(tx)
	})
}

// VolumeDeleteOperation implements the operation functions used to
// delete an existing volume.
type VolumeDeleteOperation struct {
	OperationManager
	noRetriesOperation
	vol       *VolumeEntry
	reclaimed ReclaimMap // gets set by Exec() call
}

func NewVolumeDeleteOperation(
	vol *VolumeEntry, db wdb.DB) *VolumeDeleteOperation {

	return &VolumeDeleteOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		vol: vol,
	}
}

// loadVolumeDeleteOperation returns a VolumeDeleteOperation populated
// from an existing pending operation entry in the db.
func loadVolumeDeleteOperation(
	db wdb.DB, p *PendingOperationEntry) (*VolumeDeleteOperation, error) {

	vols, err := volumesFromOp(db, p)
	if err != nil {
		return nil, err
	}
	if len(vols) != 1 {
		return nil, fmt.Errorf(
			"Incorrect number of volumes (%v) for delete operation: %v",
			len(vols), p.Id)
	}

	return &VolumeDeleteOperation{
		OperationManager: OperationManager{
			db: db,
			op: p,
		},
		vol: vols[0],
	}, nil
}

func (vdel *VolumeDeleteOperation) Label() string {
	return "Delete Volume"
}

func (vdel *VolumeDeleteOperation) ResourceUrl() string {
	return ""
}

// Build determines what volumes and bricks need to be deleted and
// marks the db entries as such.
func (vdel *VolumeDeleteOperation) Build() error {
	return vdel.db.Update(func(tx *bolt.Tx) error {
		v, err := NewVolumeEntryFromId(tx, vdel.vol.Info.Id)
		if err != nil {
			return err
		}
		vdel.vol = v
		if vdel.vol.Pending.Id != "" {
			logger.LogError("Pending volume %v can not be deleted",
				vdel.vol.Info.Id)
			return ErrConflict
		}
		txdb := wdb.WrapTx(tx)
		brick_entries, err := vdel.vol.deleteVolumeComponents(txdb)
		if err != nil {
			return err
		}
		for _, brick := range brick_entries {
			if brick.Pending.Id != "" {
				logger.LogError("Pending brick %v can not be deleted",
					brick.Info.Id)
				return ErrConflict
			}
			vdel.op.RecordDeleteBrick(brick)
			if e := brick.Save(tx); e != nil {
				return e
			}
		}
		vdel.op.RecordDeleteVolume(vdel.vol)
		if e := vdel.op.Save(tx); e != nil {
			return e
		}
		if e := vdel.vol.Save(tx); e != nil {
			return e
		}
		return nil
	})
}

// Exec performs the volume and brick deletions on the storage systems.
func (vdel *VolumeDeleteOperation) Exec(executor executors.Executor) error {
	var err error
	vdel.reclaimed, err = removeVolumeWithOp(
		vdel.db, executor, vdel.op, vdel.vol.Info.Id)
	if err != nil {
		logger.LogError("Error executing delete volume: %v", err)
	}
	return err
}

func (vdel *VolumeDeleteOperation) Rollback(executor executors.Executor) error {
	// currently rollback only removes the pending operation for delete volume,
	// leaving the db in the same state as it was before an exec failure.
	// In the future we should make this operation resume-able
	return vdel.db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		brick_entries, err := bricksFromOp(txdb, vdel.op, vdel.vol.Info.Gid)
		if err != nil {
			logger.LogError("Failed to get bricks from op: %v", err)
			return err
		}

		for _, brick := range brick_entries {
			vdel.op.FinalizeBrick(brick)
			if e := brick.Save(tx); e != nil {
				return e
			}
		}
		vdel.op.FinalizeVolume(vdel.vol)
		if e := vdel.vol.Save(tx); e != nil {
			return err
		}

		vdel.op.Delete(tx)
		return nil
	})
}

// Finalize marks all brick and volume entries for this operation as
// fully deleted.
func (vdel *VolumeDeleteOperation) Finalize() error {
	if vdel.reclaimed == nil || len(vdel.reclaimed) == 0 {
		return logger.LogError("brick reclaim map is empty (was Exec called?)")
	}
	_, err := expungeVolumeWithOp(vdel.db, vdel.op, vdel.vol.Info.Id, vdel.reclaimed)
	return err
}

// Clean tries to re-execute the volume delete operation.
func (vdel *VolumeDeleteOperation) Clean(executor executors.Executor) error {
	// for a delete, clean is essentially a replay of exec
	// because exec must be robust against restarts now we can just call Exec
	logger.Info("Starting Clean for %v op:%v", vdel.Label(), vdel.op.Id)
	return vdel.Exec(executor)
}

func (vdel *VolumeDeleteOperation) CleanDone() error {
	// for a delete, clean done is essentially a replay of finalize
	logger.Info("Clean is done for %v op:%v", vdel.Label(), vdel.op.Id)
	return vdel.Finalize()
}

// VolumeCloneOperation implements the operation functions used to
// clone an existing volume.
type VolumeCloneOperation struct {
	OperationManager
	noRetriesOperation

	// The volume to use as source for the clone
	vol *VolumeEntry
	// Optional name for the new volume
	clonename string
	// The newly cloned volume, will be set in Exec()
	clone *VolumeEntry
	// The bricks for the clone
	bricks []*BrickEntry
	// The devices of the bricks
	devices []*DeviceEntry
}

func NewVolumeCloneOperation(
	vol *VolumeEntry, db wdb.DB, clonename string) *VolumeCloneOperation {

	return &VolumeCloneOperation{
		OperationManager: OperationManager{
			db: db,
			op: NewPendingOperationEntry(NEW_ID),
		},
		vol:       vol,
		clonename: clonename,
		clone:     nil,
	}
}

func (vc *VolumeCloneOperation) Label() string {
	return "Create Clone of a Volume"
}

func (vc *VolumeCloneOperation) ResourceUrl() string {
	return fmt.Sprintf("/volumes/%v", vc.clone.Info.Id)
}

func (vc *VolumeCloneOperation) Build() error {
	return vc.db.Update(func(tx *bolt.Tx) error {
		vc.op.RecordCloneVolume(vc.vol)
		clone, bricks, devices, err := vc.vol.prepareVolumeClone(tx, vc.clonename)
		if err != nil {
			return err
		}
		vc.clone = clone
		vc.bricks = bricks
		vc.devices = devices
		vc.op.RecordAddVolumeClone(vc.clone)
		// record new bricks
		for _, b := range bricks {
			vc.op.RecordAddBrick(b)
			if e := b.Save(tx); e != nil {
				return e
			}
		}
		// save device updates
		for _, d := range vc.devices {
			if e := d.Save(tx); e != nil {
				return e
			}
		}
		// record changes to parent volume
		if e := vc.vol.Save(tx); e != nil {
			return e
		}
		// add the new volume to the cluster
		c, err := NewClusterEntryFromId(tx, vc.clone.Info.Cluster)
		if err != nil {
			return err
		}
		c.VolumeAdd(vc.clone.Info.Id)
		if err := c.Save(tx); err != nil {
			return err
		}
		// save the pending operation
		if e := vc.op.Save(tx); e != nil {
			return e
		}
		return nil
	})
}

func (vc *VolumeCloneOperation) Exec(executor executors.Executor) error {
	vcr, host, err := vc.vol.cloneVolumeRequest(vc.db, vc.clone.Info.Name)
	if err != nil {
		return err
	}

	// get all details of the original volume (order of bricks etc)
	orig, err := executor.VolumeInfo(host, vc.vol.Info.Name)
	if err != nil {
		return err
	}

	clone, err := executor.VolumeClone(host, vcr)
	if err != nil {
		return err
	}

	if err := updateCloneBrickPaths(vc.bricks, orig, clone); err != nil {
		return err
	}
	return nil
}

func (vc *VolumeCloneOperation) Rollback(executor executors.Executor) error {
	return vc.db.Update(func(tx *bolt.Tx) error {
		vc.op.FinalizeVolumeClone(vc.vol)
		if e := vc.vol.Save(tx); e != nil {
			return e
		}

		// TODO: Bricks and a snapshot may have been created in the
		// executor. These will need to be removed again. The
		// CmdExecutor.VolumeClone() operation will need to be moved up
		// to a higher level. This can easiest be done when the
		// advanced Snapshot*() operations are available.

		vc.op.Delete(tx)
		return nil
	})
}

func (vc *VolumeCloneOperation) Finalize() error {
	return vc.db.Update(func(tx *bolt.Tx) error {
		vc.op.FinalizeVolumeClone(vc.vol)
		if err := vc.vol.Save(tx); err != nil {
			return err
		}
		// finalize the new clone
		vc.op.FinalizeVolume(vc.clone)
		if err := vc.clone.Save(tx); err != nil {
			return err
		}
		// finalize the new bricks
		for _, b := range vc.bricks {
			vc.op.FinalizeBrick(b)
			b.Save(tx)
		}
		// the DeviceEntry of each brick was updated too
		for _, d := range vc.devices {
			// because the bricks are cloned, they do not take extra space
			d.Save(tx)
		}

		vc.op.Delete(tx)
		return nil
	})
}

// removeVolumeWithOp is a helper function that implements common
// code for getting the needed parts of a volume from the db and
// using them to remove the volume and bricks from the db.
// This logic is shared by both volume create and volume delete.
func removeVolumeWithOp(
	db wdb.RODB, executor executors.Executor,
	op *PendingOperationEntry, volId string) (ReclaimMap, error) {

	var (
		err   error
		v     *VolumeEntry
		hosts nodeHosts
		bmap  brickHostMap
	)
	logger.Info("preparing to remove volume %v in op:%v", volId, op.Id)
	err = db.View(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		// get a fresh volume object from db
		v, err = NewVolumeEntryFromId(tx, volId)
		if err != nil {
			return err
		}
		hosts, err = v.hosts(txdb)
		if err != nil {
			return err
		}
		bricks, err := bricksFromOp(txdb, op, v.Info.Gid)
		if err != nil {
			return err
		}
		bmap, err = newBrickHostMap(txdb, bricks)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.LogError(
			"failed to get state needed to destroy volume: %v", err)
		return nil, err
	}
	// nothing past this point needs a db reference
	logger.Info("executing removal of volume %v in op:%v", volId, op.Id)
	err = newTryOnHosts(hosts).run(func(h string) error {
		return v.destroyVolumeFromHost(executor, h)
	})
	if err != nil {
		return nil, err
	}
	return bmap.destroy(executor)
}

// expungeVolumeWithOp is a helper function that removes a given volume and
// associated operation from the db. It can be shared with volume create and
// volume delete operation functions.
func expungeVolumeWithOp(
	db wdb.DB,
	op *PendingOperationEntry, volId string,
	reclaimed ReclaimMap) (*VolumeEntry, error) {

	var v *VolumeEntry
	return v, db.Update(func(tx *bolt.Tx) error {
		var err error
		txdb := wdb.WrapTx(tx)
		v, err = NewVolumeEntryFromId(tx, volId)
		if err != nil {
			return err
		}
		bricks, err := bricksFromOp(txdb, op, v.Info.Gid)
		if err != nil {
			return err
		}
		if err := v.teardown(txdb, bricks, reclaimed); err != nil {
			return err
		}
		return op.Delete(tx)
	})
}
