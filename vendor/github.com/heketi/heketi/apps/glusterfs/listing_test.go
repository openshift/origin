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

func TestListCompleteVolumes(t *testing.T) {
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

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(req)
	err = vol.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		vols, err := ListCompleteVolumes(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got:", len(vols))

		// this next bit just tickles test coverage for now
		cids, err := ClusterList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(cids) == 1, "expected len(cids) == 1, got:", len(cids))
		ce, err := NewClusterEntryFromId(tx, cids[0])
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		info, err := ce.NewClusterInfoResponse(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		err = UpdateClusterInfoComplete(tx, info)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(info.Volumes) == 1,
			"expected len(info.Volumes) == 1, got:", len(info.Volumes))
		return nil
	})
}

func TestListCompleteBlockVolumes(t *testing.T) {
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

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 1024

	vol := NewBlockVolumeEntryFromRequest(req)
	err = vol.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		vols, err := ListCompleteBlockVolumes(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got:", len(vols))

		// this next bit just tickles test coverage for now
		cids, err := ClusterList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(cids) == 1, "expected len(cids) == 1, got:", len(cids))
		ce, err := NewClusterEntryFromId(tx, cids[0])
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		info, err := ce.NewClusterInfoResponse(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		err = UpdateClusterInfoComplete(tx, info)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(info.BlockVolumes) == 1,
			"expected len(info.BlockVolumes) == 1, got:", len(info.BlockVolumes))
		return nil
	})
}

func TestListingRemoveKeysFromList(t *testing.T) {
	r := removeKeysFromList(
		[]string{"foo"},
		map[string]string{"foo": "a"})
	tests.Assert(t, len(r) == 0, "expected len(r) == 0, got:", len(r))

	r = removeKeysFromList(
		[]string{"foo", "bar"},
		map[string]string{"foo": "a"})
	tests.Assert(t, len(r) == 1, "expected len(r) == 1, got:", len(r))
	tests.Assert(t, r[0] == "bar", "expected r[0] == \"bar\", got:", r)

	r = removeKeysFromList(
		[]string{"foo", "bar"},
		map[string]string{"baz": "a"})
	tests.Assert(t, len(r) == 2, "expected len(r) == 2, got:", len(r))
	tests.Assert(t, r[0] == "foo", "expected r[0] == \"foo\", got:", r)
	tests.Assert(t, r[1] == "bar", "expected r[1] == \"bar\", got:", r)
}

func TestListCompleteVolumesFakedPending(t *testing.T) {
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

	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(req)
	err = vol.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		vols, err := ListCompleteVolumes(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got:", len(vols))
		return nil
	})

	// set up a fake pending op
	app.db.Update(func(tx *bolt.Tx) error {
		vols, err := VolumeList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		po := NewPendingOperationEntry(NEW_ID)
		po.Type = OperationCreateVolume
		po.Actions = append(po.Actions, PendingOperationAction{
			Change: OpAddVolume,
			Id:     vols[0],
		})
		err = po.Save(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		return nil
	})

	app.db.View(func(tx *bolt.Tx) error {
		vols, err := ListCompleteVolumes(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(vols) == 0, "expected len(vols) == 0, got:", len(vols))
		return nil
	})
}

func TestListCompleteBlockVolumesFakedPending(t *testing.T) {
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

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 1024

	vol := NewBlockVolumeEntryFromRequest(req)
	err = vol.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		bvols, err := ListCompleteBlockVolumes(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 1, got:", len(bvols))
		return nil
	})

	// set up a fake pending op
	app.db.Update(func(tx *bolt.Tx) error {
		bvols, err := BlockVolumeList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		po := NewPendingOperationEntry(NEW_ID)
		po.Type = OperationCreateBlockVolume
		po.Actions = append(po.Actions, PendingOperationAction{
			Change: OpAddBlockVolume,
			Id:     bvols[0],
		})
		err = po.Save(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		return nil
	})

	app.db.View(func(tx *bolt.Tx) error {
		bvols, err := ListCompleteBlockVolumes(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(bvols) == 0, "expected len(bvols) == 0, got:", len(bvols))
		return nil
	})
}

