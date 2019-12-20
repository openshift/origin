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
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/idgen"
	"github.com/heketi/heketi/pkg/sortedstrings"
	"github.com/heketi/tests"
)

func createSampleDeviceEntry(nodeid string, disksize uint64) *DeviceEntry {

	req := &api.DeviceAddRequest{}
	req.NodeId = nodeid
	req.Name = "/dev/" + idgen.GenUUID()[:8]

	d := NewDeviceEntryFromRequest(req)
	d.StorageSet(disksize, disksize, 0)

	return d
}

func TestNewDeviceEntry(t *testing.T) {

	d := NewDeviceEntry()
	tests.Assert(t, d != nil)
	tests.Assert(t, d.Info.Id == "")
	tests.Assert(t, d.Info.Name == "")
	tests.Assert(t, d.Info.Storage.Free == 0)
	tests.Assert(t, d.Info.Storage.Total == 0)
	tests.Assert(t, d.Info.Storage.Used == 0)
	tests.Assert(t, d.Bricks != nil)
	tests.Assert(t, len(d.Bricks) == 0)

}

func TestNewDeviceEntryFromRequest(t *testing.T) {
	req := &api.DeviceAddRequest{}
	req.NodeId = "123"
	req.Name = "/dev/" + idgen.GenUUID()

	d := NewDeviceEntryFromRequest(req)
	tests.Assert(t, d != nil)
	tests.Assert(t, d.Info.Id != "")
	tests.Assert(t, d.Info.Name == req.Name)
	tests.Assert(t, d.Info.Storage.Free == 0)
	tests.Assert(t, d.Info.Storage.Total == 0)
	tests.Assert(t, d.Info.Storage.Used == 0)
	tests.Assert(t, d.NodeId == "123")
	tests.Assert(t, d.Bricks != nil)
	tests.Assert(t, len(d.Bricks) == 0)

}

func TestNewDeviceEntryMarshal(t *testing.T) {
	req := &api.DeviceAddRequest{}
	req.NodeId = "abc"
	req.Name = "/dev/" + idgen.GenUUID()

	d := NewDeviceEntryFromRequest(req)
	d.Info.Storage.Free = 10
	d.Info.Storage.Total = 100
	d.Info.Storage.Used = 1000
	d.BrickAdd("abc")
	d.BrickAdd("def")

	buffer, err := d.Marshal()
	tests.Assert(t, err == nil)
	tests.Assert(t, buffer != nil)
	tests.Assert(t, len(buffer) > 0)

	um := &DeviceEntry{}
	err = um.Unmarshal(buffer)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(um, d))

}

func TestDeviceEntryNewBrickEntry(t *testing.T) {
	req := &api.DeviceAddRequest{}
	req.NodeId = "abc"
	req.Name = "/dev/" + idgen.GenUUID()

	d := NewDeviceEntryFromRequest(req)
	d.Info.Storage.Free = 900
	d.Info.Storage.Total = 1000
	d.Info.Storage.Used = 100

	// Alignment
	d.ExtentSize = 8

	// Too large
	brick := d.NewBrickEntry(1000000000, 1.5, 1000, "abc")
	tests.Assert(t, brick == nil)

	// --- Now check with a real value ---

	// Check newly created brick
	size := 201
	tpsize := uint64(float32(size) * 1.5)

	// Alignment
	tpsize += d.ExtentSize - (tpsize % d.ExtentSize)

	// Calculate metadatasize
	metadatasize := d.poolMetadataSize(tpsize)

	// Alignment
	metadatasize += d.ExtentSize - (metadatasize % d.ExtentSize)
	total := tpsize + metadatasize

	brick = d.NewBrickEntry(200, 1.5, 1000, "abc")
	tests.Assert(t, brick != nil)
	tests.Assert(t, brick.TpSize == tpsize)
	tests.Assert(t, brick.PoolMetadataSize == metadatasize, brick.PoolMetadataSize, metadatasize)
	tests.Assert(t, brick.Info.Size == 200)
	tests.Assert(t, brick.gidRequested == 1000)
	tests.Assert(t, brick.Info.VolumeId == "abc")

	// Check it was subtracted from device storage
	tests.Assert(t, d.Info.Storage.Used == 100+total)
	tests.Assert(t, d.Info.Storage.Free == 900-total)
	tests.Assert(t, d.Info.Storage.Total == 1000)
}

