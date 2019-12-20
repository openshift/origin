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
	"testing"

	"github.com/heketi/tests"

	inj "github.com/heketi/heketi/executors/injectexec"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/testutils"
	"github.com/heketi/heketi/server/config"
)

func TestOnlineCleanup(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	baseConf := tests.Tempfile()
	defer os.Remove(baseConf)
	UpdateConfig(origConf, baseConf, func(c *config.Config) {
		// we want the background cleaner disabled for all
		// of the sub-tests we'll be running as we are testing
		// on demand cleaning and want predictable behavior.
		c.GlusterFS.DisableBackgroundCleaner = true
	})

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(baseConf, heketiServer.ConfPath)

	fullTeardown := func() {
		CopyFile(baseConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}
	partialTeardown := func() {
		CopyFile(baseConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.VolumeTeardown(t)
	}

	defer fullTeardown()
	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 3)

	t.Run("NoOp", func(t *testing.T) {
		defer partialTeardown()
		testOnlineCleanupNoOp(t, heketiServer, origConf)
	})
	t.Run("ThreeVolumesFailed", func(t *testing.T) {
		defer partialTeardown()
		testOnlineCleanupThreeVolumesFailed(t, heketiServer, baseConf)
	})
	t.Run("RetryThreeVolumes", func(t *testing.T) {
		defer partialTeardown()
		testOnlineCleanupRetryThreeVolumes(t, heketiServer, origConf)
	})
	t.Run("VolumeExpand", func(t *testing.T) {
		defer partialTeardown()
		testOnlineCleanupVolumeExpand(t, heketiServer, origConf)
	})
	t.Run("VolumeDelete", func(t *testing.T) {
		defer partialTeardown()
		testOnlineCleanupVolumeDelete(t, heketiServer, origConf)
	})
	t.Run("BlockVolumeCreates", func(t *testing.T) {
		defer partialTeardown()
		testOnlineCleanupBlockVolumeCreates(t, heketiServer, origConf)
	})
	t.Run("BlockVolumeCreateOldBHV", func(t *testing.T) {
		defer partialTeardown()
		testOnlineCleanupBlockVolumeCreateOldBHV(t, heketiServer, origConf)
	})
	t.Run("BlockVolumeDelete", func(t *testing.T) {
		defer partialTeardown()
		testOnlineCleanupBlockVolumeDelete(t, heketiServer, origConf)
	})
}

func testOnlineCleanupNoOp(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	testutils.ServerStarted(t, heketiServer)

	// create three volumes
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	info, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)

	err = heketi.PendingOperationCleanUp(
		&api.PendingOperationsCleanRequest{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func testOnlineCleanupThreeVolumesFailed(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: ".*blammo.*",
				Reaction: inj.Reaction{
					Err: "saw blammo. got blammod!",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// create three good volumes
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
	// fail three volumes
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Name = fmt.Sprintf("vblammo%v", i)
		volReq.Durability.Type = api.DurabilityReplicate
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	}

	info, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)

	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 3,
		"expected len(l.PendingOperations)t == 3, got:", len(l.PendingOperations))

	// restart server reverting injected errors
	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerRestarted(t, heketiServer)

	// request cleanup from server
	err = heketi.PendingOperationCleanUp(
		&api.PendingOperationsCleanRequest{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	l, err = heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))

	// assert that the non-failed volumes still exist
	vl, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 3,
		"expected len(vl.Volumes) == 3, got:", vl.Volumes)
}

func testOnlineCleanupRetryThreeVolumes(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: ".*blammo.*",
				Reaction: inj.Reaction{
					Err: "saw blammo. got blammod!",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// create three good volumes
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
	// fail three volumes
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Name = fmt.Sprintf("vblammo%v", i)
		volReq.Durability.Type = api.DurabilityReplicate
		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	}

	info, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)

	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 3,
		"expected len(l.PendingOperations)t == 3, got:", len(l.PendingOperations))

	// request cleanup from server
	err = heketi.PendingOperationCleanUp(
		&api.PendingOperationsCleanRequest{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	l, err = heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 3,
		"expected len(l.PendingOperations)t == 3, got:", len(l.PendingOperations))

	// now retry the clean up with the "good" config
	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerRestarted(t, heketiServer)

	// request cleanup from server
	err = heketi.PendingOperationCleanUp(
		&api.PendingOperationsCleanRequest{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	l, err = heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))
}

