//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/gorilla/mux"
	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/middleware"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/idgen"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/heketi/server/admin"
	"github.com/heketi/tests"
	"github.com/urfave/negroni"
)

const (
	TEST_ADMIN_KEY = "adminkey"
)

type clientTestMiddleware struct {
	intercept func(w http.ResponseWriter, r *http.Request) bool
}

func (cm *clientTestMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if cm.intercept != nil && cm.intercept(w, r) {
		return
	}
	next(w, r)
}

func setupHeketiServer(app *glusterfs.App) *httptest.Server {
	return setupHeketiServerAndMiddleware(app, nil)
}

func setupHeketiServerAndMiddleware(
	app *glusterfs.App, m negroni.Handler) *httptest.Server {

	router := mux.NewRouter()
	app.SetRoutes(router)
	n := negroni.New()

	jwtconfig := &middleware.JwtAuthConfig{}
	jwtconfig.Admin.PrivateKey = TEST_ADMIN_KEY
	jwtconfig.User.PrivateKey = "userkey"

	adminss := admin.New()
	adminss.SetRoutes(router)

	// Setup middleware
	n.Use(middleware.NewJwtAuth(jwtconfig))
	if m != nil {
		n.Use(m)
	}
	n.Use(adminss)
	n.UseFunc(app.Auth)
	n.UseHandler(router)

	// Create server
	return httptest.NewServer(n)
}

func TestTopology(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster correctly
	c := NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil)

	//Create multiple clusters
	clusteridlist := make([]api.ClusterInfoResponse, 0)
	cluster_req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	for m := 0; m < 4; m++ {
		cluster, err := c.ClusterCreate(cluster_req)
		tests.Assert(t, err == nil)
		tests.Assert(t, cluster.Id != "")
		clusteridlist = append(clusteridlist, *cluster)
	}
	tests.Assert(t, len(clusteridlist) == 4)

	//Verify the topology info and then delete the clusters
	topology, err := c.TopologyInfo()
	tests.Assert(t, err == nil)
	for _, cid := range topology.ClusterList {
		clusterid := cid.Id
		err = c.ClusterDelete(clusterid)
		tests.Assert(t, err == nil)
	}

	//Create a cluster and add multiple nodes,devices and volumes
	cluster_req = &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	cluster, err := c.ClusterCreate(cluster_req)
	tests.Assert(t, err == nil)
	tests.Assert(t, cluster.Id != "")
	tests.Assert(t, len(cluster.Nodes) == 0)
	tests.Assert(t, len(cluster.Volumes) == 0)
	tests.Assert(t, len(cluster.BlockVolumes) == 0)

	// Get information about the client
	clusterinfo, err := c.ClusterInfo(cluster.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(clusterinfo, cluster))

	// Get information about the Topology and verify the cluster creation
	topology, err = c.TopologyInfo()
	tests.Assert(t, err == nil)
	tests.Assert(t, topology.ClusterList[0].Id == cluster.Id)

	// Create multiple nodes and add devices to the nodes
	nodeinfos := make([]api.NodeInfoResponse, 0)
	for n := 0; n < 4; n++ {
		nodeReq := &api.NodeAddRequest{}
		nodeReq.ClusterId = cluster.Id
		nodeReq.Hostnames.Manage = []string{"manage" + fmt.Sprintf("%v", n)}
		nodeReq.Hostnames.Storage = []string{"storage" + fmt.Sprintf("%v", n)}
		nodeReq.Zone = n + 1

		// Create node
		node, err := c.NodeAdd(nodeReq)
		nodeinfos = append(nodeinfos, *node)
		tests.Assert(t, err == nil)

		// Create a device request
		sg := utils.NewStatusGroup()
		for i := 0; i < 50; i++ {
			sg.Add(1)
			go func() {
				defer sg.Done()

				deviceReq := &api.DeviceAddRequest{}
				deviceReq.Name = "/sd" + idgen.GenUUID()
				deviceReq.NodeId = node.Id

				// Create device
				err := c.DeviceAdd(deviceReq)
				sg.Err(err)
			}()
		}
		tests.Assert(t, sg.Result() == nil)
	}
	tests.Assert(t, len(nodeinfos) != 0)

	// Get list of volumes
	list, err := c.VolumeList()
	tests.Assert(t, err == nil)
	tests.Assert(t, len(list.Volumes) == 0)

	//Create multiple volumes to the cluster
	volumeinfos := make([]api.VolumeInfoResponse, 0)
	for n := 0; n < 4; n++ {
		volumeReq := &api.VolumeCreateRequest{}
		volumeReq.Size = 10
		volume, err := c.VolumeCreate(volumeReq)
		tests.Assert(t, err == nil)
		tests.Assert(t, volume.Id != "")
		tests.Assert(t, volume.Size == volumeReq.Size)
		volumeinfos = append(volumeinfos, *volume)
	}
	topology, err = c.TopologyInfo()
	tests.Assert(t, err == nil)

	// Test topology have all the existing volumes
	var volumefound int
	for _, volumeid := range topology.ClusterList[0].Volumes {
		volumeInfo := volumeid
		for _, singlevolumei := range volumeinfos {
			if singlevolumei.Id == volumeInfo.Id {
				volumefound++
				break
			}
		}
	}
	tests.Assert(t, volumefound == 4)

	// Delete all the volumes
	for _, volumeid := range topology.ClusterList[0].Volumes {
		volumeInfo := volumeid
		err = c.VolumeDelete(volumeInfo.Id)
		tests.Assert(t, err == nil)

	}

	// Verify the nodes and devices info from topology info and delete the entries
	for _, nodeid := range topology.ClusterList[0].Nodes {
		nodeInfo := nodeid
		var found bool
		for _, singlenodei := range nodeinfos {
			found = false
			if singlenodei.Id == nodeInfo.Id {
				found = true
				break
			}
		}
		tests.Assert(t, found == true)

		// Change device state to offline
		sg := utils.NewStatusGroup()
		for index := range nodeInfo.DevicesInfo {
			sg.Add(1)
			go func(i int) {
				defer sg.Done()
				sg.Err(c.DeviceState(nodeInfo.DevicesInfo[i].Id, &api.StateRequest{State: api.EntryStateOffline}))
			}(index)
		}
		err = sg.Result()
		tests.Assert(t, err == nil, err)

		// Change device state to failed
		sg = utils.NewStatusGroup()
		for index := range nodeInfo.DevicesInfo {
			sg.Add(1)
			go func(i int) {
				defer sg.Done()
				sg.Err(c.DeviceState(nodeInfo.DevicesInfo[i].Id, &api.StateRequest{State: api.EntryStateFailed}))
			}(index)
		}
		err = sg.Result()
		tests.Assert(t, err == nil, err)

		// Delete all devices
		sg = utils.NewStatusGroup()
		for index := range nodeInfo.DevicesInfo {
			sg.Add(1)
			go func(i int) {
				defer sg.Done()
				sg.Err(c.DeviceDelete(nodeInfo.DevicesInfo[i].Id))
			}(index)
		}
		err = sg.Result()
		tests.Assert(t, err == nil, err)

		// Delete node
		err = c.NodeDelete(nodeInfo.Id)
		tests.Assert(t, err == nil)

	}

	// Delete cluster
	err = c.ClusterDelete(cluster.Id)
	tests.Assert(t, err == nil)

}

