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
	"strings"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/idgen"
	"github.com/heketi/heketi/pkg/paths"
	"github.com/lpabon/godbc"
)

type BrickEntry struct {
	Info             api.BrickInfo
	TpSize           uint64
	PoolMetadataSize uint64
	gidRequested     int64
	Pending          PendingItem

	// the following is used when tracking the
	// bricks in cloned volumes. They follow a different
	// scheme than the bricks created directly by Heketi.
	LvmThinPool string
	LvmLv       string

	// currently sub type is only used when the brick is first created
	// this is only exported for placer use and db serialization
	SubType BrickSubType
}

func BrickList(tx *bolt.Tx) ([]string, error) {

	list := EntryKeys(tx, BOLTDB_BUCKET_BRICK)
	if list == nil {
		return nil, ErrAccessList
	}
	return list, nil
}

func NewBrickEntry(size, tpsize, poolMetadataSize uint64,
	deviceid, nodeid string, gid int64, volumeid string) *BrickEntry {

	godbc.Require(size > 0)
	godbc.Require(tpsize > 0)
	godbc.Require(deviceid != "")
	godbc.Require(nodeid != "")

	entry := &BrickEntry{}
	entry.gidRequested = gid
	entry.TpSize = tpsize
	entry.PoolMetadataSize = poolMetadataSize
	entry.Info.Id = idgen.GenUUID()
	entry.Info.Size = size
	entry.Info.NodeId = nodeid
	entry.Info.DeviceId = deviceid
	entry.Info.VolumeId = volumeid
	entry.LvmThinPool = paths.BrickIdToThinPoolName(entry.Info.Id)
	entry.UpdatePath()

	godbc.Ensure(entry.Info.Id != "")
	godbc.Ensure(entry.TpSize == tpsize)
	godbc.Ensure(entry.Info.Size == size)
	godbc.Ensure(entry.Info.NodeId == nodeid)
	godbc.Ensure(entry.Info.DeviceId == deviceid)

	return entry
}

func NewBrickEntryFromId(tx *bolt.Tx, id string) (*BrickEntry, error) {
	godbc.Require(tx != nil)

	entry := &BrickEntry{}
	err := EntryLoad(tx, entry, id)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func CloneBrickEntryFromId(tx *bolt.Tx, id string) (*BrickEntry, error) {
	godbc.Require(tx != nil)
	godbc.Require(id != "")

	entry := &BrickEntry{}
	err := EntryLoad(tx, entry, id)
	if err != nil {
		return nil, err
	}

	entry.Info.Id = idgen.GenUUID()
	// bricks always share a thin pool with the original brick
	if entry.LvmThinPool == "" {
		entry.LvmThinPool = paths.BrickIdToThinPoolName(entry.Info.Id)
	}
	// brick clones have their own lv name (not yet known)
	entry.LvmLv = ""

	return entry, nil
}

func (b *BrickEntry) BucketName() string {
	return BOLTDB_BUCKET_BRICK
}

func (b *BrickEntry) SetId(id string) {
	b.Info.Id = id
	b.UpdatePath()
}

func (b *BrickEntry) Id() string {
	return b.Info.Id
}

func (b *BrickEntry) Save(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	godbc.Require(len(b.Info.Id) > 0)

	return EntrySave(tx, b, b.Info.Id)
}

func (b *BrickEntry) Delete(tx *bolt.Tx) error {
	return EntryDelete(tx, b, b.Info.Id)
}

func (b *BrickEntry) NewInfoResponse(tx *bolt.Tx) (*api.BrickInfo, error) {
	info := &api.BrickInfo{}
	*info = b.Info

	return info, nil
}

func (b *BrickEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*b)

	return buffer.Bytes(), err
}

func (b *BrickEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(b)
	if err != nil {
		return err
	}

	return nil
}

func (b *BrickEntry) brickRequest(path string, create bool) *executors.BrickRequest {
	req := &executors.BrickRequest{}
	req.Gid = b.gidRequested
	req.Name = b.Info.Id
	req.Size = b.Info.Size
	req.TpSize = b.TpSize
	req.VgId = b.Info.DeviceId
	req.PoolMetadataSize = b.PoolMetadataSize
	req.TpName = b.TpName()
	req.LvName = b.LvName()
	// path varies depending on what it is called from
	req.Path = path
	// figure out how to format brick via subtype
	switch b.BrickType() {
	case NormalSubType:
		req.Format = executors.NormalFormat
	case ArbiterSubType:
		req.Format = executors.ArbiterFormat
	default:
		// this can only happen if we try to directly create a brick for
		// an entry that was not created by a current placer
		if create {
			panic("Can not create a brick of unknown type")
		}
	}
	return req
}

func (b *BrickEntry) Create(db wdb.RODB, executor executors.Executor) error {
	godbc.Require(db != nil)
	godbc.Require(b.TpSize > 0)
	godbc.Require(b.Info.Size > 0)
	godbc.Require(b.Info.Path != "")

	// Get node hostname
	var host string
	err := db.View(func(tx *bolt.Tx) error {
		node, err := NewNodeEntryFromId(tx, b.Info.NodeId)
		if err != nil {
			return err
		}

		host = node.ManageHostName()
		godbc.Check(host != "")
		return nil
	})
	if err != nil {
		return err
	}

	req := b.brickRequest(b.Info.Path, true)
	// remove this some time post-refactoring
	godbc.Require(req.Path == paths.BrickPath(req.VgId, req.Name))

	// Create brick on node
	logger.Info("Creating brick %v", b.Info.Id)
	_, err = executor.BrickCreate(host, req)
	if err != nil {
		return err
	}
	return nil
}

