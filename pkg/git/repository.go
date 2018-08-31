package git

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"
)

// Repository represents a git source repository
type Repository interface {
	GetRootDir(dir string) (string, error)
	GetOriginURL(dir string) (string, bool, error)
	GetRef(dir string) string
	Clone(dir string, url string) error
	CloneWithOptions(dir string, url string, args ...string) error
	CloneBare(dir string, url string) error
	CloneMirror(dir string, url string) error
	Fetch(dir string, url string, ref string) error
	Checkout(dir string, ref string) error
	PotentialPRRetryAsFetch(dir string, url string, ref string, err error) error
	SubmoduleUpdate(dir string, init, recursive bool) error
	Archive(dir, ref, format string, w io.Writer) error
	Init(dir string, bare bool) error
	Add(dir string, spec string) error
	Commit(dir string, message string) error
	AddRemote(dir string, name, url string) error
	AddLocalConfig(dir, name, value string) error
	ShowFormat(dir, commit, format string) (string, error)
	ListRemote(url string, args ...string) (string, string, error)
	TimedListRemote(timeout time.Duration, url string, args ...string) (string, string, error)
	GetInfo(location string) (*SourceInfo, []error)
}

const (
	// defaultCommandTimeout is the default timeout for git commands that we want to enforce timeouts on
	defaultCommandTimeout = 30 * time.Second

	// Shallow maps to --depth=1, which clones a Git repository without
	// downloading history
	Shallow = "--depth=1"

	// noCommandTimeout signals that there should be no timeout for the command when passed as the timeout
	// for the default timedExecGitFunc
	noCommandTimeout = 0 * time.Second
)

// ErrGitNotAvailable will be returned if the git call fails because a git binary
// could not be found
var ErrGitNotAvailable = errors.New("git binary not available")

// SourceInfo stores information about the source code
type SourceInfo struct {
	// Ref represents a commit SHA-1, valid Git branch name or a Git tag
	// The output image will contain this information as 'io.openshift.build.commit.ref' label.
	Ref string

	// CommitID represents an arbitrary extended object reference in Git as SHA-1
	// The output image will contain this information as 'io.openshift.build.commit.id' label.
	CommitID string

	// Date contains a date when the committer created the commit.
	// The output image will contain this information as 'io.openshift.build.commit.date' label.
	Date string

	// AuthorName contains the name of the author
	// The output image will contain this information (along with AuthorEmail) as 'io.openshift.build.commit.author' label.
	AuthorName string

	// AuthorEmail contains the e-mail of the author
	// The output image will contain this information (along with AuthorName) as 'io.openshift.build.commit.author' lablel.
	AuthorEmail string

	// CommitterName contains the name of the committer
	CommitterName string

	// CommitterEmail contains the e-mail of the committer
	CommitterEmail string

	// Message represents the first 80 characters from the commit message.
	// The output image will contain this information as 'io.openshift.build.commit.message' label.
	Message string

	// Location contains a valid URL to the original repository.
	// The output image will contain this information as 'io.openshift.build.source-location' label.
	Location string

	// ContextDir contains path inside the Location directory that
	// contains the application source code.
	// The output image will contain this information as 'io.openshift.build.source-context-dir'
	// label.
	ContextDir string
}

// execGitFunc is a function that executes a Git command
type execGitFunc func(dir string, args ...string) (string, string, error)

// timedExecGitFunc is a function that executes a Git command with a timeout
type timedExecGitFunc func(timeout time.Duration, dir string, args ...string) (string, string, error)

type repository struct {
	git      execGitFunc
	timedGit timedExecGitFunc

	shallow bool
}

// NewRepository creates a new Repository
func NewRepository() Repository {
	return NewRepositoryWithEnv(nil)
}

// NewRepositoryForEnv creates a new Repository using the specified environment
func NewRepositoryWithEnv(env []string) Repository {
	return &repository{
		git: func(dir string, args ...string) (string, string, error) {
			return command("git", dir, env, args...)
		},
		timedGit: func(timeout time.Duration, dir string, args ...string) (string, string, error) {
			return timedCommand(timeout, "git", dir, env, args...)
		},
	}
}

// NewRepositoryForBinary returns a Repository using the specified
// git executable.
func NewRepositoryForBinary(gitBinaryPath string) Repository {
	return NewRepositoryForBinaryWithEnvironment(gitBinaryPath, nil)
}

