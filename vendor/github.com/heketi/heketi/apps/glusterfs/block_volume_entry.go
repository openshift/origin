//
// Copyright (c) 2017 The heketi Authors
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

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/idgen"
	"github.com/heketi/heketi/pkg/sortedstrings"
	"github.com/lpabon/godbc"
)

type BlockVolumeEntry struct {
	Info    api.BlockVolumeInfo
	Pending PendingItem
}

func BlockVolumeList(tx *bolt.Tx) ([]string, error) {
	list := EntryKeys(tx, BOLTDB_BUCKET_BLOCKVOLUME)
	if list == nil {
		return nil, ErrAccessList
	}
	return list, nil
}

func NewVolumeEntryForBlockHosting(clusters []string) (*VolumeEntry, error) {
	var msg api.VolumeCreateRequest
	msg.Clusters = clusters
	msg.Durability.Type = api.DurabilityReplicate
	msg.Size = BlockHostingVolumeSize
	msg.Durability.Replicate.Replica = 3
	msg.Block = true

	vol := NewVolumeEntryFromRequest(&msg)

	if !CreateBlockHostingVolumes {
		return nil, fmt.Errorf("Block Hosting Volume Creation is " +
			"disabled. Create a Block hosting volume and try " +
			"again.")
	}

	if uint64(msg.Size)*GB < vol.Durability.MinVolumeSize() {
		return nil, fmt.Errorf("Requested volume size (%v GB) is "+
			"smaller than the minimum supported volume size (%v)",
			msg.Size, vol.Durability.MinVolumeSize())
	}
	return vol, nil
}

func NewBlockVolumeEntry() *BlockVolumeEntry {
	entry := &BlockVolumeEntry{}

	return entry
}

func NewBlockVolumeEntryFromRequest(req *api.BlockVolumeCreateRequest) *BlockVolumeEntry {
	godbc.Require(req != nil)

	vol := NewBlockVolumeEntry()
	vol.Info.Id = idgen.GenUUID()
	vol.Info.Size = req.Size
	vol.Info.Auth = req.Auth

	if req.Name == "" {
		vol.Info.Name = "blockvol_" + vol.Info.Id
	} else {
		vol.Info.Name = req.Name
	}

	// If Clusters is zero, then it will be assigned during volume creation
	vol.Info.Clusters = req.Clusters
	vol.Info.Hacount = req.Hacount

	return vol
}

func NewBlockVolumeEntryFromId(tx *bolt.Tx, id string) (*BlockVolumeEntry, error) {
	godbc.Require(tx != nil)

	entry := NewBlockVolumeEntry()
	err := EntryLoad(tx, entry, id)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (v *BlockVolumeEntry) BucketName() string {
	return BOLTDB_BUCKET_BLOCKVOLUME
}

func (v *BlockVolumeEntry) Visible() bool {
	return v.Pending.Id == ""
}

func (v *BlockVolumeEntry) Save(tx *bolt.Tx) error {
	godbc.Require(tx != nil)
	godbc.Require(len(v.Info.Id) > 0)

	return EntrySave(tx, v, v.Info.Id)
}

func (v *BlockVolumeEntry) Delete(tx *bolt.Tx) error {
	return EntryDelete(tx, v, v.Info.Id)
}

func (v *BlockVolumeEntry) NewInfoResponse(tx *bolt.Tx) (*api.BlockVolumeInfoResponse, error) {
	godbc.Require(tx != nil)

	info := api.NewBlockVolumeInfoResponse()
	info.Id = v.Info.Id
	info.Cluster = v.Info.Cluster
	info.BlockVolume = v.Info.BlockVolume
	info.Size = v.Info.Size
	info.Name = v.Info.Name
	info.Hacount = v.Info.Hacount
	info.BlockHostingVolume = v.Info.BlockHostingVolume

	return info, nil
}

func (v *BlockVolumeEntry) Marshal() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(*v)

	return buffer.Bytes(), err
}

func (v *BlockVolumeEntry) Unmarshal(buffer []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	err := dec.Decode(v)
	if err != nil {
		return err
	}

	return nil
}

