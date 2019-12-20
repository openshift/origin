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
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/sortedstrings"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

func createSampleReplicaVolumeEntry(size int, replica int) *VolumeEntry {
	req := &api.VolumeCreateRequest{}
	req.Size = size
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = replica

	v := NewVolumeEntryFromRequest(req)

	return v
}

func setupSampleDbWithTopology(app *App,
	clusters, nodes_per_cluster, devices_per_node int,
	disksize uint64) error {

	return setupSampleDbWithTopologyWithZones(app,
		clusters,
		nodes_per_cluster,
		nodes_per_cluster,
		devices_per_node,
		disksize)
}

func setupSampleDbWithTopologyWithZones(app *App,
	clusters, zones_per_cluster, nodes_per_cluster, devices_per_node int,
	disksize uint64) error {

	err := app.db.Update(func(tx *bolt.Tx) error {
		for c := 0; c < clusters; c++ {
			cluster := createSampleClusterEntry()

			for n := 0; n < nodes_per_cluster; n++ {
				node := createSampleNodeEntry()
				node.Info.ClusterId = cluster.Info.Id
				node.Info.Zone = n % zones_per_cluster

				cluster.NodeAdd(node.Info.Id)

				for d := 0; d < devices_per_node; d++ {
					device := createSampleDeviceEntry(node.Info.Id, disksize)
					node.DeviceAdd(device.Id())

					err := device.Save(tx)
					if err != nil {
						return err
					}
				}
				err := node.Save(tx)
				if err != nil {
					return err
				}
			}
			err := cluster.Save(tx)
			if err != nil {
				return err
			}
		}

		var err error
		_, err = ClusterList(tx)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil
	}

	return nil
}

// This creates and unbalanced topology in the sense that the zones
// do not contain the same number of nodes, but zone #i contains #i
// nodes. Each node contains zone-number many devices.
func setupSampleDbWithUnbalancedTopology(app *App,
	zones int, disksize uint64) error {

	err := app.db.Update(func(tx *bolt.Tx) error {
		cluster := createSampleClusterEntry()

		for z := 1; z <= zones; z++ {
			// create zone number nodes in the zone
			for n := 1; n <= z; n++ {
				node := createSampleNodeEntry()
				node.Info.ClusterId = cluster.Info.Id
				node.Info.Zone = z

				cluster.NodeAdd(node.Info.Id)

				for d := 1; d <= z; d++ {
					device := createSampleDeviceEntry(
						node.Info.Id, disksize)
					node.DeviceAdd(device.Id())

					if err := device.Save(tx); err != nil {
						return err
					}
				}

				if err := node.Save(tx); err != nil {
					return err
				}

			}
		}

		if err := cluster.Save(tx); err != nil {
			return err
		}

		var err error
		_, err = ClusterList(tx)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil
	}

	return nil
}

func TestNewVolumeEntry(t *testing.T) {
	v := NewVolumeEntry()

	tests.Assert(t, v.Bricks != nil)
	tests.Assert(t, len(v.Info.Id) == 0)
	tests.Assert(t, len(v.Info.Cluster) == 0)
	tests.Assert(t, len(v.Info.Clusters) == 0)
}

func TestNewVolumeEntryFromRequestOnlySize(t *testing.T) {

	req := &api.VolumeCreateRequest{}
	req.Size = 1024

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, len(v.Info.Clusters) == 0)
	tests.Assert(t, v.Info.Snapshot.Enable == false)
	tests.Assert(t, v.Info.Snapshot.Factor == 1)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, v.Info.Cluster == "")
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)
	tests.Assert(t, v.Info.Durability.Type == "")
}

func TestNewVolumeEntryFromRequestReplicaDefault(t *testing.T) {
	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, len(v.Info.Clusters) == 0)
	tests.Assert(t, v.Info.Snapshot.Enable == false)
	tests.Assert(t, v.Info.Snapshot.Factor == 1)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, v.Info.Cluster == "")
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)
	tests.Assert(t, v.Info.Durability.Type == api.DurabilityReplicate)
	tests.Assert(t, v.Info.Durability.Replicate.Replica == 0)
}

func TestNewVolumeEntryFromRequestReplica5(t *testing.T) {
	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 5

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, len(v.Info.Clusters) == 0)
	tests.Assert(t, v.Info.Snapshot.Enable == false)
	tests.Assert(t, v.Info.Snapshot.Factor == 1)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, v.Info.Cluster == "")
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)
	tests.Assert(t, v.Info.Durability.Type == api.DurabilityReplicate)
	tests.Assert(t, v.Info.Durability.Replicate.Replica == 5)
}

func TestNewVolumeEntryFromRequestDistribute(t *testing.T) {
	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityDistributeOnly

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, len(v.Info.Clusters) == 0)
	tests.Assert(t, v.Info.Snapshot.Enable == false)
	tests.Assert(t, v.Info.Snapshot.Factor == 1)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, v.Info.Cluster == "")
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)
	tests.Assert(t, v.Info.Durability.Type == api.DurabilityDistributeOnly)
}

func TestNewVolumeEntryFromRequestDisperseDefault(t *testing.T) {
	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityEC

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, len(v.Info.Clusters) == 0)
	tests.Assert(t, v.Info.Snapshot.Enable == false)
	tests.Assert(t, v.Info.Snapshot.Factor == 1)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, v.Info.Cluster == "")
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)
	tests.Assert(t, v.Info.Durability.Type == api.DurabilityEC)
	tests.Assert(t, v.Info.Durability.Disperse.Data == 0)
	tests.Assert(t, v.Info.Durability.Disperse.Redundancy == 0)
}

func TestNewVolumeEntryFromRequestDisperseDefault48(t *testing.T) {
	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityEC
	req.Durability.Disperse.Data = 8
	req.Durability.Disperse.Redundancy = 4

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, len(v.Info.Clusters) == 0)
	tests.Assert(t, v.Info.Snapshot.Enable == false)
	tests.Assert(t, v.Info.Snapshot.Factor == 1)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, v.Info.Cluster == "")
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)
	tests.Assert(t, v.Info.Durability.Type == api.DurabilityEC)
	tests.Assert(t, v.Info.Durability.Disperse.Data == 8)
	tests.Assert(t, v.Info.Durability.Disperse.Redundancy == 4)
}

func TestNewVolumeEntryFromRequestClusters(t *testing.T) {

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Clusters = []string{"abc", "def"}

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, v.Info.Snapshot.Enable == false)
	tests.Assert(t, v.Info.Snapshot.Factor == 1)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, reflect.DeepEqual(req.Clusters, v.Info.Clusters))
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)

}

func TestNewVolumeEntryFromRequestSnapshotEnabledDefaultFactor(t *testing.T) {

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Clusters = []string{"abc", "def"}
	req.Snapshot.Enable = true

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, v.Info.Snapshot.Enable == true)
	tests.Assert(t, v.Info.Snapshot.Factor == DEFAULT_THINP_SNAPSHOT_FACTOR)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, reflect.DeepEqual(req.Clusters, v.Info.Clusters))
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)

}

func TestNewVolumeEntryFromRequestSnapshotFactor(t *testing.T) {

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Clusters = []string{"abc", "def"}
	req.Snapshot.Enable = true
	req.Snapshot.Factor = 1.3

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, v.Info.Snapshot.Enable == true)
	tests.Assert(t, v.Info.Snapshot.Factor == 1.3)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, reflect.DeepEqual(req.Clusters, v.Info.Clusters))
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)

}

func TestNewVolumeEntryFromRequestName(t *testing.T) {

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Clusters = []string{"abc", "def"}
	req.Snapshot.Enable = true
	req.Snapshot.Factor = 1.3
	req.Name = "myvol"

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "myvol")
	tests.Assert(t, v.Info.Snapshot.Enable == true)
	tests.Assert(t, v.Info.Snapshot.Factor == 1.3)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, reflect.DeepEqual(req.Clusters, v.Info.Clusters))
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)

}

func TestNewVolumeEntryMarshal(t *testing.T) {

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Clusters = []string{"abc", "def"}
	req.Snapshot.Enable = true
	req.Snapshot.Factor = 1.3
	req.Name = "myvol"

	v := NewVolumeEntryFromRequest(req)
	v.BrickAdd("abc")
	v.BrickAdd("def")

	buffer, err := v.Marshal()
	tests.Assert(t, err == nil)
	tests.Assert(t, buffer != nil)
	tests.Assert(t, len(buffer) > 0)

	um := &VolumeEntry{}
	err = um.Unmarshal(buffer)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(v, um))

}

