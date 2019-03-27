// +build functional

//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package functional

import (
	"fmt"
	"strings"
	"testing"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/remoteexec/ssh"
	"github.com/heketi/tests"
)

func TestPartialDeletes(t *testing.T) {
	setupCluster(t, 4, 8)
	defer teardownCluster(t)
	t.Run("testDeleteNormal", testDeleteNormal)
	t.Run("testDeletedOnGluster", testDeletedOnGluster)
	t.Run("testDeletedUnmountedBrick", testDeletedUnmountedBrick)
	t.Run("testDeletedBrickPv", testDeletedBrickPv)
}

func testPrepareVolume(t *testing.T) *api.VolumeInfoResponse {
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	vcr, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	return vcr
}

func testDeleteNormal(t *testing.T) {
	vcr := testPrepareVolume(t)

	err := heketi.VolumeDelete(vcr.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func testDeletedOnGluster(t *testing.T) {
	vcr := testPrepareVolume(t)

	s := ssh.NewSshExecWithKeyFile(
		logger, "vagrant", "../config/insecure_private_key")
	cmds := []string{
		fmt.Sprintf("gluster --mode=script volume stop %v", vcr.Name),
		fmt.Sprintf("gluster --mode=script volume delete %v", vcr.Name),
	}
	_, err := s.ConnectAndExec(storage0ssh, cmds, 10, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = heketi.VolumeDelete(vcr.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func testDeletedUnmountedBrick(t *testing.T) {
	vcr := testPrepareVolume(t)

	s := ssh.NewSshExecWithKeyFile(
		logger, "vagrant", "../config/insecure_private_key")
	cmds := []string{
		fmt.Sprintf("gluster --mode=script volume info %v", vcr.Name),
		fmt.Sprintf("gluster --mode=script volume stop %v", vcr.Name),
		fmt.Sprintf("gluster --mode=script volume delete %v", vcr.Name),
	}
	o, err := s.ConnectAndExec(storage0ssh, cmds, 10, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(o) >= 1)
	var host, brickPath string
	for _, line := range strings.Split(o[0], "\n") {
		if len(line) >= 8 && line[:8] == "Brick1: " {
			parts := strings.SplitN(line[8:], ":", 2)
			host, brickPath = parts[0], parts[1]
			break
		}
	}
	tests.Assert(t, len(host) > 0, "expected len(host) > 0, got:", host)
	tests.Assert(t, len(brickPath) > 0, "expected len(brickPath) > 0, got:", host)
	cmds = []string{
		fmt.Sprintf("umount %v", strings.TrimSuffix(brickPath, "/brick")),
	}
	_, err = s.ConnectAndExec(host+":"+portNum, cmds, 10, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = heketi.VolumeDelete(vcr.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func testDeletedBrickPv(t *testing.T) {
	vcr := testPrepareVolume(t)

	s := ssh.NewSshExecWithKeyFile(
		logger, "vagrant", "../config/insecure_private_key")
	cmds := []string{
		fmt.Sprintf("gluster --mode=script volume info %v", vcr.Name),
		fmt.Sprintf("gluster --mode=script volume stop %v", vcr.Name),
		fmt.Sprintf("gluster --mode=script volume delete %v", vcr.Name),
	}
	o, err := s.ConnectAndExec(storage0ssh, cmds, 10, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(o) >= 1)
	var host, brickPath string
	for _, line := range strings.Split(o[0], "\n") {
		if len(line) >= 8 && line[:8] == "Brick1: " {
			parts := strings.SplitN(line[8:], ":", 2)
			host, brickPath = parts[0], parts[1]
			break
		}
	}
	tests.Assert(t, len(host) > 0, "expected len(host) > 0, got:", host)
	tests.Assert(t, len(brickPath) > 0, "expected len(brickPath) > 0, got:", host)
	cmds = []string{
		fmt.Sprintf("umount %v", strings.TrimSuffix(brickPath, "/brick")),
		"lvs --noheadings --sep / -o vg_name,lv_name | grep vg_ | xargs -L1 echo lvremove -f",
	}
	_, err = s.ConnectAndExec(host+":"+portNum, cmds, 10, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = heketi.VolumeDelete(vcr.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}
