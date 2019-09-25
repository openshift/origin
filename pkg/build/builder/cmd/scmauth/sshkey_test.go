package scmauth

import (
	"io/ioutil"
	"os"
	"strings"
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
	fileName, isSet := context.Get("GIT_SSH")
	if !isSet {
		t.Errorf("GIT_SSH is not set")
	}
	buf, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Errorf("problem reading ssh file %s", err.Error())
	}
	str := string(buf)
	if !strings.Contains(str, "StrictHostKeyChecking") {
		t.Errorf("ssh script had wrong contents %s", str)
	}
}

func TestSSHPrivateKeyWithKnownHostsSetup(t *testing.T) {
	context := NewDefaultSCMContext()
	sshKey := &SSHPrivateKey{}
	secretDir := secretDir(t, "ssh-privatekey", "known_hosts")
	defer os.RemoveAll(secretDir)

	err := sshKey.Setup(secretDir, context)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	fileName, isSet := context.Get("GIT_SSH")
	if !isSet {
		t.Errorf("GIT_SSH is not set")
	}
	buf, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Errorf("problem reading ssh file %s", err.Error())
	}
	str := string(buf)
	if !strings.Contains(str, "UserKnownHostsFile") {
		t.Errorf("ssh script had wrong contents %s", str)
	}
}