func TestVolumeEntryAddDeleteDevices(t *testing.T) {

	v := NewVolumeEntry()
	tests.Assert(t, len(v.Bricks) == 0)

	v.BrickAdd("123")
	tests.Assert(t, sortedstrings.Has(v.Bricks, "123"))
	tests.Assert(t, len(v.Bricks) == 1)
	v.BrickAdd("abc")
	tests.Assert(t, sortedstrings.Has(v.Bricks, "123"))
	tests.Assert(t, sortedstrings.Has(v.Bricks, "abc"))
	tests.Assert(t, len(v.Bricks) == 2)

	v.BrickDelete("123")
	tests.Assert(t, !sortedstrings.Has(v.Bricks, "123"))
	tests.Assert(t, sortedstrings.Has(v.Bricks, "abc"))
	tests.Assert(t, len(v.Bricks) == 1)

	v.BrickDelete("ccc")
	tests.Assert(t, !sortedstrings.Has(v.Bricks, "123"))
	tests.Assert(t, sortedstrings.Has(v.Bricks, "abc"))
	tests.Assert(t, len(v.Bricks) == 1)
}

func TestVolumeEntryFromIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Test for ID not found
	err := app.db.View(func(tx *bolt.Tx) error {
		_, err := NewVolumeEntryFromId(tx, "123")
		return err
	})
	tests.Assert(t, err == ErrNotFound)

}

func TestVolumeEntryFromId(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a volume entry
	v := createSampleReplicaVolumeEntry(1024, 2)

	// Save in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return v.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Load from database
	var entry *VolumeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewVolumeEntryFromId(tx, v.Info.Id)
		return err
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(entry, v))

}

func TestVolumeEntrySaveDelete(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a volume entry
	v := createSampleReplicaVolumeEntry(1024, 2)

	// Save in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return v.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Delete entry which has devices
	var entry *VolumeEntry
	err = app.db.Update(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewVolumeEntryFromId(tx, v.Info.Id)
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
		entry, err = NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}
		return nil

	})
	tests.Assert(t, err == ErrNotFound)
}

// TestNewVolumeEntryNewInfoResponse creates a sample cluster and a volume
// using a volumeEntry from request.  We verify that the volumeInfoResponse
// matches with the input request and the volumeEntry on the serverside.
func TestNewVolumeEntryNewInfoResponse(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,     // clusters
		4,     // nodes_per_cluster
		4,     // devices_per_node,
		10*GB, // disksize, 10G)
	)
	tests.Assert(t, err == nil)

	// Create a volume entry
	v := createSampleReplicaVolumeEntry(5, 3)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Retrieve info response
	var info *api.VolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		volume, err := NewVolumeEntryFromId(tx, v.Info.Id)
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

	tests.Assert(t, info.Cluster == v.Info.Cluster)
	tests.Assert(t, reflect.DeepEqual(info.Snapshot, v.Info.Snapshot))
	tests.Assert(t, info.Name == v.Info.Name)
	tests.Assert(t, info.Id == v.Info.Id)
	tests.Assert(t, info.Size == v.Info.Size)
	tests.Assert(t, len(info.Bricks) == 3)
}

func TestVolumeEntryCreateMissingCluster(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a volume entry
	v := createSampleReplicaVolumeEntry(1024, 2)
	v.Info.Clusters = []string{}

	// Save in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return v.Save(tx)
	})
	tests.Assert(t, err == nil)

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err != nil, "expected err != nil")
	tests.Assert(t, strings.Contains(err.Error(), "No clusters"),
		`expected strings.Contains(err.Error(), "No clusters"), got:`, err)
}

func TestVolumeEntryCreateRunOutOfSpaceMinBrickSizeLimit(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Total 80GB
	err := setupSampleDbWithTopology(app,
		1,     // clusters
		2,     // nodes_per_cluster
		4,     // devices_per_node,
		10*GB, // disksize, 10G)
	)
	tests.Assert(t, err == nil)

	// Create a 100 GB volume
	// Shouldn't be able to break it down enough to allocate volume
	v := createSampleReplicaVolumeEntry(100, 2)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == ErrNoSpace)
	tests.Assert(t, v.Info.Cluster == "")

	// Check database volume does not exist
	err = app.db.View(func(tx *bolt.Tx) error {
		_, err := NewVolumeEntryFromId(tx, v.Info.Id)
		return err
	})
	tests.Assert(t, err == ErrNotFound)

	// Check no bricks or volumes exist
	var bricks []string
	var volumes []string
	err = app.db.View(func(tx *bolt.Tx) error {
		bricks = EntryKeys(tx, BOLTDB_BUCKET_BRICK)
		volumes = EntryKeys(tx, BOLTDB_BUCKET_VOLUME)

		return nil
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, len(bricks) == 0, bricks)
	tests.Assert(t, len(volumes) == 0)

}

func TestVolumeEntryCreateRunOutOfSpaceMaxBrickLimit(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Lots of nodes with little drives
	err := setupSampleDbWithTopology(app,
		1,  // clusters
		20, // nodes_per_cluster
		40, // devices_per_node,

		// Must be larger than the brick min size
		BrickMinSize*2, // disksize
	)
	tests.Assert(t, err == nil)

	// Create a volume who will be broken down to
	// Shouldn't be able to break it down enough to allocate volume
	v := createSampleReplicaVolumeEntry(BrickMaxNum*2*int(BrickMinSize/GB), 2)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == ErrNoSpace)

	// Check database volume does not exist
	err = app.db.View(func(tx *bolt.Tx) error {
		_, err := NewVolumeEntryFromId(tx, v.Info.Id)
		return err
	})
	tests.Assert(t, err == ErrNotFound)

	// Check no bricks or volumes exist
	var bricks []string
	var volumes []string
	app.db.View(func(tx *bolt.Tx) error {
		bricks = EntryKeys(tx, BOLTDB_BUCKET_BRICK)

		volumes = EntryKeys(tx, BOLTDB_BUCKET_VOLUME)
		return nil
	})
	tests.Assert(t, len(bricks) == 0)
	tests.Assert(t, len(volumes) == 0)

}

func TestVolumeEntryCreateTwoBricks(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a cluster in the database
	err := setupSampleDbWithTopology(app,
		1,      // clusters
		4,      // nodes_per_cluster
		4,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Mock Brick creation and check it was called
	brickCreateCount := 0
	gid := int64(1000)
	var mutex sync.Mutex
	app.xo.MockBrickCreate = func(host string,
		brick *executors.BrickRequest) (*executors.BrickInfo, error) {

		mutex.Lock()
		brickCreateCount++
		mutex.Unlock()

		bInfo := &executors.BrickInfo{
			Path: "/mockpath",
		}

		tests.Assert(t, brick.Gid == gid,
			"expected brick.Gid == gid, got:", brick.Gid, gid)
		return bInfo, nil
	}

	// Create a volume who will be broken down to
	v := createSampleReplicaVolumeEntry(250, 2)

	// Set a GID
	v.Info.Gid = gid

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, err)
	tests.Assert(t, brickCreateCount == 2)

	// Check database
	var info *api.VolumeInfoResponse
	var nodelist sort.StringSlice
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		cluster, err := NewClusterEntryFromId(tx, v.Info.Cluster)
		if err != nil {
			return err
		}
		nodelist = make(sort.StringSlice, len(cluster.Info.Nodes))

		for i, id := range cluster.Info.Nodes {
			node, err := NewNodeEntryFromId(tx, id)
			if err != nil {
				return err
			}
			nodelist[i] = node.StorageHostName()
		}
		nodelist.Sort()

		return nil

	})
	tests.Assert(t, err == nil)

	// Check that it used only two bricks each with only two replicas
	tests.Assert(t, len(info.Bricks) == 2)
	tests.Assert(t, info.Bricks[0].Size == info.Bricks[1].Size)
	tests.Assert(t, info.Cluster == v.Info.Cluster)

	// Check information on the bricks
	for _, brick := range info.Bricks {
		tests.Assert(t, brick.DeviceId != "")
		tests.Assert(t, brick.NodeId != "")
		tests.Assert(t, brick.Path != "")
	}

	// Check mount information
	host := strings.Split(info.Mount.GlusterFS.MountPoint, ":")[0]
	tests.Assert(t, sortedstrings.Has(nodelist, host), host, nodelist)
	volfileServers := strings.Split(info.Mount.GlusterFS.Options["backup-volfile-servers"], ",")
	for index, node := range volfileServers {
		tests.Assert(t, node != host, index, node, host)
	}

	// Should have at least the number nodes as replicas
	tests.Assert(t, len(info.Mount.GlusterFS.Hosts) >= info.Durability.Replicate.Replica,
		info.Mount.GlusterFS.Hosts,
		info)

	// Check all hosts are in the list
	app.db.View(func(tx *bolt.Tx) error {
		for _, brick := range info.Bricks {
			found := false

			node, err := NewNodeEntryFromId(tx, brick.NodeId)
			tests.Assert(t, err == nil)

			for _, host := range info.Mount.GlusterFS.Hosts {
				if host == node.StorageHostName() {
					found = true
					break
				}
			}
			tests.Assert(t, found, node.StorageHostName(),
				info.Mount.GlusterFS.Hosts)
		}

		return nil
	})

}

