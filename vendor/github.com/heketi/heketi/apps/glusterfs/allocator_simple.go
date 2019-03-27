//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

// Simple allocator contains a map to rings of clusters
type SimpleAllocator struct {
}

// Create a new simple allocator
func NewSimpleAllocator() *SimpleAllocator {
	s := &SimpleAllocator{}
	return s
}

func loadRingFromDeviceSource(dsrc DeviceSource) (
	*SimpleAllocatorRing, error) {

	ring := NewSimpleAllocatorRing()
	dnl, err := dsrc.Devices()
	if err != nil {
		return nil, err
	}
	for _, dan := range dnl {
		ring.Add(&SimpleDevice{
			zone:     dan.Node.Info.Zone,
			nodeId:   dan.Node.Info.Id,
			deviceId: dan.Device.Info.Id,
		})
	}
	return ring, nil
}

// GetNodesFromDeviceSource is a shim function that should only
// exist as long as we keep the intermediate simple allocator.
func (s *SimpleAllocator) GetNodesFromDeviceSource(dsrc DeviceSource,
	brickId string) (
	<-chan string, chan<- struct{}, error) {

	device, done := make(chan string), make(chan struct{})

	ring, err := loadRingFromDeviceSource(dsrc)
	if err != nil {
		close(device)
		return device, done, err
	}
	devicelist := ring.GetDeviceList(brickId)

	generateDevices(devicelist, device, done)
	return device, done, nil
}

func generateDevices(devicelist SimpleDevices,
	device chan<- string, done <-chan struct{}) {

	// Start generator in a new goroutine
	go func() {
		defer func() {
			close(device)
		}()

		for _, d := range devicelist {
			select {
			case device <- d.deviceId:
			case <-done:
				return
			}
		}
	}()
}
