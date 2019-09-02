// +build functional

//
// Copyright (c) 2019 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

package tests

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/heketi/tests"

	"github.com/heketi/heketi/pkg/testutils"
)

func TestServerStartUnknownDbAttrs(t *testing.T) {
	heketiServer := testutils.NewServerCtlFromEnv("..")
	origConf := path.Join(heketiServer.ServerDir, heketiServer.ConfPath)

	heketiServer.ConfPath = tests.Tempfile()
	defer os.Remove(heketiServer.ConfPath)
	CopyFile(origConf, heketiServer.ConfPath)

	defer func() {
		CopyFile(origConf, heketiServer.ConfPath)
		testutils.ServerRestarted(t, heketiServer)
		testCluster.Teardown(t)
		testutils.ServerStopped(t, heketiServer)
	}()

	testutils.ServerStarted(t, heketiServer)
	heketiServer.KeepDB = true

	// we only need setup to put something in the db
	testCluster.Setup(t, 3, 3)
	testutils.ServerStopped(t, heketiServer)

	// we need a clean copy of the db so we can actually clean up later
	tmpDb := tests.Tempfile()
	defer os.Remove(tmpDb)

	dbPath := path.Join(heketiServer.ServerDir, heketiServer.DbPath)
	err := CopyFile(dbPath, tmpDb)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	defer CopyFile(tmpDb, dbPath)

	dbJson := tests.Tempfile()
	defer os.Remove(dbJson)

	// export the db to json so we can hack it up
	err = heketiServer.RunOfflineCmd(
		[]string{"db", "export",
			"--dbfile", heketiServer.DbPath,
			"--jsonfile", dbJson})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// edit the json dump so that it contains a bogus db attribute
	var dump map[string]interface{}
	readJsonDump(t, dbJson, &dump)
	dbat, ok := dump["dbattributeentries"].(map[string]interface{})
	tests.Assert(t, ok, "conversion failed")
	type tempk struct {
		Key   string
		Value string
	}
	dbat["NOPE_NOPE_NOPE"] = tempk{"NOPE_NOPE_NOPE", "no"}
	writeJsonDump(t, dbJson, dump)

	// restore the "hacked" json to a heketi db (replacing old version)
	os.Remove(dbPath)
	err = heketiServer.RunOfflineCmd(
		[]string{"db", "import",
			"--dbfile", heketiServer.DbPath,
			"--jsonfile", dbJson})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// server should refuse to start due to "bad" attributes
	err = heketiServer.Start()
	tests.Assert(t, err != nil, "expected err != nil")
	tests.Assert(t, !heketiServer.IsAlive(), "expected heketi server stopped")
}

func readJsonDump(t *testing.T, path string, v interface{}) {
	fp, err := os.Open(path)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	defer fp.Close()

	jdec := json.NewDecoder(fp)
	err = jdec.Decode(v)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func writeJsonDump(t *testing.T, path string, v interface{}) {
	fp, err := os.Create(path)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	defer fp.Close()

	enc := json.NewEncoder(fp)
	enc.SetIndent("", "    ")
	err = enc.Encode(v)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}
