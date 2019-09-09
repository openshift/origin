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
	"github.com/boltdb/bolt"

	"github.com/heketi/heketi/pkg/glusterfs/api"
)

// ListCompleteVolumes returns a list of volume ID strings for volumes
// that are not pending.
func ListCompleteVolumes(tx *bolt.Tx) ([]string, error) {
	p, err := MapPendingVolumes(tx)
	if err != nil {
		return []string{}, err
	}
	v, err := VolumeList(tx)
	if err != nil {
		return []string{}, err
	}
	if len(p) == 0 {
		// avoid extra copy loop
		return v, nil
	}
	return removeKeysFromList(v, p), nil
}

// ListCompleteBlockVolumes returns a list of block volume ID strings for
// block volumes that are not pending.
func ListCompleteBlockVolumes(tx *bolt.Tx) ([]string, error) {
	p, err := MapPendingBlockVolumes(tx)
	if err != nil {
		return []string{}, err
	}
	v, err := BlockVolumeList(tx)
	if err != nil {
		return []string{}, err
	}
	if len(p) == 0 {
		// avoid extra copy loop
		return v, nil
	}
	return removeKeysFromList(v, p), nil
}

// UpdateVolumeInfoComplete updates the given VolumeInfoResponse object so
// that it only contains references to complete block volumes.
func UpdateVolumeInfoComplete(tx *bolt.Tx, vi *api.VolumeInfoResponse) error {
	pblk, err := MapPendingBlockVolumes(tx)
	if err != nil {
		return err
	}

	if len(pblk) > 0 {
		vi.BlockInfo.BlockVolumes = removeKeysFromList(vi.BlockInfo.BlockVolumes, pblk)
	}
	return nil
}

// UpdateClusterInfoComplete updates the given ClusterInfoResponse object so
// that it only contains references to complete volumes, etc.
func UpdateClusterInfoComplete(tx *bolt.Tx, ci *api.ClusterInfoResponse) error {
	pvol, err := MapPendingVolumes(tx)
	if err != nil {
		return err
	}
	pblk, err := MapPendingBlockVolumes(tx)
	if err != nil {
		return err
	}

	if len(pvol) > 0 {
		ci.Volumes = removeKeysFromList(ci.Volumes, pvol)
	}
	if len(pblk) > 0 {
		ci.BlockVolumes = removeKeysFromList(ci.BlockVolumes, pblk)
	}
	return nil
}

// MapPendingVolumes returns a map of volume-id to pending-op-id or
// an error if the db cannot be read.
func MapPendingVolumes(tx *bolt.Tx) (map[string]string, error) {
	return mapPendingItems(tx, func(op *PendingOperationEntry, a PendingOperationAction) bool {
		t := op.Type
		c := a.Change
		return ((t == OperationCreateVolume && c == OpAddVolume) ||
			(t == OperationDeleteVolume && c == OpDeleteVolume) ||
			(t == OperationCreateBlockVolume && c == OpAddVolume) ||
			(t == OperationCloneVolume && c == OpAddVolumeClone))
	})
}

// MapPendingBlockVolumes returns a map of block-volume-id to pending-op-id or
// an error if the db cannot be read.
func MapPendingBlockVolumes(tx *bolt.Tx) (map[string]string, error) {
	return mapPendingItems(tx, func(op *PendingOperationEntry, a PendingOperationAction) bool {
		t := op.Type
		c := a.Change
		return ((t == OperationCreateBlockVolume && c == OpAddBlockVolume) ||
			(t == OperationDeleteBlockVolume && c == OpDeleteBlockVolume))
	})
}

// MapPendingBricks returns a map of brick-id to pending-op-id or
// an error if the db cannot be read.
func MapPendingBricks(tx *bolt.Tx) (map[string]string, error) {
	return mapPendingItems(tx, func(op *PendingOperationEntry, a PendingOperationAction) bool {
		return (a.Change == OpAddBrick)
	})
}

func MapPendingDeviceRemoves(tx *bolt.Tx) (map[string]string, error) {
	return mapPendingItems(tx, func(op *PendingOperationEntry, a PendingOperationAction) bool {
		return (a.Change == OpRemoveDevice)
	})
}

func mapPendingItems(tx *bolt.Tx,
	pred func(op *PendingOperationEntry, a PendingOperationAction) bool) (
	items map[string]string, e error) {

	items = map[string]string{}
	ids, e := PendingOperationList(tx)
	if e != nil {
		return
	}
	for _, opId := range ids {
		op, err := NewPendingOperationEntryFromId(tx, opId)
		if err != nil {
			e = err
			return
		}
		for _, a := range op.Actions {
			if pred(op, a) {
				items[a.Id] = op.Id
			}
		}
	}
	return
}

// removeKeysFromList returns a new list of strings where all strings
// found as a key in map m are removed from the output list.
func removeKeysFromList(l []string, m map[string]string) []string {
	out := []string{}
	for _, v := range l {
		if _, has := m[v]; !has {
			out = append(out, v)
		}
	}
	return out
}
