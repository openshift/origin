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

	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"
)

func TestVolumeSetBlockRestriction(t *testing.T) {
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
	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 200
	vreq.Block = true
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(vreq)
	vc := NewVolumeCreateOperation(vol, app.db)

	e := RunOperation(vc, app.executor)
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

	t.Run("SetLocked", func(t *testing.T) {
		sro := NewVolumeSetBlockRestrictionOperation(vol, app.db, api.Locked)
		e := sro.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		e = sro.Exec(app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		e = sro.Finalize()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
	})
	t.Run("SetLockedWhenLocked", func(t *testing.T) {
		sro := NewVolumeSetBlockRestrictionOperation(vol, app.db, api.Locked)
		e := sro.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		e = sro.Exec(app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		e = sro.Finalize()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
	})
	t.Run("SetUnrestricted", func(t *testing.T) {
		sro := NewVolumeSetBlockRestrictionOperation(vol, app.db, api.Unrestricted)
		e := sro.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		e = sro.Exec(app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		e = sro.Finalize()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
	})
	t.Run("UnsetLBU-ReservedAlready", func(t *testing.T) {
		app.db.Update(func(tx *bolt.Tx) error {
			vol.Info.BlockInfo.Restriction = api.LockedByUpdate
			e := vol.Save(tx)
			tests.Assert(t, e == nil, "expected e == nil, got:", e)
			return nil
		})
		sro := NewVolumeSetBlockRestrictionOperation(vol, app.db, api.Unrestricted)
		e := sro.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		e = sro.Exec(app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		e = sro.Finalize()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
	})
	t.Run("UnsetLBU-FreeOk", func(t *testing.T) {
		// return to original sizes after we muck with them
		defer func(f, r int) {
			app.db.Update(func(tx *bolt.Tx) error {
				vol.Info.BlockInfo.FreeSize = f
				vol.Info.BlockInfo.ReservedSize = r
				vol.Save(tx)
				return nil
			})
		}(vol.Info.BlockInfo.FreeSize, vol.Info.BlockInfo.ReservedSize)
		app.db.Update(func(tx *bolt.Tx) error {
			vol.Info.BlockInfo.Restriction = api.LockedByUpdate
			vol.Info.BlockInfo.ReservedSize = 0
			e := vol.Save(tx)
			tests.Assert(t, e == nil, "expected e == nil, got:", e)
			return nil
		})
		sro := NewVolumeSetBlockRestrictionOperation(vol, app.db, api.Unrestricted)
		e := sro.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		e = sro.Exec(app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		e = sro.Finalize()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
	})
	t.Run("UnsetLBU-FreeBad", func(t *testing.T) {
		// return to original sizes after we muck with them
		defer func(f, r int) {
			app.db.Update(func(tx *bolt.Tx) error {
				vol.Info.BlockInfo.FreeSize = f
				vol.Info.BlockInfo.ReservedSize = r
				vol.Save(tx)
				return nil
			})
		}(vol.Info.BlockInfo.FreeSize, vol.Info.BlockInfo.ReservedSize)
		app.db.Update(func(tx *bolt.Tx) error {
			vol.Info.BlockInfo.Restriction = api.LockedByUpdate
			vol.Info.BlockInfo.FreeSize = 1
			vol.Info.BlockInfo.ReservedSize = 0
			e := vol.Save(tx)
			tests.Assert(t, e == nil, "expected e == nil, got:", e)
			return nil
		})
		sro := NewVolumeSetBlockRestrictionOperation(vol, app.db, api.Unrestricted)
		e := sro.Build()
		tests.Assert(t, e != nil, "expected e != nil, got:", e)
	})
	t.Run("GarbageValue", func(t *testing.T) {
		app.db.Update(func(tx *bolt.Tx) error {
			vol.Info.BlockInfo.Restriction = "trogdor"
			e := vol.Save(tx)
			tests.Assert(t, e == nil, "expected e == nil, got:", e)
			return nil
		})
		sro := NewVolumeSetBlockRestrictionOperation(vol, app.db, api.Unrestricted)
		e := sro.Build()
		tests.Assert(t, e != nil, "expected e != nil, got:", e)
	})
}

func TestVolumeSetBlockRestrictionNotBHV(t *testing.T) {
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

	// first we create a volume that is not a bhv
	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 200
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(vreq)
	vc := NewVolumeCreateOperation(vol, app.db)
	e := RunOperation(vc, app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	sro := NewVolumeSetBlockRestrictionOperation(vol, app.db, api.Locked)
	e = sro.Build()
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
}
