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
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"
)

func (v *VolumeEntry) allocBricksInCluster(db wdb.DB,
	cluster string,
	gbsize int) ([]*BrickEntry, error) {

	size := uint64(gbsize) * GB

	// Setup a brick size generator
	// Note: subsequent calls to gen need to return decreasing
	//       brick sizes in order for the following code to work!
	gen := v.Durability.BrickSizeGenerator(size)

	// Try decreasing possible brick sizes until space is found
	for {
		// Determine next possible brick size
		sets, brick_size, err := gen()
		if err != nil {
			logger.Err(err)
			return nil, err
		}

		num_bricks := sets * v.Durability.BricksInSet()

		logger.Debug("brick_size = %v", brick_size)
		logger.Debug("sets = %v", sets)
		logger.Debug("num_bricks = %v", num_bricks)

		// Check that the volume would not have too many bricks
		if (num_bricks + len(v.Bricks)) > BrickMaxNum {
			logger.Debug("Maximum number of bricks reached")
			return nil, ErrMaxBricks
		}

		// Allocate bricks in the cluster
		brick_entries, err := v.allocBricks(db, cluster, sets, brick_size)
		if err == ErrNoSpace {
			logger.Debug("No space, re-trying with smaller brick size")
			continue
		}
		if err != nil {
			logger.Err(err)
			return nil, err
		}

		// We were able to allocate bricks
		return brick_entries, nil
	}
}

func (v *VolumeEntry) brickNameMap(db wdb.RODB) (
	map[string]*BrickEntry, error) {

	bmap := map[string]*BrickEntry{}

	err := db.View(func(tx *bolt.Tx) error {
		for _, brickid := range v.BricksIds() {
			brickEntry, err := NewBrickEntryFromId(tx, brickid)
			if err != nil {
				return err
			}
			nodeEntry, err := NewNodeEntryFromId(tx, brickEntry.Info.NodeId)
			if err != nil {
				return err
			}

			bname := fmt.Sprintf("%v:%v",
				nodeEntry.Info.Hostnames.Storage[0],
				brickEntry.Info.Path)
			bmap[bname] = brickEntry
		}
		return nil
	})
	return bmap, err
}

func (v *VolumeEntry) getBrickSetForBrickId(db wdb.DB,
	executor executors.Executor,
	oldBrickId string, node string) (*BrickSet, int, error) {

	// First gather the list of bricks from gluster which, unlike heketi,
	// retains the brick order
	vinfo, err := executor.VolumeInfo(node, v.Info.Name)
	if err != nil {
		logger.LogError("Unable to get volume info from gluster node %v for volume %v: %v", node, v.Info.Name, err)
		return nil, 0, err
	}

	var foundbrickset bool
	var oldBrickIndex int
	// BrickList in volume info is a slice of all bricks in volume
	// We loop over the slice in steps of BricksInSet()
	// If brick to be replaced is found in an iteration, other bricks in that slice form the Brick Set
	bmap, err := v.brickNameMap(db)
	if err != nil {
		return nil, 0, err
	}
	ssize := v.Durability.BricksInSet()
	for slicestartindex := 0; slicestartindex <= len(vinfo.Bricks.BrickList)-ssize; slicestartindex += ssize {
		bs := NewBrickSet(ssize)
		for _, brick := range vinfo.Bricks.BrickList[slicestartindex : slicestartindex+ssize] {
			brickentry, found := bmap[brick.Name]
			if !found {
				logger.LogError("Unable to create brick entry using brick name:%v",
					brick.Name)
				return nil, 0, ErrNotFound
			}
			if brickentry.Id() == oldBrickId {
				foundbrickset = true
				oldBrickIndex = len(bs.Bricks)
			}
			bs.Bricks = append(bs.Bricks, brickentry)
		}
		if foundbrickset {
			return bs, oldBrickIndex, nil
		}
	}

	logger.LogError("Unable to find brick set for brick %v, db is possibly corrupt", oldBrickId)
	return nil, 0, ErrNotFound
}

