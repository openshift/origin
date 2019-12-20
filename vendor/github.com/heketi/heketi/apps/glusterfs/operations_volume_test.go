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
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"
)

func TestVolumeCreatePendingCreatedCleared(t *testing.T) {
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

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify volumes, bricks, & pending ops exist
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

		for _, vid := range vl {
			v, e := NewVolumeEntryFromId(tx, vid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Pending.Id == pol[0],
				"expected v.Pending.Id == pol[0], got:", v.Pending.Id, pol[0])
		}
		for _, bid := range bl {
			b, e := NewBrickEntryFromId(tx, bid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, b.Pending.Id == pol[0],
				"expected b.Pending.Id == pol[0], got:", b.Pending.Id, pol[0])
		}
		return nil
	})

	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify volumes & bricks exist but pending is gone
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

func TestVolumeCreatePendingRollback(t *testing.T) {
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

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify volumes, bricks, & pending ops exist
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

		for _, vid := range vl {
			v, e := NewVolumeEntryFromId(tx, vid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Pending.Id == pol[0],
				"expected v.Pending.Id == pol[0], got:", v.Pending.Id, pol[0])
		}
		for _, bid := range bl {
			b, e := NewBrickEntryFromId(tx, bid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, b.Pending.Id == pol[0],
				"expected b.Pending.Id == pol[0], got:", b.Pending.Id, pol[0])
		}
		return nil
	})

	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc.Rollback(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

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
}

func TestVolumeCreateRollbackCleanupFailure(t *testing.T) {
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

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// now we're going to pretend exec failed and inject an
	// error condition into VolumeDestroy

	app.xo.MockVolumeDestroy = func(host string, volume string) error {
		return fmt.Errorf("fake error")
	}

	e = vc.Rollback(app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got", e)
	markFailedIfSupported(vc)

	// verify that the pending items remain in the db due to rollback
	// failure
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
		pop, e := NewPendingOperationEntryFromId(tx, pol[0])
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, pop.Status == FailedOperation,
			"expected pop.Status == FailedOperation, got:", pop.Status)
		return nil
	})
}

func TestVolumeCreatePendingNoSpace(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		1*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.VolumeCreateRequest{}
	req.Size = 1024 * 5
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

	e := vc.Build()
	// verify that we failed to allocate due to lack of space
	tests.Assert(t, strings.Contains(e.Error(), ErrNoSpace.Error()),
		"expected strings.Contains(e.Error(), ErrNoSpace.Error()) got", e)

	// verify no volumes, bricks or pending ops in db
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

func TestVolumeCreatePendingBrickMissing(t *testing.T) {
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

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify volumes, bricks, & pending ops exist
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

		for _, vid := range vl {
			v, e := NewVolumeEntryFromId(tx, vid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Pending.Id == pol[0],
				"expected v.Pending.Id == pol[0], got:", v.Pending.Id, pol[0])
		}
		for _, bid := range bl {
			b, e := NewBrickEntryFromId(tx, bid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, b.Pending.Id == pol[0],
				"expected b.Pending.Id == pol[0], got:", b.Pending.Id, pol[0])
		}
		return nil
	})

	app.db.Update(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		b, e := NewBrickEntryFromId(tx, bl[0])
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		e = b.Delete(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	// now that the brick list in the db is broken Exec/Finalize/Rollback
	// will return errors

	e = vc.Exec(app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got", e)

	e = vc.Finalize()
	tests.Assert(t, e != nil, "expected e != nil, got", e)

	e = vc.Rollback(app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got", e)
}

func TestVolumeCreateOperationBasics(t *testing.T) {
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
	vol.Info.Id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	vc := NewVolumeCreateOperation(vol, app.db)

	tests.Assert(t, vc.Id() == vc.op.Id,
		"expected vc.Id() == vc.op.Id, got:", vc.Id(), vc.op.Id)
	tests.Assert(t, vc.Label() == "Create Volume",
		`expected vc.Label() == "Volume Create", got:`, vc.Label())
	tests.Assert(t, vc.ResourceUrl() == "/volumes/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		`expected vc.ResourceUrl() == "/volumes/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", got:`,
		vc.ResourceUrl())
}

// Test that volume create operations can retry with some
// "bad nodes" and still succeed overall.
func TestVolumeCreateOperationRetrying(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		2,    // clusters
		5,    // nodes_per_cluster
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

	l := sync.Mutex{}
	brickCreates := map[string]int{}
	bCreate := app.xo.MockBrickCreate
	app.xo.MockBrickCreate = func(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error) {
		l.Lock()
		defer l.Unlock()
		defer func() { brickCreates[host]++ }()
		if brickCreates[host] > 1 {
			return bCreate(host, brick)
		}
		return nil, fmt.Errorf("FAKE ERR")
	}

	vc.maxRetries = 10
	err = RunOperation(vc, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	x := 0
	for _, v := range brickCreates {
		x += v
	}
	tests.Assert(t, x >= 9, "expected x >= 10, got:", x)

	app.db.View(func(tx *bolt.Tx) error {
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(bl) == 1, got:", len(bl))
		vol, e := NewVolumeEntryFromId(tx, vl[0])
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, vol.Info.Size == 1024)
		tests.Assert(t, len(vol.Bricks) == 3,
			"expected len(vol.Bricks) == 3, got:", len(vol.Bricks))
		return nil
	})
}

func TestVolumeDeleteOperation(t *testing.T) {
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

	e = vd.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vd.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

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

func TestVolumeDeleteOperationRollback(t *testing.T) {
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

	e = vd.Rollback(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})
}

func TestVolumeDeleteOperationTwice(t *testing.T) {
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
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))

		// check that the volume is pending in the db
		v, e := NewVolumeEntryFromId(tx, vol.Info.Id)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, v.Pending.Id != "", "expected v to be pending")
		return nil
	})

	vd2 := NewVolumeDeleteOperation(vol, app.db)
	e = vd2.Build()
	tests.Assert(t, e == ErrConflict, "expected e == ErrConflict, got:", e)
}