func TestDeviceEntryAddDeleteBricks(t *testing.T) {
	d := NewDeviceEntry()
	tests.Assert(t, len(d.Bricks) == 0)

	d.BrickAdd("123")
	tests.Assert(t, sortedstrings.Has(d.Bricks, "123"))
	tests.Assert(t, len(d.Bricks) == 1)
	d.BrickAdd("abc")
	tests.Assert(t, sortedstrings.Has(d.Bricks, "123"))
	tests.Assert(t, sortedstrings.Has(d.Bricks, "abc"))
	tests.Assert(t, len(d.Bricks) == 2)

	d.BrickDelete("123")
	tests.Assert(t, !sortedstrings.Has(d.Bricks, "123"))
	tests.Assert(t, sortedstrings.Has(d.Bricks, "abc"))
	tests.Assert(t, len(d.Bricks) == 1)

	d.BrickDelete("ccc")
	tests.Assert(t, !sortedstrings.Has(d.Bricks, "123"))
	tests.Assert(t, sortedstrings.Has(d.Bricks, "abc"))
	tests.Assert(t, len(d.Bricks) == 1)
}

func TestNewDeviceEntryFromIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Test for ID not found
	err := app.db.View(func(tx *bolt.Tx) error {
		_, err := NewDeviceEntryFromId(tx, "123")
		return err
	})
	tests.Assert(t, err == ErrNotFound)

}

func TestNewDeviceEntryFromId(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a device
	req := &api.DeviceAddRequest{}
	req.NodeId = "abc"
	req.Name = "/dev/" + idgen.GenUUID()

	d := NewDeviceEntryFromRequest(req)
	d.Info.Storage.Free = 10
	d.Info.Storage.Total = 100
	d.Info.Storage.Used = 1000
	d.BrickAdd("abc")
	d.BrickAdd("def")

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return d.Save(tx)
	})
	tests.Assert(t, err == nil)

	var device *DeviceEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		device, err = NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}
		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(device, d))
}

func TestNewDeviceEntrySaveDelete(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a device
	req := &api.DeviceAddRequest{}
	req.NodeId = "abc"
	req.Name = "/dev/" + idgen.GenUUID()

	d := NewDeviceEntryFromRequest(req)
	d.Info.Storage.Free = 10
	d.Info.Storage.Total = 100
	d.Info.Storage.Used = 1000
	d.BrickAdd("abc")
	d.BrickAdd("def")

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return d.Save(tx)
	})
	tests.Assert(t, err == nil)

	var device *DeviceEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		device, err = NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}
		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(device, d))

	// Delete device which has bricks
	err = app.db.Update(func(tx *bolt.Tx) error {
		var err error
		device, err = NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}

		err = device.Delete(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, strings.Contains(err.Error(), "is not in failed state"), err)

	// Delete bricks in device
	device.BrickDelete("abc")
	device.BrickDelete("def")
	tests.Assert(t, len(device.Bricks) == 0)
	err = app.db.Update(func(tx *bolt.Tx) error {
		return device.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Set offline
	err = device.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, device.State == api.EntryStateOffline)
	tests.Assert(t, err == nil, err)

	// Set failed
	err = device.SetState(app.db, app.executor, api.EntryStateFailed)
	tests.Assert(t, device.State == api.EntryStateFailed, device.State)
	tests.Assert(t, err == nil, err)

	// Now try to delete the device
	err = app.db.Update(func(tx *bolt.Tx) error {
		var err error
		device, err = NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}

		err = device.Delete(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Check device has been deleted and is not in db
	err = app.db.View(func(tx *bolt.Tx) error {
		var err error
		device, err = NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}
		return nil

	})
	tests.Assert(t, err == ErrNotFound)
}

func TestNewDeviceEntryNewInfoResponseBadBrickIds(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a device
	req := &api.DeviceAddRequest{}
	req.NodeId = "abc"
	req.Name = "/dev/" + idgen.GenUUID()

	// Add bad brick ids
	d := NewDeviceEntryFromRequest(req)
	d.Info.Storage.Free = 10
	d.Info.Storage.Total = 100
	d.Info.Storage.Used = 1000
	d.BrickAdd("abc")
	d.BrickAdd("def")

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		return d.Save(tx)
	})
	tests.Assert(t, err == nil)

	err = app.db.View(func(tx *bolt.Tx) error {
		device, err := NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}

		_, err = device.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == ErrNotFound)
}

