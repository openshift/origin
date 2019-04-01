//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package sshexec

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/lpabon/godbc"

	"github.com/heketi/heketi/executors/cmdexec"
	"github.com/heketi/heketi/pkg/logging"
	rex "github.com/heketi/heketi/pkg/remoteexec"
	"github.com/heketi/heketi/pkg/remoteexec/ssh"
)

type Ssher interface {
	ConnectAndExec(host string, commands []string, timeoutMinutes int, useSudo bool) ([]string, error)
	ExecCommands(host string, commands []string, timeoutMinutes int, useSudo bool) (rex.Results, error)
}

type SshExecutor struct {
	cmdexec.CmdExecutor

	// Private
	private_keyfile string
	user            string
	exec            Ssher
	config          *SshConfig
	port            string
}

var (
	ErrSshPrivateKey = errors.New("Unable to read private key file")
	sshNew           = func(logger *logging.Logger, user string, file string) (Ssher, error) {
		s := ssh.NewSshExecWithKeyFile(logger, user, file)
		if s == nil {
			return nil, ErrSshPrivateKey
		}
		return s, nil
	}
)

func setWithEnvVariables(config *SshConfig) {
	var env string

	env = os.Getenv("HEKETI_SSH_KEYFILE")
	if "" != env {
		config.PrivateKeyFile = env
	}

	env = os.Getenv("HEKETI_SSH_USER")
	if "" != env {
		config.User = env
	}

	env = os.Getenv("HEKETI_SSH_PORT")
	if "" != env {
		config.Port = env
	}

	env = os.Getenv("HEKETI_FSTAB")
	if "" != env {
		config.Fstab = env
	}

	env = os.Getenv("HEKETI_SNAPSHOT_LIMIT")
	if "" != env {
		i, err := strconv.Atoi(env)
		if err == nil {
			config.SnapShotLimit = i
		}
	}

}

func NewSshExecutor(config *SshConfig) (*SshExecutor, error) {
	// Override configuration
	setWithEnvVariables(config)

	s := &SshExecutor{}
	s.RemoteExecutor = s
	s.Throttlemap = make(map[string]chan bool)

	// Set configuration
	if config.PrivateKeyFile == "" {
		return nil, fmt.Errorf("Missing ssh private key file in configuration")
	}
	s.private_keyfile = config.PrivateKeyFile

	if config.User == "" {
		s.user = "heketi"
	} else {
		s.user = config.User
	}

	if config.Port == "" {
		s.port = "22"
	} else {
		s.port = config.Port
	}

	if config.Fstab == "" {
		s.Fstab = "/etc/fstab"
	} else {
		s.Fstab = config.Fstab
	}

	s.BackupLVM = config.BackupLVM

	// Save the configuration
	s.config = config

	// Setup key
	var err error
	s.exec, err = sshNew(s.Logger(), s.user, s.private_keyfile)
	if err != nil {
		s.Logger().Err(err)
		return nil, err
	}

	godbc.Ensure(s != nil)
	godbc.Ensure(s.config == config)
	godbc.Ensure(s.user != "")
	godbc.Ensure(s.private_keyfile != "")
	godbc.Ensure(s.port != "")
	godbc.Ensure(s.Fstab != "")

	return s, nil
}

func (s *SshExecutor) RemoteCommandExecute(host string,
	commands []string,
	timeoutMinutes int) ([]string, error) {

	// Throttle
	s.AccessConnection(host)
	defer s.FreeConnection(host)

	// Execute
	return s.exec.ConnectAndExec(host+":"+s.port, commands, timeoutMinutes, s.config.Sudo)
}

func (s *SshExecutor) ExecCommands(
	host string, commands []string, timeoutMinutes int) (rex.Results, error) {

	// Throttle
	s.AccessConnection(host)
	defer s.FreeConnection(host)

	// Execute
	return s.exec.ExecCommands(host+":"+s.port, commands, timeoutMinutes, s.config.Sudo)
}

func (s *SshExecutor) RebalanceOnExpansion() bool {
	return s.config.RebalanceOnExpansion
}

func (s *SshExecutor) SnapShotLimit() int {
	return s.config.SnapShotLimit
}
