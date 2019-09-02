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
	"strings"
	"testing"

	"github.com/heketi/tests"

	client "github.com/heketi/heketi/client/api/go-client"
	inj "github.com/heketi/heketi/executors/injectexec"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/testutils"
	"github.com/heketi/heketi/server/config"
)

func TestHeketiStart(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	err := heketiServer.Start()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, heketiServer.IsAlive(), "expected server is alive")
	err = heketiServer.Stop()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestConfigChange(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)
	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.Port = "8181"
	})
	defer testutils.ServerStopped(t, heketiServer)
	heketiServer.HelloPort = "8181"
	testutils.ServerStarted(t, heketiServer)

	// verify that the default heketi client can't talk to the server
	_, err := heketi.VolumeList()
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	// verify that a custom client using the new port can talk to server
	h2 := client.NewClientNoAuth("http://localhost:8181")
	_, err = h2.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// verify that we can enable the mock executor and use it
	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "mock"
	})
	heketiServer.HelloPort = "8080"
	testutils.ServerRestarted(t, heketiServer)

	testCluster.Setup(t, 2, 3)
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 5
	volReq.Durability.Type = api.DurabilityDistributeOnly

	volInfo, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, volInfo != nil)
	// no cleanup needed, as the db will be discarded
}

func TestHeketiPersistence(t *testing.T) {
	// do this twice to verify that all checks pass
	// even after "dirtying" the nodes once
	t.Run("Pass1", testDbPersists)
	t.Run("Pass2", testDbPersists)
}

func testDbPersists(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	defer testutils.ServerStopped(t, heketiServer)
	testutils.ServerStarted(t, heketiServer)

	defer testCluster.Teardown(t)
	testCluster.Setup(t, 2, 3)

	// create a volume
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 5
	volReq.Durability.Type = api.DurabilityReplicate

	volInfo, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, volInfo != nil)

	// restart the heketi server
	heketiServer.KeepDB = true
	testutils.ServerRestarted(t, heketiServer)

	// verify that the volume still exists
	vl, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 1,
		"expected len(vl.Volumes) == 1, got", len(vl.Volumes))
	tests.Assert(t, vl.Volumes[0] == volInfo.Id,
		"expected vl.Volumes[0] == volInfo.Id, got", vl.Volumes[0], volInfo.Id)
}

func TestErrorInjection(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")

	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)
	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(origConf, heketiServer.ConfPath)

	defer testutils.ServerStopped(t, heketiServer)
	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true

	defer func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
	}()
	testCluster.Setup(t, 3, 3)

	t.Run("VolumeCreateOk", func(t *testing.T) {
		// verify things work w/o error injection in place
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate

		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})
	t.Run("VolumeCreateErr", func(t *testing.T) {
		tmpDb := tests.Tempfile()
		defer os.Remove(tmpDb)

		// will simulate a failure to create a brick
		testutils.ServerStopped(t, heketiServer)
		dbPath := path.Join(heketiServer.ServerDir, heketiServer.DbPath)
		err := CopyFile(dbPath, tmpDb)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.DBfile = tmpDb
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd:      "^lvcreate.*",
					Reaction: inj.Reaction{Err: "fooey"},
				},
			}
		})

		testutils.ServerRestarted(t, heketiServer)
		// create a volume
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate

		_, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	})
	t.Run("VolumeDeleteErr", func(t *testing.T) {
		// simulate a failure to umount a brick fs
		testutils.ServerStopped(t, heketiServer)
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^umount.*",
					Reaction: inj.Reaction{
						Err:   "unable to umount",
						Pause: 2, // make it slow too
					},
				},
			}
		})

		testutils.ServerRestarted(t, heketiServer)
		// create a volume
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate

		volInfo, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = heketi.VolumeDelete(volInfo.Id)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	})
	t.Run("VolumeDeletePanic", func(t *testing.T) {
		tmpDb := tests.Tempfile()
		defer os.Remove(tmpDb)

		// simulate server crash (panic) running lvcreate
		testutils.ServerStopped(t, heketiServer)
		dbPath := path.Join(heketiServer.ServerDir, heketiServer.DbPath)
		err := CopyFile(dbPath, tmpDb)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.DBfile = tmpDb
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^lvcreate.*",
					Reaction: inj.Reaction{
						Panic: "KaBoom",
					},
				},
			}
		})

		testutils.ServerRestarted(t, heketiServer)
		// create a volume
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate

		_, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)

		isAlive := heketiServer.IsAlive()
		tests.Assert(t, !isAlive, "expected heketi server stopped")
	})
}

func TestGlusterBlockError(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")

	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)
	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(origConf, heketiServer.ConfPath)

	defer testutils.ServerStopped(t, heketiServer)
	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true

	defer func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
	}()
	testCluster.Setup(t, 3, 3)

	t.Run("ParseableError", func(t *testing.T) {
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block create .*",
					Reaction: inj.Reaction{
						Result:         `{"RESULT": "FAIL", "errCode": 1, "errMsg": "ZOWIE"}`,
						ForceErr:       true,
						ForceErrOutput: true,
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		// create a block volume
		req := &api.BlockVolumeCreateRequest{}
		req.Size = 5

		_, err := heketi.BlockVolumeCreate(req)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t,
			strings.Contains(err.Error(), "Failed to create block volume"),
			`expected string "Failed to create block volume" in err, got:`,
			err.Error())
		tests.Assert(t,
			strings.Contains(err.Error(), "ZOWIE"),
			`expected string "ZOWIE" in err, got:`,
			err.Error())
	})
	t.Run("UnparseableError", func(t *testing.T) {
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block create .*",
					Reaction: inj.Reaction{
						Err: "SPLAT. I failed.",
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		// create a block volume
		req := &api.BlockVolumeCreateRequest{}
		req.Size = 5

		_, err := heketi.BlockVolumeCreate(req)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t,
			strings.Contains(err.Error(), "Unparsable error"),
			`expected string "Unparsable error" in err, got:`,
			err.Error())
		tests.Assert(t,
			strings.Contains(err.Error(), "SPLAT"),
			`expected string "SPLAT" in err, got:`,
			err.Error())
	})
	t.Run("NoFlaggedError", func(t *testing.T) {
		// the command exits non-zero but FAIL flag is not set
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: "^gluster-block create .*",
					Reaction: inj.Reaction{
						Err: `{"RESULT": "", "errCode": 1, "errMsg": "Me broken."}`,
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)

		// create a block volume
		req := &api.BlockVolumeCreateRequest{}
		req.Size = 5

		_, err := heketi.BlockVolumeCreate(req)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
		tests.Assert(t,
			strings.Contains(err.Error(), "Failed to create block volume"),
			`expected string "Failed to create block volume" in err, got:`,
			err.Error())
		tests.Assert(t,
			strings.Contains(err.Error(), "Me broken"),
			`expected string "Me broken" in err, got:`,
			err.Error())
	})
}