func TestNewDeviceEntryNewInfoResponse(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create a device
	req := &api.DeviceAddRequest{}
	req.NodeId = "abc"
	req.Name = "/dev/" + idgen.GenUUID()

	d := NewDeviceEntryFromRequest(req)
	d.Info.Storage.Free = 10
	d.Info.Storage.Total = 100
	d.Info.Storage.Used = 1000

	// Create a brick
	b := &BrickEntry{}
	b.Info.Id = "bbb"
	b.Info.Size = 10
	b.Info.NodeId = "abc"
	b.Info.DeviceId = d.Info.Id
	b.Info.Path = "/somepath"

	// Add brick to device
	d.BrickAdd("bbb")

	// Save element in database
	err := app.db.Update(func(tx *bolt.Tx) error {
		err := d.Save(tx)
		if err != nil {
			return err
		}

		return b.Save(tx)
	})
	tests.Assert(t, err == nil)

	var info *api.DeviceInfoResponse
	err = app.db.View(func(tx *bolt.Tx) error {
		device, err := NewDeviceEntryFromId(tx, d.Info.Id)
		if err != nil {
			return err
		}

		info, err = device.NewInfoResponse(tx)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)
	tests.Assert(t, info.Id == d.Info.Id)
	tests.Assert(t, info.Name == d.Info.Name)
	tests.Assert(t, reflect.DeepEqual(info.Storage, d.Info.Storage))
	tests.Assert(t, len(info.Bricks) == 1)
	tests.Assert(t, info.Bricks[0].Id == "bbb")
	tests.Assert(t, info.Bricks[0].Path == "/somepath")
	tests.Assert(t, info.Bricks[0].NodeId == "abc")
	tests.Assert(t, info.Bricks[0].DeviceId == d.Info.Id)
	tests.Assert(t, info.Bricks[0].Size == 10)

}

func TestDeviceEntryStorage(t *testing.T) {
	d := NewDeviceEntry()

	tests.Assert(t, d.Info.Storage.Free == 0)
	tests.Assert(t, d.Info.Storage.Total == 0)
	tests.Assert(t, d.Info.Storage.Used == 0)

	d.StorageSet(1000, 1000, 0)
	tests.Assert(t, d.Info.Storage.Free == 1000)
	tests.Assert(t, d.Info.Storage.Total == 1000)
	tests.Assert(t, d.Info.Storage.Used == 0)

	d.StorageSet(2000, 2000, 0)
	tests.Assert(t, d.Info.Storage.Free == 2000)
	tests.Assert(t, d.Info.Storage.Total == 2000)
	tests.Assert(t, d.Info.Storage.Used == 0)

	d.StorageAllocate(1000)
	tests.Assert(t, d.Info.Storage.Free == 1000)
	tests.Assert(t, d.Info.Storage.Total == 2000)
	tests.Assert(t, d.Info.Storage.Used == 1000)

	d.StorageAllocate(500)
	tests.Assert(t, d.Info.Storage.Free == 500)
	tests.Assert(t, d.Info.Storage.Total == 2000)
	tests.Assert(t, d.Info.Storage.Used == 1500)

	d.StorageFree(500)
	tests.Assert(t, d.Info.Storage.Free == 1000)
	tests.Assert(t, d.Info.Storage.Total == 2000)
	tests.Assert(t, d.Info.Storage.Used == 1000)

	d.StorageFree(1000)
	tests.Assert(t, d.Info.Storage.Free == 2000)
	tests.Assert(t, d.Info.Storage.Total == 2000)
	tests.Assert(t, d.Info.Storage.Used == 0)
}

func TestDeviceSetStateFailed(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create cluster entry
	c := NewClusterEntry()
	c.Info.Id = "cluster"

	// Create a node
	n := NewNodeEntry()
	tests.Assert(t, n != nil)
	tests.Assert(t, n.State == api.EntryStateOnline)

	// Initialize node
	n.Info.Id = "node"
	n.Info.ClusterId = c.Info.Id

	c.NodeAdd(n.Info.Id)

	// Create device entry
	d := NewDeviceEntry()
	d.Info.Id = "d1"
	d.Info.Name = "/d1"
	d.NodeId = n.Info.Id

	n.DeviceAdd(d.Info.Id)

	// Save in db
	app.db.Update(func(tx *bolt.Tx) error {
		err := c.Save(tx)
		tests.Assert(t, err == nil)

		err = n.Save(tx)
		tests.Assert(t, err == nil)

		err = d.Save(tx)
		tests.Assert(t, err == nil)

		return nil
	})

	// Set offline
	err := d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, d.State == api.EntryStateOffline)
	tests.Assert(t, err == nil, err)

	// Set failed, Note: this requires the current state to be offline
	err = d.SetState(app.db, app.executor, api.EntryStateFailed)
	tests.Assert(t, d.State == api.EntryStateFailed)
	tests.Assert(t, err == nil)

	// Set failed again
	err = d.SetState(app.db, app.executor, api.EntryStateFailed)
	tests.Assert(t, d.State == api.EntryStateFailed)
	tests.Assert(t, err == nil)

	// Set online from failed, this should fail
	err = d.SetState(app.db, app.executor, api.EntryStateOnline)
	tests.Assert(t, d.State == api.EntryStateFailed)
	tests.Assert(t, err != nil)

	// Set offline
	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, d.State == api.EntryStateOffline)
	tests.Assert(t, err == nil)

	// Set online from offline, this should pass
	err = d.SetState(app.db, app.executor, api.EntryStateOnline)
	tests.Assert(t, d.State == api.EntryStateOnline)
	tests.Assert(t, err == nil)
}

