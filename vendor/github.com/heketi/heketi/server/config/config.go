//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/middleware"
)

type Config struct {
	Port                 string                   `json:"port"`
	AuthEnabled          bool                     `json:"use_auth"`
	JwtConfig            middleware.JwtAuthConfig `json:"jwt"`
	BackupDbToKubeSecret bool                     `json:"backup_db_to_kube_secret"`
	EnableTls            bool                     `json:"enable_tls"`
	CertFile             string                   `json:"cert_file"`
	KeyFile              string                   `json:"key_file"`

	// pull in the config sub-object for glusterfs app
	GlusterFS *glusterfs.GlusterFSConfig `json:"glusterfs"`
}

func ParseConfig(input io.Reader) (config *Config, e error) {
	configParser := json.NewDecoder(input)
	if e = configParser.Decode(&config); e != nil {
		fmt.Fprintf(os.Stderr,
			"ERROR: Unable to parse configuration: %v\n",
			e.Error())
		return
	}
	return
}

func ReadConfig(configfile string) (config *Config, e error) {
	fp, e := os.Open(configfile)
	if e != nil {
		fmt.Fprintf(os.Stderr,
			"ERROR: Unable to open config file %v: %v\n",
			configfile,
			e.Error())
		return
	}
	defer fp.Close()
	return ParseConfig(fp)
}
