package scm

import (
	"os/exec"
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/scm/file"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/util"
)

// DownloaderForSource determines what SCM plugin should be used for downloading
// the sources from the repository.
func DownloaderForSource(s string) build.Downloader {
	// If the source starts with file:// and there is no GIT binary, use 'file'
	// SCM plugin
	if (strings.HasPrefix(s, "file://") || strings.HasPrefix(s, "/")) && !hasGitBinary() {
		return &file.File{util.NewFileSystem()}
	}

	g := git.New()
	if g.ValidCloneSpec(s) {
		return &git.Clone{g, util.NewFileSystem()}
	}

	glog.Errorf("No downloader defined for %q source URL", s)
	return nil
}

func hasGitBinary() bool {
	_, err := exec.LookPath("git")
	return err == nil
}