func TestClientCluster(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster with unknown user
	c := NewClient(ts.URL, "asdf", "")
	tests.Assert(t, c != nil)
	cluster_req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	cluster, err := c.ClusterCreate(cluster_req)
	tests.Assert(t, err != nil)
	tests.Assert(t, cluster == nil)

	// Create cluster with bad password
	c = NewClient(ts.URL, "admin", "badpassword")
	tests.Assert(t, c != nil)
	cluster, err = c.ClusterCreate(cluster_req)
	tests.Assert(t, err != nil)
	tests.Assert(t, cluster == nil)

	// Create cluster correctly
	c = NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil)
	cluster, err = c.ClusterCreate(cluster_req)
	tests.Assert(t, err == nil)
	tests.Assert(t, cluster.Id != "")
	tests.Assert(t, len(cluster.Nodes) == 0)
	tests.Assert(t, len(cluster.Volumes) == 0)

	// Request bad id
	info, err := c.ClusterInfo("bad")
	tests.Assert(t, err != nil)
	tests.Assert(t, info == nil)

	// Get information about the cluster
	info, err = c.ClusterInfo(cluster.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(info, cluster))

	// Set flags on the cluster
	cluster_setflags_req := &api.ClusterSetFlagsRequest{
		ClusterFlags: api.ClusterFlags{
			File:  true,
			Block: false,
		},
	}
	err = c.ClusterSetFlags(cluster.Id, cluster_setflags_req)
	tests.Assert(t, err == nil, err)

	// Check flags result
	info, err = c.ClusterInfo(cluster.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, info.File == true)
	tests.Assert(t, info.Block == false)

	// Get a list of clusters
	list, err := c.ClusterList()
	tests.Assert(t, err == nil)
	tests.Assert(t, len(list.Clusters) == 1)
	tests.Assert(t, list.Clusters[0] == info.Id)

	// Delete non-existent cluster
	err = c.ClusterDelete("badid")
	tests.Assert(t, err != nil)

	// Delete current cluster
	err = c.ClusterDelete(info.Id)
	tests.Assert(t, err == nil)
}

