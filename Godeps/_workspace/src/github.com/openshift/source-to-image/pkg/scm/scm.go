package scm

import (
	"fmt"

	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/scm/file"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/util"
)

// DownloaderForSource determines what SCM plugin should be used for downloading
// the sources from the repository.
func DownloaderForSource(s string) (build.Downloader, string, error) {
	details, _ := git.ParseFile(s)

	if details.FileExists && details.UseCopy {
		if !details.ProtoSpecified {
			// since not using git, any resulting URLs need to be explicit with file:// protocol specified
			s = "file://" + s
		}
		return &file.File{util.NewFileSystem()}, s, nil
	}

	if details.ProtoSpecified && !details.FileExists {
		return nil, s, fmt.Errorf("local location: %s does not exist", s)
	}

	if !details.ProtoSpecified && details.FileExists {
		// if local file system, without file://, when using git, should not need file://, but we'll be safe;
		// satisfies previous constructed test case in scm_test.go as well
		s = "file://" + s
	}

	// If the source is valid  GIT remote protocol (ssh://, git://, git@, etc..) use GIT
	// binary to download the sources
	g := git.New()
	if g.ValidCloneSpec(s) {
		return &git.Clone{g, util.NewFileSystem()}, s, nil
	}

	return nil, s, fmt.Errorf("no downloader defined for location: %q", s)
}
