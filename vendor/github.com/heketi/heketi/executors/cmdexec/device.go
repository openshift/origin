//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmdexec

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/heketi/heketi/executors"
	conv "github.com/heketi/heketi/pkg/conversions"
	"github.com/heketi/heketi/pkg/paths"
	rex "github.com/heketi/heketi/pkg/remoteexec"
)

const (
	VGDISPLAY_SIZE_KB                  = 11
	VGDISPLAY_PHYSICAL_EXTENT_SIZE     = 12
	VGDISPLAY_TOTAL_NUMBER_EXTENTS     = 13
	VGDISPLAY_ALLOCATED_NUMBER_EXTENTS = 14
	VGDISPLAY_FREE_NUMBER_EXTENTS      = 15
)

// Read:
// https://access.redhat.com/documentation/en-US/Red_Hat_Storage/3.1/html/Administration_Guide/Brick_Configuration.html
//

func (s *CmdExecutor) DeviceSetup(host, device, vgid string, destroy bool) (d *executors.DeviceInfo, e error) {

	// Setup commands
	commands := []string{}

	if destroy {
		logger.Info("Data on device %v (host %v) will be destroyed", device, host)
		commands = append(commands, fmt.Sprintf("wipefs --all %v", device))
	}
	commands = append(commands, fmt.Sprintf("pvcreate -qq --metadatasize=128M --dataalignment=%v '%v'", s.PVDataAlignment(), device))
	commands = append(commands, fmt.Sprintf("vgcreate -qq --physicalextentsize=%v --autobackup=%v %v %v",

		// Physical extent size
		s.VGPhysicalExtentSize(),

		// Autobackup
		conv.BoolToYN(s.BackupLVM),

		// Device
		paths.VgIdToName(vgid), device),
	)

	// Execute command
	err := rex.AnyError(s.RemoteExecutor.ExecCommands(host, commands, 5))
	if err != nil {
		err = fmt.Errorf("Setup of device %v failed (already initialized or contains data?): %v", device, err)
		return nil, err
	}

	// Create a cleanup function if anything fails
	defer func() {
		if e != nil {
			s.DeviceTeardown(host, device, vgid)
		}
	}()

	return s.GetDeviceInfo(host, device, vgid)
}

func (s *CmdExecutor) PVS(host string) (d *executors.PVSCommandOutput, e error) {

	// Setup commands
	commands := []string{}

	commands = append(commands, fmt.Sprintf("pvs --reportformat json --units k"))

	results, err := s.RemoteExecutor.ExecCommands(host, commands,
		s.GlusterCliExecTimeout())
	if err := rex.AnyError(results, err); err != nil {
		return nil, fmt.Errorf("Unable to get data for LVM PVs")
	}
	var pvsCommandOutput executors.PVSCommandOutput
	err = json.Unmarshal([]byte(results[0].Output), &pvsCommandOutput)
	if err != nil {
		return nil, fmt.Errorf("Unable to determine LVM PVs : %v", err)
	}
	return &pvsCommandOutput, nil
}

func (s *CmdExecutor) VGS(host string) (d *executors.VGSCommandOutput, e error) {

	// Setup commands
	commands := []string{}

	commands = append(commands, fmt.Sprintf("vgs --reportformat json --units k"))

	results, err := s.RemoteExecutor.ExecCommands(host, commands,
		s.GlusterCliExecTimeout())
	if err := rex.AnyError(results, err); err != nil {
		return nil, fmt.Errorf("Unable to get data for LVM VGs")
	}
	var vgsCommandOutput executors.VGSCommandOutput
	err = json.Unmarshal([]byte(results[0].Output), &vgsCommandOutput)
	if err != nil {
		return nil, fmt.Errorf("Unable to determine LVM VGs : %v", err)
	}
	return &vgsCommandOutput, nil
}

func (s *CmdExecutor) LVS(host string) (d *executors.LVSCommandOutput, e error) {

	// Setup commands
	commands := []string{}

	commands = append(commands, fmt.Sprintf("lvs --reportformat json --units k"))

	results, err := s.RemoteExecutor.ExecCommands(host, commands,
		s.GlusterCliExecTimeout())
	if err := rex.AnyError(results, err); err != nil {
		return nil, fmt.Errorf("Unable to get data for LVM LVs")
	}
	var lvsCommandOutput executors.LVSCommandOutput
	err = json.Unmarshal([]byte(results[0].Output), &lvsCommandOutput)
	if err != nil {
		return nil, fmt.Errorf("Unable to determine LVM LVs : %v", err)
	}
	return &lvsCommandOutput, nil
}

