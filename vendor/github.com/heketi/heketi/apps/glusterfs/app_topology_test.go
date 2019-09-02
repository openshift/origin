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

	"github.com/heketi/tests"
)

func TestTopologyInfo(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)
	app := NewTestApp(tmpfile)
	defer app.Close()

	err := setupSampleDbWithTopology(app,
		1,    // clusters
		3,    // nodes_per_cluster
		3,    // devices_per_node,
		1*TB, // disksize
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// Create a volume of size 500
	v := createSampleReplicaVolumeEntry(500, 2)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// Create a block volume of size 10
	bv := createSampleBlockVolumeEntry(10)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = bv.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	topologyInfo, err := app.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testCluster := topologyInfo.ClusterList[0]

	tests.Assert(t, testCluster.Id != "", `expected testCluster.Id != "", got`, testCluster.Id)
	tests.Assert(t, len(testCluster.Volumes) > 0, `expected len(testCluster.Volumes) > 0 , got`, len(testCluster.Volumes))
	tests.Assert(t, len(testCluster.BlockVolumes) > 0, `expected len(testCluster.BlockVolumes) > 0 , got`, len(testCluster.BlockVolumes))
	tests.Assert(t, len(testCluster.Nodes) > 0, `expected len(testCluster.Nodes) > 0 , got`, len(testCluster.Nodes))
}