func testOnlineCleanupVolumeExpand(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	var err error
	// create three good volumes
	testutils.ServerStarted(t, heketiServer)
	var v *api.VolumeInfoResponse
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate
		v, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
	tests.Assert(t, v.Id != "")

	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: ".*add-brick .*",
				Reaction: inj.Reaction{
					Panic: "injected panic on add-brick",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// attempt to expand one volume
	volExReq := &api.VolumeExpandRequest{Size: 10}
	_, err = heketi.VolumeExpand(v.Id, volExReq)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, !heketiServer.IsAlive(),
		"server is alive; expected server dead due to panic")

	// clean up with the "good" config
	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerRestarted(t, heketiServer)

	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 1,
		"expected len(l.PendingOperations)t == 1, got:", len(l.PendingOperations))

	// request cleanup from server
	err = heketi.PendingOperationCleanUp(
		&api.PendingOperationsCleanRequest{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	l, err = heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))
}

func testOnlineCleanupVolumeDelete(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	var err error
	// create three good volumes
	testutils.ServerStarted(t, heketiServer)
	var v *api.VolumeInfoResponse
	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 5
		volReq.Durability.Type = api.DurabilityReplicate
		v, err = heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
	tests.Assert(t, v.Id != "")

	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: "^gluster .*volume .*delete .*",
				Reaction: inj.Reaction{
					Panic: "injected panic on delete!",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// attempt to delete one volume
	err = heketi.VolumeDelete(v.Id)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, !heketiServer.IsAlive(),
		"server is alive; expected server dead due to panic")

	// clean up with the "good" config
	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerRestarted(t, heketiServer)

	// request cleanup from server
	err = heketi.PendingOperationCleanUp(
		&api.PendingOperationsCleanRequest{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testutils.ServerStarted(t, heketiServer)
	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))
}

func testOnlineCleanupBlockVolumeCreates(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: "^gluster-block .*",
				Reaction: inj.Reaction{
					Err: "failing g-b command",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// fail to create a block volume
	volReq := &api.BlockVolumeCreateRequest{}
	volReq.Size = 8
	_, err := heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	info, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)

	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 1,
		"expected len(l.PendingOperations)t == 1, got:", len(l.PendingOperations))

	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerRestarted(t, heketiServer)

	// request cleanup from server
	err = heketi.PendingOperationCleanUp(
		&api.PendingOperationsCleanRequest{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	l, err = heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))
}

func testOnlineCleanupBlockVolumeCreateOldBHV(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	// create a BHV and block volumes
	testutils.ServerStarted(t, heketiServer)
	volReq := &api.BlockVolumeCreateRequest{}
	volReq.Size = 8
	_, err := heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	_, err = heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: "^gluster-block .*",
				Reaction: inj.Reaction{
					Err: "failing g-b command",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// fail to create a block volume
	_, err = heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	info, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, info.InFlight == 0,
		"expected info.InFlight == 0, got:", info.InFlight)

	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 1,
		"expected len(l.PendingOperations)t == 1, got:", len(l.PendingOperations))

	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerRestarted(t, heketiServer)

	// request cleanup from server
	err = heketi.PendingOperationCleanUp(
		&api.PendingOperationsCleanRequest{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	l, err = heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))

	// assert that the BHV still exists
	vl, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 1)
}

func testOnlineCleanupBlockVolumeDelete(
	t *testing.T, heketiServer *testutils.ServerCtl, origConf string) {

	// create a BHV and block volumes
	testutils.ServerStarted(t, heketiServer)
	volReq := &api.BlockVolumeCreateRequest{}
	volReq.Size = 8
	_, err := heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	victim, err := heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: "^gluster-block .*",
				Reaction: inj.Reaction{
					Panic: "panicking on g-b command",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	// check the number of volumes, get num. bvs
	var bvCount int
	ci, err := heketi.ClusterInfo(victim.Cluster)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ci.Volumes) == 1,
		"expected len(ci.Volumes) == 1, got:", ci.Volumes)
	bvCount = len(ci.BlockVolumes)

	// fail to delete a bv
	err = heketi.BlockVolumeDelete(victim.Id)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
	tests.Assert(t, !heketiServer.IsAlive(),
		"server is alive; expected server dead due to panic")

	CopyFile(origConf, heketiServer.ConfPath)
	testutils.ServerRestarted(t, heketiServer)

	// request cleanup from server
	err = heketi.PendingOperationCleanUp(
		&api.PendingOperationsCleanRequest{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))

	// assert that the BHV and first BV still exists
	ci, err = heketi.ClusterInfo(victim.Cluster)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ci.Volumes) == 1,
		"expected len(ci.Volumes) == 1, got:", ci.Volumes)
	tests.Assert(t, len(ci.BlockVolumes) == bvCount-1,
		"expected len(ci.BlockVolumes) == bvCount - 1, got:", ci.BlockVolumes, bvCount-1)
}

