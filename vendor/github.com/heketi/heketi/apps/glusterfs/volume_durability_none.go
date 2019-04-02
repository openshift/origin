//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"encoding/gob"

	"github.com/heketi/heketi/executors"
)

func init() {
	// Volume Entry has VolumeDurability interface as a member.
	// Serialization tools need to know the types that satisfy this
	// interface. gob is used to serialize entries for db. Strictly
	// speaking, it is not required to store VolumeDurability member in db
	// as it can be recreated from volumeInfo. But removing it now would
	// break backward with db.
	gob.Register(&NoneDurability{})
}

type NoneDurability struct {
	VolumeReplicaDurability
}

func NewNoneDurability() *NoneDurability {
	n := &NoneDurability{}
	n.Replica = 1

	return n
}

func (n *NoneDurability) SetDurability() {
	n.Replica = 1
}

func (n *NoneDurability) BricksInSet() int {
	return 1
}

func (n *NoneDurability) QuorumBrickCount() int {
	return n.BricksInSet()
}

func (n *NoneDurability) SetExecutorVolumeRequest(v *executors.VolumeRequest) {
	v.Type = executors.DurabilityNone
	v.Replica = n.Replica
}
