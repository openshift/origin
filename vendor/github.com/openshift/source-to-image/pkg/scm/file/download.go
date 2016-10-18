package file

import (
	"path/filepath"
	"strings"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/util"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"
)

var glog = utilglog.StderrLog

// File represents a simplest possible Downloader implementation where the
// sources are just copied from local directory.
type File struct {
	util.FileSystem
}

// Download copies sources from a local directory into the working directory
func (f *File) Download(config *api.Config) (*api.SourceInfo, error) {
	targetSourceDir := filepath.Join(config.WorkingDir, api.Source)
	sourceDir := strings.TrimPrefix(config.Source, "file://")
	config.WorkingSourceDir = targetSourceDir

	if len(config.ContextDir) > 0 {
		targetSourceDir = filepath.Join(config.WorkingDir, api.ContextTmp)
	}

	glog.V(1).Infof("Copying sources from %q to %q", sourceDir, targetSourceDir)
	err := f.CopyContents(sourceDir, targetSourceDir)
	if err != nil {
		return nil, err
	}

	if len(config.ContextDir) > 0 {
		originalTargetDir := filepath.Join(config.WorkingDir, api.Source)
		f.RemoveDirectory(originalTargetDir)
		// we want to copy entire dir contents, thus we need to use dir/. construct
		path := filepath.Join(targetSourceDir, config.ContextDir) + string(filepath.Separator) + "."
		err := f.Copy(path, originalTargetDir)
		if err != nil {
			return nil, err
		}
		f.RemoveDirectory(targetSourceDir)
	}

	return &api.SourceInfo{
		Location:   sourceDir,
		ContextDir: config.ContextDir,
	}, nil
}
