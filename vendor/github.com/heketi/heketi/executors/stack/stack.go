//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package stack

import (
	"github.com/heketi/heketi/executors"
)

var (
	NotSupportedError = executors.NotSupportedError
)

type ExecutorStack struct {
	executors []executors.Executor

	// control for "special" functions
	CheckAllGlusterd bool
}

func NewExecutorStack(e ...executors.Executor) *ExecutorStack {
	return &ExecutorStack{
		executors: e,
	}
}

func (es *ExecutorStack) SetExec(e []executors.Executor) {
	es.executors = e
}

func (es *ExecutorStack) GlusterdCheck(host string) error {
	err := NotSupportedError
	for _, e := range es.executors {
		err = e.GlusterdCheck(host)
		if err == NotSupportedError || (err == nil && es.CheckAllGlusterd) {
			continue
		}
		return err
	}
	return err
}

func (es *ExecutorStack) PeerProbe(exec_host, newnode string) error {
	for _, e := range es.executors {
		err := e.PeerProbe(exec_host, newnode)
		if err != NotSupportedError {
			return err
		}
	}
	return NotSupportedError
}

func (es *ExecutorStack) PeerDetach(exec_host, detachnode string) error {
	for _, e := range es.executors {
		err := e.PeerDetach(exec_host, detachnode)
		if err != NotSupportedError {
			return err
		}
	}
	return NotSupportedError
}

