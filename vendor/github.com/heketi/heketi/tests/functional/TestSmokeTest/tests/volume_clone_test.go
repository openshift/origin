// +build functional

//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package functional

import (
	"testing"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/tests"
)

func TestCloneVolume(t *testing.T) {
	setupCluster(t, 4, 8)
	defer teardownCluster(t)

	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	vol, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	cloneReq := &api.VolumeCloneRequest{}
	clonedVol, err := heketi.VolumeClone(vol.Id, cloneReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, vol.Id != clonedVol.Id, "expected vol.Id != clonedVol.Id, got:", clonedVol.Id)

	cloneReq = &api.VolumeCloneRequest{
		Name: "my_cloned_volume",
	}
	namedClonedVol, err := heketi.VolumeClone(vol.Id, cloneReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, namedClonedVol.Name == cloneReq.Name, "expected namedClonedVol.Name == cloneReq.Name, got:", namedClonedVol.Name)
}

func TestCloneVolumeDelete(t *testing.T) {
	setupCluster(t, 4, 8)
	defer teardownCluster(t)

	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	vol, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	cloneReq := &api.VolumeCloneRequest{}
	clonedVol, err := heketi.VolumeClone(vol.Id, cloneReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, vol.Id != clonedVol.Id, "expected vol.Id != clonedVol.Id, got:", clonedVol.Id)

	err = heketi.VolumeDelete(clonedVol.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// the deleted volume should not exist anymore
	_, err = heketi.VolumeInfo(clonedVol.Id)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
}

func TestCloneBlockVolumeFails(t *testing.T) {
	setupCluster(t, 4, 8)
	defer teardownCluster(t)

	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	volReq.Block = true

	vol, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	cloneReq := &api.VolumeCloneRequest{}
	clonedVol, err := heketi.VolumeClone(vol.Id, cloneReq)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, clonedVol == nil, "expected clonedVol == nil, got:", clonedVol)
}
