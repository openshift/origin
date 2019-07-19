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
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/idgen"
	"github.com/lpabon/godbc"
)

func (s *CmdExecutor) VolumeCreate(host string,
	volume *executors.VolumeRequest) (*executors.Volume, error) {

	godbc.Require(volume != nil)
	godbc.Require(host != "")
	godbc.Require(len(volume.Bricks) > 0)
	godbc.Require(volume.Name != "")

	cmd := fmt.Sprintf("gluster --mode=script volume create %v ", volume.Name)

	var (
		inSet     int
		maxPerSet int
	)
	switch volume.Type {
	case executors.DurabilityNone:
		logger.Info("Creating volume %v with no durability", volume.Name)
		inSet = 1
		maxPerSet = 15
	case executors.DurabilityReplica:
		logger.Info("Creating volume %v replica %v", volume.Name, volume.Replica)
		cmd += fmt.Sprintf("replica %v ", volume.Replica)
		if volume.Arbiter {
			cmd += "arbiter 1 "
		}
		inSet = volume.Replica
		maxPerSet = 5
	case executors.DurabilityDispersion:
		logger.Info("Creating volume %v dispersion %v+%v",
			volume.Name, volume.Data, volume.Redundancy)
		cmd += fmt.Sprintf("disperse-data %v redundancy %v ", volume.Data, volume.Redundancy)
		inSet = volume.Data + volume.Redundancy
		maxPerSet = 1
	}

	// There could many, many bricks, which could render a single command
	// line that creates the volume with all the bricks too long.
	// Therefore, we initially create the volume with the first brick set
	// only, and then add each brick set in one subsequent command.

	for _, brick := range volume.Bricks[:inSet] {
		cmd += fmt.Sprintf("%v:%v ", brick.Host, brick.Path)
	}

	commands := []string{cmd}

	commands = append(commands, s.createAddBrickCommands(volume, inSet, inSet, maxPerSet)...)

	commands = append(commands, s.createVolumeOptionsCommand(volume)...)

	commands = append(commands, fmt.Sprintf("gluster --mode=script volume start %v", volume.Name))

	_, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		s.VolumeDestroy(host, volume.Name)
		return nil, err
	}

	return &executors.Volume{}, nil
}

func (s *CmdExecutor) VolumeExpand(host string,
	volume *executors.VolumeRequest) (*executors.Volume, error) {

	godbc.Require(volume != nil)
	godbc.Require(host != "")
	godbc.Require(len(volume.Bricks) > 0)
	godbc.Require(volume.Name != "")

	var (
		inSet     int
		maxPerSet int
	)
	switch volume.Type {
	case executors.DurabilityNone:
		inSet = 1
		maxPerSet = 15
	case executors.DurabilityReplica:
		inSet = volume.Replica
		maxPerSet = 5
	case executors.DurabilityDispersion:
		inSet = volume.Data + volume.Redundancy
		maxPerSet = 1
	}

	commands := s.createAddBrickCommands(volume,
		0, // start at the beginning of the brick list
		inSet,
		maxPerSet)
	_, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		return nil, err
	}

	if s.RemoteExecutor.RebalanceOnExpansion() {
		commands = []string{fmt.Sprintf("gluster --mode=script volume rebalance %v start", volume.Name)}
		_, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
		if err != nil {
			// This is a hack. We fake success if rebalance fails.
			// Mainly because rebalance may fail even if one brick is down for the given volume.
			// The probability is just too high to undo the work done to create and attach bricks.
			// Admins should be able to get new size to reflect by executing the rebalance cmd manually.
			logger.LogError("Unable to start rebalance on the volume %v: %v", volume, err)
			logger.LogError("Action Required: run rebalance manually on the volume %v", volume)
			return &executors.Volume{}, nil
		}
	}

	return &executors.Volume{}, nil
}

func (s *CmdExecutor) VolumeDestroy(host string, volume string) error {
	godbc.Require(host != "")
	godbc.Require(volume != "")

	// First stop the volume, then delete it

	commands := []string{
		fmt.Sprintf("gluster --mode=script volume stop %v force", volume),
	}

	_, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		logger.LogError("Unable to stop volume %v: %v", volume, err)
	}

	commands = []string{
		fmt.Sprintf("gluster --mode=script volume delete %v", volume),
	}

	_, err = s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		return logger.Err(fmt.Errorf("Unable to delete volume %v: %v", volume, err))
	}

	return nil
}

