// +build functional

//
// Copyright (c) 2019 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//
package functional

import (
	"fmt"
	"testing"

	"github.com/heketi/tests"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/testutils"
)

func TestHeketiVolumeSnapshotBehavior(t *testing.T) {
	na := testutils.RequireNodeAccess(t)

	// Setup the VM storage topology
	teardownCluster(t)
	setupCluster(t, 4, 8)
	defer teardownCluster(t)

	// Create a volume
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1024
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	volReq.Snapshot.Enable = true
	volReq.Snapshot.Factor = 1.5

	volInfo, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, err)

	// SSH into system and execute gluster command to create a snapshot
	exec := na.Use(logger)
	cmd := []string{
		fmt.Sprintf("sudo gluster --mode=script snapshot create mysnap %v no-timestamp", volInfo.Name),
		"sudo gluster --mode=script snapshot activate mysnap",
	}
	_, err = exec.ConnectAndExec(cenv.SshHost(0), cmd, 10, true)
	tests.Assert(t, err == nil, err)

	// Try to delete the volume
	err = heketi.VolumeDelete(volInfo.Id)
	tests.Assert(t, err != nil, err)

	// Now clean up the snapshot
	cmd = []string{
		"sudo gluster --mode=script snapshot deactivate mysnap",
		"sudo gluster --mode=script snapshot delete mysnap",
	}
	_, err = exec.ConnectAndExec(cenv.SshHost(0), cmd, 10, true)
	tests.Assert(t, err == nil, err)

	// Try to delete the volume
	err = heketi.VolumeDelete(volInfo.Id)
	tests.Assert(t, err == nil, err)
}
