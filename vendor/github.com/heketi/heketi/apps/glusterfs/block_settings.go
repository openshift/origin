//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

var (
	// Default block settings
	CreateBlockHostingVolumes = false
	// Default 1 TB
	BlockHostingVolumeSize    = 1024
	BlockHostingVolumeOptions = "group gluster-block"
)
