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
		1,      // clusters
		2,      // nodes_per_cluster
		1,      // devices_per_node,
		500*GB, // disksize)
	)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// Create a volume which uses the entire storage
	v := createSampleReplicaVolumeEntry(495, 2)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = v.Create(app.db, app.executor)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	topologyInfo, err := app.TopologyInfo()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	testCluster := topologyInfo.ClusterList[0]

	tests.Assert(t, testCluster.Id != "", `expected testCluster.Id != "", got`, testCluster.Id)
	tests.Assert(t, len(testCluster.Volumes) > 0, `expected len(testCluster.Volumes) > 0 , got`, len(testCluster.Volumes))
	tests.Assert(t, len(testCluster.Nodes) > 0, `expected len(testCluster.Nodes) > 0 , got`, len(testCluster.Nodes))
}
