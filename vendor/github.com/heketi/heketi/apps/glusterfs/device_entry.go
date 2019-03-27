//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/idgen"
	"github.com/heketi/heketi/pkg/sortedstrings"
	"github.com/lpabon/godbc"
)

const (
	maxPoolMetadataSizeMb = 16 * GB
)

type DeviceEntry struct {
	Entry

	Info       api.DeviceInfo
	Bricks     sort.StringSlice
	NodeId     string
	ExtentSize uint64
}

func DeviceList(tx *bolt.Tx) ([]string, error) {

	list := EntryKeys(tx, BOLTDB_BUCKET_DEVICE)
	if list == nil {
		return nil, ErrAccessList
	}
	return list, nil
}

func NewDeviceEntry() *DeviceEntry {
	entry := &DeviceEntry{}
	entry.Bricks = make(sort.StringSlice, 0)
	entry.SetOnline()

	// Default to 4096KB
	entry.ExtentSize = 4096

	return entry
}

func NewDeviceEntryFromRequest(req *api.DeviceAddRequest) *DeviceEntry {
	godbc.Require(req != nil)

	device := NewDeviceEntry()
	device.Info.Id = idgen.GenUUID()
	device.Info.Name = req.Name
	device.NodeId = req.NodeId
	device.Info.Tags = copyTags(req.Tags)

	return device
}

