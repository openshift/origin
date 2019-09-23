package glusterfs

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/glusterfs/api"

	"github.com/boltdb/bolt"
	"github.com/heketi/tests"

	"github.com/gorilla/mux"
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

type testOperation struct {
	label    string
	rurl     string
	retryMax int
	build    func() error
	exec     func() error
	finalize func() error
	rollback func() error
}

func (o *testOperation) Label() string {
	return o.label
}

func (o *testOperation) ResourceUrl() string {
	return o.rurl
}

func (o *testOperation) MaxRetries() int {
	return o.retryMax
}

func (o *testOperation) Build() error {
	if o.build == nil {
		return nil
	}
	return o.build()
}

func (o *testOperation) Exec(executor executors.Executor) error {
	if o.exec == nil {
		return nil
	}
	return o.exec()
}

func (o *testOperation) Rollback(executor executors.Executor) error {
	if o.rollback == nil {
		return nil
	}
	return o.rollback()
}

func (o *testOperation) Finalize() error {
	if o.finalize == nil {
		return nil
	}
	return o.finalize()
}

func TestAsyncHttpOperationOK(t *testing.T) {
	o := &testOperation{}
	o.rurl = "/myresource"
	testAsyncHttpOperation(t, o, func(t *testing.T, url string) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		r, err := client.Get(url + "/app")
		tests.Assert(t, r.StatusCode == http.StatusAccepted)
		tests.Assert(t, err == nil)
		location, err := r.Location()
		tests.Assert(t, err == nil)

		for done := false; !done; {
			time.Sleep(time.Millisecond)
			r, err = client.Get(location.String())
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			switch r.StatusCode {
			case http.StatusSeeOther:
				location, err = r.Location()
				tests.Assert(t, err == nil, "expected err == nil, got", err)
				tests.Assert(t, strings.Contains(location.String(), "/myresource"),
					`expected trings.Contains(location.String(), "/myresource") got:`,
					location.String())
			case http.StatusOK:
				if r.ContentLength > 0 {
					body, err := ioutil.ReadAll(r.Body)
					r.Body.Close()
					tests.Assert(t, err == nil)
					tests.Assert(t, string(body) == "HelloWorld")
					done = true
				} else {
					t.Fatalf("unexpected content length %v", r.ContentLength)
				}
			default:
				t.Fatalf("unexpected http return code %v", r.StatusCode)
			}
		}
	})
}

func TestAsyncHttpOperationBuildFailure(t *testing.T) {
	o := &testOperation{}
	o.rurl = "/myresource"
	o.build = func() error {
		return fmt.Errorf("buildfail")
	}
	testAsyncHttpOperation(t, o, func(t *testing.T, url string) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		r, err := client.Get(url + "/app")
		tests.Assert(t, err == nil, "expected err == nil, got", err)
		tests.Assert(t, r.StatusCode == http.StatusInternalServerError)
	})
}

func TestAsyncHttpOperationExecFailure(t *testing.T) {
	o := &testOperation{}
	o.rurl = "/myresource"
	o.exec = func() error {
		return fmt.Errorf("execfail")
	}
	testAsyncHttpOperation(t, o, func(t *testing.T, url string) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		r, err := client.Get(url + "/app")
		tests.Assert(t, err == nil, "expected err == nil, got", err)
		tests.Assert(t, r.StatusCode == http.StatusAccepted)
		location, err := r.Location()
		tests.Assert(t, err == nil)

		for done := false; !done; {
			time.Sleep(time.Millisecond)
			r, err = client.Get(location.String())
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			switch r.StatusCode {
			case http.StatusSeeOther:
				location, err = r.Location()
				tests.Assert(t, err == nil, "expected err == nil, got", err)
				tests.Assert(t, strings.Contains(location.String(), "/myresource"),
					`expected trings.Contains(location.String(), "/myresource") got:`,
					location.String())
			case http.StatusInternalServerError:
				if r.ContentLength > 0 {
					body, err := ioutil.ReadAll(r.Body)
					r.Body.Close()
					tests.Assert(t, err == nil)
					s := string(body)
					tests.Assert(t, strings.Contains(s, "execfail"),
						`expected strings.Contains(s, "execfail"), got:`, s)
					done = true
				} else {
					t.Fatalf("unexpected content length %v", r.ContentLength)
				}
			default:
				t.Fatalf("unexpected http return code %v", r.StatusCode)
			}
		}
	})
}