func TestDeviceSetStateOfflineOnline(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	// Create cluster entry
	c := NewClusterEntry()
	c.Info.Id = "cluster"

	// Create a node
	n := NewNodeEntry()
	tests.Assert(t, n != nil)
	tests.Assert(t, n.State == api.EntryStateOnline)

	// Initialize node
	n.Info.Id = "node"
	n.Info.ClusterId = c.Info.Id

	c.NodeAdd(n.Info.Id)

	// Create device entry
	d := NewDeviceEntry()
	d.Info.Id = "d1"
	d.Info.Name = "/d1"
	d.NodeId = n.Info.Id

	n.DeviceAdd(d.Info.Id)

	// Save in db
	app.db.Update(func(tx *bolt.Tx) error {
		err := c.Save(tx)
		tests.Assert(t, err == nil)

		err = n.Save(tx)
		tests.Assert(t, err == nil)

		err = d.Save(tx)
		tests.Assert(t, err == nil)

		return nil
	})

	// Set offline
	err := d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, d.State == api.EntryStateOffline)
	tests.Assert(t, err == nil)

	// Set offline again
	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, d.State == api.EntryStateOffline)
	tests.Assert(t, err == nil)

	// Set online
	err = d.SetState(app.db, app.executor, api.EntryStateOnline)
	tests.Assert(t, d.State == api.EntryStateOnline)
	tests.Assert(t, err == nil)
}

func TestDeviceSetStateFailedWithBricks(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		5*TB, // disksize
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create a few volumes
	for i := 0; i < 5; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a device that has bricks
	var d *DeviceEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		dl, err := DeviceList(tx)
		if err != nil {
			return err
		}
		for _, id := range dl {
			d, err = NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			if len(d.Bricks) > 0 {
				return nil
			}
		}
		t.Fatalf("should have at least one device with bricks")
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.State == api.EntryStateOffline)

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.State == api.EntryStateOffline)
	tests.Assert(t, len(d.Bricks) > 0,
		"expected len(d.Bricks) > 0, got:", len(d.Bricks))

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	err = d.SetState(app.db, app.executor, api.EntryStateFailed)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.State == api.EntryStateFailed)
	tests.Assert(t, len(d.Bricks) == 0,
		"expected len(d.Bricks) == 0, got:", len(d.Bricks))
}

