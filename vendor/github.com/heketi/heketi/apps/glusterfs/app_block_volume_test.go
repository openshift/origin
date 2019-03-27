//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

func TestBlockVolumeCreateBadJson(t *testing.T) {
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

	// VolumeCreate JSON Request
	request := []byte(`{
        asdfsdf
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/blockvolumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == 422)
}

func TestBlockVolumeCreateNoTopology(t *testing.T) {
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

	// VolumeCreate JSON Request
	request := []byte(`{
        "size" : 100
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/blockvolumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)
}
func TestBlockVolumeCreateInvalidSize(t *testing.T) {
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

	// VolumeCreate JSON Request
	request := []byte(`{
        "size" : 0
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/blockvolumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, r.ContentLength))
	tests.Assert(t, err == nil)
	r.Body.Close()
	tests.Assert(t, strings.Contains(string(body), "size: cannot be blank"), string(body))
}

func TestBlockVolumeCreateBadClusters(t *testing.T) {
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

	// Create a cluster
	// Setup database
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// VolumeCreate JSON Request
	request := []byte(`{
        "size" : 10,
        "clusters" : [
            "bad"
        ]
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/blockvolumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest)
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, r.ContentLength))
	tests.Assert(t, err == nil)
	r.Body.Close()
	tests.Assert(t, strings.Contains(string(body), "Cluster id bad not found"))
}

func TestBlockVolumeLargerThanBlockHostingVolume(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	//Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	//setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Setup database
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		1,    // devices_per_node,
		2*TB, // disksize)
	)
	tests.Assert(t, err == nil)
	var info api.BlockVolumeInfoResponse

	// Create a small blockvolume to auto create block hosting volume
	request := []byte(`{
        "size" : 1
    }`)
	r, err := http.Post(ts.URL+"/blockvolumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
		} else {
			tests.Assert(t, r.StatusCode == http.StatusOK, "got", r.StatusCode)
			err := utils.GetJsonFromResponse(r, &info)
			tests.Assert(t, err == nil)
			tests.Assert(t, info.Id != "")
			break
		}
	}

	// now create blockvolume request which can't fit in existing 500 GiB
	// blockhosting volume and any new blockhosting volume that can
	// be created
	request = []byte(`{
        "size" : 1079
    }`)
	r, err = http.Post(ts.URL+"/blockvolumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	// NOTE: as of the pending operations work this now fails faster with
	// an out of space error on the submission, not in the async reply
	tests.Assert(t, r.StatusCode == http.StatusInternalServerError)
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, r.ContentLength))
	tests.Assert(t, err == nil)
	r.Body.Close()
	tests.Assert(t, strings.Contains(string(body), "Failed to allocate new block volume: The size configured for automatic creation of block hosting volumes (1100) is too small to host the requested block volume of size 1079. The available size on this block hosting volume, minus overhead, is 1078. Please create a sufficiently large block hosting volume manually."), "got", string(body))

	//check are we able to create a block volume size except reserved 2%
	request = []byte(`{
        "size" : 1077
    }`)
	r, err = http.Post(ts.URL+"/blockvolumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err = r.Location()
	tests.Assert(t, err == nil)

	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") == "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond * 10)
		} else {
			tests.Assert(t, r.StatusCode == http.StatusOK, "got", r.StatusCode)
			err := utils.GetJsonFromResponse(r, &info)
			tests.Assert(t, err == nil)
			tests.Assert(t, info.Id != "")
			break
		}
	}
}

