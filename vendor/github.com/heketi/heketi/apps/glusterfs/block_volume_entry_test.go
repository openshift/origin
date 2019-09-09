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
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/tests"
)

func createSampleBlockVolumeEntry(size int) *BlockVolumeEntry {
	req := &api.BlockVolumeCreateRequest{}
	req.Size = size

	v := NewBlockVolumeEntryFromRequest(req)

	return v
}

func TestNewBlockVolumeEntry(t *testing.T) {
	bv := NewBlockVolumeEntry()

	tests.Assert(t, len(bv.Info.Id) == 0)
	tests.Assert(t, len(bv.Info.Cluster) == 0)
	tests.Assert(t, len(bv.Info.Clusters) == 0)
	tests.Assert(t, len(bv.Info.BlockHostingVolume) == 0)
	tests.Assert(t, len(bv.Info.Name) == 0)
	tests.Assert(t, bv.Info.Auth == false)
}

func TestNewBlockVolumeEntryFromRequestOnlySize(t *testing.T) {

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 512

	bv := NewBlockVolumeEntryFromRequest(req)
	tests.Assert(t, bv.Info.Name == "blockvol_"+bv.Info.Id)
	tests.Assert(t, len(bv.Info.Clusters) == 0)
	tests.Assert(t, bv.Info.Size == 512)
	tests.Assert(t, bv.Info.Cluster == "")
	tests.Assert(t, len(bv.Info.Id) != 0)
	tests.Assert(t, len(bv.Info.BlockHostingVolume) == 0)
	tests.Assert(t, bv.Info.Hacount == 0)
	tests.Assert(t, bv.Info.Auth == false)
}

func TestNewBlockVolumeEntryFromRequestAuthEnable(t *testing.T) {

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 512
	req.Auth = true

	bv := NewBlockVolumeEntryFromRequest(req)
	tests.Assert(t, bv.Info.Name == "blockvol_"+bv.Info.Id)
	tests.Assert(t, len(bv.Info.Clusters) == 0)
	tests.Assert(t, bv.Info.Size == 512)
	tests.Assert(t, bv.Info.Cluster == "")
	tests.Assert(t, len(bv.Info.Id) != 0)
	tests.Assert(t, len(bv.Info.BlockHostingVolume) == 0)
	tests.Assert(t, bv.Info.Hacount == 0)
	tests.Assert(t, bv.Info.Auth == true)
}

func TestNewBlockVolumeEntryFromRequestClusters(t *testing.T) {

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 512
	req.Clusters = []string{"abc", "def"}

	bv := NewBlockVolumeEntryFromRequest(req)
	tests.Assert(t, bv.Info.Name == "blockvol_"+bv.Info.Id)
	tests.Assert(t, bv.Info.Size == 512)
	tests.Assert(t, reflect.DeepEqual(req.Clusters, bv.Info.Clusters))
	tests.Assert(t, len(bv.Info.Id) != 0)
	tests.Assert(t, len(bv.Info.BlockHostingVolume) == 0)
	tests.Assert(t, bv.Info.Hacount == 0)
	tests.Assert(t, bv.Info.Auth == false)
}

func TestNewBlockVolumeEntryFromRequestName(t *testing.T) {

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 512
	req.Clusters = []string{"abc", "def"}
	req.Name = "myblockvol"

	bv := NewBlockVolumeEntryFromRequest(req)
	tests.Assert(t, bv.Info.Name == "myblockvol")
	tests.Assert(t, bv.Info.Size == 512)
	tests.Assert(t, reflect.DeepEqual(req.Clusters, bv.Info.Clusters))
	tests.Assert(t, len(bv.Info.Id) != 0)
	tests.Assert(t, len(bv.Info.BlockHostingVolume) == 0)
	tests.Assert(t, bv.Info.Hacount == 0)
	tests.Assert(t, bv.Info.Auth == false)

}

func TestNewBlockVolumeEntryMarshal(t *testing.T) {

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 512
	req.Clusters = []string{"abc", "def"}
	req.Name = "myvol"

	bv := NewBlockVolumeEntryFromRequest(req)

	buffer, err := bv.Marshal()
	tests.Assert(t, err == nil)
	tests.Assert(t, buffer != nil)
	tests.Assert(t, len(buffer) > 0)

	um := &BlockVolumeEntry{}
	err = um.Unmarshal(buffer)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(bv, um))

}

func TestBlockVolumeEntryFromIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Test for ID not found
	err := app.db.View(func(tx *bolt.Tx) error {
		_, err := NewBlockVolumeEntryFromId(tx, "123")
		return err
	})
	tests.Assert(t, err == ErrNotFound)

}

func TestBlockVolumeEntryFromId(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a blockvolume entry
	bv := createSampleBlockVolumeEntry(1024)

	// Save in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return bv.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Load from database
	var entry *BlockVolumeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewBlockVolumeEntryFromId(tx, bv.Info.Id)
		return err
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(entry, bv))

}

func TestBlockVolumeEntrySaveDelete(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a blockvolume entry
	bv := createSampleBlockVolumeEntry(1024)

	// Save in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return bv.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Delete entry which has devices
	var entry *BlockVolumeEntry
	err = app.db.Update(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewBlockVolumeEntryFromId(tx, bv.Info.Id)
		if err != nil {
			return err
		}

		err = entry.Delete(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Check volume has been deleted and is not in db
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewBlockVolumeEntryFromId(tx, bv.Info.Id)
		if err != nil {
			return err
		}
		return nil

	})
	tests.Assert(t, err == ErrNotFound)
}

