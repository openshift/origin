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
	"fmt"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/lpabon/godbc"
)

func (v *VolumeEntry) createVolume(db wdb.RODB,
	executor executors.Executor,
	brick_entries []*BrickEntry) error {

	godbc.Require(db != nil)
	godbc.Require(brick_entries != nil)

	// Create a volume request for executor with
	// the bricks allocated
	vr, host, err := v.createVolumeRequest(db, brick_entries)
	if err != nil {
		return err
	}

	// Create the volume
	if _, err := executor.VolumeCreate(host, vr); err != nil {
		return err
	}
	return nil
}

func (v *VolumeEntry) updateHostandMountPoint(hosts []string, deletedHost string) {

	v.Info.Mount.GlusterFS.Hosts = hosts
	// Save volume information
	if strings.Contains(v.Info.Mount.GlusterFS.MountPoint, deletedHost) {
		v.Info.Mount.GlusterFS.MountPoint = fmt.Sprintf("%v:%v",
			hosts[0], v.Info.Name)
	}
	v.Info.Mount.GlusterFS.Options["backup-volfile-servers"] =
		strings.Join(hosts[1:], ",")
}

func getHostsFromCluster(db wdb.RODB, clusterID string) ([]string, error) {
	hosts := []string{}
	if err := db.View(func(tx *bolt.Tx) error {
		cluster, err := NewClusterEntryFromId(tx, clusterID)
		if err != nil {
			return err
		}
		for _, nodeID := range cluster.Info.Nodes {
			node, err := NewNodeEntryFromId(tx, nodeID)
			if err != nil {
				return err
			}
			hosts = append(hosts, node.StorageHostName())
		}
		return err
	}); err != nil {
		return []string{}, err
	}

	return hosts, nil
}
func (v *VolumeEntry) updateMountInfo(db wdb.RODB) error {
	godbc.Require(v.Info.Cluster != "")

	// Get all brick hosts
	hosts, err := getHostsFromCluster(db, v.Info.Cluster)
	if err != nil {
		return err
	}
	v.Info.Mount.GlusterFS.Hosts = hosts

	// Save volume information
	v.Info.Mount.GlusterFS.MountPoint = fmt.Sprintf("%v:%v",
		hosts[0], v.Info.Name)

	// Set glusterfs mount volfile-servers options
	v.Info.Mount.GlusterFS.Options = make(map[string]string)
	v.Info.Mount.GlusterFS.Options["backup-volfile-servers"] =
		strings.Join(hosts[1:], ",")

	return nil
}

func (v *VolumeEntry) createVolumeRequest(db wdb.RODB,
	brick_entries []*BrickEntry) (*executors.VolumeRequest, string, error) {
	godbc.Require(db != nil)
	godbc.Require(brick_entries != nil)

	// Setup list of bricks
	vr := &executors.VolumeRequest{}
	vr.Bricks = make([]executors.BrickInfo, len(brick_entries))
	var sshhost string
	for i, b := range brick_entries {

		// Setup path
		vr.Bricks[i].Path = b.Info.Path

		// Get storage host name from Node entry
		err := db.View(func(tx *bolt.Tx) error {
			node, err := NewNodeEntryFromId(tx, b.Info.NodeId)
			if err != nil {
				return err
			}

			if sshhost == "" {
				sshhost = node.ManageHostName()
			}
			vr.Bricks[i].Host = node.StorageHostName()
			godbc.Check(vr.Bricks[i].Host != "")

			return nil
		})
		if err != nil {
			logger.Err(err)
			return nil, "", err
		}
	}

	// Setup volume information in the request
	vr.Name = v.Info.Name
	v.Durability.SetExecutorVolumeRequest(vr)
	vr.GlusterVolumeOptions = v.GlusterVolumeOptions
	vr.GlusterVolumeOptions = append(vr.GlusterVolumeOptions,
		fmt.Sprintf("%s %s", HEKETI_ID_KEY, v.Info.Id))
	vr.Arbiter = v.HasArbiterOption()

	return vr, sshhost, nil
}
