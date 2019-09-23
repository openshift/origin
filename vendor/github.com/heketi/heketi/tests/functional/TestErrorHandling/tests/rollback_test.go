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

	inj "github.com/heketi/heketi/executors/injectexec"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/testutils"
	"github.com/heketi/heketi/server/config"
)

func TestBlockVolumeRollback(t *testing.T) {
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

	blockReq := &api.BlockVolumeCreateRequest{}
	blockReq.Size = 3
	blockReq.Hacount = 3

	t.Run("FailBlockCreate", func(t *testing.T) {
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block create .*",
					Reaction: inj.Reaction{
						Err: "unhandled floop!",
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		bvol, err := heketi.BlockVolumeCreate(blockReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t, bvol == nil)

		// assert that no volumes exist and no pending ops exist
		v, err := heketi.VolumeList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(v.Volumes) == 0,
			"expected len(v.Volumes) == 0, got:", len(v.Volumes))

		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))
	})
	t.Run("FailBHVCreate", func(t *testing.T) {
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster .* volume create .*",
					Reaction: inj.Reaction{
						Err: "unhandled bloop!",
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		bvol, err := heketi.BlockVolumeCreate(blockReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t, bvol == nil)

		// assert that no volumes exist and no pending ops exist
		v, err := heketi.VolumeList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(v.Volumes) == 0,
			"expected len(v.Volumes) == 0, got:", len(v.Volumes))

		l, err := heketi.PendingOperationList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(l.PendingOperations) == 0,
			"expected len(l.PendingOperations) == 0, got:", len(l.PendingOperations))
	})
}
