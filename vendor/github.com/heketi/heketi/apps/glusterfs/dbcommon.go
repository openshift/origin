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
	"time"

	"github.com/boltdb/bolt"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/idgen"
)

const (
	DB_GENERATION_ID      = "DB_GENERATION_ID"
	DB_DEVICE_HAS_ID_META = "DB_DEVICE_HAS_ID_META"
)

type Db struct {
	Clusters          map[string]ClusterEntry          `json:"clusterentries"`
	Volumes           map[string]VolumeEntry           `json:"volumeentries"`
	Bricks            map[string]BrickEntry            `json:"brickentries"`
	Nodes             map[string]NodeEntry             `json:"nodeentries"`
	Devices           map[string]DeviceEntry           `json:"deviceentries"`
	BlockVolumes      map[string]BlockVolumeEntry      `json:"blockvolumeentries"`
	DbAttributes      map[string]DbAttributeEntry      `json:"dbattributeentries"`
	PendingOperations map[string]PendingOperationEntry `json:"pendingoperations"`
}

//DbEntryCheckResponse ... is summary of check on a db entry.
type DbEntryCheckResponse struct {
	Pending         bool     `json:"pending"`
	Inconsistencies []string `json:"inconsistencies"`
}

//DbBucketCheckResponse ... is summary of check on a db bucket.
type DbBucketCheckResponse struct {
	Total           int      `json:"total"`
	Pending         int      `json:"pending"`
	Ok              int      `json:"ok"`
	NotOk           int      `json:"notok"`
	Inconsistencies []string `json:"inconsistencies"`
}

//DbCheckResponse ... is the output of db check. It lists a summary of db state
//and inconsistencies related to each bucket, if any.
type DbCheckResponse struct {
	Clusters             DbBucketCheckResponse `json:"clusters"`
	Volumes              DbBucketCheckResponse `json:"volumes"`
	Bricks               DbBucketCheckResponse `json:"bricks"`
	Nodes                DbBucketCheckResponse `json:"nodes"`
	Devices              DbBucketCheckResponse `json:"devices"`
	BlockVolumes         DbBucketCheckResponse `json:"blockvolumes"`
	DbAttributes         DbBucketCheckResponse `json:"dbattributes"`
	PendingOperations    DbBucketCheckResponse `json:"pendingoperations"`
	TotalInconsistencies int                   `json:"totalinconsistencies"`
}

func initializeBuckets(tx *bolt.Tx) error {
	// Create Cluster Bucket
	_, err := tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_CLUSTER))
	if err != nil {
		logger.LogError("Unable to create cluster bucket in DB")
		return err
	}

	// Create Node Bucket
	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_NODE))
	if err != nil {
		logger.LogError("Unable to create node bucket in DB")
		return err
	}

	// Create Volume Bucket
	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_VOLUME))
	if err != nil {
		logger.LogError("Unable to create volume bucket in DB")
		return err
	}

	// Create Device Bucket
	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_DEVICE))
	if err != nil {
		logger.LogError("Unable to create device bucket in DB")
		return err
	}

	// Create Brick Bucket
	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_BRICK))
	if err != nil {
		logger.LogError("Unable to create brick bucket in DB")
		return err
	}

	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_BLOCKVOLUME))
	if err != nil {
		logger.LogError("Unable to create blockvolume bucket in DB")
		return err
	}

	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_DBATTRIBUTE))
	if err != nil {
		logger.LogError("Unable to create dbattribute bucket in DB")
		return err
	}

	_, err = tx.CreateBucketIfNotExists([]byte(BOLTDB_BUCKET_PENDING_OPS))
	if err != nil {
		logger.LogError("Unable to create pending ops bucket in DB")
		return err
	}

	return nil
}

// UpgradeDB runs all upgrade routines in order to to update the DB
// to the latest "schemas" and data.
func UpgradeDB(tx *bolt.Tx) error {

	err := ClusterEntryUpgrade(tx)
	if err != nil {
		logger.LogError("Failed to upgrade db for cluster entries")
		return err
	}

	err = NodeEntryUpgrade(tx)
	if err != nil {
		logger.LogError("Failed to upgrade db for node entries")
		return err
	}

	err = VolumeEntryUpgrade(tx)
	if err != nil {
		logger.LogError("Failed to upgrade db for volume entries")
		return err
	}

	err = DeviceEntryUpgrade(tx)
	if err != nil {
		logger.LogError("Failed to upgrade db for device entries")
		return err
	}

	err = BrickEntryUpgrade(tx)
	if err != nil {
		logger.LogError("Failed to upgrade db for brick entries: %v", err)
		return err
	}

	err = PendingOperationUpgrade(tx)
	if err != nil {
		logger.LogError("Failed to upgrade db for pending operations: %v", err)
		return err
	}

	err = upgradeDBGenerationID(tx)
	if err != nil {
		logger.LogError("Failed to record DB Generation ID: %v", err)
		return err
	}

	err = fixIncorrectBlockHostingFreeSize(tx)
	if err != nil {
		logger.LogError(
			"Failed to fix incorrect sizes for block hosting volumes %v", err)
		return err
	}

	err = fixBlockHostingReservedSize(tx)
	if err != nil {
		logger.LogError(
			"Failed to fix reserved sizes for block hosting volumes %v", err)
		return err
	}

	err = upgradeDeviceEntryMetaSupport(tx)
	if err != nil {
		logger.LogError(
			"Failed to set device entry metadata support: %v", err)
		return err
	}

	return nil
}

func upgradeDBGenerationID(tx *bolt.Tx) error {
	_, err := NewDbAttributeEntryFromKey(tx, DB_GENERATION_ID)
	switch err {
	case ErrNotFound:
		return recordNewDBGenerationID(tx)
	case nil:
		return nil
	default:
		return err
	}
}