func (v *BlockVolumeEntry) eligibleClustersAndVolumes(db wdb.RODB) (
	possibleClusters []string, volumes []*VolumeEntry, e error) {

	if len(v.Info.Clusters) == 0 {
		err := db.View(func(tx *bolt.Tx) error {
			var err error
			possibleClusters, err = ClusterList(tx)
			return err
		})
		if err != nil {
			e = err
			return
		}
	} else {
		possibleClusters = v.Info.Clusters
	}

	// find clusters that support block volumes
	cr := ClusterReq{Block: true}
	possibleClusters, e = eligibleClusters(db, cr, possibleClusters)
	if e != nil {
		return
	}
	logger.Debug("Using the following clusters: %+v", possibleClusters)

	var possibleVolumes []string
	for _, clusterId := range possibleClusters {
		err := db.View(func(tx *bolt.Tx) error {
			var err error
			c, err := NewClusterEntryFromId(tx, clusterId)
			for _, vol := range c.Info.Volumes {
				volEntry, err := NewVolumeEntryFromId(tx, vol)
				if err != nil {
					return err
				}
				if volEntry.Info.Block && volEntry.Pending.Id == "" {
					possibleVolumes = append(possibleVolumes, vol)
				}
			}
			return err
		})
		if err != nil {
			e = err
			return
		}
	}

	logger.Debug("Using the following possible block hosting volumes: %+v", possibleVolumes)

	for _, vol := range possibleVolumes {
		err := db.View(func(tx *bolt.Tx) error {
			volEntry, err := NewVolumeEntryFromId(tx, vol)
			if err != nil {
				return err
			}
			if ok, err := canHostBlockVolume(tx, v, volEntry); ok {
				volumes = append(volumes, volEntry)
			} else if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			e = err
			return
		}
	}
	return
}

func (v *BlockVolumeEntry) Create(db wdb.DB,
	executor executors.Executor) (e error) {

	return RunOperation(
		NewBlockVolumeCreateOperation(v, db),
		executor)
}

func (v *BlockVolumeEntry) saveNewEntry(db wdb.DB) error {
	return db.Update(func(tx *bolt.Tx) error {

		err := v.Save(tx)
		if err != nil {
			return err
		}

		cluster, err := NewClusterEntryFromId(tx, v.Info.Cluster)
		if err != nil {
			return err
		}

		cluster.BlockVolumeAdd(v.Info.Id)

		err = cluster.Save(tx)
		if err != nil {
			return err
		}

		volume, err := NewVolumeEntryFromId(tx, v.Info.BlockHostingVolume)
		if err != nil {
			return err
		}

		if err := volume.ModifyFreeSize(-v.Info.Size); err != nil {
			return err
		}
		logger.Debug("Reduced free size on volume %v by %v",
			volume.Info.Id, v.Info.Size)

		volume.BlockVolumeAdd(v.Info.Id)
		err = volume.Save(tx)
		if err != nil {
			return err
		}

		return err
	})
}

func (v *BlockVolumeEntry) blockHostingVolumeName(db wdb.RODB) (name string, e error) {
	e = db.View(func(tx *bolt.Tx) error {
		volume, err := NewVolumeEntryFromId(tx, v.Info.BlockHostingVolume)
		if err != nil {
			logger.LogError("Unable to load block hosting volume: %v", err)
			return err
		}
		name = volume.Info.Name
		return nil
	})
	return
}

func (v *BlockVolumeEntry) deleteBlockVolumeExec(db wdb.RODB,
	hvname string,
	executor executors.Executor) error {

	executorhost, err := GetVerifiedManageHostname(db, executor, v.Info.Cluster)
	if err != nil {
		return err
	}

	logger.Debug("Using executor host [%v]", executorhost)
	return v.destroyFromHost(executor, hvname, executorhost)
}

// destroyFromHost removes the block volume using the provided
// executor, block hosting volume name, and host.
func (v *BlockVolumeEntry) destroyFromHost(
	executor executors.Executor, hvname, h string) error {

	err := executor.BlockVolumeDestroy(h, hvname, v.Info.Name)
	if _, ok := err.(*executors.VolumeDoesNotExistErr); ok {
		logger.Warning(
			"Block volume %v (%v) does not exist: assuming already deleted",
			v.Info.Id, v.Info.Name)
	} else if err != nil {
		logger.LogError("Unable to delete volume: %v", err)
		return err
	}
	return nil
}

func (v *BlockVolumeEntry) removeComponents(db wdb.DB, keepSize bool) error {
	return db.Update(func(tx *bolt.Tx) error {
		// Remove volume from cluster
		cluster, err := NewClusterEntryFromId(tx, v.Info.Cluster)
		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}
		cluster.BlockVolumeDelete(v.Info.Id)
		err = cluster.Save(tx)
		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}

		blockHostingVolume, err := NewVolumeEntryFromId(tx, v.Info.BlockHostingVolume)
		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}

		blockHostingVolume.BlockVolumeDelete(v.Info.Id)
		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}
		if !keepSize {
			if err := blockHostingVolume.ModifyFreeSize(v.Info.Size); err != nil {
				return err
			}
		}
		blockHostingVolume.Save(tx)

		if err != nil {
			logger.Err(err)
			// Do not return here.. keep going
		}

		v.Delete(tx)

		return nil
	})
}

func (v *BlockVolumeEntry) Destroy(db wdb.DB, executor executors.Executor) error {
	logger.Info("Destroying volume %v", v.Info.Id)

	return RunOperation(
		NewBlockVolumeDeleteOperation(v, db),
		executor)
}

