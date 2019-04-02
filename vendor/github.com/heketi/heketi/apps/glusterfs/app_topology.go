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
	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/pkg/glusterfs/api"
)

func (a *App) TopologyInfo() (*api.TopologyInfoResponse, error) {
	topo := &api.TopologyInfoResponse{
		ClusterList: make([]api.Cluster, 0),
	}

	err := a.db.View(func(tx *bolt.Tx) error {
		clusters, err := ClusterList(tx)
		if err != nil {
			return err
		}

		for _, cluster := range clusters {
			clusterInfo, err := clusterInfo(tx, cluster)
			if err != nil {
				return err
			}
			cluster := api.Cluster{
				Id:      clusterInfo.Id,
				Volumes: make([]api.VolumeInfoResponse, 0),
				Nodes:   make([]api.NodeInfoResponse, 0),
				ClusterFlags: api.ClusterFlags{
					Block: clusterInfo.Block,
					File:  clusterInfo.File,
				},
			}
			cluster.Id = clusterInfo.Id

			for _, volume := range clusterInfo.Volumes {
				volumeInfo, err := volumeInfo(tx, volume)
				if err != nil {
					return err
				}
				cluster.Volumes = append(cluster.Volumes, *volumeInfo)
			}

			for _, node := range clusterInfo.Nodes {
				nodei, err := nodeInfo(tx, string(node))
				if err != nil {
					return err
				}
				cluster.Nodes = append(cluster.Nodes, *nodei)
			}
			topo.ClusterList = append(topo.ClusterList, cluster)
		}
		return nil
	})

	return topo, err
}

func clusterInfo(tx *bolt.Tx, id string) (*api.ClusterInfoResponse, error) {
	var info *api.ClusterInfoResponse
	entry, err := NewClusterEntryFromId(tx, id)
	if err != nil {
		return info, err
	}

	info, err = entry.NewClusterInfoResponse(tx)
	if err != nil {
		return info, err
	}

	err = UpdateClusterInfoComplete(tx, info)

	return info, err
}

func volumeInfo(tx *bolt.Tx, id string) (*api.VolumeInfoResponse, error) {
	var info *api.VolumeInfoResponse

	entry, err := NewVolumeEntryFromId(tx, id)
	if err != nil {
		return info, err
	}

	info, err = entry.NewInfoResponse(tx)

	return info, err
}

func nodeInfo(tx *bolt.Tx, id string) (*api.NodeInfoResponse, error) {
	var info *api.NodeInfoResponse

	entry, err := NewNodeEntryFromId(tx, id)
	if err != nil {
		return info, err
	}

	info, err = entry.NewInfoReponse(tx)

	return info, err
}
