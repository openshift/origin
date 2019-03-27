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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/pkg/glusterfs/api"
)

func dbDumpInternal(db *bolt.DB) (Db, error) {
	var dump Db
	clusterEntryList := make(map[string]ClusterEntry, 0)
	volEntryList := make(map[string]VolumeEntry, 0)
	brickEntryList := make(map[string]BrickEntry, 0)
	nodeEntryList := make(map[string]NodeEntry, 0)
	deviceEntryList := make(map[string]DeviceEntry, 0)
	blockvolEntryList := make(map[string]BlockVolumeEntry, 0)
	dbattributeEntryList := make(map[string]DbAttributeEntry, 0)
	pendingOpEntryList := make(map[string]PendingOperationEntry, 0)

	err := db.View(func(tx *bolt.Tx) error {

		logger.Debug("volume bucket")

		// Volume Bucket
		volumes, err := VolumeList(tx)
		if err != nil {
			return err
		}

		for _, volume := range volumes {
			logger.Debug("adding volume entry %v", volume)
			volEntry, err := NewVolumeEntryFromId(tx, volume)
			if err != nil {
				return err
			}
			volEntryList[volEntry.Info.Id] = *volEntry
		}

		// Brick Bucket
		logger.Debug("brick bucket")
		bricks, err := BrickList(tx)
		if err != nil {
			return err
		}

		for _, brick := range bricks {
			logger.Debug("adding brick entry %v", brick)
			brickEntry, err := NewBrickEntryFromId(tx, brick)
			if err != nil {
				return err
			}
			brickEntryList[brickEntry.Info.Id] = *brickEntry
		}

		// Cluster Bucket
		logger.Debug("cluster bucket")
		clusters, err := ClusterList(tx)
		if err != nil {
			return err
		}

		for _, cluster := range clusters {
			logger.Debug("adding cluster entry %v", cluster)
			clusterEntry, err := NewClusterEntryFromId(tx, cluster)
			if err != nil {
				return err
			}
			clusterEntryList[clusterEntry.Info.Id] = *clusterEntry
		}

		// Node Bucket
		logger.Debug("node bucket")
		nodes, err := NodeList(tx)
		if err != nil {
			return err
		}

		for _, node := range nodes {
			logger.Debug("adding node entry %v", node)
			// Some entries are added for easy lookup of existing entries
			// Refer to http://lists.gluster.org/pipermail/heketi-devel/2017-May/000107.html
			// Don't output them to JSON. However, these entries must be created when
			// importing nodes into db from JSON.
			if strings.HasPrefix(node, "MANAGE") || strings.HasPrefix(node, "STORAGE") {
				logger.Debug("ignoring registry key %v", node)
			} else {
				nodeEntry, err := NewNodeEntryFromId(tx, node)
				if err != nil {
					return err
				}
				nodeEntryList[nodeEntry.Info.Id] = *nodeEntry
			}
		}

		// Device Bucket
		logger.Debug("device bucket")
		devices, err := DeviceList(tx)
		if err != nil {
			return err
		}

		for _, device := range devices {
			logger.Debug("adding device entry %v", device)
			// Some entries are added for easy lookup of existing entries
			// Refer to http://lists.gluster.org/pipermail/heketi-devel/2017-May/000107.html
			// Don't output them to JSON. However, these entries must be created when
			// importing devices into db from JSON.
			if strings.HasPrefix(device, "DEVICE") {
				logger.Debug("ignoring registry key %v", device)
			} else {
				deviceEntry, err := NewDeviceEntryFromId(tx, device)
				if err != nil {
					return err
				}
				deviceEntryList[deviceEntry.Info.Id] = *deviceEntry
			}
		}

		if b := tx.Bucket([]byte(BOLTDB_BUCKET_BLOCKVOLUME)); b == nil {
			logger.Warning("unable to find block volume bucket... skipping")
		} else {
			// BlockVolume Bucket
			logger.Debug("blockvolume bucket")
			blockvolumes, err := BlockVolumeList(tx)
			if err != nil {
				return err
			}

			for _, blockvolume := range blockvolumes {
				logger.Debug("adding blockvolume entry %v", blockvolume)
				blockvolEntry, err := NewBlockVolumeEntryFromId(tx, blockvolume)
				if err != nil {
					return err
				}
				blockvolEntryList[blockvolEntry.Info.Id] = *blockvolEntry
			}
		}

		has_pendingops := false

		if b := tx.Bucket([]byte(BOLTDB_BUCKET_DBATTRIBUTE)); b == nil {
			logger.Warning("unable to find dbattribute bucket... skipping")
		} else {
			// DbAttributes Bucket
			dbattributes, err := DbAttributeList(tx)
			if err != nil {
				return err
			}

			for _, dbattribute := range dbattributes {
				logger.Debug("adding dbattribute entry %v", dbattribute)
				dbattributeEntry, err := NewDbAttributeEntryFromKey(tx, dbattribute)
				if err != nil {
					return err
				}
				dbattributeEntryList[dbattributeEntry.Key] = *dbattributeEntry
				has_pendingops = (has_pendingops ||
					(dbattributeEntry.Key == DB_HAS_PENDING_OPS_BUCKET &&
						dbattributeEntry.Value == "yes"))
			}
		}

		if has_pendingops {
			pendingops, err := PendingOperationList(tx)
			if err != nil {
				return err
			}

			for _, opid := range pendingops {
				entry, err := NewPendingOperationEntryFromId(tx, opid)
				if err != nil {
					return err
				}
				pendingOpEntryList[opid] = *entry
			}
		}

		return nil
	})
	if err != nil {
		return Db{}, fmt.Errorf("Could not construct dump from DB: %v", err.Error())
	}

	dump.Clusters = clusterEntryList
	dump.Volumes = volEntryList
	dump.Bricks = brickEntryList
	dump.Nodes = nodeEntryList
	dump.Devices = deviceEntryList
	dump.BlockVolumes = blockvolEntryList
	dump.DbAttributes = dbattributeEntryList
	dump.PendingOperations = pendingOpEntryList

	return dump, nil
}

