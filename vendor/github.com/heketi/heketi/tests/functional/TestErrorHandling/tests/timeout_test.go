// +build functional

//
// Copyright (c) 2019 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//
package tests

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	rex "github.com/heketi/heketi/pkg/remoteexec"
	"github.com/heketi/heketi/pkg/remoteexec/ssh"
	"github.com/heketi/heketi/pkg/testutils"
	"github.com/heketi/heketi/server/config"

	"github.com/heketi/tests"
)

var (
	pidFile   = "/var/tmp/hook.pid"
	delayFile = "/var/tmp/hook-delay"
	killPid   = "/var/tmp/kill-pid.sh"
)

func addDelay(timeOut int, node string, exec *ssh.SshExec) error {
	dumpData := fmt.Sprintf("echo -e %ds >%s", timeOut, delayFile)
	cmd := rex.OneCmd(dumpData)
	err := rex.AnyError(exec.ExecCommands(node, cmd, 10, true))
	return err
}

func cleanDelay(node string, exec *ssh.SshExec) error {
	removeSleep := fmt.Sprintf("sh %s", killPid)
	removeDelayFile := fmt.Sprintf("rm -f %s", delayFile)
	removePIDFile := fmt.Sprintf("rm -f %s", pidFile)

	cmd := []string{removeSleep, removeDelayFile, removePIDFile}
	err := rex.AnyError(exec.ExecCommands(node, rex.ToCmds(cmd), 10, true))
	return err
}

func cleanupOp(exec *ssh.SshExec, t *testing.T) {
	for i := 0; i < len(testCluster.Nodes); i++ {
		sshHost := testCluster.SshHost(i)
		err := cleanDelay(sshHost, exec)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

	}
}

func TestGlusterTimeout(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	testutils.ServerStarted(t, heketiServer)
	defer testutils.ServerStopped(t, heketiServer)

	testCluster.Teardown(t)
	testCluster.Setup(t, len(testCluster.Nodes), len(testCluster.Disks))
	defer testCluster.Teardown(t)

	na := testutils.RequireNodeAccess(t)
	exec := na.Use(logger)
	//Create a volume
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1
	volReq.Snapshot.Enable = true
	volReq.Snapshot.Factor = 1.5
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	t.Run("deleteSucceeds", func(t *testing.T) {
		for i := 0; i < len(testCluster.Nodes); i++ {
			//61s+61s =2.02 minutes to ensure we are not hitting
			//default gluster timeout of 2 minutes
			//this  adds 61s sleep on initiator node and 61s parallel sleep
			//on remaining nodes
			err := addDelay(61, testCluster.SshHost(i), exec)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)

		}
		//remove pid,delay file and clean up sleep process if present
		defer cleanupOp(exec, t)

		simplevol, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		// Delete volume
		err = heketi.VolumeDelete(simplevol.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	})

	t.Run("deleteFails", func(t *testing.T) {
		origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)
		heketiServer.ConfPath = tests.Tempfile()
		defer os.Remove(heketiServer.ConfPath)
		err := UpdateConfig(origConf, heketiServer.ConfPath, func(c *config.Config) {
			//gluster cli timeout set to 5 seconds
			c.GlusterFS.SshConfig.GlusterCliTimeout = 5
		})
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		heketiServer.KeepDB = true
		testutils.ServerRestarted(t, heketiServer)

		simplevol, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		//volume deletion should fail as we have the sleep of
		//20s+20s=40s which is greater than gluster  cli timeout of 5s
		for i := 0; i < len(testCluster.Nodes); i++ {
			//having extra sleep does not affect  the testing
			err := addDelay(20, testCluster.SshHost(i), exec)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}
		defer cleanupOp(exec, t)
		// Delete volume
		err = heketi.VolumeDelete(simplevol.Id)
		tests.Assert(t, err != nil)
	})
}
