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
	"testing"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"
)

func TestBlockHostingVolumeExpandOperation(t *testing.T) {
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
	req.Block = true

	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		volumelist, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(volumelist) == 1, "expected len(bl) == 1, got:", len(volumelist))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		for _, id := range volumelist {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Info.Size == 1024, "expected volume size == 1024, got:", v.Info.Size)
			expectedFreeSize := ReduceRawSize(1024)
			tests.Assert(t, v.Info.BlockInfo.FreeSize == expectedFreeSize, "expected free size == ", expectedFreeSize, " got:", v.Info.BlockInfo.FreeSize)
		}
		return nil
	})

	ve := NewVolumeExpandOperation(vol, app.db, 100)
	e = ve.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	e = ve.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	e = ve.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	app.db.View(func(tx *bolt.Tx) error {
		volumelist, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(volumelist) == 1, "expected len(bl) == 1, got:", len(volumelist))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		for _, id := range volumelist {
			v, e := NewVolumeEntryFromId(tx, id)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Info.Size == 1124, "expected volume size == 1124, got:", v.Info.Size)
			expectedFreeSize := ReduceRawSize(1024) + ReduceRawSize(100)
			tests.Assert(t, v.Info.BlockInfo.FreeSize == expectedFreeSize, "expected free size == ", expectedFreeSize, " got:", v.Info.BlockInfo.FreeSize)
		}
		return nil
	})

}

func TestBlockVolumeCreateOperation(t *testing.T) {
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

	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify the volume and bricks exist but no pending op
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
		return nil
	})
}

func TestBlockVolumeCreateOperationTooLargeSizeRequested(t *testing.T) {
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
	// request a size larger than the BlockHostingVolumeSize
	// can host (the raw capacity is 1100GiB, but that filesystem
	// can not hold a file of exactly that size)
	req.Size = 1100

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
	error_string := "The size configured for automatic creation of block hosting volumes (1100) is too small to host the requested block volume of size 1100. The available size on this block hosting volume, minus overhead, is 1078. Please create a sufficiently large block hosting volume manually."
	tests.Assert(t, e != nil, "expected e != nil, got nil")
	tests.Assert(t, e.Error() == error_string,
		"expected '", error_string, "', got '", e.Error(), "'")
}

func TestBlockVolumeCreateBlockHostingVolumeCreationDisabled(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	CreateBlockHostingVolumes = false

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		2*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 100

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
	error_string := "Block Hosting Volume Creation is disabled. Create a Block hosting volume and try again."
	tests.Assert(t, e != nil, "expected e != nil, got nil")
	tests.Assert(t, e.Error() == error_string,
		"expected '", error_string, "', got '", e.Error(), "'")
}

func TestBlockVolumeCreateOperationExistingHostVol(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		3*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// first we create a volume to host the block volume

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 2048
	vreq.Block = true
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(vreq)
	vc := NewVolumeCreateOperation(vol, app.db)

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 0, "expected len(bvl) == 0, got", len(bvl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})

	breq := &api.BlockVolumeCreateRequest{}
	breq.Size = 1024

	bvol := NewBlockVolumeEntryFromRequest(breq)
	bco := NewBlockVolumeCreateOperation(bvol, app.db)

	e = bco.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	// at this point we shouldn't have a new volume or bricks,
	// just a pending op for the block volume itself
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(bvl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))
		return nil
	})

	e = bco.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	e = bco.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	// the block volume is there but the pending op is gone
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(bvl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})
}

