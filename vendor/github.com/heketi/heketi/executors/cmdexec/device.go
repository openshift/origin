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

	// used for certain lvm functions
	LV_UUID_PREFIX = "/dev/disk/by-id/lvm-pv-uuid-"
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
	err := rex.AnyError(s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands), 5))
	if err != nil {
		err = s.deviceSetupError(err, host, device)
		return nil, err
	}

	// Create a cleanup function if anything fails
	defer func() {
		if e != nil {
			s.DeviceTeardown(host, executors.SimpleDeviceVgHandle(device, vgid))
		}
	}()

	d = &executors.DeviceInfo{}
	dh, err := s.getDeviceHandle(host, device)
	if err != nil {
		return nil, err
	}
	d.Meta = dh
	err = s.getVgSizeFromNode(d, host, device, vgid)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (s *CmdExecutor) PVS(host string) (d *executors.PVSCommandOutput, e error) {

	// Setup commands
	commands := []string{}

	commands = append(commands, fmt.Sprintf("pvs --reportformat json --units k"))

	results, err := s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands),
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

	results, err := s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands),
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

	results, err := s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands),
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

func (s *CmdExecutor) GetDeviceInfo(host string, dh *executors.DeviceVgHandle) (d *executors.DeviceInfo, e error) {
	// TODO: the actions of upgradeHandle and getDeviceHandle are a bit
	// overlapping. Perhaps it would be good to reduce the overlap if possible
	if err := s.upgradeHandle(host, dh); err != nil {
		return nil, err
	}
	paths := handlePaths(dh)
	d = &executors.DeviceInfo{}
	newdh, err := s.getDeviceHandle(host, paths[0])
	if err != nil {
		return nil, err
	}
	d.Meta = newdh
	err = s.getVgSizeFromNode(d, host, paths[0], dh.VgId)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (s *CmdExecutor) DeviceTeardown(host string, dh *executors.DeviceVgHandle) error {
	if err := s.upgradeHandle(host, dh); err != nil {
		return err
	}
	paths := handlePaths(dh)
	if err := s.removeDevice(host, paths[0], dh.VgId); err != nil {
		return err
	}
	return s.removeDeviceMountPoint(host, dh.VgId)
}

// DeviceForget attempts a best effort remove of the device's vg and
// pv and always returns a nil error.
func (s *CmdExecutor) DeviceForget(host string, dh *executors.DeviceVgHandle) error {
	if err := s.upgradeHandle(host, dh); err != nil {
		return err
	}
	paths := handlePaths(dh)
	s.removeDeviceMountPoint(host, dh.VgId)
	s.removeDevice(host, paths[0], dh.VgId)
	return nil
}

func (s *CmdExecutor) removeDevice(host, device, vgid string) error {
	commands := []string{
		fmt.Sprintf("vgremove -qq %v", paths.VgIdToName(vgid)),
		fmt.Sprintf("pvremove -qq '%v'", device),
	}

	// Execute command
	err := rex.AnyError(s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands), 5))
	if err != nil {
		return logger.LogError(
			"Failed to delete device %v with id %v on host %v: %v",
			device, vgid, host, err)
	}
	return nil
}

func (s *CmdExecutor) removeDeviceMountPoint(host, vgid string) error {
	pdir := paths.BrickMountPointParent(vgid)
	commands := []string{
		fmt.Sprintf("rmdir %v", pdir),
	}

	err := rex.AnyError(s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands), 5))
	if err != nil && !strings.Contains(err.Error(), "No such file or directory") {
		logger.LogError("Error while removing the VG directory: %v", err)
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
	results, err := s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands), 5)
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

func (s *CmdExecutor) getDeviceHandle(host, device string) (
	*executors.DeviceHandle, error) {

	commands := []string{}
	commands = append(commands,
		fmt.Sprintf("pvs -o pv_name,pv_uuid,vg_name --reportformat=json %v", device))
	commands = append(commands,
		fmt.Sprintf("udevadm info --query=symlink --name=%v", device))

	results, err := s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands), 5)
	if err != nil {
		return nil, connErr("failed to get device handle", err)
	}
	if !results[0].Ok() {
		// pvs command is expected to always work
		return nil, fmt.Errorf("failed to read PV data for %v [%v]", device, results[0].Error())
	}
	dh := &executors.DeviceHandle{Paths: []string{}}
	dh.UUID, err = parsePvsResult(results[0].Output)
	if err != nil {
		return nil, logger.LogError("failed to parse lvs output: %v", err)
	}
	if results[1].Ok() {
		// udev returned a list of aliases (symlinks) for the device
		// parse and collect them
		foundPaths, err := parseUdevPaths(results[1].Output, device)
		if err != nil {
			return nil, err
		}
		dh.Paths = append(dh.Paths, foundPaths...)
	} else {
		dh.Paths = append(dh.Paths, device)
	}
	return dh, nil
}

