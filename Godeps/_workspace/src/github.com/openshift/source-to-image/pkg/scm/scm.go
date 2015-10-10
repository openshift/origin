package scm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/scm/file"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/util"
)

// DownloaderForSource determines what SCM plugin should be used for downloading
// the sources from the repository.
func DownloaderForSource(s string) (build.Downloader, string, error) {
	// If the source is using file:// protocol but it is not a GIT repository,
	// trim the prefix and treat it as a file copy.
	if strings.HasPrefix(s, "file://") && !isLocalGitRepository(s) {
		s = strings.TrimPrefix(s, "file://")
	}

	// If the source is file:// protocol and it is GIT repository, but we don't
	// have GIT binary to fetch it, treat it as file copy.
	if strings.HasPrefix(s, "file://") && !hasGitBinary() {
		s = strings.TrimPrefix(s, "file://")
	}

	// If the source is valid GIT protocol (file://, git://, git@, etc..) use GIT
	// binary to download the sources
	if g := git.New(); g.ValidCloneSpec(s) {
		return &git.Clone{g, util.NewFileSystem()}, s, nil
	}

	// Convert relative path to absolute path.
	if !strings.HasPrefix(s, "/") {
		if absolutePath, err := filepath.Abs(s); err == nil {
			s = absolutePath
		}
	}

	if isLocalGitRepository(s) {
		return DownloaderForSource("file://" + s)
	}

	// If we have local directory and that directory exists, use file copy
	if _, err := os.Stat(s); err == nil {
		return &file.File{util.NewFileSystem()}, s, nil
	}

	return nil, s, fmt.Errorf("No downloader defined for location: %q", s)
}

// isLocalGitRepository checks if the specified directory has .git subdirectory (it
// is a GIT repository)
func isLocalGitRepository(dir string) bool {
	_, err := os.Stat(fmt.Sprintf("%s/.git", strings.TrimPrefix(dir, "file://")))
	return !(err != nil && os.IsNotExist(err))
}

// hasGitBinary checks if the 'git' binary is available on the system
func hasGitBinary() bool {
	_, err := exec.LookPath("git")
	return err == nil
}
