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
	"testing"

	"github.com/heketi/tests"
)

func TestPendingOperationTypeName(t *testing.T) {
	nope := PendingOperationType(9999)
	vals := []struct {
		p    PendingOperationType
		name string
	}{
		{OperationCreateVolume, "create-volume"},
		{OperationDeleteVolume, "delete-volume"},
		{OperationExpandVolume, "expand-volume"},
		{OperationCreateBlockVolume, "create-block-volume"},
		{OperationDeleteBlockVolume, "delete-block-volume"},
		{OperationRemoveDevice, "remove-device"},
		{OperationCloneVolume, "clone-volume"},
		{OperationUnknown, "unknown"},
		{nope, "unknown"},
	}

	for _, v := range vals {
		tests.Assert(t, v.name == v.p.Name(),
			"expected", v.name, "got", v.p.Name())
	}
}

func TestPendingChangeTypeName(t *testing.T) {
	nope := PendingChangeType(9999)
	vals := []struct {
		p    PendingChangeType
		name string
	}{
		{OpAddBrick, "Add brick"},
		{OpAddVolume, "Add volume"},
		{OpDeleteBrick, "Delete brick"},
		{OpDeleteVolume, "Delete volume"},
		{OpExpandVolume, "Expand volume"},
		{OpAddBlockVolume, "Add block volume"},
		{OpDeleteBlockVolume, "Delete block volume"},
		{OpRemoveDevice, "Remove device"},
		{OpCloneVolume, "Clone volume from"},
		{OpSnapshotVolume, "Snapshot volume"},
		{OpAddVolumeClone, "Expand volume to"},
		{OpUnknown, "Unknown"},
		{nope, "Unknown"},
	}

	for _, v := range vals {
		tests.Assert(t, v.name == v.p.Name(),
			"expected", v.name, "got", v.p.Name())
	}
}
