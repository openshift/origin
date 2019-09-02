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
	"time"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"

	"github.com/heketi/heketi/pkg/glusterfs/api"
)

func TestBasicOperationsCleanup(t *testing.T) {
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
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// assert that pending volume create got cleaned up
	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
}

func TestOperationsCleanupThreeOps(t *testing.T) {
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

	// create a volume we can delete later
	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)
	e := RunOperation(vc, app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
	dvol := vol

	// create 1st pending op
	vol = NewVolumeEntryFromRequest(req)
	vc = NewVolumeCreateOperation(vol, app.db)
	e = vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// create 2nd pending op
	vol = NewVolumeEntryFromRequest(req)
	vc = NewVolumeCreateOperation(vol, app.db)
	e = vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// create 3rd pending op
	vdel := NewVolumeDeleteOperation(dvol, app.db)
	e = vdel.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 3, "expected len(l) == 3, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
}

func TestOperationsCleanupSkipNonLoadable(t *testing.T) {
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

	// create a dummy volume so there's at least one brick on a device
	req := &api.VolumeCreateRequest{}
	req.Size = 1024
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	// create a volume we can delete later
	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)
	e := RunOperation(vc, app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	var deviceId string
	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		dl, e := DeviceList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(dl) > 1, "expected len(dl) > 1, got:", len(dl))
		for _, d := range dl {
			dev, e := NewDeviceEntryFromId(tx, d)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			if len(dev.Bricks) >= 1 {
				deviceId = d
			}
		}
		tests.Assert(t, deviceId != "")
		return nil
	})

	dro := NewDeviceRemoveOperation(deviceId, app.db)
	e = dro.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// the non cleanable device remove operation remains
	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})
}

func TestOperationsCleanupVolumeExpand(t *testing.T) {
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

	// create a volume we can delete later
	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)
	e := RunOperation(vc, app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})

	ve := NewVolumeExpandOperation(vol, app.db, 50)
	e = ve.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.Update(func(tx *bolt.Tx) error {
		po, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(po) == 1, "expected len(po) == 1, got:", len(po))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got:", len(l))
		vol, e := NewVolumeEntryFromId(tx, vl[0])
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, vol.Info.Size == 1024,
			"expected vol.Info.Size == 1024, got:", vol.Info.Size)
		return nil
	})
}

func TestOperationsCleanupBlockVolumeCreate(t *testing.T) {
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
	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
}

func TestOperationsCleanupBlockVolumeDelete(t *testing.T) {
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
	e := RunOperation(vc, app.executor)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	vdel := NewBlockVolumeDeleteOperation(vol, app.db)
	e = vdel.Build()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
}

func TestOperationsCleanupCleanFail(t *testing.T) {
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
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	app.xo.MockVolumeDestroy = func(host string, volume string) error {
		return fmt.Errorf("fake error")
	}

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	// even if an individual clean fails, the overall clean succeeds
	// unless some really horribly goes wrong
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// the pending op should remain because the Clean failed
	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})
}

func TestOperationsCleanupBrokenOp(t *testing.T) {
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
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		// we delete the volume from the db to purposefully
		// "break" the pending operation load
		pop, e := NewPendingOperationEntryFromId(tx, l[0])
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		var volId string
		for _, action := range pop.Actions {
			if action.Change == OpAddVolume {
				volId = action.Id
				break
			}
		}
		tests.Assert(t, volId != "", "expected volId != \"\"")
		v, e := NewVolumeEntryFromId(tx, volId)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		e = v.Delete(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	app.xo.MockVolumeDestroy = func(host string, volume string) error {
		return fmt.Errorf("fake error")
	}

	oc := OperationCleaner{
		db:       app.db,
		executor: app.executor,
		sel:      CleanAll,
	}
	e = oc.Clean()
	// even if an individual clean fails, the overall clean succeeds
	// unless some really horribly goes wrong
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// the pending op should remain because the Clean failed
	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})
}

func TestOperationCleanerMarkStale(t *testing.T) {
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

	// take control of time
	fakeTime := operationTimestamp()
	operationTimestamp = func() int64 {
		return fakeTime
	}

	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)
	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		pstale, e := PendingOperationEntrySelection(tx, CleanAll)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pstale) == 0,
			"expected len(pstale) == 0, got:", len(pstale))
		return nil
	})

	ot := newOpTracker(8)
	oc := OperationCleaner{
		db:        app.db,
		executor:  app.executor,
		sel:       CleanAll,
		optracker: ot,
		opClass:   TrackNormal,
	}
	e = oc.MarkStale()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// we ran the mark function, but time had not advanced. no changes
	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		pstale, e := PendingOperationEntrySelection(tx, CleanAll)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pstale) == 0,
			"expected len(pstale) == 0, got:", len(pstale))
		return nil
	})

	fakeTime += 10 // advance time by 10 seconds
	e = oc.MarkStale()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// we ran the mark function, but time had not gone far. no changes
	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		pstale, e := PendingOperationEntrySelection(tx, CleanAll)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pstale) == 0,
			"expected len(pstale) == 0, got:", len(pstale))
		return nil
	})

	fakeTime += 60 // advance time by 60 seconds
	e = oc.MarkStale()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// we ran the mark function, but time had not gone far. no changes
	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		pstale, e := PendingOperationEntrySelection(tx, CleanAll)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pstale) == 1,
			"expected len(pstale) == 1, got:", len(pstale))
		return nil
	})
}

