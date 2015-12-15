package scmauth

import (
	"os"
	"testing"
)

func TestGitConfigHandles(t *testing.T) {
	caCert := &GitConfig{}
	if !caCert.Handles(".gitconfig") {
		t.Errorf("should handle .gitconfig")
	}
	if caCert.Handles("username") {
		t.Errorf("should not handle username")
	}
	if caCert.Handles("gitconfig") {
		t.Errorf("should not handle gitconfig")
	}
}

func TestGitConfigSetup(t *testing.T) {
	context := NewDefaultSCMContext()
	gitConfig := &GitConfig{}
	secretDir := secretDir(t, ".gitconfig")
	defer os.RemoveAll(secretDir)

	err := gitConfig.Setup(secretDir, context)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	config, _ := context.Get("GIT_CONFIG")
	defer cleanupConfig(config)
	validateConfig(t, config, "test")
}
