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
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/heketi/tests"

	//inj "github.com/heketi/heketi/executors/injectexec"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	rex "github.com/heketi/heketi/pkg/remoteexec"
	"github.com/heketi/heketi/pkg/testutils"
	//"github.com/heketi/heketi/server/config"
)

func TestUpdateDbVol(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)
	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(origConf, heketiServer.ConfPath)

	fullTeardown := func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}

	partialTeardown := func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.VolumeTeardown(t)
	}

	defer fullTeardown()
	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 3)

	t.Run("plainVolume", func(t *testing.T) {
		defer partialTeardown()

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 2
		volReq.Durability.Type = api.DurabilityReplicate
		res, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, res.Name != "")

		testutils.ServerStopped(t, heketiServer)
		err = heketiServer.RunOfflineCmd(
			[]string{"offline", "update-dbvol", heketiServer.ConfigArg(), "--force-volume-name", res.Name})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("level1Check", func(t *testing.T) {
		defer partialTeardown()
		na := testutils.RequireNodeAccess(t)
		exec := na.Use(logger)

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 2
		volReq.Durability.Type = api.DurabilityReplicate
		res, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, res.Name != "")

		testutils.ServerStopped(t, heketiServer)
		err = heketiServer.RunOfflineCmd(
			[]string{"offline", "update-dbvol", heketiServer.ConfigArg(), "--force-volume-name", res.Name})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		sshHost := testCluster.SshHost(0)
		cmd := rex.OneCmd(
			fmt.Sprintf("gluster volume get %v user.heketi.dbstoragelevel", res.Name))
		r, err := exec.ExecCommands(sshHost, cmd, 10, true)
		tests.Assert(t, rex.AnyError(r, err) == nil,
			"expected err == nil, got:", err)
		tests.Assert(t,
			strings.Contains(r[0].Output, "1"),
			"expected 1 in output, got:", r[0].Output)
	})

	t.Run("levelCustomUntouched", func(t *testing.T) {
		defer partialTeardown()
		na := testutils.RequireNodeAccess(t)
		exec := na.Use(logger)

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 2
		volReq.Durability.Type = api.DurabilityReplicate
		res, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, res.Name != "")

		sshHost := testCluster.SshHost(0)
		cmds := rex.Cmds{
			rex.ToCmd(fmt.Sprintf("gluster volume set %v user.heketi.dbstoragelevel custom", res.Name)),
			rex.ToCmd(fmt.Sprintf("gluster volume set %v performance.io-cache on", res.Name)),
		}
		err = rex.AnyError(exec.ExecCommands(sshHost, cmds, 10, true))

		testutils.ServerStopped(t, heketiServer)
		err = heketiServer.RunOfflineCmd(
			[]string{"offline", "update-dbvol", heketiServer.ConfigArg(), "--force-volume-name", res.Name})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		cmds = rex.Cmds{
			rex.ToCmd(fmt.Sprintf("gluster volume get %v user.heketi.dbstoragelevel", res.Name)),
			rex.ToCmd(fmt.Sprintf("gluster volume get %v performance.io-cache", res.Name)),
		}
		r, err := exec.ExecCommands(sshHost, cmds, 10, true)
		tests.Assert(t, rex.AnyError(r, err) == nil,
			"expected err == nil, got:", err)
		tests.Assert(t,
			strings.Contains(r[0].Output, "custom"),
			"expected 'custom' in output, got:", r[0].Output)
		tests.Assert(t,
			strings.Contains(r[1].Output, "on"),
			"expected 'on' in output, got:", r[0].Output)
	})
}