func TestVolumeDeleteOperationDuringExpand(t *testing.T) {
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

	ve := NewVolumeExpandOperation(vol, app.db, 50)
	e = ve.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))
		return nil
	})

	vd := NewVolumeDeleteOperation(vol, app.db)
	e = vd.Build()
	tests.Assert(t, e == ErrConflict, "expected e == ErrConflict, got:", e)
}

func TestVolumeExpandOperation(t *testing.T) {
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

	e = ve.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	e = ve.Finalize()
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
		tests.Assert(t, pcount == 0, "expected len(bl) == 0, got:", pcount)
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})
}

func TestListCompleteVolumesDuringOperation(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		4,    // devices_per_node,
		6*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	t.Run("VolumeCreate", func(t *testing.T) {
		req := &api.VolumeCreateRequest{}
		req.Size = 1024
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		vol := NewVolumeEntryFromRequest(req)
		o := NewVolumeCreateOperation(vol, app.db)
		e := o.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		defer o.Rollback(app.executor)

		app.db.View(func(tx *bolt.Tx) error {
			vols, err := ListCompleteVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 0,
				"expected len(vols) == 0, got:", len(vols))
			vols, err = VolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 1,
				"expected len(vols) == 1, got:", len(vols))
			return nil
		})
	})
	t.Run("VolumeDelete", func(t *testing.T) {
		req := &api.VolumeCreateRequest{}
		req.Size = 1024
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		vol := NewVolumeEntryFromRequest(req)
		vco := NewVolumeCreateOperation(vol, app.db)
		e := RunOperation(vco, app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)

		app.db.View(func(tx *bolt.Tx) error {
			vols, err := ListCompleteVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 1,
				"expected len(vols) == 1, got:", len(vols))
			vols, err = VolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 1,
				"expected len(vols) == 1, got:", len(vols))
			return nil
		})

		vdo := NewVolumeDeleteOperation(vol, app.db)
		e = vdo.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		defer func() {
			e := vdo.Exec(app.executor)
			tests.Assert(t, e == nil, "expected e == nil, got:", e)
			e = vdo.Finalize()
			tests.Assert(t, e == nil, "expected e == nil, got:", e)
		}()

		app.db.View(func(tx *bolt.Tx) error {
			vols, err := ListCompleteVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 0,
				"expected len(vols) == 0, got:", len(vols))
			vols, err = VolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 1,
				"expected len(vols) == 1, got:", len(vols))
			return nil
		})
	})
	t.Run("BlockVolumeCreate", func(t *testing.T) {
		req := &api.BlockVolumeCreateRequest{}
		req.Size = 1024
		bvol := NewBlockVolumeEntryFromRequest(req)
		bvco := NewBlockVolumeCreateOperation(bvol, app.db)
		e := bvco.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		defer bvco.Rollback(app.executor)

		app.db.View(func(tx *bolt.Tx) error {
			vols, err := ListCompleteVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 0,
				"expected len(vols) == 0, got:", len(vols))
			vols, err = VolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 1,
				"expected len(vols) == 1, got:", len(vols))
			return nil
		})
	})
}

