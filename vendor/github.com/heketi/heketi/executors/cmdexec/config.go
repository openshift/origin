//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmdexec

type CmdConfig struct {
	Fstab                string `json:"fstab"`
	Sudo                 bool   `json:"sudo"`
	SnapShotLimit        int    `json:"snapshot_limit"`
	RebalanceOnExpansion bool   `json:"rebalance_on_expansion"`
	BackupLVM            bool   `json:"backup_lvm_metadata"`
	GlusterCliTimeout    uint32 `json:"gluster_cli_timeout"`
	PVDataAlignment      string `json:"pv_data_alignment"`
	VGPhysicalExtentSize string `json:"vg_physicalextentsize"`
	LVChunkSize          string `json:"lv_chunksize"`
	XfsSw                int    `json:"xfs_sw"`
	XfsSu                int    `json:"xfs_su"`
	DebugUmountFailures  bool   `json:"debug_umount_failures"`
	BlockVolumePrealloc  string `json:"block_prealloc"`
}
