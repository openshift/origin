package scm

import (
	"fmt"

	"github.com/openshift/source-to-image/pkg/errors"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"

	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/scm/empty"
	"github.com/openshift/source-to-image/pkg/scm/file"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/util"
)

var glog = utilglog.StderrLog

// DownloaderForSource determines what SCM plugin should be used for downloading
// the sources from the repository.
func DownloaderForSource(s string, forceCopy bool) (build.Downloader, string, error) {
	glog.V(4).Infof("DownloadForSource %s", s)

	if len(s) == 0 {
		return &empty.Noop{}, s, nil
	}

	details, mods, err := git.ParseFile(s)
	glog.V(4).Infof("return from ParseFile file exists %v proto specified %v use copy %v", details.FileExists, details.ProtoSpecified, details.UseCopy)
	if err != nil {
		if e, ok := err.(errors.Error); !forceCopy || !(ok && (e.ErrorCode == errors.EmptyGitRepositoryError)) {
			return nil, s, err
		}
	}

	if details.FileExists && details.BadRef {
		return nil, s, fmt.Errorf("local location referenced by %s exists but the input after the # is malformed", s)
	}

	if details.FileExists && mods != nil {
		glog.V(4).Infof("new source from parse file %s", mods.Path)
		if details.ProtoSpecified {
			s = mods.Path
		} else {
			// prepending with file:// is a precautionary step which previous incarnations of this code did; we
			// preserve that behavior (it is more explicit, if not absolutely necessary; but we do it here as was done before
			// vs. down in our generic git layer (which is leveraged separately in origin)
			s = "file://" + mods.Path
		}
	}

	if details.FileExists && (details.UseCopy || forceCopy) {
		return &file.File{FileSystem: util.NewFileSystem()}, s, nil
	}

	// If the source is valid  Git protocol (file://, ssh://, git://, git@, etc..) use Git
	// binary to download the sources
	g := git.New()
	ok, err := g.ValidCloneSpec(s)
	if err != nil {
		return nil, s, err
	}

	if ok {
		return &git.Clone{Git: g, FileSystem: util.NewFileSystem()}, s, nil
	}

	return nil, s, fmt.Errorf("no downloader defined for location: %q", s)
}