// DbDump ... Creates a JSON output representing the state of DB
// This is the variant to be called offline, i.e. when the server is not
// running.
func DbDump(jsonfile string, dbfile string) error {

	fp, err := os.OpenFile(jsonfile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("Could not create json file: %v", err.Error())
	}
	defer fp.Close()

	db, err := OpenDB(dbfile, false)
	if err != nil {
		return fmt.Errorf("Unable to open database: %v", err)
	}

	dump, err := dbDumpInternal(db)
	if err != nil {
		return fmt.Errorf("Could not construct dump from DB: %v", err.Error())
	}
	enc := json.NewEncoder(fp)
	enc.SetIndent("", "    ")

	if err := enc.Encode(dump); err != nil {
		return fmt.Errorf("Could not encode dump as JSON: %v", err.Error())
	}

	return nil
}

// DbCreate ... Creates a bolt db file based on JSON input
func DbCreate(jsonfile string, dbfile string) error {

	var dump Db

	fp, err := os.Open(jsonfile)
	if err != nil {
		return fmt.Errorf("Could not open input file: %v", err.Error())
	}
	defer fp.Close()

	dbParser := json.NewDecoder(fp)
	if err = dbParser.Decode(&dump); err != nil {
		return fmt.Errorf("Could not decode input file as JSON: %v", err.Error())
	}

	// We don't want to overwrite existing db file
	_, err = os.Stat(dbfile)
	if err == nil {
		return fmt.Errorf("%v file already exists", dbfile)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("unable to stat path given for dbfile: %v", dbfile)
	}

	dbhandle, err := OpenDB(dbfile, false)
	if err != nil {
		return fmt.Errorf("Could not open db file: %v", err.Error())
	}

	err = dbhandle.Update(func(tx *bolt.Tx) error {
		return initializeBuckets(tx)
	})
	if err != nil {
		logger.Err(err)
		return nil
	}

	err = dbhandle.Update(func(tx *bolt.Tx) error {
		for _, cluster := range dump.Clusters {
			logger.Debug("adding cluster entry %v", cluster.Info.Id)
			err := cluster.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save cluster bucket: %v", err.Error())
			}
		}
		for _, volume := range dump.Volumes {
			logger.Debug("adding volume entry %v", volume.Info.Id)
			// When serializing to JSON we skipped volume.Durability
			// Hence, while creating volume entry, we populate it
			durability := volume.Info.Durability.Type
			switch {

			case durability == api.DurabilityReplicate:
				volume.Durability = NewVolumeReplicaDurability(&volume.Info.Durability.Replicate)

			case durability == api.DurabilityEC:
				volume.Durability = NewVolumeDisperseDurability(&volume.Info.Durability.Disperse)

			case durability == api.DurabilityDistributeOnly || durability == "":
				volume.Durability = NewNoneDurability()

			default:
				return fmt.Errorf("Not a known volume durability type: %v", durability)
			}

			// Set the default values accordingly
			volume.Durability.SetDurability()
			err := volume.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save volume bucket: %v", err.Error())
			}
		}
		for _, brick := range dump.Bricks {
			logger.Debug("adding brick entry %v", brick.Info.Id)
			err := brick.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save brick bucket: %v", err.Error())
			}
		}
		for _, node := range dump.Nodes {
			logger.Debug("adding node entry %v", node.Info.Id)
			err := node.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save node bucket: %v", err.Error())
			}
			logger.Debug("registering node entry %v", node.Info.Id)
			err = node.Register(tx)
			if err != nil {
				return fmt.Errorf("Could not register node: %v", err.Error())
			}
		}
		for _, device := range dump.Devices {
			logger.Debug("adding device entry %v", device.Info.Id)
			err := device.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save device bucket: %v", err.Error())
			}
			logger.Debug("registering device entry %v", device.Info.Id)
			err = device.Register(tx)
			if err != nil {
				return fmt.Errorf("Could not register device: %v", err.Error())
			}
		}
		for _, blockvolume := range dump.BlockVolumes {
			logger.Debug("adding blockvolume entry %v", blockvolume.Info.Id)
			err := blockvolume.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save blockvolume bucket: %v", err.Error())
			}
		}
		for _, dbattribute := range dump.DbAttributes {
			logger.Debug("adding dbattribute entry %v", dbattribute.Key)
			err := dbattribute.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save dbattribute bucket: %v", err.Error())
			}
		}
		for _, pendingop := range dump.PendingOperations {
			logger.Debug("adding pending operation entry %v", pendingop.Id)
			err := pendingop.Save(tx)
			if err != nil {
				return fmt.Errorf("Could not save pending operation bucket: %v", err.Error())
			}
		}
		// always record a new generation id on db import as the db contents
		// were no longer fully under heketi's control
		logger.Debug("recording new DB generation ID")
		if err := recordNewDBGenerationID(tx); err != nil {
			return fmt.Errorf("Could not record DB generation ID: %v", err.Error())
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func DeleteBricksWithEmptyPath(db *bolt.DB, all bool, clusterIDs []string, nodeIDs []string, deviceIDs []string) error {

	for _, id := range clusterIDs {
		if err := api.ValidateUUID(id); err != nil {
			return err
		}
	}
	for _, id := range nodeIDs {
		if err := api.ValidateUUID(id); err != nil {
			return err
		}
	}
	for _, id := range deviceIDs {
		if err := api.ValidateUUID(id); err != nil {
			return err
		}
	}

	err := db.Update(func(tx *bolt.Tx) error {
		if true == all {
			logger.Debug("deleting all bricks with empty path")
			clusters, err := ClusterList(tx)
			if err != nil {
				return err
			}
			for _, cluster := range clusters {
				clusterEntry, err := NewClusterEntryFromId(tx, cluster)
				if err != nil {
					return err
				}
				err = clusterEntry.DeleteBricksWithEmptyPath(tx)
				if err != nil {
					return err
				}
			}
			// no need to look at other IDs as we cleaned all bricks
			return nil
		}
		for _, cluster := range clusterIDs {
			clusterEntry, err := NewClusterEntryFromId(tx, cluster)
			if err != nil {
				return err
			}
			logger.Debug("deleting bricks with empty path in cluster %v from given list of clusters", clusterEntry.Info.Id)
			err = clusterEntry.DeleteBricksWithEmptyPath(tx)
			if err != nil {
				return err
			}
		}
		for _, node := range nodeIDs {
			nodeEntry, err := NewNodeEntryFromId(tx, node)
			if err != nil {
				return err
			}
			logger.Debug("deleting bricks with empty path in node %v from given list of nodes", nodeEntry.Info.Id)
			err = nodeEntry.DeleteBricksWithEmptyPath(tx)
			if err != nil {
				return err
			}
		}
		for _, device := range deviceIDs {
			deviceEntry, err := NewDeviceEntryFromId(tx, device)
			if err != nil {
				return err
			}
			logger.Debug("deleting bricks with empty path in device %v from given list of devices", deviceEntry.Info.Id)
			err = deviceEntry.DeleteBricksWithEmptyPath(tx)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func deleteChangeOwnerEntry(tx *bolt.Tx, action PendingOperationAction, dryRun bool) error {
	switch action.Change {

	case OpAddBrick:
		logger.Debug("Found a pending add brick change with id: %v", action.Id)
		logger.Info("Deleting brick with id: %v", action.Id)
		brickEntry, err := NewBrickEntryFromId(tx, action.Id)
		if err != nil {
			return err
		}
		logger.Info("brickentry %+v", brickEntry)
		logger.Info("USER ACTION REQUIRED: cleanup brick or create brick(in case of expand op) with path:%v on node:%v", brickEntry.Info.Path, brickEntry.Info.NodeId)
		if !dryRun {
			err = brickEntry.Delete(tx)
			if err != nil {
				return err
			}
		}
	case OpDeleteBrick:
		logger.Debug("Found a pending delete brick change with id: %v", action.Id)
		logger.Info("Deleting brick with id: %v", action.Id)
		brickEntry, err := NewBrickEntryFromId(tx, action.Id)
		if err != nil {
			return err
		}
		logger.Info("brickEntry %+v", brickEntry)
		logger.Info("USER ACTION REQUIRED: cleanup brick with path:%v on node:%v", brickEntry.Info.Path, brickEntry.Info.NodeId)
		if !dryRun {
			err = brickEntry.Delete(tx)
			if err != nil {
				return err
			}
		}
	case OpAddVolume:
		logger.Debug("Found a pending add volume change with id: %v", action.Id)
		logger.Info("Deleting volume with id: %v", action.Id)
		volumeEntry, err := NewVolumeEntryFromId(tx, action.Id)
		if err != nil {
			return err
		}
		logger.Info("volumeEntry %+v", volumeEntry)
		logger.Info("USER ACTION REQUIRED: cleanup volume:%v on cluster:%v", volumeEntry.Info.Name, volumeEntry.Info.Cluster)
		if !dryRun {
			err = volumeEntry.Delete(tx)
			if err != nil {
				return err
			}
		}
	case OpDeleteVolume:
		logger.Debug("Found a pending delete volume change with id: %v", action.Id)
		logger.Info("Deleting volume with id: %v", action.Id)
		volumeEntry, err := NewVolumeEntryFromId(tx, action.Id)
		if err != nil {
			return err
		}
		logger.Info("volumeEntry %+v", volumeEntry)
		logger.Info("USER ACTION REQUIRED: cleanup volume:%v on cluster:%v", volumeEntry.Info.Name, volumeEntry.Info.Cluster)
		if !dryRun {
			err = volumeEntry.Delete(tx)
			if err != nil {
				return err
			}
		}
	case OpExpandVolume:
		logger.Debug("Found a pending expand volume change with id: %v", action.Id)
		volumeEntry, err := NewVolumeEntryFromId(tx, action.Id)
		if err != nil {
			return err
		}
		logger.Info("USER ACTION REQUIRED: complete volume expand operation on %v using bricks listed above", volumeEntry.Info.Name)
		if !dryRun {
			logger.Info("volumeEntry %+v", volumeEntry)
			err = volumeEntry.Delete(tx)
			if err != nil {
				return err
			}
		}
	case OpAddBlockVolume:
		logger.Debug("Found a pending add blockvolume change with id: %v", action.Id)
		logger.Info("Deleting blockvolume with id: %v", action.Id)
		blockVolumeEntry, err := NewBlockVolumeEntryFromId(tx, action.Id)
		if err != nil {
			return err
		}
		logger.Info("blockVolumeEntry %+v", blockVolumeEntry)
		logger.Info("USER ACTION REQUIRED: cleanup blockvolume:%v on hostingvolume:%v", blockVolumeEntry.Info.Name, blockVolumeEntry.Info.BlockHostingVolume)
		if !dryRun {
			err = blockVolumeEntry.Delete(tx)
			if err != nil {
				return err
			}
		}
	case OpDeleteBlockVolume:
		logger.Debug("Found a pending delete blockvolume change with id: %v", action.Id)
		logger.Info("Deleting blockvolume with id: %v", action.Id)
		blockVolumeEntry, err := NewBlockVolumeEntryFromId(tx, action.Id)
		if err != nil {
			return err
		}
		logger.Info("blockVolumeEntry %+v", blockVolumeEntry)
		logger.Info("USER ACTION REQUIRED: cleanup blockvolume:%v on hostingvolume:%v", blockVolumeEntry.Info.Name, blockVolumeEntry.Info.BlockHostingVolume)
		if !dryRun {
			err = blockVolumeEntry.Delete(tx)
			if err != nil {
				return err
			}
		}
	case OpRemoveDevice:
		logger.Debug("Found a pending remove device change with id: %v", action.Id)
		logger.Info("Deleting device with id: %v", action.Id)
		deviceEntry, err := NewDeviceEntryFromId(tx, action.Id)
		if err != nil {
			return err
		}
		logger.Info("deviceEntry %+v", deviceEntry)
		logger.Info("USER ACTION REQUIRED: cleanup device:%v on node:%v", deviceEntry.Info.Name, deviceEntry.NodeId)
		if !dryRun {
			err = deviceEntry.Delete(tx)
			if err != nil {
				return err
			}
		}
	default:
		logger.Debug("Not a known change type: %v", action.Change)
	}
	return nil
}

func deleteChangeEntriesInOp(tx *bolt.Tx, pendingOpEntry *PendingOperationEntry, dryRun bool) error {
	switch pendingOpEntry.Type {

	case OperationCreateVolume:
		logger.Info("Found a pending volume create operation with id: %v and timestamp: %v", pendingOpEntry.Id, pendingOpEntry.Timestamp)

	case OperationDeleteVolume:
		logger.Info("Found a pending volume delete operation with id: %v and timestamp: %v", pendingOpEntry.Id, pendingOpEntry.Timestamp)

	case OperationExpandVolume:
		logger.Info("Found a pending volume expand operation with id: %v and timestamp: %v", pendingOpEntry.Id, pendingOpEntry.Timestamp)

	case OperationCreateBlockVolume:
		logger.Info("Found a pending blockvolume create operation with id: %v and timestamp: %v", pendingOpEntry.Id, pendingOpEntry.Timestamp)

	case OperationDeleteBlockVolume:
		logger.Info("Found a pending blockvolume delete operation with id: %v and timestamp: %v", pendingOpEntry.Id, pendingOpEntry.Timestamp)

	case OperationRemoveDevice:
		logger.Info("Found a pending device remove operation with id: %v and timestamp: %v", pendingOpEntry.Id, pendingOpEntry.Timestamp)

	default:
		logger.Debug("Not a known pending Operation type: %v", pendingOpEntry.Type)
	}
	for _, action := range pendingOpEntry.Actions {
		err := deleteChangeOwnerEntry(tx, action, dryRun)
		if err != nil {
			return err
		}
	}

	return nil
}

func DeletePendingEntries(db *bolt.DB, dryRun bool, force bool) error {

	err := db.Update(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(BOLTDB_BUCKET_PENDING_OPS)); b == nil {
			return logger.LogError("unable to find pending ops bucket... exiting")
		}
		var pendingOpsFoundCount int
		var pendingOpsDeletedCount int

		logger.Info("traversing through pending ops bucket to delete pending entries")
		logger.Debug("pendingops bucket")
		pendingops, err := PendingOperationList(tx)
		if err != nil {
			return err
		}
		for _, opid := range pendingops {
			pendingOpEntry, err := NewPendingOperationEntryFromId(tx, opid)
			if err != nil {
				return err
			}
			pendingOpsFoundCount++
			logger.Info("\nPending Operation %v Start", pendingOpsFoundCount)
			// Special case for expand volume operation
			// Always dry-run
			if pendingOpEntry.Type == OperationExpandVolume {
				logger.Info("USER ACTION REQUIRED: Found an expand volume operation, it won't be cleaned")
				logger.Info("USER ACTION REQUIRED: Note the add brick action printed below and complete the action on Gluster if not completed already")
				err = deleteChangeEntriesInOp(tx, pendingOpEntry, true)
				if err != nil {
					return err
				}
			} else {
				err = deleteChangeEntriesInOp(tx, pendingOpEntry, dryRun)
				if err != nil {
					return err
				}
			}
			// Again, skip deleting main op if it is expand volume
			if !dryRun && pendingOpEntry.Type != OperationExpandVolume {
				err = pendingOpEntry.Delete(tx)
				if err != nil {
					return err
				}
				pendingOpsDeletedCount++
			}
			logger.Info("\nPending Operation %v End", pendingOpsFoundCount)
		}
		logger.Info("Found %v pending entries and deleted %v", pendingOpsFoundCount, pendingOpsDeletedCount)

		// Here onwards, we should not find any entry with pending id set if dry-run is not used
		// If we do find any entries with pending ID, then it is case of db corruption
		// Warn users if force flag is not set
		// Clean the entries if force flag is set
		if !dryRun {
			logger.Info("traversing through other buckets to ensure no pending entries are left")
			logger.Debug("volume bucket")
			volumes, err := VolumeList(tx)
			if err != nil {
				return err
			}

			for _, volume := range volumes {
				volEntry, err := NewVolumeEntryFromId(tx, volume)
				if err != nil {
					return err
				}
				if volEntry.Pending.Id != "" {
					logger.Info("found untracked pending volume entry %v, use force flag to delete it", volume)
					if force {
						logger.Info("deleting untracked pending volume entry %v", volume)
						err = volEntry.Delete(tx)
						if err != nil {
							return err
						}
					}
				}
			}

			logger.Debug("brick bucket")
			bricks, err := BrickList(tx)
			if err != nil {
				return err
			}

			for _, brick := range bricks {
				brickEntry, err := NewBrickEntryFromId(tx, brick)
				if err != nil {
					return err
				}
				if brickEntry.Pending.Id != "" {
					logger.Info("found untracked pending brick entry %v, use force flag to delete it", brick)
					if force {
						logger.Info("deleting untracked pending brick entry %v", brick)
						err = brickEntry.Delete(tx)
						if err != nil {
							return err
						}
					}
				}
			}

			// BlockVolume Bucket
			logger.Debug("blockvolume bucket")
			blockvolumes, err := BlockVolumeList(tx)
			if err != nil {
				return err
			}

			for _, blockvolume := range blockvolumes {
				blockvolEntry, err := NewBlockVolumeEntryFromId(tx, blockvolume)
				if err != nil {
					return err
				}
				if blockvolEntry.Pending.Id != "" {
					logger.Info("found untracked pending blockvolume entry %v, use force flag to delete it", blockvolume)
					if force {
						logger.Info("deleting untracked pending blockvolume entry %v", blockvolume)
						err = blockvolEntry.Delete(tx)
						if err != nil {
							return err
						}
					}
				}
			}

		}

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