func (s *CmdExecutor) upgradeHandle(host string, dh *executors.DeviceVgHandle) error {
	if err := checkHandle(dh); err != nil {
		return err
	}
	if dh.UUID != "" {
		return nil
	}
	// this handle lacks a uuid, our preferred way to get a persistent
	// path in /dev. Try to get one based on the vg id
	commands := []string{}
	commands = append(commands,
		fmt.Sprintf("vgs -o pv_name,pv_uuid,vg_name --reportformat=json %v",
			paths.VgIdToName(dh.VgId)))
	results, err := s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands), 5)
	if e := rex.AnyError(results, err); e != nil {
		logger.Warning("failed to get vgs info for handle: %v", err)
		return nil
	}
	dh.UUID, err = parsePvsResult(results[0].Output)
	if err != nil {
		logger.Warning("failed to parse vgs output: %v", err)
	}
	return nil
}

func (s *CmdExecutor) deviceSetupError(e error, host, device string) *executors.DeviceNotAvailableErr {
	var out *executors.DeviceNotAvailableErr
	dh, err := s.getDeviceHandle(host, device)
	if err == nil {
		// device has lvm metadata on it. return the specific error type
		// with the current lvm metadata set
		out = &executors.DeviceNotAvailableErr{
			OriginalError: e,
			Path:          device,
			ConnectionOk:  true,
			CurrentMeta:   dh,
		}
	} else if _, ok := err.(connectionErr); ok {
		// we failed to get any metadata from the node. We can't trust
		// this "non-result" so we assume device may still be in use
		out = &executors.DeviceNotAvailableErr{
			OriginalError: e,
			Path:          device,
			ConnectionOk:  false,
			CurrentMeta:   nil,
		}
	} else {
		// we connected to the node and ran our info commands but they
		// did not return lvm pv metadata
		out = &executors.DeviceNotAvailableErr{
			OriginalError: e,
			Path:          device,
			ConnectionOk:  true,
			CurrentMeta:   nil,
		}
	}
	return out
}

func checkHandle(dh *executors.DeviceVgHandle) error {
	if dh.UUID == "" && len(dh.Paths) == 0 {
		return errors.New("device handle missing UUID and Paths")
	}
	if dh.VgId == "" {
		return errors.New("device handle missing VG id")
	}
	return nil
}

func handlePaths(dh *executors.DeviceVgHandle) []string {
	p := []string{}
	if dh.UUID != "" {
		p = append(p, fmt.Sprintf("%v%v", LV_UUID_PREFIX, dh.UUID))
	}
	p = append(p, dh.Paths...)
	return p
}

func parsePvsResult(o string) (string, error) {
	type pvEntry struct {
		PVName string `json:"pv_name"`
		PVUUID string `json:"pv_uuid"`
		VGName string `json:"vg_name"`
	}
	type pvsInfoOutput struct {
		Report []struct {
			PVS []pvEntry `json:"pv"`
			VGS []pvEntry `json:"vg"`
		} `json:"report"`
	}
	var pvout pvsInfoOutput
	err := json.Unmarshal([]byte(o), &pvout)
	if err != nil {
		return "", fmt.Errorf("Failed to parse output: %v", err)
	}
	if len(pvout.Report) != 1 {
		return "", fmt.Errorf("Expected exactly 1 report in output")
	}
	if len(pvout.Report[0].PVS) == 1 {
		return pvout.Report[0].PVS[0].PVUUID, nil
	}
	if len(pvout.Report[0].VGS) == 1 {
		return pvout.Report[0].VGS[0].PVUUID, nil
	}
	return "", fmt.Errorf("Expected exactly 1 PV or 1 VG in output")
}

func parseUdevPaths(o, basePath string) ([]string, error) {
	paths := []string{}
	foundBasePath := false
	for _, s := range strings.Fields(o) {
		s = "/dev/" + s
		if strings.HasPrefix(s, LV_UUID_PREFIX) {
			// skip the lvm uuid path as we capture the lvm uuid separately
			continue
		}
		foundBasePath = foundBasePath || s == basePath
		paths = append(paths, s)
	}
	if !foundBasePath {
		paths = append(paths, basePath)
	}
	return paths, nil
}

type connectionErr struct {
	msg string
	err error
}

func (e connectionErr) Error() string {
	return fmt.Sprintf("%s: %v", e.msg, e.err)
}

func connErr(m string, e error) connectionErr {
	return connectionErr{m, e}
}
