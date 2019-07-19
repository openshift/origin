//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package paths

import (
	"testing"

	"github.com/heketi/tests"
)

func TestVgIdToName(t *testing.T) {
	expected := "vg_asdf"
	result := VgIdToName("asdf")
	tests.Assert(t, expected == result,
		`calling VgIdToName("asdf"), expected`, expected, "got", result)
}

func TestBrickIdToName(t *testing.T) {
	expected := "brick_fireplace"
	result := BrickIdToName("fireplace")
	tests.Assert(t, expected == result,
		`calling BrickIdToName("fireplace"), expected`, expected, "got", result)
}

func TestBrickIdToThinPoolName(t *testing.T) {
	expected := "tp_123456"
	result := BrickIdToThinPoolName("123456")
	tests.Assert(t, expected == result,
		`calling BrickIdToThinPoolName("123456"), expected`, expected, "got", result)
}

func TestBrickMountPoint(t *testing.T) {
	expected := "/var/lib/heketi/mounts/vg_asdf/brick_fireplace"
	result := BrickMountPoint("asdf", "fireplace")
	tests.Assert(t, expected == result,
		`calling BrickMountPoint("asdf", "fireplace"), expected`,
		expected, "got", result)
}

func TestBrickMountPointParent(t *testing.T) {
	expected := "/var/lib/heketi/mounts/vg_asdf"
	result := BrickMountPointParent("asdf")
	tests.Assert(t, expected == result,
		`calling BrickMountPointParent("asdf"), expected`,
		expected, "got", result)
}

func TestBrickThinLvName(t *testing.T) {
	expected := "vg_asdf/tp_fireplace"
	result := BrickThinLvName("asdf", "fireplace")
	tests.Assert(t, expected == result,
		`calling BrickThinLvName("asdf", "fireplace"), expected`,
		expected, "got", result)
}

func TestBrickDevNode(t *testing.T) {
	expected := "/dev/mapper/vg_asdf-brick_fireplace"
	result := BrickDevNode("asdf", "fireplace")
	tests.Assert(t, expected == result,
		`calling BrickDevNode("asdf", "fireplace"), expected`,
		expected, "got", result)
}

func TestBrickMountFromPath(t *testing.T) {
	p := "/var/lib/heketi/mounts/vg_asdf/brick_fireplace/brick"
	expected := "/var/lib/heketi/mounts/vg_asdf/brick_fireplace"
	result := BrickMountFromPath(p)
	tests.Assert(t, expected == result,
		"expected", expected, "got", result)

	tests.Assert(t,
		BrickMountPoint("abc", "def") == BrickMountFromPath(BrickPath("abc", "def")),
		`expected BrickMountPoint("abc", "def") == BrickMountFromPath(BrickPath("abc", "def")), got:`,
		BrickMountPoint("abc", "def"), BrickMountFromPath(BrickPath("abc", "def")))
}

func TestBrickMountFromPathIsStrict(t *testing.T) {
	defer func() {
		err := recover()
		tests.Assert(t, err != nil, "epxected err != nil")
	}()
	BrickMountFromPath("asdf")
	t.Fatalf("should not be reached")
}
