//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/idgen"
	"github.com/heketi/heketi/pkg/sortedstrings"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

func TestDeviceAddBadRequests(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// ClusterCreate JSON Request
	request := []byte(`{
        bad json
    }`)

	// Post bad JSON
	r, err := http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == 422)

	// Make a request with no device
	request = []byte(`{
        "node" : "3071582c8575a06d824f6bfc125eb270"
    }`)

	// Post bad JSON
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)

	// Make a request with unknown node
	request = []byte(`{
        "node" : "3071582c8575a06d824f6bfc125eb270",
        "name" : "/dev/fake"
    }`)

	// Post unknown node
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound, r.StatusCode)

}

func TestDeviceAddDelete(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Add Cluster then a Node on the cluster
	// node
	cluster_req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	cluster := NewClusterEntryFromRequest(cluster_req)
	nodereq := &api.NodeAddRequest{
		ClusterId: cluster.Info.Id,
		Hostnames: api.HostAddresses{
			Manage:  []string{"manage"},
			Storage: []string{"storage"},
		},
		Zone: 99,
	}
	node := NewNodeEntryFromRequest(nodereq)
	cluster.NodeAdd(node.Info.Id)

	// Save information in the db
	err := app.db.Update(func(tx *bolt.Tx) error {
		err := cluster.Save(tx)
		if err != nil {
			return err
		}

		err = node.Save(tx)
		if err != nil {
			return err
		}
		return nil
	})
	tests.Assert(t, err == nil)

	// Create a request to a device
	request := []byte(`{
        "node" : "` + node.Info.Id + `",
        "name" : "/dev/fake1"
    }`)

	// Add device using POST
	r, err := http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}

	// Add the same device.  It should conflict
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusConflict)

	// Add a second device
	request = []byte(`{
        "node" : "` + node.Info.Id + `",
        "name" : "/dev/fake2"
    }`)

	// Add device using POST
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err = r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}

	// Check db to make sure devices where added
	devicemap := make(map[string]*DeviceEntry)
	err = app.db.View(func(tx *bolt.Tx) error {
		node, err = NewNodeEntryFromId(tx, node.Info.Id)
		if err != nil {
			return err
		}

		for _, id := range node.Devices {
			device, err := NewDeviceEntryFromId(tx, id)
			if err != nil {
				return err
			}
			devicemap[device.Info.Name] = device
		}

		return nil
	})
	tests.Assert(t, err == nil)

	val, ok := devicemap["/dev/fake1"]
	tests.Assert(t, ok)
	tests.Assert(t, val.Info.Name == "/dev/fake1")
	tests.Assert(t, len(val.Bricks) == 0)

	val, ok = devicemap["/dev/fake2"]
	tests.Assert(t, ok)
	tests.Assert(t, val.Info.Name == "/dev/fake2")
	tests.Assert(t, len(val.Bricks) == 0)

	// Add some bricks to check if delete conflicts works
	fakeid := devicemap["/dev/fake1"].Info.Id
	err = app.db.Update(func(tx *bolt.Tx) error {
		device, err := NewDeviceEntryFromId(tx, fakeid)
		if err != nil {
			return err
		}

		device.BrickAdd("123")
		device.BrickAdd("456")
		return device.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Now delete device and check for bad request
	req, err := http.NewRequest("DELETE", ts.URL+"/devices/"+fakeid, nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)

	err = app.db.Update(func(tx *bolt.Tx) error {
		device, err := NewDeviceEntryFromId(tx, fakeid)
		if err != nil {
			return err
		}
		device.State = api.EntryStateFailed
		return device.Save(tx)
	})
	tests.Assert(t, err == nil)
	// Now delete device and check for bad request
	req, err = http.NewRequest("DELETE", ts.URL+"/devices/"+fakeid, nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusConflict)
	tests.Assert(t, utils.GetErrorFromResponse(r).Error() == devicemap["/dev/fake1"].ConflictString())
	// Check the db is still intact
	err = app.db.View(func(tx *bolt.Tx) error {
		device, err := NewDeviceEntryFromId(tx, fakeid)
		if err != nil {
			return err
		}

		node, err = NewNodeEntryFromId(tx, device.NodeId)
		if err != nil {
			return err
		}

		return nil
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, sortedstrings.Has(node.Devices, fakeid))

	// Node delete bricks from the device
	err = app.db.Update(func(tx *bolt.Tx) error {
		device, err := NewDeviceEntryFromId(tx, fakeid)
		if err != nil {
			return err
		}

		device.BrickDelete("123")
		device.BrickDelete("456")
		return device.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Set offline
	request = []byte(`{
				"state" : "offline"
				}`)
	r, err = http.Post(ts.URL+"/devices/"+fakeid+"/state",
		"application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)

	location, err = r.Location()
	tests.Assert(t, err == nil)
	// Query queue until finished
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}
	// Get Device Info
	r, err = http.Get(ts.URL + "/devices/" + fakeid)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	var info api.DeviceInfoResponse
	err = utils.GetJsonFromResponse(r, &info)
	tests.Assert(t, info.Id == fakeid)
	tests.Assert(t, info.State == "offline")

	// Set failed
	request = []byte(`{
				"state" : "failed"
				}`)
	r, err = http.Post(ts.URL+"/devices/"+fakeid+"/state",
		"application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)

	location, err = r.Location()
	tests.Assert(t, err == nil)
	// Query queue until finished
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}
	// Get Device Info
	r, err = http.Get(ts.URL + "/devices/" + fakeid)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	err = utils.GetJsonFromResponse(r, &info)
	tests.Assert(t, info.Id == fakeid)
	tests.Assert(t, info.State == "failed")

	// Delete device
	req, err = http.NewRequest("DELETE", ts.URL+"/devices/"+fakeid, nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err = r.Location()
	tests.Assert(t, err == nil)

	// Wait for deletion
	for {
		r, err := http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
			continue
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}

	// Check db
	err = app.db.View(func(tx *bolt.Tx) error {
		_, err := NewDeviceEntryFromId(tx, fakeid)
		return err
	})
	tests.Assert(t, err == ErrNotFound)

	// Check node does not have the device
	err = app.db.View(func(tx *bolt.Tx) error {
		node, err = NewNodeEntryFromId(tx, node.Info.Id)
		return err
	})
	tests.Assert(t, err == nil)
	tests.Assert(t, !sortedstrings.Has(node.Devices, fakeid))

	// Check the registration of the device has been removed,
	// and the device can be added again
	request = []byte(`{
        "node" : "` + node.Info.Id + `",
        "name" : "/dev/fake1"
    }`)
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err = r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}
}

func TestDeviceAddCleansUp(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Add Cluster then a Node on the cluster
	// node
	cluster_req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	cluster := NewClusterEntryFromRequest(cluster_req)
	nodereq := &api.NodeAddRequest{
		ClusterId: cluster.Info.Id,
		Hostnames: api.HostAddresses{
			Manage:  []string{"manage"},
			Storage: []string{"storage"},
		},
		Zone: 99,
	}
	node := NewNodeEntryFromRequest(nodereq)
	cluster.NodeAdd(node.Info.Id)

	// Save information in the db
	err := app.db.Update(func(tx *bolt.Tx) error {
		err := cluster.Save(tx)
		if err != nil {
			return err
		}

		err = node.Save(tx)
		if err != nil {
			return err
		}
		return nil
	})
	tests.Assert(t, err == nil)

	// Mock the device setup to return an error, which will
	// cause the cleanup.
	deviceSetupFn := app.xo.MockDeviceSetup
	app.xo.MockDeviceSetup = func(host, device, vgid string, destroy bool) (*executors.DeviceInfo, error) {
		return nil, ErrDbAccess
	}

	// Create a request to a device
	request := []byte(`{
        "node" : "` + node.Info.Id + `",
        "name" : "/dev/fake1"
    }`)

	// Add device using POST
	r, err := http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
		} else {
			tests.Assert(t, r.StatusCode != http.StatusNoContent)
			break
		}
	}

	// Let's reset the mocked function
	app.xo.MockDeviceSetup = deviceSetupFn

	// Now it should work
	// Add device using POST
	r, err = http.Post(ts.URL+"/devices", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err = r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}
}

func TestDeviceInfoIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Get unknown device id
	r, err := http.Get(ts.URL + "/devices/123456789")
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)

}