func TestAsyncHttpOperationRollbackFailure(t *testing.T) {
	o := &testOperation{}
	o.rurl = "/myresource"
	o.exec = func() error {
		return fmt.Errorf("execfail")
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return fmt.Errorf("rollbackfail")
	}
	testAsyncHttpOperation(t, o, func(t *testing.T, url string) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		r, err := client.Get(url + "/app")
		tests.Assert(t, err == nil, "expected err == nil, got", err)
		tests.Assert(t, r.StatusCode == http.StatusAccepted)
		location, err := r.Location()
		tests.Assert(t, err == nil)

		for done := false; !done; {
			time.Sleep(time.Millisecond)
			r, err = client.Get(location.String())
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			switch r.StatusCode {
			case http.StatusSeeOther:
				location, err = r.Location()
				tests.Assert(t, err == nil, "expected err == nil, got", err)
				tests.Assert(t, strings.Contains(location.String(), "/myresource"),
					`expected trings.Contains(location.String(), "/myresource") got:`,
					location.String())
			case http.StatusInternalServerError:
				if r.ContentLength > 0 {
					body, err := ioutil.ReadAll(r.Body)
					r.Body.Close()
					tests.Assert(t, err == nil)
					s := string(body)
					tests.Assert(t, strings.Contains(s, "execfail"),
						`expected strings.Contains(s, "execfail"), got:`, s)
					done = true
				} else {
					t.Fatalf("unexpected content length %v", r.ContentLength)
				}
			default:
				t.Fatalf("unexpected http return code %v", r.StatusCode)
			}
		}
	})
	tests.Assert(t, rollback_cc == 1, "expected rollback_cc == 1, got:", rollback_cc)
}

func TestAsyncHttpOperationFinalizeFailure(t *testing.T) {
	o := &testOperation{}
	o.rurl = "/myresource"
	o.finalize = func() error {
		return fmt.Errorf("finfail")
	}
	testAsyncHttpOperation(t, o, func(t *testing.T, url string) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		r, err := client.Get(url + "/app")
		tests.Assert(t, err == nil, "expected err == nil, got", err)
		tests.Assert(t, r.StatusCode == http.StatusAccepted)
		location, err := r.Location()
		tests.Assert(t, err == nil)

		for done := false; !done; {
			time.Sleep(time.Millisecond)
			r, err = client.Get(location.String())
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			switch r.StatusCode {
			case http.StatusSeeOther:
				location, err = r.Location()
				tests.Assert(t, err == nil, "expected err == nil, got", err)
				tests.Assert(t, strings.Contains(location.String(), "/myresource"),
					`expected trings.Contains(location.String(), "/myresource") got:`,
					location.String())
			case http.StatusInternalServerError:
				if r.ContentLength > 0 {
					body, err := ioutil.ReadAll(r.Body)
					r.Body.Close()
					tests.Assert(t, err == nil)
					s := string(body)
					tests.Assert(t, strings.Contains(s, "finfail"),
						`expected strings.Contains(s, "finfail"), got:`, s)
					done = true
				} else {
					t.Fatalf("unexpected content length %v", r.ContentLength)
				}
			default:
				t.Fatalf("unexpected http return code %v", r.StatusCode)
			}
		}
	})
}

func testAsyncHttpOperation(t *testing.T,
	o Operation,
	testFunc func(*testing.T, string)) {

	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc("/queue/{id}", app.asyncManager.HandlerStatus).Methods("GET")
	router.HandleFunc("/myresource", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "HelloWorld")
	}).Methods("GET")

	router.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		if x := AsyncHttpOperation(app, w, r, o); x != nil {
			http.Error(w, x.Error(), http.StatusInternalServerError)
		}
	}).Methods("GET")

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	testFunc(t, ts.URL)
}

func TestRunOperationRollbackFailure(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{}
	o.rurl = "/myresource"
	o.exec = func() error {
		return fmt.Errorf("execfail")
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return fmt.Errorf("rollbackfail")
	}
	e := RunOperation(o, app.executor)
	// even if rollback fails we expect the error from Exec
	tests.Assert(t, strings.Contains(e.Error(), "execfail"),
		`expected strings.Contains(e.Error(), "execfail"), got:`, e)
	// check that rollback got called
	tests.Assert(t, rollback_cc == 1,
		"expected rollback_cc == 1, got:", rollback_cc)
}

func TestRunOperationFinalizeFailure(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{}
	o.label = "Funky Fresh"
	o.rurl = "/myresource"
	o.finalize = func() error {
		return fmt.Errorf("finfail")
	}

	e := RunOperation(o, app.executor)
	// check error from finalize
	tests.Assert(t, strings.Contains(e.Error(), "finfail"),
		`expected strings.Contains(e.Error(), "finfail"), got:`, e)
}

