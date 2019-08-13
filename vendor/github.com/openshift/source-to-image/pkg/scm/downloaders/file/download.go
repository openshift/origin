package file

import (
	"fmt"
	"path/filepath"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/constants"
	"github.com/openshift/source-to-image/pkg/ignore"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/util/fs"
	utillog "github.com/openshift/source-to-image/pkg/util/log"
)

var log = utillog.StderrLog

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

	log.V(1).Infof("Copying sources from %q to %q", copySrc, config.WorkingSourceDir)
	absWorkingSourceDir, err := filepath.Abs(config.WorkingSourceDir)
	if err != nil {
		return nil, err
	}
	absCopySrc, err := filepath.Abs(copySrc)
	if err != nil {
		return nil, err
	}
	if filepath.HasPrefix(absWorkingSourceDir, absCopySrc) {
		return nil, RecursiveCopyError{error: fmt.Errorf("recursive copy requested, source directory %q contains the target directory %q", copySrc, config.WorkingSourceDir)}
	}

	di := ignore.DockerIgnorer{}
	filesToIgnore, lerr := di.GetListOfFilesToIgnore(copySrc)
	if lerr != nil {
		return nil, lerr
	}

	if copySrc != config.WorkingSourceDir {
		f.KeepSymlinks(config.KeepSymlinks)
		err := f.CopyContents(copySrc, config.WorkingSourceDir, filesToIgnore)
		if err != nil {
			return nil, err
		}
	}

	return &git.SourceInfo{
		Location:   config.Source.LocalPath(),
		ContextDir: config.ContextDir,
	}, nil
}
