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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

func TestClusterCreate(t *testing.T) {
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
    }`)

	// Post nothing
	r, err := http.Post(ts.URL+"/clusters", "application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusCreated)

	// Read JSON
	var msg api.ClusterInfoResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)

	// Test JSON
	tests.Assert(t, len(msg.Nodes) == 0)
	tests.Assert(t, len(msg.Volumes) == 0)

	// Check that the data on the database is recorded correctly
	var entry ClusterEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		return entry.Unmarshal(
			tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER)).
				Get([]byte(msg.Id)))
	})
	tests.Assert(t, err == nil)

	// Make sure they entries are euqal
	tests.Assert(t, entry.Info.Id == msg.Id)
	tests.Assert(t, len(entry.Info.Volumes) == 0)
	tests.Assert(t, len(entry.Info.Nodes) == 0)
}

func TestClusterSetFlags(t *testing.T) {
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

	clusterId := "123abc"

	// Store a cluster in the DB
	entry := NewClusterEntry()
	entry.Info.Id = clusterId
	entry.Info.File = true
	entry.Info.Block = true

	err := app.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
		if b == nil {
			return errors.New("Unable to open bucket")
		}

		buffer, err := entry.Marshal()
		if err != nil {
			return err
		}

		err = b.Put([]byte(entry.Info.Id), buffer)
		if err != nil {
			return err
		}

		return nil
	})
	tests.Assert(t, err == nil)

	// ClusterSetFlags JSON Request
	request := []byte(`{
"file": true,
"block": false
}`)

	// Send the ClusterSetFlags request
	r, err := http.Post(ts.URL+"/clusters/"+clusterId+"/flags",
		"application/json", bytes.NewBuffer(request))
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK,
		"Expected http status OK, got: ", http.StatusText(r.StatusCode))

	// Check that the data on the database is recorded correctly
	var ce ClusterEntry
	err = app.db.View(func(tx *bolt.Tx) error {
		return ce.Unmarshal(
			tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER)).
				Get([]byte(clusterId)))
	})
	tests.Assert(t, err == nil)

	tests.Assert(t, ce.Info.Id == clusterId)
	tests.Assert(t, ce.Info.File == true)
	tests.Assert(t, ce.Info.Block == false)
}

func TestClusterList(t *testing.T) {
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

	// Save some objects in the database
	numclusters := 5
	err := app.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
		if b == nil {
			return errors.New("Unable to open bucket")
		}

		for i := 0; i < numclusters; i++ {
			var entry ClusterEntry

			entry.Info.Id = fmt.Sprintf("%v", 5000+i)
			buffer, err := entry.Marshal()
			if err != nil {
				return err
			}

			err = b.Put([]byte(entry.Info.Id), buffer)
			if err != nil {
				return err
			}
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Now that we have some data in the database, we can
	// make a request for the clutser list
	r, err := http.Get(ts.URL + "/clusters")
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read response
	var msg api.ClusterListResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)

	// Thanks to BoltDB they come back in order
	mockid := 5000 // This is the mock id value we set above
	for _, id := range msg.Clusters {
		tests.Assert(t, id == fmt.Sprintf("%v", mockid))
		mockid++
	}
}

func TestClusterInfoIdNotFound(t *testing.T) {
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
	r, err := http.Get(ts.URL + "/clusters/12345")
	tests.Assert(t, r.StatusCode == http.StatusNotFound)
	tests.Assert(t, err == nil)
}

func TestClusterInfo(t *testing.T) {
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

	// Create a new ClusterInfo
	entry := NewClusterEntry()
	entry.Info.Id = "123"
	entry.Info.Block = true
	entry.Info.File = true
	for _, node := range []string{"a1", "a2", "a3"} {
		entry.NodeAdd(node)
	}
	for _, vol := range []string{"b1", "b2", "b3"} {
		entry.VolumeAdd(vol)
	}

	// Save the info in the database
	err := app.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
		if b == nil {
			return errors.New("Unable to open bucket")
		}

		buffer, err := entry.Marshal()
		if err != nil {
			return err
		}

		err = b.Put([]byte(entry.Info.Id), buffer)
		if err != nil {
			return err
		}

		return nil

	})
	tests.Assert(t, err == nil)

	// Now that we have some data in the database, we can
	// make a request for the clutser list
	r, err := http.Get(ts.URL + "/clusters/" + "123")
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.Header.Get("Content-Type") == "application/json; charset=UTF-8")

	// Read response
	var msg api.ClusterInfoResponse
	err = utils.GetJsonFromResponse(r, &msg)
	tests.Assert(t, err == nil)

	// Check values are equal
	tests.Assert(t, entry.Info.Id == msg.Id)
	tests.Assert(t, entry.Info.Block == msg.Block)
	tests.Assert(t, entry.Info.File == msg.File)
	tests.Assert(t, entry.Info.Volumes[0] == msg.Volumes[0])
	tests.Assert(t, entry.Info.Volumes[1] == msg.Volumes[1])
	tests.Assert(t, entry.Info.Volumes[2] == msg.Volumes[2])
	tests.Assert(t, entry.Info.Nodes[0] == msg.Nodes[0])
	tests.Assert(t, entry.Info.Nodes[1] == msg.Nodes[1])
	tests.Assert(t, entry.Info.Nodes[2] == msg.Nodes[2])
}

func TestClusterDeleteBadId(t *testing.T) {
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

	// Delete cluster with no elements
	req, err := http.NewRequest("DELETE", ts.URL+"/clusters/12345", nil)
	tests.Assert(t, err == nil)
	r, err := http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusNotFound)
}

func TestClusterDelete(t *testing.T) {
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

	clusters := [](*ClusterEntry){}
	nodes := [](*NodeEntry){}
	volumes := [](*VolumeEntry){}

	// Create a cluster with volumes and nodes
	cluster := NewClusterEntry()
	cluster.Info.Id = "a1"
	for _, node := range []string{"n11", "n12", "n13"} {
		cluster.NodeAdd(node)
		n := NewNodeEntry()
		n.Info.Id = node
		nodes = append(nodes, n)
	}
	for _, vol := range []string{"v1", "v2", "v3"} {
		cluster.VolumeAdd(vol)
		v := NewVolumeEntry()
		v.Info.Id = vol
		volumes = append(volumes, v)
	}
	clusters = append(clusters, cluster)

	// Create a cluster with only volumes
	cluster = NewClusterEntry()
	cluster.Info.Id = "a2"
	for _, vol := range []string{"b1", "b2", "b3"} {
		cluster.VolumeAdd(vol)
		v := NewVolumeEntry()
		v.Info.Id = vol
		volumes = append(volumes, v)
	}
	clusters = append(clusters, cluster)

	// Create a cluster with only nodes
	cluster = NewClusterEntry()
	cluster.Info.Id = "a3"
	for _, node := range []string{"n31", "n32", "n33"} {
		cluster.NodeAdd(node)
		n := NewNodeEntry()
		n.Info.Id = node
		nodes = append(nodes, n)
	}
	clusters = append(clusters, cluster)

	// Create an empty cluster
	cluster = NewClusterEntry()
	cluster.Info.Id = "000"
	clusters = append(clusters, cluster)

	// Save the info in the database
	err := app.db.Update(func(tx *bolt.Tx) error {
		for _, entry := range clusters {
			if err := EntrySave(tx, entry, entry.Info.Id); err != nil {
				return err
			}
		}
		for _, entry := range nodes {
			if err := EntrySave(tx, entry, entry.Info.Id); err != nil {
				return err
			}
		}
		for _, entry := range volumes {
			if err := EntrySave(tx, entry, entry.Info.Id); err != nil {
				return err
			}
		}
		return nil
	})
	tests.Assert(t, err == nil)

	// Check that we cannot delete a cluster with elements
	req, err := http.NewRequest("DELETE", ts.URL+"/clusters/"+"a1", nil)
	tests.Assert(t, err == nil)
	r, err := http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusConflict)
	tests.Assert(t, utils.GetErrorFromResponse(r).Error() == clusters[0].ConflictString())

	// Check that we cannot delete a cluster with volumes
	req, err = http.NewRequest("DELETE", ts.URL+"/clusters/"+"a2", nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusConflict)
	tests.Assert(t, utils.GetErrorFromResponse(r).Error() == clusters[1].ConflictString())

	// Check that we cannot delete a cluster with nodes
	req, err = http.NewRequest("DELETE", ts.URL+"/clusters/"+"a3", nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusConflict)
	tests.Assert(t, utils.GetErrorFromResponse(r).Error() == clusters[2].ConflictString())

	// Delete cluster with no elements
	req, err = http.NewRequest("DELETE", ts.URL+"/clusters/"+"000", nil)
	tests.Assert(t, err == nil)
	r, err = http.DefaultClient.Do(req)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)

	// Check database still has a1,a2, and a3, but not '000'
	err = app.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BOLTDB_BUCKET_CLUSTER))
		if b == nil {
			return errors.New("Unable to open bucket")
		}

		// Check that the ids are still in the database
		for _, id := range []string{"a1", "a2", "a3"} {
			buffer := b.Get([]byte(id))
			if buffer == nil {
				return errors.New(fmt.Sprintf("Id %v not found", id))
			}
		}

		// Check that the id 000 is no longer in the database
		buffer := b.Get([]byte("000"))
		if buffer != nil {
			return errors.New(fmt.Sprintf("Id 000 still in database and was deleted"))
		}

		return nil

	})
	tests.Assert(t, err == nil, err)

}