func TestBlockVolumeCreateOperationRollback(t *testing.T) {
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

	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// it doesn't matter if exec worked, were going to rollback for test
	e = vc.Rollback(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify that everything got trashed
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 0, "expected len(bvl) == 0, got", len(bvl))
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

func TestBlockVolumeCreateOperationExistingHostVolRollback(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		3*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// first we create a volume to host the block volume

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 2048
	vreq.Block = true
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(vreq)
	vc := NewVolumeCreateOperation(vol, app.db)

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 0, "expected len(bvl) == 0, got", len(bvl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})

	breq := &api.BlockVolumeCreateRequest{}
	breq.Size = 1024

	bvol := NewBlockVolumeEntryFromRequest(breq)
	bco := NewBlockVolumeCreateOperation(bvol, app.db)

	e = bco.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	// at this point we shouldn't have a new volume or bricks,
	// just a pending op for the block volume itself
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(bvl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got:", len(bl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))
		return nil
	})

	e = bco.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	// it doesn't matter if exec worked, were going to rollback for test
	e = bco.Rollback(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify that only the block volume got trashed
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

func TestBlockVolumeDeleteOperation(t *testing.T) {
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

	e = bdel.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = bdel.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

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

func TestBlockVolumeDeleteOperationRollback(t *testing.T) {
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

	e = bdel.Rollback(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// the pending op should be gone, but other items remain
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
}

func TestBlockVolumeDeleteOperationTwice(t *testing.T) {
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

	app.db.View(func(tx *bolt.Tx) error {
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

	var vol2 *BlockVolumeEntry
	app.db.View(func(tx *bolt.Tx) error {
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		vol2, e = NewBlockVolumeEntryFromId(tx, vol.Info.Id)
		return nil
	})
	tests.Assert(t, vol.Pending.Id == "")
	tests.Assert(t, vol2.Pending.Id == "")

	bdel := NewBlockVolumeDeleteOperation(vol, app.db)
	e = bdel.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 1, "expected len(pol) == 1, got", len(pol))
		return nil
	})

	bdel2 := NewBlockVolumeDeleteOperation(vol2, app.db)
	e = bdel2.Build()
	tests.Assert(t, e == ErrConflict, "expected e ErrConflict, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 1, "expected len(pol) == 1, got", len(pol))
		return nil
	})
}

// TestBlockVolumeCreatePendingBHV tests the behavior of the system
// when a block hosting volume exists but is pending and another
// block volume request is received.
func TestBlockVolumeCreateBuildRollback(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		2,    // devices_per_node,
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

	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 1, "expected len(pol) == 1, got", len(pol))
		return nil
	})

	e = vc.Rollback(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})
}

func TestBlockVolumeCloneFails(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		3*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// first we create a volume to host the block volume

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 2048
	vreq.Block = true
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(vreq)
	vc := NewVolumeCreateOperation(vol, app.db)

	err = RunOperation(vc, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, vol.Info.Id != "", "expected vol.Info.Id != \"\", got:", vol.Info.Id)

	cloneOp := NewVolumeCloneOperation(vol, app.db, "foo")
	err = RunOperation(cloneOp, app.executor)
	tests.Assert(t, err == ErrCloneBlockVol, "expected err == ErrCloneBlockVol, got:", err)
}

func TestBlockVolumesCreateRejectPendingBHV(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		3*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 10

	vol1 := NewBlockVolumeEntryFromRequest(req)
	vc1 := NewBlockVolumeCreateOperation(vol1, app.db)
	vol2 := NewBlockVolumeEntryFromRequest(req)
	vc2 := NewBlockVolumeCreateOperation(vol2, app.db)

	// verify that there are no volumes yet
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	e := vc1.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc2.Build()
	tests.Assert(t, e != nil, "expected e != nil, got", e)
	tests.Assert(t, e == ErrTooManyOperations,
		"expected e == ErrTooManyOperations, got:", e)

	e = vc1.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc1.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify that there is now a BHV
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	// try the same request again
	// it should work and used the just created BHV
	vc2 = NewBlockVolumeCreateOperation(vol2, app.db)
	e = vc2.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = vc2.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)
	e = vc2.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify that it now used the same BHV
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})
}

