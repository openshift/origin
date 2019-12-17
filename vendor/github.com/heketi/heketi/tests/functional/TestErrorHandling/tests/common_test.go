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

package tests

import (
	"encoding/json"
	"io"
	"os"

	"github.com/heketi/heketi/pkg/logging"
	"github.com/heketi/heketi/pkg/testutils"
	"github.com/heketi/heketi/server/config"
)

var (
	logger = logging.NewLogger("[test]", logging.LEVEL_DEBUG)

	heketiUrl = "http://localhost:8080"

	testCluster = &testutils.ClusterEnv{
		HeketiUrl: heketiUrl,
		Nodes: []string{
			"192.168.10.100",
			"192.168.10.101",
			"192.168.10.102",
			"192.168.10.103",
		},
		SSHPort: "22",
		Disks: []string{
			"/dev/vdb",
			"/dev/vdc",
			"/dev/vdd",
			"/dev/vde",
			"/dev/vdf",
			"/dev/vdg",
			"/dev/vdh",
			"/dev/vdi",
		},
	}
	heketi = testCluster.HeketiClient()
)

// Using the original configuration file located at the path given
// in `orig` write a new config file to `dest` having transformed
// the configuration object sourced from orig via the update
// callback.
func UpdateConfig(orig, dest string, update func(*config.Config)) error {
	c, err := config.ReadConfig(orig)
	if err != nil {
		return err
	}
	update(c)
	return writeConfig(dest, c)
}

func writeConfig(dest string, c *config.Config) error {
	fp, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer fp.Close()
	err = json.NewEncoder(fp).Encode(c)
	if err != nil {
		return err
	}
	return nil
}

// CopyFile copies the existing file `orig` to the new path `dest`.
// No metadata is preserved.
func CopyFile(orig, dest string) error {
	origFile, err := os.Open(orig)
	if err != nil {
		return err
	}
	defer origFile.Close()
	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()
	_, err = io.Copy(destFile, origFile)
	return err
}
