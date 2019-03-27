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
	"github.com/heketi/heketi/pkg/glusterfs/api"
)

func init() {
	// Volume Entry has VolumeDurability interface as a member.
	// Serialization tools need to know the types that satisfy this
	// interface. gob is used to serialize entries for db. Strictly
	// speaking, it is not required to store VolumeDurability member in db
	// as it can be recreated from volumeInfo. But removing it now would
	// break backward with db.
	gob.Register(&VolumeReplicaDurability{})
}

type VolumeReplicaDurability struct {
	api.ReplicaDurability
}

func NewVolumeReplicaDurability(r *api.ReplicaDurability) *VolumeReplicaDurability {
	v := &VolumeReplicaDurability{}
	v.Replica = r.Replica

	return v
}

func (r *VolumeReplicaDurability) SetDurability() {
	if r.Replica == 0 {
		r.Replica = DEFAULT_REPLICA
	}
}

func (r *VolumeReplicaDurability) BrickSizeGenerator(size uint64) func() (int, uint64, error) {

	sets := 1
	return func() (int, uint64, error) {

		var brick_size uint64
		var num_sets int

		for {
			num_sets = sets
			sets *= 2
			brick_size = size / uint64(num_sets)

			if brick_size < BrickMinSize {
				return 0, 0, ErrMinimumBrickSize
			} else if brick_size <= BrickMaxSize {
				break
			}
		}

		return num_sets, brick_size, nil
	}
}

func (r *VolumeReplicaDurability) MinVolumeSize() uint64 {
	return BrickMinSize
}

func (r *VolumeReplicaDurability) BricksInSet() int {
	return r.Replica
}

func (r *VolumeReplicaDurability) QuorumBrickCount() int {
	return r.BricksInSet()/2 + 1
}

func (r *VolumeReplicaDurability) SetExecutorVolumeRequest(v *executors.VolumeRequest) {
	v.Type = executors.DurabilityReplica
	v.Replica = r.Replica
}