func (s *CmdExecutor) VolumeDestroyCheck(host, volume string) error {
	godbc.Require(host != "")
	godbc.Require(volume != "")

	// Determine if the volume is able to be deleted
	return s.checkForSnapshots(host, volume)
}

func (s *CmdExecutor) createVolumeOptionsCommand(volume *executors.VolumeRequest) []string {
	commands := []string{}
	var cmd string

	// Go through all the Options and create volume set command
	for _, volOption := range volume.GlusterVolumeOptions {
		if volOption != "" {
			cmd = fmt.Sprintf("gluster --mode=script volume set %v %v", volume.Name, volOption)
			commands = append(commands, cmd)
		}

	}
	return commands
}

func (s *CmdExecutor) createAddBrickCommands(volume *executors.VolumeRequest,
	start, inSet, maxPerSet int) []string {

	commands := []string{}
	var cmd string

	// Go through all the bricks and create add-brick commands
	for index, brick := range volume.Bricks[start:] {
		if index%(inSet*maxPerSet) == 0 {
			if cmd != "" {
				// Add add-brick command to the command list
				commands = append(commands, cmd)
			}

			// Create a new add-brick command
			cmd = fmt.Sprintf("gluster --mode=script volume add-brick %v ", volume.Name)
		}

		// Add this brick to the add-brick command
		cmd += fmt.Sprintf("%v:%v ", brick.Host, brick.Path)
	}

	// Add the last add-brick command to the command list
	if cmd != "" {
		commands = append(commands, cmd)
	}

	return commands
}

func (s *CmdExecutor) checkForSnapshots(host, volume string) error {

	// Structure used to unmarshal XML from snapshot gluster cli
	type CliOutput struct {
		OpRet    int    `xml:"opRet"`
		OpErrno  int    `xml:"opErrno"`
		OpErrStr string `xml:"opErrstr"`
		SnapList struct {
			Count int `xml:"count"`
		} `xml:"snapList"`
	}

	commands := []string{
		fmt.Sprintf("gluster --mode=script snapshot list %v --xml", volume),
	}

	output, err := s.RemoteExecutor.RemoteCommandExecute(host, commands, 10)
	if err != nil {
		return fmt.Errorf("Unable to get snapshot information from volume %v: %v", volume, err)
	}

	var snapInfo CliOutput
	err = xml.Unmarshal([]byte(output[0]), &snapInfo)
	if err != nil {
		return fmt.Errorf("Unable to determine snapshot information from volume %v: %v", volume, err)
	}

	if strings.Contains(snapInfo.OpErrStr, "does not exist") &&
		strings.Contains(snapInfo.OpErrStr, volume) {
		return &executors.VolumeDoesNotExistErr{Name: volume}
	}

	if snapInfo.SnapList.Count > 0 {
		return fmt.Errorf("Unable to delete volume %v because it contains %v snapshots",
			volume, snapInfo.SnapList.Count)
	}

	return nil
}

func (s *CmdExecutor) VolumeInfo(host string, volume string) (*executors.Volume, error) {

	godbc.Require(volume != "")
	godbc.Require(host != "")

	type CliOutput struct {
		OpRet    int               `xml:"opRet"`
		OpErrno  int               `xml:"opErrno"`
		OpErrStr string            `xml:"opErrstr"`
		VolInfo  executors.VolInfo `xml:"volInfo"`
	}

	command := []string{
		fmt.Sprintf("gluster --mode=script volume info %v --xml", volume),
	}

	//Get the xml output of volume info
	output, err := s.RemoteExecutor.RemoteCommandExecute(host, command, 10)
	if err != nil {
		return nil, fmt.Errorf("Unable to get volume info of volume name: %v", volume)
	}
	var volumeInfo CliOutput
	err = xml.Unmarshal([]byte(output[0]), &volumeInfo)
	if err != nil {
		return nil, fmt.Errorf("Unable to determine volume info of volume name: %v", volume)
	}
	logger.Debug("%+v\n", volumeInfo)
	return &volumeInfo.VolInfo.Volumes.VolumeList[0], nil
}