func TestDeviceSetStateFailedTooFewDevices(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		5*TB, // disksize
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create a few volumes
	for i := 0; i < 5; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a device that has bricks
	var d *DeviceEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		dl, err := DeviceList(tx)
		if err != nil {
			return err
		}
		for _, id := range dl {
			d, err = NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			if len(d.Bricks) > 0 {
				return nil
			}
		}
		t.Fatalf("should have at least one device with bricks")
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.State == api.EntryStateOffline)

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.State == api.EntryStateOffline)
	tests.Assert(t, len(d.Bricks) > 0,
		"expected len(d.Bricks) > 0, got:", len(d.Bricks))

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	err = d.SetState(app.db, app.executor, api.EntryStateFailed)
	tests.Assert(t, strings.Contains(err.Error(), ErrNoReplacement.Error()),
		"expected strings.Contains(err.Error(), ErrNoReplacement.Error()), got:",
		err.Error())
}

func mockVolumeInfoFromDb(db *bolt.DB, volume string) (*executors.Volume, error) {
	volume = volume[4:]
	vi := &executors.Volume{}
	db.View(func(tx *bolt.Tx) error {
		bl, _ := BrickList(tx)
		for _, id := range bl {
			b, err := NewBrickEntryFromId(tx, id)
			if err != err {
				return err
			}
			if b.Info.VolumeId != volume {
				continue
			}
			n, err := NewNodeEntryFromId(tx, b.Info.NodeId)
			if err != err {
				return err
			}
			vi.Bricks.BrickList = append(vi.Bricks.BrickList, executors.Brick{
				Name: fmt.Sprintf("%v:%v", n.Info.Hostnames.Storage[0], b.Info.Path),
			})
		}
		return nil
	})
	return vi, nil
}

func mockHealStatusFromDb(db *bolt.DB, volume string) (*executors.HealInfo, error) {
	hi := &executors.HealInfo{}
	volume = volume[4:]
	db.View(func(tx *bolt.Tx) error {
		bl, _ := BrickList(tx)
		for _, id := range bl {
			b, err := NewBrickEntryFromId(tx, id)
			if err != err {
				return err
			}
			if b.Info.VolumeId != volume {
				continue
			}
			n, err := NewNodeEntryFromId(tx, b.Info.NodeId)
			if err != err {
				return err
			}
			hi.Bricks.BrickList = append(hi.Bricks.BrickList, executors.BrickHealStatus{
				Name:            fmt.Sprintf("%v:%v", n.Info.Hostnames.Storage[0], b.Info.Path),
				NumberOfEntries: "0",
			})
		}
		return nil
	})
	return hi, nil
}

func TestDeviceSetStateFailedWithEmptyPathBricks(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		5*TB, // disksize
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create a few volumes
	for i := 0; i < 5; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a device that has bricks
	// and a brick to create copy of it
	var d *DeviceEntry
	var newbrick *BrickEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		dl, err := DeviceList(tx)
		if err != nil {
			return err
		}
		for _, id := range dl {
			d, err = NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			if len(d.Bricks) > 0 {
				return nil
			}
		}
		t.Fatalf("should have at least one device with bricks")
		return nil
	})

	// create a brick in a device
	// make the path empty
	// save device and brick to db
	newbrick = d.NewBrickEntry(102400, 1, 2000, idgen.GenUUID())
	newbrick.Info.Path = ""
	d.BrickAdd(newbrick.Id())
	err = app.db.Update(func(tx *bolt.Tx) error {
		err = d.Save(tx)
		tests.Assert(t, err == nil)
		return newbrick.Save(tx)
	})
	tests.Assert(t, err == nil)

	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.State == api.EntryStateOffline)

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.State == api.EntryStateOffline)
	tests.Assert(t, len(d.Bricks) > 0,
		"expected len(d.Bricks) > 0, got:", len(d.Bricks))

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	// Fail the device, it should go through
	// however, we would skipped on brick and that should remain in the device
	err = d.SetState(app.db, app.executor, api.EntryStateFailed)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.State == api.EntryStateFailed)
	tests.Assert(t, len(d.Bricks) == 1,
		"expected len(d.Bricks) == 1, got:", len(d.Bricks))
}