func TestClientNode(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster
	c := NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil)
	cluster_req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	cluster, err := c.ClusterCreate(cluster_req)
	tests.Assert(t, err == nil)
	tests.Assert(t, cluster.Id != "")
	tests.Assert(t, len(cluster.Nodes) == 0)
	tests.Assert(t, len(cluster.Volumes) == 0)

	// Add node to unknown cluster
	nodeReq := &api.NodeAddRequest{}
	nodeReq.ClusterId = "badid"
	nodeReq.Hostnames.Manage = []string{"manage"}
	nodeReq.Hostnames.Storage = []string{"storage"}
	nodeReq.Zone = 10
	_, err = c.NodeAdd(nodeReq)
	tests.Assert(t, err != nil)

	// Create node request packet
	nodeReq.ClusterId = cluster.Id
	node, err := c.NodeAdd(nodeReq)
	tests.Assert(t, err == nil)
	tests.Assert(t, node.Zone == nodeReq.Zone)
	tests.Assert(t, node.State == api.EntryStateOnline)
	tests.Assert(t, node.Id != "")
	tests.Assert(t, reflect.DeepEqual(nodeReq.Hostnames, node.Hostnames))
	tests.Assert(t, len(node.DevicesInfo) == 0)

	// Info on invalid id
	info, err := c.NodeInfo("badid")
	tests.Assert(t, err != nil)
	tests.Assert(t, info == nil)

	// Set offline
	err = c.NodeState(node.Id, &api.StateRequest{
		State: api.EntryStateOffline,
	})
	tests.Assert(t, err == nil)

	// Get node info
	info, err = c.NodeInfo(node.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, info.State == api.EntryStateOffline)

	// Set online
	err = c.NodeState(node.Id, &api.StateRequest{
		State: api.EntryStateOnline,
	})
	tests.Assert(t, err == nil)

	// Get node info
	info, err = c.NodeInfo(node.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, info.State == api.EntryStateOnline)
	tests.Assert(t, reflect.DeepEqual(info, node))

	// Delete invalid node
	err = c.NodeDelete("badid")
	tests.Assert(t, err != nil)

	// Can't delete cluster with a node
	err = c.ClusterDelete(cluster.Id)
	tests.Assert(t, err != nil)

	// Delete node
	err = c.NodeDelete(node.Id)
	tests.Assert(t, err == nil)

	// Delete cluster
	err = c.ClusterDelete(cluster.Id)
	tests.Assert(t, err == nil)

}

func TestClientDevice(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster
	c := NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil)
	cluster_req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	cluster, err := c.ClusterCreate(cluster_req)
	tests.Assert(t, err == nil)

	// Create node request packet
	nodeReq := &api.NodeAddRequest{}
	nodeReq.ClusterId = cluster.Id
	nodeReq.Hostnames.Manage = []string{"manage"}
	nodeReq.Hostnames.Storage = []string{"storage"}
	nodeReq.Zone = 10

	// Create node
	node, err := c.NodeAdd(nodeReq)
	tests.Assert(t, err == nil)

	// Create a device request
	deviceReq := &api.DeviceAddRequest{}
	deviceReq.Name = "/sda"
	deviceReq.NodeId = node.Id

	// Create device
	err = c.DeviceAdd(deviceReq)
	tests.Assert(t, err == nil)

	// Get node information
	info, err := c.NodeInfo(node.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, len(info.DevicesInfo) == 1)
	tests.Assert(t, len(info.DevicesInfo[0].Bricks) == 0)
	tests.Assert(t, info.DevicesInfo[0].Name == deviceReq.Name)
	tests.Assert(t, info.DevicesInfo[0].Id != "")

	// Get info from an unknown id
	_, err = c.DeviceInfo("badid")
	tests.Assert(t, err != nil)

	// Get device information
	deviceId := info.DevicesInfo[0].Id
	deviceInfo, err := c.DeviceInfo(deviceId)
	tests.Assert(t, err == nil)
	tests.Assert(t, deviceInfo.State == api.EntryStateOnline)
	tests.Assert(t, reflect.DeepEqual(*deviceInfo, info.DevicesInfo[0]))

	// Set offline
	err = c.DeviceState(deviceId, &api.StateRequest{
		State: api.EntryStateOffline,
	})
	tests.Assert(t, err == nil, err)
	deviceInfo, err = c.DeviceInfo(deviceId)
	tests.Assert(t, err == nil)
	tests.Assert(t, deviceInfo.State == api.EntryStateOffline)

	// Set online
	err = c.DeviceState(deviceId, &api.StateRequest{
		State: api.EntryStateOnline,
	})
	tests.Assert(t, err == nil)
	deviceInfo, err = c.DeviceInfo(deviceId)
	tests.Assert(t, err == nil)
	tests.Assert(t, deviceInfo.State == api.EntryStateOnline)

	// Resync
	err = c.DeviceResync(deviceId)
	tests.Assert(t, err == nil)
	deviceInfo, err = c.DeviceInfo(deviceId)
	tests.Assert(t, err == nil)
	tests.Assert(t, deviceInfo.Storage.Total == 500*1024*1024)
	tests.Assert(t, deviceInfo.Storage.Free == 500*1024*1024)
	tests.Assert(t, deviceInfo.Storage.Used == 0)

	// Try to delete node, and will not until we delete the device
	err = c.NodeDelete(node.Id)
	tests.Assert(t, err != nil)

	// Delete unknown device
	err = c.DeviceDelete("badid")
	tests.Assert(t, err != nil)

	// Offline Device
	err = c.DeviceState(deviceInfo.Id, &api.StateRequest{State: api.EntryStateOffline})
	tests.Assert(t, err == nil)
	// Fail Device
	err = c.DeviceState(deviceInfo.Id, &api.StateRequest{State: api.EntryStateFailed})
	tests.Assert(t, err == nil)

	// Delete device
	err = c.DeviceDelete(deviceInfo.Id)
	tests.Assert(t, err == nil)

	// Delete node
	err = c.NodeDelete(node.Id)
	tests.Assert(t, err == nil)

	// Delete cluster
	err = c.ClusterDelete(cluster.Id)
	tests.Assert(t, err == nil)

}