func TestNewBlockVolumeEntryNewInfoResponse(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a blockvolume entry
	bv := createSampleBlockVolumeEntry(1024)

	// Save in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return bv.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Retrieve info response
	var info *api.BlockVolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		volume, err := NewBlockVolumeEntryFromId(tx, bv.Info.Id)
		if err != nil {
			return err
		}

		info, err = volume.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil, err)

	tests.Assert(t, info.Cluster == bv.Info.Cluster)
	tests.Assert(t, info.Name == bv.Info.Name)
	tests.Assert(t, info.Id == bv.Info.Id)
	tests.Assert(t, info.Size == bv.Info.Size)
}

func TestBlockVolumeEntryCreateMissingCluster(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a blockvolume entry
	bv := createSampleBlockVolumeEntry(1024)
	bv.Info.Clusters = []string{}

	// Save in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return bv.Save(tx)
	})
	tests.Assert(t, err == nil)

	err = bv.Create(app.db, app.executor)
	tests.Assert(t, err != nil, "expected err != nil")
	tests.Assert(t, strings.Contains(err.Error(), "No clusters"),
		`expected strings.Contains(err.Error(), "No clusters"), got:`, err)
}

func TestBlockVolumeEntryDestroy(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Lots of nodes with little drives
	err := setupSampleDbWithTopology(app,
		1,      // clusters
		4,      // nodes_per_cluster
		4,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create a blockvolume
	bv := createSampleBlockVolumeEntry(200)

	err = bv.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Destroy the blockvolume
	err = bv.Destroy(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Destroy the block hosting volume
	var vol *VolumeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		vol, err = NewVolumeEntryFromId(tx, bv.Info.BlockHostingVolume)
		tests.Assert(t, err == nil)
		tests.Assert(t, vol != nil)
		// Destroy block volumes from list
		tests.Assert(t, len(vol.Info.BlockInfo.BlockVolumes) == 0)

		return nil
	})
	tests.Assert(t, err == nil)
	err = vol.Destroy(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume does not exist
	err = app.db.View(func(tx *bolt.Tx) error {

		// Check that all devices have no used data
		devices, err := DeviceList(tx)
		tests.Assert(t, err == nil)
		for _, id := range devices {
			device, err := NewDeviceEntryFromId(tx, id)
			tests.Assert(t, err == nil)
			tests.Assert(t, device.Info.Storage.Used == 0)
			tests.Assert(t, device.Info.Storage.Total == device.Info.Storage.Free)
		}

		// Check there are no bricks
		bricks, err := BrickList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(bricks) == 0)

		return nil

	})
	tests.Assert(t, err == nil)

	// Check that the devices have no bricks
	err = app.db.View(func(tx *bolt.Tx) error {
		devices, err := DeviceList(tx)
		if err != nil {
			return err
		}

		for _, id := range devices {
			device, err := NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			tests.Assert(t, len(device.Bricks) == 0, id, device)
		}

		return err
	})
	tests.Assert(t, err == nil)

	// Check that the cluster has no volumes
	err = app.db.View(func(tx *bolt.Tx) error {
		clusters, err := ClusterList(tx)
		if err != nil {
			return err
		}

		tests.Assert(t, len(clusters) == 1)
		cluster, err := NewClusterEntryFromId(tx, clusters[0])
		tests.Assert(t, err == nil)
		tests.Assert(t, len(cluster.Info.Volumes) == 0)

		return nil
	})
	tests.Assert(t, err == nil)

}

func TestBlockVolumeEntryNameConflictSingleVolume(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// create a cluster to restrict to 1 Block Hosting Volume
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		2*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create blockvolume
	bv := createSampleBlockVolumeEntry(500)
	bv.Info.Name = "myvol"
	err = bv.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Create another blockvolume same name
	bv = createSampleBlockVolumeEntry(400)
	bv.Info.Name = "myvol"
	err = bv.Create(app.db, app.executor)
	tests.Assert(t, err != nil, err)
}

func TestBlockVolumeEntryNameConflictMultiVolume(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// create a cluster to restrict to 8 Block Hosting Volume
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		10,   // devices_per_node,
		1*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create 8 blockvolume
	for i := 0; i < 8; i++ {
		bv := createSampleBlockVolumeEntry(500)
		bv.Info.Name = "myvol"
		err = bv.Create(app.db, app.executor)
		tests.Assert(t, err == nil)
	}
	// Create another blockvolume same name
	bv := createSampleBlockVolumeEntry(400)
	bv.Info.Name = "myvol"
	err = bv.Create(app.db, app.executor)
	tests.Assert(t, err != nil, err)
}

func TestBlockVolumeEntryNameConflictMultiCluster(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// create a cluster to restrict to 1 Block Hosting Volume
	err := setupSampleDbWithTopology(app,
		10,   // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		2*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create 10 blockvolume
	for i := 0; i < 10; i++ {
		bv := createSampleBlockVolumeEntry(500)
		bv.Info.Name = "myvol"
		err = bv.Create(app.db, app.executor)
		tests.Assert(t, err == nil)
	}
	// Create another blockvolume same name
	bv := createSampleBlockVolumeEntry(400)
	bv.Info.Name = "myvol"
	err = bv.Create(app.db, app.executor)
	tests.Assert(t, err != nil, err)
}
