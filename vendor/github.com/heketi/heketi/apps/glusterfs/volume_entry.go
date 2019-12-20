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
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/idgen"
	"github.com/heketi/heketi/pkg/paths"
	"github.com/heketi/heketi/pkg/sortedstrings"
	"github.com/lpabon/godbc"
)

const (

	// Byte values in KB
	KB = 1
	MB = KB * 1024
	GB = MB * 1024
	TB = GB * 1024

	// Default values
	DEFAULT_REPLICA               = 2
	DEFAULT_EC_DATA               = 4
	DEFAULT_EC_REDUNDANCY         = 2
	DEFAULT_THINP_SNAPSHOT_FACTOR = 1.5

	HEKETI_ID_KEY                = "user.heketi.id"
	HEKETI_ARBITER_KEY           = "user.heketi.arbiter"
	HEKETI_AVERAGE_FILE_SIZE_KEY = "user.heketi.average-file-size"
	HEKETI_ZONE_CHECKING_KEY     = "user.heketi.zone-checking"
)

var (
	// Average size of files on a volume, currently used only for arbiter sizing.
	// Might be used for other purposes later.
	averageFileSize uint64 = 64 * KB
)

// VolumeEntry struct represents a volume in heketi. Serialization is done using
// gob when written to db and using json package when exportdb/importdb is used
// There are two reasons I skip Durability field for json pkg
//   1. Durability is used in some places in code, however, it represents the
//      same info that is in Info.Durability.
//   2. I wasn't able to serialize interface type to json in a straightfoward
//      way.
// Chose to skip writing redundant data than adding kludgy code
type VolumeEntry struct {
	Info                 api.VolumeInfo
	Bricks               sort.StringSlice
	Durability           VolumeDurability `json:"-"`
	GlusterVolumeOptions []string
	Pending              PendingItem
}

func VolumeList(tx *bolt.Tx) ([]string, error) {

	list := EntryKeys(tx, BOLTDB_BUCKET_VOLUME)
	if list == nil {
		return nil, ErrAccessList
	}
	return list, nil
}

func NewVolumeEntry() *VolumeEntry {
	entry := &VolumeEntry{}
	entry.Bricks = make(sort.StringSlice, 0)

	return entry
}

func NewVolumeEntryFromRequest(req *api.VolumeCreateRequest) *VolumeEntry {
	godbc.Require(req != nil)

	vol := NewVolumeEntry()
	vol.Info.Gid = req.Gid
	vol.Info.Id = idgen.GenUUID()
	vol.Info.Durability = req.Durability
	vol.Info.Snapshot = req.Snapshot
	vol.Info.Size = req.Size
	vol.Info.Block = req.Block

	// Set default durability values
	durability := vol.Info.Durability.Type
	switch {

	case durability == api.DurabilityReplicate:
		logger.Debug("[%v] Replica %v",
			vol.Info.Id,
			vol.Info.Durability.Replicate.Replica)
		vol.Durability = NewVolumeReplicaDurability(&vol.Info.Durability.Replicate)

	case durability == api.DurabilityEC:
		logger.Debug("[%v] EC %v + %v ",
			vol.Info.Id,
			vol.Info.Durability.Disperse.Data,
			vol.Info.Durability.Disperse.Redundancy)
		vol.Durability = NewVolumeDisperseDurability(&vol.Info.Durability.Disperse)

	case durability == api.DurabilityDistributeOnly || durability == "":
		logger.Debug("[%v] Distributed", vol.Info.Id)
		vol.Durability = NewNoneDurability()

	default:
		panic(fmt.Sprintf("BUG: Unknown type: %v\n", vol.Info.Durability))
	}

	// Set the default values accordingly
	vol.Durability.SetDurability()

	// Set default name
	if req.Name == "" {
		vol.Info.Name = "vol_" + vol.Info.Id
	} else {
		vol.Info.Name = req.Name
	}

	// Set default thinp factor
	if vol.Info.Snapshot.Enable && vol.Info.Snapshot.Factor == 0 {
		vol.Info.Snapshot.Factor = DEFAULT_THINP_SNAPSHOT_FACTOR
	} else if !vol.Info.Snapshot.Enable {
		vol.Info.Snapshot.Factor = 1
	}

	// If it is zero, then no volume options are set.
	vol.GlusterVolumeOptions = req.GlusterVolumeOptions

	if vol.Info.Block {
		if err := vol.SetRawCapacity(req.Size); err != nil {
			logger.Err(err)
			// we can either panic here or return nil. We panic because
			// returning nil is most likely going to lead to nil
			// dereference panics anyway
			panic(err)
		}
		// prepend the gluster-block group option,
		// so that the user-specified options can take precedence
		blockoptions := strings.Split(BlockHostingVolumeOptions, ",")
		vol.GlusterVolumeOptions = append(
			blockoptions,
			vol.GlusterVolumeOptions...)
	}

	// Add volume options using PreRequestVolumeOptions, this must be
	// set before volume options from the request are set.
	preReqVolumeOptions := strings.Split(PreReqVolumeOptions, ",")
	vol.GlusterVolumeOptions = append(preReqVolumeOptions,
		vol.GlusterVolumeOptions...)

	// Add volume options using PostRequestVolumeOptions, this must be
	// set after volume options from the request are set.
	postReqVolumeOptions := strings.Split(PostReqVolumeOptions, ",")
	vol.GlusterVolumeOptions = append(vol.GlusterVolumeOptions,
		postReqVolumeOptions...)

	// If it is zero, then it will be assigned during volume creation
	vol.Info.Clusters = req.Clusters

	return vol
}

