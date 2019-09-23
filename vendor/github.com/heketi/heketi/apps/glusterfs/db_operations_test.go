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
	"testing"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/idgen"
	"github.com/heketi/tests"
)

func TestDeleteBricksWithEmptyPath(t *testing.T) {
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

	var nodeEntry *NodeEntry
	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create a few volumes
	for i := 0; i < 15; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a device that has bricks
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

	// At this point, we have 15 legal bricks on each device and we have
	// made note of a device(node and cluster in corollary) where we will
	// create 25 bricks with empty path.

	// CASE1: use "all" bool to delete empty path bricks from all clusters
	// ====================================================================
	// create bricks in device and set the path empty
	// save device and brick to db
	for i := 0; i < 25; i++ {
		newbrick = d.NewBrickEntry(102400, 1, 2000, idgen.GenUUID())
		newbrick.Info.Path = ""
		d.BrickAdd(newbrick.Id())
		err = app.db.Update(func(tx *bolt.Tx) error {
			err = d.Save(tx)
			tests.Assert(t, err == nil)
			return newbrick.Save(tx)
		})
		tests.Assert(t, err == nil)
	}
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 40,
		"expected len(d.Bricks) == 40, got:", len(d.Bricks))

	// now delete bricks with empty path
	err = DeleteBricksWithEmptyPath(app.db, true, []string{}, []string{}, []string{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 15,
		"expected len(d.Bricks) == 15, got:", len(d.Bricks))

	// CASE2: delete empty path bricks from this device
	// ====================================================================
	// create bricks in device and set the path empty
	// save device and brick to db
	for i := 0; i < 25; i++ {
		newbrick = d.NewBrickEntry(102400, 1, 2000, idgen.GenUUID())
		newbrick.Info.Path = ""
		d.BrickAdd(newbrick.Id())
		err = app.db.Update(func(tx *bolt.Tx) error {
			err = d.Save(tx)
			tests.Assert(t, err == nil)
			return newbrick.Save(tx)
		})
		tests.Assert(t, err == nil)
	}
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 40,
		"expected len(d.Bricks) == 40, got:", len(d.Bricks))

	err = DeleteBricksWithEmptyPath(app.db, false, []string{}, []string{}, []string{d.Info.Id})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 15,
		"expected len(d.Bricks) == 15, got:", len(d.Bricks))

	// CASE3: delete empty path bricks from a node
	// ====================================================================
	// create bricks in device and set the path empty
	// save device and brick to db
	for i := 0; i < 25; i++ {
		newbrick = d.NewBrickEntry(102400, 1, 2000, idgen.GenUUID())
		newbrick.Info.Path = ""
		d.BrickAdd(newbrick.Id())
		err = app.db.Update(func(tx *bolt.Tx) error {
			err = d.Save(tx)
			tests.Assert(t, err == nil)
			return newbrick.Save(tx)
		})
		tests.Assert(t, err == nil)
	}
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 40,
		"expected len(d.Bricks) == 40, got:", len(d.Bricks))

	err = DeleteBricksWithEmptyPath(app.db, false, []string{}, []string{d.NodeId, d.NodeId}, []string{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 15,
		"expected len(d.Bricks) == 15, got:", len(d.Bricks))

	// CASE4: delete empty path bricks from a cluster
	// ====================================================================
	// create bricks in device and set the path empty
	// save device and brick to db
	for i := 0; i < 25; i++ {
		newbrick = d.NewBrickEntry(102400, 1, 2000, idgen.GenUUID())
		newbrick.Info.Path = ""
		d.BrickAdd(newbrick.Id())
		err = app.db.Update(func(tx *bolt.Tx) error {
			err = d.Save(tx)
			tests.Assert(t, err == nil)
			return newbrick.Save(tx)
		})
		tests.Assert(t, err == nil)
	}
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 40,
		"expected len(d.Bricks) == 40, got:", len(d.Bricks))

	err = app.db.View(func(tx *bolt.Tx) error {
		nodeEntry, err = NewNodeEntryFromId(tx, d.NodeId)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = DeleteBricksWithEmptyPath(app.db, false, []string{nodeEntry.Info.ClusterId}, []string{d.NodeId}, []string{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 15,
		"expected len(d.Bricks) == 15, got:", len(d.Bricks))
}

func TestDeletePendingEntriesVolumeCreate(t *testing.T) {
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
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)

	// verify that there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	// Create first volume fully
	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// only build the second volume, this will leave pending entries
	vol2 := NewVolumeEntryFromRequest(req)
	vc2 := NewVolumeCreateOperation(vol2, app.db)
	e = vc2.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify volumes, bricks, & pending ops exist
	app.db.View(func(tx *bolt.Tx) error {
		var pendingVols int
		var pendingBricks int

		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 2, "expected len(vl) == 2, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 6, "expected len(bl) == 6, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 1, "expected len(pol) == 1, got", len(pol))

		for _, vid := range vl {
			v, e := NewVolumeEntryFromId(tx, vid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			if v.Pending.Id != "" {
				pendingVols++
			}
		}
		tests.Assert(t, pendingVols == 1, "expected pendingVols == 1, got:", pendingVols)
		for _, bid := range bl {
			b, e := NewBrickEntryFromId(tx, bid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			if b.Pending.Id != "" {
				pendingBricks++
			}
		}
		tests.Assert(t, pendingBricks == 3, "expected pendingBricks == 3, got:", pendingBricks)
		return nil
	})

	err = DeletePendingEntries(app.db, false, false)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	// verify that completed volume & bricks exist but incomplete volume and bricks are gone
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))

		for _, vid := range vl {
			v, e := NewVolumeEntryFromId(tx, vid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Pending.Id == "",
				`expected v.Pending.Id == "", got:`, v.Pending.Id)
		}
		for _, bid := range bl {
			b, e := NewBrickEntryFromId(tx, bid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, b.Pending.Id == "",
				`expected b.Pending.Id == "", got:`, b.Pending.Id)
		}
		return nil
	})
}