func recordNewDBGenerationID(tx *bolt.Tx) error {
	entry := NewDbAttributeEntry()
	entry.Key = DB_GENERATION_ID
	entry.Value = idgen.GenUUID()
	return entry.Save(tx)
}

// OpenDB is a wrapper over bolt.Open. It takes a bool to decide whether it should be a read-only open.
// Other bolt DB config options remain local to this function.
func OpenDB(dbfilename string, ReadOnly bool) (dbhandle *bolt.DB, err error) {

	if ReadOnly {
		dbhandle, err = bolt.Open(dbfilename, 0666, &bolt.Options{ReadOnly: true})
		if err != nil {
			logger.LogError("Unable to open database in read only mode: %v", err)
		}
		return dbhandle, err
	}

	dbhandle, err = bolt.Open(dbfilename, 0600, &bolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		logger.LogError("Unable to open database: %v", err)
	}
	return dbhandle, err

}

// fixIncorrectBlockHostingFreeSize attempts to fix invalid block hosting volume
// free size amounts by checking them against the block volumes.
func fixIncorrectBlockHostingFreeSize(tx *bolt.Tx) error {
	vols, err := VolumeList(tx)
	if err != nil {
		return err
	}
	for _, vid := range vols {
		vol, err := NewVolumeEntryFromId(tx, vid)
		if err != nil {
			return err
		}
		if !vol.Info.Block || vol.Pending.Id != "" {
			continue // ignore non-BHVs and pending BHVs
		}
		bvsum, err := vol.TotalSizeBlockVolumes(tx)
		if err != nil {
			return err
		}
		if !vol.blockHostingSizeIsCorrect(bvsum) {
			newSize := vol.Info.Size - (vol.Info.BlockInfo.ReservedSize + bvsum)
			if newSize < 0 {
				logger.Warning(
					"new size [%v] is invalid, not changing volume [%v]",
					newSize, vol.Info.Id)
				continue
			}
			if newSize > vol.Info.Size {
				logger.Warning(
					"new size [%v] is too large [>%v], not changing volume [%v]",
					newSize, vol.Info.Size, vol.Info.Id)
				continue
			}
			logger.Info("changing free size of volume [%v] to [%v]",
				vol.Info.Id, newSize)
			vol.Info.BlockInfo.FreeSize = newSize
			// if the size was already messed up, lock the volume
			// and admins can unlock it later
			vol.Info.BlockInfo.Restriction = api.LockedByUpdate
			err = vol.Save(tx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// fixBlockHostingReservedSize  attempts to set block hosting volume
// free size amounts by checking them against the block volumes.
func fixBlockHostingReservedSize(tx *bolt.Tx) error {
	vols, err := VolumeList(tx)
	if err != nil {
		return err
	}
	deductReserved := func(v *VolumeEntry, rSize int) error {
		// we do this osize thing because we may want to save v
		// on error and we don't want any incorrectly modified
		// sizes saved
		osize := v.Info.BlockInfo.FreeSize
		if err := v.ModifyFreeSize(-rSize); err != nil {
			v.Info.BlockInfo.FreeSize = osize
			return err
		}
		osize = v.Info.BlockInfo.ReservedSize
		if err := v.ModifyReservedSize(rSize); err != nil {
			v.Info.BlockInfo.ReservedSize = osize
			return err
		}
		return nil
	}
	for _, vid := range vols {
		vol, err := NewVolumeEntryFromId(tx, vid)
		if err != nil {
			return err
		}
		if !vol.Info.Block {
			continue // ignore non-block hosting volumes
		}
		bvsum, err := vol.TotalSizeBlockVolumes(tx)
		if err != nil {
			return err
		}
		if vol.Info.BlockInfo.ReservedSize > 0 {
			// reserved size is already set, nothing to do
			continue
		}
		if !vol.blockHostingSizeIsCorrect(bvsum) {
			logger.Debug(
				"Volume [%v] missing reserved, but sizes are invalid",
				vol.Info.Id)
			continue
		}
		if vol.Info.BlockInfo.Restriction == api.LockedByUpdate {
			logger.Debug(
				"Volume [%v] is missing reserved, but is already locked-by-update",
				vol.Info.Id)
			continue
		}
		rSize := vol.Info.Size - ReduceRawSize(vol.Info.Size)
		if vol.Info.BlockInfo.FreeSize >= rSize {
			if err := deductReserved(vol, rSize); err != nil {
				logger.Warning(
					"Volume [%v] unable to correct reserved size value: %v",
					vol.Info.Id, err)
				vol.Info.BlockInfo.Restriction = api.LockedByUpdate
			}
			err = vol.Save(tx)
			if err != nil {
				return err
			}
		} else {
			logger.Warning(
				"Volume [%v] not enough free space for reservation,"+
					" wanted %v have %v. Locking volume.",
				vol.Info.Id, rSize, vol.Info.BlockInfo.FreeSize)
			vol.Info.BlockInfo.Restriction = api.LockedByUpdate
			err = vol.Save(tx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// upgradeDeviceEntryMetaSupport sets the dbattribute flag needed to
// indicate that this db support extra identifying metadata on device
// entries. It primarily exists as a guard against old versions of
// heketi trying to access new dbs and ignoring the new metadata.
func upgradeDeviceEntryMetaSupport(tx *bolt.Tx) error {
	_, err := NewDbAttributeEntryFromKey(tx, DB_DEVICE_HAS_ID_META)
	switch err {
	case ErrNotFound:
		entry := NewDbAttributeEntry()
		entry.Key = DB_DEVICE_HAS_ID_META
		entry.Value = "yes"
		return entry.Save(tx)
	case nil:
		return nil
	default:
		return err
	}
}