func TestBlockVolumesCreatePendingBHVIgnoredItems(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		3*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 10

	vol1 := NewBlockVolumeEntryFromRequest(req)
	vc1 := NewBlockVolumeCreateOperation(vol1, app.db)
	vol2 := NewBlockVolumeEntryFromRequest(req)
	vc2 := NewBlockVolumeCreateOperation(vol2, app.db)

	// verify that there are no volumes yet
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	e := vc1.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// make the current pending operation stale
	e = app.ServerReset()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify that the pending volume exists
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 1, "expected len(pol) == 1, got", len(pol))
		return nil
	})

	// create a pending (new) non-block volume
	req2 := &api.VolumeCreateRequest{}
	req2.Size = 5
	vol := NewVolumeEntryFromRequest(req2)
	vco := NewVolumeCreateOperation(vol, app.db)
	e = vco.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc2.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc2.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	e = vc2.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify that there is now a BHV (and a stale one)
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 3, "expected len(vl) == 3, got", len(vl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 2, "expected len(pol) == 2, got", len(pol))
		return nil
	})
}

func TestBlockVolumeCreateRollbackCleanupFailure(t *testing.T) {
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

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 1024

	vol := NewBlockVolumeEntryFromRequest(req)
	vc := NewBlockVolumeCreateOperation(vol, app.db)

	// verify that there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// now we're going to pretend exec failed and inject an
	// error condition into BlockVolumeDestroy
	app.xo.MockBlockVolumeDestroy = func(host, bhv, volume string) error {
		return fmt.Errorf("fake error")
	}

	e = vc.Rollback(app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got", e)

	// verify that the pending items remain in the db due to rollback
	// failure
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		bl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 1, "expected len(bl) == 1, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 1, "expected len(pol) == 1, got", len(pol))
		return nil
	})
}

func TestBlockVolumeCreateOperationOveruse(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	app.conf.CreateBlockHostingVolumes = false
	app.setBlockSettings()
	CreateBlockHostingVolumes = false
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		3*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// first we create a volume to host the block volume
	// Using size=150 so that only one 100 block volume should
	// be able to be placed on this hosting volume
	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 150
	vreq.Block = true
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(vreq)
	vc := NewVolumeCreateOperation(vol, app.db)

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 0, "expected len(bvl) == 0, got", len(bvl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})

	breq := &api.BlockVolumeCreateRequest{}
	breq.Size = 100

	bvol1 := NewBlockVolumeEntryFromRequest(breq)
	bvol2 := NewBlockVolumeEntryFromRequest(breq)

	bco1 := NewBlockVolumeCreateOperation(bvol1, app.db)
	bco2 := NewBlockVolumeCreateOperation(bvol2, app.db)

	e = bco1.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	e = bco2.Build()
	tests.Assert(t, e != nil, "expected e != nil, got:", e)

	// at this point we shouldn't have a new volume or bricks,
	// just a pending op for the block volume itself
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(bvl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))
		return nil
	})

	e = bco1.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	e = bco1.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	// the block volume is there but the pending op is gone
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(bvl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})
}

func TestBlockVolumeCreateOperationLockedBHV(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	app.conf.CreateBlockHostingVolumes = false
	app.setBlockSettings()
	CreateBlockHostingVolumes = false
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		3*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// first we create a volume to host the block volume
	// Using size=150 so that only one 100 block volume should
	// be able to be placed on this hosting volume
	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 150
	vreq.Block = true
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(vreq)
	vc := NewVolumeCreateOperation(vol, app.db)

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = vc.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 0, "expected len(bvl) == 0, got", len(bvl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})

	breq := &api.BlockVolumeCreateRequest{}
	breq.Size = 2

	bvol1 := NewBlockVolumeEntryFromRequest(breq)
	bco1 := NewBlockVolumeCreateOperation(bvol1, app.db)
	e = bco1.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = bco1.Exec(app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	e = bco1.Finalize()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	// check that block vol created successfully
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(bvl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})

	// now set volume to locked
	app.db.Update(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		vol, e := NewVolumeEntryFromId(tx, vl[0])
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		vol.Info.BlockInfo.Restriction = api.Locked
		e = vol.Save(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	bvol2 := NewBlockVolumeEntryFromRequest(breq)
	bco2 := NewBlockVolumeCreateOperation(bvol2, app.db)
	e = bco2.Build()
	tests.Assert(t, e != nil, "expected e != nil, got:", e)

	// check that db state is unchanged
	app.db.View(func(tx *bolt.Tx) error {
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(bvl))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(vl))
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 0, "expected len(po) == 0, got:", len(po))
		return nil
	})
}