// canReplaceBrickInBrickSet
// check if a BrickSet is in a state where it's possible
// to replace a given one of its bricks:
// - no heals going on on the brick to be replaced
// - enough bricks of the set are up
func (v *VolumeEntry) canReplaceBrickInBrickSet(db wdb.DB,
	executor executors.Executor,
	node string,
	bs *BrickSet,
	index int) error {

	// Get self heal status for this brick's volume
	healinfo, err := executor.HealInfo(node, v.Info.Name)
	if err != nil {
		return err
	}

	var onlinePeerBrickCount = 0
	brickId := bs.Bricks[index].Id()
	bmap, err := v.brickNameMap(db)
	if err != nil {
		return err
	}
	for _, brickHealStatus := range healinfo.Bricks.BrickList {
		// Gluster has a bug that it does not send Name for bricks that are down.
		// Skip such bricks; it is safe because it is not source if it is down
		if brickHealStatus.Name == "information not available" {
			continue
		}
		iBrickEntry, found := bmap[brickHealStatus.Name]
		if !found {
			return fmt.Errorf("Unable to determine heal status of brick")
		}
		if iBrickEntry.Id() == brickId {
			// If we are here, it means the brick to be replaced is
			// up and running. We need to ensure that it is not a
			// source for any files.
			if brickHealStatus.NumberOfEntries != "-" &&
				brickHealStatus.NumberOfEntries != "0" {
				return fmt.Errorf("Cannot replace brick %v as it is source brick for data to be healed", iBrickEntry.Id())
			}
		}
		for i, brickInSet := range bs.Bricks {
			if i != index && brickInSet.Id() == iBrickEntry.Id() {
				onlinePeerBrickCount++
			}
		}
	}
	if onlinePeerBrickCount < v.Durability.QuorumBrickCount() {
		return fmt.Errorf("Cannot replace brick %v as only %v of %v "+
			"required peer bricks are online",
			brickId, onlinePeerBrickCount,
			v.Durability.QuorumBrickCount())
	}

	return nil
}

type replacementItems struct {
	oldBrickEntry     *BrickEntry
	oldDeviceEntry    *DeviceEntry
	oldBrickNodeEntry *NodeEntry
	bs                *BrickSet
	index             int
}

