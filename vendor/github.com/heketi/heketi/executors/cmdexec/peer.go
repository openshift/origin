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

	"github.com/lpabon/godbc"

	rex "github.com/heketi/heketi/pkg/remoteexec"
)

// :TODO: Rename this function to NodeInit or something
func (s *CmdExecutor) PeerProbe(host, newnode string) error {

	godbc.Require(host != "")
	godbc.Require(newnode != "")

	logger.Info("Probing: %v -> %v", host, newnode)
	// create the commands
	commands := []string{
		fmt.Sprintf("%v peer probe %v", s.glusterCommand(), newnode),
	}
	err := rex.AnyError(s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands),
		s.GlusterCliExecTimeout()))
	if err != nil {
		return err
	}

	// Determine if there is a snapshot limit configuration setting
	if s.RemoteExecutor.SnapShotLimit() > 0 {
		logger.Info("Setting snapshot limit")
		commands = []string{
			fmt.Sprintf("%v snapshot config snap-max-hard-limit %v",
				s.glusterCommand(), s.RemoteExecutor.SnapShotLimit()),
		}
		err := rex.AnyError(s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands),
			s.GlusterCliExecTimeout()))
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *CmdExecutor) PeerDetach(host, detachnode string) error {
	godbc.Require(host != "")
	godbc.Require(detachnode != "")

	// create the commands
	logger.Info("Detaching node %v", detachnode)
	commands := []string{
		fmt.Sprintf("%v peer detach %v", s.glusterCommand(), detachnode),
	}
	err := rex.AnyError(s.RemoteExecutor.ExecCommands(host, rex.ToCmds(commands),
		s.GlusterCliExecTimeout()))
	if err != nil {
		logger.Err(err)
	}

	return nil
}

func (s *CmdExecutor) GlusterdCheck(host string) error {
	godbc.Require(host != "")

	logger.Info("Check Glusterd service status in node %v", host)
	cmd := rex.ToCmd("systemctl status glusterd")
	cmd.Options.Quiet = true
	err := rex.AnyError(s.RemoteExecutor.ExecCommands(host, rex.Cmds{cmd}, 10))
	if err != nil {
		logger.Err(err)
		return err
	}

	return nil
}
