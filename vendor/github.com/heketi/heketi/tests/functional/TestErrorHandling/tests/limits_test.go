// +build functional

//
// Copyright (c) 2019 The heketi Authors
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

	"github.com/heketi/tests"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/testutils"
	"github.com/heketi/heketi/server/config"
)

func TestMaxVolumesPerCluster(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(origConf, heketiServer.ConfPath)

	defer func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 3)

	t.Run("LimitFiveVolumes", func(t *testing.T) {
		defer testCluster.VolumeTeardown(t)
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.MaxVolumesPerCluster = 5
		})
		testutils.ServerRestarted(t, heketiServer)

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 1
		for i := 0; i < 5; i++ {
			_, err := heketi.VolumeCreate(volReq)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}

		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	})
	t.Run("LimitSixVolumes", func(t *testing.T) {
		defer testCluster.VolumeTeardown(t)
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.MaxVolumesPerCluster = 6
		})
		testutils.ServerRestarted(t, heketiServer)

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 1
		for i := 0; i < 5; i++ {
			_, err := heketi.VolumeCreate(volReq)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}

		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
	t.Run("UnlimitedVolumes", func(t *testing.T) {
		defer testCluster.VolumeTeardown(t)
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.MaxVolumesPerCluster = -1
		})
		testutils.ServerRestarted(t, heketiServer)

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 1
		// OK, so ten volumes isn't exactly "unlimited" but we're really not
		// going to take the time to create thousands of volumes.  this is just
		// checking that negative number does something sane
		for i := 0; i < 10; i++ {
			_, err := heketi.VolumeCreate(volReq)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}
	})
}
