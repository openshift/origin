package file

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/util"
)

// File represents a simplest possible Downloader implementation where the
// sources are just copied from local directory.
type File struct {
	util.FileSystem
}

func (f *File) Download(config *api.Config) (*api.SourceInfo, error) {
	targetSourceDir := filepath.Join(config.WorkingDir, api.Source)
	if !strings.HasPrefix(config.Source, "file://") {
		return nil, fmt.Errorf("File downloader can be used only for file:// protocol")
	}
	sourceDir := strings.TrimPrefix(config.Source, "file://")
	config.WorkingSourceDir = targetSourceDir

	if len(config.ContextDir) > 0 {
		targetSourceDir = filepath.Join(targetSourceDir, config.ContextDir, ".")
	}

	glog.V(1).Infof("Copying sources from %q to %q", sourceDir, targetSourceDir)
	err := f.Copy(sourceDir, targetSourceDir)
	if err != nil {
		return nil, err
	}

	return &api.SourceInfo{
		Location:   sourceDir,
		ContextDir: config.ContextDir,
	}, nil
}
