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

package testutils

import (
	"fmt"
	"os"
	"testing"

	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"

	"github.com/heketi/tests"
)

type ClusterEnv struct {
	Nodes   []string
	SSHPort string
	Disks   []string

	HeketiUrl string

	// helper callbacks
	CustomizeNodeRequest   func(int, *api.NodeAddRequest)
	CustomizeDeviceRequest func(*api.DeviceAddRequest)
}

func (ce *ClusterEnv) Update() {
	// update the port from the env
	env := os.Getenv("HEKETI_TEST_STORAGEPORT")
	if "" != env {
		ce.SSHPort = env
	}

	// update the node IPs from the env
	for i := 0; i < len(ce.Nodes); i++ {
		key := fmt.Sprintf("HEKETI_TEST_STORAGE%v", i)
		value := os.Getenv(key)
		if "" != value {
			ce.Nodes[i] = value
		}
	}
}

func (ce *ClusterEnv) Copy() *ClusterEnv {
	newce := ClusterEnv{}
	newce.SSHPort = ce.SSHPort
	newce.HeketiUrl = ce.HeketiUrl
	newce.Nodes = make([]string, len(ce.Nodes))
	copy(newce.Nodes, ce.Nodes)
	newce.Disks = make([]string, len(ce.Disks))
	copy(newce.Disks, ce.Disks)
	newce.CustomizeNodeRequest = ce.CustomizeNodeRequest
	newce.CustomizeDeviceRequest = ce.CustomizeDeviceRequest
	return &newce
}

func (ce *ClusterEnv) client() *client.Client {
	return client.NewClientNoAuth(ce.HeketiUrl)
}

func (ce *ClusterEnv) SshHost(index int) string {
	return ce.Nodes[index] + ":" + ce.SSHPort
}

// Setup creates a new test cluster using default cluster parameters
// and the given number of nodes and disks. The test environment
// must already be set up with a sufficient number of nodes and disks.
func (ce *ClusterEnv) Setup(t *testing.T, numNodes, numDisks int) {
	// Create a cluster
	req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	ce.SetupWithCluster(t, req, numNodes, numDisks)
}

// SetupWithCluster creates a new test cluster using the provided
// cluster parameters and the given number of nodes and disks.
// The test environment must already be set up with a sufficient
// number of nodes and disks.
func (ce *ClusterEnv) SetupWithCluster(
	t *testing.T, req *api.ClusterCreateRequest, numNodes, numDisks int) {

	heketi := ce.client()

	// As a testing invariant, we always expect to set up a cluster
	// at the start of a test on a _clean_ server.
	// Verify that there are no outstanding operations on the
	// server. A test that needs to mess with the operations _must_
	// clean up after itself.
	oi, err := heketi.OperationsInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, oi.Total == 0, "expected oi.Total == 0, got", oi.Total)
	tests.Assert(t, oi.InFlight == 0, "expected oi.InFlight == 0, got", oi.Total)

	cluster, err := heketi.ClusterCreate(req)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// hardcoded limits from the lists above
	// possible TODO: generalize
	tests.Assert(t, numNodes <= 4)
	tests.Assert(t, numDisks <= 8)

	// Add nodes
	for index, hostname := range ce.Nodes[:numNodes] {
		nodeReq := &api.NodeAddRequest{}
		nodeReq.ClusterId = cluster.Id
		nodeReq.Hostnames.Manage = []string{hostname}
		nodeReq.Hostnames.Storage = []string{hostname}
		nodeReq.Zone = index%2 + 1
		ce.customizeNodeReq(index, nodeReq)

		node, err := heketi.NodeAdd(nodeReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// Add devices
		sg := utils.NewStatusGroup()
		for _, disk := range ce.Disks[:numDisks] {
			sg.Add(1)
			go func(d string) {
				defer sg.Done()

				driveReq := &api.DeviceAddRequest{}
				driveReq.Name = d
				driveReq.NodeId = node.Id
				ce.customizeDeviceReq(driveReq)

				err := heketi.DeviceAdd(driveReq)
				sg.Err(err)
			}(disk)
		}

		err = sg.Result()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
}

func (ce *ClusterEnv) StateDump(t *testing.T) {
	heketi := ce.client()
	if t.Failed() {
		fmt.Println("~~~~~ dumping db state prior to teardown ~~~~~")
		dump, err := heketi.DbDump()
		if err != nil {
			fmt.Printf("Unable to get db dump: %v\n", err)
		} else {
			fmt.Printf("\n%v\n", dump)
		}
		fmt.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
	}
}

func (ce *ClusterEnv) VolumeTeardown(t *testing.T) {
	heketi := ce.client()
	fmt.Println("~~~ tearing down volumes")

	clusters, err := heketi.ClusterList()
	tests.Assert(t, err == nil, err)

	for _, cluster := range clusters.Clusters {

		clusterInfo, err := heketi.ClusterInfo(cluster)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// Delete block volumes in this cluster
		for _, bv := range clusterInfo.BlockVolumes {
			err := heketi.BlockVolumeDelete(bv)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}

		// Delete volumes in this cluster
		for _, volume := range clusterInfo.Volumes {
			err := heketi.VolumeDelete(volume)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}
	}
}

func (ce *ClusterEnv) Teardown(t *testing.T) {
	heketi := ce.client()
	fmt.Println("~~~ tearing down cluster")
	ce.StateDump(t)

	clusters, err := heketi.ClusterList()
	tests.Assert(t, err == nil, err)

	for _, cluster := range clusters.Clusters {

		clusterInfo, err := heketi.ClusterInfo(cluster)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		// Delete block volumes in this cluster
		for _, bv := range clusterInfo.BlockVolumes {
			err := heketi.BlockVolumeDelete(bv)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}

		// Delete volumes in this cluster
		for _, volume := range clusterInfo.Volumes {
			err := heketi.VolumeDelete(volume)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}

		// Delete nodes
		for _, node := range clusterInfo.Nodes {

			// Get node information
			nodeInfo, err := heketi.NodeInfo(node)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)

			// Delete each device
			sg := utils.NewStatusGroup()
			for _, device := range nodeInfo.DevicesInfo {
				sg.Add(1)
				go func(id string) {
					defer sg.Done()

					stateReq := &api.StateRequest{}
					stateReq.State = api.EntryStateOffline
					err := heketi.DeviceState(id, stateReq)
					if err != nil {
						sg.Err(err)
						return
					}

					stateReq.State = api.EntryStateFailed
					err = heketi.DeviceState(id, stateReq)
					if err != nil {
						sg.Err(err)
						return
					}

					err = heketi.DeviceDelete(id)
					sg.Err(err)

				}(device.Id)
			}
			err = sg.Result()
			tests.Assert(t, err == nil, "expected err == nil, got:", err)

			// Delete node
			err = heketi.NodeDelete(node)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
		}

		// Delete cluster
		err = heketi.ClusterDelete(cluster)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
}

func (ce *ClusterEnv) customizeNodeReq(i int, req *api.NodeAddRequest) {
	if ce.CustomizeNodeRequest != nil {
		ce.CustomizeNodeRequest(i, req)
	}
}

func (ce *ClusterEnv) customizeDeviceReq(req *api.DeviceAddRequest) {
	if ce.CustomizeDeviceRequest != nil {
		ce.CustomizeDeviceRequest(req)
	}
}