// See also RHBZ#1572661
func TestDeviceRemoveSizeAccounting(t *testing.T) {
	t.Run("standard", func(t *testing.T) {
		testDeviceRemoveSizeAccounting(t, false)
	})
	t.Run("arbiter", func(t *testing.T) {
		testDeviceRemoveSizeAccounting(t, true)
	})
}

func testDeviceRemoveSizeAccounting(t *testing.T, useArbiter bool) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		5*TB, // disksize
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	if useArbiter {
		vreq.GlusterVolumeOptions = []string{"user.heketi.arbiter true"}
	}
	v := NewVolumeEntryFromRequest(vreq)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// grab a device that has bricks
	var d *DeviceEntry
	vols := []string{}
	err = app.db.View(func(tx *bolt.Tx) error {
		dl, err := DeviceList(tx)
		if err != nil {
			return err
		}
		for _, id := range dl {
			d, err = NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			if len(d.Bricks) > 0 {
				for _, b := range d.Bricks {
					be, err := NewBrickEntryFromId(tx, b)
					tests.Assert(t, err == nil, "expected err == nil, got:", err)
					vols = append(vols, be.Info.VolumeId)
				}
				return nil
			}
		}
		t.Fatalf("should have at least one device with bricks")
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.State == api.EntryStateOffline)

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.State == api.EntryStateOffline)
	tests.Assert(t, len(d.Bricks) > 0,
		"expected len(d.Bricks) > 0, got:", len(d.Bricks))

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	err = d.SetState(app.db, app.executor, api.EntryStateFailed)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.State == api.EntryStateFailed)
	tests.Assert(t, len(d.Bricks) == 0,
		"expected len(d.Bricks) == 0, got:", len(d.Bricks))

	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.EntryStateOnline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.State == api.EntryStateOnline)
	tests.Assert(t, d.Info.Storage.Used == 0,
		"expected d.Info.Storage.Used == 0, got:", d.Info.Storage.Used)

	app.db.View(func(tx *bolt.Tx) error {
		for _, vid := range vols {
			v, err := NewVolumeEntryFromId(tx, vid)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			for _, brickId := range v.Bricks {
				brick, err := NewBrickEntryFromId(tx, brickId)
				tests.Assert(t, err == nil, "expected err == nil, got:", err)
				device, err := NewDeviceEntryFromId(tx, brick.Info.DeviceId)
				tests.Assert(t, err == nil, "expected err == nil, got:", err)
				bsize := brick.TpSize + brick.PoolMetadataSize
				tests.Assert(t, device.Info.Storage.Used == bsize,
					"expected device.Info.Storage.Used == bsize, got:",
					device.Info.Storage.Used,
					bsize)
			}
		}
		return nil
	})
}

func TestDbEntryLoadUnmarshalError(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app for a db
	app := NewTestApp(tmpfile)
	defer app.Close()

	// precreate a dummy cluster
	c := NewClusterEntry()
	c.Info.Id = idgen.GenUUID()
	c.Info.Block = true
	c.Info.File = true
	app.db.Update(func(tx *bolt.Tx) error {
		err := c.Save(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		return nil
	})

	// works in the normal case
	app.db.View(func(tx *bolt.Tx) error {
		err := EntryLoad(tx, c, c.Info.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		return nil
	})

	// mess up the value
	app.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(c.BucketName()))
		tests.Assert(t, b != nil, "expected b != nil")
		err := b.Put([]byte(c.Info.Id), []byte("bob"))
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		return nil
	})

	// fails with a "malformed" value
	app.db.View(func(tx *bolt.Tx) error {
		err := EntryLoad(tx, c, c.Info.Id)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		// assert that the error indicates the bucket and key that failed
		tests.Assert(t, strings.Contains(err.Error(), c.BucketName()),
			"expected", c.BucketName(), "in", err)
		tests.Assert(t, strings.Contains(err.Error(), c.Info.Id),
			"expected", c.Info.Id, "in", err)
		return nil
	})
}