// NewRepositoryForBinary returns a Repository using the specified
// git executable and environment
func NewRepositoryForBinaryWithEnvironment(gitBinaryPath string, env []string) Repository {
	return &repository{
		git: func(dir string, args ...string) (string, string, error) {
			return command(gitBinaryPath, dir, env, args...)
		},
		timedGit: func(timeout time.Duration, dir string, args ...string) (string, string, error) {
			return timedCommand(timeout, gitBinaryPath, dir, env, args...)
		},
	}
}

// IsRoot returns true if location is the root of a bare git repository
func IsBareRoot(path string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, "HEAD"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// PotentialPRRetryAsFetch is used on checkout errors after a clone where the possibility
// that a fetch or a PR ref is needed between the clone and checkout operations
// Cases include: 1) GitHub PRs (example ref forms: (refs/)?pull/[1-9][0-9]*/head);
// 2) Refs which the RHEL7 git version appears to be too old to handle correctly
// (example ref form: foo-bar-1), but which newer git versions seem to manage OK.
func (r *repository) PotentialPRRetryAsFetch(dir, remote, ref string, err error) error {
	glog.V(4).Infof("Checkout after clone failed for ref %s with error: %v, attempting fetch", ref, err)
	err = r.Fetch(dir, remote, ref)
	if err != nil {
		return err
	}

	err = r.Checkout(dir, "FETCH_HEAD")
	if err != nil {
		return err
	}
	glog.V(4).Infof("Fetch / checkout for %s successful", ref)
	return nil
}

// GetRootDir obtains the directory root for a Git repository
func (r *repository) GetRootDir(location string) (string, error) {
	dir, _, err := r.git(location, "rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", fmt.Errorf("%s is not a git repository", dir)
	}
	if strings.HasSuffix(dir, ".git") {
		dir = dir[:len(dir)-4]
		if strings.HasSuffix(dir, "/") {
			dir = dir[:len(dir)-1]
		}
	}
	if len(dir) == 0 {
		dir = location
	}
	return dir, nil
}

var (
	remoteURLExtract  = regexp.MustCompile("^remote\\.(.*)\\.url (.*?)$")
	remoteOriginNames = []string{"origin", "upstream", "github", "openshift", "heroku"}
)

// GetOriginURL returns the origin branch URL for the git repository
func (r *repository) GetOriginURL(location string) (string, bool, error) {
	text, _, err := r.git(location, "config", "--get-regexp", "^remote\\..*\\.url$")
	if err != nil {
		if IsExitCode(err, 1) {
			return "", false, nil
		}
		return "", false, err
	}

	remotes := make(map[string]string)
	s := bufio.NewScanner(bytes.NewBufferString(text))
	for s.Scan() {
		if matches := remoteURLExtract.FindStringSubmatch(s.Text()); matches != nil {
			remotes[matches[1]] = matches[2]
		}
	}
	if err := s.Err(); err != nil {
		return "", false, err
	}
	for _, remote := range remoteOriginNames {
		if url, ok := remotes[remote]; ok {
			return url, true, nil
		}
	}

	return "", false, nil
}

// GetRef retrieves the current branch reference for the git repository
func (r *repository) GetRef(location string) string {
	branch, _, err := r.git(location, "symbolic-ref", "-q", "--short", "HEAD")
	if err != nil {
		branch = ""
	}
	return branch
}

// AddRemote adds a new remote to the repository.
func (r *repository) AddRemote(location, name, url string) error {
	_, _, err := r.git(location, "remote", "add", name, url)
	return err
}

// AddLocalConfig adds a value to the current repository
func (r *repository) AddLocalConfig(location, name, value string) error {
	_, _, err := r.git(location, "config", "--local", "--add", name, value)
	return err
}

// CloneWithOptions clones a remote git repository to a local directory
func (r *repository) CloneWithOptions(location string, url string, args ...string) error {
	gitArgs := []string{"clone"}
	gitArgs = append(gitArgs, args...)
	gitArgs = append(gitArgs, url)
	gitArgs = append(gitArgs, location)

	// We need to check to see if we're importing reference information, for
	// for error checking later on
	for _, opt := range gitArgs {
		if opt == Shallow {
			r.shallow = true
			break
		}
	}

	_, _, err := r.git("", gitArgs...)
	return err
}

// Clone clones a remote git repository to a local directory
func (r *repository) Clone(location string, url string) error {
	return r.CloneWithOptions(location, url, "--recursive")
}

// CloneMirror clones a remote git repository to a local directory as a mirror
func (r *repository) CloneMirror(location string, url string) error {
	return r.CloneWithOptions(location, url, "--mirror")
}

// CloneBare clones a remote git repository to a local directory
func (r *repository) CloneBare(location string, url string) error {
	return r.CloneWithOptions(location, url, "--bare")
}

// ListRemote lists references in a remote repository
// ListRemote will time out with a default timeout of 10s. If a different timeout is
// required, TimedListRemote should be used instead
func (r *repository) ListRemote(url string, args ...string) (string, string, error) {
	return r.TimedListRemote(defaultCommandTimeout, url, args...)
}

// TimedListRemote lists references in a remote repository, or fails if the list does
// not complete before the given timeout
func (r *repository) TimedListRemote(timeout time.Duration, url string, args ...string) (string, string, error) {
	gitArgs := []string{"ls-remote"}
	gitArgs = append(gitArgs, args...)
	gitArgs = append(gitArgs, url)
	// `git ls-remote` does not allow for any timeout to be set, and defaults to a timeout
	// of five minutes, so we enforce a timeout here to allow it to fail eariler than that
	return r.timedGit(timeout, "", gitArgs...)
}

// Fetch updates the provided git repository
func (r *repository) Fetch(location, uri, ref string) error {
	_, _, err := r.git(location, "fetch", uri, ref)
	return err
}

// Archive creates a archive of the Git repo at directory location at commit ref and with the given Git format,
// and then writes that to the provided io.Writer
func (r *repository) Archive(location, ref, format string, w io.Writer) error {
	stdout, _, err := r.git(location, "archive", fmt.Sprintf("--format=%s", format), ref)
	w.Write([]byte(stdout))
	return err
}

// Checkout switches to the given ref for the git repository
func (r *repository) Checkout(location string, ref string) error {
	if r.shallow {
		return errors.New("cannot checkout ref on shallow clone")
	}
	_, _, err := r.git(location, "checkout", ref, "--")
	return err
}

// SubmoduleUpdate updates submodules, optionally recursively
func (r *repository) SubmoduleUpdate(location string, init, recursive bool) error {
	updateArgs := []string{"submodule", "update"}
	if init {
		updateArgs = append(updateArgs, "--init")
	}
	if recursive {
		updateArgs = append(updateArgs, "--recursive")
	}

	_, _, err := r.git(location, updateArgs...)
	return err
}

// ShowFormat formats the ref with the given git show format string
func (r *repository) ShowFormat(location, ref, format string) (string, error) {
	out, _, err := r.git(location, "show", "-s", ref, fmt.Sprintf("--format=%s", format))
	return out, err
}

// Init initializes a new git repository in the provided location
func (r *repository) Init(location string, bare bool) error {
	args := []string{"init"}
	if bare {
		args = append(args, "--bare")
	}
	args = append(args, location)
	_, _, err := r.git("", args...)
	return err
}

func (r *repository) Add(location, spec string) error {
	_, _, err := r.git(location, "add", spec)
	return err
}

func (r *repository) Commit(location, message string) error {
	_, _, err := r.git(location, "commit", "-m", message)
	return err
}

// GetInfo retrieves the informations about the source code and commit
func (r *repository) GetInfo(location string) (*SourceInfo, []error) {
	errors := []error{}
	git := func(arg ...string) string {
		stdout, stderr, err := r.git(location, arg...)
		if err != nil {
			errors = append(errors, fmt.Errorf("error invoking 'git %s': %v. Out: %s, Err: %s",
				strings.Join(arg, " "), err, stdout, stderr))
		}
		return strings.TrimSpace(stdout)
	}
	info := &SourceInfo{}
	info.Ref = git("rev-parse", "--abbrev-ref", "HEAD")
	info.CommitID = git("rev-parse", "--verify", "HEAD")
	info.AuthorName = git("--no-pager", "show", "-s", "--format=%an", "HEAD")
	info.AuthorEmail = git("--no-pager", "show", "-s", "--format=%ae", "HEAD")
	info.CommitterName = git("--no-pager", "show", "-s", "--format=%cn", "HEAD")
	info.CommitterEmail = git("--no-pager", "show", "-s", "--format=%ce", "HEAD")
	info.Date = git("--no-pager", "show", "-s", "--format=%ad", "HEAD")
	info.Message = git("--no-pager", "show", "-s", "--format=%<(80,trunc)%s", "HEAD")

	// it is not required for a Git repository to have a remote "origin" defined
	if out, _, err := r.git(location, "config", "--get", "remote.origin.url"); err == nil {
		info.Location = out
	}

	return info, errors
}

// command executes an external command in the given directory.
// The command's standard out and error are trimmed and returned as strings
// It may return the type *GitError if the command itself fails.
func command(name, dir string, env []string, args ...string) (stdout, stderr string, err error) {
	return timedCommand(noCommandTimeout, name, dir, env, args...)
}

// timedCommand executes an external command in the given directory with a timeout.
// The command's standard out and error are returned as strings.
// It may return the type *GitError if the command itself fails or the type *TimeoutError
// if the command times out before finishing.
// If the git binary cannot be found, ErrGitNotAvailable will be returned as the error.
func timedCommand(timeout time.Duration, name, dir string, env []string, args ...string) (stdout, stderr string, err error) {
	var stdoutBuffer, stderrBuffer bytes.Buffer

	glog.V(4).Infof("Executing %s %s", name, strings.Join(args, " "))

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer

	if env != nil {
		glog.V(8).Infof("Environment:\n")
		for _, e := range env {
			glog.V(8).Infof("- %s", e)
		}
	}

	err, timedOut := runCommand(cmd, timeout)
	if timedOut {
		return "", "", &TimeoutError{
			Err: fmt.Errorf("execution of %s %s timed out after %s", name, strings.Join(args, " "), timeout),
		}
	}

	// we don't want captured output to have a trailing newline for formatting reasons
	stdout, stderr = strings.TrimRight(stdoutBuffer.String(), "\n"), strings.TrimRight(stderrBuffer.String(), "\n")

	// check whether git was available in the first place
	if err != nil {
		if !isBinaryInstalled(name) {
			return "", "", ErrGitNotAvailable
		}
	}

	// if we encounter an error we recognize, return a typed error
	if exitErr, ok := err.(*exec.ExitError); ok {
		return stdout, stderr, &GitError{
			Err:    exitErr,
			Stdout: stdout,
			Stderr: stderr,
		}
	}

	// if we didn't encounter an ExitError or a timeout, simply return the error
	return stdout, stderr, err
}

// runCommand runs the command with the given timeout, and returns any errors encountered and whether
// the command timed out or not
func runCommand(cmd *exec.Cmd, timeout time.Duration) (error, bool) {
	out := make(chan error)
	go func() {
		if err := cmd.Start(); err != nil {
			glog.V(4).Infof("Error starting execution: %v", err)
		}
		out <- cmd.Wait()
	}()

	if timeout == noCommandTimeout {
		select {
		case err := <-out:
			if err != nil {
				glog.V(4).Infof("Error executing command: %v", err)
			}
			return err, false
		}
	} else {
		select {
		case err := <-out:
			if err != nil {
				glog.V(4).Infof("Error executing command: %v", err)
			}
			return err, false
		case <-time.After(timeout):
			glog.V(4).Infof("Command execution timed out after %s", timeout)
			return nil, true
		}
	}
}

// TimeoutError is returned when the underlying Git coommand times out before finishing
type TimeoutError struct {
	Err error
}

func (e *TimeoutError) Error() string {
	return e.Err.Error()
}

// GitError is returned when the underlying Git command returns a non-zero exit code.
type GitError struct {
	Err    error
	Stdout string
	Stderr string
}

func (e *GitError) Error() string {
	if len(e.Stderr) > 0 {
		return e.Stderr
	}
	return e.Err.Error()
}

func IsExitCode(err error, exitCode int) bool {
	switch t := err.(type) {
	case *GitError:
		return IsExitCode(t.Err, exitCode)
	case *exec.ExitError:
		if ws, ok := t.Sys().(syscall.WaitStatus); ok {
			return ws.ExitStatus() == exitCode
		}
		return false
	}
	return false
}

func gitBinary() string {
	if runtime.GOOS == "windows" {
		return "git.exe"
	}
	return "git"
}

func IsGitInstalled() bool {
	return isBinaryInstalled(gitBinary())
}

func isBinaryInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