func TestDeletePendingEntriesVolumeDelete(t *testing.T) {
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
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	// first we need to create a volume to delete
	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})

	vd := NewVolumeDeleteOperation(vol, app.db)
	e = vd.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))
		return nil
	})
	err = DeletePendingEntries(app.db, false, false)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got:", len(bl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})
}

func TestDeletePendingEntriesVolumeExpandOperation(t *testing.T) {
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
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})

	ve := NewVolumeExpandOperation(vol, app.db, 100)
	e = ve.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 6, "expected len(bl) == 6, got:", len(bl))
		pcount := 0
		for _, id := range bl {
			b, e := NewBrickEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			if b.Pending.Id != "" {
				pcount++
			}
		}
		tests.Assert(t, pcount == 3, "expected len(bl) == 3, got:", pcount)
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))
		return nil
	})

	// expand is a special case, nothing should be deleted
	err = DeletePendingEntries(app.db, false, false)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 6, "expected len(bl) == 6, got:", len(bl))
		pcount := 0
		for _, id := range bl {
			b, e := NewBrickEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			if b.Pending.Id != "" {
				pcount++
			}
		}
		tests.Assert(t, pcount == 3, "expected len(bl) == 3, got:", pcount)
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))
		return nil
	})
}

func TestDeletePendingEntriesBlockVolumeCreate(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		2*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 1024

	vol := NewBlockVolumeEntryFromRequest(req)
	vc := NewBlockVolumeCreateOperation(vol, app.db)

	// verify that there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify there is one pending op, volume and some bricks
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 1, "expected len(pol) == 1, got", len(pol))
		return nil
	})

	err = DeletePendingEntries(app.db, false, false)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})
}

func TestDeletePendingEntriesBlockVolumeDelete(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		2*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 1024

	vol := NewBlockVolumeEntryFromRequest(req)
	vc := NewBlockVolumeCreateOperation(vol, app.db)

	// verify that there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify the volume and bricks exist but no pending op
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(bvl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	bdel := NewBlockVolumeDeleteOperation(vol, app.db)

	e = bdel.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// we should now have a pending op for the delete
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(bvl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 1, "expected len(pol) == 1, got", len(pol))
		return nil
	})

	// clean
	err = DeletePendingEntries(app.db, false, false)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	// the block volume and pending op should be gone. hosting volume stays
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 0, "expected len(bvl) == 0, got", len(bvl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})
}

