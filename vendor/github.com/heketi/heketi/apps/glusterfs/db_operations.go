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
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"
)

func dbDumpInternal(db wdb.DB) (Db, error) {
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

	var fp *os.File
	if jsonfile == "-" {
		fp = os.Stdout
	} else {
		var err error
		fp, err = os.OpenFile(jsonfile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("Could not create json file: %v", err.Error())
		}
		defer fp.Close()
	}

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
	defer dbhandle.Close()

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

func DeleteBricksWithEmptyPath(db wdb.DB, all bool, clusterIDs []string, nodeIDs []string, deviceIDs []string) error {

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

// DbCheck ... is the offline version
func DbCheck(dbfile string) error {

	db, err := OpenDB(dbfile, false)
	if err != nil {
		return fmt.Errorf("Unable to open database: %v", err)
	}

	checkresponse, err := dbCheckConsistency(db)
	if err != nil {
		return fmt.Errorf("Unable to check the database: %v", err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "    ")

	if err := encoder.Encode(checkresponse); err != nil {
		return fmt.Errorf("Unable to encode the response into json: %v", err)
	}

	return nil
}

// dbCheckConsistency ... checks the current db state to determine if contents
// of all the buckets represent a consistent view.
func dbCheckConsistency(db *bolt.DB) (response DbCheckResponse, err error) {

	dump, err := dbDumpInternal(db)
	if err != nil {
		return response, fmt.Errorf("Could not construct dump from DB: %v", err.Error())
	}

	response.Volumes = dbCheckVolumes(dump)
	response.TotalInconsistencies += len(response.Volumes.Inconsistencies)
	response.Clusters = dbCheckClusters(dump)
	response.TotalInconsistencies += len(response.Clusters.Inconsistencies)
	response.Nodes = dbCheckNodes(dump)
	response.TotalInconsistencies += len(response.Nodes.Inconsistencies)
	response.Devices = dbCheckDevices(dump)
	response.TotalInconsistencies += len(response.Devices.Inconsistencies)
	response.BlockVolumes = dbCheckBlockVolumes(dump)
	response.TotalInconsistencies += len(response.BlockVolumes.Inconsistencies)
	response.Bricks = dbCheckBricks(dump)
	response.TotalInconsistencies += len(response.Bricks.Inconsistencies)
	response.PendingOperations = dbCheckPendingOps(dump)
	response.TotalInconsistencies += len(response.PendingOperations.Inconsistencies)

	return
}

func dbCheckVolumes(dump Db) (volumesCheckResponse DbBucketCheckResponse) {

	for _, volumeEntry := range dump.Volumes {

		volumesCheckResponse.Total++

		volumeCheckResponse := volumeEntry.consistencyCheck(dump)
		if volumeCheckResponse.Pending {
			volumesCheckResponse.Pending++
		}

		if len(volumeCheckResponse.Inconsistencies) > 0 {
			volumesCheckResponse.Inconsistencies = append(volumesCheckResponse.Inconsistencies, volumeCheckResponse.Inconsistencies...)
			volumesCheckResponse.NotOk++
		} else {
			volumesCheckResponse.Ok++
		}
	}

	return
}

func dbCheckClusters(dump Db) (clustersCheckResponse DbBucketCheckResponse) {
	for _, clusterEntry := range dump.Clusters {

		clustersCheckResponse.Total++
		// Cluster Entries don't have pending operations

		clusterCheckResponse := clusterEntry.consistencyCheck(dump)

		if len(clusterCheckResponse.Inconsistencies) > 0 {
			clustersCheckResponse.Inconsistencies = append(clustersCheckResponse.Inconsistencies, clusterCheckResponse.Inconsistencies...)
			clustersCheckResponse.NotOk++
		} else {
			clustersCheckResponse.Ok++
		}
	}

	return
}

func dbCheckNodes(dump Db) (nodesCheckResponse DbBucketCheckResponse) {
	for _, nodeEntry := range dump.Nodes {

		nodesCheckResponse.Total++
		// Node Entries don't have pending operations

		nodeCheckResponse := nodeEntry.consistencyCheck(dump)

		if len(nodeCheckResponse.Inconsistencies) > 0 {
			nodesCheckResponse.Inconsistencies = append(nodesCheckResponse.Inconsistencies, nodeCheckResponse.Inconsistencies...)
			nodesCheckResponse.NotOk++
		} else {
			nodesCheckResponse.Ok++
		}
	}

	return
}

func dbCheckDevices(dump Db) (devicesCheckResponse DbBucketCheckResponse) {
	for _, deviceEntry := range dump.Devices {

		devicesCheckResponse.Total++
		// Device Entries don't have pending operations

		deviceCheckResponse := deviceEntry.consistencyCheck(dump)

		if len(deviceCheckResponse.Inconsistencies) > 0 {
			devicesCheckResponse.Inconsistencies = append(devicesCheckResponse.Inconsistencies, deviceCheckResponse.Inconsistencies...)
			devicesCheckResponse.NotOk++
		} else {
			devicesCheckResponse.Ok++
		}
	}

	return
}

func dbCheckBlockVolumes(dump Db) (blockVolumesCheckResponse DbBucketCheckResponse) {
	for _, blockVolumeEntry := range dump.BlockVolumes {

		blockVolumesCheckResponse.Total++

		blockVolumeCheckResponse := blockVolumeEntry.consistencyCheck(dump)
		if blockVolumeCheckResponse.Pending {
			blockVolumesCheckResponse.Pending++
		}
		if len(blockVolumeCheckResponse.Inconsistencies) > 0 {
			blockVolumesCheckResponse.Inconsistencies = append(blockVolumesCheckResponse.Inconsistencies, blockVolumeCheckResponse.Inconsistencies...)
			blockVolumesCheckResponse.NotOk++
		} else {
			blockVolumesCheckResponse.Ok++
		}
	}

	return
}

func dbCheckBricks(dump Db) (bricksCheckResponse DbBucketCheckResponse) {
	for _, brickEntry := range dump.Bricks {

		bricksCheckResponse.Total++

		brickCheckResponse := brickEntry.consistencyCheck(dump)
		if brickCheckResponse.Pending {
			bricksCheckResponse.Pending++
		}

		if len(brickCheckResponse.Inconsistencies) > 0 {
			bricksCheckResponse.Inconsistencies = append(bricksCheckResponse.Inconsistencies, brickCheckResponse.Inconsistencies...)
			bricksCheckResponse.NotOk++
		} else {
			bricksCheckResponse.Ok++
		}
	}

	return
}

func dbCheckPendingOps(dump Db) (pendingOpsCheckResponse DbBucketCheckResponse) {
	for _, pendingOpEntry := range dump.PendingOperations {

		pendingOpsCheckResponse.Total++

		pendingOpCheckResponse := pendingOpEntry.consistencyCheck(dump)

		if len(pendingOpCheckResponse.Inconsistencies) > 0 {
			pendingOpsCheckResponse.Inconsistencies = append(pendingOpsCheckResponse.Inconsistencies, pendingOpCheckResponse.Inconsistencies...)
			pendingOpsCheckResponse.NotOk++
		} else {
			pendingOpsCheckResponse.Ok++
		}
	}

	return
}