func (b *BrickEntry) Destroy(db wdb.RODB, executor executors.Executor) (bool, error) {

	godbc.Require(db != nil)
	godbc.Require(b.TpSize > 0)
	godbc.Require(b.Info.Size > 0)

	// Get node hostname
	var host string
	err := db.View(func(tx *bolt.Tx) error {
		node, err := NewNodeEntryFromId(tx, b.Info.NodeId)
		if err != nil {
			return err
		}

		host = node.ManageHostName()
		godbc.Check(host != "")
		return nil
	})
	if err != nil {
		return false, err
	}

	req := b.brickRequest(
		strings.TrimSuffix(b.Info.Path, "/brick"), false)

	// Delete brick on node
	logger.Info("Deleting brick %v", b.Info.Id)
	spaceReclaimed, err := executor.BrickDestroy(host, req)
	if err != nil {
		return spaceReclaimed, err
	}

	return spaceReclaimed, nil
}

func (b *BrickEntry) DestroyCheck(db wdb.RODB, executor executors.Executor) error {
	godbc.Require(db != nil)
	godbc.Require(b.TpSize > 0)
	godbc.Require(b.Info.Size > 0)

	// Get node hostname
	var host string
	err := db.View(func(tx *bolt.Tx) error {
		node, err := NewNodeEntryFromId(tx, b.Info.NodeId)
		if err != nil {
			return err
		}

		host = node.ManageHostName()
		godbc.Check(host != "")
		return nil
	})

	// TODO: any additional checks in the DB? The detection of the VG/LV and its users is done in cmdexec.BrickDestroy()
	return err
}

// Size consumed on device
func (b *BrickEntry) TotalSize() uint64 {
	return b.TpSize + b.PoolMetadataSize
}

func BrickEntryUpgrade(tx *bolt.Tx) error {
	err := addVolumeIdInBrickEntry(tx)
	if err != nil {
		return err
	}
	err = addSubTypeFieldFlagForBrickEntry(tx)
	if err != nil {
		return err
	}
	return nil
}

func addVolumeIdInBrickEntry(tx *bolt.Tx) error {
	volumes, err := VolumeList(tx)
	if err != nil {
		return err
	}
	for _, volume := range volumes {
		volumeEntry, err := NewVolumeEntryFromId(tx, volume)
		if err != nil {
			return err
		}
		for _, brick := range volumeEntry.Bricks {
			brickEntry, err := NewBrickEntryFromId(tx, brick)
			if err == ErrNotFound {
				logger.Warning("Volume [%v] links to "+
					"nonexistent brick [%v]. Ignoring.",
					volume, brick)
				continue
			}
			if err != nil {
				return err
			}
			if brickEntry.Info.VolumeId == "" {
				brickEntry.Info.VolumeId = volume
				err = brickEntry.Save(tx)
				if err != nil {
					return err
				}
			} else {
				break
			}
		}
	}
	return nil
}

func addSubTypeFieldFlagForBrickEntry(tx *bolt.Tx) error {
	entry, err := NewDbAttributeEntryFromKey(tx, DB_BRICK_HAS_SUBTYPE_FIELD)
	// This key won't exist if we are introducing the feature now
	if err != nil && err != ErrNotFound {
		return err
	}

	if err == ErrNotFound {
		// no flag in db. create it with default of "yes"
		entry = NewDbAttributeEntry()
		entry.Key = DB_BRICK_HAS_SUBTYPE_FIELD
		entry.Value = "yes"
		return entry.Save(tx)
	}
	return nil
}

func (b *BrickEntry) UpdatePath() {
	b.Info.Path = paths.BrickPath(b.Info.DeviceId, b.Info.Id)
}

func (b *BrickEntry) RemoveFromDevice(tx *bolt.Tx) error {
	// Access device
	device, err := NewDeviceEntryFromId(tx, b.Info.DeviceId)
	if err != nil {
		logger.Err(err)
		return err
	}

	// Delete brick from device
	device.BrickDelete(b.Info.Id)

	// Save device
	err = device.Save(tx)
	if err != nil {
		logger.Err(err)
		return err
	}

	return nil
}

// TpName returns the expected name of the lvm thin pool that
// stores this brick.
func (b *BrickEntry) TpName() string {
	if b.LvmThinPool != "" {
		return b.LvmThinPool
	}
	return paths.BrickIdToThinPoolName(b.Info.Id)
}

// LvName returns the expected name of the lvm lv that stores
// this brick.
func (b *BrickEntry) LvName() string {
	if b.LvmLv != "" {
		return b.LvmLv
	}
	return paths.BrickIdToName(b.Info.Id)
}

// BrickType returns the sub-type of a brick. SubType helps determine
// brick formatting, etc.
func (b *BrickEntry) BrickType() BrickSubType {
	return b.SubType
}