func NewDeviceEntryFromId(tx *bolt.Tx, id string) (*DeviceEntry, error) {
	godbc.Require(tx != nil)

	entry := NewDeviceEntry()
	err := EntryLoad(tx, entry, id)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (d *DeviceEntry) registerKey() string {
	return "DEVICE" + d.NodeId + d.Info.Name
}

func (d *DeviceEntry) Register(tx *bolt.Tx) error {
	godbc.Require(tx != nil)

	val, err := EntryRegister(tx,
		d,
		d.registerKey(),
		[]byte(d.Id()))
	if err == ErrKeyExists {

		// Now check if the node actually exists.  This only happens
		// when the application crashes and it doesn't clean up stale
		// registrations.
		conflictId := string(val)
		_, err := NewDeviceEntryFromId(tx, conflictId)
		if err == ErrNotFound {
			// (stale) There is actually no conflict, we can allow
			// the registration
			return nil
		} else if err != nil {
			return logger.Err(err)
		}

		return fmt.Errorf("Device %v is already used on node %v by device %v",
			d.Info.Name,
			d.NodeId,
			conflictId)

	} else if err != nil {
		return err
	}

	return nil
}

func (d *DeviceEntry) Deregister(tx *bolt.Tx) error {
	godbc.Require(tx != nil)

	err := EntryDelete(tx, d, d.registerKey())
	if err != nil {
		return err
	}

	return nil
}

func (d *DeviceEntry) SetId(id string) {
	d.Info.Id = id
}

func (d *DeviceEntry) Id() string {
	return d.Info.Id
}

func (d *DeviceEntry) BucketName() string {
	return BOLTDB_BUCKET_DEVICE
}

func (d *DeviceEntry) Save(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	godbc.Require(len(d.Info.Id) > 0)

	return EntrySave(tx, d, d.Info.Id)

}

func (d *DeviceEntry) HasBricks() bool {
	if len(d.Bricks) > 0 {
		return true
	}
	return false
}

func (d *DeviceEntry) ConflictString() string {
	return fmt.Sprintf("Unable to delete device [%v] because it contains bricks", d.Info.Id)
}

func (d *DeviceEntry) Delete(tx *bolt.Tx) error {
	godbc.Require(tx != nil)

	// Don't delete device unless it is in failed state
	if d.State != api.EntryStateFailed {
		return logger.LogError("device: %v is not in failed state", d.Info.Id)
	}

	// Check if the device still has bricks
	// Ideally, if the device is in failed state it should have no bricks
	// This is just for bricks with empty paths
	if d.HasBricks() {
		logger.LogError(d.ConflictString())
		return ErrConflict
	}

	return EntryDelete(tx, d, d.Info.Id)
}

func (d *DeviceEntry) modifyState(db wdb.DB, s api.EntryState) error {
	return db.Update(func(tx *bolt.Tx) error {
		// Save state
		d.State = s
		// Save new state
		if err := d.Save(tx); err != nil {
			return err
		}
		return nil
	})
}

func (d *DeviceEntry) SetState(db wdb.DB,
	e executors.Executor,
	s api.EntryState) error {

	if e := d.stateCheck(s); e != nil {
		return e
	}
	if d.State == s {
		return nil
	}

	switch s {
	case api.EntryStateOffline, api.EntryStateOnline:
		// simply update the state and move on
		if err := d.modifyState(db, s); err != nil {
			return err
		}
	case api.EntryStateFailed:
		if err := d.Remove(db, e); err != nil {
			if err == ErrNoReplacement {
				return logger.LogError("Unable to delete device [%v] as no device was found to replace it", d.Id())
			}
			return err
		}
	}
	return nil
}

func (d *DeviceEntry) stateCheck(s api.EntryState) error {
	// Check current state
	switch d.State {

	// Device is in removed/failed state
	case api.EntryStateFailed:
		switch s {
		case api.EntryStateFailed:
			return nil
		case api.EntryStateOnline:
			return fmt.Errorf("Cannot move a failed/removed device to online state")
		case api.EntryStateOffline:
			return nil
		default:
			return fmt.Errorf("Unknown state type: %v", s)
		}

	// Device is in enabled/online state
	case api.EntryStateOnline:
		switch s {
		case api.EntryStateOnline:
			return nil
		case api.EntryStateOffline:
			return nil
		case api.EntryStateFailed:
			return fmt.Errorf("Device must be offline before remove operation is performed, device:%v", d.Id())
		default:
			return fmt.Errorf("Unknown state type: %v", s)
		}

	// Device is in disabled/offline state
	case api.EntryStateOffline:
		switch s {
		case api.EntryStateOffline:
			return nil
		case api.EntryStateOnline:
			return nil
		case api.EntryStateFailed:
			return nil
		default:
			return fmt.Errorf("Unknown state type: %v", s)
		}
	}

	return nil
}

func (d *DeviceEntry) NewInfoResponse(tx *bolt.Tx) (*api.DeviceInfoResponse, error) {

	godbc.Require(tx != nil)

	info := &api.DeviceInfoResponse{}
	info.Id = d.Info.Id
	info.Name = d.Info.Name
	info.Storage = d.Info.Storage
	info.State = d.State
	info.Bricks = make([]api.BrickInfo, 0)
	info.Tags = copyTags(d.Info.Tags)

	// Add each drive information
	for _, id := range d.Bricks {
		brick, err := NewBrickEntryFromId(tx, id)
		if err != nil {
			return nil, err
		}

		brickinfo, err := brick.NewInfoResponse(tx)
		if err != nil {
			return nil, err
		}
		info.Bricks = append(info.Bricks, *brickinfo)
	}

	return info, nil
}

func (d *DeviceEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*d)

	return buffer.Bytes(), err
}

func (d *DeviceEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(d)
	if err != nil {
		return err
	}

	// Make sure to setup arrays if nil
	if d.Bricks == nil {
		d.Bricks = make(sort.StringSlice, 0)
	}

	return nil
}

func (d *DeviceEntry) BrickAdd(id string) {
	godbc.Require(!sortedstrings.Has(d.Bricks, id))

	d.Bricks = append(d.Bricks, id)
	d.Bricks.Sort()
}

func (d *DeviceEntry) BrickDelete(id string) {
	d.Bricks = sortedstrings.Delete(d.Bricks, id)
}

func (d *DeviceEntry) StorageSet(total uint64, free uint64, used uint64) {
	godbc.Check(total == free+used)

	d.Info.Storage.Total = total
	d.Info.Storage.Free = free
	d.Info.Storage.Used = used
}

func (d *DeviceEntry) StorageAllocate(amount uint64) {
	d.Info.Storage.Free -= amount
	d.Info.Storage.Used += amount
}

func (d *DeviceEntry) StorageFree(amount uint64) {
	d.Info.Storage.Free += amount
	d.Info.Storage.Used -= amount
}

func (d *DeviceEntry) StorageCheck(amount uint64) bool {
	return d.Info.Storage.Free > amount
}

