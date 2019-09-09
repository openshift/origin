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

	_ "github.com/boltdb/bolt"
	"github.com/heketi/tests"

	_ "github.com/heketi/heketi/pkg/db"
)

func TestNewDeviceZoneMap(t *testing.T) {
	dzm := NewDeviceZoneMap()
	dzm.Add("foobar", 1)
	dzm.Add("beep", 1)
	dzm.Add("blap", 2)

	tests.Assert(t, len(dzm.AvailableZones) == 2)
	tests.Assert(t, len(dzm.DeviceZones) == 3)
}

func TestNewDeviceZoneMapFromDb(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		1*TB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	dzm, err := NewDeviceZoneMapFromDb(app.db)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	tests.Assert(t, len(dzm.AvailableZones) == 3,
		"expected len(dzm.AvailableZones) == 3, got:", len(dzm.AvailableZones))
	tests.Assert(t, len(dzm.DeviceZones) == 9,
		"expected len(dzm.DeviceZones) == 9, got:", len(dzm.DeviceZones))
}

func TestDeviceZoneMapFilter(t *testing.T) {

	bs := NewBrickSet(3)
	dzm := NewDeviceZoneMap()
	dzm.Add("aaa", 1)
	dzm.Add("bbb", 2)
	dzm.Add("ccc", 3)
	dzm.Add("ddd", 1)
	dzm.Add("eee", 2)
	dzm.Add("fff", 3)

	d := NewDeviceEntry()
	d.Info.Id = "aaa"
	result := dzm.Filter(bs, d)
	tests.Assert(t, result, "expected result true")

	b := NewBrickEntry(5, 100, 100, "eee", "xxx", 0, "foo")
	bs.Add(b)

	d.Info.Id = "bbb"
	result = dzm.Filter(bs, d)
	tests.Assert(t, !result, "expected result false")

	d.Info.Id = "ccc"
	result = dzm.Filter(bs, d)
	tests.Assert(t, result, "expected result true")

	// insert a bad brick
	b = NewBrickEntry(5, 100, 100, "qqqqqq", "xxx", 0, "foo")
	bs.Add(b)
	result = dzm.Filter(bs, d)
	tests.Assert(t, !result, "expected result false")
}
