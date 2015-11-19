package scmauth

import (
	"net/url"
	"path/filepath"
)

const GitConfigName = ".gitconfig"

// GitConfig implements SCMAuth interface for using a custom .gitconfig file
type GitConfig struct{}

// Setup adds the secret .gitconfig as an include to the .gitconfig file to be used in the build
func (_ GitConfig) Setup(baseDir string) (*url.URL, error) {
	return nil, ensureGitConfigIncludes(filepath.Join(baseDir, GitConfigName))
}

// Name returns the name of this auth method.
func (_ GitConfig) Name() string {
	return GitConfigName
}

// Handles returns true if the secret file is a gitconfig
func (_ GitConfig) Handles(name string) bool {
	return name == GitConfigName
}