func NewVolumeEntryFromId(tx *bolt.Tx, id string) (*VolumeEntry, error) {
	godbc.Require(tx != nil)

	entry := NewVolumeEntry()
	err := EntryLoad(tx, entry, id)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func NewVolumeEntryFromClone(v *VolumeEntry, name string) *VolumeEntry {
	entry := NewVolumeEntry()

	entry.Info.Id = idgen.GenUUID()
	if name == "" {
		entry.Info.Name = "vol_" + entry.Info.Id
	} else {
		entry.Info.Name = name
	}

	entry.GlusterVolumeOptions = v.GlusterVolumeOptions
	entry.Info.Cluster = v.Info.Cluster
	entry.Info.Durability = v.Info.Durability
	entry.Info.Durability.Type = v.Info.Durability.Type
	entry.Info.Gid = v.Info.Gid
	entry.Info.Mount = v.Info.Mount
	entry.Info.Size = v.Info.Size
	entry.Info.Snapshot = v.Info.Snapshot
	copy(entry.Info.Mount.GlusterFS.Hosts, v.Info.Mount.GlusterFS.Hosts)
	entry.Info.Mount.GlusterFS.MountPoint = v.Info.Mount.GlusterFS.Hosts[0] + ":" + entry.Info.Name
	entry.Info.Mount.GlusterFS.Options = v.Info.Mount.GlusterFS.Options
	entry.Info.BlockInfo.FreeSize = v.Info.BlockInfo.FreeSize
	copy(entry.Info.BlockInfo.BlockVolumes, v.Info.BlockInfo.BlockVolumes)

	// entry.Bricks is still empty, these need to be filled by the caller
	return entry
}

func (v *VolumeEntry) BucketName() string {
	return BOLTDB_BUCKET_VOLUME
}

func (v *VolumeEntry) Save(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	godbc.Require(len(v.Info.Id) > 0)

	return EntrySave(tx, v, v.Info.Id)
}

func (v *VolumeEntry) Delete(tx *bolt.Tx) error {
	return EntryDelete(tx, v, v.Info.Id)
}

func (v *VolumeEntry) NewInfoResponse(tx *bolt.Tx) (*api.VolumeInfoResponse, error) {
	godbc.Require(tx != nil)

	info := api.NewVolumeInfoResponse()
	info.Id = v.Info.Id
	info.Cluster = v.Info.Cluster
	if err := v.updateMountInfo(wdb.WrapTx(tx), &info.VolumeInfo); err != nil {
		return nil, err
	}
	info.Snapshot = v.Info.Snapshot
	info.Size = v.Info.Size
	info.Durability = v.Info.Durability
	info.Name = v.Info.Name
	info.GlusterVolumeOptions = v.GlusterVolumeOptions
	info.Block = v.Info.Block
	info.BlockInfo = v.Info.BlockInfo
	info.Gid = v.Info.Gid

	for _, brickid := range v.BricksIds() {
		brick, err := NewBrickEntryFromId(tx, brickid)
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

func (v *VolumeEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*v)

	return buffer.Bytes(), err
}

func (v *VolumeEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(v)
	if err != nil {
		return err
	}

	// Make sure to setup arrays if nil
	if v.Bricks == nil {
		v.Bricks = make(sort.StringSlice, 0)
	}

	return nil
}

// volOptsMap returns the volume options as a map of space separated
// key-value pairs. Keys that are found later in the list will overwrite
// the same key if it occurred earlier in the list. Strings that lack a
// space separating key from value will be treated as a key  with a
// value of "".
func (v *VolumeEntry) volOptsMap() map[string]string {
	om := map[string]string{}
	for _, s := range v.GlusterVolumeOptions {
		r := strings.SplitN(s, " ", 2)
		if len(r) == 2 {
			om[r[0]] = r[1]
		} else {
			om[r[0]] = ""
		}
	}
	return om
}

// HasArbiterOption returns true if this volume is flagged for
// arbiter support.
func (v *VolumeEntry) HasArbiterOption() bool {
	value := v.volOptsMap()[HEKETI_ARBITER_KEY]
	if b, e := strconv.ParseBool(value); e == nil {
		return b
	}
	return false
}

// GetAverageFileSize returns averageFileSize provided by user or default averageFileSize
func (v *VolumeEntry) GetAverageFileSize() uint64 {
	value := v.volOptsMap()[HEKETI_AVERAGE_FILE_SIZE_KEY]
	if size, e := strconv.ParseUint(value, 10, 64); e == nil {
		if size == 0 {
			logger.LogError("Average File Size cannot be zero, using default file size %v", averageFileSize)
			return averageFileSize
		}
		return size
	}
	return averageFileSize
}

// GetZoneCheckingStrategy returns a ZoneCheckingStrategy based on
// the volume's options.
func (v *VolumeEntry) GetZoneCheckingStrategy() ZoneCheckingStrategy {
	value := v.volOptsMap()[HEKETI_ZONE_CHECKING_KEY]
	if value != "" {
		return ZoneCheckingStrategy(value)
	}
	return ZONE_CHECKING_UNSET
}

func (v *VolumeEntry) BrickAdd(id string) {
	godbc.Require(!sortedstrings.Has(v.Bricks, id))

	v.Bricks = append(v.Bricks, id)
	v.Bricks.Sort()
}

func (v *VolumeEntry) BrickDelete(id string) {
	v.Bricks = sortedstrings.Delete(v.Bricks, id)
}

func (v *VolumeEntry) Create(db wdb.DB,
	executor executors.Executor) (e error) {

	return RunOperation(
		NewVolumeCreateOperation(v, db),
		executor)
}

// ModifyFreeSize adjusts the free size of a block hosting volume.
// When taking space from the volume the value must be negative (on
// block volume add) and positive when the space is being "freed."
func (v *VolumeEntry) ModifyFreeSize(delta int) error {
	logger.Debug("Modifying free size: FreeSize=[%v] delta=[%v]",
		v.Info.BlockInfo.FreeSize, delta)
	v.Info.BlockInfo.FreeSize += delta
	if v.Info.BlockInfo.FreeSize < 0 {
		return logger.Err(fmt.Errorf(
			"Volume %v free size may not be set less than zero", v.Info.Id))
	}
	if v.Info.BlockInfo.FreeSize+v.Info.BlockInfo.ReservedSize > v.Info.Size {
		return logger.Err(fmt.Errorf(
			"Volume %v free size may not be set greater than %v",
			v.Info.Id, v.Info.Size))
	}
	return nil
}

func (v *VolumeEntry) ModifyReservedSize(delta int) error {
	logger.Debug("Modifying reserved size: ReservedSize=[%v] delta=[%v]",
		v.Info.BlockInfo.ReservedSize, delta)
	v.Info.BlockInfo.ReservedSize += delta
	if v.Info.BlockInfo.ReservedSize < 0 {
		return logger.Err(fmt.Errorf(
			"Volume %v reserved size may not be set less than zero", v.Info.Id))
	}
	if v.Info.BlockInfo.ReservedSize+v.Info.BlockInfo.FreeSize > v.Info.Size {
		return logger.Err(fmt.Errorf(
			"Volume %v reserved size may not be set greater than %v",
			v.Info.Id, v.Info.Size))
	}
	return nil
}

//ReduceRawSize reserves 2% of size for block volume creation
func ReduceRawSize(size int) int {
	return size * 98 / 100
}

// AddRawCapacity adds raw capacity to the BlockInfo
// FreeSize tracking, reserving one percent of the
// raw capacity for the filesystem.
func (v *VolumeEntry) AddRawCapacity(delta int) error {
	var freeDelta int
	var reservedDelta int

	freeDelta = ReduceRawSize(delta)
	reservedDelta = delta - freeDelta

	if err := v.ModifyFreeSize(freeDelta); err != nil {
		return err
	}
	if err := v.ModifyReservedSize(reservedDelta); err != nil {
		return err
	}
	return nil
}

func (v *VolumeEntry) SetRawCapacity(delta int) error {
	v.Info.BlockInfo.FreeSize = 0
	v.Info.BlockInfo.ReservedSize = 0
	return v.AddRawCapacity(delta)
}

// TotalSizeBlockVolumes returns the total size of the block volumes that
// the given volume is hosting. This function iterates over the block
// volumes in the db to calculate the total.
func (v *VolumeEntry) TotalSizeBlockVolumes(tx *bolt.Tx) (int, error) {
	if !v.Info.Block {
		return 0, fmt.Errorf(
			"Volume %v is not a block hosting volume", v.Info.Id)
	}
	bvsum := 0
	for _, bvid := range v.Info.BlockInfo.BlockVolumes {
		bvol, err := NewBlockVolumeEntryFromId(tx, bvid)
		if err != nil {
			return 0, err
		}
		bvsum += bvol.Info.Size
	}
	return bvsum, nil
}

// blockHostingSizeIsCorrect returns true if the total size of the volume
// is equal to the sum of the used, free and reserved block hosting size values.
// The used size must be provided and should be calculated based on the sizes
// of the block volumes.
func (v *VolumeEntry) blockHostingSizeIsCorrect(used int) bool {
	logger.Debug("volume [%v]: comparing %v == %v + %v + %v",
		v.Info.Id, v.Info.Size,
		used, v.Info.BlockInfo.FreeSize, v.Info.BlockInfo.ReservedSize)
	unused := v.Info.BlockInfo.FreeSize + v.Info.BlockInfo.ReservedSize
	if v.Info.Size != (used + unused) {
		logger.Warning("detected volume [%v] has size %v != %v + %v + %v",
			v.Info.Id, v.Info.Size,
			used, v.Info.BlockInfo.FreeSize, v.Info.BlockInfo.ReservedSize)
		return false
	}
	return true
}

func (v *VolumeEntry) tryAllocateBricks(
	db wdb.DB,
	possibleClusters []string) (brick_entries []*BrickEntry, err error) {

	cerr := ClusterErrorMap{}
	for _, cluster := range possibleClusters {
		// Check this cluster for space
		brick_entries, err = v.allocBricksInCluster(db, cluster, v.Info.Size)

		if err == nil {
			v.Info.Cluster = cluster
			logger.Debug("Volume to be created on cluster %v", cluster)
			break
		} else if err == ErrNoSpace ||
			err == ErrMaxBricks ||
			err == ErrMinimumBrickSize {
			logger.Debug("Cluster %v can not accommodate volume "+
				"(%v), trying next cluster", cluster, err)
			// Map these errors to NoSpace here as that is what heketi
			// traditionally did. Its not particularly helpful but it
			// is more backwards compatible.
			cerr.Add(cluster, ErrNoSpace)
		} else if err == ErrEmptyCluster ||
			err == ErrNoStorage {
			logger.Debug("Issue on cluster %v: %v", cluster, err)
			cerr.Add(cluster, err)
		} else {
			// A genuine error occurred - bail out
			logger.LogError("Error calling v.allocBricksInCluster: %v", err)
			return
		}
	}
	// if our last attempt failed and we collected at least one error
	// return the short form all the errors we collected
	if err != nil && len(cerr) > 0 {
		err = cerr.ToError("Unable to create volume on any cluster:")
	}
	return
}

// checkDestroyVolumeFromHost returns an error if the volume may
// not be destroyed at this time. Nil is returned if the volume
// may be destroyed.
func (v *VolumeEntry) checkDestroyVolumeFromHost(
	executor executors.Executor, h string) error {

	err := executor.VolumeDestroyCheck(h, v.Info.Name)
	if _, ok := err.(*executors.VolumeDoesNotExistErr); ok {
		logger.Warning("volume %v not present in gluster", v.Info.Id)
		return nil
	}
	return err
}

func (v *VolumeEntry) destroyVolumeFromHost(
	executor executors.Executor, h string) error {

	err := executor.VolumeDestroy(h, v.Info.Name)
	switch {
	case err == nil:
		// no errors, so we just deleted the volume from gluster
		return nil
	case strings.Contains(err.Error(), "does not exist"):
		// we asked gluster to delete a volume that already does not exist
		return nil
	default:
		logger.Warning("failed to delete volume %v via %v: %v",
			v.Info.Id, h, err)
		return err
	}
}

func (v *VolumeEntry) destroyGlusterVolume(
	db wdb.RODB, executor executors.Executor) error {

	hosts, err := v.hosts(db)
	if err != nil {
		return err
	}
	err = newTryOnHosts(hosts).run(func(h string) error {
		return v.destroyVolumeFromHost(executor, h)
	})
	if err != nil {
		logger.LogError("failed to delete volume in cleanup: %v", err)
		return fmt.Errorf("failed to clean up volume: %v", v.Info.Id)
	}
	return nil
}

func (v *VolumeEntry) cleanupCreateVolume(db wdb.DB,
	executor executors.Executor,
	brick_entries []*BrickEntry) error {

	if err := v.destroyGlusterVolume(db, executor); err != nil {
		return err
	}

	// from a quick read its "safe" to unconditionally try to delete
	// bricks. TODO: find out if that is true with functional tests
	reclaimed, err := DestroyBricks(db, executor, brick_entries)
	if err != nil {
		logger.LogError("failed to destroy bricks during cleanup: %v", err)
	}
	return v.teardown(db, brick_entries, reclaimed)
}

func (v *VolumeEntry) createVolumeComponents(
	db wdb.DB) ([]*BrickEntry, error) {

	// Get list of clusters
	var possibleClusters []string
	if len(v.Info.Clusters) == 0 {
		err := db.View(func(tx *bolt.Tx) error {
			var err error
			possibleClusters, err = ClusterList(tx)
			return err
		})
		if err != nil {
			return nil, err
		}
	} else {
		possibleClusters = v.Info.Clusters
	}

	cr := clusterReq{
		allowBlock:  v.Info.Block,
		allowName:   v.Info.Name,
		allowCreate: true,
	}
	possibleClusters, err := eligibleClusters(db, cr, possibleClusters)
	if err != nil {
		return nil, err
	}
	logger.Debug("Using the following clusters: %+v", possibleClusters)

	return v.saveCreateVolume(db, possibleClusters)
}

func (v *VolumeEntry) createVolumeExec(db wdb.DB,
	executor executors.Executor,
	brick_entries []*BrickEntry) (e error) {

	// Create the bricks on the nodes
	e = CreateBricks(db, executor, brick_entries)
	if e != nil {
		return
	}

	// Create GlusterFS volume
	return v.createVolume(db, executor, brick_entries)
}

func (v *VolumeEntry) saveCreateVolume(db wdb.DB,
	possibleClusters []string) (brick_entries []*BrickEntry, err error) {

	err = db.Update(func(tx *bolt.Tx) error {
		txdb := wdb.WrapTx(tx)
		// For each cluster look for storage space for this volume
		brick_entries, err = v.tryAllocateBricks(txdb, possibleClusters)
		if err != nil {
			return err
		}
		if brick_entries == nil {
			// Map all 'valid' errors to NoSpace here:
			// Only the last such error could get propagated down,
			// so it does not make sense to hand the granularity on.
			// But for other callers (Expand), we keep it.
			return ErrNoSpace
		}

		err = v.updateMountInfo(txdb, &v.Info)
		if err != nil {
			return err
		}

		// Save volume information
		if v.Info.Block {
			if err := v.SetRawCapacity(v.Info.Size); err != nil {
				return err
			}
		}
		err := v.Save(tx)
		if err != nil {
			return err
		}

		// Save cluster
		cluster, err := NewClusterEntryFromId(tx, v.Info.Cluster)
		if err != nil {
			return err
		}
		cluster.VolumeAdd(v.Info.Id)
		return cluster.Save(tx)
	})
	return
}

func (v *VolumeEntry) deleteVolumeExec(db wdb.RODB,
	executor executors.Executor,
	brick_entries []*BrickEntry,
	sshhost string) (map[string]bool, error) {

	if err := v.checkDestroyVolumeFromHost(executor, sshhost); err != nil {
		return nil, err
	}
	if err := v.destroyVolumeFromHost(executor, sshhost); err != nil {
		logger.LogError("Unable to delete volume: %v", err)
		return nil, err
	}

	// Destroy bricks
	space_reclaimed, err := DestroyBricks(db, executor, brick_entries)
	if err != nil {
		logger.LogError("Unable to delete bricks: %v", err)
		return nil, err
	}

	return space_reclaimed, nil
}

// teardown removes a volume and the volume's bricks from the database.
// It updates related objects meta-data such as cluster contents and
// device used/free sizes.
func (v *VolumeEntry) teardown(
	db wdb.DB, brick_entries []*BrickEntry, reclaimed ReclaimMap) error {

	return db.Update(func(tx *bolt.Tx) error {
		for _, brick := range brick_entries {
			err := brick.removeAndFree(tx, v, reclaimed[brick.Info.DeviceId])
			if err != nil {
				return err
			}
		}

		if v.Info.Cluster != "" {
			cluster, err := NewClusterEntryFromId(tx, v.Info.Cluster)
			if err != nil {
				return err
			}
			cluster.VolumeDelete(v.Info.Id)
			err = cluster.Save(tx)
			if err != nil {
				return err
			}
		}
		return v.Delete(tx)
	})
}

func (v *VolumeEntry) manageHostFromBricks(db wdb.DB,
	brick_entries []*BrickEntry) (sshhost string, err error) {

	err = db.View(func(tx *bolt.Tx) error {
		for _, brick := range brick_entries {
			node, err := NewNodeEntryFromId(tx, brick.Info.NodeId)
			if err != nil {
				return err
			}
			sshhost = node.ManageHostName()
			return nil
		}
		return fmt.Errorf("Unable to get management host from bricks")
	})
	return
}

func (v *VolumeEntry) deleteVolumeComponents(
	db wdb.RODB) (brick_entries []*BrickEntry, e error) {

	e = db.View(func(tx *bolt.Tx) error {
		for _, id := range v.BricksIds() {
			brick, err := NewBrickEntryFromId(tx, id)
			if err != nil {
				logger.LogError("Brick %v not found in db: %v", id, err)
				return err
			}
			brick_entries = append(brick_entries, brick)
		}
		return nil
	})
	return
}

func (v *VolumeEntry) Destroy(db wdb.DB, executor executors.Executor) error {
	logger.Info("Destroying volume %v", v.Info.Id)

	return RunOperation(
		NewVolumeDeleteOperation(v, db),
		executor)
}

func (v *VolumeEntry) expandVolumeComponents(db wdb.DB,
	sizeGB int,
	setSize bool) (brick_entries []*BrickEntry, e error) {

	e = db.Update(func(tx *bolt.Tx) error {
		// Allocate new bricks in the cluster
		txdb := wdb.WrapTx(tx)
		var err error
		brick_entries, err = v.allocBricksInCluster(txdb, v.Info.Cluster, sizeGB)
		if err != nil {
			return err
		}

		// Increase the recorded volume size
		if setSize {
			v.Info.Size += sizeGB
		}

		// Save brick entries
		for _, brick := range brick_entries {
			err := brick.Save(tx)
			if err != nil {
				return err
			}
		}

		return v.Save(tx)
	})
	return
}

func (v *VolumeEntry) expandVolumeExec(db wdb.DB,
	executor executors.Executor,
	brick_entries []*BrickEntry) (e error) {

	// Create bricks
	err := CreateBricks(db, executor, brick_entries)
	if err != nil {
		return err
	}

	// Create a volume request to send to executor
	// so that it can add the new bricks
	vr, host, err := v.createVolumeRequest(db, brick_entries)
	if err != nil {
		return err
	}

	// Expand the volume
	_, err = executor.VolumeExpand(host, vr)
	if err != nil {
		return err
	}

	return err
}

func (v *VolumeEntry) Expand(db wdb.DB,
	executor executors.Executor,
	sizeGB int) (e error) {

	return RunOperation(
		NewVolumeExpandOperation(v, db, sizeGB),
		executor)
}

func (v *VolumeEntry) BricksIds() sort.StringSlice {
	ids := make(sort.StringSlice, len(v.Bricks))
	copy(ids, v.Bricks)
	return ids
}

func VolumeEntryUpgrade(tx *bolt.Tx) error {
	return nil
}

func (v *VolumeEntry) BlockVolumeAdd(id string) {
	v.Info.BlockInfo.BlockVolumes = append(v.Info.BlockInfo.BlockVolumes, id)
	v.Info.BlockInfo.BlockVolumes.Sort()
}

func (v *VolumeEntry) BlockVolumeDelete(id string) {
	v.Info.BlockInfo.BlockVolumes = sortedstrings.Delete(v.Info.BlockInfo.BlockVolumes, id)
}

// Visible returns true if this volume is meant to be visible to
// API calls.
func (v *VolumeEntry) Visible() bool {
	return v.Pending.Id == ""
}

func volumeNameExistsInCluster(tx *bolt.Tx, cluster *ClusterEntry,
	name string) (found bool, e error) {
	for _, volumeId := range cluster.Info.Volumes {
		volume, err := NewVolumeEntryFromId(tx, volumeId)
		if err != nil {
			return false, err
		}
		if name == volume.Info.Name {
			found = true
			return
		}
	}

	return
}

type clusterReq struct {
	allowBlock  bool
	allowName   string
	allowCreate bool
}

func eligibleClusters(db wdb.RODB, req clusterReq,
	possibleClusters []string) ([]string, error) {
	//
	// If the request carries the Block flag, consider only
	// those clusters that carry the Block flag if there are
	// any, otherwise consider all clusters.
	// If the request does *not* carry the Block flag, consider
	// only those clusters that do not carry the Block flag.
	//
	if len(possibleClusters) == 0 {
		return nil, fmt.Errorf("No clusters configured")
	}
	candidateClusters := []string{}
	cerr := ClusterErrorMap{}
	err := db.View(func(tx *bolt.Tx) error {
		for _, clusterId := range possibleClusters {
			c, err := NewClusterEntryFromId(tx, clusterId)
			if err != nil {
				return err
			}
			switch {
			case req.allowBlock && c.Info.Block:
			case !req.allowBlock && c.Info.File:
			case !(c.Info.Block || c.Info.File):
				// possibly bad cluster config
				logger.Info("Cluster %v lacks both block and file flags",
					clusterId)
				cerr.Add(
					c.Info.Id,
					fmt.Errorf("Cluster has disabled all volume types"))
				continue
			default:
				cerr.Add(
					c.Info.Id,
					fmt.Errorf("Cluster does not support requested volume type"))
				continue
			}
			if req.allowName != "" {
				found, err := volumeNameExistsInCluster(tx, c, req.allowName)
				if err != nil {
					return err
				}
				if found {
					logger.LogError("Name %v already in use in cluster %v",
						req.allowName, clusterId)
					cerr.Add(
						c.Info.Id,
						fmt.Errorf("Volume name '%v' already in use", req.allowName))
					continue
				}
			}
			if req.allowCreate && c.volumeCount() >= maxVolumesPerCluster {
				cerr.Add(
					c.Info.Id,
					fmt.Errorf("Cluster has %v volumes and limit is %v", c.volumeCount(), maxVolumesPerCluster))
				continue
			}
			candidateClusters = append(candidateClusters, clusterId)
		}
		return nil
	})

	if err == nil && len(candidateClusters) == 0 {
		logger.LogError("No clusters eligible to satisfy create volume request")
		// use generic "no space" error if cluster errors is empty
		err = ErrNoSpace
		if len(cerr) > 0 {
			err = cerr.ToError("No eligible cluster for volume")
		}
	}
	return candidateClusters, err
}

// hosts returns a node-to-host mapping for all nodes in the
// volume's cluster. These hosts can be used as destinations
// for gluster commands.
func (v *VolumeEntry) hosts(db wdb.RODB) (nodeHosts, error) {
	var hosts nodeHosts
	err := db.View(func(tx *bolt.Tx) error {
		vol, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		cluster, err := NewClusterEntryFromId(tx, vol.Info.Cluster)
		if err != nil {
			return err
		}
		hosts, err = cluster.hosts(wdb.WrapTx(tx))
		return err
	})
	return hosts, err
}

func (v *VolumeEntry) prepareVolumeClone(tx *bolt.Tx, clonename string) (
	*VolumeEntry, []*BrickEntry, []*DeviceEntry, error) {

	if v.Info.Block {
		return nil, nil, nil, ErrCloneBlockVol
	}
	bricks := []*BrickEntry{}
	devices := []*DeviceEntry{}
	cvol := NewVolumeEntryFromClone(v, clonename)
	for _, brickId := range v.Bricks {
		brick, err := CloneBrickEntryFromId(tx, brickId)
		if err != nil {
			return nil, nil, nil, err
		}
		device, err := NewDeviceEntryFromId(tx, brick.Info.DeviceId)
		if err != nil {
			return nil, nil, nil, err
		}

		brick.Info.VolumeId = cvol.Info.Id

		cvol.Bricks = append(cvol.Bricks, brick.Id())
		bricks = append(bricks, brick)

		// Add the cloned brick to the device (clones do not take extra storage space)
		device.BrickAdd(brick.Id())
		devices = append(devices, device)
	}
	return cvol, bricks, devices, nil
}

func updateCloneBrickPaths(bricks []*BrickEntry,
	orig, clone *executors.Volume) error {

	pathIndex := map[string]int{}
	for i, brick := range bricks {
		pathIndex[brick.Info.Path] = i
	}
	if len(pathIndex) != len(bricks) {
		return fmt.Errorf(
			"Unexpected number of brick paths. %v unique paths, %v bricks",
			len(pathIndex), len(bricks))
	}

	for i, b := range orig.Bricks.BrickList {
		c := clone.Bricks.BrickList[i]
		origPath := strings.Split(b.Name, ":")[1]
		clonePath := strings.Split(c.Name, ":")[1]

		bidx, ok := pathIndex[origPath]
		if !ok {
			return fmt.Errorf(
				"Failed to find brick path %v in known brick paths",
				origPath)
		}
		brick := bricks[bidx]
		logger.Debug("Updating brick %v with new path %v (had %v)",
			brick.Id(), clonePath, origPath)
		brick.Info.Path = clonePath
		brick.LvmLv = paths.VolumeIdToCloneLv(clone.ID)
	}
	return nil
}

func (v *VolumeEntry) cloneVolumeRequest(db wdb.RODB, clonename string) (*executors.VolumeCloneRequest, string, error) {
	godbc.Require(db != nil)
	godbc.Require(clonename != "")

	// Setup list of bricks
	vcr := &executors.VolumeCloneRequest{}
	vcr.Volume = v.Info.Name
	vcr.Clone = clonename

	var sshhost string
	err := db.View(func(tx *bolt.Tx) error {
		vol, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		cluster, err := NewClusterEntryFromId(tx, vol.Info.Cluster)
		if err != nil {
			return err
		}

		// TODO: verify if the node is available/online?
		// picking the 1st node for now...
		node, err := NewNodeEntryFromId(tx, cluster.Info.Nodes[0])
		if err != nil {
			return err
		}
		sshhost = node.ManageHostName()

		return nil
	})
	if err != nil {
		return nil, "", err
	}

	if sshhost == "" {
		return nil, "", errors.New("failed to find host for cloning volume " + v.Info.Name)
	}

	return vcr, sshhost, nil
}

// consistencyCheck ... verifies that a volumeEntry is consistent with rest of the database.
// It is a method on volumeEntry and needs rest of the database as its input.
func (v *VolumeEntry) consistencyCheck(db Db) (response DbEntryCheckResponse) {

	var aggregateBlockVolumesSize = 0

	// No consistency check required for following attributes
	// Id
	// Name
	// Durability
	// GlusterVolumeOptions
	// Gid

	// PendingId
	if v.Pending.Id != "" {
		response.Pending = true
		if _, found := db.PendingOperations[v.Pending.Id]; !found {
			response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("Volume %v marked pending but no pending op %v", v.Info.Id, v.Pending.Id))
		}
		// TODO: Validate back the pending operations' relationship to the volume
		// This is skipped because some of it is handled in auto cleanup code.
	}

	// Cluster and mount hosts
	if clusterEntry, found := db.Clusters[v.Info.Cluster]; !found {
		response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("Volume %v unknown cluster %v", v.Info.Id, v.Info.Cluster))
	} else {
		if !sortedstrings.Has(clusterEntry.Info.Volumes, v.Info.Id) {
			response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("Volume %v no link back to volume from cluster %v", v.Info.Id, v.Info.Cluster))
		}
		if len(v.Info.Mount.GlusterFS.Hosts) != len(clusterEntry.Info.Nodes) {
			response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("Volume %v mount hosts list(%v) is not same as list of all nodes of the cluster(%v)", v.Info.Id, v.Info.Mount.GlusterFS.Hosts, clusterEntry.Info.Nodes))
		}
		// TODO: I should probably match the IPs of the mount hosts to that of storage hostnames but not sure if it is worth it.
	}

	// Bricks
	for _, brick := range v.Bricks {
		if brickEntry, found := db.Bricks[brick]; !found {
			response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("Volume %v unknown brick %v", v.Info.Id, brick))
		} else {
			if brickEntry.Info.VolumeId != v.Info.Id {
				response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("Volume %v no link back to volume from brick %v", v.Info.Id, brick))
			}
		}
	}

	// BlockVolumes and sizes
	for _, blockVolume := range v.Info.BlockInfo.BlockVolumes {
		if blockVolumeEntry, found := db.BlockVolumes[blockVolume]; !found {
			response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("Volume %v unknown blockvolume %v", v.Info.Id, blockVolume))
		} else {
			if blockVolumeEntry.Info.BlockHostingVolume != v.Info.Id {
				response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("Volume %v no link back to volume from blockvolume %v", v.Info.Id, blockVolume))
			}
			aggregateBlockVolumesSize += blockVolumeEntry.Info.Size
		}
	}
	if v.Info.Block == true {
		if aggregateBlockVolumesSize != v.Info.Size-v.Info.BlockInfo.FreeSize-v.Info.BlockInfo.ReservedSize {
			response.Inconsistencies = append(response.Inconsistencies,
				fmt.Sprintf("Volume %v blocksize differs aggregateSize %v != volumeSize %v - freeSize %v - reservedSize %v",
					v.Info.Id, aggregateBlockVolumesSize, v.Info.Size, v.Info.BlockInfo.FreeSize, v.Info.BlockInfo.ReservedSize))
		}
	} else if aggregateBlockVolumesSize != 0 {
		response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("Volume %v has blockvolumes but not block flag", v.Info.Id))
	}

	return

}
