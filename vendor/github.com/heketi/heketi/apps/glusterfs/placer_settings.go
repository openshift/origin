//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

type ZoneCheckingStrategy string

const (
	ZONE_CHECKING_UNSET  ZoneCheckingStrategy = ""
	ZONE_CHECKING_NONE   ZoneCheckingStrategy = "none"
	ZONE_CHECKING_STRICT ZoneCheckingStrategy = "strict"
)

var (
	ZoneChecking = ZONE_CHECKING_NONE
)