func TestVolumeEntryCreateBrickDivision(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create 50TB of storage
	err := setupSampleDbWithTopology(app,
		1,      // clusters
		10,     // nodes_per_cluster
		10,     // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create a volume which is so big that it does
	// not fit into a single replica set
	v := createSampleReplicaVolumeEntry(2000, 2)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	var info *api.VolumeInfoResponse
	var nodelist sort.StringSlice
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		cluster, err := NewClusterEntryFromId(tx, v.Info.Cluster)
		if err != nil {
			return err
		}
		nodelist = make(sort.StringSlice, len(cluster.Info.Nodes))

		for i, id := range cluster.Info.Nodes {
			node, err := NewNodeEntryFromId(tx, id)
			if err != nil {
				return err
			}
			nodelist[i] = node.StorageHostName()
		}
		nodelist.Sort()

		return nil

	})
	tests.Assert(t, err == nil)

	// Will need 3 splits for a total of 8 bricks + replicas
	//
	// NOTE: Why 8 bricksets of 250GB instead of 4 bricksets
	// of 500GB each? Because the disk needed for hosting
	// a brick if size X is a little larger than X, because
	// we additionally allocate at least some space for metadata.
	// Hence we will end up using 250GB bricks, and no two bricks
	// will be on the same disk. Requesting slightly less than
	// 2000GB, e.g. 1940GB yields a four 485GB bricksets.
	//
	tests.Assert(t, len(info.Bricks) == 16)
	for b := 1; b < 16; b++ {
		tests.Assert(t, 250*GB == info.Bricks[b].Size, b)
	}
	tests.Assert(t, info.Cluster == v.Info.Cluster)

	// Check mount information
	host := strings.Split(info.Mount.GlusterFS.MountPoint, ":")[0]
	tests.Assert(t, sortedstrings.Has(nodelist, host), host, nodelist)
	volfileServers := strings.Split(info.Mount.GlusterFS.Options["backup-volfile-servers"], ",")
	for index, node := range volfileServers {
		tests.Assert(t, node != host, index, node, host)
	}

}

func TestVolumeEntryCreateMaxBrickSize(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create 500TB of storage
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create a volume whose bricks must be at most BrickMaxSize
	v := createSampleReplicaVolumeEntry(int(BrickMaxSize/GB*4), 2)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Get volume information
	var info *api.VolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Check the size of the bricks are not bigger than BrickMaxSize
	tests.Assert(t, len(info.Bricks) == 8)
	for b := 1; b < len(info.Bricks); b++ {
		tests.Assert(t, info.Bricks[b].Size <= BrickMaxSize)
	}
	tests.Assert(t, info.Cluster == v.Info.Cluster)

}

func TestVolumeEntryCreateOnClustersRequested(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create 50TB of storage
	err := setupSampleDbWithTopology(app,
		10,   // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Get a cluster list
	var clusters sort.StringSlice
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		clusters, err = ClusterList(tx)
		return err
	})
	tests.Assert(t, err == nil)
	clusters.Sort()

	// Create a 1TB volume
	v := createSampleReplicaVolumeEntry(1024, 2)

	// Set the clusters to the first two cluster ids
	v.Info.Clusters = []string{clusters[0]}

	// Create volume
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume does not exist
	var info *api.VolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, info.Cluster == clusters[0])

	// Create a new volume on either of three clusters
	clusterset := clusters[2:5]
	v = createSampleReplicaVolumeEntry(1024, 2)
	v.Info.Clusters = clusterset
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume exists
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, sortedstrings.Has(clusterset, info.Cluster))

}

func TestVolumeEntryCreateCheckingClustersForSpace(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create 100 small clusters
	err := setupSampleDbWithTopology(app,
		10,    // clusters
		1,     // nodes_per_cluster
		1,     // devices_per_node,
		10*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create one large cluster
	cluster := createSampleClusterEntry()
	err = app.db.Update(func(tx *bolt.Tx) error {
		for n := 0; n < 100; n++ {
			node := createSampleNodeEntry()
			node.Info.ClusterId = cluster.Info.Id
			node.Info.Zone = n % 2

			cluster.NodeAdd(node.Info.Id)

			for d := 0; d < 10; d++ {
				device := createSampleDeviceEntry(node.Info.Id, 4*TB)
				node.DeviceAdd(device.Id())

				// Save
				err = device.Save(tx)
				if err != nil {
					return err
				}
			}
			err := node.Save(tx)
			if err != nil {
				return err
			}
		}
		err := cluster.Save(tx)
		if err != nil {
			return err
		}

		return nil
	})

	// Create a 1TB volume
	v := createSampleReplicaVolumeEntry(1024, 2)

	// Create volume
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume exists
	var info *api.VolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, info.Cluster == cluster.Info.Id)
}

func TestVolumeEntryCreateWithSnapshot(t *testing.T) {
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

	// Create a volume with a snapshot factor of 1.5
	// For a 200G vol, it would get a brick size of 100G, with a thin pool
	// size of 100G * 1.5 = 150GB.
	v := createSampleReplicaVolumeEntry(200, 2)
	v.Info.Snapshot.Enable = true
	v.Info.Snapshot.Factor = 1.5

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume exists
	var info *api.VolumeInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		entry, err := NewVolumeEntryFromId(tx, v.Info.Id)
		if err != nil {
			return err
		}

		info, err = entry.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Check that it used only two bricks each with only two replicas
	tests.Assert(t, len(info.Bricks) == 2)
	err = app.db.View(func(tx *bolt.Tx) error {
		for _, b := range info.Bricks {
			device, err := NewDeviceEntryFromId(tx, b.DeviceId)
			if err != nil {
				return err
			}

			tests.Assert(t, device.Info.Storage.Used >= uint64(1.5*float32(b.Size)))
		}

		return nil
	})
	tests.Assert(t, err == nil)
}

func TestVolumeEntryCreateBrickCreationFailure(t *testing.T) {
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

	// Cause a brick creation failure
	mockerror := errors.New("MOCK")
	app.xo.MockBrickCreate = func(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error) {
		return nil, mockerror
	}

	// Create a volume with a snapshot factor of 1.5
	// For a 200G vol, it would get a brick size of 100G, with a thin pool
	// size of 100G * 1.5 = 150GB.
	v := createSampleReplicaVolumeEntry(200, 2)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == mockerror, err, mockerror)

	// Check database is still clean. No bricks and No volumes
	err = app.db.View(func(tx *bolt.Tx) error {
		volumes, err := VolumeList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(volumes) == 0)

		bricks, err := BrickList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(bricks) == 0)

		clusters, err := ClusterList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(clusters) == 1)

		cluster, err := NewClusterEntryFromId(tx, clusters[0])
		tests.Assert(t, err == nil)
		tests.Assert(t, len(cluster.Info.Volumes) == 0)

		return nil

	})
	tests.Assert(t, err == nil)
}

func TestVolumeEntryCreateVolumeCreationFailure(t *testing.T) {
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

	// Cause a brick creation failure
	mockerror := errors.New("MOCK")
	app.xo.MockVolumeCreate = func(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
		return nil, mockerror
	}

	// Create a volume with a snapshot factor of 1.5
	// For a 200G vol, it would get a brick size of 100G, with a thin pool
	// size of 100G * 1.5 = 150GB.
	v := createSampleReplicaVolumeEntry(200, 2)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == mockerror)

	// Check database is still clean. No bricks and No volumes
	err = app.db.View(func(tx *bolt.Tx) error {
		volumes, err := VolumeList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(volumes) == 0)

		bricks, err := BrickList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(bricks) == 0)

		clusters, err := ClusterList(tx)
		tests.Assert(t, err == nil)
		tests.Assert(t, len(clusters) == 1)

		cluster, err := NewClusterEntryFromId(tx, clusters[0])
		tests.Assert(t, err == nil)
		tests.Assert(t, len(cluster.Info.Volumes) == 0,
			"expected len(cluster.Info.Volumes) == 0, got:",
			len(cluster.Info.Volumes))

		return nil

	})
	tests.Assert(t, err == nil)
}

