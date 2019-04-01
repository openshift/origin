//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"fmt"
	"math/rand"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/executors"
	wdb "github.com/heketi/heketi/pkg/db"
	"github.com/lpabon/godbc"
)

// Creates a block volume
func (v *BlockVolumeEntry) createBlockVolume(db wdb.RODB,
	executor executors.Executor, blockHostingVolumeId string) error {

	godbc.Require(db != nil)
	godbc.Require(blockHostingVolumeId != "")

	// Create the block volume create request for executor
	vr, host, err := v.createBlockVolumeRequest(db, executor,
		blockHostingVolumeId)
	if err != nil {
		return err
	}

	// Request the executor to create block volume based on the request created
	blockVolumeInfo, err := executor.BlockVolumeCreate(host, vr)
	if err != nil {
		return err
	}

	v.Info.BlockVolume.Iqn = blockVolumeInfo.Iqn
	v.Info.BlockVolume.Hosts = blockVolumeInfo.BlockHosts
	v.Info.BlockVolume.Lun = 0
	v.Info.BlockVolume.Username = blockVolumeInfo.Username
	v.Info.BlockVolume.Password = blockVolumeInfo.Password

	return nil
}

func (v *BlockVolumeEntry) createBlockVolumeRequest(db wdb.RODB,
	executor executors.Executor,
	blockHostingVolumeId string) (*executors.BlockVolumeRequest, string, error) {
	godbc.Require(db != nil)
	godbc.Require(blockHostingVolumeId != "")

	var blockHostingVolumeName string

	err := db.View(func(tx *bolt.Tx) error {
		logger.Debug("Getting info for block hosting volume %v", blockHostingVolumeId)
		bhvol, err := NewVolumeEntryFromId(tx, blockHostingVolumeId)
		if err != nil {
			return err
		}

		if v.Info.Hacount > 0 && v.Info.Hacount <= len(bhvol.Info.Mount.GlusterFS.Hosts) {
			v.Info.BlockVolume.Hosts = nil
			for _, i := range rand.Perm(len(bhvol.Info.Mount.GlusterFS.Hosts)) {
				managehostname, e := GetManageHostnameFromStorageHostname(tx, bhvol.Info.Mount.GlusterFS.Hosts[i])
				if e != nil {
					return fmt.Errorf("Could not find managehostname for %v", bhvol.Info.Mount.GlusterFS.Hosts[i])
				}
				e = executor.GlusterdCheck(managehostname)
				if e == nil {
					v.Info.BlockVolume.Hosts = append(v.Info.BlockVolume.Hosts, bhvol.Info.Mount.GlusterFS.Hosts[i])
					if len(v.Info.BlockVolume.Hosts) == v.Info.Hacount {
						break
					}
				}
			}
			if len(v.Info.BlockVolume.Hosts) < v.Info.Hacount {
				return fmt.Errorf("insufficient block hosts online")
			}
		} else {
			v.Info.BlockVolume.Hosts = bhvol.Info.Mount.GlusterFS.Hosts
			v.Info.Hacount = len(v.Info.BlockVolume.Hosts)
		}

		v.Info.Cluster = bhvol.Info.Cluster
		blockHostingVolumeName = bhvol.Info.Name

		return nil
	})
	if err != nil {
		logger.Err(err)
		return nil, "", err
	}

	// Select the host on which glusterd is running. To avoid request failing on host down senario.
	executorhost, err := GetVerifiedManageHostname(db, executor, v.Info.Cluster)
	if err != nil {
		return nil, "", err
	}

	logger.Debug("Using executor host [%v]", executorhost)

	// Setup volume information in the request
	vr := &executors.BlockVolumeRequest{}
	vr.Name = v.Info.Name
	vr.BlockHosts = v.Info.BlockVolume.Hosts
	vr.GlusterVolumeName = blockHostingVolumeName
	vr.Hacount = v.Info.Hacount
	vr.Size = v.Info.Size
	vr.Auth = v.Info.Auth

	return vr, executorhost, nil
}
