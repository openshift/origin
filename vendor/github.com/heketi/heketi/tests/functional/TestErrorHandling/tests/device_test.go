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
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/heketi/tests"

	inj "github.com/heketi/heketi/executors/injectexec"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	rex "github.com/heketi/heketi/pkg/remoteexec"
	"github.com/heketi/heketi/pkg/remoteexec/ssh"
	"github.com/heketi/heketi/pkg/testutils"
	"github.com/heketi/heketi/server/config"
)

func TestDeviceAddRemoveSymlink(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")

	defer func() {
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 2)

	na := testutils.RequireNodeAccess(t)
	exec := na.Use(logger)

	for i := 0; i < len(testCluster.Nodes); i++ {
		err := linkDevice(testCluster, exec, i, 3, "/dev/bender")
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	topo, err := heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	var firstNode string
	preDevices := map[string]bool{}
	for i, n := range topo.ClusterList[0].Nodes {
		if i == 0 {
			firstNode = n.Id
		}
		for _, d := range n.DevicesInfo {
			preDevices[d.Id] = true
		}
	}

	req := &api.DeviceAddRequest{}
	req.Name = "/dev/bender"
	req.NodeId = firstNode
	err = heketi.DeviceAdd(req)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	for i := 0; i < len(testCluster.Nodes); i++ {
		err := rmLink(testCluster, exec, i, "/dev/bender")
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	topo, err = heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	var newDeviceId string
	for _, n := range topo.ClusterList[0].Nodes {
		for _, d := range n.DevicesInfo {
			if !preDevices[d.Id] {
				newDeviceId = d.Id
			}
		}
	}

	stateReq := &api.StateRequest{}
	stateReq.State = api.EntryStateOffline
	err = heketi.DeviceState(newDeviceId, stateReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	stateReq = &api.StateRequest{}
	stateReq.State = api.EntryStateFailed
	err = heketi.DeviceState(newDeviceId, stateReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = heketi.DeviceDelete(newDeviceId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestDeviceStrippedMetadata(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")

	defer func() {
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 2)

	oldDevices := []string{}
	topo, err := heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	for _, n := range topo.ClusterList[0].Nodes {
		for i, d := range n.DevicesInfo {
			if i == 0 {
				oldDevices = append(oldDevices, d.Id)
			}
		}
	}

	testutils.ServerStopped(t, heketiServer)
	stripDeviceMetadata(t, heketiServer, oldDevices)
	testutils.ServerStarted(t, heketiServer)

	for _, devId := range oldDevices {
		stateReq := &api.StateRequest{}
		stateReq.State = api.EntryStateOffline
		err = heketi.DeviceState(devId, stateReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		stateReq = &api.StateRequest{}
		stateReq.State = api.EntryStateFailed
		err = heketi.DeviceState(devId, stateReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = heketi.DeviceDelete(devId)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
}

func TestDeviceStrippedMetadataRemoveSymlink(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")

	defer func() {
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 2)

	na := testutils.RequireNodeAccess(t)
	exec := na.Use(logger)

	// create symlink aliases
	for i := 0; i < len(testCluster.Nodes); i++ {
		err := linkDevice(testCluster, exec, i, 3, "/dev/bender")
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// add our new devices using the symlink-alias
	topo, err := heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	for _, n := range topo.ClusterList[0].Nodes {
		req := &api.DeviceAddRequest{}
		req.Name = "/dev/bender"
		req.NodeId = n.Id
		err = heketi.DeviceAdd(req)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// remove our alias symlinks
	for i := 0; i < len(testCluster.Nodes); i++ {
		err := rmLink(testCluster, exec, i, "/dev/bender")
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// gather devices to fiddle with
	oldDevices := []string{}
	topo, err = heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	for _, n := range topo.ClusterList[0].Nodes {
		for i, d := range n.DevicesInfo {
			if i == 0 || d.Name == "/dev/bender" {
				oldDevices = append(oldDevices, d.Id)
			}
		}
	}

	// strip devices of new identifying metadata
	testutils.ServerStopped(t, heketiServer)
	stripDeviceMetadata(t, heketiServer, oldDevices)
	testutils.ServerStarted(t, heketiServer)

	// Removing these devices should succeed even though they've been fiddled
	// with because the code contains the ability to auto-detect the device
	// based on the vg id, not just the extra metadata we added in this
	// version.
	for _, devId := range oldDevices {
		stateReq := &api.StateRequest{}
		stateReq.State = api.EntryStateOffline
		err = heketi.DeviceState(devId, stateReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		stateReq = &api.StateRequest{}
		stateReq.State = api.EntryStateFailed
		err = heketi.DeviceState(devId, stateReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		err = heketi.DeviceDelete(devId)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
}

func TestDeviceHandlingFallbacks(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(origConf, heketiServer.ConfPath)

	partialTeardown := func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
	}

	defer func() {
		partialTeardown()
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 2)

	diskPath := testCluster.Disks[3]

	t.Run("disableUdevadm", func(t *testing.T) {
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: ".*udevadm.*",
					Reaction: inj.Reaction{
						Err: "boop!",
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)
		defer partialTeardown()

		topo, err := heketi.TopologyInfo()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		for _, n := range topo.ClusterList[0].Nodes {
			req := &api.DeviceAddRequest{}
			req.Name = diskPath
			req.NodeId = n.Id
			err = heketi.DeviceAdd(req)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}

		rmDevices := []string{}
		topo, err = heketi.TopologyInfo()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		for _, n := range topo.ClusterList[0].Nodes {
			for i, d := range n.DevicesInfo {
				if i == 0 || d.Name == diskPath {
					rmDevices = append(rmDevices, d.Id)
				}
			}
		}
		for _, devId := range rmDevices {
			stateReq := &api.StateRequest{}
			stateReq.State = api.EntryStateOffline
			err = heketi.DeviceState(devId, stateReq)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)

			stateReq = &api.StateRequest{}
			stateReq.State = api.EntryStateFailed
			err = heketi.DeviceState(devId, stateReq)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)

			err = heketi.DeviceDelete(devId)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}
	})

	t.Run("disableUdevadmAndVgs", func(t *testing.T) {
		UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			c.GlusterFS.Executor = "inject/ssh"
			c.GlusterFS.InjectConfig.CmdInjection.CmdHooks = inj.CmdHooks{
				inj.CmdHook{
					Cmd: ".*udevadm.*",
					Reaction: inj.Reaction{
						Err: "boop!",
					},
				},
				inj.CmdHook{
					Cmd: ".*vgs .*",
					Reaction: inj.Reaction{
						Err: "bonk!",
					},
				},
			}
		})
		testutils.ServerRestarted(t, heketiServer)
		defer partialTeardown()

		topo, err := heketi.TopologyInfo()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		for _, n := range topo.ClusterList[0].Nodes {
			req := &api.DeviceAddRequest{}
			req.Name = diskPath
			req.NodeId = n.Id
			err = heketi.DeviceAdd(req)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}

		rmDevices := []string{}
		topo, err = heketi.TopologyInfo()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		for _, n := range topo.ClusterList[0].Nodes {
			for i, d := range n.DevicesInfo {
				if i == 0 || d.Name == diskPath {
					rmDevices = append(rmDevices, d.Id)
				}
			}
		}

		// strip devices of new identifying metadata
		testutils.ServerStopped(t, heketiServer)
		stripDeviceMetadata(t, heketiServer, rmDevices)
		testutils.ServerStarted(t, heketiServer)

		for _, devId := range rmDevices {
			stateReq := &api.StateRequest{}
			stateReq.State = api.EntryStateOffline
			err = heketi.DeviceState(devId, stateReq)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)

			stateReq = &api.StateRequest{}
			stateReq.State = api.EntryStateFailed
			err = heketi.DeviceState(devId, stateReq)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)

			err = heketi.DeviceDelete(devId)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}
	})
}

func TestDeviceAddDupePaths(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")

	defer func() {
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 2)

	na := testutils.RequireNodeAccess(t)
	exec := na.Use(logger)

	// create symlink aliases
	for i := 0; i < len(testCluster.Nodes); i++ {
		err := linkDevice(testCluster, exec, i, 3, "/dev/bender")
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// add our new devices using the symlink-alias
	topo, err := heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	for _, n := range topo.ClusterList[0].Nodes {
		req := &api.DeviceAddRequest{}
		req.Name = "/dev/bender"
		req.NodeId = n.Id
		err = heketi.DeviceAdd(req)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// assert we can not add the same devices again
	for _, n := range topo.ClusterList[0].Nodes {
		req := &api.DeviceAddRequest{}
		req.Name = "/dev/bender"
		req.NodeId = n.Id
		req.DestroyData = true
		err = heketi.DeviceAdd(req)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	}
}

func TestDeviceAddShuffledPaths(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")

	defer func() {
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true
	testCluster.Setup(t, 3, 2)

	na := testutils.RequireNodeAccess(t)
	exec := na.Use(logger)

	// create symlink aliases
	for i := 0; i < len(testCluster.Nodes); i++ {
		err := linkDevice(testCluster, exec, i, 3, "/dev/bender")
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// add our new devices using the symlink-alias
	topo, err := heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	for _, n := range topo.ClusterList[0].Nodes {
		req := &api.DeviceAddRequest{}
		req.Name = "/dev/bender"
		req.NodeId = n.Id
		err = heketi.DeviceAdd(req)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// swap our alias symlinks
	for i := 0; i < len(testCluster.Nodes); i++ {
		err := rmLink(testCluster, exec, i, "/dev/bender")
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		err = linkDevice(testCluster, exec, i, 4, "/dev/bender")
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// assert we can add new clean devices with the same paths used
	// previously
	for _, n := range topo.ClusterList[0].Nodes {
		req := &api.DeviceAddRequest{}
		req.Name = "/dev/bender"
		req.NodeId = n.Id
		err = heketi.DeviceAdd(req)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// clean up all devices
	topo, err = heketi.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	for _, n := range topo.ClusterList[0].Nodes {
		for _, d := range n.DevicesInfo {
			stateReq := &api.StateRequest{}
			stateReq.State = api.EntryStateOffline
			err = heketi.DeviceState(d.Id, stateReq)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)

			stateReq = &api.StateRequest{}
			stateReq.State = api.EntryStateFailed
			err = heketi.DeviceState(d.Id, stateReq)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)

			err = heketi.DeviceDelete(d.Id)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}
	}
}

func linkDevice(
	tc *testutils.ClusterEnv, exec *ssh.SshExec,
	hostIdx, diskIdx int, newPath string) error {

	sshHost := tc.SshHost(hostIdx)
	diskPath := tc.Disks[diskIdx]
	cmds := rex.OneCmd(
		fmt.Sprintf("ln -sf %s %s", diskPath, newPath),
	)
	return rex.AnyError(exec.ExecCommands(sshHost, cmds, 10, true))
}

func rmLink(
	tc *testutils.ClusterEnv, exec *ssh.SshExec,
	hostIdx int, path string) error {

	sshHost := tc.SshHost(hostIdx)
	cmds := rex.OneCmd(
		fmt.Sprintf("rm -f %s", path),
	)
	return rex.AnyError(exec.ExecCommands(sshHost, cmds, 10, true))
}

func stripDeviceMetadata(
	t *testing.T, heketiServer *testutils.ServerCtl, deviceIds []string) {

	dbJson := tests.Tempfile()
	defer os.Remove(dbJson)

	// export the db to json so we can hack it up
	err := heketiServer.RunOfflineCmd(
		[]string{"db", "export",
			"--dbfile", heketiServer.DbPath,
			"--jsonfile", dbJson})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// edit the json dump so that recently created device entries
	// are stripped of the new device identifying metadata and
	// act like old db entries
	var dump map[string]interface{}
	readJsonDump(t, dbJson, &dump)
	deviceEntries, ok := dump["deviceentries"].(map[string]interface{})
	tests.Assert(t, ok, "conversion failed")
	for _, id := range deviceIds {
		dev, ok := deviceEntries[id].(map[string]interface{})
		tests.Assert(t, ok, "conversion failed")
		info, ok := dev["Info"].(map[string]interface{})
		tests.Assert(t, ok, "conversion failed")
		delete(info, "paths")
		delete(info, "pv_uuid")
	}
	writeJsonDump(t, dbJson, dump)

	// restore the "hacked" json to a heketi db (replacing old version)
	dbPath := path.Join(heketiServer.ServerDir, heketiServer.DbPath)
	os.Remove(dbPath)
	err = heketiServer.RunOfflineCmd(
		[]string{"db", "import",
			"--dbfile", heketiServer.DbPath,
			"--jsonfile", dbJson})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}
