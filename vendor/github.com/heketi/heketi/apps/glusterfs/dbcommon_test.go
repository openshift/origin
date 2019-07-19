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

func TestFixIncorrectBlockHostingFreeSize(t *testing.T) {
	setup := func(t *testing.T) (*App, string) {
		tmpfile := tests.Tempfile()
		defer os.Remove(tmpfile)

		// Create the app
		app := NewTestApp(tmpfile)

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

		err = RunOperation(vc, app.executor)
		tests.Assert(t, err == nil, "expected err == nil, got", err)

		// we should now have one block volume with one bhv
		var volId string
		app.db.View(func(tx *bolt.Tx) error {
			vl, e := VolumeList(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
			volId = vl[0]
			bvl, e := BlockVolumeList(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(vl))
			pol, e := PendingOperationList(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
			return nil
		})

		return app, volId
	}

	t.Run("CorrectBadFreeSize", func(t *testing.T) {
		app, volId := setup(t)
		defer app.Close()

		app.db.Update(func(tx *bolt.Tx) error {
			// first, we intentionally mess up the FreeSize
			vol, e := NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			vol.Info.BlockInfo.FreeSize = 2048
			e = vol.Save(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// now run the autocorrection function
			e = fixIncorrectBlockHostingFreeSize(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// verify it was corrected
			vol, e = NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, vol.Info.BlockInfo.FreeSize == 54,
				"expected vol.Info.BlockInfo.FreeSize == 54, got:",
				vol.Info.BlockInfo.FreeSize)
			return nil
		})
	})
	t.Run("AlreadyOk", func(t *testing.T) {
		app, volId := setup(t)
		defer app.Close()

		app.db.Update(func(tx *bolt.Tx) error {
			// we run the autocorrect func on entries that are already ok
			e := fixIncorrectBlockHostingFreeSize(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// verify it is ok
			vol, e := NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, vol.Info.BlockInfo.FreeSize == 54,
				"expected vol.Info.BlockInfo.FreeSize == 54, got:",
				vol.Info.BlockInfo.FreeSize)
			return nil
		})
	})
	t.Run("SkipTooLow", func(t *testing.T) {
		app, volId := setup(t)
		defer app.Close()

		app.db.Update(func(tx *bolt.Tx) error {
			// first, we intentionally mess up the FreeSize
			vol, e := NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			vol.Info.BlockInfo.FreeSize = 2048
			e = vol.Save(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			// also change the block volume size to something silly
			bvid := vol.Info.BlockInfo.BlockVolumes[0]
			bv, e := NewBlockVolumeEntryFromId(tx, bvid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			bv.Info.Size = 10001
			e = bv.Save(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// now run the autocorrection function
			e = fixIncorrectBlockHostingFreeSize(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// verify it was not changed
			vol, e = NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, vol.Info.BlockInfo.FreeSize == 2048,
				"expected vol.Info.BlockInfo.FreeSize == 2048, got:",
				vol.Info.BlockInfo.FreeSize)
			return nil
		})
	})
	t.Run("SkipTooHigh", func(t *testing.T) {
		app, volId := setup(t)
		defer app.Close()

		app.db.Update(func(tx *bolt.Tx) error {
			// first, we intentionally mess up the FreeSize
			vol, e := NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			vol.Info.BlockInfo.FreeSize = 2048
			// also change the reserved size to some nonsense
			vol.Info.BlockInfo.ReservedSize = -5000
			e = vol.Save(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// now run the autocorrection function
			e = fixIncorrectBlockHostingFreeSize(tx)
			tests.Assert(t, e == nil, "expected e == nil, got", e)

			// verify it was not changed
			vol, e = NewVolumeEntryFromId(tx, volId)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, vol.Info.BlockInfo.FreeSize == 2048,
				"expected vol.Info.BlockInfo.FreeSize == 2048, got:",
				vol.Info.BlockInfo.FreeSize)
			return nil
		})
	})
}

func TestFixBlockHostingReservedSize(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		2,    // devices_per_node,
		2*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	req := &api.BlockVolumeCreateRequest{}
	req.Size = 1024

	bvol := NewBlockVolumeEntryFromRequest(req)
	vc := NewBlockVolumeCreateOperation(bvol, app.db)

	err = RunOperation(vc, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got", err)

	// we should now have one block volume with one bhv
	var vol *VolumeEntry
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		vol, e = NewVolumeEntryFromId(tx, vl[0])
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		bvl, e := BlockVolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bvl) == 1, "expected len(bvl) == 1, got", len(vl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	resetVol := func(rn api.BlockRestriction, f, r int) {
		app.db.Update(func(tx *bolt.Tx) error {
			vol.Info.BlockInfo.Restriction = rn
			vol.Info.BlockInfo.FreeSize = f
			vol.Info.BlockInfo.ReservedSize = r
			return vol.Save(tx)
		})
	}

	resetBvol := func() {
		app.db.Update(func(tx *bolt.Tx) error {
			bvol.Info.Size = req.Size
			return bvol.Save(tx)
		})
	}

	assertRestrictionIs := func(t *testing.T, r api.BlockRestriction) {
		app.db.View(func(tx *bolt.Tx) error {
			v, err := NewVolumeEntryFromId(tx, vol.Info.Id)
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			tests.Assert(t, v.Info.BlockInfo.Restriction == r,
				"expected retriction", r, "got:", v.Info.BlockInfo.Restriction)
			return nil
		})
	}

	t.Run("VolumeOk", func(t *testing.T) {
		defer resetVol(
			vol.Info.BlockInfo.Restriction,
			vol.Info.BlockInfo.FreeSize,
			vol.Info.BlockInfo.ReservedSize)
		app.db.Update(func(tx *bolt.Tx) error {
			err := fixBlockHostingReservedSize(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			return nil
		})
	})
	t.Run("VolumeNeedsFixing", func(t *testing.T) {
		defer resetVol(
			vol.Info.BlockInfo.Restriction,
			vol.Info.BlockInfo.FreeSize,
			vol.Info.BlockInfo.ReservedSize)
		app.db.Update(func(tx *bolt.Tx) error {
			vol.Info.BlockInfo.FreeSize += vol.Info.BlockInfo.ReservedSize
			vol.Info.BlockInfo.ReservedSize = 0
			err := vol.Save(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)

			err = fixBlockHostingReservedSize(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			return nil
		})
		assertRestrictionIs(t, api.Unrestricted)
	})
	t.Run("VolumeInvalidSizes", func(t *testing.T) {
		defer resetVol(
			vol.Info.BlockInfo.Restriction,
			vol.Info.BlockInfo.FreeSize,
			vol.Info.BlockInfo.ReservedSize)
		app.db.Update(func(tx *bolt.Tx) error {
			vol.Info.BlockInfo.FreeSize = 0
			vol.Info.BlockInfo.ReservedSize = 0
			err := vol.Save(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)

			err = fixBlockHostingReservedSize(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			return nil
		})
		assertRestrictionIs(t, api.Unrestricted)
	})
	t.Run("VolumeAlreadyLocked", func(t *testing.T) {
		defer resetVol(
			vol.Info.BlockInfo.Restriction,
			vol.Info.BlockInfo.FreeSize,
			vol.Info.BlockInfo.ReservedSize)
		defer resetBvol()
		app.db.Update(func(tx *bolt.Tx) error {
			vol.Info.BlockInfo.FreeSize += vol.Info.BlockInfo.ReservedSize
			vol.Info.BlockInfo.ReservedSize = 0
			vol.Info.BlockInfo.Restriction = api.LockedByUpdate
			err := vol.Save(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)

			err = fixBlockHostingReservedSize(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			return nil
		})
		assertRestrictionIs(t, api.LockedByUpdate)
	})
	t.Run("VolumeCantReserve", func(t *testing.T) {
		defer resetVol(
			vol.Info.BlockInfo.Restriction,
			vol.Info.BlockInfo.FreeSize,
			vol.Info.BlockInfo.ReservedSize)
		defer resetBvol()
		app.db.Update(func(tx *bolt.Tx) error {
			vol.Info.BlockInfo.FreeSize = 0
			vol.Info.BlockInfo.ReservedSize = 0
			err := vol.Save(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			bvol.Info.Size = 1100
			err = bvol.Save(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)

			err = fixBlockHostingReservedSize(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			return nil
		})
		assertRestrictionIs(t, api.LockedByUpdate)
	})
	t.Run("Insanity", func(t *testing.T) {
		defer resetVol(
			vol.Info.BlockInfo.Restriction,
			vol.Info.BlockInfo.FreeSize,
			vol.Info.BlockInfo.ReservedSize)
		defer resetBvol()
		app.db.Update(func(tx *bolt.Tx) error {
			vol.Info.BlockInfo.FreeSize = 50
			vol.Info.BlockInfo.ReservedSize = -50
			err := vol.Save(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			bvol.Info.Size = 1100
			err = bvol.Save(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)

			err = fixBlockHostingReservedSize(tx)
			tests.Assert(t, err == nil, "expected err == nil, got", err)
			return nil
		})
		assertRestrictionIs(t, api.LockedByUpdate)
	})
}
