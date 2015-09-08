package git

import (
	"path/filepath"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/util"
)

type Clone struct {
	Git
	util.FileSystem
}

// Download downloads the application source code from the GIT repository
// and checkout the Ref specified in the config.
func (c *Clone) Download(config *api.Config) (*api.SourceInfo, error) {
	targetSourceDir := filepath.Join(config.WorkingDir, api.Source)
	config.WorkingSourceDir = targetSourceDir
	var info *api.SourceInfo

	if c.ValidCloneSpec(config.Source) {
		if len(config.ContextDir) > 0 {
			targetSourceDir = filepath.Join(config.WorkingDir, api.ContextTmp)
		}
		glog.V(2).Infof("Cloning into %s", targetSourceDir)
		if err := c.Clone(config.Source, targetSourceDir); err != nil {
			glog.V(1).Infof("Git clone failed: %+v", err)
			return nil, err
		}

		if len(config.Ref) > 0 {
			if err := c.Checkout(targetSourceDir, config.Ref); err != nil {
				return nil, err
			}
			glog.V(1).Infof("Checked out %q", config.Ref)
		}

		if len(config.ContextDir) > 0 {
			originalTargetDir := filepath.Join(config.WorkingDir, api.Source)
			c.RemoveDirectory(originalTargetDir)
			// we want to copy entire dir contents, thus we need to use dir/. construct
			path := filepath.Join(targetSourceDir, config.ContextDir) + string(filepath.Separator) + "."
			err := c.Copy(path, originalTargetDir)
			if err != nil {
				return nil, err
			}
			info = c.GetInfo(targetSourceDir)
			c.RemoveDirectory(targetSourceDir)
		} else {
			info = c.GetInfo(targetSourceDir)
		}

		if len(config.ContextDir) > 0 {
			info.ContextDir = config.ContextDir
		}

		return info, nil
	}
	// we want to copy entire dir contents, thus we need to use dir/. construct
	path := filepath.Join(config.Source, config.ContextDir) + string(filepath.Separator) + "."
	if !c.Exists(path) {
		return nil, errors.NewSourcePathError(path)
	}
	if err := c.Copy(path, targetSourceDir); err != nil {
		return nil, err
	}

	// When building from a local directory (not using GIT clone spec scheme) we
	// skip gathering informations about the source as there is no guarantee that
	// the folder is a GIT repository or it requires context-dir to be set.
	if !config.Quiet {
		glog.Warning("You are using <source> location that is not valid GIT repository. The source code information will not be stored into the output image. Use this image only for local testing and development.")
	}
	return nil, nil
}