func TestDeviceInfo(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create a device to save in the db
	device := NewDeviceEntry()
	device.Info.Id = "abc"
	device.Info.Name = "/dev/fake1"
	device.NodeId = "def"
	device.StorageSet(10000, 10000, 0)
	device.StorageAllocate(1000)

	// Save device in the db
	err := app.db.Update(func(tx *bolt.Tx) error {
		return device.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Get device information
	r, err := http.Get(ts.URL + "/devices/" + device.Info.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	var info api.DeviceInfoResponse
	err = utils.GetJsonFromResponse(r, &info)
	tests.Assert(t, info.Id == device.Info.Id)
	tests.Assert(t, info.Name == device.Info.Name)
	tests.Assert(t, info.State == "online")
	tests.Assert(t, info.Storage.Free == device.Info.Storage.Free)
	tests.Assert(t, info.Storage.Used == device.Info.Storage.Used)
	tests.Assert(t, info.Storage.Total == device.Info.Storage.Total)

}

func TestDeviceDeleteErrors(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create a device to save in the db
	device := NewDeviceEntry()
	device.Info.Id = "abc"
	device.Info.Name = "/dev/fake1"
	device.NodeId = "def"
	device.StorageSet(10000, 10000, 0)
	device.StorageAllocate(1000)

	// Save device in the db
	err := app.db.Update(func(tx *bolt.Tx) error {
		return device.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Delete unknown id
	req, err := http.NewRequest("DELETE", ts.URL+"/devices/123", nil)
	tests.Assert(t, err == nil)
	r, err := http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)

	// Delete device without a node there.. that's probably a really
	// bad situation
	req, err = http.NewRequest("DELETE", ts.URL+"/devices/"+device.Info.Id, nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusInternalServerError)
}

func TestDeviceState(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create a client
	c := client.NewClientNoAuth(ts.URL)
	tests.Assert(t, c != nil)

	// Create Cluster
	cluster_req := &api.ClusterCreateRequest{
		ClusterFlags: api.ClusterFlags{
			Block: true,
			File:  true,
		},
	}
	cluster, err := c.ClusterCreate(cluster_req)
	tests.Assert(t, err == nil)

	// Create Node
	nodeReq := &api.NodeAddRequest{
		Zone:      1,
		ClusterId: cluster.Id,
	}
	nodeReq.Hostnames.Manage = sort.StringSlice{"manage.host"}
	nodeReq.Hostnames.Storage = sort.StringSlice{"storage.host"}
	node, err := c.NodeAdd(nodeReq)
	tests.Assert(t, err == nil)

	// Add device
	deviceReq := &api.DeviceAddRequest{}
	deviceReq.Name = "/dev/fake1"
	deviceReq.NodeId = node.Id

	err = c.DeviceAdd(deviceReq)
	tests.Assert(t, err == nil)

	// Get node information again
	node, err = c.NodeInfo(node.Id)
	tests.Assert(t, err == nil)

	// Get device information
	deviceId := node.DevicesInfo[0].Id
	device, err := c.DeviceInfo(deviceId)
	tests.Assert(t, err == nil)

	// Get info
	deviceInfo, err := c.DeviceInfo(device.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, deviceInfo.State == "online")

	// Set offline
	request := []byte(`{
				"state" : "offline"
				}`)
	r, err := http.Post(ts.URL+"/devices/"+device.Id+"/state",
		"application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)

	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}

	// Get Device Info
	r, err = http.Get(ts.URL + "/devices/" + device.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	var info api.DeviceInfoResponse
	err = utils.GetJsonFromResponse(r, &info)
	tests.Assert(t, err == nil)
	tests.Assert(t, info.Id == device.Id)
	tests.Assert(t, info.Name == device.Name)
	tests.Assert(t, info.State == "offline")
	tests.Assert(t, info.Storage.Free == device.Storage.Free)
	tests.Assert(t, info.Storage.Used == device.Storage.Used)
	tests.Assert(t, info.Storage.Total == device.Storage.Total)

	// Set online again
	request = []byte(`{
				"state" : "online"
				}`)
	r, err = http.Post(ts.URL+"/devices/"+device.Id+"/state",
		"application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err = r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}

	// Get Device Info
	r, err = http.Get(ts.URL + "/devices/" + device.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	err = utils.GetJsonFromResponse(r, &info)
	tests.Assert(t, err == nil)
	tests.Assert(t, info.Id == device.Id)
	tests.Assert(t, info.Name == device.Name)
	tests.Assert(t, info.State == "online")
	tests.Assert(t, info.Storage.Free == device.Storage.Free)
	tests.Assert(t, info.Storage.Used == device.Storage.Used)
	tests.Assert(t, info.Storage.Total == device.Storage.Total)

	// Set to unknown state
	request = []byte(`{
				"state" : "blah"
			}`)
	r, err = http.Post(ts.URL+"/devices/"+device.Id+"/state",
		"application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest, r.StatusCode)

	// Make sure the state did not change
	r, err = http.Get(ts.URL + "/devices/" + device.Id)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	err = utils.GetJsonFromResponse(r, &info)
	tests.Assert(t, err == nil)
	tests.Assert(t, info.Id == device.Id)
	tests.Assert(t, info.Name == device.Name)
	tests.Assert(t, info.State == "online")
	tests.Assert(t, info.Storage.Free == device.Storage.Free)
	tests.Assert(t, info.Storage.Used == device.Storage.Used)
	tests.Assert(t, info.Storage.Total == device.Storage.Total)
}

func TestDeviceSync(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	nodeId := idgen.GenUUID()
	deviceId := idgen.GenUUID()

	var total uint64 = 600 * 1024 * 1024
	var used uint64 = 250 * 1024 * 1024
	var free uint64 = 350 * 1024 * 1024

	// Init test database
	err := app.db.Update(func(tx *bolt.Tx) error {
		cluster := NewClusterEntry()
		cluster.Info.Id = idgen.GenUUID()
		if err := cluster.Save(tx); err != nil {
			return err
		}

		device := NewDeviceEntry()
		device.Info.Id = deviceId
		device.Info.Name = "/dev/abc"
		device.NodeId = nodeId
		device.StorageSet(total, total, 0)
		device.StorageAllocate(100)

		if err := device.Save(tx); err != nil {
			return err
		}

		node := NewNodeEntry()
		node.Info.Id = nodeId
		node.Info.ClusterId = cluster.Info.Id
		node.Info.Hostnames.Manage = sort.StringSlice{"manage.system"}
		node.Info.Hostnames.Storage = sort.StringSlice{"storage.system"}
		node.Info.Zone = 10

		node.DeviceAdd(device.Info.Id)

		if err := node.Save(tx); err != nil {
			return err
		}

		return nil
	})
	tests.Assert(t, err == nil)

	app.xo.MockGetDeviceInfo = func(host, device, vgid string) (*executors.DeviceInfo, error) {
		d := &executors.DeviceInfo{}
		d.TotalSize = total
		d.FreeSize = free
		d.UsedSize = used
		d.ExtentSize = 4096
		return d, nil
	}
	r, err := http.Get(ts.URL + "/devices/" + deviceId + "/resync")
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)

	location, err := r.Location()
	tests.Assert(t, err == nil)

	for {
		r, err := http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
			continue
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			break
		}
	}

	err = app.db.View(func(tx *bolt.Tx) error {
		device, err := NewDeviceEntryFromId(tx, deviceId)
		tests.Assert(t, err == nil)
		tests.Assert(t, device.Info.Storage.Total == total, "expected:", total, "got:", device.Info.Storage.Total)
		tests.Assert(t, device.Info.Storage.Free == free)
		tests.Assert(t, device.Info.Storage.Used == used)
		return nil
	})
	tests.Assert(t, err == nil)

}

func TestDeviceSyncIdNotFound(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	deviceId := idgen.GenUUID()

	// Get unknown node id
	r, err := http.Get(ts.URL + "/devices/" + deviceId + "/resync")
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)
}

func TestDeviceSetTags(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	deviceId := idgen.GenUUID()
	// Create a device to save in the db
	device := NewDeviceEntry()
	device.Info.Id = deviceId
	device.Info.Name = "/dev/fake1"
	device.NodeId = "def"
	device.StorageSet(10000, 10000, 0)
	device.StorageAllocate(1000)
	err := app.db.Update(func(tx *bolt.Tx) error {
		return device.Save(tx)
	})
	tests.Assert(t, err == nil)

	// set some tags
	request := []byte(`{
		"change_type": "set",
		"tags": {"foo": "bar", "salad": "ceasar"}
	}`)
	r, err := http.Post(ts.URL+"/devices/"+deviceId+"/tags",
		"application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, r.StatusCode == http.StatusOK,
		"expected r.StatusCode == http.StatusOK, got:", r.StatusCode)

	r, err = http.Get(ts.URL + "/devices/" + deviceId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, r.StatusCode == http.StatusOK,
		"expected r.StatusCode == http.StatusOK, got:", r.StatusCode)

	var info api.DeviceInfoResponse
	err = utils.GetJsonFromResponse(r, &info)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(info.Tags) == 2,
		"expected len(info.Tags) == 2, got:", len(info.Tags))

	// add a new tag
	request = []byte(`{
		"change_type": "update",
		"tags": {"color": "blue"}
	}`)
	r, err = http.Post(ts.URL+"/devices/"+deviceId+"/tags",
		"application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, r.StatusCode == http.StatusOK,
		"expected r.StatusCode == http.StatusOK, got:", r.StatusCode)

	r, err = http.Get(ts.URL + "/devices/" + deviceId)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, r.StatusCode == http.StatusOK,
		"expected r.StatusCode == http.StatusOK, got:", r.StatusCode)

	err = utils.GetJsonFromResponse(r, &info)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(info.Tags) == 3,
		"expected len(info.Tags) == 3, got:", len(info.Tags))

	// submit garbage body
	request = []byte(`~~~~~`)
	r, err = http.Post(ts.URL+"/devices/"+deviceId+"/tags",
		"application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, r.StatusCode == http.StatusUnprocessableEntity,
		"expected r.StatusCode == http.StatusUnprocessableEntity, got:", r.StatusCode)

	// valid json, but nonsense
	request = []byte(`[{
		"flavor": "Purple",
		"doo_wop": 8888899
	}]`)
	r, err = http.Post(ts.URL+"/devices/"+deviceId+"/tags",
		"application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, r.StatusCode == http.StatusUnprocessableEntity,
		"expected r.StatusCode == http.StatusUnprocessableEntity, got:", r.StatusCode)

	// invalid params
	request = []byte(`{
		"change_type": "batman",
		"tags": {"": ""}
	}`)
	r, err = http.Post(ts.URL+"/devices/"+deviceId+"/tags",
		"application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest,
		"expected r.StatusCode == http.StatusBadRequest, got:", r.StatusCode)

	// invalid device id
	request = []byte(`{
		"change_type": "update",
		"tags": {"color": "blue"}
	}`)
	r, err = http.Post(ts.URL+"/devices/abc123/tags",
		"application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, r.StatusCode == http.StatusNotFound,
		"expected r.StatusCode == http.StatusNotFound, got:", r.StatusCode)
}
