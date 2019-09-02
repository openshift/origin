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
	"sync"
	"testing"
	"time"

	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"

	"github.com/heketi/tests"
)

func TestThrottledOps(t *testing.T) {

	teardownCluster(t)
	setupCluster(t, 3, 8)
	defer teardownCluster(t)

	t.Run("VolumeCreate", testThrottledVolumeCreate)
	teardownVolumes(t)
	t.Run("VolumeCreateFails", testThrottledVolumeCreateFails)
	teardownVolumes(t)
	t.Run("Removes", testThrottledRemoves)
}

func testThrottledVolumeCreate(t *testing.T) {
	// create a client with internal retries disabled
	// we will be able to use this to test that the server returned
	// 429 error responses
	hc := client.NewClientWithOptions(heketiUrl, "", "", client.ClientOptions{
		RetryEnabled: false,
	})

	oi, err := hc.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, oi.Total == 0, "expected oi.Total == 0, got", oi.Total)
	tests.Assert(t, oi.InFlight == 0, "expected oi.InFlight == 0, got", oi.Total)

	l := sync.Mutex{}
	errCount := 0
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 2
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	// create a bunch of volume requests at once
	sg := utils.NewStatusGroup()
	for i := 0; i < 12; i++ {
		sg.Add(1)
		go func() {
			defer sg.Done()
			_, err := hc.VolumeCreate(volReq)
			if err != nil {
				l.Lock()
				defer l.Unlock()
				errCount++
			}
			sg.Err(err)
		}()
	}

	sg.Result()
	tests.Assert(t, errCount > 1, "expected errCount > 1, got:", errCount)

	oi, err = hc.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, oi.Total == 0, "expected oi.Total == 0, got", oi.Total)
	tests.Assert(t, oi.InFlight == 0, "expected oi.InFlight == 0, got", oi.Total)

	volumes, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(volumes.Volumes) == 5,
		"expected len(volumes.Volumes) == 5, got:", len(volumes.Volumes))
}

func testThrottledVolumeCreateFails(t *testing.T) {
	oi, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, oi.Total == 0, "expected oi.Total == 0, got", oi.Total)
	tests.Assert(t, oi.InFlight == 0, "expected oi.InFlight == 0, got", oi.Total)

	l := sync.Mutex{}
	errCount := 0
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 300
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	// create a bunch of volume requests at once
	sg := utils.NewStatusGroup()
	for i := 0; i < 25; i++ {
		sg.Add(1)
		go func() {
			defer sg.Done()
			_, err := heketi.VolumeCreate(volReq)
			if err != nil {
				l.Lock()
				defer l.Unlock()
				errCount++
			}
			sg.Err(err)
		}()
	}

	sg.Result()
	tests.Assert(t, errCount > 1, "expected errCount > 1, got:", errCount)

	// there should not be any ops on the server now
	oi, err = heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, oi.Total == 0, "expected oi.Total == 0, got", oi.Total)
	tests.Assert(t, oi.InFlight == 0, "expected oi.InFlight == 0, got", oi.Total)

	// we use a count of the volumes as a proxy for determining how
	// many volume requests failed. We made 25 requests but should
	// only have been able to allocate a few. This tests two things:
	// - when the Operation's build step fails it decrements the op count
	// - that the scenario where large amount of requests come into
	//   the server and only a portion of them can ultimately be done
	volumes, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(volumes.Volumes) >= 8,
		"expected len(volumes.Volumes) >= 8, got:", len(volumes.Volumes))
	tests.Assert(t, len(volumes.Volumes) < 20,
		"expected len(volumes.Volumes) < 20, got:", len(volumes.Volumes))
}

