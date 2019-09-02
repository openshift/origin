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
	"github.com/lpabon/godbc"
)

func NewTestApp(dbfile string) *App {

	// Create simple configuration for unit tests
	appConfig := &GlusterFSConfig{
		DBfile:                    dbfile,
		Executor:                  "mock",
		CreateBlockHostingVolumes: true,
		BlockHostingVolumeSize:    1100,
		MaxInflightOperations:     64, // avoid throttling test code
	}
	app, err := NewApp(appConfig)
	godbc.Check(err == nil)

	return app
}
