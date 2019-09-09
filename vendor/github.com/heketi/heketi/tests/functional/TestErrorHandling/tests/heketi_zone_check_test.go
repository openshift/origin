// +build functional

//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

package tests

import (
	"os"
	"path"
	"testing"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/testutils"
	"github.com/heketi/heketi/server/config"
	"github.com/heketi/tests"
)

func TestVolumeCreateMultipleZone(t *testing.T) {

	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)

	// set zoneChecking  to  strict
	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.ZoneChecking = "strict"
	})
	testutils.ServerStarted(t, heketiServer)
	defer testutils.ServerStopped(t, heketiServer)

	tce := testCluster.Copy()
	tce.Update()
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	t.Run("volumeCreateSucceeds", func(t *testing.T) {
		//Make sure we are adding nodes as below
		//Node0 ---> Zone1
		//Node1 ---> Zone2
		//Node2,Node3 ---> Zone3
		tce.CustomizeNodeRequest = func(i int, req *api.NodeAddRequest) {
			if i >= 2 {
				req.Zone = 3
			} else {
				req.Zone = i + 1
			}
		}
		tce.Teardown(t)
		tce.Setup(t, 4, 4)
		defer tce.Teardown(t)
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("volumeCreateFails", func(t *testing.T) {
		//Make sure we are adding nodes as below
		//Node0 ---> Zone1
		//Node1 ---> Zone1
		//Node2,Node3 ---> Zone2
		tce.CustomizeNodeRequest = func(i int, req *api.NodeAddRequest) {
			if i >= 2 {
				req.Zone = 2
			} else {
				req.Zone = 1
			}
		}
		tce.Teardown(t)
		tce.Setup(t, 4, 4)
		defer tce.Teardown(t)
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil")
	})

	t.Run("volumeCreateIgnoreServerSetting", func(t *testing.T) {
		tce.CustomizeNodeRequest = func(i int, req *api.NodeAddRequest) {
			req.Zone = 1
		}
		tce.Teardown(t)
		tce.Setup(t, 4, 4)
		defer tce.Teardown(t)
		volReq.GlusterVolumeOptions = []string{"user.heketi.zone-checking none"}
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil")
	})
}