func TestVolumeEntryDestroy(t *testing.T) {
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

	// Create a volume with a snapshot factor of 1.5
	// For a 200G vol, it would get a brick size of 100G, with a thin pool
	// size of 100G * 1.5 = 150GB.
	v := createSampleReplicaVolumeEntry(200, 2)
	v.Info.Snapshot.Enable = true
	v.Info.Snapshot.Factor = 1.5

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Destroy the volume
	err = v.Destroy(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Check database volume does not exist
	app.db.View(func(tx *bolt.Tx) error {

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
		tests.Assert(t, len(bricks) == 0)
		tests.Assert(t, err == nil)

		return nil

	})

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

func TestVolumeEntryExpandNoSpace(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create cluster
	err := setupSampleDbWithTopology(app,
		10,     // clusters
		2,      // nodes_per_cluster
		2,      // devices_per_node,
		600*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create large volume
	v := createSampleReplicaVolumeEntry(1190, 2)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Save a copy of the volume before expansion
	vcopy := &VolumeEntry{}
	*vcopy = *v

	// Asking for a large amount will require too many little bricks
	err = v.Expand(app.db, app.executor, 5000)
	tests.Assert(t, err == ErrMaxBricks, err)

	// Asking for a small amount will set the bricks too small
	err = v.Expand(app.db, app.executor, 10)
	tests.Assert(t, err == ErrMinimumBrickSize, err)

	// Check db is the same as before expansion
	var entry *VolumeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewVolumeEntryFromId(tx, v.Info.Id)

		return err
	})
	tests.Assert(t, err == nil, err)
	tests.Assert(t, reflect.DeepEqual(vcopy, entry))
}

func TestVolumeEntryExpandMaxBrickLimit(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a large cluster
	err := setupSampleDbWithTopology(app,
		10,     // clusters
		4,      // nodes_per_cluster
		24,     // devices_per_node,
		600*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create large volume
	v := createSampleReplicaVolumeEntry(100, 2)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Add a bunch of bricks until the limit
	fakebricks := make(sort.StringSlice, BrickMaxNum-len(v.Bricks))
	v.Bricks = append(v.Bricks, fakebricks...)

	// Try to expand the volume, but it will return that the max number
	// of bricks has been reached
	err = v.Expand(app.db, app.executor, 100)
	tests.Assert(t, err == ErrMaxBricks, err)
}

func TestVolumeEntryExpandCreateBricksFailure(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create large cluster
	err := setupSampleDbWithTopology(app,
		10,     // clusters
		10,     // nodes_per_cluster
		20,     // devices_per_node,
		600*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create volume
	v := createSampleReplicaVolumeEntry(100, 2)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Save a copy of the volume before expansion
	vcopy := &VolumeEntry{}
	*vcopy = *v

	// Mock create bricks to fail
	ErrMock := errors.New("MOCK")
	app.xo.MockBrickCreate = func(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error) {
		return nil, ErrMock
	}

	// Expand volume
	err = v.Expand(app.db, app.executor, 500)
	tests.Assert(t, err == ErrMock)

	// Check db is the same as before expansion
	var entry *VolumeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewVolumeEntryFromId(tx, v.Info.Id)

		return err
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(vcopy, entry))
}

func TestVolumeEntryExpand(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create large cluster
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		10,   // nodes_per_cluster
		20,   // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create volume
	v := createSampleReplicaVolumeEntry(1024, 2)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, len(v.Bricks) == 2)

	// Expand volume
	err = v.Expand(app.db, app.executor, 1234)
	tests.Assert(t, err == nil)
	tests.Assert(t, v.Info.Size == 1024+1234)
	tests.Assert(t, len(v.Bricks) == 4)

	// Check db
	var entry *VolumeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		entry, err = NewVolumeEntryFromId(tx, v.Info.Id)

		return err
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(entry, v))
}

func TestVolumeEntryDoNotAllowDeviceOnSameNode(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create cluster with plenty of space, but
	// it will not have enough nodes
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		1,    // nodes_per_cluster
		200,  // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create volume
	v := createSampleReplicaVolumeEntry(100, 2)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err != nil, err)
	tests.Assert(t, err == ErrNoSpace)

	v = createSampleReplicaVolumeEntry(10000, 2)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err != nil, err)
	tests.Assert(t, err == ErrNoSpace)
}

func TestVolumeEntryDestroyCheck(t *testing.T) {
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

	// Create a volume with a snapshot factor of 1.5
	// For a 200G vol, it would get a brick size of 100G, with a thin pool
	// size of 100G * 1.5 = 150GB.
	v := createSampleReplicaVolumeEntry(200, 2)
	v.Info.Snapshot.Enable = true
	v.Info.Snapshot.Factor = 1.5

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// volumes (and bricks) can now be destroyed even when snapshots exist
	// Now it should be able to be deleted
	err = v.Destroy(app.db, app.executor)
	tests.Assert(t, err == nil)

}

func TestVolumeEntryNameConflictSingleCluster(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		6,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create volume
	v := createSampleReplicaVolumeEntry(1024, 2)
	v.Info.Name = "myvol"
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Create another volume same name
	v = createSampleReplicaVolumeEntry(10000, 2)
	v.Info.Name = "myvol"
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err != nil, err)
}

func TestVolumeEntryNameConflictMultiCluster(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create 10 clusters
	err := setupSampleDbWithTopology(app,
		10,   // clusters
		3,    // nodes_per_cluster
		6,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create 10 volumes
	for i := 0; i < 10; i++ {
		v := createSampleReplicaVolumeEntry(1024, 2)
		v.Info.Name = "myvol"
		err = v.Create(app.db, app.executor)
		logger.Info("%v", v.Info.Cluster)
		tests.Assert(t, err == nil, err)
	}

	// Create another volume same name
	v := createSampleReplicaVolumeEntry(10000, 2)
	v.Info.Name = "myvol"
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err != nil, err)
}

func TestReplaceBrickInVolume(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a cluster in the database
	err := setupSampleDbWithTopology(app,
		1,      // clusters
		4,      // nodes_per_cluster
		1,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	v := createSampleReplicaVolumeEntry(100, 3)

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, err)
	var brickNames []string
	var be *BrickEntry
	err = app.db.View(func(tx *bolt.Tx) error {

		for _, brick := range v.Bricks {
			be, err = NewBrickEntryFromId(tx, brick)
			if err != nil {
				return err
			}
			ne, err := NewNodeEntryFromId(tx, be.Info.NodeId)
			if err != nil {
				return err
			}
			brickName := fmt.Sprintf("%v:%v", ne.Info.Hostnames.Storage[0], be.Info.Path)
			brickNames = append(brickNames, brickName)
		}
		return nil
	})
	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		var bricks []executors.Brick
		brick := executors.Brick{Name: brickNames[0]}
		bricks = append(bricks, brick)
		brick = executors.Brick{Name: brickNames[1]}
		bricks = append(bricks, brick)
		brick = executors.Brick{Name: brickNames[2]}
		bricks = append(bricks, brick)
		Bricks := executors.Bricks{
			BrickList: bricks,
		}
		b := &executors.Volume{
			Bricks: Bricks,
		}
		return b, nil
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		var bricks executors.HealInfoBricks
		brick := executors.BrickHealStatus{Name: brickNames[0],
			NumberOfEntries: "0"}
		bricks.BrickList = append(bricks.BrickList, brick)
		brick = executors.BrickHealStatus{Name: brickNames[1],
			NumberOfEntries: "0"}
		bricks.BrickList = append(bricks.BrickList, brick)
		brick = executors.BrickHealStatus{Name: brickNames[2],
			NumberOfEntries: "0"}
		bricks.BrickList = append(bricks.BrickList, brick)
		h := &executors.HealInfo{
			Bricks: bricks,
		}
		return h, nil
	}
	brickId := be.Id()
	err = v.replaceBrickInVolume(app.db, app.executor, brickId)
	tests.Assert(t, err == nil, err)

	oldNode := be.Info.NodeId
	brickOnOldNode := false
	oldBrickIdExists := false

	err = app.db.View(func(tx *bolt.Tx) error {

		for _, brick := range v.Bricks {
			be, err = NewBrickEntryFromId(tx, brick)
			if err != nil {
				return err
			}
			ne, err := NewNodeEntryFromId(tx, be.Info.NodeId)
			if err != nil {
				return err
			}
			if ne.Info.Id == oldNode {
				brickOnOldNode = true
			}
			if be.Info.Id == brickId {
				oldBrickIdExists = true
			}
		}
		return nil
	})

	tests.Assert(t, !brickOnOldNode, "brick found on oldNode")
	tests.Assert(t, !oldBrickIdExists, "old Brick not deleted")
}

func TestNewVolumeEntryWithVolumeOptions(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a cluster in the database
	err := setupSampleDbWithTopology(app,
		1,      // clusters
		3,      // nodes_per_cluster
		1,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)
	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.GlusterVolumeOptions = []string{"test-option"}

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, v.Info.Snapshot.Enable == false)
	tests.Assert(t, v.Info.Snapshot.Factor == 1)
	tests.Assert(t, v.Info.Size == 1024)
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)
	tests.Assert(t, strings.Contains(strings.Join(v.GlusterVolumeOptions, ","), "test-option"))

	err = v.Create(app.db, app.executor)
	logger.Info("%v", v.Info.Cluster)
	tests.Assert(t, err == nil, err)

	// Check that the data on the database is recorded correctly
	var entry VolumeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		return entry.Unmarshal(
			tx.Bucket([]byte(BOLTDB_BUCKET_VOLUME)).
				Get([]byte(v.Info.Id)))
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, strings.Contains(strings.Join(entry.GlusterVolumeOptions, ","), "test-option"))

}

func TestNewVolumeSetsIdInVolumeOptions(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a cluster in the database
	err := setupSampleDbWithTopology(app,
		1,      // clusters
		3,      // nodes_per_cluster
		1,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)
	req := &api.VolumeCreateRequest{}
	req.Size = 1024

	v := NewVolumeEntryFromRequest(req)

	var glusterVolumeOptions []string
	app.xo.MockVolumeCreate = func(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
		glusterVolumeOptions = volume.GlusterVolumeOptions
		return &executors.Volume{}, nil
	}

	err = v.Create(app.db, app.executor)
	logger.Info("%v", v.Info.Cluster)
	tests.Assert(t, err == nil, err)

	heketiIDOption := fmt.Sprintf("%s %s", HEKETI_ID_KEY, v.Info.Id)
	tests.Assert(t, glusterVolumeOptions[len(glusterVolumeOptions)-1] == heketiIDOption)
}

