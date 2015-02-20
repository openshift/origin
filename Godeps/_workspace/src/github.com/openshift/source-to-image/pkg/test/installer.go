package test

import (
	"errors"
	"github.com/openshift/source-to-image/pkg/api"
)

// FakeInstaller provides a fake installer
type FakeInstaller struct {
	Scripts    [][]api.Script
	WorkingDir []string
	Required   []bool

	Download  bool
	ErrScript api.Script
}

// DownloadAndInstall downloads and install the fake STI scripts
func (f *FakeInstaller) DownloadAndInstall(scripts []api.Script, workingDir string, required bool) (bool, error) {
	f.Scripts = append(f.Scripts, scripts)
	f.WorkingDir = append(f.WorkingDir, workingDir)
	f.Required = append(f.Required, required)
	for _, s := range scripts {
		if f.ErrScript == s {
			return f.Download, errors.New(string(f.ErrScript))
		}
	}
	return f.Download, nil
}