func (es *ExecutorStack) DeviceSetup(host, device, vgid string, destroy bool) (*executors.DeviceInfo, error) {
	for _, e := range es.executors {
		di, err := e.DeviceSetup(host, device, vgid, destroy)
		if err != NotSupportedError {
			return di, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) GetDeviceInfo(host, device, vgid string) (*executors.DeviceInfo, error) {
	for _, e := range es.executors {
		di, err := e.GetDeviceInfo(host, device, vgid)
		if err != NotSupportedError {
			return di, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) DeviceTeardown(host, device, vgid string) error {
	for _, e := range es.executors {
		err := e.DeviceTeardown(host, device, vgid)
		if err != NotSupportedError {
			return err
		}
	}
	return NotSupportedError
}

func (es *ExecutorStack) DeviceForget(host, device, vgid string) error {
	for _, e := range es.executors {
		err := e.DeviceForget(host, device, vgid)
		if err != NotSupportedError {
			return err
		}
	}
	return NotSupportedError
}

func (es *ExecutorStack) BrickCreate(host string, brick *executors.BrickRequest) (*executors.BrickInfo, error) {
	for _, e := range es.executors {
		bi, err := e.BrickCreate(host, brick)
		if err != NotSupportedError {
			return bi, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) BrickDestroy(host string, brick *executors.BrickRequest) (bool, error) {
	for _, e := range es.executors {
		r, err := e.BrickDestroy(host, brick)
		if err != NotSupportedError {
			return r, err
		}
	}
	return false, NotSupportedError
}

func (es *ExecutorStack) VolumeCreate(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
	for _, e := range es.executors {
		v, err := e.VolumeCreate(host, volume)
		if err != NotSupportedError {
			return v, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) VolumeDestroy(host string, volume string) error {
	for _, e := range es.executors {
		err := e.VolumeDestroy(host, volume)
		if err != NotSupportedError {
			return err
		}
	}
	return NotSupportedError
}

func (es *ExecutorStack) VolumeDestroyCheck(host, volume string) error {
	for _, e := range es.executors {
		err := e.VolumeDestroyCheck(host, volume)
		if err != NotSupportedError {
			return err
		}
	}
	return NotSupportedError
}

func (es *ExecutorStack) VolumeExpand(host string, volume *executors.VolumeRequest) (*executors.Volume, error) {
	for _, e := range es.executors {
		v, err := e.VolumeExpand(host, volume)
		if err != NotSupportedError {
			return v, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) VolumeReplaceBrick(host string, volume string, oldBrick *executors.BrickInfo, newBrick *executors.BrickInfo) error {
	for _, e := range es.executors {
		err := e.VolumeReplaceBrick(host, volume, oldBrick, newBrick)
		if err != NotSupportedError {
			return err
		}
	}
	return NotSupportedError
}

func (es *ExecutorStack) VolumeInfo(host string, volume string) (*executors.Volume, error) {
	for _, e := range es.executors {
		v, err := e.VolumeInfo(host, volume)
		if err != NotSupportedError {
			return v, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) VolumesInfo(host string) (*executors.VolInfo, error) {
	for _, e := range es.executors {
		v, err := e.VolumesInfo(host)
		if err != NotSupportedError {
			return v, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) HealInfo(host string, volume string) (*executors.HealInfo, error) {
	for _, e := range es.executors {
		hi, err := e.HealInfo(host, volume)
		if err != NotSupportedError {
			return hi, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) SetLogLevel(level string) {
	for _, e := range es.executors {
		e.SetLogLevel(level)
	}
}

func (es *ExecutorStack) BlockVolumeCreate(host string, blockVolume *executors.BlockVolumeRequest) (*executors.BlockVolumeInfo, error) {
	for _, e := range es.executors {
		bvi, err := e.BlockVolumeCreate(host, blockVolume)
		if err != NotSupportedError {
			return bvi, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) BlockVolumeDestroy(host string, blockHostingVolumeName string, blockVolumeName string) error {
	for _, e := range es.executors {
		err := e.BlockVolumeDestroy(host, blockHostingVolumeName, blockVolumeName)
		if err != NotSupportedError {
			return err
		}
	}
	return NotSupportedError
}

func (es *ExecutorStack) VolumeClone(
	host string, vsr *executors.VolumeCloneRequest) (*executors.Volume, error) {

	for _, e := range es.executors {
		v, err := e.VolumeClone(host, vsr)
		if err != NotSupportedError {
			return v, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) VolumeSnapshot(
	host string, vsr *executors.VolumeSnapshotRequest) (*executors.Snapshot, error) {

	for _, e := range es.executors {
		s, err := e.VolumeSnapshot(host, vsr)
		if err != NotSupportedError {
			return s, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) SnapshotCloneVolume(
	host string, scr *executors.SnapshotCloneRequest) (*executors.Volume, error) {

	for _, e := range es.executors {
		v, err := e.SnapshotCloneVolume(host, scr)
		if err != NotSupportedError {
			return v, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) SnapshotCloneBlockVolume(
	host string, scr *executors.SnapshotCloneRequest) (*executors.BlockVolumeInfo, error) {

	for _, e := range es.executors {
		bvi, err := e.SnapshotCloneBlockVolume(host, scr)
		if err != NotSupportedError {
			return bvi, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) SnapshotDestroy(
	host string, snapshot string) error {

	for _, e := range es.executors {
		err := e.SnapshotDestroy(host, snapshot)
		if err != NotSupportedError {
			return err
		}
	}
	return NotSupportedError
}

func (es *ExecutorStack) PVS(host string) (*executors.PVSCommandOutput, error) {
	for _, e := range es.executors {
		v, err := e.PVS(host)
		if err != NotSupportedError {
			return v, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) VGS(host string) (*executors.VGSCommandOutput, error) {
	for _, e := range es.executors {
		v, err := e.VGS(host)
		if err != NotSupportedError {
			return v, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) LVS(host string) (*executors.LVSCommandOutput, error) {
	for _, e := range es.executors {
		v, err := e.LVS(host)
		if err != NotSupportedError {
			return v, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) GetBrickMountStatus(host string) (*executors.BricksMountStatus, error) {
	for _, e := range es.executors {
		v, err := e.GetBrickMountStatus(host)
		if err != NotSupportedError {
			return v, err
		}
	}
	return nil, NotSupportedError
}

func (es *ExecutorStack) ListBlockVolumes(host string, blockhostingvolume string) ([]string, error) {
	for _, e := range es.executors {
		v, err := e.ListBlockVolumes(host, blockhostingvolume)
		if err != NotSupportedError {
			return v, err
		}
	}
	return nil, NotSupportedError
}