func TestBlockVolumeCreate(t *testing.T) {
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

	// Setup database
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// BlockVolumeCreate
	request := []byte(`{
        "size" : 100
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/blockvolumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	var info api.BlockVolumeInfoResponse
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		tests.Assert(t, r.StatusCode == http.StatusOK)
		if r.ContentLength <= 0 {
			time.Sleep(time.Millisecond * 10)
			continue
		} else {
			// Should have node information here
			tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")
			err = utils.GetJsonFromResponse(r, &info)
			tests.Assert(t, err == nil)
			break
		}
	}
	tests.Assert(t, info.Id != "")
	tests.Assert(t, info.Cluster != "")
	tests.Assert(t, info.BlockHostingVolume != "")
	tests.Assert(t, len(info.BlockVolume.Hosts) == 10)
	tests.Assert(t, info.BlockVolume.Iqn != "")
	tests.Assert(t, info.BlockVolume.Password == "")
	tests.Assert(t, info.BlockVolume.Username == "")
	tests.Assert(t, info.Size == 100)
	tests.Assert(t, info.Name == "blockvol_"+info.Id)
	tests.Assert(t, info.Auth == false)
}

func blockVolumeTestResult(t *testing.T, r *http.Response) api.BlockVolumeInfoResponse {
	var info api.BlockVolumeInfoResponse
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		tests.Assert(t, r.StatusCode == http.StatusOK)
		if r.ContentLength <= 0 {
			time.Sleep(time.Millisecond * 10)
			continue
		} else {
			// Should have node information here
			tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")
			err = utils.GetJsonFromResponse(r, &info)
			tests.Assert(t, err == nil)
			break
		}
	}
	return info
}

func TestBlockVolumeCreateHACount(t *testing.T) {
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

	// Setup database
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// BlockVolumeCreate
	request := []byte(`{
	"size" : 100,
	"hacount" : 3
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/blockvolumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	info := blockVolumeTestResult(t, r)

	tests.Assert(t, info.Id != "")
	tests.Assert(t, info.Cluster != "")
	tests.Assert(t, info.BlockHostingVolume != "")
	tests.Assert(t, len(info.BlockVolume.Hosts) == 3)
	tests.Assert(t, info.BlockVolume.Iqn != "")
	tests.Assert(t, info.BlockVolume.Password == "")
	tests.Assert(t, info.BlockVolume.Username == "")
	tests.Assert(t, info.Size == 100)
	tests.Assert(t, info.Name == "blockvol_"+info.Id)
	tests.Assert(t, info.Auth == false)
	tests.Assert(t, info.Hacount == 3)
}

func makeGlusterdCheck(available map[string]bool) func(string) error {
	return func(host string) error {
		if _, exists := available[host]; !exists {
			// every second host is unavailable
			available[host] = (len(available) % 2) == 0
		}
		if !available[host] {
			return fmt.Errorf("host %s unavailable", host)
		}
		return nil
	}
}

func countTrue(m map[string]bool) int {
	counter := 0
	for _, v := range m {
		if v {
			counter++
		}
	}
	return counter
}

func TestBlockVolumeCreateHACountHostUnavailableSuccess(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	hostAvailable := make(map[string]bool)
	app.xo.MockGlusterdCheck = makeGlusterdCheck(hostAvailable)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Setup database
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		5,    // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// BlockVolumeCreate
	request := []byte(`{
	"size" : 100,
	"hacount" : 3
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/blockvolumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)

	info := blockVolumeTestResult(t, r)
	tests.Assert(t, info.Id != "")
	tests.Assert(t, info.Cluster != "")
	tests.Assert(t, info.BlockHostingVolume != "")
	tests.Assert(t, len(info.BlockVolume.Hosts) == 3)
	tests.Assert(t, info.BlockVolume.Iqn != "")
	tests.Assert(t, info.BlockVolume.Password == "")
	tests.Assert(t, info.BlockVolume.Username == "")
	tests.Assert(t, info.Size == 100)
	tests.Assert(t, info.Name == "blockvol_"+info.Id)
	tests.Assert(t, info.Auth == false)
	tests.Assert(t, info.Hacount == 3)
	tests.Assert(t, len(hostAvailable) == 5)
	tests.Assert(t, countTrue(hostAvailable) == 3)
}

func TestBlockVolumeCreateHACountHostUnavailableFail(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	hostAvailable := make(map[string]bool)
	app.xo.MockGlusterdCheck = makeGlusterdCheck(hostAvailable)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Setup database
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		4,    // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// BlockVolumeCreate
	request := []byte(`{
	"size" : 100,
	"hacount" : 3
    }`)

	// Send request
	r, err := http.Post(ts.URL+"/blockvolumes", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Query queue until finished
	for {
		r, err = http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.StatusCode == http.StatusInternalServerError {
			break
		}
		tests.Assert(t, r.StatusCode == http.StatusOK)
		tests.Assert(t, r.ContentLength <= 0)
		time.Sleep(time.Millisecond * 10)
	}
	tests.Assert(t, len(hostAvailable) == 4)
	tests.Assert(t, countTrue(hostAvailable) == 2)
}

func TestBlockVolumeCreateHAShuffled(t *testing.T) {
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

	// Setup database
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// BlockVolumeCreate
	request := []byte(`{"size": 100, "hacount": 3}`)

	// reset a static seed to make test deterministic
	rand.Seed(42)
	var prevHosts []string
	changed := false
	for i := 0; i < 5; i++ {
		r, err := http.Post(ts.URL+"/blockvolumes", "application/json", bytes.NewBuffer(request))
		tests.Assert(t, err == nil)
		info := blockVolumeTestResult(t, r)

		// check that the hosts counts are as expected and match the ha counts
		// verify that we do not get the same hosts every time this function is
		// called.
		tests.Assert(t, len(info.BlockVolume.Hosts) == 3)
		tests.Assert(t, info.Hacount == 3)
		if prevHosts != nil && !reflect.DeepEqual(prevHosts, info.BlockVolume.Hosts) {
			changed = true
			break
		}
		prevHosts = info.BlockVolume.Hosts
	}
	tests.Assert(t, changed)
}

func TestBlockVolumeInfoIdNotFound(t *testing.T) {
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

	// Now that we have some data in the database, we can
	// make a request for the clutser list
	r, err := http.Get(ts.URL + "/blockvolumes/12345")
	tests.Assert(t, r.StatusCode == http.StatusNotFound)
	tests.Assert(t, err == nil)
}

func TestBlockVolumeInfo(t *testing.T) {
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

	// Setup database
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create a volume
	req := &api.BlockVolumeCreateRequest{}
	req.Size = 100
	req.Auth = true
	v := NewBlockVolumeEntryFromRequest(req)
	tests.Assert(t, v != nil)
	tests.Assert(t, v.Info.Auth == true)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Now that we have some data in the database, we can
	// make a request for the clutser list
	r, err := http.Get(ts.URL + "/blockvolumes/" + v.Info.Id)
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read response
	var msg api.BlockVolumeInfoResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)

	tests.Assert(t, msg.Id == v.Info.Id)
	tests.Assert(t, msg.Cluster == v.Info.Cluster)
	tests.Assert(t, msg.Name == v.Info.Name)
	tests.Assert(t, msg.Size == v.Info.Size)
	tests.Assert(t, msg.BlockHostingVolume != "")
	tests.Assert(t, len(msg.BlockVolume.Hosts) == 10)
	tests.Assert(t, msg.BlockVolume.Iqn != "")
	tests.Assert(t, msg.Name == "blockvol_"+v.Info.Id)
	// These tests are for auth enabled
	tests.Assert(t, msg.BlockVolume.Username != "")
	tests.Assert(t, msg.BlockVolume.Password != "")
}

func TestBlockVolumeListEmpty(t *testing.T) {
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

	// Get volumes, there should be none
	r, err := http.Get(ts.URL + "/blockvolumes")
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read response
	var msg api.BlockVolumeListResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)
	tests.Assert(t, len(msg.BlockVolumes) == 0)
}

func TestBlockVolumeList(t *testing.T) {
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

	// Create some volumes
	numvolumes := 1000
	err := app.db.Update(func(tx *bolt.Tx) error {

		for i := 0; i < numvolumes; i++ {
			v := createSampleBlockVolumeEntry(100)
			err := v.Save(tx)
			if err != nil {
				return err
			}
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Get volumes, there should be none
	r, err := http.Get(ts.URL + "/blockvolumes")
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read response
	var msg api.BlockVolumeListResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)
	tests.Assert(t, len(msg.BlockVolumes) == numvolumes)

	// Check that all the volumes are in the database
	err = app.db.View(func(tx *bolt.Tx) error {
		for _, id := range msg.BlockVolumes {
			_, err := NewBlockVolumeEntryFromId(tx, id)
			if err != nil {
				return err
			}
		}

		return nil
	})
	tests.Assert(t, err == nil)

}

func TestBlockVolumeListReadOnlyDb(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)

	// Create some volumes
	numvolumes := 1000
	err := app.db.Update(func(tx *bolt.Tx) error {

		for i := 0; i < numvolumes; i++ {
			v := createSampleBlockVolumeEntry(100)
			err := v.Save(tx)
			if err != nil {
				return err
			}
		}

		return nil

	})
	tests.Assert(t, err == nil)
	app.Close()

	// Open Db here to force read only mode
	db, err := bolt.Open(tmpfile, 0666, &bolt.Options{
		ReadOnly: true,
	})
	tests.Assert(t, err == nil, err)
	tests.Assert(t, db != nil)

	// Create the app
	app = NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Get volumes, there should be none
	r, err := http.Get(ts.URL + "/blockvolumes")
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read response
	var msg api.BlockVolumeListResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)
	tests.Assert(t, len(msg.BlockVolumes) == numvolumes)

	// Check that all the volumes are in the database
	err = app.db.View(func(tx *bolt.Tx) error {
		for _, id := range msg.BlockVolumes {
			_, err := NewBlockVolumeEntryFromId(tx, id)
			if err != nil {
				return err
			}
		}

		return nil
	})
	tests.Assert(t, err == nil)

}

func TestBlockVolumeDeleteIdNotFound(t *testing.T) {
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

	// Now that we have some data in the database, we can
	// make a request for the clutser list
	req, err := http.NewRequest("DELETE", ts.URL+"/blockvolumes/12345", nil)
	tests.Assert(t, err == nil)
	r, err := http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)
}

func TestBlockVolumeDelete(t *testing.T) {
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

	// Setup database
	err := setupSampleDbWithTopology(app,
		1,    // clusters
		10,   // nodes_per_cluster
		10,   // devices_per_node,
		5*TB, // disksize)
	)
	tests.Assert(t, err == nil)

	// Create a volume
	v := createSampleBlockVolumeEntry(100)
	tests.Assert(t, v != nil)
	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil)

	// Delete the volume
	req, err := http.NewRequest("DELETE", ts.URL+"/blockvolumes/"+v.Info.Id, nil)
	tests.Assert(t, err == nil)
	r, err := http.DefaultClient.Do(req)
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
			continue
		} else {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			tests.Assert(t, err == nil)
			break
		}
	}

	// Check it is not there
	r, err = http.Get(ts.URL + "/blockvolumes/" + v.Info.Id)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)
	tests.Assert(t, err == nil)
}
