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

	"github.com/heketi/tests"

	"github.com/heketi/heketi/pkg/glusterfs/api"
)

func TestLoadOperation(t *testing.T) {
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

	t.Run("invalidOp", func(t *testing.T) {
		p := NewPendingOperationEntry("invalid")
		_, e := LoadOperation(app.db, p)
		tests.Assert(t, e != nil, "expected e != nil, got:", e)
		_, ok := e.(ErrNotLoadable)
		tests.Assert(t, ok, "expected e to be NewErrNotLoadable")
		tests.Assert(t, strings.Contains(e.Error(), "invalid"),
			`expected strings.Contains(e.Error(), "invalid"), got:`,
			e.Error())
	})
	t.Run("volume create", func(t *testing.T) {
		req := &api.VolumeCreateRequest{}
		req.Size = 1024
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		vol := NewVolumeEntryFromRequest(req)
		vc := NewVolumeCreateOperation(vol, app.db)
		e := vc.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		o, e := LoadOperation(app.db, vc.op)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		vc2, ok := o.(*VolumeCreateOperation)
		tests.Assert(t, ok, "expected e to be VolumeCreateOperation")
		tests.Assert(t, vc2.op.Id == vc.op.Id,
			"expected vc2.op.Id == vc.op.Id, got:", vc2.op.Id, vc.op.Id)
		tests.Assert(t, vc2.vol.Info.Id == vc.vol.Info.Id,
			"expected vc2.vol.Info.Id == vc.vol.Info.Id")
		tests.Assert(t, vc2.vol != vc.vol,
			"expected vc2.vol != vc.vol")
	})
	t.Run("volume delete", func(t *testing.T) {
		req := &api.VolumeCreateRequest{}
		req.Size = 1024
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		vol := NewVolumeEntryFromRequest(req)
		vc := NewVolumeCreateOperation(vol, app.db)
		e := RunOperation(vc, app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		vdel := NewVolumeDeleteOperation(vol, app.db)
		e = vdel.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		o, e := LoadOperation(app.db, vdel.op)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		vdel2, ok := o.(*VolumeDeleteOperation)
		tests.Assert(t, ok, "expected e to be VolumeDeleteOperation")
		tests.Assert(t, vdel2.op.Id == vdel.op.Id,
			"expected vdel2.op.Id == vdel.op.Id, got", vdel2.op.Id, vdel2.op.Id)
		tests.Assert(t, vol.Info.Id == vdel2.vol.Info.Id,
			"expected vol.Info.Id == vdel2.vol.Info.Id")
		tests.Assert(t, vdel2.vol != vdel.vol,
			"expected vdel2.vol != vdel.vol")
	})
	t.Run("volume expand", func(t *testing.T) {
		req := &api.VolumeCreateRequest{}
		req.Size = 1024
		req.Durability.Type = api.DurabilityReplicate
		req.Durability.Replicate.Replica = 3
		vol := NewVolumeEntryFromRequest(req)
		vc := NewVolumeCreateOperation(vol, app.db)
		e := RunOperation(vc, app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		ve := NewVolumeExpandOperation(vol, app.db, 6)
		e = ve.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		o, e := LoadOperation(app.db, ve.op)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		ve2, ok := o.(*VolumeExpandOperation)
		tests.Assert(t, ok, "expected e to be VolumExpandOperation")
		tests.Assert(t, ve2.op.Id == ve.op.Id,
			"expected ve2.op.Id == ve.op.Id, got", ve2.op.Id, ve2.op.Id)
		tests.Assert(t, vol.Info.Id == ve2.vol.Info.Id,
			"expected vol.Info.Id == ve2.vol.Info.Id")
		tests.Assert(t, ve2.vol != ve.vol,
			"expected ve2.vol != ve.vol")
	})
	t.Run("block volume create", func(t *testing.T) {
		req := &api.BlockVolumeCreateRequest{}
		req.Size = 1024
		vol := NewBlockVolumeEntryFromRequest(req)
		vc := NewBlockVolumeCreateOperation(vol, app.db)
		e := vc.Build()
		// need to roll back the bhv create in order to test other ops later
		defer vc.Rollback(app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		o, e := LoadOperation(app.db, vc.op)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		vc2, ok := o.(*BlockVolumeCreateOperation)
		tests.Assert(t, ok, "expected e to be BlockVolumeCreateOperation")
		tests.Assert(t, vc2.op.Id == vc.op.Id,
			"expected vc2.op.Id == vc.op.Id, got:", vc2.op.Id, vc.op.Id)
		tests.Assert(t, vc2.bvol.Info.Id == vc.bvol.Info.Id,
			"expected vc2.vol.Info.Id == vc.vol.Info.Id")
		tests.Assert(t, vc2.bvol != vc.bvol,
			"expected vc2.vol != vc.vol")
	})
	t.Run("block volume delete", func(t *testing.T) {
		req := &api.BlockVolumeCreateRequest{}
		req.Size = 1024
		vol := NewBlockVolumeEntryFromRequest(req)
		vc := NewBlockVolumeCreateOperation(vol, app.db)
		e := RunOperation(vc, app.executor)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		vdel := NewBlockVolumeDeleteOperation(vol, app.db)
		e = vdel.Build()
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		o, e := LoadOperation(app.db, vdel.op)
		tests.Assert(t, e == nil, "expected e == nil, got:", e)
		vdel2, ok := o.(*BlockVolumeDeleteOperation)
		tests.Assert(t, ok, "expected e to be BlockVolumeDeleteOperation")
		tests.Assert(t, vdel2.op.Id == vdel.op.Id,
			"expected vdel2.op.Id == vdel.op.Id, got", vdel2.op.Id, vdel2.op.Id)
		tests.Assert(t, vol.Info.Id == vdel2.bvol.Info.Id,
			"expected vol.Info.Id == vdel2.vol.Info.Id")
		tests.Assert(t, vdel2.bvol != vdel.bvol,
			"expected vdel2.vol != vdel.vol")
	})
}
