package scmauth

import (
	"os"
	"testing"

	"github.com/openshift/source-to-image/pkg/scm/git"
)

func TestCACertHandles(t *testing.T) {
	caCert := &CACert{}
	if !caCert.Handles("ca.crt") {
		t.Errorf("should handle ca.crt")
	}
	if caCert.Handles("username") {
		t.Errorf("should not handle username")
	}
}

func TestCACertSetup(t *testing.T) {
	context := NewDefaultSCMContext()
	caCert := &CACert{
		SourceURL: *git.MustParse("https://my.host/git/repo"),
	}
	secretDir := secretDir(t, "ca.crt")
	defer os.RemoveAll(secretDir)

	err := caCert.Setup(secretDir, context)
	gitConfig, _ := context.Get("GIT_CONFIG")
	defer cleanupConfig(gitConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	validateConfig(t, gitConfig, "sslCAInfo")
}

func TestCACertSetupNoSSL(t *testing.T) {
	context := NewDefaultSCMContext()
	caCert := &CACert{
		SourceURL: *git.MustParse("http://my.host/git/repo"),
	}
	secretDir := secretDir(t, "ca.crt")
	defer os.RemoveAll(secretDir)

	err := caCert.Setup(secretDir, context)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, gitConfigPresent := context.Get("GIT_CONFIG")
	if gitConfigPresent {
		t.Fatalf("git config not expected")
	}
}