// testThrottledRemoves is intended to test that the throttling behaves
// as expected for node and device remove (via SetState).
func testThrottledRemoves(t *testing.T) {
	// create clients with custom retry values
	hc := client.NewClientWithOptions(heketiUrl, "", "", client.ClientOptions{
		RetryEnabled:  true,
		RetryCount:    60, // allow large number of retries
		RetryMinDelay: 30, // do "slow" retries
		RetryMaxDelay: 60,
	})
	hc2 := client.NewClientWithOptions(heketiUrl, "", "", client.ClientOptions{
		RetryEnabled:  true,
		RetryCount:    60, // allow large number of retries
		RetryMinDelay: 2,  // allow "fast" retries
		RetryMaxDelay: 10,
	})

	oi, err := hc.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, oi.Total == 0, "expected oi.Total == 0, got", oi.Total)
	tests.Assert(t, oi.InFlight == 0, "expected oi.InFlight == 0, got", oi.Total)

	// pick a node and a device that will be disabled later
	var targetDevice, targetNode string
	clusters, err := heketi.ClusterList()
	tests.Assert(t, err == nil, err)
	clusterInfo, err := hc.ClusterInfo(clusters.Clusters[0])
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	for _, node := range clusterInfo.Nodes {
		nodeInfo, err := heketi.NodeInfo(node)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		if len(nodeInfo.DevicesInfo) > 0 {
			targetDevice = nodeInfo.DevicesInfo[0].Id
			targetNode = node
			break
		}
	}
	tests.Assert(t, targetDevice != "")
	tests.Assert(t, targetNode != "")

	// we offline the node now in order to avoid using it for volumes
	// as we are testing the in-flight operations not device conflicts
	stateReq := &api.StateRequest{}
	stateReq.State = api.EntryStateOffline
	err = hc2.NodeState(targetNode, stateReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	l := sync.Mutex{}
	errCount := 0
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1
	volReq.Durability.Type = api.DurabilityDistributeOnly

	// create a bunch of volume requests at once
	sg := utils.NewStatusGroup()
	go func() {
		for i := 0; i < 15; i++ {
			sg.Add(1)
			go func() {
				defer sg.Done()
				_, err := hc.VolumeCreate(volReq)
				if err != nil {
					l.Lock()
					defer l.Unlock()
					errCount++
				}
				sg.Err(err)
			}()
		}
	}()
	sg.Add(1)
	go func() {
		defer sg.Done()
		// allow for volume create requests to get in first
		time.Sleep(500 * time.Millisecond)

		// do a device remove
		stateReq := &api.StateRequest{}
		stateReq.State = api.EntryStateOffline
		err := hc2.DeviceState(targetDevice, stateReq)
		if err != nil {
			l.Lock()
			defer l.Unlock()
			errCount++
			sg.Err(err)
			return
		}

		stateReq.State = api.EntryStateFailed
		err = hc2.DeviceState(targetDevice, stateReq)
		if err != nil {
			l.Lock()
			defer l.Unlock()
			errCount++
			sg.Err(err)
			return
		}

		// pause again for a short moment
		time.Sleep(500 * time.Millisecond)

		// do a node remove
		stateReq = &api.StateRequest{}
		stateReq.State = api.EntryStateOffline
		err = hc2.NodeState(targetNode, stateReq)
		if err != nil {
			l.Lock()
			defer l.Unlock()
			errCount++
			sg.Err(err)
			return
		}

		stateReq.State = api.EntryStateFailed
		err = hc2.NodeState(targetNode, stateReq)
		if err != nil {
			l.Lock()
			defer l.Unlock()
			errCount++
			sg.Err(err)
			return
		}
	}()

	sg.Result()
	tests.Assert(t, errCount == 0, "expected errCount == 0, got:", errCount)

	// there should not be any ops on the server now
	oi, err = hc.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, oi.Total == 0, "expected oi.Total == 0, got", oi.Total)
	tests.Assert(t, oi.InFlight == 0, "expected oi.InFlight == 0, got", oi.Total)

	volumes, err := hc.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(volumes.Volumes) == 15,
		"expected len(volumes.Volumes) == 5, got:", len(volumes.Volumes))
}