func TestClientVolume(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster
	c := NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil)
	cluster_req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	cluster, err := c.ClusterCreate(cluster_req)
	tests.Assert(t, err == nil)

	// Create node request packet
	for n := 0; n < 4; n++ {
		nodeReq := &api.NodeAddRequest{}
		nodeReq.ClusterId = cluster.Id
		nodeReq.Hostnames.Manage = []string{"manage" + fmt.Sprintf("%v", n)}
		nodeReq.Hostnames.Storage = []string{"storage" + fmt.Sprintf("%v", n)}
		nodeReq.Zone = n + 1

		// Create node
		node, err := c.NodeAdd(nodeReq)
		tests.Assert(t, err == nil)

		// Create a device request
		sg := utils.NewStatusGroup()
		for i := 0; i < 50; i++ {
			sg.Add(1)
			go func() {
				defer sg.Done()

				deviceReq := &api.DeviceAddRequest{}
				deviceReq.Name = "/dev/by-magic/id:" + idgen.GenUUID()
				deviceReq.NodeId = node.Id

				// Create device
				err := c.DeviceAdd(deviceReq)
				sg.Err(err)

			}()
		}
		r := sg.Result()
		tests.Assert(t, r == nil, r)
	}

	// Get list of volumes
	list, err := c.VolumeList()
	tests.Assert(t, err == nil)
	tests.Assert(t, len(list.Volumes) == 0)

	// Create a volume
	volumeReq := &api.VolumeCreateRequest{}
	volumeReq.Size = 10
	volume, err := c.VolumeCreate(volumeReq)
	tests.Assert(t, err == nil)
	tests.Assert(t, volume.Id != "")
	tests.Assert(t, volume.Size == volumeReq.Size)

	// Get list of volumes
	list, err = c.VolumeList()
	tests.Assert(t, err == nil)
	tests.Assert(t, len(list.Volumes) == 1)
	tests.Assert(t, list.Volumes[0] == volume.Id)

	// Get info on incorrect id
	info, err := c.VolumeInfo("badid")
	tests.Assert(t, err != nil)

	// Get info
	info, err = c.VolumeInfo(volume.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, reflect.DeepEqual(info, volume))

	// Expand volume with a bad id
	expandReq := &api.VolumeExpandRequest{}
	expandReq.Size = 10
	volumeInfo, err := c.VolumeExpand("badid", expandReq)
	tests.Assert(t, err != nil)

	// Expand volume
	volumeInfo, err = c.VolumeExpand(volume.Id, expandReq)
	tests.Assert(t, err == nil)
	tests.Assert(t, volumeInfo.Size == 20)

	// Delete bad id
	err = c.VolumeDelete("badid")
	tests.Assert(t, err != nil)

	// Delete volume
	err = c.VolumeDelete(volume.Id)
	tests.Assert(t, err == nil)

	clusterInfo, err := c.ClusterInfo(cluster.Id)
	for _, nodeid := range clusterInfo.Nodes {
		// Get node information
		nodeInfo, err := c.NodeInfo(nodeid)
		tests.Assert(t, err == nil)

		// Change device state to offline
		sg := utils.NewStatusGroup()
		for index := range nodeInfo.DevicesInfo {
			sg.Add(1)
			go func(i int) {
				defer sg.Done()
				sg.Err(c.DeviceState(nodeInfo.DevicesInfo[i].Id, &api.StateRequest{State: api.EntryStateOffline}))
			}(index)
		}
		err = sg.Result()
		tests.Assert(t, err == nil, err)

		// Change device state to failed
		sg = utils.NewStatusGroup()
		for index := range nodeInfo.DevicesInfo {
			sg.Add(1)
			go func(i int) {
				defer sg.Done()
				sg.Err(c.DeviceState(nodeInfo.DevicesInfo[i].Id, &api.StateRequest{State: api.EntryStateFailed}))
			}(index)
		}
		err = sg.Result()
		tests.Assert(t, err == nil, err)

		// Delete all devices
		sg = utils.NewStatusGroup()
		for index := range nodeInfo.DevicesInfo {
			sg.Add(1)
			go func(i int) {
				defer sg.Done()
				sg.Err(c.DeviceDelete(nodeInfo.DevicesInfo[i].Id))
			}(index)
		}
		err = sg.Result()
		tests.Assert(t, err == nil, err)

		// Delete node
		err = c.NodeDelete(nodeid)
		tests.Assert(t, err == nil)

	}

	// Delete cluster
	err = c.ClusterDelete(cluster.Id)
	tests.Assert(t, err == nil)

}