func (v *VolumeEntry) prepForBrickReplacement(db wdb.DB,
	executor executors.Executor,
	oldBrickId string) (ri replacementItems, node string, err error) {

	var oldBrickEntry *BrickEntry
	var oldDeviceEntry *DeviceEntry
	var oldBrickNodeEntry *NodeEntry

	err = db.View(func(tx *bolt.Tx) error {
		var err error
		oldBrickEntry, err = NewBrickEntryFromId(tx, oldBrickId)
		if err != nil {
			return err
		}

		oldDeviceEntry, err = NewDeviceEntryFromId(tx, oldBrickEntry.Info.DeviceId)
		if err != nil {
			return err
		}
		oldBrickNodeEntry, err = NewNodeEntryFromId(tx, oldBrickEntry.Info.NodeId)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return
	}

	node = oldBrickNodeEntry.ManageHostName()
	err = executor.GlusterdCheck(node)
	if err != nil {
		node, err = GetVerifiedManageHostname(db, executor, oldBrickNodeEntry.Info.ClusterId)
		if err != nil {
			return
		}
	}

	bs, index, err := v.getBrickSetForBrickId(db, executor, oldBrickId, node)
	if err != nil {
		return
	}

	err = v.canReplaceBrickInBrickSet(db, executor, node, bs, index)
	if err != nil {
		return
	}

	ri = replacementItems{
		oldBrickEntry:     oldBrickEntry,
		oldDeviceEntry:    oldDeviceEntry,
		oldBrickNodeEntry: oldBrickNodeEntry,
		bs:                bs,
		index:             index,
	}
	return
}

func (v *VolumeEntry) generateDeviceFilter(db wdb.RODB) (DeviceFilter, error) {

	var filter DeviceFilter = nil
	zoneChecking := v.GetZoneCheckingStrategy()
	if zoneChecking == ZONE_CHECKING_UNSET {
		zoneChecking = ZoneChecking
	}

	switch zoneChecking {
	case ZONE_CHECKING_STRICT:
		dzm, err := NewDeviceZoneMapFromDb(db)
		if err != nil {
			return nil, err
		}

		filter = dzm.Filter
	case ZONE_CHECKING_NONE:
	default:
		logger.Warning(
			"ZoneChecking set to unknown value '%v', "+
				"treating as 'none'", ZoneChecking)
	}

	return filter, nil
}

func (v *VolumeEntry) allocBrickReplacement(db wdb.DB,
	oldBrickEntry *BrickEntry,
	oldDeviceEntry *DeviceEntry,
	bs *BrickSet,
	index int) (newBrickEntry *BrickEntry,
	newDeviceEntry *DeviceEntry, err error) {

	var r *BrickAllocation
	err = db.Update(func(tx *bolt.Tx) error {
		// returns true if new device differs from old device
		diffDevice := func(bs *BrickSet, d *DeviceEntry) bool {
			return oldDeviceEntry.Info.Id != d.Info.Id
		}

		var err error
		txdb := wdb.WrapTx(tx)
		defaultFilter, err := v.generateDeviceFilter(txdb)
		if err != nil {
			return err
		}

		deviceFilter := func(bs *BrickSet, d *DeviceEntry) bool {
			if defaultFilter != nil && !defaultFilter(bs, d) {
				return false
			}

			return diffDevice(bs, d)
		}

		placer := PlacerForVolume(v)
		r, err = placer.Replace(
			NewClusterDeviceSource(tx, v.Info.Cluster),
			NewVolumePlacementOpts(v, oldBrickEntry.Info.Size, bs.SetSize),
			deviceFilter, bs, index)
		if err == ErrNoSpace {
			// swap error conditions to better match the intent
			return ErrNoReplacement
		} else if err != nil {
			return err
		}
		// Unfortunately, we need to save the updated device here in order
		// to preserve the space allocated for the new brick.
		if err := r.DeviceSets[0].Devices[index].Save(tx); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return
	}

	newBrickEntry = r.BrickSets[0].Bricks[index]
	newDeviceEntry = r.DeviceSets[0].Devices[index]
	return
}

func (v *VolumeEntry) replaceBrickInVolume(db wdb.DB, executor executors.Executor,
	oldBrickId string) (e error) {

	if api.DurabilityDistributeOnly == v.Info.Durability.Type {
		return fmt.Errorf("replace brick is not supported for volume durability type %v", v.Info.Durability.Type)
	}

	ri, node, err := v.prepForBrickReplacement(
		db, executor, oldBrickId)
	if err != nil {
		return err
	}
	// unpack the struct so we don't have to mess w/ the lower half of
	// this function
	oldBrickEntry := ri.oldBrickEntry
	oldDeviceEntry := ri.oldDeviceEntry
	oldBrickNodeEntry := ri.oldBrickNodeEntry

	newBrickEntry, newDeviceEntry, err := v.allocBrickReplacement(
		db, oldBrickEntry, oldDeviceEntry, ri.bs, ri.index)
	if err != nil {
		return err
	}

	defer func() {
		if e != nil {
			db.Update(func(tx *bolt.Tx) error {
				newDeviceEntry, err = NewDeviceEntryFromId(tx, newBrickEntry.Info.DeviceId)
				if err != nil {
					return err
				}
				newDeviceEntry.StorageFree(newBrickEntry.TotalSize())
				newDeviceEntry.Save(tx)
				return nil
			})
		}
	}()

	var newBrickNodeEntry *NodeEntry
	err = db.View(func(tx *bolt.Tx) error {
		newBrickNodeEntry, err = NewNodeEntryFromId(tx, newBrickEntry.Info.NodeId)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	brickEntries := []*BrickEntry{newBrickEntry}
	err = CreateBricks(db, executor, brickEntries)
	if err != nil {
		return err
	}

	defer func() {
		if e != nil {
			DestroyBricks(db, executor, brickEntries)
		}
	}()

	var oldBrick executors.BrickInfo
	var newBrick executors.BrickInfo

	oldBrick.Path = oldBrickEntry.Info.Path
	oldBrick.Host = oldBrickNodeEntry.StorageHostName()
	newBrick.Path = newBrickEntry.Info.Path
	newBrick.Host = newBrickNodeEntry.StorageHostName()

	err = executor.VolumeReplaceBrick(node, v.Info.Name, &oldBrick, &newBrick)
	if err != nil {
		return err
	}

	// After this point we should not call any defer func()
	// We don't have a *revert* of replace brick operation

	spaceReclaimed, err := oldBrickEntry.Destroy(db, executor)
	if err != nil {
		logger.LogError("Error destroying old brick: %v", err)
	}

	// We must read entries from db again as state on disk might
	// have changed

	err = db.Update(func(tx *bolt.Tx) error {
		err = newBrickEntry.Save(tx)
		if err != nil {
			return err
		}
		reReadNewDeviceEntry, err := NewDeviceEntryFromId(tx, newBrickEntry.Info.DeviceId)
		if err != nil {
			return err
		}
		reReadNewDeviceEntry.BrickAdd(newBrickEntry.Id())
		err = reReadNewDeviceEntry.Save(tx)
		if err != nil {
			return err
		}
		if spaceReclaimed {
			oldDevice2, err := NewDeviceEntryFromId(tx, oldBrickEntry.Info.DeviceId)
			if err != nil {
				return err
			}
			oldDevice2.StorageFree(oldBrickEntry.TotalSize())
			err = oldDevice2.Save(tx)
			if err != nil {
				return err
			}
		}

		reReadVolEntry, err := NewVolumeEntryFromId(tx, newBrickEntry.Info.VolumeId)
		if err != nil {
			return err
		}
		reReadVolEntry.BrickAdd(newBrickEntry.Id())
		err = oldBrickEntry.remove(tx, reReadVolEntry)
		if err != nil {
			return err
		}
		err = reReadVolEntry.Save(tx)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Err(err)
	}

	logger.Info("replaced brick:%v on node:%v at path:%v with brick:%v on node:%v at path:%v",
		oldBrickEntry.Id(), oldBrickEntry.Info.NodeId, oldBrickEntry.Info.Path,
		newBrickEntry.Id(), newBrickEntry.Info.NodeId, newBrickEntry.Info.Path)
	return nil
}

func (v *VolumeEntry) allocBricks(
	db wdb.DB,
	cluster string,
	bricksets int,
	brick_size uint64) (brick_entries []*BrickEntry, e error) {

	// Setup garbage collector function in case of error
	defer func() {

		// Check the named return value 'err'
		if e != nil {
			logger.Debug("Error detected.  Cleaning up volume %v: Len(%v) ", v.Info.Id, len(brick_entries))
			db.Update(func(tx *bolt.Tx) error {
				for _, brick := range brick_entries {
					brick.remove(tx, v)
				}
				return nil
			})
		}
	}()

	// mimic the previous unconditional db update behavior
	opts := NewVolumePlacementOpts(v, brick_size, bricksets)
	err := db.Update(func(tx *bolt.Tx) error {
		dsrc := NewClusterDeviceSource(tx, cluster)
		placer := PlacerForVolume(v)

		txdb := wdb.WrapTx(tx)
		deviceFilter, err := v.generateDeviceFilter(txdb)
		if err != nil {
			return err
		}

		r, e := placer.PlaceAll(dsrc, opts, deviceFilter)
		if e != nil {
			return e
		}
		brick_entries = []*BrickEntry{}
		for _, bs := range r.BrickSets {
			for _, x := range bs.Bricks {
				brick_entries = append(brick_entries, x)
				err := x.Save(tx)
				if err != nil {
					return err
				}
				logger.Debug("Adding brick %v to volume %v", x.Id(), v.Info.Id)
				v.BrickAdd(x.Id())
			}
		}
		for _, ds := range r.DeviceSets {
			for _, x := range ds.Devices {
				err := x.Save(tx)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return brick_entries, err
	}

	return brick_entries, nil
}