// TestBlockVolumeCreateInsufficientHosts checks both that the
// function fails due to insufficient hosts being online for
// the requested HA count, and that no block volumes are left
// in the db when we roll-back the operation.
func TestBlockVolumeCreateInsufficientHosts(t *testing.T) {
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

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 1024
	req.Hacount = 3

	vol := NewBlockVolumeEntryFromRequest(req)
	vc := NewBlockVolumeCreateOperation(vol, app.db)

	// verify that there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	// now we're going to pretend exec failed and inject an
	// error condition into BlockVolumeDestroy
	zapHosts := map[string]bool{}
	app.xo.MockGlusterdCheck = func(host string) error {
		if len(zapHosts) == 0 {
			// we zap whatever the first host we see
			zapHosts[host] = true
		}
		if zapHosts[host] {
			return fmt.Errorf("you shall not pass")
		}
		return nil
	}
	app.xo.MockBlockVolumeDestroy = func(host, bhv, volume string) error {
		return &executors.VolumeDoesNotExistErr{Name: volume}
	}

	e := RunOperation(vc, app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got", e)

	// verify that everything got cleaned up
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		bl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})
}

func TestListCompleteBlockVolumesDuringOperation(t *testing.T) {
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

	t.Run("BlockVolumeCreate", func(t *testing.T) {
		req := &api.BlockVolumeCreateRequest{}
		req.Size = 1024
		bvol := NewBlockVolumeEntryFromRequest(req)
		bvco := NewBlockVolumeCreateOperation(bvol, app.db)
		e := bvco.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		defer bvco.Rollback(app.executor)

		app.db.View(func(tx *bolt.Tx) error {
			vols, err := ListCompleteBlockVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 0,
				"expected len(vols) == 0, got:", len(vols))
			vols, err = BlockVolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 1,
				"expected len(vols) == 1, got:", len(vols))
			return nil
		})
	})
	t.Run("BlockVolumeDelete", func(t *testing.T) {
		req := &api.BlockVolumeCreateRequest{}
		req.Size = 1024
		bvol := NewBlockVolumeEntryFromRequest(req)
		bvco := NewBlockVolumeCreateOperation(bvol, app.db)
		e := RunOperation(bvco, app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)

		app.db.View(func(tx *bolt.Tx) error {
			vols, err := ListCompleteBlockVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 1,
				"expected len(vols) == 1, got:", len(vols))
			vols, err = BlockVolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 1,
				"expected len(vols) == 1, got:", len(vols))
			return nil
		})

		bvdo := NewBlockVolumeDeleteOperation(bvol, app.db)
		e = bvdo.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		defer func() {
			e := bvdo.Exec(app.executor)
			tests.Assert(t, e == nil, "expected e == nil, got:", e)
			e = bvdo.Finalize()
			tests.Assert(t, e == nil, "expected e == nil, got:", e)
		}()

		app.db.View(func(tx *bolt.Tx) error {
			vols, err := ListCompleteBlockVolumes(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 0,
				"expected len(vols) == 0, got:", len(vols))
			vols, err = BlockVolumeList(tx)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			tests.Assert(t, len(vols) == 1,
				"expected len(vols) == 1, got:", len(vols))
			return nil
		})
	})
}
