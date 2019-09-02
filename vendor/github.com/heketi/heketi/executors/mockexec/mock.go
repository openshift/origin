//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package mockexec

import (
	"os"
	"strconv"

	"github.com/heketi/heketi/executors"
)

type MockExecutor struct {
	// These functions can be overwritten for testing
	MockGlusterdCheck            func(host string) error
	MockPeerProbe                func(exec_host, newnode string) error
	MockPeerDetach               func(exec_host, newnode string) error
	MockDeviceSetup              func(host, device, vgid string, destroy bool) (*executors.DeviceInfo, error)
	MockDeviceTeardown           func(host, device, vgid string) error
	MockGetDeviceInfo            func(host, device, vgid string) (*executors.DeviceInfo, error)
	MockBrickCreate              func(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error)
	MockBrickDestroy             func(host string, brick *executors.BrickRequest) (bool, error)
	MockVolumeCreate             func(host string, volume *executors.VolumeRequest) (*executors.Volume, error)
	MockVolumeExpand             func(host string, volume *executors.VolumeRequest) (*executors.Volume, error)
	MockVolumeDestroy            func(host string, volume string) error
	MockVolumeDestroyCheck       func(host, volume string) error
	MockVolumeReplaceBrick       func(host string, volume string, oldBrick *executors.BrickInfo, newBrick *executors.BrickInfo) error
	MockVolumeInfo               func(host string, volume string) (*executors.Volume, error)
	MockVolumesInfo              func(host string) (*executors.VolInfo, error)
	MockVolumeClone              func(host string, volume *executors.VolumeCloneRequest) (*executors.Volume, error)
	MockVolumeSnapshot           func(host string, volume *executors.VolumeSnapshotRequest) (*executors.Snapshot, error)
	MockSnapshotCloneVolume      func(host string, volume *executors.SnapshotCloneRequest) (*executors.Volume, error)
	MockSnapshotCloneBlockVolume func(host string, volume *executors.SnapshotCloneRequest) (*executors.BlockVolumeInfo, error)
	MockSnapshotDestroy          func(host string, snapshot string) error
	MockHealInfo                 func(host string, volume string) (*executors.HealInfo, error)
	MockBlockVolumeCreate        func(host string, blockVolume *executors.BlockVolumeRequest) (*executors.BlockVolumeInfo, error)
	MockBlockVolumeDestroy       func(host string, blockHostingVolumeName string, blockVolumeName string) error
	MockPVS                      func(host string) (*executors.PVSCommandOutput, error)
	MockVGS                      func(host string) (*executors.VGSCommandOutput, error)
	MockLVS                      func(host string) (*executors.LVSCommandOutput, error)
	MockGetBrickMountStatus      func(host string) (*executors.BricksMountStatus, error)
	MockListBlockVolumes         func(host string, blockhostingvolume string) ([]string, error)

	// default values
	DeviceSizeGb func() uint64
}

