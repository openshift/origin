//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/logging"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

func TestGetLogLevelNoLog(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// remember orig logging state
	orig := logger.Level()
	defer logger.SetLevel(orig)
	// our test starts with logging at NOLOG
	logger.SetLevel(logging.LEVEL_NOLOG)
	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	r, err := http.Get(ts.URL + "/internal/logging")
	tests.Assert(t, err == nil, "http.Get failed:", err)
	tests.Assert(t, r.StatusCode == http.StatusOK,
		"expected r.StatusCode == http.StatusOK, got", r.StatusCode)

	data := api.LogLevelInfo{}
	err = utils.GetJsonFromResponse(r, &data)
	tests.Assert(t, err == nil, "GetJsonFromResponse failed:", err)
	tests.Assert(t, data.LogLevel["glusterfs"] == "none",
		`expected data.LogLevel == "none", got:`, data.LogLevel)
}

func TestGetLogLevelDebug(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// remember orig logging state
	orig := logger.Level()
	defer logger.SetLevel(orig)
	// our test starts with logging at NOLOG
	logger.SetLevel(logging.LEVEL_DEBUG)
	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	r, err := http.Get(ts.URL + "/internal/logging")
	tests.Assert(t, err == nil, "http.Get failed:", err)
	tests.Assert(t, r.StatusCode == http.StatusOK,
		"expected r.StatusCode == http.StatusOK, got", r.StatusCode)

	var data api.LogLevelInfo
	err = utils.GetJsonFromResponse(r, &data)
	tests.Assert(t, err == nil, "GetJsonFromResponse failed:", err)
	tests.Assert(t, data.LogLevel["glusterfs"] == "debug",
		`expected data.LogLevel == "debug", got:`, data.LogLevel)
}

func TestSetLogLevelDebug(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// remember orig logging state
	orig := logger.Level()
	defer logger.SetLevel(orig)
	// our test starts with logging at NOLOG
	logger.SetLevel(logging.LEVEL_NOLOG)
	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// having done nothing yet our log level should still be NOLOG
	tests.Assert(t, logger.Level() == logging.LEVEL_NOLOG,
		`expected logger.Level() == logging.LEVEL_NOLOG, got:`, logger.Level())

	request := []byte(`{"loglevel":{"glusterfs": "debug"} }`)
	r, err := http.Post(ts.URL+"/internal/logging",
		"application/json",
		bytes.NewBuffer(request))
	tests.Assert(t, err == nil, "http.Post failed:", err)
	tests.Assert(t, r.StatusCode == http.StatusOK,
		"expected r.StatusCode == http.StatusOK, got", r.StatusCode)

	var data api.LogLevelInfo
	err = utils.GetJsonFromResponse(r, &data)
	tests.Assert(t, err == nil, "GetJsonFromResponse failed:", err)
	tests.Assert(t, data.LogLevel["glusterfs"] == "debug",
		`expected data.LogLevel == "debug", got:`, data.LogLevel)

	// check the actual log level now
	tests.Assert(t, logger.Level() == logging.LEVEL_DEBUG,
		`expected logger.Level() == logging.LEVEL_DEBUG, got:`, logger.Level())
}

func TestSetLogLevelRoundtrips(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// remember orig logging state
	orig := logger.Level()
	defer logger.SetLevel(orig)
	// our test starts with logging at NOLOG
	logger.SetLevel(logging.LEVEL_NOLOG)
	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// having done nothing yet our log level should still be NOLOG
	tests.Assert(t, logger.Level() == logging.LEVEL_NOLOG,
		`expected logger.Level() == logging.LEVEL_NOLOG, got:`, logger.Level())
	data := api.LogLevelInfo{LogLevel: map[string]string{}}

	names := []string{"none", "critical", "error", "warning", "info"}

	for _, name := range names {
		data.LogLevel["glusterfs"] = name
		b := &bytes.Buffer{}
		json.NewEncoder(b).Encode(data)

		r, err := http.Post(ts.URL+"/internal/logging", "application/json", b)
		tests.Assert(t, err == nil, "http.Post failed:", err)
		tests.Assert(t, r.StatusCode == http.StatusOK,
			"expected r.StatusCode == http.StatusOK, got", r.StatusCode)

		err = utils.GetJsonFromResponse(r, &data)
		tests.Assert(t, err == nil, "GetJsonFromResponse failed:", err)
		tests.Assert(t, data.LogLevel["glusterfs"] == name,
			`expected data.LogLevel == name, got:`, data.LogLevel, name)
	}
}

func TestSetLogLevelBadJson(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// remember orig logging state
	orig := logger.Level()
	defer logger.SetLevel(orig)
	// our test starts with logging at NOLOG
	logger.SetLevel(logging.LEVEL_NOLOG)
	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// having done nothing yet our log level should still be NOLOG
	tests.Assert(t, logger.Level() == logging.LEVEL_NOLOG,
		`expected logger.Level() == logging.LEVEL_NOLOG, got:`, logger.Level())

	request := []byte(`{"loglevel": debug}`)
	r, err := http.Post(ts.URL+"/internal/logging",
		"application/json",
		bytes.NewBuffer(request))
	tests.Assert(t, err == nil, "http.Post failed:", err)
	tests.Assert(t, r.StatusCode == http.StatusBadRequest,
		"expected r.StatusCode == http.StatusBadRequest, got", r.StatusCode)

	var data api.LogLevelInfo
	err = utils.GetJsonFromResponse(r, &data)
	// check that an error message has been set
	tests.Assert(t, err != nil, "GetJsonFromResponse unset:", err)
}

func TestSetLogLevelBadLogLevel(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// remember orig logging state
	orig := logger.Level()
	defer logger.SetLevel(orig)
	// our test starts with logging at NOLOG
	logger.SetLevel(logging.LEVEL_NOLOG)
	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()
	router := mux.NewRouter()
	app.SetRoutes(router)

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// having done nothing yet our log level should still be NOLOG
	tests.Assert(t, logger.Level() == logging.LEVEL_NOLOG,
		`expected logger.Level() == logging.LEVEL_NOLOG, got:`, logger.Level())

	request := []byte(`{"loglevel":{"glusterfs": "verdant"}}`)
	r, err := http.Post(ts.URL+"/internal/logging",
		"application/json",
		bytes.NewBuffer(request))
	tests.Assert(t, err == nil, "http.Post failed:", err)
	tests.Assert(t, r.StatusCode == http.StatusUnprocessableEntity,
		"expected r.StatusCode == http.StatusUnprocessableEntity, got", r.StatusCode)

	var data api.LogLevelInfo
	err = utils.GetJsonFromResponse(r, &data)
	// check that an error message has been set
	tests.Assert(t, err != nil, "GetJsonFromResponse unset:", err)
}

func TestLogLevelNameUnexpected(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// remember orig logging state
	orig := logger.Level()
	defer logger.SetLevel(orig)
	// our test starts with logging at NOLOG
	logger.SetLevel(800)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	v := app.logLevelName()
	tests.Assert(t, v == "(unknown)",
		`expected v == "(unknown)", got`, v)
}