func TestLogLevel(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster
	c := NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil, "NewClient failed:", c)
	llinfo, err := c.LogLevelGet()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, llinfo.LogLevel["glusterfs"] == "info",
		`expected llinfo.LogLevel["glusterfs"] == "info", get:`, llinfo.LogLevel)

	llinfo.LogLevel["glusterfs"] = "debug"
	err = c.LogLevelSet(llinfo)
	tests.Assert(t, err == nil, "unexpected error running c.LogLevelSet:", err)

	llinfo, err = c.LogLevelGet()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, llinfo.LogLevel["glusterfs"] == "debug",
		`expected llinfo.LogLevel["glusterfs"] == "debug", get:`, llinfo.LogLevel)

	llinfo.LogLevel["glusterfs"] = "bingo"
	err = c.LogLevelSet(llinfo)
	tests.Assert(t, err != nil, "expected error running c.LogLevelSet:", err)

	llinfo, err = c.LogLevelGet()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, llinfo.LogLevel["glusterfs"] == "debug",
		`expected llinfo.LogLevel["glusterfs"] == "debug", get:`, llinfo.LogLevel)

	llinfo.LogLevel["glusterfs"] = "info"
	err = c.LogLevelSet(llinfo)
	tests.Assert(t, err == nil, "unexpected error running c.LogLevelSet:", err)

	llinfo, err = c.LogLevelGet()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, llinfo.LogLevel["glusterfs"] == "info",
		`expected llinfo.LogLevel["glusterfs"] == "info", get:`, llinfo.LogLevel)
	return
}