func TestDeletePendingEntriesManyEntries(t *testing.T) {
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
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// Create 10 complete volumes and 15 pending volumes
	req := &api.VolumeCreateRequest{}
	req.Size = 10
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3
	req.Block = true

	for i := 0; i < 10; i++ {
		v := NewVolumeEntryFromRequest(req)
		vc := NewVolumeCreateOperation(v, app.db)
		e := vc.Build()
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		e = vc.Exec(app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		e = vc.Finalize()
		tests.Assert(t, e == nil, "expected e == nil, got", e)
	}
	for i := 0; i < 15; i++ {
		v := NewVolumeEntryFromRequest(req)
		vc := NewVolumeCreateOperation(v, app.db)
		e := vc.Build()
		tests.Assert(t, e == nil, "expected e == nil, got", e)
	}

	// Create 10 complete blockvolumes and 15 pending blockvolumes
	blockVolreq := &api.BlockVolumeCreateRequest{}
	blockVolreq.Size = 1
	for i := 0; i < 10; i++ {
		blockvol := NewBlockVolumeEntryFromRequest(blockVolreq)
		vc := NewBlockVolumeCreateOperation(blockvol, app.db)
		e := vc.Build()
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		e = vc.Exec(app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		e = vc.Finalize()
		tests.Assert(t, e == nil, "expected e == nil, got", e)
	}
	for i := 0; i < 15; i++ {
		blockvol := NewBlockVolumeEntryFromRequest(blockVolreq)
		vc := NewBlockVolumeCreateOperation(blockvol, app.db)
		e := vc.Build()
		tests.Assert(t, e == nil, "expected e == nil, got", e)
	}

	// verify volumes, bricks, & pending ops exist
	app.db.View(func(tx *bolt.Tx) error {
		var pendingVols int
		var pendingBricks int
		var pendingBlockVols int

		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 25, "expected len(vl) == 25, got", len(vl))
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 25, "expected len(bvl) == 25, got", len(bvl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 75, "expected len(bl) == 75, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 30, "expected len(pol) == 30, got", len(pol))

		for _, vid := range vl {
			v, e := NewVolumeEntryFromId(tx, vid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			if v.Pending.Id != "" {
				pendingVols++
			}
		}
		tests.Assert(t, pendingVols == 15, "expected pendingVols == 15, got:", pendingVols)
		for _, bid := range bl {
			b, e := NewBrickEntryFromId(tx, bid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			if b.Pending.Id != "" {
				pendingBricks++
			}
		}
		tests.Assert(t, pendingBricks == 45, "expected pendingBricks == 45, got:", pendingBricks)
		for _, bvid := range bvl {
			bv, e := NewBlockVolumeEntryFromId(tx, bvid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			if bv.Pending.Id != "" {
				pendingBlockVols++
			}
		}
		tests.Assert(t, pendingBlockVols == 15, "expected pendingBricks == 15, got:", pendingBlockVols)
		return nil
	})

	err = DeletePendingEntries(app.db, false, false)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	// verify that completed volume & bricks exist but incomplete volume and bricks are gone
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 10, "expected len(vl) == 10, got", len(vl))
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 10, "expected len(bvl) == 10, got", len(bvl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 30, "expected len(bl) == 30, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))

		for _, vid := range vl {
			v, e := NewVolumeEntryFromId(tx, vid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Pending.Id == "",
				`expected v.Pending.Id == "", got:`, v.Pending.Id)
		}
		for _, bid := range bl {
			b, e := NewBrickEntryFromId(tx, bid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, b.Pending.Id == "",
				`expected b.Pending.Id == "", got:`, b.Pending.Id)
		}
		for _, bvid := range bvl {
			bv, e := NewBlockVolumeEntryFromId(tx, bvid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, bv.Pending.Id == "",
				`expected bv.Pending.Id == "" , got:`, bv.Pending.Id)
		}
		return nil
	})
}
