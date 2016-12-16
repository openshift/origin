package git

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/util"
)

// Clone knows how to clone a Git repository.
type Clone struct {
	Git
	util.FileSystem
}

// Download downloads the application source code from the Git repository
// and checkout the Ref specified in the config.
func (c *Clone) Download(config *api.Config) (*api.SourceInfo, error) {
	targetSourceDir := filepath.Join(config.WorkingDir, api.Source)
	config.WorkingSourceDir = targetSourceDir

	ok, err := c.ValidCloneSpec(config.Source)
	if err != nil {
		return nil, err
	}
	if !ok {
		glog.Errorf("Clone.Download was passed an invalid source %s", config.Source)
		return nil, fmt.Errorf("invalid source %s", config.Source)
	}

	ref := "HEAD"
	if config.Ref != "" {
		ref = config.Ref
	}

	if strings.HasPrefix(config.Source, "file://") {
		s := strings.TrimPrefix(config.Source, "file://")

		if util.UsingCygwinGit {
			var err error
			s, err = util.ToSlashCygwin(s)
			if err != nil {
				glog.V(0).Infof("error: Cygwin path conversion failed: %v", err)
				return nil, err
			}
		}
		config.Source = "file://" + s
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

	cloneConfig := api.CloneConfig{Quiet: true}
	err = c.Clone(config.Source, targetSourceDir, cloneConfig)
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
