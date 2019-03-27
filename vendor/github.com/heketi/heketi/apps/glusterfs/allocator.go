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
	wdb "github.com/heketi/heketi/pkg/db"
)

type Allocator interface {

	// Returns a generator, done, and error channel.
	// The generator returns the location for the brick, then the possible locations
	// of its replicas. The caller must close() the done channel when it no longer
	// needs to read from the generator.
	GetNodes(db wdb.RODB, clusterId, brickId string) (<-chan string,
		chan<- struct{}, error)
}
