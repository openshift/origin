//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package injectexec

import (
	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/executors/mockexec"
)

var NotSupportedError = executors.NotSupportedError

// newMockBase returns a mock executor set up for use as the first "dummy"
// executor in the error inject executor's stack.
// This functions on this executor can be overridden directly for test
// purposes.
func newMockBase() *mockexec.MockExecutor {
	m, _ := mockexec.NewMockExecutor()

	m.MockGlusterdCheck = func(host string) error {
		return NotSupportedError
	}
	m.MockPeerProbe = func(exec_host, newnode string) error {
		return NotSupportedError
	}
	m.MockPeerDetach = func(exec_host, newnode string) error {
		return NotSupportedError
	}
	m.MockDeviceSetup = func(host, device, vgid string, destroy bool) (*executors.DeviceInfo, error) {
		return nil, NotSupportedError
	}
	m.MockDeviceTeardown = func(host, device, vgid string) error {
		return NotSupportedError
	}
	m.MockBrickCreate = func(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error) {
		return nil, NotSupportedError
	}
	m.MockBrickDestroy = func(host string, brick *executors.BrickRequest) (bool, error) {
		return true, NotSupportedError
	}
	m.MockVolumeCreate = func(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
		return nil, NotSupportedError
	}
	m.MockVolumeExpand = func(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
		return nil, NotSupportedError
	}
	m.MockVolumeDestroy = func(host string, volume string) error {
		return NotSupportedError
	}
	m.MockVolumeDestroyCheck = func(host, volume string) error {
		return NotSupportedError
	}
	m.MockVolumeReplaceBrick = func(host string, volume string, oldBrick *executors.BrickInfo, newBrick *executors.BrickInfo) error {
		return NotSupportedError
	}
	m.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		return nil, NotSupportedError
	}
	m.MockVolumesInfo = func(host string) (*executors.VolInfo, error) {
		return nil, NotSupportedError
	}
	m.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return nil, NotSupportedError
	}
	m.MockBlockVolumeCreate = func(host string, blockVolume *executors.BlockVolumeRequest) (*executors.BlockVolumeInfo, error) {
		return nil, NotSupportedError
	}
	m.MockBlockVolumeDestroy = func(host string, blockHostingVolumeName string, blockVolumeName string) error {
		return NotSupportedError
	}
	m.MockVolumeClone = func(host string, volume *executors.VolumeCloneRequest) (*executors.Volume, error) {
		return nil, NotSupportedError
	}
	m.MockVolumeSnapshot = func(host string, volume *executors.VolumeSnapshotRequest) (*executors.Snapshot, error) {
		return nil, NotSupportedError
	}
	m.MockSnapshotCloneVolume = func(host string, volume *executors.SnapshotCloneRequest) (*executors.Volume, error) {
		return nil, NotSupportedError
	}
	m.MockSnapshotCloneBlockVolume = func(host string, volume *executors.SnapshotCloneRequest) (*executors.BlockVolumeInfo, error) {
		return nil, NotSupportedError
	}
	m.MockSnapshotDestroy = func(host string, snapshot string) error {
		return NotSupportedError
	}
	m.MockPVS = func(host string) (*executors.PVSCommandOutput, error) {
		return nil, NotSupportedError
	}
	m.MockVGS = func(host string) (*executors.VGSCommandOutput, error) {
		return nil, NotSupportedError
	}
	m.MockLVS = func(host string) (*executors.LVSCommandOutput, error) {
		return nil, NotSupportedError
	}
	m.MockGetBrickMountStatus = func(host string) (*executors.BricksMountStatus, error) {
		return nil, NotSupportedError
	}
	m.MockListBlockVolumes = func(host string, blockhostingvolume string) ([]string, error) {
		return nil, NotSupportedError
	}
	return m
}
