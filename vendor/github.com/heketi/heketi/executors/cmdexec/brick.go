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
	"fmt"
	"strconv"
	"strings"

	"github.com/heketi/heketi/executors"
	conv "github.com/heketi/heketi/pkg/conversions"
	"github.com/heketi/heketi/pkg/paths"
	"github.com/lpabon/godbc"
)

func (s *CmdExecutor) BrickCreate(host string,
	brick *executors.BrickRequest) (*executors.BrickInfo, error) {

	godbc.Require(brick != nil)
	godbc.Require(host != "")
	godbc.Require(brick.Name != "")
	godbc.Require(brick.Size > 0)
	godbc.Require(brick.TpSize >= brick.Size)
	godbc.Require(brick.VgId != "")
	godbc.Require(brick.Path != "")
	godbc.Require(s.Fstab != "")

	// make local vars with more accurate names to cut down on name confusion
	// and make future refactoring easier
	brickPath := brick.Path
	mountPath := paths.BrickMountFromPath(brickPath)

	var xfsInodeOptions string
	if brick.Format == executors.ArbiterFormat {
		xfsInodeOptions = "maxpct=100"
	} else {
		xfsInodeOptions = "size=512"
	}

	// Create command set to execute on the node
	devnode := paths.BrickDevNode(brick.VgId, brick.Name)
	commands := []string{

		// Create a directory
		fmt.Sprintf("mkdir -p %v", mountPath),

		// Setup the LV
		fmt.Sprintf("lvcreate -qq --autobackup=%v --poolmetadatasize %vK --chunksize 256K --size %vK --thin %v/%v --virtualsize %vK --name %v",
			// backup LVM metadata
			conv.BoolToYN(s.BackupLVM),

			// MetadataSize
			brick.PoolMetadataSize,

			//Thin Pool Size
			brick.TpSize,

			// volume group
			paths.VgIdToName(brick.VgId),

			// ThinP name
			brick.TpName,

			// Allocation size
			brick.Size,

			// Logical Vol name
			brick.LvName),

		// Format
		fmt.Sprintf("mkfs.xfs -i %v -n size=8192 %v", xfsInodeOptions, devnode),

		// Fstab
		fmt.Sprintf("awk \"BEGIN {print \\\"%v %v xfs rw,inode64,noatime,nouuid 1 2\\\" >> \\\"%v\\\"}\"",
			devnode,
			mountPath,
			s.Fstab),

		// Mount
		fmt.Sprintf("mount -o rw,inode64,noatime,nouuid %v %v", devnode, mountPath),

		// Create a directory inside the formated volume for GlusterFS
		fmt.Sprintf("mkdir %v", brickPath),
	}

	// Only set the GID if the value is other than root(gid 0).
	// When no gid is set, root is the only one that can write to the volume
	if 0 != brick.Gid {
		commands = append(commands, []string{
			// Set GID on brick
			fmt.Sprintf("chown :%v %v", brick.Gid, brickPath),

			// Set writable by GID and UID
			fmt.Sprintf("chmod 2775 %v", brickPath),
		}...)
	}

	// Execute commands
	_, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		// Cleanup
		s.BrickDestroy(host, brick)
		return nil, err
	}

	// Save brick location
	b := &executors.BrickInfo{
		Path: brickPath,
	}
	return b, nil
}

func (s *CmdExecutor) brickStorage(host string,
	brick *executors.BrickRequest) (string, string, error) {

	// Cloned bricks do not follow 'our' VG/LV naming, detect it.
	commands := []string{
		fmt.Sprintf("mount | grep -w %v | cut -d\" \" -f1", brick.Path),
	}
	output, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if output == nil || err != nil {
		return "", "", fmt.Errorf("No brick mounted on %v, unable to proceed with removing", brick.Path)
	}
	dev := output[0]
	// detect the thinp LV used by this brick (in "vg_.../tp_..." format)
	commands = []string{
		fmt.Sprintf("lvs --noheadings --separator=/ -ovg_name,pool_lv %v", dev),
	}
	output, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}
	tp := output[0]

	return dev, tp, nil
}

func (s *CmdExecutor) deleteBrickLV(host, lv string) error {
	// Remove the LV (by device name)
	commands := []string{
		fmt.Sprintf("lvremove --autobackup=%v -f %v",
			conv.BoolToYN(s.BackupLVM), lv),
	}
	_, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	return err
}

func (s *CmdExecutor) countThinLVsInPool(host, tp string) (int, error) {
	// Detect the number of bricks using the thin-pool
	commands := []string{
		fmt.Sprintf("lvs --noheadings --options=thin_count %v", tp),
	}
	output, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		return 0, err
	}
	thin_count, err := strconv.Atoi(strings.TrimSpace(output[0]))
	if err != nil {
		return 0, fmt.Errorf("Failed to convert number of logical volumes in thin pool %v on host %v: %v", tp, host, err)
	}
	return thin_count, nil
}