func (d *DeviceEntry) SetExtentSize(amount uint64) {
	d.ExtentSize = amount
}

// Allocates a new brick if the space is available.  It will automatically reserve
// the storage amount required from the device's used storage, but it will not add
// the brick id to the brick list.  The caller is responsible for adding the brick
// id to the list.
func (d *DeviceEntry) NewBrickEntry(amount uint64, snapFactor float64, gid int64, volumeid string) *BrickEntry {

	// :TODO: This needs unit test

	sn := d.SpaceNeeded(amount, snapFactor)

	logger.Debug("device %v[%v] > required size [%v] ?",
		d.Id(),
		d.Info.Storage.Free, sn.Total)
	if !d.StorageCheck(sn.Total) {
		return nil
	}

	// Allocate amount from disk
	d.StorageAllocate(sn.Total)

	// Create brick
	return NewBrickEntry(amount, sn.TpSize, sn.PoolMetadataSize, d.Info.Id, d.NodeId, gid, volumeid)
}

type SpaceNeeded struct {
	TpSize           uint64
	PoolMetadataSize uint64
	Total            uint64
}

// SpaceNeeded returns the (estimated) space needed to add a brick
// of the given size amount and snapFactor to this device.
func (d *DeviceEntry) SpaceNeeded(amount uint64, snapFactor float64) SpaceNeeded {
	// Calculate thinpool size
	tpsize := uint64(float64(amount) * snapFactor)

	// Align tpsize to extent
	alignment := tpsize % d.ExtentSize
	if alignment != 0 {
		tpsize += d.ExtentSize - alignment
	}

	// Determine if we need to allocate space for the metadata
	metadataSize := d.poolMetadataSize(tpsize)

	// Align to extent
	alignment = metadataSize % d.ExtentSize
	if alignment != 0 {
		metadataSize += d.ExtentSize - alignment
	}

	// Total required size
	total := tpsize + metadataSize
	logger.Debug("expected space needed for amount=%v snapFactor=%v : %v",
		amount, snapFactor, total)
	return SpaceNeeded{tpsize, metadataSize, total}
}

// Return poolmetadatasize in KB
func (d *DeviceEntry) poolMetadataSize(tpsize uint64) uint64 {

	// TP size is in KB
	p := uint64(float64(tpsize) * 0.005)
	if p > maxPoolMetadataSizeMb {
		p = maxPoolMetadataSizeMb
	}

	return p
}

// Moves all the bricks from the device to one or more other devices
func (d *DeviceEntry) Remove(db wdb.DB,
	executor executors.Executor) (e error) {

	if e = RunOperation(
		NewDeviceRemoveOperation(d.Info.Id, db),
		executor); e != nil {
		return e
	}
	// tests currently expect d to be updated to match db state
	// this is another fairly ugly hack
	return db.View(func(tx *bolt.Tx) error {
		dbdev, err := NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}
		d.State = dbdev.State
		return nil
	})

}

