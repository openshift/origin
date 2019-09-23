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
	"strings"

	"github.com/boltdb/bolt"

	wdb "github.com/heketi/heketi/pkg/db"
)

type DeviceZoneMap struct {
	AvailableZones map[int]bool
	DeviceZones    map[string]int
}

func NewDeviceZoneMap() *DeviceZoneMap {
	return &DeviceZoneMap{
		AvailableZones: map[int]bool{},
		DeviceZones:    map[string]int{},
	}
}

func NewDeviceZoneMapFromDb(db wdb.RODB) (*DeviceZoneMap, error) {
	dzm := NewDeviceZoneMap()
	err := db.View(func(tx *bolt.Tx) error {
		dl, err := DeviceList(tx)
		if err != nil {
			return err
		}
		for _, deviceId := range dl {
			if strings.HasPrefix(deviceId, "DEVICE") {
				logger.Debug("ignoring registry key %v", deviceId)
				continue
			}
			device, err := NewDeviceEntryFromId(tx, deviceId)
			if err != nil {
				return err
			}
			n, err := NewNodeEntryFromId(tx, device.NodeId)
			if err != nil {
				return err
			}
			dzm.Add(device.Info.Id, n.Info.Zone)
		}
		return nil
	})
	return dzm, err
}

func (dzm *DeviceZoneMap) Add(deviceId string, zone int) {
	dzm.AvailableZones[zone] = true
	dzm.DeviceZones[deviceId] = zone
}

func (dzm *DeviceZoneMap) Filter(bs *BrickSet, d *DeviceEntry) bool {
	// TODO: need to consider cluster
	zonesUsed := map[int]bool{}
	for _, b := range bs.Contents() {
		brickZone, found := dzm.DeviceZones[b.Info.DeviceId]
		if !found {
			// Should not happen
			logger.Warning("device id %v not found in zone map", b.Info.DeviceId)
			return false
		}
		logger.Debug("zone %v in use by brick set", brickZone)
		zonesUsed[brickZone] = true
	}
	dzone := dzm.DeviceZones[d.Info.Id]
	return !zonesUsed[dzone]
}
