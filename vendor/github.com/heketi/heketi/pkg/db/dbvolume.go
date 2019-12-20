//
// Copyright (c) 2016 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package db

const (
	HeketiStorageVolumeName = "heketidbstorage"
)

var DbVolumeGlusterOptions = []string{
	"performance.stat-prefetch off",
	"performance.write-behind off",
	"performance.open-behind off",
	"performance.quick-read off",
	"performance.strict-o-direct on",
	"performance.read-ahead off",
	"performance.io-cache off",
	"performance.readdir-ahead off",
	"user.heketi.dbstoragelevel 1",
}
