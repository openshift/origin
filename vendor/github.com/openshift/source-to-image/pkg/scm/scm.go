package scm

import (
	"fmt"

	s2ierr "github.com/openshift/source-to-image/pkg/errors"
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
func DownloaderForSource(fs util.FileSystem, s string, forceCopy bool) (build.Downloader, string, error) {
	glog.V(4).Infof("DownloadForSource %s", s)

	if len(s) == 0 {
		return &empty.Noop{}, s, nil
	}

	details, mods, err := git.ParseFile(fs, s)
	glog.V(4).Infof("return from ParseFile file exists %v proto specified %v use copy %v", details.FileExists, details.ProtoSpecified, details.UseCopy)
	if err != nil {
		if e, ok := err.(s2ierr.Error); !forceCopy || !(ok && (e.ErrorCode == s2ierr.EmptyGitRepositoryError)) {
			return nil, s, err
		}
	}

	if details.FileExists && details.BadRef {
		return nil, s, fmt.Errorf("local location referenced by %s exists but the input after the # is malformed", s)
	}

	if details.FileExists && mods != nil {
		glog.V(4).Infof("new source from parse file %s", mods.Path)
		s = "file://" + mods.Path
	}

	if details.FileExists && (details.UseCopy || forceCopy) {
		return &file.File{FileSystem: fs}, s, nil
	}

	// If s is a valid Git clone spec, use git to download the source
	g := git.New(fs)
	ok, err := g.ValidCloneSpec(s)
	if err != nil {
		return nil, s, err
	}

	if ok {
		return &git.Clone{Git: g, FileSystem: fs}, s, nil
	}

	return nil, s, fmt.Errorf("no downloader defined for location: %q", s)
}