func TestNewVolumeEntryWithTSPForMountHosts(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a cluster in the database
	err := setupSampleDbWithTopology(app,
		1,      // clusters
		5,      // nodes_per_cluster
		1,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)
	req := &api.VolumeCreateRequest{}
	req.Size = 100

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.Info.Name == "vol_"+v.Info.Id)
	tests.Assert(t, v.Info.Snapshot.Enable == false)
	tests.Assert(t, v.Info.Snapshot.Factor == 1)
	tests.Assert(t, v.Info.Size == 100)
	tests.Assert(t, len(v.Info.Id) != 0)
	tests.Assert(t, len(v.Bricks) == 0)

	err = v.Create(app.db, app.executor)
	logger.Info("%v", v.Info.Cluster)
	tests.Assert(t, err == nil, err)

	// Check that the data on the database is recorded correctly
	var entry VolumeEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		return entry.Unmarshal(
			tx.Bucket([]byte(BOLTDB_BUCKET_VOLUME)).
				Get([]byte(v.Info.Id)))
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, len(entry.Info.Mount.GlusterFS.Hosts) == 5)

}

func TestReplaceBrickInVolumeSelfHeal1(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a cluster in the database
	err := setupSampleDbWithTopology(app,
		1,      // clusters
		4,      // nodes_per_cluster
		1,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	v := createSampleReplicaVolumeEntry(100, 3)

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, err)
	var brickNames []string
	var be *BrickEntry
	err = app.db.View(func(tx *bolt.Tx) error {

		for _, brick := range v.Bricks {
			be, err = NewBrickEntryFromId(tx, brick)
			if err != nil {
				return err
			}
			ne, err := NewNodeEntryFromId(tx, be.Info.NodeId)
			if err != nil {
				return err
			}
			brickName := fmt.Sprintf("%v:%v", ne.Info.Hostnames.Storage[0], be.Info.Path)
			brickNames = append(brickNames, brickName)
		}
		return nil
	})
	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		var bricks []executors.Brick
		brick := executors.Brick{Name: brickNames[0]}
		bricks = append(bricks, brick)
		brick = executors.Brick{Name: brickNames[1]}
		bricks = append(bricks, brick)
		brick = executors.Brick{Name: brickNames[2]}
		bricks = append(bricks, brick)
		Bricks := executors.Bricks{
			BrickList: bricks,
		}
		b := &executors.Volume{
			Bricks: Bricks,
		}
		return b, nil
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		var bricks executors.HealInfoBricks
		brick := executors.BrickHealStatus{Name: brickNames[0],
			NumberOfEntries: "0"}
		bricks.BrickList = append(bricks.BrickList, brick)
		brick = executors.BrickHealStatus{Name: brickNames[1],
			NumberOfEntries: "0"}
		bricks.BrickList = append(bricks.BrickList, brick)
		// Skip entry for brick to be replaced, should pass
		h := &executors.HealInfo{
			Bricks: bricks,
		}
		return h, nil
	}
	brickId := be.Id()
	err = v.replaceBrickInVolume(app.db, app.executor, brickId)
	tests.Assert(t, err == nil, err)

	oldNode := be.Info.NodeId
	brickOnOldNode := false
	oldBrickIdExists := false

	err = app.db.View(func(tx *bolt.Tx) error {

		for _, brick := range v.Bricks {
			be, err = NewBrickEntryFromId(tx, brick)
			if err != nil {
				return err
			}
			ne, err := NewNodeEntryFromId(tx, be.Info.NodeId)
			if err != nil {
				return err
			}
			if ne.Info.Id == oldNode {
				brickOnOldNode = true
			}
			if be.Info.Id == brickId {
				oldBrickIdExists = true
			}
		}
		return nil
	})

	tests.Assert(t, !brickOnOldNode, "brick found on oldNode")
	tests.Assert(t, !oldBrickIdExists, "old Brick not deleted")
}

func TestReplaceBrickInVolumeSelfHeal2(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a cluster in the database
	err := setupSampleDbWithTopology(app,
		1,      // clusters
		4,      // nodes_per_cluster
		1,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	v := createSampleReplicaVolumeEntry(100, 3)

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, err)
	var brickNames []string
	var be *BrickEntry
	err = app.db.View(func(tx *bolt.Tx) error {

		for _, brick := range v.Bricks {
			be, err = NewBrickEntryFromId(tx, brick)
			if err != nil {
				return err
			}
			ne, err := NewNodeEntryFromId(tx, be.Info.NodeId)
			if err != nil {
				return err
			}
			brickName := fmt.Sprintf("%v:%v", ne.Info.Hostnames.Storage[0], be.Info.Path)
			brickNames = append(brickNames, brickName)
		}
		return nil
	})
	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		var bricks []executors.Brick
		brick := executors.Brick{Name: brickNames[0]}
		bricks = append(bricks, brick)
		brick = executors.Brick{Name: brickNames[1]}
		bricks = append(bricks, brick)
		brick = executors.Brick{Name: brickNames[2]}
		bricks = append(bricks, brick)
		Bricks := executors.Bricks{
			BrickList: bricks,
		}
		b := &executors.Volume{
			Bricks: Bricks,
		}
		return b, nil
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		var bricks executors.HealInfoBricks
		brick := executors.BrickHealStatus{Name: brickNames[0],
			NumberOfEntries: "0"}
		bricks.BrickList = append(bricks.BrickList, brick)
		brick = executors.BrickHealStatus{Name: brickNames[1],
			NumberOfEntries: "0"}
		bricks.BrickList = append(bricks.BrickList, brick)
		// Brick to be replaced is source, should fail
		brick = executors.BrickHealStatus{Name: brickNames[2],
			NumberOfEntries: "100"}
		bricks.BrickList = append(bricks.BrickList, brick)
		h := &executors.HealInfo{
			Bricks: bricks,
		}
		return h, nil
	}
	brickId := be.Id()
	err = v.replaceBrickInVolume(app.db, app.executor, brickId)
	tests.Assert(t, err != nil, err)

	oldNode := be.Info.NodeId
	brickOnOldNode := false
	oldBrickIdExists := false

	err = app.db.View(func(tx *bolt.Tx) error {

		for _, brick := range v.Bricks {
			be, err = NewBrickEntryFromId(tx, brick)
			if err != nil {
				return err
			}
			ne, err := NewNodeEntryFromId(tx, be.Info.NodeId)
			if err != nil {
				return err
			}
			if ne.Info.Id == oldNode {
				brickOnOldNode = true
			}
			if be.Info.Id == brickId {
				oldBrickIdExists = true
			}
		}
		return nil
	})

	tests.Assert(t, brickOnOldNode, "brick found on oldNode")
	tests.Assert(t, oldBrickIdExists, "old Brick not deleted")
}

func TestReplaceBrickInVolumeSelfHealQuorumNotMet(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a cluster in the database
	err := setupSampleDbWithTopology(app,
		1,      // clusters
		4,      // nodes_per_cluster
		1,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil)

	v := createSampleReplicaVolumeEntry(100, 3)

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, err)
	var brickNames []string
	var be *BrickEntry
	err = app.db.View(func(tx *bolt.Tx) error {

		for _, brick := range v.Bricks {
			be, err = NewBrickEntryFromId(tx, brick)
			if err != nil {
				return err
			}
			ne, err := NewNodeEntryFromId(tx, be.Info.NodeId)
			if err != nil {
				return err
			}
			brickName := fmt.Sprintf("%v:%v", ne.Info.Hostnames.Storage[0], be.Info.Path)
			brickNames = append(brickNames, brickName)
		}
		return nil
	})
	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		var bricks []executors.Brick
		brick := executors.Brick{Name: brickNames[0]}
		bricks = append(bricks, brick)
		brick = executors.Brick{Name: brickNames[1]}
		bricks = append(bricks, brick)
		brick = executors.Brick{Name: brickNames[2]}
		bricks = append(bricks, brick)
		Bricks := executors.Bricks{
			BrickList: bricks,
		}
		b := &executors.Volume{
			Bricks: Bricks,
		}
		return b, nil
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		var bricks executors.HealInfoBricks
		brick := executors.BrickHealStatus{Name: brickNames[0],
			NumberOfEntries: "0"}
		bricks.BrickList = append(bricks.BrickList, brick)
		// Quorum not met, should fail
		brick = executors.BrickHealStatus{Name: brickNames[2],
			NumberOfEntries: "0"}
		bricks.BrickList = append(bricks.BrickList, brick)
		h := &executors.HealInfo{
			Bricks: bricks,
		}
		return h, nil
	}
	brickId := be.Id()
	err = v.replaceBrickInVolume(app.db, app.executor, brickId)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	oldNode := be.Info.NodeId
	brickOnOldNode := false
	oldBrickIdExists := false

	err = app.db.View(func(tx *bolt.Tx) error {

		for _, brick := range v.Bricks {
			be, err = NewBrickEntryFromId(tx, brick)
			if err != nil {
				return err
			}
			ne, err := NewNodeEntryFromId(tx, be.Info.NodeId)
			if err != nil {
				return err
			}
			if ne.Info.Id == oldNode {
				brickOnOldNode = true
			}
			if be.Info.Id == brickId {
				oldBrickIdExists = true
			}
		}
		return nil
	})

	tests.Assert(t, brickOnOldNode, "brick found on oldNode")
	tests.Assert(t, oldBrickIdExists, "old Brick not deleted")
}