func (d *DeviceEntry) removeBricksFromDevice(db wdb.DB,
	executor executors.Executor) (e error) {

	var errBrickWithEmptyPath error = fmt.Errorf("Brick has no path")

	for _, brickId := range d.Bricks {
		var brickEntry *BrickEntry
		var volumeEntry *VolumeEntry
		err := db.View(func(tx *bolt.Tx) error {
			var err error
			brickEntry, err = NewBrickEntryFromId(tx, brickId)
			if err != nil {
				return err
			}
			// Handle the special error case when brick has no path
			// we skip the brick and continue
			if brickEntry.Info.Path == "" {
				return errBrickWithEmptyPath
			}
			volumeEntry, err = NewVolumeEntryFromId(tx, brickEntry.Info.VolumeId)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			if err == errBrickWithEmptyPath {
				logger.Warning("Skipping brick with empty path, brickID: %v, volumeID: %v, error: %v", brickEntry.Info.Id, brickEntry.Info.VolumeId, err)
				continue
			}
			return err
		}
		logger.Info("Replacing brick %v on device %v on node %v", brickEntry.Id(), d.Id(), d.NodeId)
		err = volumeEntry.replaceBrickInVolume(db, executor, brickEntry.Id())
		if err != nil {
			return logger.Err(fmt.Errorf("Failed to remove device, error: %v", err))
		}
	}
	return nil
}

func DeviceEntryUpgrade(tx *bolt.Tx) error {
	return nil
}

// PendingOperationsOnDevice returns true if there are any pending operations
// whose bricks are linked to the given device. The error e will be non-nil
// if any db errors were encountered.
func PendingOperationsOnDevice(db wdb.RODB, deviceId string) (pdev bool, e error) {

	e = db.View(func(tx *bolt.Tx) error {
		pb, err := MapPendingBricks(tx)
		if err != nil {
			return err
		}
		for brickId, opId := range pb {
			b, err := NewBrickEntryFromId(tx, brickId)
			if err != nil {
				return err
			}
			if b.Info.DeviceId == deviceId {
				logger.Warning("Device %v used on pending brick %v in operation %v",
					deviceId, brickId, opId)
				pdev = true
				return nil
			}
		}
		pdr, err := MapPendingDeviceRemoves(tx)
		if err != nil {
			return err
		}
		if _, found := pdr[deviceId]; found {
			logger.Warning(
				"Device %v used in another pending device remove operation",
				deviceId)
			pdev = true
		}
		return nil
	})
	return
}

func (d *DeviceEntry) markFailed(db wdb.DB) error {
	// this is done on the ID in order to force a full fetch-check
	// inside one transaction
	err := markEmptyDeviceFailed(db, d.Info.Id)
	if err == nil {
		// update the in-memory device state to match
		// that in the db
		d.State = api.EntryStateFailed
	}
	return err
}

// markEmptyDeviceFailed takes a device id and, in one single
// transaction, checks if the device is valid for delete and
// if so marks it failed. If the change was applied the function
// returns nil. If ErrConflict is returned the device was not
// empty. Any other error is a database failure.
func markEmptyDeviceFailed(db wdb.DB, id string) error {
	return markDeviceFailed(db, id, false)
}

// markDeviceFailed takes a device id and a force flag,
// and in one transaction, checks the status of the device
// and if ready or force is set, sets the failed flag.
// If the change was applied the function
// returns nil. If ErrConflict is returned the device was not
// empty. Any other error is a database failure.
func markDeviceFailed(db wdb.DB, id string, force bool) error {
	return db.Update(func(tx *bolt.Tx) error {
		d, err := NewDeviceEntryFromId(tx, id)
		if err != nil {
			return err
		}
		if !force && d.HasBricks() {
			return ErrConflict
		}
		d.State = api.EntryStateFailed
		return d.Save(tx)
	})
}

func (d *DeviceEntry) DeleteBricksWithEmptyPath(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	var bricksToDelete []*BrickEntry

	logger.Debug("Deleting bricks with empty path on device [%v].",
		d.Info.Id)

	for _, id := range d.Bricks {
		brick, err := NewBrickEntryFromId(tx, id)
		if err == ErrNotFound {
			logger.Warning("Ignoring nonexistent brick [%v] on "+
				"disk [%v].", id, d.Info.Id)
			continue
		}
		if err != nil {
			logger.LogError("Unable to fetch brick [%v] from db: %v",
				id, err)
			return err
		}
		if brick.Info.Path == "" {
			bricksToDelete = append(bricksToDelete, brick)
		}
	}
	for _, brick := range bricksToDelete {
		logger.Debug("Deleting brick [%v] which has empty path.",
			brick.Info.Id)
		err := brick.Delete(tx)
		if err != nil {
			return logger.LogError("Unable to remove brick %v: %v", brick.Info.Id, err)
		}
		d.StorageFree(brick.TotalSize())
		d.BrickDelete(brick.Info.Id)
		err = d.Save(tx)
		if err != nil {
			logger.LogError("Unable to save device %v: %v", d.Info.Id, err)
			return err
		}
	}
	return nil
}

func (d *DeviceEntry) AllTags() map[string]string {
	if d.Info.Tags == nil {
		return map[string]string{}
	}
	return d.Info.Tags
}

func (d *DeviceEntry) SetTags(t map[string]string) error {
	d.Info.Tags = t
	return nil
}