func NewMockExecutor() (*MockExecutor, error) {
	m := &MockExecutor{}

	m.MockGlusterdCheck = func(host string) error {
		return nil
	}

	m.MockPeerProbe = func(exec_host, newnode string) error {
		return nil
	}

	m.MockPeerDetach = func(exec_host, newnode string) error {
		return nil
	}

	m.MockDeviceSetup = func(host, device, vgid string, destroy bool) (*executors.DeviceInfo, error) {
		dsize := m.DeviceSizeGb() * 1024 * 1024
		d := &executors.DeviceInfo{}
		d.TotalSize = dsize
		d.FreeSize = dsize
		d.UsedSize = 0
		d.ExtentSize = 4096
		return d, nil
	}

	m.MockDeviceTeardown = func(host, device, vgid string) error {
		return nil
	}

	m.MockGetDeviceInfo = func(host, device, vgid string) (*executors.DeviceInfo, error) {
		dsize := m.DeviceSizeGb() * 1024 * 1024
		d := &executors.DeviceInfo{}
		d.TotalSize = dsize
		d.FreeSize = dsize
		d.UsedSize = 0
		d.ExtentSize = 4096
		return d, nil
	}

	m.MockBrickCreate = func(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error) {
		b := &executors.BrickInfo{
			Path: "/mockpath",
		}
		return b, nil
	}

	m.MockBrickDestroy = func(host string, brick *executors.BrickRequest) (bool, error) {
		// We'll assume that the space of the brick has been reclaimed
		return true, nil
	}

	m.MockVolumeCreate = func(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
		return &executors.Volume{}, nil
	}

	m.MockVolumeExpand = func(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
		return &executors.Volume{}, nil
	}

	m.MockVolumeDestroy = func(host string, volume string) error {
		return nil
	}

	m.MockVolumeDestroyCheck = func(host, volume string) error {
		return nil
	}

	m.MockVolumeReplaceBrick = func(host string, volume string, oldBrick *executors.BrickInfo, newBrick *executors.BrickInfo) error {
		return nil
	}

	m.MockVolumeInfo = func(host string, volume string) (*executors.Volume, error) {
		var bricks []executors.Brick
		brick := executors.Brick{Name: host + ":/mockpath"}
		bricks = append(bricks, brick)
		brick = executors.Brick{Name: host + ":/mockpath"}
		bricks = append(bricks, brick)
		brick = executors.Brick{Name: host + ":/mockpath"}
		bricks = append(bricks, brick)
		Bricks := executors.Bricks{
			BrickList: bricks,
		}
		vinfo := &executors.Volume{
			Bricks: Bricks,
		}
		return vinfo, nil
	}

	m.MockVolumesInfo = func(host string) (*executors.VolInfo, error) {
		var bricks []executors.Brick
		brick := executors.Brick{Name: host + ":/mockpath"}
		bricks = append(bricks, brick)
		brick = executors.Brick{Name: host + ":/mockpath"}
		bricks = append(bricks, brick)
		brick = executors.Brick{Name: host + ":/mockpath"}
		bricks = append(bricks, brick)
		Bricks := executors.Bricks{
			BrickList: bricks,
		}
		vinfo := executors.Volume{
			Bricks: Bricks,
		}
		volumelist := make([]executors.Volume, 0)
		volumelist = append(volumelist, vinfo)
		volumelist = append(volumelist, vinfo)

		volumes := executors.Volumes{
			Count:      2,
			VolumeList: volumelist,
		}
		volinfo := &executors.VolInfo{
			Volumes: volumes,
		}
		return volinfo, nil
	}

	m.MockVolumeSnapshot = func(host string, vsr *executors.VolumeSnapshotRequest) (*executors.Snapshot, error) {
		snapshot := &executors.Snapshot{
			Name: vsr.Snapshot,
			// TODO: fill more properties
		}

		return snapshot, nil
	}

	m.MockVolumeClone = func(host string, vcr *executors.VolumeCloneRequest) (*executors.Volume, error) {
		vinfo := &executors.Volume{
			VolumeName: "clone_of_" + vcr.Volume,
			// TODO: fill more properties
		}

		return vinfo, nil
	}

	m.MockSnapshotCloneVolume = func(host string, scr *executors.SnapshotCloneRequest) (*executors.Volume, error) {
		vinfo := &executors.Volume{
			VolumeName: scr.Volume,
			// TODO: fill more properties
		}

		return vinfo, nil
	}

	m.MockSnapshotCloneBlockVolume = func(host string, scr *executors.SnapshotCloneRequest) (*executors.BlockVolumeInfo, error) {
		bvi := &executors.BlockVolumeInfo{
			Name: scr.Volume,
			// TODO: fill more properties
		}

		return bvi, nil
	}

	m.MockSnapshotDestroy = func(host string, snapshot string) error {
		return nil
	}

	m.MockHealInfo = func(host string, volume string) (*executors.HealInfo, error) {
		return &executors.HealInfo{}, nil
	}

	m.MockBlockVolumeCreate = func(host string, blockVolume *executors.BlockVolumeRequest) (*executors.BlockVolumeInfo, error) {
		var blockVolumeInfo executors.BlockVolumeInfo
		blockVolumeInfo.BlockHosts = blockVolume.BlockHosts
		blockVolumeInfo.GlusterNode = blockVolume.GlusterNode
		blockVolumeInfo.GlusterVolumeName = blockVolume.GlusterVolumeName
		blockVolumeInfo.Hacount = blockVolume.Hacount
		blockVolumeInfo.Iqn = "fakeIQN"
		if blockVolume.Auth {
			blockVolumeInfo.Username = "heketi-user"
			blockVolumeInfo.Password = "secret"
		}
		blockVolumeInfo.Name = blockVolume.Name
		blockVolumeInfo.Size = blockVolume.Size

		return &blockVolumeInfo, nil
	}

	m.MockBlockVolumeDestroy = func(host string, blockHostingVolumeName string, blockVolumeName string) error {
		return nil
	}

	m.MockPVS = func(host string) (*executors.PVSCommandOutput, error) {
		return &executors.PVSCommandOutput{}, nil
	}

	m.MockVGS = func(host string) (*executors.VGSCommandOutput, error) {
		return &executors.VGSCommandOutput{}, nil
	}

	m.MockLVS = func(host string) (*executors.LVSCommandOutput, error) {
		return &executors.LVSCommandOutput{}, nil
	}

	m.MockGetBrickMountStatus = func(host string) (*executors.BricksMountStatus, error) {
		return &executors.BricksMountStatus{}, nil
	}

	m.MockListBlockVolumes = func(host string, blockhostingvolume string) ([]string, error) {
		return []string{}, nil
	}

	m.DeviceSizeGb = func() uint64 {
		env := os.Getenv("HEKETI_MOCK_DEVICE_SIZE_GB")
		if env != "" {
			value, err := strconv.ParseInt(env, 10, 64)
			if err == nil {
				return uint64(value)
			}
		}
		return 500
	}

	return m, nil
}