func TestVolumeEntryNoMatchingFlags(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		6,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil)
	// now change the clusters to disable block access
	err = app.db.Update(func(tx *bolt.Tx) error {
		cl, err := ClusterList(tx)
		if err != nil {
			return err
		}
		for _, cid := range cl {
			c, err := NewClusterEntryFromId(tx, cid)
			if err != nil {
				return err
			}
			c.Info.Block = false
			c.Save(tx)
		}
		return nil
	})
	tests.Assert(t, err == nil)

	// Create volume
	v := createSampleReplicaVolumeEntry(1024, 2)
	v.Info.Name = "blockhead"
	// request block volume
	v.Info.Block = true
	err = v.Create(app.db, app.executor)
	// expect error due to no clusters able to satisfy block volume
	tests.Assert(t, err != nil, "expected err != nil")
	_, ok := err.(*MultiClusterError)
	tests.Assert(t, ok, "expected err to be MultiClusterError, got:", err)
}

func TestVolumeEntryMissingFlags(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		6,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil)
	// now change the clusters to disable block and file flags
	err = app.db.Update(func(tx *bolt.Tx) error {
		cl, err := ClusterList(tx)
		if err != nil {
			return err
		}
		for _, cid := range cl {
			c, err := NewClusterEntryFromId(tx, cid)
			if err != nil {
				return err
			}
			c.Info.File = false
			c.Info.Block = false
			c.Save(tx)
		}
		return nil
	})
	tests.Assert(t, err == nil)

	// Create volume
	v := createSampleReplicaVolumeEntry(1024, 2)
	err = v.Create(app.db, app.executor)
	// expect error due to no clusters able to satisfy block volume
	tests.Assert(t, err != nil, "expected err != nil")
	_, ok := err.(*MultiClusterError)
	tests.Assert(t, ok, "expected err to be MultiClusterError, got:", err)
}

func TestVolumeCreateBrickAlloc(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		2*TB, // disksize)
	)

	// create a fairly large volume
	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3
	v := NewVolumeEntryFromRequest(req)

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// check that the volume has exactly 3 bricks
	tests.Assert(t, len(v.Bricks) == 3,
		"expected len(v.Bricks) == 3, got:", len(v.Bricks))

	req = &api.VolumeCreateRequest{}
	req.Size = 1024 * 6
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3
	v2 := NewVolumeEntryFromRequest(req)

	err = v2.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// check that the volume has exactly 12 bricks
	tests.Assert(t, len(v2.Bricks) == 12,
		"expected len(v.Bricks) == 12, got:", len(v2.Bricks))

	// verify that the corrent number of bricks is saved in the DB
	var bc int
	app.db.View(func(tx *bolt.Tx) error {
		bl, err := BrickList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		bc = len(bl)
		return nil
	})
	tests.Assert(t, bc == 15, "expected bc == 15, got:", bc)
}

func TestVolumeCreateConcurrent(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)

	sg := utils.NewStatusGroup()
	vols := [](*VolumeEntry){}
	for i := 0; i < 9; i++ {
		req := &api.VolumeCreateRequest{}
		req.Size = 1024
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		v := NewVolumeEntryFromRequest(req)
		vols = append(vols, v)

		sg.Add(1)
		go func() {
			defer sg.Done()
			err := v.Create(app.db, app.executor)
			sg.Err(err)
		}()
	}

	err = sg.Result()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	for _, v := range vols {
		// check that the volume has exactly 3 bricks
		tests.Assert(t, len(v.Bricks) == 3,
			"expected len(v.Bricks) == 3, got:", len(v.Bricks))
	}

	brickCount := 0
	app.db.View(func(tx *bolt.Tx) error {
		devices, err := DeviceList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(devices) == 24,
			"expected len(devices) == 24, got:", len(devices))
		for _, deviceId := range devices {
			d, err := NewDeviceEntryFromId(tx, deviceId)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			brickCount += len(d.Bricks)
		}
		return nil
	})
	tests.Assert(t, brickCount == 27,
		"expected brickCount == 27, got:", brickCount)
}

func TestVolumeCreateArbiter(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	cc := 0
	mVolumeCreate := app.xo.MockVolumeCreate
	app.xo.MockVolumeCreate = func(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
		cc++
		tests.Assert(t, volume.Arbiter, "expected volumeArbiter=true, was false")
		return mVolumeCreate(host, volume)
	}

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3
	req.GlusterVolumeOptions = []string{"user.heketi.arbiter true"}

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.HasArbiterOption(),
		"expected v.HasArbiterOption() to be true, got false")

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// verify that the mock volume create was called
	tests.Assert(t, cc == 1, "expected cc == 1, got:", cc)

	app.db.View(func(tx *bolt.Tx) error {
		bl, err := BrickList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(bl) == 3,
			"expected len(devices) == 3, got:", len(bl))
		return nil
	})
}
func TestVolumeCreateArbiterSizing(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	cc := 0
	arbiterBrickCount := 0
	mVolumeCreate := app.xo.MockVolumeCreate
	app.xo.MockVolumeCreate = func(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
		cc++
		tests.Assert(t, volume.Arbiter, "expected volumeArbiter=true, was false")
		return mVolumeCreate(host, volume)
	}

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)

	req := &api.VolumeCreateRequest{}
	req.Size = 64
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3
	req.GlusterVolumeOptions = []string{"user.heketi.arbiter true"}

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.HasArbiterOption(),
		"expected v.HasArbiterOption() to be true, got false")

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// verify that the mock volume create was called
	tests.Assert(t, cc == 1, "expected cc == 1, got:", cc)

	app.db.View(func(tx *bolt.Tx) error {
		bl, err := BrickList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(bl) == 3,
			"expected len(devices) == 3, got:", len(bl))
		for _, id := range bl {
			brick, err := NewBrickEntryFromId(tx, id)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			if brick.Info.Size == 1*GB {
				arbiterBrickCount++
			}
		}

		tests.Assert(t, arbiterBrickCount == 1, "expected arbiterBrickCount == 1, got:", arbiterBrickCount)
		return nil
	})
}

func TestVolumeCreateArbiterSizingCustom(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	cc := 0
	arbiterBrickCount := 0
	mVolumeCreate := app.xo.MockVolumeCreate
	app.xo.MockVolumeCreate = func(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
		cc++
		tests.Assert(t, volume.Arbiter, "expected volumeArbiter=true, was false")
		return mVolumeCreate(host, volume)
	}

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)

	req := &api.VolumeCreateRequest{}
	req.Size = 100
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3
	req.GlusterVolumeOptions = []string{"user.heketi.arbiter true", "user.heketi.average-file-size 100"}

	v := NewVolumeEntryFromRequest(req)
	tests.Assert(t, v.HasArbiterOption(),
		"expected v.HasArbiterOption() to be true, got false")

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// verify that the mock volume create was called
	tests.Assert(t, cc == 1, "expected cc == 1, got:", cc)

	app.db.View(func(tx *bolt.Tx) error {
		bl, err := BrickList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(bl) == 3,
			"expected len(devices) == 3, got:", len(bl))
		for _, id := range bl {
			brick, err := NewBrickEntryFromId(tx, id)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			if brick.Info.Size == 1*GB {
				arbiterBrickCount++
			}
		}

		tests.Assert(t, arbiterBrickCount == 1, "expected arbiterBrickCount == 1, got:", arbiterBrickCount)
		return nil
	})
}