func TestRunOperationExecRetryError(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{label: "X"}
	o.rurl = "/myresource"
	o.retryMax = 4
	o.exec = func() error {
		return OperationRetryError{
			OriginalError: fmt.Errorf("foobar"),
		}
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return nil
	}
	e := RunOperation(o, app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	// even if rollback fails we expect the error from Exec
	tests.Assert(t, strings.Contains(e.Error(), "foobar"),
		`expected strings.Contains(e.Error(), "foobar"), got:`, e)
	// check that rollback got called
	tests.Assert(t, rollback_cc == 5,
		"expected rollback_cc == 5, got:", rollback_cc)
}

func TestRunOperationExecRetryRollbackFail(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{label: "X"}
	o.rurl = "/myresource"
	o.retryMax = 4
	o.exec = func() error {
		return OperationRetryError{
			OriginalError: fmt.Errorf("foobar"),
		}
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return fmt.Errorf("rollbackfail")
	}
	build_cc := 0
	o.build = func() error {
		build_cc++
		return nil
	}
	e := RunOperation(o, app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	// even if rollback fails we expect the error from Exec
	tests.Assert(t, strings.Contains(e.Error(), "rollbackfail"),
		`expected strings.Contains(e.Error(), "rollbackfail"), got:`, e)
	// check that rollback got called
	tests.Assert(t, rollback_cc == 1,
		"expected rollback_cc == 1, got:", rollback_cc)
	tests.Assert(t, build_cc == 1,
		"expected build_cc == 1, got:", build_cc)
}

func TestRunOperationExecRetryThenBuildFail(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{label: "X"}
	o.rurl = "/myresource"
	o.retryMax = 4
	o.exec = func() error {
		return OperationRetryError{
			OriginalError: fmt.Errorf("foobar"),
		}
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return nil
	}
	build_cc := 0
	o.build = func() error {
		build_cc++
		if build_cc > 1 {
			return fmt.Errorf("buildfail")
		}
		return nil
	}
	e := RunOperation(o, app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	// even if rollback fails we expect the error from Exec
	tests.Assert(t, strings.Contains(e.Error(), "buildfail"),
		`expected strings.Contains(e.Error(), "buildfail"), got:`, e)
	// check that rollback got called
	tests.Assert(t, rollback_cc == 1,
		"expected rollback_cc == 1, got:", rollback_cc)
	tests.Assert(t, build_cc == 2,
		"expected build_cc == 2, got:", build_cc)
}

func TestRunOperationExecRetryThenSucceed(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{label: "X"}
	o.rurl = "/myresource"
	o.retryMax = 4
	exec_cc := 0
	o.exec = func() error {
		exec_cc++
		if exec_cc > 2 {
			return nil
		}
		return OperationRetryError{
			OriginalError: fmt.Errorf("foobar"),
		}
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return nil
	}
	e := RunOperation(o, app.executor)
	// even if rollback fails we expect the error from Exec
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	tests.Assert(t, rollback_cc == 2,
		"expected rollback_cc == 2, got:", rollback_cc)
}

func TestRunOperationExecRetryThenNonRetryError(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)

	o := &testOperation{label: "X"}
	o.rurl = "/myresource"
	o.retryMax = 4
	exec_cc := 0
	o.exec = func() error {
		exec_cc++
		if exec_cc > 2 {
			return fmt.Errorf("execfail")
		}
		return OperationRetryError{
			OriginalError: fmt.Errorf("foobar"),
		}
	}
	rollback_cc := 0
	o.rollback = func() error {
		rollback_cc++
		return nil
	}
	e := RunOperation(o, app.executor)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	// even if rollback fails we expect the error from Exec
	tests.Assert(t, strings.Contains(e.Error(), "execfail"),
		`expected strings.Contains(e.Error(), "execfail"), got:`, e)
	// check that rollback got called
	tests.Assert(t, rollback_cc == 3,
		"expected rollback_cc == 3, got:", rollback_cc)
}

func TestExpandSizeFromOp(t *testing.T) {
	op := NewPendingOperationEntry("jjjj")
	op.Actions = append(op.Actions, PendingOperationAction{
		Change: OpExpandVolume,
		Id:     "foofoofoo",
		Delta:  495,
	})
	// this op lacks the expand metadata, should return error
	v, e := expandSizeFromOp(op)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)
	tests.Assert(t, v == 495, "expected v == 495, got:", v)
}

func TestExpandSizeFromOpErrorHandling(t *testing.T) {
	op := NewPendingOperationEntry("jjjj")
	// this op lacks the expand metadata, should return error
	_, e := expandSizeFromOp(op)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	tests.Assert(t, strings.Contains(e.Error(), "no OpExpandVolume action"),
		`expected strings.Contains(e.Error(), "no OpExpandVolume action"), got:`,
		e)
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

func TestAppServerResetStaleOps(t *testing.T) {
	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)

	// create a app that will only be used to set up the test
	app := NewTestApp(dbfile)
	tests.Assert(t, app != nil)

	// pretend first server startup
	err := app.ServerReset()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		1*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// create pending operations that we will "orphan"
	req := &api.VolumeCreateRequest{}
	req.Size = 1

	vol1 := NewVolumeEntryFromRequest(req)
	vc1 := NewVolumeCreateOperation(vol1, app.db)
	err = vc1.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	vol2 := NewVolumeEntryFromRequest(req)
	vc2 := NewVolumeCreateOperation(vol2, app.db)
	err = vc2.Build()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 2, "expected len(pol) == 2, got", len(pol))
		for _, poid := range pol {
			po, e := NewPendingOperationEntryFromId(tx, poid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, po.Status == NewOperation,
				"expected po.Status == NewOperation, got:", po.Status)
		}
		return nil
	})

	// pretend the server was restarted
	err = app.ServerReset()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	app.db.View(func(tx *bolt.Tx) error {
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 2, "expected len(pol) == 2, got", len(pol))
		for _, poid := range pol {
			po, e := NewPendingOperationEntryFromId(tx, poid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, po.Status == StaleOperation,
				"expected po.Status == NewOperation, got:", po.Status)
		}
		return nil
	})
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