func TestUpdateVolumeInfoCompleteFakedPending(t *testing.T) {
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

	breq := &api.BlockVolumeCreateRequest{}
	breq.Size = 1024

	bvol := NewBlockVolumeEntryFromRequest(breq)
	err = bvol.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		vols, err := ListCompleteVolumes(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got:", len(vols))
		bvols, err := ListCompleteBlockVolumes(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 1, got:", len(bvols))
		return nil
	})

	// set up fake pending ops
	app.db.Update(func(tx *bolt.Tx) error {
		bvols, err := BlockVolumeList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		po := NewPendingOperationEntry(NEW_ID)
		po.Type = OperationCreateBlockVolume
		po.Actions = append(po.Actions, PendingOperationAction{
			Change: OpAddBlockVolume,
			Id:     bvols[0],
		})
		err = po.Save(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		return nil
	})

	app.db.View(func(tx *bolt.Tx) error {
		vids, err := VolumeList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(vids) == 1, "expected len(vids) == 1, got:", len(vids))

		ve, err := NewVolumeEntryFromId(tx, vids[0])
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		info, err := ve.NewInfoResponse(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(info.BlockInfo.BlockVolumes) == 1,
			"expected len(info.BlockInfo.BlockVolumes) == 1, got:", len(info.BlockInfo.BlockVolumes))

		err = UpdateVolumeInfoComplete(tx, info)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(info.BlockInfo.BlockVolumes) == 0,
			"expected len(info.BlockInfo.BlockVolumes) == 0, got:", len(info.BlockInfo.BlockVolumes))
		return nil
	})
}

func TestUpdateClusterInfoCompleteFakedPending(t *testing.T) {
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

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 1024
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(vreq)
	err = vol.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	breq := &api.BlockVolumeCreateRequest{}
	breq.Size = 1024

	bvol := NewBlockVolumeEntryFromRequest(breq)
	err = bvol.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		vols, err := ListCompleteVolumes(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(vols) == 2, "expected len(vols) == 2, got:", len(vols))
		bvols, err := ListCompleteBlockVolumes(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(bvols) == 1, "expected len(bvols) == 1, got:", len(bvols))
		return nil
	})

	// set up fake pending ops
	app.db.Update(func(tx *bolt.Tx) error {
		vols, err := VolumeList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		po := NewPendingOperationEntry(NEW_ID)
		po.Type = OperationCreateVolume
		po.Actions = append(po.Actions, PendingOperationAction{
			Change: OpAddVolume,
			Id:     vols[0],
		})
		err = po.Save(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		bvols, err := BlockVolumeList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		po = NewPendingOperationEntry(NEW_ID)
		po.Type = OperationCreateBlockVolume
		po.Actions = append(po.Actions, PendingOperationAction{
			Change: OpAddBlockVolume,
			Id:     bvols[0],
		})
		err = po.Save(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		return nil
	})

	app.db.View(func(tx *bolt.Tx) error {
		cids, err := ClusterList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(cids) == 1, "expected len(cids) == 1, got:", len(cids))

		ce, err := NewClusterEntryFromId(tx, cids[0])
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		info, err := ce.NewClusterInfoResponse(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(info.Volumes) == 2,
			"expected len(info.Volumes) == 2, got:", len(info.Volumes))
		tests.Assert(t, len(info.BlockVolumes) == 1,
			"expected len(info.BlockVolumes) == 1, got:", len(info.BlockVolumes))

		err = UpdateClusterInfoComplete(tx, info)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(info.Volumes) == 1,
			"expected len(info.Volumes) == 1, got:", len(info.Volumes))
		tests.Assert(t, len(info.BlockVolumes) == 0,
			"expected len(info.BlockVolumes) == 0, got:", len(info.BlockVolumes))
		return nil
	})
}

func TestListCompleteVolumesFakedPendingBlockHosting(t *testing.T) {
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

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 1024

	vol := NewBlockVolumeEntryFromRequest(req)
	err = vol.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		vols, err := ListCompleteVolumes(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(vols) == 1, "expected len(vols) == 1, got:", len(vols))
		return nil
	})

	// set up a fake pending op
	app.db.Update(func(tx *bolt.Tx) error {
		vols, err := VolumeList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		po := NewPendingOperationEntry(NEW_ID)
		po.Type = OperationCreateBlockVolume
		po.Actions = append(po.Actions, PendingOperationAction{
			Change: OpAddVolume,
			Id:     vols[0],
		})
		err = po.Save(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		return nil
	})

	app.db.View(func(tx *bolt.Tx) error {
		vols, err := ListCompleteVolumes(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(vols) == 0, "expected len(vols) == 0, got:", len(vols))
		return nil
	})
}
