package builder

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/glog"

	s2igit "github.com/openshift/source-to-image/pkg/scm/git"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/generate/git"
)

const (
	// urlCheckTimeout is the timeout used to check the source URL
	// If fetching the URL exceeds the timeout, then the build will
	// not proceed further and stop
	urlCheckTimeout = 16 * time.Second
)

type gitAuthError string
type gitNotFoundError string

func (e gitAuthError) Error() string {
	return fmt.Sprintf("failed to fetch requested repository %q with provided credentials", string(e))
}

func (e gitNotFoundError) Error() string {
	return fmt.Sprintf("requested repository %q not found", string(e))
}

// fetchSource retrieves the inputs defined by the build source into the
// provided directory, or returns an error if retrieval is not possible.
func fetchSource(dir string, build *api.Build, urlTimeout time.Duration, in io.Reader, gitClient GitClient) (*git.SourceInfo, error) {
	hasGitSource := false

	// expect to receive input from STDIN
	if err := extractInputBinary(in, build.Spec.Source.Binary, dir); err != nil {
		return nil, err
	}

	// may retrieve source from Git
	hasGitSource, err := extractGitSource(gitClient, build.Spec.Source.Git, build.Spec.Revision, dir, urlTimeout)
	if err != nil {
		return nil, err
	}
	var sourceInfo *git.SourceInfo
	if hasGitSource {
		var errs []error
		sourceInfo, errs = gitClient.GetInfo(dir)
		if len(errs) > 0 {
			for _, e := range errs {
				glog.Warningf("Error getting git info: %v", e)
			}
		}
	}

	// a Dockerfile has been specified, create or overwrite into the destination
	if dockerfileSource := build.Spec.Source.Dockerfile; dockerfileSource != nil {
		baseDir := dir
		// if a context dir has been defined and we cloned source, overwrite the destination
		if hasGitSource && len(build.Spec.Source.ContextDir) != 0 {
			baseDir = filepath.Join(baseDir, build.Spec.Source.ContextDir)
		}
		return sourceInfo, ioutil.WriteFile(filepath.Join(baseDir, "Dockerfile"), []byte(*dockerfileSource), 0660)
	}

	return sourceInfo, nil
}

// checkRemoteGit validates the specified Git URL. It returns GitNotFoundError
// when the remote repository not found and GitAuthenticationError when the
// remote repository failed to authenticate.
// Since this is calling the 'git' binary, the proxy settings should be
// available for this command.
func checkRemoteGit(gitClient GitClient, url string, timeout time.Duration) error {
	glog.V(4).Infof("git ls-remote %s --heads", url)

	var (
		out    string
		errOut string
		err    error
	)

	finish := make(chan struct{}, 1)
	go func() {
		out, errOut, err = gitClient.ListRemote(url, "--heads")
		close(finish)
	}()
	select {
	case <-finish:
	case <-time.After(timeout):
		return fmt.Errorf("timeout while waiting for remote repository %q", url)
	}

	if len(out) != 0 {
		glog.V(4).Infof(out)
	}
	if len(errOut) != 0 {
		glog.V(4).Infof(errOut)
	}

	combinedOut := out + errOut
	switch {
	case strings.Contains(combinedOut, "Authentication failed"):
		return gitAuthError(url)
	case strings.Contains(combinedOut, "not found"):
		return gitNotFoundError(url)
	}

	return err
}

// checkSourceURI performs a check on the URI associated with the build
// to make sure that it is valid.
func checkSourceURI(gitClient GitClient, rawurl string, timeout time.Duration) error {
	if !s2igit.New().ValidCloneSpec(rawurl) {
		return fmt.Errorf("Invalid git source url: %s", rawurl)
	}
	return checkRemoteGit(gitClient, rawurl, timeout)
}

// extractInputBinary processes the provided input stream as directed by BinaryBuildSource
// into dir.
func extractInputBinary(in io.Reader, source *api.BinaryBuildSource, dir string) error {
	if source == nil {
		return nil
	}

	var path string
	if len(source.AsFile) > 0 {
		glog.V(2).Infof("Receiving source from STDIN as file %s", source.AsFile)
		path = filepath.Join(dir, source.AsFile)

		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0664)
		if err != nil {
			return err
		}
		defer f.Close()
		n, err := io.Copy(f, os.Stdin)
		if err != nil {
			return err
		}
		glog.V(4).Infof("Received %d bytes into %s", n, path)
		return nil
	}

	glog.V(2).Infof("Receiving source from STDIN as archive")

	cmd := exec.Command("bsdtar", "-x", "-o", "-m", "-f", "-", "-C", dir)
	cmd.Stdin = in
	out, err := cmd.CombinedOutput()
	if err != nil {
		glog.V(2).Infof("Extracting...\n%s", string(out))
		return fmt.Errorf("unable to extract binary build input, must be a zip, tar, or gzipped tar, or specified as a file: %v", err)
	}
	return nil
}

func extractGitSource(gitClient GitClient, gitSource *api.GitBuildSource, revision *api.SourceRevision, dir string, timeout time.Duration) (bool, error) {
	if gitSource == nil {
		return false, nil
	}

	// Check source URI, trying to connect to the server only if not using a proxy.
	if err := checkSourceURI(gitClient, gitSource.URI, timeout); err != nil {
		return true, err
	}

	glog.V(2).Infof("Cloning source from %s", gitSource.URI)

	// Only use the quiet flag if Verbosity is not 5 or greater
	quiet := !bool(glog.V(5))
	if err := gitClient.CloneWithOptions(dir, gitSource.URI, git.CloneOptions{Recursive: true, Quiet: quiet}); err != nil {
		return true, err
	}

	// if we specify a commit, ref, or branch to checkout, do so
	if len(gitSource.Ref) != 0 || (revision != nil && revision.Git != nil && len(revision.Git.Commit) != 0) {
		commit := gitSource.Ref
		if revision != nil && revision.Git != nil && revision.Git.Commit != "" {
			commit = revision.Git.Commit
		}
		if err := gitClient.Checkout(dir, commit); err != nil {
			return true, err
		}
	}
	return true, nil
}