// canHostBlockVolume returns true if the existing volume entry object
// can host the incoming block volume. It returns false (and nil error) if
// the volume is incompatible. It returns false, and an error if the
// database operation fails.
func canHostBlockVolume(tx *bolt.Tx, bv *BlockVolumeEntry, vol *VolumeEntry) (bool, error) {
	if vol.Info.BlockInfo.Restriction != api.Unrestricted {
		logger.Warning("Block hosting volume %v usage is restricted: %v",
			vol.Info.Id, vol.Info.BlockInfo.Restriction)
		return false, nil
	}
	if vol.Info.BlockInfo.FreeSize < bv.Info.Size {
		logger.Warning("Free size %v is less than the requested block volume size %v",
			vol.Info.BlockInfo.FreeSize, bv.Info.Size)
		return false, nil
	}

	for _, blockvol := range vol.Info.BlockInfo.BlockVolumes {
		existingbv, err := NewBlockVolumeEntryFromId(tx, blockvol)
		if err != nil {
			return false, err
		}
		if bv.Info.Name == existingbv.Info.Name {
			logger.Warning("Name %v already in use in file volume %v",
				bv.Info.Name, vol.Info.Name)
			return false, nil
		}
	}

	return true, nil
}

func (v *BlockVolumeEntry) updateHosts(hosts []string) {
	v.Info.BlockVolume.Hosts = hosts
}

// hosts returns a node-to-host mapping for all nodes suitable
// for running commands related to this block volume
func (v *BlockVolumeEntry) hosts(db wdb.RODB) (nodeHosts, error) {
	var hosts nodeHosts
	err := db.View(func(tx *bolt.Tx) error {
		cluster, err := NewClusterEntryFromId(tx, v.Info.Cluster)
		if err != nil {
			return err
		}
		hosts, err = cluster.hosts(wdb.WrapTx(tx))
		return err
	})
	return hosts, err
}

// hasPendingBlockHostingVolume returns true if the db contains pending
// block hosting volumes.
func hasPendingBlockHostingVolume(tx *bolt.Tx) (bool, error) {
	pmap, err := MapPendingVolumes(tx)
	if err != nil {
		return false, err
	}
	// filter out any volumes that are not marked for block
	for volId, popId := range pmap {
		vol, err := NewVolumeEntryFromId(tx, volId)
		if err != nil {
			return false, err
		}
		if !vol.Info.Block {
			// drop volumes that are not BHVs
			delete(pmap, volId)
		}
		pop, err := NewPendingOperationEntryFromId(tx, popId)
		if err != nil {
			return false, err
		}
		if pop.Status != NewOperation {
			// drop pending operations that are not being worked on
			// e.g. stale pending ops
			delete(pmap, volId)
		}
	}
	return (len(pmap) != 0), nil
}

// consistencyCheck ... verifies that a blockVolumeEntry is consistent with rest of the database.
// It is a method on blockVolumeEntry and needs rest of the database as its input.
func (v *BlockVolumeEntry) consistencyCheck(db Db) (response DbEntryCheckResponse) {

	// No consistency check required for following attributes
	// Id
	// Name
	// Size
	// HaCount
	// Auth

	// PendingId
	if v.Pending.Id != "" {
		response.Pending = true
		if _, found := db.PendingOperations[v.Pending.Id]; !found {
			response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("BlockVolume %v marked pending but no pending op %v", v.Info.Id, v.Pending.Id))
		}
		// TODO: Validate back the pending operations' relationship to the blockVolume
		// This is skipped because some of it is handled in auto cleanup code.
	}

	// Cluster
	if clusterEntry, found := db.Clusters[v.Info.Cluster]; !found {
		response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("BlockVolume %v unknown cluster %v", v.Info.Id, v.Info.Cluster))
	} else {
		if !sortedstrings.Has(clusterEntry.Info.BlockVolumes, v.Info.Id) {
			response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("BlockVolume %v no link back to blockVolume from cluster %v", v.Info.Id, v.Info.Cluster))
		}
		// TODO: Check if BlockVolume Hosts belong to the cluster.
	}

	// Volume
	if volumeEntry, found := db.Volumes[v.Info.BlockHostingVolume]; !found {
		response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("BlockVolume %v unknown volume %v", v.Info.Id, v.Info.BlockHostingVolume))
	} else {
		if !sortedstrings.Has(volumeEntry.Info.BlockInfo.BlockVolumes, v.Info.Id) {
			response.Inconsistencies = append(response.Inconsistencies, fmt.Sprintf("BlockVolume %v no link back to blockVolume from volume %v", v.Info.Id, v.Info.BlockHostingVolume))
		}
		// TODO: Check if BlockVolume Hosts belong to the volume.
	}
	return

}
