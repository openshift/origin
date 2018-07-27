package file

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/constants"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/util/fs"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"
)

var glog = utilglog.StderrLog

// RecursiveCopyError indicates a copy operation failed because the destination is within the copy's source tree.
type RecursiveCopyError struct {
	error
}

// File represents a simplest possible Downloader implementation where the
// sources are just copied from local directory.
type File struct {
	fs.FileSystem
}

// Download copies sources from a local directory into the working directory.
// Caller guarantees that config.Source.IsLocal() is true.
func (f *File) Download(config *api.Config) (*git.SourceInfo, error) {
	config.WorkingSourceDir = filepath.Join(config.WorkingDir, constants.Source)

	copySrc := config.Source.LocalPath()
	if len(config.ContextDir) > 0 {
		copySrc = filepath.Join(copySrc, config.ContextDir)
	}

	glog.V(1).Infof("Copying sources from %q to %q", copySrc, config.WorkingSourceDir)
	if strings.HasPrefix(filepath.Clean(config.WorkingSourceDir), filepath.Clean(copySrc)) {
		return nil, RecursiveCopyError{error: fmt.Errorf("recursive copy requested, source directory %q contains the target directory %q", copySrc, config.WorkingSourceDir)}
	}
	if copySrc != config.WorkingSourceDir {
		f.KeepSymlinks(config.KeepSymlinks)
		err := f.CopyContents(copySrc, config.WorkingSourceDir)
		if err != nil {
			return nil, err
		}
	}

	return &git.SourceInfo{
		Location:   config.Source.LocalPath(),
		ContextDir: config.ContextDir,
	}, nil
}
