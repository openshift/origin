//
// Copyright (c) 2014 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package ssh

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/heketi/heketi/pkg/logging"
	rex "github.com/heketi/heketi/pkg/remoteexec"
)

type SshExec struct {
	clientConfig *ssh.ClientConfig
	logger       *logging.Logger
}

func getKeyFile(file string) (key ssh.Signer, err error) {
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	key, err = ssh.ParsePrivateKey(buf)
	if err != nil {
		return
	}
	return
}

func NewSshExecWithAuth(logger *logging.Logger, user string) *SshExec {

	sshexec := &SshExec{}
	sshexec.logger = logger

	authSocket := os.Getenv("SSH_AUTH_SOCK")
	if authSocket == "" {
		log.Fatal("SSH_AUTH_SOCK required, check that your ssh agent is running")
		return nil
	}

	agentUnixSock, err := net.Dial("unix", authSocket)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	agent := agent.NewClient(agentUnixSock)
	signers, err := agent.Signers()
	if err != nil {
		log.Fatal(err)
		return nil
	}

	sshexec.clientConfig = &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signers...)},
	}

	return sshexec
}

func NewSshExecWithKeyFile(logger *logging.Logger, user string, file string) *SshExec {

	var key ssh.Signer
	var err error

	sshexec := &SshExec{}
	sshexec.logger = logger

	// Now in the main function DO:
	if key, err = getKeyFile(file); err != nil {
		logger.LogError("Unable to get keyfile: %v", err)
		return nil
	}
	// Define the Client Config as :
	sshexec.clientConfig = &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}

	return sshexec
}

// This function requires the password string to be crypt encrypted
func NewSshExecWithPassword(logger *logging.Logger, user string, password string) *SshExec {

	sshexec := &SshExec{}
	sshexec.logger = logger

	// Define the Client Config as :
	sshexec.clientConfig = &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.Password(password)},
	}

	return sshexec
}

// This function was based from https://github.com/coreos/etcd-manager/blob/master/main.go
func (s *SshExec) ConnectAndExec(host string, commands []string, timeoutMinutes int, useSudo bool) ([]string, error) {

	results, err := s.ExecCommands(host, commands, timeoutMinutes, useSudo)
	if err != nil {
		return nil, err
	}
	return results.SquashErrors()
}

func (s *SshExec) ExecCommands(
	host string, commands []string,
	timeoutMinutes int, useSudo bool) (rex.Results, error) {

	results := make(rex.Results, len(commands))

	// :TODO: Will need a timeout here in case the server does not respond
	client, err := ssh.Dial("tcp", host, s.clientConfig)
	if err != nil {
		s.logger.Warning("Failed to create SSH connection to %v: %v", host, err)
		return nil, err
	}
	defer client.Close()

	// Execute each command
	for index, command := range commands {

		session, err := client.NewSession()
		if err != nil {
			s.logger.LogError("Unable to create SSH session: %v", err)
			return nil, err
		}
		defer session.Close()

		// Create a buffer to trap session output
		var b bytes.Buffer
		var berr bytes.Buffer
		session.Stdout = &b
		session.Stderr = &berr

		if useSudo {
			command = "sudo " + command
		}
		// Execute command in a shell
		command = "/bin/bash -c '" + command + "'"

		// Execute command
		err = session.Start(command)
		if err != nil {
			return nil, err
		}

		// Spawn function to wait for results
		errch := make(chan error)
		go func() {
			errch <- session.Wait()
		}()

		// Set the timeout
		timeout := time.After(time.Minute * time.Duration(timeoutMinutes))

		// Wait for either the command completion or timeout
		select {
		case err := <-errch:
			r := rex.Result{
				Completed: true,
				Output:    b.String(),
				ErrOutput: berr.String(),
				Err:       err,
			}
			if err == nil {
				s.logger.Debug(
					"Ran command [%v] on %v: Stdout [%v]: Stderr [%v]",
					command, host, r.Output, r.ErrOutput)
			} else {
				s.logger.LogError(
					"Failed to run command [%v] on %v: Err[%v]: Stdout [%v]: Stderr [%v]",
					command, host, err, r.Output, r.ErrOutput)
				// extract the real error code if possible
				if ee, ok := err.(*ssh.ExitError); ok {
					r.ExitStatus = ee.ExitStatus()
				} else {
					r.ExitStatus = 1
				}
			}
			results[index] = r
			if r.ExitStatus != 0 {
				// stop running commands on error
				// TODO: make caller configurable?)
				return results, nil
			}

		case <-timeout:
			s.logger.LogError("Timeout on command [%v] on %v: Err[%v]: Stdout [%v]: Stderr [%v]",
				command, host, err, b.String(), berr.String())
			err := session.Signal(ssh.SIGKILL)
			if err != nil {
				s.logger.LogError("Unable to send kill signal to command [%v] on host [%v]: %v",
					command, host, err)
			}
			return results, errors.New("SSH command timeout")
		}
	}

	return results, nil
}