func TestVolumeCreateBoundarySizing(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,      // clusters
		4,      // nodes_per_cluster
		8,      // devices_per_node,
		500*GB, // disksize)
	)

	for i := 0; i < 2; i++ {
		req := &api.VolumeCreateRequest{}
		// setting the size to 300 or more fails the test intermittently
		// due to randomization in the device selection
		req.Size = 2500
		req.Snapshot.Enable = true
		req.Snapshot.Factor = 1.5
		req.Durability.Type = api.DurabilityReplicate

		v := NewVolumeEntryFromRequest(req)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = v.Destroy(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// Create a 1TB volume
	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Snapshot.Enable = true
	req.Snapshot.Factor = 1.5
	req.Durability.Type = api.DurabilityReplicate

	err = NewVolumeEntryFromRequest(req).Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestVolumeCreateMultiClusterErrorsNodes(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		3,      // clusters
		0,      // nodes_per_cluster
		0,      // devices_per_node,
		500*GB, // disksize)
	)

	var clusters []string
	app.db.View(func(tx *bolt.Tx) error {
		var err error
		clusters, err = ClusterList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		return nil
	})

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	err = NewVolumeEntryFromRequest(req).Create(app.db, app.executor)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	etext := err.Error()
	tests.Assert(t, strings.Contains(etext, ErrEmptyCluster.Error()),
		"expected strings.Contains(etext, ErrEmptyCluster.Error()), got:",
		etext)
	// verify every cluster id is listed
	for _, cid := range clusters {
		tests.Assert(t, strings.Contains(etext, cid),
			"expected strings.Contains(etext, cid), got:",
			etext)
	}
}

func TestVolumeCreateMultiClusterErrorsDevices(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		3,      // clusters
		3,      // nodes_per_cluster
		0,      // devices_per_node,
		500*GB, // disksize)
	)

	var clusters []string
	app.db.View(func(tx *bolt.Tx) error {
		var err error
		clusters, err = ClusterList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		return nil
	})

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	err = NewVolumeEntryFromRequest(req).Create(app.db, app.executor)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	etext := err.Error()
	tests.Assert(t, strings.Contains(etext, ErrNoStorage.Error()),
		"expected strings.Contains(etext, ErrNoStorage.Error()), got:",
		etext)
	// verify every cluster id is listed
	for _, cid := range clusters {
		tests.Assert(t, strings.Contains(etext, cid),
			"expected strings.Contains(etext, cid), got:",
			etext)
	}
}

