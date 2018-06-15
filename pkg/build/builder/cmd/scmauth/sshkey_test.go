package scmauth

import (
	"os"
	"testing"
)

func TestSSHPrivateKeyHandles(t *testing.T) {
	sshKey := &SSHPrivateKey{}
	if !sshKey.Handles("ssh-privatekey") {
		t.Errorf("should handle ssh-privatekey")
	}
	if sshKey.Handles("ca.crt") {
		t.Errorf("should not handle ca.crt")
	}
}

func TestSSHPrivateKeySetup(t *testing.T) {
	context := NewDefaultSCMContext()
	sshKey := &SSHPrivateKey{}
	secretDir := secretDir(t, "ssh-privatekey")
	defer os.RemoveAll(secretDir)

	err := sshKey.Setup(secretDir, context)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	_, isSet := context.Get("GIT_SSH")
	if !isSet {
		t.Errorf("GIT_SSH is not set")
	}
}
