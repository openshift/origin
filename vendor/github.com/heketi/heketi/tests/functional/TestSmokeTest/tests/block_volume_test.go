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
	"testing"
	"time"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/remoteexec/ssh"
	"github.com/heketi/tests"
)

func TestBlockVolumeOperation(t *testing.T) {

	// Setup the VM storage topology
	setupCluster(t, 3, 4)
	defer teardownCluster(t)

	defer teardownBlock(t)

	req := &api.BlockVolumeCreateRequest{}
	//check it is not possible to create block volume if  size is greated then block hosting volume
	req.Size = 201

	_, err := heketi.BlockVolumeCreate(req)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	//check it is not possible to create block volume as same size of block hosting volume
	req.Size = 200
	_, err = heketi.BlockVolumeCreate(req)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	//check it is not possible to create block volume of size 197
	req.Size = 197
	_, err = heketi.BlockVolumeCreate(req)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	//check it is possible to create and delete block volume of size 196
	req.Size = 196
	vol, err := heketi.BlockVolumeCreate(req)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = heketi.BlockVolumeDelete(vol.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// 2% is reserved in blockhosting volume, we should be able to create
	//block volumes of total size 196 GB
	req.Size = 4
	for i := 1; i <= 49; i++ {
		vol, err := heketi.BlockVolumeCreate(req)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, vol.Size == 4, "expected vol.Size == 4 got:", vol.Size)
	}

	volList, err := heketi.BlockVolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(volList.BlockVolumes) == 49, "expected len(volList.BlockVolumes) == 49 got:", len(volList.BlockVolumes))

	for _, ID := range volList.BlockVolumes {
		volInfo, err := heketi.BlockVolumeInfo(ID)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.Size == 4, "expected volInfo.Size == 4 got:", volInfo.Size)

		err = heketi.BlockVolumeDelete(ID)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

}

func teardownBlock(t *testing.T) {

	volList, err := heketi.BlockVolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	for _, ID := range volList.BlockVolumes {
		err = heketi.BlockVolumeDelete(ID)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
}

func TestBlockVolumeCreateManyAtOnce(t *testing.T) {

	// Setup the VM storage topology
	setupCluster(t, 3, 4)
	defer teardownCluster(t)
	defer teardownBlock(t)

	// pre-create a block hosting volume that will not be able
	// to hold all of our requests
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	volReq.Block = true

	_, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	blockReq := &api.BlockVolumeCreateRequest{}
	blockReq.Size = 3

	type result struct {
		index int
		err   error
	}

	count := 5
	results := make(chan result, count)
	defer close(results)
	for i := 0; i < count; i++ {
		go func(i int) {
			logger.Info("sending block request to server (%v)", i)
			_, err := heketi.BlockVolumeCreate(blockReq)
			logger.Info("got result: %v @ %v", err, i)
			results <- result{i, err}
		}(i)
	}

	ra := make([]result, count)
	for i := 0; i < count; i++ {
		logger.Info("Waiting for result [%v] ...", i)
		ra[i] = <-results
	}
	for _, r := range ra {
		tests.Assert(t, r.err == nil,
			"expected r.err == nil, got:", r.err, "@", r.index)
	}
}

func TestBlockVolumeAlreadyDeleted(t *testing.T) {

	// Setup the VM storage topology
	setupCluster(t, 3, 4)
	defer teardownCluster(t)
	defer teardownBlock(t)

	blockReq := &api.BlockVolumeCreateRequest{}
	blockReq.Size = 3
	blockReq.Hacount = 3

	type result struct {
		index int
		err   error
	}

	bvol, err := heketi.BlockVolumeCreate(blockReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	hvol, err := heketi.VolumeInfo(bvol.BlockHostingVolume)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	host := storage0ssh
	cmd := fmt.Sprintf("gluster-block delete %v/%v --json",
		hvol.Name, bvol.Name)
	s := ssh.NewSshExecWithKeyFile(
		logger, "vagrant", "../config/insecure_private_key")
	o, err := s.ConnectAndExec(host, []string{cmd}, 10, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(o) == 1, "expected len(o) == 1, got:", len(o))

	err = heketi.BlockVolumeDelete(bvol.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestBlockVolumeDeleteFailureConditions(t *testing.T) {

	// Setup the VM storage topology
	setupCluster(t, 3, 4)
	defer teardownCluster(t)
	defer teardownBlock(t)

	blockReq := &api.BlockVolumeCreateRequest{}
	blockReq.Size = 3
	blockReq.Hacount = 3

	type result struct {
		index int
		err   error
	}

	bvol, err := heketi.BlockVolumeCreate(blockReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	_, err = heketi.BlockVolumeInfo(bvol.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	downHosts := []string{storage1ssh, storage2ssh}

	var cmds []string
	s := ssh.NewSshExecWithKeyFile(
		logger, "vagrant", "../config/insecure_private_key")
	stopServices := func(both bool) {
		logger.Info("Stopping services for test")
		for _, host := range downHosts {
			cmds = []string{
				"systemctl stop gluster-blockd",
			}
			if both {
				cmds = append(cmds, "systemctl stop glusterd")
			}
			_, err := s.ConnectAndExec(host, cmds, 10, true)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}
		time.Sleep(time.Second * 5)
	}
	startServices := func() {
		logger.Info("Starting services back up")
		for _, host := range downHosts {
			cmds = []string{
				"systemctl start glusterd",
				"systemctl start gluster-blockd",
			}
			_, err := s.ConnectAndExec(host, cmds, 10, true)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}
		time.Sleep(time.Second * 5)
	}

	defer startServices()

	l, err := heketi.BlockVolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(l.BlockVolumes) == 1,
		"expected len(l.BlockVolumes) == 1, got:", len(l.BlockVolumes))

	stopServices(false)
	// we need to try a few times in order to potentially run the command
	// on different gluster nodes. 10 is a nice round number
	for i := 0; i < 10; i++ {
		err = heketi.BlockVolumeDelete(bvol.Id)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	}

	stopServices(true)
	for i := 0; i < 10; i++ {
		err = heketi.BlockVolumeDelete(bvol.Id)
		tests.Assert(t, err != nil, "expected err != nil, got:", err)
	}

	startServices()
	_, err = heketi.BlockVolumeInfo(bvol.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	logger.Info("Doing real delete")
	err = heketi.BlockVolumeDelete(bvol.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}