func TestVolumeCreateRollbackSpaceReclaimed(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	mockerror := errors.New("MOCK")
	app.xo.MockVolumeCreate = func(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
		return nil, mockerror
	}

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	err = app.db.View(func(tx *bolt.Tx) error {
		devices, e := DeviceList(tx)
		if e != nil {
			return e
		}

		for _, id := range devices {
			device, e := NewDeviceEntryFromId(tx, id)
			if e != nil {
				return e
			}
			tests.Assert(t, device.Info.Storage.Free == 6*TB, id, device)
		}
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc.Exec(app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got", e)

	e = vc.Rollback(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	err = app.db.View(func(tx *bolt.Tx) error {
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))

		devices, e := DeviceList(tx)
		if e != nil {
			return e
		}

		for _, id := range devices {
			device, e := NewDeviceEntryFromId(tx, id)
			if e != nil {
				return e
			}
			tests.Assert(t, device.Info.Storage.Free == 6*TB, id, device)
		}
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got", err)
}

func TestBlockVolumeCreateRollbackSpaceReclaimed(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	mockerror := errors.New("MOCK")
	app.xo.MockBlockVolumeCreate = func(host string, blockVolume *executors.BlockVolumeRequest) (*executors.BlockVolumeInfo, error) {
		return nil, mockerror
	}

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	err = app.db.View(func(tx *bolt.Tx) error {
		devices, e := DeviceList(tx)
		if e != nil {
			return e
		}

		for _, id := range devices {
			device, e := NewDeviceEntryFromId(tx, id)
			if e != nil {
				return e
			}
			tests.Assert(t, device.Info.Storage.Free == 6*TB, id, device)
		}
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got", err)
	bv := createSampleBlockVolumeEntry(500)
	bv.Info.Name = "myvol"
	bc := NewBlockVolumeCreateOperation(bv, app.db)

	e := bc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = bc.Exec(app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got", e)

	e = bc.Rollback(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	err = app.db.View(func(tx *bolt.Tx) error {
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))

		devices, e := DeviceList(tx)
		if e != nil {
			return e
		}

		for _, id := range devices {
			device, e := NewDeviceEntryFromId(tx, id)
			if e != nil {
				return e
			}
			tests.Assert(t, device.Info.Storage.Free == 6*TB, id, device)
		}
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got", err)
}

func TestVolumeEntryBlockCapacityLimits(t *testing.T) {
	var (
		err   error
		v     *VolumeEntry
		mkVol = func() *VolumeEntry {
			v := NewVolumeEntry()
			v.Info.Name = "Foo"
			v.Info.Size = 100
			v.Info.Block = true
			return v
		}
	)

	t.Run("AddCapacityOver", func(t *testing.T) {
		v = mkVol()
		v.SetRawCapacity(100)
		err = v.AddRawCapacity(5)
		tests.Assert(t, err != nil, "expected err != nil, got", err)
	})
	t.Run("AddCapacity", func(t *testing.T) {
		v = mkVol()
		v.SetRawCapacity(95)
		err = v.AddRawCapacity(5)
		tests.Assert(t, err == nil, "expected err == nil, got", err)
	})
	t.Run("SetCapacityOver", func(t *testing.T) {
		v = mkVol()
		err = v.SetRawCapacity(101)
		tests.Assert(t, err != nil, "expected err != nil, got", err)
	})
	t.Run("TakeFreeSpace", func(t *testing.T) {
		v = mkVol()
		v.SetRawCapacity(100)
		err = v.ModifyFreeSize(-10)
		tests.Assert(t, err == nil, "expected err == nil, got", err)
	})
	t.Run("ReturnFreeSpace", func(t *testing.T) {
		v = mkVol()
		v.SetRawCapacity(95)
		err = v.ModifyFreeSize(1)
		tests.Assert(t, err == nil, "expected err == nil, got", err)
	})
	t.Run("TakeTooMuchFreeSpace", func(t *testing.T) {
		v = mkVol()
		v.SetRawCapacity(100)
		err = v.ModifyFreeSize(-1000)
		tests.Assert(t, err != nil, "expected err != nil, got", err)
	})
	t.Run("ReturnTooMuchFreeSpace", func(t *testing.T) {
		v = mkVol()
		v.SetRawCapacity(100)
		err = v.ModifyFreeSize(1)
		tests.Assert(t, err != nil, "expected err != nil, got", err)
	})
	t.Run("TakeFreeSpaceInvalidSize", func(t *testing.T) {
		v = mkVol()
		v.SetRawCapacity(100)
		v.Info.Size = 50
		err = v.ModifyFreeSize(-1)
		tests.Assert(t, err != nil, "expected err != nil, got", err)
	})
	t.Run("TakeTooMuchReservedSpace", func(t *testing.T) {
		v = mkVol()
		v.SetRawCapacity(100)
		err = v.ModifyReservedSize(-200)
		tests.Assert(t, err != nil, "expected err != nil, got", err)
	})
	t.Run("ReturnTooMuchReservedSpace", func(t *testing.T) {
		v = mkVol()
		v.SetRawCapacity(100)
		err = v.ModifyReservedSize(2)
		tests.Assert(t, err != nil, "expected err != nil, got", err)
	})
}

func TestVolumeCreateTooFewZones(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	origZoneChecking := ZoneChecking
	defer func() {
		ZoneChecking = origZoneChecking
	}()

	err := setupSampleDbWithTopologyWithZones(app,
		1,    // clusters
		2,    // zones_per_cluster
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		2*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	t.Run("ZoneChecking none replica 3", func(t *testing.T) {
		// Without strict zone checking, a replica-3 volume fits into
		// two zones.
		ZoneChecking = ZONE_CHECKING_NONE
		req := &api.VolumeCreateRequest{}
		req.Size = 10
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 2
		v := NewVolumeEntryFromRequest(req)

		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("ZoneChecking invalid replica 3", func(t *testing.T) {
		// Specifying an invalid string for ZoneChecking behaves
		// like "none". A replica-3 volume can fit into two zones.
		ZoneChecking = "invalid"
		req := &api.VolumeCreateRequest{}
		req.Size = 10
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		v := NewVolumeEntryFromRequest(req)

		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("ZoneChecking strict replica 3", func(t *testing.T) {
		// With strict zone checking, a replica-3 volume does not fit
		// into two zones.
		ZoneChecking = ZONE_CHECKING_STRICT
		req := &api.VolumeCreateRequest{}
		req.Size = 10
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		v := NewVolumeEntryFromRequest(req)

		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == ErrNoSpace, "expected err == ErrNoSpace, got:", err)
	})

	t.Run("ZoneChecking strict replica 2", func(t *testing.T) {
		// But a replica-2 fits into the two zones.
		ZoneChecking = ZONE_CHECKING_STRICT
		req := &api.VolumeCreateRequest{}
		req.Size = 10
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 2
		v := NewVolumeEntryFromRequest(req)

		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("VolOpt ZoneChecking none replica 3", func(t *testing.T) {
		// the server defaults to strict checking but the volume wants
		// non-strict checking. Volume wins and volume is created w/ two zones
		ZoneChecking = ZONE_CHECKING_STRICT
		req := &api.VolumeCreateRequest{}
		req.Size = 10
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 2
		req.GlusterVolumeOptions = []string{"user.heketi.zone-checking none"}
		v := NewVolumeEntryFromRequest(req)

		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("VolOpt ZoneChecking invalid replica 3", func(t *testing.T) {
		// volume requests an invalid checking value. the server
		// will treat this as equivalent to "none"
		ZoneChecking = ZONE_CHECKING_NONE
		req := &api.VolumeCreateRequest{}
		req.Size = 10
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		req.GlusterVolumeOptions = []string{"user.heketi.zone-checking foobar"}
		v := NewVolumeEntryFromRequest(req)

		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("VolOpt ZoneChecking strict replica 3", func(t *testing.T) {
		// server defaults to none but volume requests strict checking
		// replica-3 volume fails to be placed with only two zones
		ZoneChecking = ZONE_CHECKING_NONE
		req := &api.VolumeCreateRequest{}
		req.Size = 10
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		req.GlusterVolumeOptions = []string{"user.heketi.zone-checking strict"}
		v := NewVolumeEntryFromRequest(req)

		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == ErrNoSpace, "expected err == ErrNoSpace, got:", err)
	})

	t.Run("VolOpt ZoneChecking strict replica 2", func(t *testing.T) {
		// server defaults to none but volume requests strict checking
		// replica-2 volume is placed with only two zones
		ZoneChecking = ZONE_CHECKING_NONE
		req := &api.VolumeCreateRequest{}
		req.Size = 10
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 2
		req.GlusterVolumeOptions = []string{"user.heketi.zone-checking strict"}
		v := NewVolumeEntryFromRequest(req)

		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("VolOpt ZoneChecking overwrite", func(t *testing.T) {
		// server defaults to none but volume requests strict checking
		// after setting none checking (test of option precedence).
		ZoneChecking = ZONE_CHECKING_NONE
		req := &api.VolumeCreateRequest{}
		req.Size = 10
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		req.GlusterVolumeOptions = []string{
			"user.heketi.zone-checking none",
			"user.phony.option 100",
			"user.heketi.zone-checking strict",
		}
		v := NewVolumeEntryFromRequest(req)

		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == ErrNoSpace, "expected err == ErrNoSpace, got:", err)
	})
}

//
// Test the result of brick placement on an unbalanced node/device distribution.
// This is to prove that without strict zone mode, we will end up with
// some bricks in the same zone.
//
func TestVolumeCreateUnbalanced(t *testing.T) {

	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	origZoneChecking := ZoneChecking
	defer func() {
		ZoneChecking = origZoneChecking
	}()

	err := setupSampleDbWithUnbalancedTopology(app, 3, 500*GB)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	t.Run("ZoneChecking strict", func(t *testing.T) {
		ZoneChecking = ZONE_CHECKING_STRICT
		ok := testVolumeCreateUnbalanced(t, app)
		tests.Assert(t, ok)
	})

	// Verify that, without strict zone checking, this topology produces
	// some volumes that have bricksets that don't span three zones.
	t.Run("ZoneChecking none", func(t *testing.T) {
		ZoneChecking = ZONE_CHECKING_NONE
		ok := testVolumeCreateUnbalanced(t, app)
		tests.Assert(t, !ok, "Expected to find volume with too few zones")
	})
}

func testVolumeCreateUnbalanced(t *testing.T, app *App) bool {

	req := &api.VolumeCreateRequest{}
	req.Size = 1
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	for i := 0; i < 100; i++ {
		v := NewVolumeEntryFromRequest(req)
		err := v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		zones := map[int]bool{}
		err = app.db.View(func(tx *bolt.Tx) error {
			for _, brickId := range v.Bricks {
				be, err := NewBrickEntryFromId(tx, brickId)
				if err != nil {
					return err
				}
				ne, err := NewNodeEntryFromId(tx, be.Info.NodeId)
				if err != nil {
					return err
				}

				zones[ne.Info.Zone] = true
			}

			return nil
		})
		tests.Assert(t, err == nil, err)

		// Usually, we would have to check each brick-set of three
		// bricks separately to span three zones. But we can't rely on
		// the order of bricks as they come from the DB. However in our
		// special case of 1 GB volumes, we will only ever have one
		// brick-set. So it is ok, to just check whether we have the
		// correct number of zones across all bricks in the volume.
		if len(zones) != 3 {
			logger.Info("Unbalanced volume created in attempt #%v.", i)
			return false
		}
	}

	return true
}

func TestVolumeExpandStrictZones(t *testing.T) {

	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopologyWithZones(app, 1, 3, 4, 3, 500*GB)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	var singleZoneNode string
	app.db.View(func(tx *bolt.Tx) error {
		zc := map[int]int{}
		zn := map[int]string{}
		nids, err := NodeList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		for _, nid := range nids {
			node, err := NewNodeEntryFromId(tx, nid)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			zc[node.Info.Zone] += 1
			zn[node.Info.Zone] = nid
		}
		tests.Assert(t, len(zc) == 3, "expected 3 zones, got:", len(zc))
		for z, count := range zc {
			if count == 1 {
				singleZoneNode = zn[z]
				break
			}
		}
		return nil
	})
	tests.Assert(t, singleZoneNode != "", "failed to find single zone node")

	req := &api.VolumeCreateRequest{}
	req.Size = 1
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3
	req.GlusterVolumeOptions = []string{"user.heketi.zone-checking strict"}

	v := NewVolumeEntryFromRequest(req)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	vexpand := v.Info.Id

	t.Run("nodeDown", func(t *testing.T) {
		// disable the node leaving only three nodes and two zones
		// online
		var v2x *VolumeEntry
		app.db.Update(func(tx *bolt.Tx) error {
			var err error
			v2x, err = NewVolumeEntryFromId(tx, vexpand)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			node, err := NewNodeEntryFromId(tx, singleZoneNode)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			err = node.SetState(
				wdb.WrapTx(tx),
				app.executor,
				api.EntryStateOffline)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			return nil
		})
		err = v2x.Expand(app.db, app.executor, 50)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	})

	t.Run("nodeUp", func(t *testing.T) {
		// ensure that the single node in a zone is online
		var v2x *VolumeEntry
		app.db.Update(func(tx *bolt.Tx) error {
			var err error
			v2x, err = NewVolumeEntryFromId(tx, vexpand)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			node, err := NewNodeEntryFromId(tx, singleZoneNode)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			err = node.SetState(
				wdb.WrapTx(tx),
				app.executor,
				api.EntryStateOnline)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			return nil
		})
		err = v2x.Expand(app.db, app.executor, 50)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
}

func TestVolumeReplaceBrickZoneChecking(t *testing.T) {
	t.Run("NonStrict", func(t *testing.T) {
		testVolumeReplaceBrickZoneChecking(t,
			[]string{},
			false)
	})
	t.Run("Strict", func(t *testing.T) {
		testVolumeReplaceBrickZoneChecking(t,
			[]string{"user.heketi.zone-checking strict"},
			true)
	})
}

func testVolumeReplaceBrickZoneChecking(
	t *testing.T, volOpts []string, expectFail bool) {

	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopologyWithZones(app, 1, 3, 4, 1, 500*GB)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	var singleZoneNode string
	app.db.View(func(tx *bolt.Tx) error {
		zc := map[int]int{}
		zn := map[int]string{}
		nids, err := NodeList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		for _, nid := range nids {
			node, err := NewNodeEntryFromId(tx, nid)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			zc[node.Info.Zone] += 1
			zn[node.Info.Zone] = nid
		}
		tests.Assert(t, len(zc) == 3, "expected 3 zones, got:", len(zc))
		for z, count := range zc {
			if count == 1 {
				singleZoneNode = zn[z]
				break
			}
		}
		return nil
	})
	tests.Assert(t, singleZoneNode != "", "failed to find single zone node")

	req := &api.VolumeCreateRequest{}
	req.Size = 1
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3
	req.GlusterVolumeOptions = volOpts

	v := NewVolumeEntryFromRequest(req)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	var brickNames []string
	err = app.db.View(func(tx *bolt.Tx) error {
		for _, brick := range v.Bricks {
			be, err := NewBrickEntryFromId(tx, brick)
			if err != nil {
				return err
			}
			ne, err := NewNodeEntryFromId(tx, be.Info.NodeId)
			if err != nil {
				return err
			}
			brickName := fmt.Sprintf("%v:%v",
				ne.Info.Hostnames.Storage[0], be.Info.Path)
			brickNames = append(brickNames, brickName)
		}
		return nil
	})
	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		var bricks []executors.Brick
		for _, b := range brickNames {
			bricks = append(bricks, executors.Brick{Name: b})
		}
		v := &executors.Volume{
			Bricks: executors.Bricks{
				BrickList: bricks,
			},
		}
		return v, nil
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		var bricks executors.HealInfoBricks
		for _, b := range brickNames {
			bricks.BrickList = append(bricks.BrickList,
				executors.BrickHealStatus{Name: b, NumberOfEntries: "0"})
		}
		h := &executors.HealInfo{
			Bricks: bricks,
		}
		return h, nil
	}

	app.db.Update(func(tx *bolt.Tx) error {
		var err error
		node, err := NewNodeEntryFromId(tx, singleZoneNode)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		err = node.SetState(
			wdb.WrapTx(tx),
			app.executor,
			api.EntryStateOffline)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		err = node.SetState(
			wdb.WrapTx(tx),
			app.executor,
			api.EntryStateFailed)
		if expectFail {
			tests.Assert(t, err != nil, "expected err != nil, got:", err)
		} else {
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}
		return nil
	})
}
