//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmdexec

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lpabon/godbc"

	"github.com/heketi/heketi/executors"
	rex "github.com/heketi/heketi/pkg/remoteexec"
)

func (s *CmdExecutor) BlockVolumeCreate(host string,
	volume *executors.BlockVolumeRequest) (*executors.BlockVolumeInfo, error) {

	godbc.Require(volume != nil)
	godbc.Require(host != "")
	godbc.Require(volume.Name != "")

	type CliOutput struct {
		Iqn      string   `json:"IQN"`
		Username string   `json:"USERNAME"`
		Password string   `json:"PASSWORD"`
		Portal   []string `json:"PORTAL(S)"`
		Result   string   `json:"RESULT"`
		ErrCode  int      `json:"errCode"`
		ErrMsg   string   `json:"errMsg"`
	}

	var auth_set string
	if volume.Auth {
		auth_set = "enable"
	} else {
		auth_set = "disable"
	}

	cmd := fmt.Sprintf(
		"gluster-block create %v/%v ha %v auth %v prealloc %v %v %vGiB --json",
		volume.GlusterVolumeName,
		volume.Name,
		volume.Hacount,
		auth_set,
		s.BlockVolumeDefaultPrealloc(),
		strings.Join(volume.BlockHosts, ","),
		volume.Size)

	// Initialize the commands with the create command
	commands := []string{cmd}

	// Execute command
	results, err := s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands), 10)
	if err != nil {
		return nil, err
	}

	output := results[0].Output
	if output == "" {
		output = results[0].ErrOutput
	}

	var blockVolumeCreate CliOutput
	err = json.Unmarshal([]byte(output), &blockVolumeCreate)
	if err != nil {
		logger.Warning("Unable to parse gluster-block output [%v]: %v",
			output, err)
		err = fmt.Errorf(
			"Unparsable error during block volume create: %v",
			output)
	} else if blockVolumeCreate.Result == "FAIL" {
		// the fail flag was set in the output json
		err = fmt.Errorf("Failed to create block volume: %v",
			blockVolumeCreate.ErrMsg)
	} else if !results.Ok() {
		// the fail flag is not set but the command still
		// exited non-zero for some reason
		err = fmt.Errorf("Failed to create block volume: %v",
			results[0].Error())
	}

	// if any of the cases above set err, log it and return
	if err != nil {
		logger.LogError("%v", err)
		return nil, err
	}

	var blockVolumeInfo executors.BlockVolumeInfo

	blockVolumeInfo.BlockHosts = volume.BlockHosts // TODO: split blockVolumeCreate.Portal into here instead of using request data
	blockVolumeInfo.GlusterNode = volume.GlusterNode
	blockVolumeInfo.GlusterVolumeName = volume.GlusterVolumeName
	blockVolumeInfo.Hacount = volume.Hacount
	blockVolumeInfo.Iqn = blockVolumeCreate.Iqn
	blockVolumeInfo.Name = volume.Name
	blockVolumeInfo.Size = volume.Size
	blockVolumeInfo.Username = blockVolumeCreate.Username
	blockVolumeInfo.Password = blockVolumeCreate.Password

	return &blockVolumeInfo, nil
}

func (s *CmdExecutor) BlockVolumeDestroy(host string, blockHostingVolumeName string, blockVolumeName string) error {
	godbc.Require(host != "")
	godbc.Require(blockHostingVolumeName != "")
	godbc.Require(blockVolumeName != "")

	commands := []string{
		fmt.Sprintf("gluster-block delete %v/%v --json",
			blockHostingVolumeName, blockVolumeName),
	}
	res, err := s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands), 10)
	if err != nil {
		// non-command error conditions
		return err
	}

	r := res[0]
	errOutput := r.ErrOutput
	if errOutput == "" {
		errOutput = r.Output
	}
	if errOutput == "" {
		// we ought to have some output but we don't
		return r.Err
	}

	type CliOutput struct {
		Result       string `json:"RESULT"`
		ResultOnHost string `json:"Result"`
		ErrCode      int    `json:"errCode"`
		ErrMsg       string `json:"errMsg"`
	}
	var blockVolumeDelete CliOutput
	if e := json.Unmarshal([]byte(errOutput), &blockVolumeDelete); e != nil {
		logger.LogError("Failed to unmarshal response from block "+
			"volume delete for volume %v", blockVolumeName)
		if r.Err != nil {
			return logger.Err(r.Err)
		}

		return logger.LogError("Unable to parse output from block "+
			"volume delete: %v", e)
	}

	if blockVolumeDelete.Result == "FAIL" {
		errHas := func(s string) bool {
			return strings.Contains(blockVolumeDelete.ErrMsg, s)
		}

		if (errHas("doesn't exist") && errHas(blockVolumeName)) ||
			(errHas("does not exist") && errHas(blockHostingVolumeName)) {
			return &executors.VolumeDoesNotExistErr{Name: blockVolumeName}
		}
		return logger.LogError("%v", blockVolumeDelete.ErrMsg)
	}
	return r.Err
}

func (c *CmdExecutor) ListBlockVolumes(host string, blockhostingvolume string) ([]string, error) {
	godbc.Require(host != "")
	godbc.Require(blockhostingvolume != "")

	commands := []string{fmt.Sprintf("gluster-block list %v --json", blockhostingvolume)}

	results, err := c.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands), 10)
	if err := rex.AnyError(results, err); err != nil {
		logger.Err(err)
		return nil, fmt.Errorf("unable to list blockvolumes on block hosting volume %v : %v", blockhostingvolume, err)
	}

	type BlockVolumeListOutput struct {
		Blocks []string `json:"blocks"`
		RESULT string   `json:"RESULT"`
	}

	var blockVolumeList BlockVolumeListOutput

	err = json.Unmarshal([]byte(results[0].Output), &blockVolumeList)
	if err != nil {
		logger.Err(err)
		return nil, fmt.Errorf("Unable to get the block volume list for block hosting volume %v : %v", blockhostingvolume, err)
	}

	return blockVolumeList.Blocks, nil
}