func (s *CmdExecutor) GetDeviceInfo(host, device, vgid string) (d *executors.DeviceInfo, e error) {
	// Vg info
	d = &executors.DeviceInfo{}
	err := s.getVgSizeFromNode(d, host, device, vgid)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (s *CmdExecutor) DeviceTeardown(host, device, vgid string) error {
	if err := s.removeDevice(host, device, vgid); err != nil {
		return err
	}
	return s.removeDeviceMountPoint(host, vgid)
}

// DeviceForget attempts a best effort remove of the device's vg and
// pv and always returns a nil error.
func (s *CmdExecutor) DeviceForget(host, device, vgid string) error {
	s.removeDeviceMountPoint(host, vgid)
	s.removeDevice(host, device, vgid)
	return nil
}

func (s *CmdExecutor) removeDevice(host, device, vgid string) error {
	commands := []string{
		fmt.Sprintf("vgremove -qq %v", paths.VgIdToName(vgid)),
		fmt.Sprintf("pvremove -qq '%v'", device),
	}

	// Execute command
	err := rex.AnyError(s.RemoteExecutor.ExecCommands(host, commands, 5))
	if err != nil {
		return logger.LogError(
			"Failed to delete device %v with id %v on host %v: %v",
			device, vgid, host, err)
	}
	return nil
}

func (s *CmdExecutor) removeDeviceMountPoint(host, vgid string) error {
	// TODO: remove this LBYL check and replace it with the rmdir
	// followed by error condition check that handles ENOENT
	pdir := paths.BrickMountPointParent(vgid)
	commands := []string{
		fmt.Sprintf("ls %v", pdir),
	}
	err := rex.AnyError(s.RemoteExecutor.ExecCommands(host, commands, 5))
	if err != nil {
		return nil
	}

	commands = []string{
		fmt.Sprintf("rmdir %v", pdir),
	}

	err = rex.AnyError(s.RemoteExecutor.ExecCommands(host, commands, 5))
	if err != nil {
		logger.LogError("Error while removing the VG directory")
	}
	return nil
}

func (s *CmdExecutor) getVgSizeFromNode(
	d *executors.DeviceInfo,
	host, device, vgid string) error {

	// Setup command
	commands := []string{
		fmt.Sprintf("vgdisplay -c %v", paths.VgIdToName(vgid)),
	}

	// Execute command
	results, err := s.RemoteExecutor.ExecCommands(host, commands, 5)
	if err := rex.AnyError(results, err); err != nil {
		return err
	}

	// Example:
	// sampleVg:r/w:772:-1:0:0:0:-1:0:4:4:2097135616:4096:511996:0:511996:rJ0bIG-3XNc-NoS0-fkKm-batK-dFyX-xbxHym
	vginfo := strings.Split(results[0].Output, ":")

	// See vgdisplay manpage
	if len(vginfo) < 17 {
		return errors.New("vgdisplay returned an invalid string")
	}

	extent_size, err :=
		strconv.ParseUint(vginfo[VGDISPLAY_PHYSICAL_EXTENT_SIZE], 10, 64)
	if err != nil {
		return err
	}

	free_extents, err :=
		strconv.ParseUint(vginfo[VGDISPLAY_FREE_NUMBER_EXTENTS], 10, 64)
	if err != nil {
		return err
	}

	allocated_extents, err :=
		strconv.ParseUint(vginfo[VGDISPLAY_ALLOCATED_NUMBER_EXTENTS], 10, 64)
	if err != nil {
		return err
	}

	total_extents, err :=
		strconv.ParseUint(vginfo[VGDISPLAY_TOTAL_NUMBER_EXTENTS], 10, 64)
	if err != nil {
		return err
	}

	d.TotalSize = total_extents * extent_size
	d.FreeSize = free_extents * extent_size
	d.UsedSize = allocated_extents * extent_size
	d.ExtentSize = extent_size
	logger.Debug("%v in %v has TotalSize:%v, FreeSize:%v, UsedSize:%v", device, host, d.TotalSize, d.FreeSize, d.UsedSize)
	return nil
}
