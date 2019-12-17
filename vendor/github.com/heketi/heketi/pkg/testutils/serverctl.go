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

package testutils

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"syscall"
	"time"
)

// ServerCfg supports the exposed configuration parameters of
// the ServerCtl type. These may be explicitly set up in code
// or, more commonly, drawn from the environment.
type ServerCfg struct {
	ServerDir string
	HeketiBin string
	LogPath   string
	ConfPath  string
	DbPath    string
	KeepDB    bool
	// disable auth in the server
	DisableAuth bool
	// HelloPort is _only_ to test if the server is running.
	// It does not control the port the server listens to.
	HelloPort string
}

// ServerCtl allows (test) code to manage the heketi server by
// running the server binary. It also supports running the binary
// command line in a non-server style to support commands such
// as 'heketi db export'.
type ServerCtl struct {
	ServerCfg

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

// NewServerCfgFromEnv returns a ServerCfg with values populated
// from environment vars or common defaults if the environment
// vars are unset.
func NewServerCfgFromEnv(dirDefault string) *ServerCfg {
	return &ServerCfg{
		ServerDir: getEnvValue("HEKETI_SERVER_DIR", dirDefault),
		HeketiBin: getEnvValue("HEKETI_SERVER", "./heketi-server"),
		LogPath:   getEnvValue("HEKETI_LOG", ""),
		DbPath:    getEnvValue("HEKETI_DB_PATH", "./heketi.db"),
		ConfPath:  getEnvValue("HEKETI_CONF_PATH", "heketi.json"),
		HelloPort: getEnvValue("HEKETI_HELLO_PORT", "8080"),
		// defaulting DisableAuth to true for now to match
		// historical behavior of our functional tests
		DisableAuth: true,
	}
}

// NewServerCtlFromEnv returns a ServerCtl with the configuration
// parameters filled in based on the environment (see NewServerCfgFromEnv).
func NewServerCtlFromEnv(dirDefault string) *ServerCtl {
	return NewServerCtl(NewServerCfgFromEnv(dirDefault))
}

// NewServerCtl returns a ServerCtl based on the provided configuration.
func NewServerCtl(cfg *ServerCfg) *ServerCtl {
	return &ServerCtl{ServerCfg: *cfg}
}

func (s *ServerCtl) openLog() error {
	if s.LogPath == "" {
		s.logF = nil
	} else {
		f, err := os.OpenFile(s.LogPath, os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return err
		}
		s.logF = f
	}
	return nil
}

func (s *ServerCtl) closeLog() error {
	if s.logF != nil {
		return s.logF.Close()
	}
	return nil
}

func (s *ServerCtl) run(c *exec.Cmd, stdout, stderr *os.File) error {
	s.cmd = c
	s.cmd.Dir = s.ServerDir
	if stdout == nil {
		stdout = s.logF
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = s.logF
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	s.cmd.Stdout = stdout
	s.cmd.Stderr = stderr
	return s.cmd.Start()
}

// ConfigArg returns the common command line argument used to locate
// the configuration file for the heketi server.
func (s *ServerCtl) ConfigArg() string {
	return fmt.Sprintf("--config=%v", s.ConfPath)
}

// ServerArgs returns a string slice of all the arguments to be
// passed to the heketi binary for server start up.
func (s *ServerCtl) ServerArgs() []string {
	args := []string{
		s.ConfigArg(),
	}
	if s.DisableAuth {
		args = append(args, "--disable-auth")
	}
	return args
}

// Start will start a new instance of the heketi server.
func (s *ServerCtl) Start() error {
	if !s.KeepDB {
		// do not preserve the heketi db between server instances
		os.Remove(path.Join(s.ServerDir, s.DbPath))
	}
	if err := s.openLog(); err != nil {
		return err
	}
	c := exec.Command(s.HeketiBin, s.ServerArgs()...)
	if err := s.run(c, nil, nil); err != nil {
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
	return nil
}

// RunOfflineCmd will run the server binary and wait for it to complete.
// Output and error are sent to stdout and stderr.
func (s *ServerCtl) RunOfflineCmd(args []string) error {
	return s.CaptureOfflineCmd(args, nil, nil)
}

// CaptureOfflineCmd will run the server binary and wait for it to complete.
// Output and error are sent to the given file arguments if non-nil.
func (s *ServerCtl) CaptureOfflineCmd(args []string, stdout, stderr *os.File) error {
	if s.IsAlive() {
		return errors.New("may not run offline commands when server is running")
	}
	cmd := append([]string{}, args...)
	if err := s.openLog(); err != nil {
		return err
	}
	defer s.closeLog()
	c := exec.Command(s.HeketiBin, cmd...)
	if err := s.run(c, stdout, stderr); err != nil {
		return err
	}
	s.cmdErr = s.cmd.Wait()
	s.cmdExited = true
	return s.cmdErr
}

// IsAlive returns true if the heketi server is running.
func (s *ServerCtl) IsAlive() bool {
	if s.cmd == nil {
		// no s.cmd object so server was never started
		// needed when this function is called prior to Start
		return false
	}
	if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

// IsAvailable returns true if the heketi server is running and
// communicating on the `HelloPort`.
func (s *ServerCtl) IsAvailable() bool {
	if !s.IsAlive() {
		return false
	}
	// TODO: make this host, port, tls, and url aware
	r, err := http.Get(fmt.Sprintf("http://localhost:%s/hello", s.HelloPort))
	if err != nil {
		return false
	}
	return r.StatusCode == http.StatusOK
}

// Stop will stop the heketi server.
func (s *ServerCtl) Stop() error {
	// close the log file fd after stopping heketi (or if stop fails)
	// this is needed in case the process has already died for some reason
	defer s.closeLog()
	if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	if !s.cmdExited {
		if err := s.cmd.Process.Kill(); err != nil {
			return err
		}
	}
	return nil
}

// Tester is an interface that can be expose a minimal
// set of functionally from testing.T.
type Tester interface {
	Fatalf(format string, args ...interface{})
	Helper()
}

// ServerStarted asserts that the server s is in the started
// state regardless of state prior to the call. If
// the server fails to start the function triggers a test
// failure (through the Tester interface).
func ServerStarted(t Tester, s *ServerCtl) {
	t.Helper()
	if s.IsAvailable() {
		return
	}
	if err := s.Start(); err != nil {
		t.Fatalf("heketi server is not started: %v", err)
	}
	if !s.IsAvailable() && !pollState(func() bool { return s.IsAvailable() }) {
		t.Fatalf("heketi server should have been started")
	}
}

// ServerStopped asserts that the server s is in the stopped
// state regardless of state prior to the call. If
// the server fails to stop the function triggers a test
// failure (through the Tester interface).
func ServerStopped(t Tester, s *ServerCtl) {
	t.Helper()
	if !s.IsAlive() {
		return
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("heketi server is not stopped: %v", err)
	}
	if s.IsAlive() && !pollState(func() bool { return !s.IsAlive() }) {
		t.Fatalf("heketi server should have been stopped")
	}
}

// ServerRestarted asserts that the server is started but
// that any existing instance is first stopped. If any
// steps fails the function triggers a test failure
// (through the TestSuite interface).
func ServerRestarted(t Tester, s *ServerCtl) {
	t.Helper()
	ServerStopped(t, s)
	ServerStarted(t, s)
}

// pollState runs a check function every 0.1 second until
// that function returns true (and pollState will return true)
// or it exhausts the retries and returns false.
func pollState(f func() bool) bool {
	for i := 0; i < 100; i++ {
		time.Sleep(100 * time.Millisecond)
		if f() {
			return true
		}
	}
	return false
}
