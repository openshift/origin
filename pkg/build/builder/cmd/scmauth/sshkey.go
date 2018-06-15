package scmauth

import (
	"io/ioutil"
	"path/filepath"

	"github.com/golang/glog"
)

const SSHPrivateKeyMethodName = "ssh-privatekey"

// SSHPrivateKey implements SCMAuth interface for using SSH private keys.
type SSHPrivateKey struct{}

// Setup creates a wrapper script for SSH command to be able to use the provided
// SSH key while accessing private repository.
func (_ SSHPrivateKey) Setup(baseDir string, context SCMAuthContext) error {
	script, err := ioutil.TempFile("", "gitssh")
	if err != nil {
		return err
	}
	defer script.Close()
	if err := script.Chmod(0711); err != nil {
		return err
	}
	content := "#!/bin/sh\nssh -i " +
		filepath.Join(baseDir, SSHPrivateKeyMethodName) +
		" -o StrictHostKeyChecking=false \"$@\"\n"

	glog.V(5).Infof("Adding Private SSH Auth:\n%s\n", content)

	if _, err := script.WriteString(content); err != nil {
		return err
	}
	// set environment variable to tell git to use the SSH wrapper
	if err := context.Set("GIT_SSH", script.Name()); err != nil {
		return err
	}
	return nil
}

// Name returns the name of this auth method.
func (_ SSHPrivateKey) Name() string {
	return SSHPrivateKeyMethodName
}

// Handles returns true if the file is an SSH private key
func (_ SSHPrivateKey) Handles(name string) bool {
	return name == SSHPrivateKeyMethodName
}