func (m *MockExecutor) SetLogLevel(level string) {

}

func (m *MockExecutor) GlusterdCheck(host string) error {
	return m.MockGlusterdCheck(host)
}

func (m *MockExecutor) PeerProbe(exec_host, newnode string) error {
	return m.MockPeerProbe(exec_host, newnode)
}

func (m *MockExecutor) PeerDetach(exec_host, newnode string) error {
	return m.MockPeerDetach(exec_host, newnode)
}

func (m *MockExecutor) DeviceSetup(host, device, vgid string, destroy bool) (*executors.DeviceInfo, error) {
	return m.MockDeviceSetup(host, device, vgid, destroy)
}

func (m *MockExecutor) GetDeviceInfo(host, device, vgid string) (*executors.DeviceInfo, error) {
	return m.MockGetDeviceInfo(host, device, vgid)
}

func (m *MockExecutor) DeviceTeardown(host, device, vgid string) error {
	return m.MockDeviceTeardown(host, device, vgid)
}

func (m *MockExecutor) DeviceForget(host, device, vgid string) error {
	return m.MockDeviceTeardown(host, device, vgid)
}

func (m *MockExecutor) BrickCreate(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error) {
	return m.MockBrickCreate(host, brick)
}

func (m *MockExecutor) BrickDestroy(host string, brick *executors.BrickRequest) (bool, error) {
	return m.MockBrickDestroy(host, brick)
}

func (m *MockExecutor) VolumeCreate(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
	return m.MockVolumeCreate(host, volume)
}

func (m *MockExecutor) VolumeExpand(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
	return m.MockVolumeExpand(host, volume)
}

func (m *MockExecutor) VolumeDestroy(host string, volume string) error {
	return m.MockVolumeDestroy(host, volume)
}

func (m *MockExecutor) VolumeDestroyCheck(host string, volume string) error {
	return m.MockVolumeDestroyCheck(host, volume)
}

func (m *MockExecutor) VolumeReplaceBrick(host string, volume string, oldBrick *executors.BrickInfo, newBrick *executors.BrickInfo) error {
	return m.MockVolumeReplaceBrick(host, volume, oldBrick, newBrick)
}

func (m *MockExecutor) VolumeInfo(host string, volume string) (*executors.Volume, error) {
	return m.MockVolumeInfo(host, volume)
}

func (m *MockExecutor) VolumesInfo(host string) (*executors.VolInfo, error) {
	return m.MockVolumesInfo(host)
}

func (m *MockExecutor) VolumeClone(host string, vcr *executors.VolumeCloneRequest) (*executors.Volume, error) {
	return m.MockVolumeClone(host, vcr)
}

func (m *MockExecutor) VolumeSnapshot(host string, vsr *executors.VolumeSnapshotRequest) (*executors.Snapshot, error) {
	return m.MockVolumeSnapshot(host, vsr)
}

func (m *MockExecutor) SnapshotCloneVolume(host string, scr *executors.SnapshotCloneRequest) (*executors.Volume, error) {
	return m.MockSnapshotCloneVolume(host, scr)
}

func (m *MockExecutor) SnapshotCloneBlockVolume(host string, scr *executors.SnapshotCloneRequest) (*executors.BlockVolumeInfo, error) {
	return m.MockSnapshotCloneBlockVolume(host, scr)
}

func (m *MockExecutor) SnapshotDestroy(host string, snapshot string) error {
	return m.MockSnapshotDestroy(host, snapshot)
}

func (m *MockExecutor) HealInfo(host string, volume string) (*executors.HealInfo, error) {
	return m.MockHealInfo(host, volume)
}

func (m *MockExecutor) BlockVolumeCreate(host string, blockVolume *executors.BlockVolumeRequest) (*executors.BlockVolumeInfo, error) {
	return m.MockBlockVolumeCreate(host, blockVolume)
}

func (m *MockExecutor) BlockVolumeDestroy(host string, blockHostingVolumeName string, blockVolumeName string) error {
	return m.MockBlockVolumeDestroy(host, blockHostingVolumeName, blockVolumeName)
}

func (m *MockExecutor) PVS(host string) (*executors.PVSCommandOutput, error) {
	return m.MockPVS(host)
}

func (m *MockExecutor) VGS(host string) (*executors.VGSCommandOutput, error) {
	return m.MockVGS(host)
}

func (m *MockExecutor) LVS(host string) (*executors.LVSCommandOutput, error) {
	return m.MockLVS(host)
}

func (m *MockExecutor) GetBrickMountStatus(host string) (*executors.BricksMountStatus, error) {
	return m.MockGetBrickMountStatus(host)
}

func (m *MockExecutor) ListBlockVolumes(host string, blockhostingvolume string) ([]string, error) {
	return m.MockListBlockVolumes(host, blockhostingvolume)
}