func TestOperationCleanerBlockedByThrottle(t *testing.T) {
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

	// take control of time
	fakeTime := operationTimestamp()
	operationTimestamp = func() int64 {
		return fakeTime
	}

	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)
	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	ot := newOpTracker(3)
	oc := OperationCleaner{
		db:        app.db,
		executor:  app.executor,
		sel:       CleanAll,
		optracker: ot,
		opClass:   TrackClean,
	}
	tests.Assert(t, !ot.ThrottleOrAdd("aaa", TrackNormal))
	tests.Assert(t, !ot.ThrottleOrAdd("bbb", TrackNormal))
	tests.Assert(t, !ot.ThrottleOrAdd("ccc", TrackNormal))

	// internally, the throttle should block any cleaning
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// assert that pending volume create got cleaned up
	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	ot.Remove("aaa")
	ot.Remove("bbb")
	ot.Remove("ccc")
	tests.Assert(t, !ot.ThrottleOrAdd("xxx", TrackClean))

	// internally, the throttle should block any cleaning
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// assert that pending volume create got cleaned up
	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	ot.Remove("xxx")

	// internally, the throttle should block any cleaning
	e = oc.Clean()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// assert that pending volume create got cleaned up
	app.db.View(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
}

func TestCleanSelectedOps(t *testing.T) {
	vals := []struct {
		p *PendingOperationEntry
		r bool
	}{
		// empty content: no match
		{&PendingOperationEntry{}, false},
		{&PendingOperationEntry{
			PendingOperation: PendingOperation{
				PendingItem: PendingItem{Id: "aaaa"},
				Type:        OperationCreateVolume,
			},
			Status: StaleOperation,
		}, true},
		// match name but not status
		{&PendingOperationEntry{
			PendingOperation: PendingOperation{
				PendingItem: PendingItem{Id: "bbbb"},
				Type:        OperationCreateVolume,
			},
			Status: NewOperation,
		}, false},
		// match status but not name
		{&PendingOperationEntry{
			PendingOperation: PendingOperation{
				PendingItem: PendingItem{Id: "cccc"},
				Type:        OperationCreateVolume,
			},
			Status: StaleOperation,
		}, false},
		// match name and alt. status
		{&PendingOperationEntry{
			PendingOperation: PendingOperation{
				PendingItem: PendingItem{Id: "dddd"},
				Type:        OperationCreateVolume,
			},
			Status: FailedOperation,
		}, true},
	}

	f := CleanSelectedOps(map[string]bool{
		"aaaa": true,
		"bbbb": true,
		"dddd": true,
	})

	for i, val := range vals {
		t.Run(fmt.Sprintf("Value%v", i), func(t *testing.T) {
			result := f(val.p)
			tests.Assert(t, result == val.r,
				"result mismatch. expected", val.r,
				"got", result, "with", fmt.Sprintf("%#v", val.p))
		})
	}
}

func TestBackgroundOperationCleaner(t *testing.T) {
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
	req.Name = "vol1"
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)
	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	boc := &backgroundOperationCleaner{
		cleaner: OperationCleaner{
			db:       app.db,
			executor: app.executor,
			sel:      CleanAll,
		},
		StartInterval: 5 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}

	boc.Start()
	defer boc.Stop()

	time.Sleep(10 * time.Millisecond)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})

	req.Name = "vol2"
	vol2 := NewVolumeEntryFromRequest(req)
	vc = NewVolumeCreateOperation(vol2, app.db)
	e = vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	time.Sleep(100 * time.Millisecond)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
}

func TestBackgroundOperationCleanerWithTracking(t *testing.T) {
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
	req.Name = "vol1"
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)
	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	ot := newOpTracker(8)
	boc := &backgroundOperationCleaner{
		cleaner: OperationCleaner{
			db:        app.db,
			executor:  app.executor,
			sel:       CleanAll,
			optracker: ot,
			opClass:   TrackClean,
		},
		StartInterval: 5 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	ot.Add("FAKEFAKE", TrackClean)

	boc.Start()
	defer boc.Stop()

	time.Sleep(10 * time.Millisecond)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	req.Name = "vol2"
	vol2 := NewVolumeEntryFromRequest(req)
	vc = NewVolumeCreateOperation(vol2, app.db)
	e = vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 2, "expected len(l) == 2, got:", len(l))
		e = MarkPendingOperationsStale(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		return nil
	})

	ot.Remove("FAKEFAKE")
	time.Sleep(100 * time.Millisecond)

	app.db.Update(func(tx *bolt.Tx) error {
		l, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})
}
