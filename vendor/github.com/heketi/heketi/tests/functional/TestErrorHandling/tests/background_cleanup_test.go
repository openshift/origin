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
	"time"

	"github.com/heketi/tests"

	inj "github.com/heketi/heketi/executors/injectexec"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/testutils"
	"github.com/heketi/heketi/server/config"
)

func TestCleanupAfterRestart(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)

	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.StartTimeBackgroundCleaner = 5
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: ".*blammo.*",
				Reaction: inj.Reaction{
					Panic: "saw blammo. got blammod!",
				},
			},
		}
	})

	fullTeardown := func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}
	defer fullTeardown()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 3)

	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 5
	volReq.Name = "vblammo1"
	volReq.Durability.Type = api.DurabilityReplicate
	_, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, !heketiServer.IsAlive(),
		"server is alive; expected server dead due to panic")

	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.StartTimeBackgroundCleaner = 5
	})

	testutils.ServerStarted(t, heketiServer)
	info, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)
	tests.Assert(t, info.Stale == 1,
		"expected info.Stale == 1, got:", info.InFlight)

	// wait for the background cleaner to be started
	time.Sleep(5 * time.Second)

	// wait around an additional 5 sec for stale ops to be cleaned up
	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		info, err = heketi.OperationsInfo()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		if info.Stale == 0 && info.InFlight == 0 {
			break
		}
	}

	info, err = heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)
	tests.Assert(t, info.Stale == 0,
		"expected info.Stale == 0, got:", info.Stale)
}

func TestCleanupPeriodic(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)

	glusterCond := tests.Tempfile()
	f, err := os.Create(glusterCond)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, f.Close() == nil, "expected err == nil, got:", err)
	defer os.Remove(glusterCond)

	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.StartTimeBackgroundCleaner = 2
		c.GlusterFS.RefreshTimeBackgroundCleaner = 8
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd:      "^gluster .*volume .*",
				CondFile: glusterCond,
				Reaction: inj.Reaction{
					Err: "failing a gluster volume command",
				},
			},
		}
	})

	fullTeardown := func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}
	defer fullTeardown()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 3)
	time.Sleep(2 * time.Second) // ensure we pass the start time

	logger.Info("fail to create a volume w/ failed rollback")
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 5
	volReq.Durability.Type = api.DurabilityReplicate
	_, err = heketi.VolumeCreate(volReq)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	info, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)
	tests.Assert(t, info.Failed == 1,
		"expected info.Failed == 1, got:", info.Failed)

	logger.Info("removing glusterCond condition file")
	os.Remove(glusterCond)
	time.Sleep(time.Second)

	// wait around up to twice the refresh interval for stale ops to be cleaned up
	for i := 0; i < 16; i++ {
		time.Sleep(time.Second)
		logger.Info("probing for server progress")
		info, err = heketi.OperationsInfo()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		if info.Stale == 0 && info.Failed == 0 && info.InFlight == 0 {
			break
		}
	}

	logger.Info("asserting server state")
	info, err = heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)
	tests.Assert(t, info.Stale == 0,
		"expected info.Stale == 0, got:", info.Stale)
	tests.Assert(t, info.Failed == 0,
		"expected info.Failed == 0, got:", info.Failed)
}
