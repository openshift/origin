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
	"os"
	"sort"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"

	"github.com/heketi/heketi/pkg/glusterfs/api"
)

func TestClusterDeviceSource(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		4,    // nodes_per_cluster
		4,    // devices_per_node,
		1*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// this function will both verify that the ClusterDeviceSource
	// meets the DeviceSource interface (at compile time). When
	// using the return value it also makes sure we only use the
	// functions that are part of the interface.
	interfaceCheck := func(dsrc DeviceSource) DeviceSource {
		return dsrc
	}

	app.db.View(func(tx *bolt.Tx) error {
		cids, err := ClusterList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(cids) == 2,
			"expected len(cids) == 2, got:", len(cids))

		dsrc := NewClusterDeviceSource(tx, cids[0])
		dsrci := interfaceCheck(dsrc)
		// test that it pulls all devices in the cluster
		dnl, err := dsrci.Devices()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(dnl) == 16,
			"expected len(dnl) == 16, got:", len(dnl))
		// test that it can lookup a device
		d, err := dsrci.Device(dnl[0].Device.Info.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, d.Info.Id == dnl[0].Device.Info.Id,
			"expected d.Info.Id == dnl[0].Device.Info.Id, got:",
			d.Info.Id, "vs", dnl[0].Device.Info.Id)
		return nil
	})
}

func TestClusterDeviceSourcePartial(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		4,    // nodes_per_cluster
		4,    // devices_per_node,
		1*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// mark a device and a node as offline
	var clusterId string
	app.db.Update(func(tx *bolt.Tx) error {
		cids, err := ClusterList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(cids) == 2,
			"expected len(cids) == 2, got:", len(cids))

		clusterId = cids[0]
		c, err := NewClusterEntryFromId(tx, clusterId)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// set the first node offline
		n, err := NewNodeEntryFromId(tx, c.Info.Nodes[0])
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		n.State = api.EntryStateOffline
		err = n.Save(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// set the 1st device of the 2nd node offline
		n, err = NewNodeEntryFromId(tx, c.Info.Nodes[1])
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		d, err := NewDeviceEntryFromId(tx, n.Devices[0])
		d.State = api.EntryStateOffline
		err = d.Save(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		return nil
	})

	app.db.View(func(tx *bolt.Tx) error {
		dsrc := NewClusterDeviceSource(tx, clusterId)
		// test that it pulls all devices in the cluster
		devices, err := dsrc.Devices()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(devices) == 11,
			"expected len(devices) == 11, got:", len(devices))
		return nil
	})
}

func TestClusterDeviceSourceLookupEmptyCache(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		4,    // nodes_per_cluster
		4,    // devices_per_node,
		1*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		cids, err := ClusterList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(cids) == 1,
			"expected len(cids) == 1, got:", len(cids))

		dsrc := NewClusterDeviceSource(tx, cids[0])
		tests.Assert(t, len(dsrc.deviceCache) == 0,
			"expected len(dsrc.deviceCache) == 0, got:", len(dsrc.deviceCache))

		dids, err := DeviceList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		for _, deviceId := range dids {
			d, err := dsrc.Device(deviceId)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, d.Info.Id == deviceId,
				"expected d.Info.Id == deviceId, got:",
				d.Info.Id, "vs", deviceId)
		}
		tests.Assert(t, len(dsrc.deviceCache) == 16,
			"expected len(dsrc.deviceCache) == 16, got:", len(dsrc.deviceCache))
		return nil
	})
}

func TestVolumePlacementOpts(t *testing.T) {
	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	s := uint64(req.Size) * GB
	vol := NewVolumeEntryFromRequest(req)
	gen := vol.Durability.BrickSizeGenerator(s)
	numSets, brickSize, err := gen()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// this function will both verify that the VolumePlacementOpts
	// meets the PlacementOpts interface (at compile time). When
	// using the return value it also makes sure we only use the
	// functions that are part of the interface.
	interfaceCheck := func(p PlacementOpts) PlacementOpts {
		return p
	}

	p := interfaceCheck(NewVolumePlacementOpts(
		vol,
		brickSize,
		numSets))
	tests.Assert(t, p.BrickOwner() == vol.Info.Id,
		"expected p.BrickOwner() == vol.Info.Id, got:",
		p.BrickOwner(), vol.Info.Id)
}

func TestClusterDeviceSourceAlloc(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		4,    // nodes_per_cluster
		4,    // devices_per_node,
		1*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		cids, err := ClusterList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(cids) == 2,
			"expected len(cids) == 2, got:", len(cids))
		cluster := cids[0]

		a := NewSimpleAllocator()
		dsrc := NewClusterDeviceSource(tx, cluster)
		deviceCh, done, err := a.GetNodesFromDeviceSource(dsrc, "0000000")
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		defer close(done)

		dtest2 := []string{}
		for id := range deviceCh {
			dtest2 = append(dtest2, id)
		}
		tests.Assert(t, len(dtest2) == 16,
			"expected len(dtest2) == 16, got:", len(dtest2))

		dtest := []string{}
		dl, err := DeviceList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		for _, id := range dl {
			d, err := NewDeviceEntryFromId(tx, id)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			n, err := NewNodeEntryFromId(tx, d.NodeId)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			if n.Info.ClusterId == cluster {
				dtest = append(dtest, id)
			}
		}
		tests.Assert(t, len(dtest) == 16,
			"expected len(dtest) == 16, got:", len(dtest))

		sort.Strings(dtest)
		sort.Strings(dtest2)
		for i := range dtest {
			tests.Assert(t, dtest[i] == dtest2[i],
				"expected dtest[i] == dtest2[i], got",
				dtest[i], dtest2[i], "@", i)
		}

		return nil
	})
}
