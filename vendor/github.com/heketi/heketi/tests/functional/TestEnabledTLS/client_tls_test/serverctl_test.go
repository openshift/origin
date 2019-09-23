// +build functional

//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

package client_tls_test

import (
	"errors"
	"os"
	"os/exec"
	"path"
	"syscall"
	"time"
)

type ServerCtl struct {
	serverDir string
	heketiBin string
	logPath   string
	dbPath    string
	keepDB    bool
	// the real stuff
	cmd       *exec.Cmd
	cmdExited bool
	cmdErr    error
	logF      *os.File
}

func getEnvValue(k, val string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return val
}

func NewServerCtlFromEnv(dir string) *ServerCtl {
	return &ServerCtl{
		serverDir: getEnvValue("HEKETI_SERVER_DIR", dir),
		heketiBin: getEnvValue("HEKETI_SERVER", "./heketi-server"),
		logPath:   getEnvValue("HEKETI_LOG", "./heketi.log"),
		dbPath:    getEnvValue("HEKETI_DB_PATH", "./heketi.db"),
	}
}

func (s *ServerCtl) Start() error {
	if !s.keepDB {
		// do not preserve the heketi db between server instances
		os.Remove(path.Join(s.serverDir, s.dbPath))
	}
	f, err := os.OpenFile(s.logPath, os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	s.logF = f
	s.cmd = exec.Command(s.heketiBin, "--config=heketi.json")
	s.cmd.Dir = s.serverDir
	s.cmd.Stdout = f
	s.cmd.Stderr = f
	if err := s.cmd.Start(); err != nil {
		return err
	}
	go func() {
		s.cmdErr = s.cmd.Wait()
		s.cmdExited = true
	}()
	time.Sleep(300 * time.Millisecond)
	if !s.IsAlive() {
		return errors.New("server exited early")
	}
	// dump some logs if heketi fails to start?
	return nil
}

func (s *ServerCtl) IsAlive() bool {
	if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

func (s *ServerCtl) Stop() error {
	if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	if !s.cmdExited {
		if err := s.cmd.Process.Kill(); err != nil {
			return err
		}
	}
	s.logF.Close()
	return nil
}
