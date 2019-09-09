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
	"strings"
	"testing"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"
)

func TestDeviceRemoveOperationEmpty(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// grab a device
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
			break
		}
		return nil
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// because there are no bricks on this device it can be disabled
	// instantly and there are no pending ops for it in the db
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})

	err = dro.Exec(app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = dro.Finalize()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestDeviceRemoveOperationWithBricks(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create two volumes
	for i := 0; i < 5; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a devices that has bricks
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

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// because there were bricks on this device it needs to perform
	// a full "operation cycle"
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	err = dro.Exec(app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// operation is not over. we should still have a pending op
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	err = dro.Finalize()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// operation is over. we should _not_ have a pending op now
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	// our d should be w/o bricks and be in failed state
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) == 0,
		"expected len(d.Bricks) == 0, got:", len(d.Bricks))
	tests.Assert(t, d.State == api.EntryStateFailed)
}

func TestDeviceRemoveOperationTooFewDevices(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create two volumes
	for i := 0; i < 5; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a devices that has bricks
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

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// because there were bricks on this device it needs to perform
	// a full "operation cycle"
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	app.xo.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return mockVolumeInfoFromDb(app.db, volume)
	}
	app.xo.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return mockHealStatusFromDb(app.db, volume)
	}

	err = dro.Exec(app.executor)
	tests.Assert(t, strings.Contains(err.Error(), ErrNoReplacement.Error()),
		"expected strings.Contains(err.Error(), ErrNoReplacement.Error()), got:",
		err.Error())

	// operation is not over. we should still have a pending op
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	err = dro.Rollback(app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// operation is over. we should _not_ have a pending op now
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 0, "expected len(l) == 0, got:", len(l))
		return nil
	})

	// update d from db
	err = app.db.View(func(tx *bolt.Tx) error {
		d, err = NewDeviceEntryFromId(tx, d.Info.Id)
		return err
	})
	// our d should be in the original state because the exec failed
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(d.Bricks) > 0,
		"expected len(d.Bricks) > 0, got:", len(d.Bricks))
	tests.Assert(t, d.State == api.EntryStateOffline)
}

func TestDeviceRemoveOperationOtherPendingOps(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create two volumes
	for i := 0; i < 4; i++ {
		v := NewVolumeEntryFromRequest(vreq)
		err = v.Create(app.db, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// grab a devices that has bricks
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

	// now start a volume create operation but don't finish it
	vol := NewVolumeEntryFromRequest(vreq)
	vc := NewVolumeCreateOperation(vol, app.db)
	err = vc.Build()
	tests.Assert(t, err == nil, "expected e == nil, got", err)
	// we should have one pending operation
	err = app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

	err = d.SetState(app.db, app.executor, api.EntryStateOffline)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dro := NewDeviceRemoveOperation(d.Info.Id, app.db)
	err = dro.Build()
	tests.Assert(t, err == ErrConflict, "expected err == ErrConflict, got:", err)

	// we should have one pending operation (the volume create)
	app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})
}

// TestDeviceRemoveOperationMultipleRequests tests that
// the system fails gracefully if a remove device request
// comes in while an existing operation is already in progress.
func TestDeviceRemoveOperationMultipleRequests(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		8*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vreq := &api.VolumeCreateRequest{}
	vreq.Size = 100
	vreq.Durability.Type = api.DurabilityReplicate
	vreq.Durability.Replicate.Replica = 3
	// create volumes
	for i := 0; i < 4; i++ {
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

	// perform the build step of one remove operation
	dro := NewDeviceRemoveOperation(d.Info.Id, app.db)
	err = dro.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// perform the build step of a 2nd remove operation
	// we can "fake' it this way in a test because the transactions
	// that cover the Build steps are effectively serializing
	// these actions.
	dro2 := NewDeviceRemoveOperation(d.Info.Id, app.db)
	err = dro2.Build()
	tests.Assert(t, err == ErrConflict, "expected err == ErrConflict, got:", err)

	// we should have one pending operation (the device remove)
	app.db.View(func(tx *bolt.Tx) error {
		l, err := PendingOperationList(tx)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l) == 1, "expected len(l) == 1, got:", len(l))
		return nil
	})

}
