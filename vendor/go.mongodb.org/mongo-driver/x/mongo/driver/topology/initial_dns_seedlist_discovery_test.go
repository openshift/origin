// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package topology

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	testhelpers "go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
)

const seedlistTestDir string = "../../../../data/initial-dns-seedlist-discovery/"

type seedlistTestCase struct {
	URI     string
	Seeds   []string
	Hosts   []string
	Error   bool
	Options map[string]interface{}
}

// Because the Go driver tests can be run either against a server with SSL enabled or without, a
// number of configurations have to be checked to ensure that the SRV tests are run properly.
//
// First, the "ssl" option in the JSON test description has to be checked. If this option is not
// present, we assume that the test will assert an error, so we proceed with the test as normal.
// If the option is false, then we skip the test if the server is running with SSL enabled.
// If the option is true, then we skip the test if the server is running without SSL enabled; if
// the server is running with SSL enabled, then we manually set the necessary SSL options in the
// connection string.
func setSSLSettings(t *testing.T, cs *connstring.ConnString, options map[string]interface{}) {
	var testCaseExpectsSSL bool
	if ssl, found := options["ssl"]; found && ssl.(bool) {
		// The options specify "ssl: true".
		testCaseExpectsSSL = true
	} else if !found {
		// No "ssl" option is specified.
		return
	}

	envSSL := os.Getenv("SSL") == "ssl"

	// Skip non-SSL tests if the server is running with SSL.
	if !testCaseExpectsSSL && envSSL {
		t.Skip()
	}

	// Skip SSL tests if the server is running without SSL.
	if testCaseExpectsSSL && !envSSL {
		t.Skip()
	}

	// If SSL tests are running, set the CA file.
	if testCaseExpectsSSL && envSSL {
		cs.SSLInsecure = true
	}
}

func runSeedlistTest(t *testing.T, filename string, test *seedlistTestCase) {
	t.Run(filename, func(t *testing.T) {
		if runtime.GOOS == "windows" && filename == "two-txt-records" {
			t.Skip("Skipping to avoid windows multiple TXT record lookup bug")
		}
		if strings.HasPrefix(runtime.Version(), "go1.11") && (filename == "one-txt-record-multiple-strings") {
			t.Skip("Skipping to avoid Go 1.11 problem with multiple strings in one TXT record")
		}

		cs, err := connstring.Parse(test.URI)
		if test.Error {
			require.Error(t, err)
			return
		}
		// The resolved connstring may not have valid credentials
		if err != nil && err.Error() == "error parsing uri: authsource without username is invalid" {
			err = nil
		}
		require.NoError(t, err)
		require.Equal(t, cs.Scheme, "mongodb+srv")
		require.Equal(t, cs.Scheme, connstring.SchemeMongoDBSRV)

		// DNS records may be out of order from the test files ordering
		seeds := buildSet(test.Seeds)
		hosts := buildSet(cs.Hosts)

		require.Equal(t, hosts, seeds)
		testhelpers.VerifyConnStringOptions(t, cs, test.Options)
		setSSLSettings(t, &cs, test.Options)

		// make a topology from the options
		c, err := New(WithConnString(func(connstring.ConnString) connstring.ConnString { return cs }))
		require.NoError(t, err)
		err = c.Connect()
		require.NoError(t, err)

		for _, host := range test.Hosts {
			_, err := getServerByAddress(host, c)
			require.NoError(t, err)
		}
	})

}

// Test case for all connection string spec tests.
func TestInitialDNSSeedlistDiscoverySpec(t *testing.T) {
	if os.Getenv("TOPOLOGY") != "replica_set" || os.Getenv("AUTH") != "noauth" {
		t.Skip("Skipping on non-replica set topology")
	}

	for _, fname := range testhelpers.FindJSONFilesInDir(t, seedlistTestDir) {
		filepath := path.Join(seedlistTestDir, fname)
		content, err := ioutil.ReadFile(filepath)
		require.NoError(t, err)

		var testCase seedlistTestCase
		require.NoError(t, json.Unmarshal(content, &testCase))

		fname = fname[:len(fname)-5]
		runSeedlistTest(t, fname, &testCase)
	}
}

func buildSet(list []string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, s := range list {
		set[s] = struct{}{}
	}
	return set
}

func getServerByAddress(address string, c *Topology) (description.Server, error) {
	selectByName := description.ServerSelectorFunc(func(_ description.Topology, servers []description.Server) ([]description.Server, error) {
		for _, s := range servers {
			if s.Addr.String() == address {
				return []description.Server{s}, nil
			}
		}
		return []description.Server{}, nil
	})

	selectedServer, err := c.SelectServerLegacy(context.Background(), selectByName)
	if err != nil {
		return description.Server{}, err
	}
	return selectedServer.Server.Description(), nil
}