func (s *CmdExecutor) BrickDestroy(host string,
	brick *executors.BrickRequest) (bool, error) {

	godbc.Require(brick != nil)
	godbc.Require(host != "")
	godbc.Require(brick.Name != "")
	godbc.Require(brick.VgId != "")
	godbc.Require(brick.Path != "")
	godbc.Require(brick.TpName != "")
	godbc.Require(brick.LvName != "")

	var (
		umountErr      error
		spaceReclaimed bool
	)

	// TODO: convert to a best effort sanity check
	// dev, tp, err := s.brickStorage(host, brick)
	// if err != nil {
	// 	return spaceReclaimed, err
	// }

	// Try to unmount first
	commands := []string{
		fmt.Sprintf("umount %v", brick.Path),
	}
	_, umountErr = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if umountErr != nil {
		logger.Err(umountErr)
		// check if the brick was previously unmounted
		out, e := s.RemoteExecutor.RemoteCommandExecute(
			host, []string{"mount"}, 5)
		if e == nil && len(out) == 1 && !strings.Contains(out[0], brick.Path) {
			logger.Warning("brick path [%v] not mounted, assuming deleted",
				brick.Path)
			umountErr = nil
		}
	}

	// remove brick from fstab before we start deleting LVM items.
	// if heketi or the node was terminated while these steps are being
	// performed we'll orphan storage but the node should still be
	// bootable. If we remove LVM stuff first but leave an entry in
	// fstab referencing it, we could end up with a non-booting system.
	// Even if we failed to umount the brick, remove it from fstab
	// so that it does not get mounted again on next reboot.
	err := s.removeBrickFromFstab(host, brick)

	// if either umount or fstab remove failed there's no point in
	// continuing. We'll need either automated or manual recovery
	// in the future, but we need to know something went wrong.
	if err != nil {
		logger.Err(err)
		return spaceReclaimed, err
	}
	if umountErr != nil {
		return spaceReclaimed, umountErr
	}

	vg := paths.VgIdToName(brick.VgId)
	lv := fmt.Sprintf("%v/%v", vg, brick.LvName)
	tp := fmt.Sprintf("%v/%v", vg, brick.TpName)

	if err := s.deleteBrickLV(host, lv); err != nil {
		if errIsLvNotFound(err) {
			logger.Warning("did not delete missing lv: %v", lv)
		} else {
			return spaceReclaimed, err
		}
	}

	thin_count, err := s.countThinLVsInPool(host, tp)
	if err != nil {
		if errIsLvNotFound(err) {
			logger.Warning("unable to count lvs in missing thin pool: %v", tp)
			// if the thin pool is gone it can't host lvs
			thin_count = 0
		} else {
			logger.Err(err)
			return spaceReclaimed, fmt.Errorf(
				"Unable to determine number of logical volumes in "+
					"thin pool %v on host %v", tp, host)
		}
	}

	// If there is no brick left in the thin-pool, it can be removed
	if thin_count == 0 {
		commands = []string{
			fmt.Sprintf("lvremove --autobackup=%v -f %v", conv.BoolToYN(s.BackupLVM), tp),
		}
		_, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
		if errIsLvNotFound(err) {
			logger.Warning("did not delete missing thin pool: %v", tp)
			// if the thin pool is gone then the bricks in the db associated
			// with it take up no space
			spaceReclaimed = true
		} else if err != nil {
			logger.Err(err)
		} else {
			spaceReclaimed = true
		}
	}

	// Now cleanup the mount point
	commands = []string{
		fmt.Sprintf("rmdir %v", brick.Path),
	}
	_, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}

	return spaceReclaimed, nil
}

func (s *CmdExecutor) removeBrickFromFstab(
	host string, brick *executors.BrickRequest) error {

	// If the brick.Path contains "(/var)?/run/gluster/", there is no entry in fstab as GlusterD manages it.
	if strings.HasPrefix(brick.Path, "/run/gluster/") || strings.HasPrefix(brick.Path, "/var/run/gluster/") {
		return nil
	}
	commands := []string{
		fmt.Sprintf("sed -i.save \"/%v/d\" %v",
			paths.BrickIdToName(brick.Name),
			s.Fstab),
	}
	_, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 5)
	if err != nil {
		logger.Err(err)
	}
	return err
}

func errIsLvNotFound(err error) bool {
	if err == nil {
		return false
	}
	e := strings.ToLower(err.Error())
	return (strings.Contains(e, "not found") ||
		strings.Contains(e, "failed to find"))
}