func TestOldBHVCleanup(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.DisableBackgroundCleaner = true
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

	// we need a clean copy of the db so we can actually clean up later
	testutils.ServerStopped(t, heketiServer)
	tmpDb := tests.Tempfile()
	defer os.Remove(tmpDb)

	dbPath := path.Join(heketiServer.ServerDir, heketiServer.DbPath)
	err := CopyFile(dbPath, tmpDb)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	defer CopyFile(tmpDb, dbPath)

	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.DisableBackgroundCleaner = true
		c.GlusterFS.Executor = "inject/ssh"
		c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
			inj.CmdHook{
				Cmd: "^gluster-block .*",
				Reaction: inj.Reaction{
					Panic: "panicking on g-b command",
				},
			},
		}
	})
	testutils.ServerRestarted(t, heketiServer)

	volReq := &api.BlockVolumeCreateRequest{}
	volReq.Size = 10
	_, err = heketi.BlockVolumeCreate(volReq)
	tests.Assert(t, err != nil, "expected err == nil, got:", err)
	tests.Assert(t, !heketiServer.IsAlive(),
		"server is alive; expected server dead due to panic")

	dbJson := tests.Tempfile()
	defer os.Remove(dbJson)

	// export the db to json so we can hack it up
	err = heketiServer.RunOfflineCmd(
		[]string{"db", "export",
			"--dbfile", heketiServer.DbPath,
			"--jsonfile", dbJson})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// edit the json dump so that it looks more like the BHV created
	// by older (v7) heketi db contents for a pending BVH+BV
	var dump map[string]interface{}
	readJsonDump(t, dbJson, &dump)
	bhvs := dump["volumeentries"].(map[string]interface{})
	tests.Assert(t, len(bhvs) == 1, "expected len(bhvs) == 1)")
	var bhvol map[string]interface{}
	for _, v := range bhvs {
		bhvol = v.(map[string]interface{})
	}
	vi := bhvol["Info"].(map[string]interface{})
	sz := int(vi["size"].(float64))
	binfo := vi["blockinfo"].(map[string]interface{})
	// since it is simpler: set freesize to the size of the vol and
	// reservedsize to 0, this is close enough to the old heketi
	// behavior to trigger the issue we're handling
	binfo["freesize"] = sz
	binfo["reservedsize"] = 0
	logger.Info("Changed BHV info to: %+v", vi)
	writeJsonDump(t, dbJson, dump)

	// restore the "hacked" json to a heketi db (replacing old version)
	os.Remove(dbPath)
	err = heketiServer.RunOfflineCmd(
		[]string{"db", "import",
			"--dbfile", heketiServer.DbPath,
			"--jsonfile", dbJson})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// restore the default config minus bg cleaner
	UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
		c.GlusterFS.DisableBackgroundCleaner = true
	})
	testutils.ServerRestarted(t, heketiServer)

	// we should have our pending BHV+BV in db still
	l, err := heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 1,
		"expected len(l.PendingOperations)t == 1, got:", len(l.PendingOperations))

	// request cleanup from server
	err = heketi.PendingOperationCleanUp(
		&api.PendingOperationsCleanRequest{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// clean up should now automatically remove even old style BHVs
	l, err = heketi.PendingOperationList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.PendingOperations) == 0,
		"expected len(l.PendingOperations)t == 0, got:", len(l.PendingOperations))
	vl, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(vl.Volumes) == 0,
		"expected len(vl.Volumes) == 0, got:", vl.Volumes)
}
