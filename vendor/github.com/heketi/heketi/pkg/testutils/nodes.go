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
	"os"
	"testing"

	"github.com/heketi/heketi/pkg/logging"
	"github.com/heketi/heketi/pkg/remoteexec/ssh"
)

var (
	DefaultSshUser    = "vagrant"
	DefaultSshKeyFile = "../config/insecure_private_key"
)

type sshAccess struct {
	User    string
	KeyFile string
}

// Use returns a new SSH executor given a logger.
func (a *sshAccess) Use(l *logging.Logger) *ssh.SshExec {
	return ssh.NewSshExecWithKeyFile(l, a.User, a.KeyFile)
}

// RequireNodeAccess returns an access helper depending on
// the test environment. If ssh access is not available or
// remote access is disabled the function triggers a test skip.
func RequireNodeAccess(t *testing.T) *sshAccess {
	// t.Helper()
	// TODO: Helper not available prior to Go1.9, enable once Go1.8 is dropped
	v := os.Getenv("HEKETI_TEST_NODE_ACCESS")
	switch v {
	// default/traditional behavior
	case "":
		return &sshAccess{DefaultSshUser, DefaultSshKeyFile}
	case "skip":
		// setting HEKETI_TEST_NODE_ACCESS to no puts
		// a blanket skip on all tests that want node access
		t.Skipf("remote node access disabled (HEKETI_TEST_NODE_ACCESS=%v)", v)
	default:
		t.Skipf("remote node access unknown (HEKETI_TEST_NODE_ACCESS=%v)", v)
	}
	return nil
}