func (s *CmdExecutor) VolumeReplaceBrick(host string, volume string, oldBrick *executors.BrickInfo, newBrick *executors.BrickInfo) error {
	godbc.Require(volume != "")
	godbc.Require(host != "")
	godbc.Require(oldBrick != nil)
	godbc.Require(newBrick != nil)

	// Replace the brick
	command := []string{
		fmt.Sprintf("gluster --mode=script volume replace-brick %v %v:%v %v:%v commit force", volume, oldBrick.Host, oldBrick.Path, newBrick.Host, newBrick.Path),
	}
	_, err := s.RemoteExecutor.RemoteCommandExecute(host, command, 10)
	if err != nil {
		return logger.Err(fmt.Errorf("Unable to replace brick %v:%v with %v:%v for volume %v", oldBrick.Host, oldBrick.Path, newBrick.Host, newBrick.Path, volume))
	}

	return nil

}

func (s *CmdExecutor) VolumeClone(host string, vcr *executors.VolumeCloneRequest) (*executors.Volume, error) {
	godbc.Require(host != "")
	godbc.Require(vcr != nil)

	vsr := executors.VolumeSnapshotRequest{
		Volume:   vcr.Volume,
		Snapshot: "tmpsnap_" + idgen.GenUUID(),
	}

	snap, err := s.VolumeSnapshot(host, &vsr)
	if err != nil {
		return nil, err
	}

	// we do not want activated snapshots sticking around
	defer s.SnapshotDestroy(host, snap.Name)

	scr := executors.SnapshotCloneRequest{
		Snapshot: snap.Name,
		Volume:   vcr.Clone,
	}

	vol, err := s.SnapshotCloneVolume(host, &scr)
	if err != nil {
		return nil, err
	}

	return vol, nil
}

func (s *CmdExecutor) VolumeSnapshot(host string, vsr *executors.VolumeSnapshotRequest) (*executors.Snapshot, error) {
	godbc.Require(host != "")
	godbc.Require(vsr != nil)

	type CliOutput struct {
		OpRet      int                  `xml:"opRet"`
		OpErrno    int                  `xml:"opErrno"`
		OpErrStr   string               `xml:"opErrstr"`
		SnapCreate executors.SnapCreate `xml:"snapCreate"`
	}

	command := []string{
		fmt.Sprintf("gluster --mode=script --xml snapshot create %v %v no-timestamp", vsr.Snapshot, vsr.Volume),
		// TODO: set the snapshot description if vsr.Description is non-empty
	}

	output, err := s.RemoteExecutor.RemoteCommandExecute(host, command, 10)
	if err != nil {
		return nil, fmt.Errorf("Unable to create snapshot of volume %v: %v", vsr.Volume, err)
	}

	var snapCreate CliOutput
	err = xml.Unmarshal([]byte(output[0]), &snapCreate)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse output of creating snapshot of volume %v: %v", vsr.Volume, err)
	}
	logger.Debug("snapCreate: %+v\n", snapCreate)

	if snapCreate.OpRet != 0 {
		return nil, fmt.Errorf("Failed to create snapshot of volume %v: %v", vsr.Volume, snapCreate.OpErrStr)
	}

	snap := &snapCreate.SnapCreate.Snapshot
	logger.Debug("snapshot: %+v\n", snap)

	return snap, nil
}

func (s *CmdExecutor) HealInfo(host string, volume string) (*executors.HealInfo, error) {

	godbc.Require(volume != "")
	godbc.Require(host != "")

	type CliOutput struct {
		OpRet    int                `xml:"opRet"`
		OpErrno  int                `xml:"opErrno"`
		OpErrStr string             `xml:"opErrstr"`
		HealInfo executors.HealInfo `xml:"healInfo"`
	}

	command := []string{
		fmt.Sprintf("gluster --mode=script volume heal %v info --xml", volume),
	}

	output, err := s.RemoteExecutor.RemoteCommandExecute(host, command, 10)
	if err != nil {
		return nil, fmt.Errorf("Unable to get heal info of volume : %v", volume)
	}
	var healInfo CliOutput
	err = xml.Unmarshal([]byte(output[0]), &healInfo)
	if err != nil {
		return nil, fmt.Errorf("Unable to determine heal info of volume : %v", volume)
	}
	logger.Debug("%+v\n", healInfo)
	return &healInfo.HealInfo, nil
}
