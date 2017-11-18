package git

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/util/fs"
)

// Clone knows how to clone a Git repository.
type Clone struct {
	git.Git
	fs.FileSystem
}

// Download downloads the application source code from the Git repository
// and checkout the Ref specified in the config.
func (c *Clone) Download(config *api.Config) (*git.SourceInfo, error) {
	targetSourceDir := filepath.Join(config.WorkingDir, api.Source)
	config.WorkingSourceDir = targetSourceDir

	ref := config.Source.URL.Fragment
	if ref == "" {
		ref = "HEAD"
	}

	if len(config.ContextDir) > 0 {
		targetSourceDir = filepath.Join(config.WorkingDir, api.ContextTmp)
		glog.V(1).Infof("Downloading %q (%q) ...", config.Source, config.ContextDir)
	} else {
		glog.V(1).Infof("Downloading %q ...", config.Source)
	}

	if !config.IgnoreSubmodules {
		glog.V(2).Infof("Cloning sources into %q", targetSourceDir)
	} else {
		glog.V(2).Infof("Cloning sources (ignoring submodules) into %q", targetSourceDir)
	}

	cloneConfig := git.CloneConfig{Quiet: true}
	err := c.Clone(config.Source, targetSourceDir, cloneConfig)
	if err != nil {
		glog.V(0).Infof("error: git clone failed: %v", err)
		return nil, err
	}

	err = c.Checkout(targetSourceDir, ref)
	if err != nil {
		return nil, err
	}
	glog.V(1).Infof("Checked out %q", ref)
	if !config.IgnoreSubmodules {
		err = c.SubmoduleUpdate(targetSourceDir, true, true)
		if err != nil {
			return nil, err
		}
		glog.V(1).Infof("Updated submodules for %q", ref)
	}

	// Record Git's knowledge about file permissions
	if runtime.GOOS == "windows" {
		filemodes, err := c.LsTree(filepath.Join(targetSourceDir, config.ContextDir), ref, true)
		if err != nil {
			return nil, err
		}
		for _, filemode := range filemodes {
			c.Chmod(filepath.Join(targetSourceDir, config.ContextDir, filemode.Name()), os.FileMode(filemode.Mode())&os.ModePerm)
		}
	}

	info := c.GetInfo(targetSourceDir)
	if len(config.ContextDir) > 0 {
		originalTargetDir := filepath.Join(config.WorkingDir, api.Source)
		c.RemoveDirectory(originalTargetDir)
		path := filepath.Join(targetSourceDir, config.ContextDir)
		err := c.CopyContents(path, originalTargetDir)
		if err != nil {
			return nil, err
		}
		c.RemoveDirectory(targetSourceDir)
	}

	if len(config.ContextDir) > 0 {
		info.ContextDir = config.ContextDir
	}

	return info, nil
}