func TestDeviceTags(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster
	c := NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil)
	cluster_req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	cluster, err := c.ClusterCreate(cluster_req)
	tests.Assert(t, err == nil)

	// Create node request packet
	nodeReq := &api.NodeAddRequest{}
	nodeReq.ClusterId = cluster.Id
	nodeReq.Hostnames.Manage = []string{"manage"}
	nodeReq.Hostnames.Storage = []string{"storage"}
	nodeReq.Zone = 10

	// Create node
	node, err := c.NodeAdd(nodeReq)
	tests.Assert(t, err == nil)

	// Create a device request
	deviceReq := &api.DeviceAddRequest{}
	deviceReq.Name = "/sda"
	deviceReq.NodeId = node.Id
	deviceReq.Tags = map[string]string{
		"weight": "light",
		"fish":   "cod",
	}

	// Create device
	err = c.DeviceAdd(deviceReq)
	tests.Assert(t, err == nil)

	// Get node information
	nodeInfo, err := c.NodeInfo(node.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, nodeInfo.DevicesInfo[0].Id != "")

	devId := nodeInfo.DevicesInfo[0].Id
	tests.Assert(t, len(devId) > 1)

	deviceInfo, err := c.DeviceInfo(devId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(deviceInfo.Tags) == 2,
		"expected len(deviceInfo.Tags) == 2, got:", len(deviceInfo.Tags))
	tests.Assert(t, deviceInfo.Tags["fish"] == "cod",
		`expected deviceInfo.Tags["fish"] == "cod", got:`,
		deviceInfo.Tags["fish"])

	err = c.DeviceSetTags(devId, &api.TagsChangeRequest{
		Change: api.UpdateTags,
		Tags: map[string]string{
			"metal": "iron",
		},
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	deviceInfo, err = c.DeviceInfo(devId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(deviceInfo.Tags) == 3,
		"expected len(deviceInfo.Tags) == 3, got:", len(deviceInfo.Tags))
	tests.Assert(t, deviceInfo.Tags["fish"] == "cod",
		`expected deviceInfo.Tags["fish"] == "cod", got:`,
		deviceInfo.Tags["fish"])
	tests.Assert(t, deviceInfo.Tags["metal"] == "iron",
		`expected deviceInfo.Tags["metal"] == "iron", got:`,
		deviceInfo.Tags["metal"])

	err = c.DeviceSetTags(devId, &api.TagsChangeRequest{
		Change: api.UpdateTags,
		Tags: map[string]string{
			"metal": "nickel",
		},
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	deviceInfo, err = c.DeviceInfo(devId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(deviceInfo.Tags) == 3,
		"expected len(deviceInfo.Tags) == 3, got:", len(deviceInfo.Tags))
	tests.Assert(t, deviceInfo.Tags["fish"] == "cod",
		`expected deviceInfo.Tags["fish"] == "cod", got:`,
		deviceInfo.Tags["fish"])
	tests.Assert(t, deviceInfo.Tags["metal"] == "nickel",
		`expected deviceInfo.Tags["metal"] == "nickel", got:`,
		deviceInfo.Tags["metal"])

	err = c.DeviceSetTags(devId, &api.TagsChangeRequest{
		Change: api.DeleteTags,
		Tags: map[string]string{
			"weight": "",
		},
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	deviceInfo, err = c.DeviceInfo(devId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(deviceInfo.Tags) == 2,
		"expected len(deviceInfo.Tags) == 2, got:", len(deviceInfo.Tags))
	tests.Assert(t, deviceInfo.Tags["fish"] == "cod",
		`expected deviceInfo.Tags["fish"] == "cod", got:`,
		deviceInfo.Tags["fish"])
	tests.Assert(t, deviceInfo.Tags["metal"] == "nickel",
		`expected deviceInfo.Tags["metal"] == "nickel", got:`,
		deviceInfo.Tags["metal"])

	err = c.DeviceSetTags(devId, &api.TagsChangeRequest{
		Change: api.SetTags,
		Tags: map[string]string{
			"metal": "heavy",
		},
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	deviceInfo, err = c.DeviceInfo(devId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(deviceInfo.Tags) == 1,
		"expected len(deviceInfo.Tags) == 1, got:", len(deviceInfo.Tags))
	tests.Assert(t, deviceInfo.Tags["metal"] == "heavy",
		`expected deviceInfo.Tags["metal"] == "heavy", got:`,
		deviceInfo.Tags["metal"])

	// Offline Device
	err = c.DeviceState(devId, &api.StateRequest{State: api.EntryStateOffline})
	tests.Assert(t, err == nil)
	// Fail Device
	err = c.DeviceState(devId, &api.StateRequest{State: api.EntryStateFailed})
	tests.Assert(t, err == nil)

	// Delete device
	err = c.DeviceDelete(devId)
	tests.Assert(t, err == nil)

	// Delete node
	err = c.NodeDelete(node.Id)
	tests.Assert(t, err == nil)

	// Delete cluster
	err = c.ClusterDelete(cluster.Id)
	tests.Assert(t, err == nil)
}

func TestNodeTags(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster
	c := NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil)
	cluster_req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	cluster, err := c.ClusterCreate(cluster_req)
	tests.Assert(t, err == nil)

	// Create node request packet
	nodeReq := &api.NodeAddRequest{}
	nodeReq.ClusterId = cluster.Id
	nodeReq.Hostnames.Manage = []string{"manage"}
	nodeReq.Hostnames.Storage = []string{"storage"}
	nodeReq.Zone = 10
	nodeReq.Tags = map[string]string{
		"weight": "100tons",
		"fish":   "cod",
	}

	// Create node
	node, err := c.NodeAdd(nodeReq)
	tests.Assert(t, err == nil)
	nodeId := node.Id

	// Get node information
	nodeInfo, err := c.NodeInfo(nodeId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(nodeInfo.Tags) == 2,
		"expected len(nodeInfo.Tags) == 2, got:", len(nodeInfo.Tags))
	tests.Assert(t, nodeInfo.Tags["fish"] == "cod",
		`expected nodeInfo.Tags["fish"] == "cod", got:`,
		nodeInfo.Tags["fish"])

	err = c.NodeSetTags(nodeId, &api.TagsChangeRequest{
		Change: api.UpdateTags,
		Tags: map[string]string{
			"metal": "iron",
		},
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	nodeInfo, err = c.NodeInfo(nodeId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(nodeInfo.Tags) == 3,
		"expected len(nodeInfo.Tags) == 3, got:", len(nodeInfo.Tags))
	tests.Assert(t, nodeInfo.Tags["fish"] == "cod",
		`expected nodeInfo.Tags["fish"] == "cod", got:`,
		nodeInfo.Tags["fish"])
	tests.Assert(t, nodeInfo.Tags["metal"] == "iron",
		`expected nodeInfo.Tags["metal"] == "iron", got:`,
		nodeInfo.Tags["metal"])

	err = c.NodeSetTags(nodeId, &api.TagsChangeRequest{
		Change: api.UpdateTags,
		Tags: map[string]string{
			"metal": "nickel",
		},
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	nodeInfo, err = c.NodeInfo(nodeId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(nodeInfo.Tags) == 3,
		"expected len(nodeInfo.Tags) == 3, got:", len(nodeInfo.Tags))
	tests.Assert(t, nodeInfo.Tags["fish"] == "cod",
		`expected nodeInfo.Tags["fish"] == "cod", got:`,
		nodeInfo.Tags["fish"])
	tests.Assert(t, nodeInfo.Tags["metal"] == "nickel",
		`expected nodeInfo.Tags["metal"] == "nickel", got:`,
		nodeInfo.Tags["metal"])

	err = c.NodeSetTags(nodeId, &api.TagsChangeRequest{
		Change: api.DeleteTags,
		Tags: map[string]string{
			"weight": "",
		},
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	nodeInfo, err = c.NodeInfo(nodeId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(nodeInfo.Tags) == 2,
		"expected len(nodeInfo.Tags) == 2, got:", len(nodeInfo.Tags))
	tests.Assert(t, nodeInfo.Tags["fish"] == "cod",
		`expected nodeInfo.Tags["fish"] == "cod", got:`,
		nodeInfo.Tags["fish"])
	tests.Assert(t, nodeInfo.Tags["metal"] == "nickel",
		`expected nodeInfo.Tags["metal"] == "nickel", got:`,
		nodeInfo.Tags["metal"])

	err = c.NodeSetTags(nodeId, &api.TagsChangeRequest{
		Change: api.SetTags,
		Tags: map[string]string{
			"metal": "heavy",
		},
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	nodeInfo, err = c.NodeInfo(nodeId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(nodeInfo.Tags) == 1,
		"expected len(nodeInfo.Tags) == 1, got:", len(nodeInfo.Tags))
	tests.Assert(t, nodeInfo.Tags["metal"] == "heavy",
		`expected nodeInfo.Tags["metal"] == "heavy", got:`,
		nodeInfo.Tags["metal"])

	// Delete node
	err = c.NodeDelete(node.Id)
	tests.Assert(t, err == nil)

	// Delete cluster
	err = c.ClusterDelete(cluster.Id)
	tests.Assert(t, err == nil)

}

func TestRetryAfterThrottle(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	m := &clientTestMiddleware{}
	ts := setupHeketiServerAndMiddleware(app, m)
	defer ts.Close()

	c := NewClientWithOptions(ts.URL, "admin", TEST_ADMIN_KEY, ClientOptions{
		RetryEnabled:  true,
		RetryCount:    RETRY_COUNT,
		RetryMinDelay: 1, // this is a test. we want short delays
		RetryMaxDelay: 2,
	})

	cluster_req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}

	cluster, err := c.ClusterCreate(cluster_req)
	tests.Assert(t, err == nil)

	ictr := 0
	permitAfter := 2
	retryAfterHeader := -1 // test with built in delay calculation
	m.intercept = func(w http.ResponseWriter, r *http.Request) bool {
		ictr++
		if ictr >= permitAfter {
			return false
		}
		if retryAfterHeader != -1 {
			w.Header().Set("Retry-After", fmt.Sprintf("%v", retryAfterHeader))
		}
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return true
	}

	nodeReq := &api.NodeAddRequest{}
	nodeReq.ClusterId = cluster.Id
	nodeReq.Hostnames.Manage = []string{"manage1"}
	nodeReq.Hostnames.Storage = []string{"storage1"}
	nodeReq.Zone = 1

	// verify that the NodeAdd command had to retry a few times but succeeds
	node, err := c.NodeAdd(nodeReq)
	tests.Assert(t, err == nil, "expected err == nil, got", err)
	tests.Assert(t, node != nil)
	tests.Assert(t, ictr >= 2, "expected ictr >= 2, got", ictr)

	// verify that the NodeAdd command fails after exhausting retries
	ictr = 0
	permitAfter = 200
	retryAfterHeader = 0 // test with delay based on server header
	nodeReq.Hostnames.Manage = []string{"manage2"}
	nodeReq.Hostnames.Storage = []string{"storage2"}
	_, err = c.NodeAdd(nodeReq)
	tests.Assert(t, err != nil, "expected err != nil, got", err)
	tests.Assert(t, err.Error() == "Too Many Requests", "expected err == 'Too Many Requests', got", err)
	tests.Assert(t, ictr >= RETRY_COUNT, "expected ictr >= 2, got", ictr)

	// disable internal retries
	c = NewClientWithOptions(ts.URL, "admin", TEST_ADMIN_KEY, ClientOptions{
		RetryEnabled:  false,
		RetryCount:    RETRY_COUNT,
		RetryMinDelay: 1, // this is a test. we want short delays
		RetryMaxDelay: 2,
	})

	// verify that the NodeAdd command fails without doing retries
	ictr = 0
	permitAfter = 200
	_, err = c.NodeAdd(nodeReq)
	tests.Assert(t, err != nil, "expected err != nil, got", err)
	tests.Assert(t, err.Error() == "Too Many Requests", "expected err == 'Too Many Requests', got", err)
	tests.Assert(t, ictr == 1, "expected ictr == 1, got", ictr)
}

func TestAdminStatus(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	c := NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil, "NewClient failed:", c)

	as, err := c.AdminStatusGet()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, as.State == api.AdminStateNormal,
		"expected as.State == api.AdminStateNormal, got:", as.State)

	as.State = api.AdminStateLocal
	err = c.AdminStatusSet(as)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	as, err = c.AdminStatusGet()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, as.State == api.AdminStateLocal,
		"expected as.State == api.AdminStateNormal, got:", as.State)

	as.State = api.AdminStateNormal
	err = c.AdminStatusSet(as)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	as, err = c.AdminStatusGet()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, as.State == api.AdminStateNormal,
		"expected as.State == api.AdminStateNormal, got:", as.State)
}

func TestVolumeSetBlockRestriction(t *testing.T) {
	db := tests.Tempfile()
	defer os.Remove(db)

	// Create the app
	app := glusterfs.NewTestApp(db)
	defer app.Close()

	// Setup the server
	ts := setupHeketiServer(app)
	defer ts.Close()

	// Create cluster
	c := NewClient(ts.URL, "admin", TEST_ADMIN_KEY)
	tests.Assert(t, c != nil)
	cluster_req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	cluster, err := c.ClusterCreate(cluster_req)
	tests.Assert(t, err == nil)

	// Create node request packet
	for n := 0; n < 4; n++ {
		nodeReq := &api.NodeAddRequest{}
		nodeReq.ClusterId = cluster.Id
		nodeReq.Hostnames.Manage = []string{"manage" + fmt.Sprintf("%v", n)}
		nodeReq.Hostnames.Storage = []string{"storage" + fmt.Sprintf("%v", n)}
		nodeReq.Zone = n + 1

		// Create node
		node, err := c.NodeAdd(nodeReq)
		tests.Assert(t, err == nil)

		deviceReq := &api.DeviceAddRequest{}
		deviceReq.Name = "/dev/by-magic/id:" + idgen.GenUUID()
		deviceReq.NodeId = node.Id

		// Create device
		err = c.DeviceAdd(deviceReq)
		tests.Assert(t, err == nil)
	}

	// Create a volume
	volumeReq := &api.VolumeCreateRequest{}
	volumeReq.Size = 10
	volumeReq.Block = true
	volume, err := c.VolumeCreate(volumeReq)
	tests.Assert(t, err == nil)
	tests.Assert(t, volume.Id != "")
	tests.Assert(t, volume.Size == volumeReq.Size)

	v2, err := c.VolumeSetBlockRestriction(
		volume.Id,
		&api.VolumeBlockRestrictionRequest{
			Restriction: api.Locked,
		})
	tests.Assert(t, err == nil)
	tests.Assert(t, v2.BlockInfo.Restriction == api.Locked)

	v2, err = c.VolumeSetBlockRestriction(
		volume.Id,
		&api.VolumeBlockRestrictionRequest{
			Restriction: api.Unrestricted,
		})
	tests.Assert(t, err == nil)
	tests.Assert(t, v2.BlockInfo.Restriction == api.Unrestricted)
}
