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
	config.WorkingSourceDir = filepath.Join(config.WorkingDir, api.Source)
	source := strings.TrimPrefix(config.Source, "file://")

	copySrc := source
	if len(config.ContextDir) > 0 {
		copySrc = filepath.Join(source, config.ContextDir)
	}

	glog.V(1).Infof("Copying sources from %q to %q", copySrc, config.WorkingSourceDir)
	if copySrc != config.WorkingSourceDir {
		err := f.CopyContents(copySrc, config.WorkingSourceDir)
		if err != nil {
			return nil, err
		}
	}

	return &api.SourceInfo{
		Location:   source,
		ContextDir: config.ContextDir,
	}, nil
}