func TestVolumeCreateLimits(t *testing.T) {
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

	// We will backup the global value and set limit to 5
	oldMaxVolumesPerCluster := maxVolumesPerCluster
	maxVolumesPerCluster = 5
	defer func() { maxVolumesPerCluster = oldMaxVolumesPerCluster }()

	var cleanupVolume = func(vol *VolumeEntry) {
		vdo := NewVolumeDeleteOperation(vol, app.db)
		e := RunOperation(vdo, app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)

		app.db.View(func(tx *bolt.Tx) error {
			vols, err := VolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 4,
				"expected len(vols) == 4, got:", len(vols))
			return nil
		})
	}

	// Create 4 volumes
	for i := 0; i < 4; i++ {
		req := &api.VolumeCreateRequest{}
		req.Size = 1
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		vol := NewVolumeEntryFromRequest(req)
		vco := NewVolumeCreateOperation(vol, app.db)
		e := RunOperation(vco, app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
	}

	t.Run("InLimit", func(t *testing.T) {
		req := &api.VolumeCreateRequest{}
		req.Size = 1
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		vol := NewVolumeEntryFromRequest(req)
		vco := NewVolumeCreateOperation(vol, app.db)
		e := RunOperation(vco, app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		defer cleanupVolume(vol)

		app.db.View(func(tx *bolt.Tx) error {
			vols, err := VolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 5,
				"expected len(vols) == 5, got:", len(vols))
			return nil
		})

	})

	t.Run("BeyondLimit", func(t *testing.T) {
		// Hit the limit
		req := &api.VolumeCreateRequest{}
		req.Size = 1
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		vol := NewVolumeEntryFromRequest(req)
		vco := NewVolumeCreateOperation(vol, app.db)
		e := RunOperation(vco, app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		defer cleanupVolume(vol)

		// Next volume create should fail
		errstring := "has 5 volumes and limit is 5"
		newvol := NewVolumeEntryFromRequest(req)
		vco = NewVolumeCreateOperation(newvol, app.db)
		e = RunOperation(vco, app.executor)
		tests.Assert(t, strings.Contains(e.Error(), errstring),
			"expected strings.Contains(e.Error(),", errstring, " got:", e)

		// Check that we don't leave any pending volume
		// and the volume count is still the same as before
		app.db.View(func(tx *bolt.Tx) error {
			vols, err := VolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 5,
				"expected len(vols) == 5, got:", len(vols))
			vols, err = ListCompleteVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 5,
				"expected len(vols) == 5, got:", len(vols))
			return nil
		})

	})

	t.Run("BeyondLimitWhenPendingVolsExist", func(t *testing.T) {
		// Hit the limit but only as Pending
		req := &api.VolumeCreateRequest{}
		req.Size = 1
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		vol := NewVolumeEntryFromRequest(req)
		vco := NewVolumeCreateOperation(vol, app.db)
		e := vco.Build()
		tests.Assert(t, e == nil, "expected e == nil, got", e)

		// Next volume create should fail
		newvol := NewVolumeEntryFromRequest(req)
		newvco := NewVolumeCreateOperation(newvol, app.db)
		e = RunOperation(newvco, app.executor)
		errstring := "has 5 volumes and limit is 5"
		tests.Assert(t, strings.Contains(e.Error(), errstring),
			"expected strings.Contains(e.Error(),", errstring, " got:", e)

		// Check that we don't leave any pending volume
		// and the volume count is still the same as before
		app.db.View(func(tx *bolt.Tx) error {
			vols, err := VolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 5,
				"expected len(vols) == 5, got:", len(vols))
			vols, err = ListCompleteVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 4,
				"expected len(vols) == 4, got:", len(vols))
			return nil
		})

		// Check the volume in pending can still proceed
		e = vco.Exec(app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		e = vco.Finalize()
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		app.db.View(func(tx *bolt.Tx) error {
			vols, err := VolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 5,
				"expected len(vols) == 5, got:", len(vols))
			vols, err = ListCompleteVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 5,
				"expected len(vols) == 5, got:", len(vols))
			return nil
		})
		cleanupVolume(vol)

	})

	t.Run("BeyondLimitBHVImplicit", func(t *testing.T) {
		req := &api.VolumeCreateRequest{}
		req.Size = 1
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		vol := NewVolumeEntryFromRequest(req)
		vco := NewVolumeCreateOperation(vol, app.db)
		e := RunOperation(vco, app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		defer cleanupVolume(vol)

		// Next volume create should fail
		errstring := "has 5 volumes and limit is 5"
		newbvol := NewBlockVolumeEntryFromRequest(&api.BlockVolumeCreateRequest{
			Size: 40,
		})
		bvco := NewBlockVolumeCreateOperation(newbvol, app.db)
		e = RunOperation(bvco, app.executor)
		tests.Assert(t, strings.Contains(e.Error(), errstring),
			"expected strings.Contains(e.Error(),", errstring, " got:", e)

		// Check that we don't leave any pending volume
		// and the volume count is still the same as before
		app.db.View(func(tx *bolt.Tx) error {
			vols, err := VolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 5,
				"expected len(vols) == 5, got:", len(vols))
			vols, err = ListCompleteVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 5,
				"expected len(vols) == 5, got:", len(vols))
			return nil
		})

	})

	t.Run("BeyondLimitBHVManual", func(t *testing.T) {
		req := &api.VolumeCreateRequest{}
		req.Size = 1
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		vol := NewVolumeEntryFromRequest(req)
		vco := NewVolumeCreateOperation(vol, app.db)
		e := RunOperation(vco, app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		defer cleanupVolume(vol)

		// Next volume create should fail
		errstring := "has 5 volumes and limit is 5"
		newvol := NewVolumeEntryFromRequest(&api.VolumeCreateRequest{
			Size:  1,
			Block: true,
		})
		vco = NewVolumeCreateOperation(newvol, app.db)
		e = RunOperation(vco, app.executor)
		tests.Assert(t, e != nil, "expected e != nil")
		tests.Assert(t, strings.Contains(e.Error(), errstring),
			"expected strings.Contains(e.Error(),", errstring, " got:", e)

		// Check that we don't leave any pending volume
		// and the volume count is still the same as before
		app.db.View(func(tx *bolt.Tx) error {
			vols, err := VolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 5,
				"expected len(vols) == 5, got:", len(vols))
			vols, err = ListCompleteVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 5,
				"expected len(vols) == 5, got:", len(vols))
			return nil
		})

	})

	t.Run("UnderLimitBHV", func(t *testing.T) {
		req := &api.VolumeCreateRequest{}
		req.Size = 40
		req.Block = true
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		vol := NewVolumeEntryFromRequest(req)
		vco := NewVolumeCreateOperation(vol, app.db)
		e := RunOperation(vco, app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		defer cleanupVolume(vol)

		// creating new block volumes go into exiting BVH
		for i := 0; i <= 5; i++ {
			newbvol := NewBlockVolumeEntryFromRequest(&api.BlockVolumeCreateRequest{
				Size: 1,
			})
			bvco := NewBlockVolumeCreateOperation(newbvol, app.db)
			e = RunOperation(bvco, app.executor)
			tests.Assert(t, e == nil, "expected e == nil, got:", e)
		}

		// trying to create a BVol that needs a new BHV will fail
		newbvol := NewBlockVolumeEntryFromRequest(&api.BlockVolumeCreateRequest{
			Size: 100,
		})
		bvco := NewBlockVolumeCreateOperation(newbvol, app.db)
		e = RunOperation(bvco, app.executor)
		errstring := "has 5 volumes and limit is 5"
		tests.Assert(t, strings.Contains(e.Error(), errstring),
			"expected strings.Contains(e.Error(),", errstring, " got:", e)

		// Check that we don't leave any pending volume
		// and the volume count is still the same as before
		app.db.View(func(tx *bolt.Tx) error {
			vols, err := VolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 5,
				"expected len(vols) == 5, got:", len(vols))
			vols, err = ListCompleteVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 5,
				"expected len(vols) == 5, got:", len(vols))
			return nil
		})

	})

}
