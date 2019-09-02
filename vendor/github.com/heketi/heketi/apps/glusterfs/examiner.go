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
	"fmt"
	"reflect"
	"sort"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
)

type ExaminerMode string

const (
	OnDemandExaminer ExaminerMode = "ondemand"
	OfflineExaminer  ExaminerMode = "offline"
)

type Examiner struct {
	db        *bolt.DB
	executor  executors.Executor
	optracker *OpTracker
	mode      ExaminerMode
}

type NodeData struct {
	NodeHeketiID      string
	VolumeInfo        *executors.VolInfo
	LVMPVInfo         *executors.PVSCommandOutput
	LVMVGInfo         *executors.VGSCommandOutput
	LVMLVInfo         *executors.LVSCommandOutput
	BricksMountStatus *executors.BricksMountStatus
	BlockVolumeNames  []string
}

type ClusterData struct {
	ClusterHeketiID string
	NodesData       []NodeData
}

type GlusterStateExaminationResponse struct {
	HeketiDB Db            `json:"heketidb"`
	Report   []string      `json:"report"`
	Clusters []ClusterData `json:"clusters"`
}

func (examiner Examiner) fetchClusterData(cluster ClusterEntry, heketidb Db) (clusterdata ClusterData, errorstrings []string) {

	clusterdata.ClusterHeketiID = cluster.Info.Id

	for _, node := range cluster.Info.Nodes {
		nodeEntry := heketidb.Nodes[node]
		var nodedata NodeData
		nodedata.NodeHeketiID = nodeEntry.Info.Id
		host := nodeEntry.ManageHostName()

		// Volume Info
		volinfo, err := examiner.executor.VolumesInfo(host)
		if err != nil {
			errorstrings = append(errorstrings, fmt.Sprintf("could not fetch data from node %v", host))
		} else {
			nodedata.VolumeInfo = volinfo
		}

		// Device Info
		nodedata.LVMPVInfo, err = examiner.executor.PVS(host)
		if err != nil {
			errorstrings = append(errorstrings, fmt.Sprintf("could not fetch LVM pvs data from node %v : %v", host, err))
		}
		nodedata.LVMVGInfo, err = examiner.executor.VGS(host)
		if err != nil {
			errorstrings = append(errorstrings, fmt.Sprintf("could not fetch LVM vgs data from node %v : %v", host, err))
		}
		nodedata.LVMLVInfo, err = examiner.executor.LVS(host)
		if err != nil {
			errorstrings = append(errorstrings, fmt.Sprintf("could not fetch LVM lvs data from node %v : %v", host, err))
		}

		// Brick Info
		nodedata.BricksMountStatus, err = examiner.executor.GetBrickMountStatus(host)
		if err != nil {
			errorstrings = append(errorstrings, fmt.Sprintf("could not fetch brick mount status from node %v : %v", host, err))
		}

		// Block Volume Info
		for _, blockHostingVolume := range heketidb.Volumes {
			if blockHostingVolume.Info.Block {
				names, err := examiner.executor.ListBlockVolumes(host, blockHostingVolume.Info.Name)
				if err != nil {
					errorstrings = append(errorstrings, fmt.Sprintf("could not fetch block volume list for block hosting volume %v : %v", blockHostingVolume.Info.Id, err))
				}
				for _, name := range names {
					nodedata.BlockVolumeNames = append(nodedata.BlockVolumeNames, fmt.Sprintf("%v/%v", blockHostingVolume.Info.Name, name))
				}
			}
		}

		// Append whatever information we could gather, minimum is the node ID
		clusterdata.NodesData = append(clusterdata.NodesData, nodedata)
	}

	return

}

func matchVolumes(heketidb Db, cdata ClusterData) (errorstrings []string) {

	var heketiVolList []string

	for _, volume := range heketidb.Clusters[cdata.ClusterHeketiID].Info.Volumes {
		heketiVolList = append(heketiVolList, heketidb.Volumes[volume].Info.Name)
	}
	sort.Strings(heketiVolList)

	for _, node := range cdata.NodesData {
		var volList []string
		if node.VolumeInfo == nil {
			errorstrings = append(errorstrings, fmt.Sprintf("heketi volume list could not be compared with volume list of node %v due to missing info", node.NodeHeketiID))
			continue
		}
		for _, volume := range node.VolumeInfo.Volumes.VolumeList {
			volList = append(volList, volume.VolumeName)
		}
		sort.Strings(volList)
		if !reflect.DeepEqual(heketiVolList, volList) {
			errorstrings = append(errorstrings, fmt.Sprintf("heketi volume list does not match with volume list of node %v", node.NodeHeketiID))
		}
	}

	return
}

func compareHeketiAndGluster(heketidb Db, cdata ClusterData) (errorstrings []string) {

	matchErrors := matchVolumes(heketidb, cdata)
	if matchErrors != nil {
		errorstrings = append(errorstrings, matchErrors...)
	} else {
		errorstrings = append(errorstrings, fmt.Sprintf("heketi volume list matches with volume list of all nodes"))
	}

	return
}

// ExamineGluster ... fetches information about resources heketi is managing
// from the database and then queries information for each of those resources.
// It matches the information from heketi and Gluster resources and reports any
// errors that are found.
func (examiner Examiner) ExamineGluster() (response GlusterStateExaminationResponse, err error) {
	logger.Debug("Examining Gluster")

	if examiner.mode == OnDemandExaminer {
		trackedOps := examiner.optracker.Get()
		response.Report = append(response.Report, fmt.Sprintf("OnDemand Examiner invoked while %v ops are in flight", trackedOps))
	}

	// Fetch information from Heketi DB
	response.HeketiDB, err = dbDumpInternal(examiner.db)
	if err != nil {
		response.Report = append(response.Report, fmt.Sprintf("not able to fetch dump of database"))
		return
	}

	// Fetch information from Gluster
	for _, clusterEntry := range response.HeketiDB.Clusters {
		var clusterdata ClusterData
		clusterdata.ClusterHeketiID = clusterEntry.Info.Id

		clusterdata, fetcherrors := examiner.fetchClusterData(clusterEntry, response.HeketiDB)
		if len(fetcherrors) > 0 {
			response.Report = append(response.Report, fetcherrors...)
		}
		if len(clusterdata.NodesData) == 0 {
			response.Report = append(response.Report, fmt.Sprintf("not able to fetch any data from cluster %v", clusterEntry.Info.Id))
		}
		response.Clusters = append(response.Clusters, clusterdata)

		// Compare data
		compareErrors := compareHeketiAndGluster(response.HeketiDB, clusterdata)
		if len(compareErrors) > 0 {
			response.Report = append(response.Report, compareErrors...)
		}
	}

	return
}
