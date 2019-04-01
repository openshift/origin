// +build dbexamples

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
	"os"
	"testing"

	//"github.com/boltdb/bolt"
	g "github.com/heketi/heketi/apps/glusterfs"
	//"github.com/heketi/heketi/pkg/glusterfs/api"
	//"github.com/heketi/tests"
)

func TestSimpleCluster(t *testing.T) {
	filename := "heketi.db.TestSimpleCluster"
	os.Remove(filename)

	g.BuildSimpleCluster(t, filename)
}

// TestLeakPendingVolumeCreate will start a volume create operation
// but never complete it "leaking" the pending operation entry
// so that we can test it can be dumped by other tooling.
func TestLeakPendingVolumeCreate(t *testing.T) {
	filename := "heketi.db.TestLeakPendingVolumeCreate"
	// remove any existing db files with the same name
	os.Remove(filename)

	g.LeakPendingVolumeCreate(t, filename)
}
