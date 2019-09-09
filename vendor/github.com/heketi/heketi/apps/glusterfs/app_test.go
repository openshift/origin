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
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/logging"
	"github.com/heketi/tests"
)

func TestAppAdvsettings(t *testing.T) {

	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)
	os.Setenv("HEKETI_EXECUTOR", "mock")
	defer os.Unsetenv("HEKETI_EXECUTOR")
	os.Setenv("HEKETI_DB_PATH", dbfile)
	defer os.Unsetenv("HEKETI_DB_PATH")

	conf := &GlusterFSConfig{
		Executor:     "crazyexec",
		Allocator:    "simple",
		DBfile:       "/path/to/nonexistent/heketi.db",
		BrickMaxSize: 1024,
		BrickMinSize: 4,
		BrickMaxNum:  33,
	}

	bmax, bmin, bnum := BrickMaxSize, BrickMinSize, BrickMaxNum
	defer func() {
		BrickMaxSize, BrickMinSize, BrickMaxNum = bmax, bmin, bnum
	}()

	app, _ := NewApp(conf)
	defer app.Close()
	tests.Assert(t, app != nil)
	tests.Assert(t, app.conf.Executor == "mock")
	tests.Assert(t, app.conf.DBfile == dbfile)
	tests.Assert(t, BrickMaxNum == 33)
	tests.Assert(t, BrickMaxSize == 1*TB)
	tests.Assert(t, BrickMinSize == 4*GB)
}

func TestAppLogLevel(t *testing.T) {
	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)

	levels := []string{
		"none",
		"critical",
		"error",
		"warning",
		"info",
		"debug",
	}

	logger.SetLevel(logging.LEVEL_DEBUG)
	for _, level := range levels {
		conf := &GlusterFSConfig{
			Executor:  "mock",
			Allocator: "simple",
			DBfile:    dbfile,
			Loglevel:  level,
		}

		app, _ := NewApp(conf)
		tests.Assert(t, app != nil, "expected app != nil, got:", app)

		switch level {
		case "none":
			tests.Assert(t, logger.Level() == logging.LEVEL_NOLOG)
		case "critical":
			tests.Assert(t, logger.Level() == logging.LEVEL_CRITICAL)
		case "error":
			tests.Assert(t, logger.Level() == logging.LEVEL_ERROR)
		case "warning":
			tests.Assert(t, logger.Level() == logging.LEVEL_WARNING)
		case "info":
			tests.Assert(t, logger.Level() == logging.LEVEL_INFO)
		case "debug":
			tests.Assert(t, logger.Level() == logging.LEVEL_DEBUG)
		}
		app.Close()
	}

	// Test that an unknown value does not change the loglevel
	logger.SetLevel(logging.LEVEL_NOLOG)
	conf := &GlusterFSConfig{
		Executor:  "mock",
		Allocator: "simple",
		DBfile:    dbfile,
		Loglevel:  "blah",
	}

	app, _ := NewApp(conf)
	defer app.Close()
	tests.Assert(t, app != nil)
	tests.Assert(t, logger.Level() == logging.LEVEL_NOLOG)
}

func TestAppReadOnlyDb(t *testing.T) {

	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)

	// First, create a db
	conf := &GlusterFSConfig{
		Executor: "mock",
		DBfile:   dbfile,
	}
	app, _ := NewApp(conf)
	tests.Assert(t, app != nil)
	tests.Assert(t, app.dbReadOnly == false)
	app.Close()

	// Now open it again here.  This will force NewApp()
	// to be unable to open RW.
	db, err := bolt.Open(dbfile, 0666, &bolt.Options{
		ReadOnly: true,
	})
	tests.Assert(t, err == nil, err)
	tests.Assert(t, db != nil)

	// Now open it again and notice how it opened
	app, _ = NewApp(conf)
	defer app.Close()
	tests.Assert(t, app != nil)
	tests.Assert(t, app.dbReadOnly == true)
}

func TestAppPathNotFound(t *testing.T) {
	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)

	app := NewTestApp(dbfile)
	tests.Assert(t, app != nil)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Setup a new client
	c := client.NewClientNoAuth(ts.URL)

	// Test paths which do not match the hexadecimal id
	_, err := c.ClusterInfo("xxx")
	tests.Assert(t, strings.Contains(err.Error(), "Invalid path or request"))

	_, err = c.NodeInfo("xxx")
	tests.Assert(t, strings.Contains(err.Error(), "Invalid path or request"))

	_, err = c.VolumeInfo("xxx")
	tests.Assert(t, strings.Contains(err.Error(), "Invalid path or request"))
}

func TestAppBlockSettings(t *testing.T) {

	dbfile := tests.Tempfile()
	defer os.Remove(dbfile)
	os.Setenv("HEKETI_EXECUTOR", "mock")
	defer os.Unsetenv("HEKETI_EXECUTOR")
	os.Setenv("HEKETI_DB_PATH", dbfile)
	defer os.Unsetenv("HEKETI_DB_PATH")

	conf := &GlusterFSConfig{
		Executor:  "crazyexec",
		Allocator: "simple",
		DBfile:    "/path/to/nonexistent/heketi.db",
		// this comment should make gofmt happy in versions >= 1.11 and <= 1.10
		CreateBlockHostingVolumes: true,
		BlockHostingVolumeSize:    500,
	}

	blockauto, blocksize := CreateBlockHostingVolumes, BlockHostingVolumeSize
	defer func() {
		CreateBlockHostingVolumes, BlockHostingVolumeSize = blockauto, blocksize
	}()

	app, _ := NewApp(conf)
	defer app.Close()
	tests.Assert(t, app != nil)
	tests.Assert(t, app.conf.Executor == "mock")
	tests.Assert(t, app.conf.DBfile == dbfile)
	tests.Assert(t, CreateBlockHostingVolumes == true)
	tests.Assert(t, BlockHostingVolumeSize == 500)
}
