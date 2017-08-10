package test

import (
	"github.com/openshift/source-to-image/pkg/api"
)

// FakeInstaller provides a fake installer
type FakeInstaller struct {
	Scripts [][]string
	DstDir  []string
	Error   error
}

func (f *FakeInstaller) run(scripts []string, dstDir string) []api.InstallResult {
	result := []api.InstallResult{}
	f.Scripts = append(f.Scripts, scripts)
	f.DstDir = append(f.DstDir, dstDir)
	return result
}

// InstallRequired downloads and installs required scripts into dstDir
func (f *FakeInstaller) InstallRequired(scripts []string, dstDir string) ([]api.InstallResult, error) {
	return f.run(scripts, dstDir), f.Error
}

// InstallOptional downloads and installs optional scripts into dstDir
func (f *FakeInstaller) InstallOptional(scripts []string, dstDir string) []api.InstallResult {
	return f.run(scripts, dstDir)
}
