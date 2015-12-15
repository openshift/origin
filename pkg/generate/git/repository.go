package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/golang/glog"

	s2iapi "github.com/openshift/source-to-image/pkg/api"
)

// Repository represents a git source repository
type Repository interface {
	GetRootDir(dir string) (string, error)
	GetOriginURL(dir string) (string, bool, error)
	GetRef(dir string) string
	Clone(dir string, url string) error
	CloneWithOptions(dir string, url string, opts CloneOptions) error
	CloneBare(dir string, url string) error
	CloneMirror(dir string, url string) error
	Fetch(dir string) error
	Checkout(dir string, ref string) error
	Archive(dir, ref, format string, w io.Writer) error
	Init(dir string, bare bool) error
	AddRemote(dir string, name, url string) error
	AddLocalConfig(dir, name, value string) error
	ShowFormat(dir, commit, format string) (string, error)
	ListRemote(url string, args ...string) (string, string, error)
	GetInfo(location string) (*SourceInfo, []error)
}

// SourceInfo stores information about the source code
type SourceInfo struct {
	s2iapi.SourceInfo
}

// CloneOptions are options used in cloning a git repository
type CloneOptions struct {
	Recursive bool
	Quiet     bool
}

// execGitFunc is a function that executes a Git command
type execGitFunc func(w io.Writer, dir string, args ...string) (string, string, error)

type repository struct {
	git execGitFunc
}

// NewRepository creates a new Repository
func NewRepository() Repository {
	return NewRepositoryWithEnv(nil)
}

// NewRepositoryForEnv creates a new Repository using the specified environment
func NewRepositoryWithEnv(env []string) Repository {
	return &repository{
		git: func(w io.Writer, dir string, args ...string) (string, string, error) {
			return command(w, "git", dir, env, args...)
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
		git: func(w io.Writer, dir string, args ...string) (string, string, error) {
			return command(w, gitBinaryPath, dir, env, args...)
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

// GetRootDir obtains the directory root for a Git repository
func (r *repository) GetRootDir(location string) (string, error) {
	dir, _, err := r.git(nil, location, "rev-parse", "--git-dir")
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
	text, _, err := r.git(nil, location, "config", "--get-regexp", "^remote\\..*\\.url$")
	if err != nil {
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
	branch, _, err := r.git(nil, location, "symbolic-ref", "-q", "--short", "HEAD")
	if err != nil {
		branch = ""
	}
	return branch
}

// AddRemote adds a new remote to the repository.
func (r *repository) AddRemote(location, name, url string) error {
	_, _, err := r.git(nil, location, "remote", "add", name, url)
	return err
}

// AddLocalConfig adds a value to the current repository
func (r *repository) AddLocalConfig(location, name, value string) error {
	_, _, err := r.git(nil, location, "config", "--local", "--add", name, value)
	return err
}

// CloneWithOptions clones a remote git repository to a local directory
func (r *repository) CloneWithOptions(location string, url string, opts CloneOptions) error {
	args := []string{"clone"}
	if opts.Quiet {
		args = append(args, "--quiet")
	}
	if opts.Recursive {
		args = append(args, "--recursive")
	}
	args = append(args, url)
	args = append(args, location)
	_, _, err := r.git(nil, "", args...)
	return err
}

// Clone clones a remote git repository to a local directory
func (r *repository) Clone(location string, url string) error {
	return r.CloneWithOptions(location, url, CloneOptions{Recursive: true})
}

// ListRemote lists references in a remote repository
func (r *repository) ListRemote(url string, args ...string) (string, string, error) {
	gitArgs := []string{"ls-remote", url}
	gitArgs = append(gitArgs, args...)
	return r.git(nil, "", gitArgs...)
}

// CloneMirror clones a remote git repository to a local directory as a mirror
func (r *repository) CloneMirror(location string, url string) error {
	_, _, err := r.git(nil, "", "clone", "--mirror", url, location)
	return err
}

// CloneBare clones a remote git repository to a local directory
func (r *repository) CloneBare(location string, url string) error {
	_, _, err := r.git(nil, "", "clone", "--bare", url, location)
	return err
}

// Fetch updates the provided git repository
func (r *repository) Fetch(location string) error {
	_, _, err := r.git(nil, location, "fetch", "--all")
	return err
}

// Archive creates a archive of the Git repo at directory location at commit ref and with the given Git format,
// and then writes that to the provided io.Writer
func (r *repository) Archive(location, ref, format string, w io.Writer) error {
	_, _, err := r.git(w, location, "archive", fmt.Sprintf("--format=%s", format), ref)
	return err
}

// Checkout switches to the given ref for the git repository
func (r *repository) Checkout(location string, ref string) error {
	_, _, err := r.git(nil, location, "checkout", ref)
	return err
}

// ShowFormat formats the ref with the given git show format string
func (r *repository) ShowFormat(location, ref, format string) (string, error) {
	out, _, err := r.git(nil, location, "show", "--quiet", ref, fmt.Sprintf("--format=%s", format))
	return out, err
}

// Init initializes a new git repository in the provided location
func (r *repository) Init(location string, bare bool) error {
	_, _, err := r.git(nil, "", "init", "--bare", location)
	return err
}

// GetInfo retrieves the informations about the source code and commit
func (r *repository) GetInfo(location string) (*SourceInfo, []error) {
	errors := []error{}
	git := func(arg ...string) string {
		stdout, stderr, err := r.git(nil, location, arg...)
		if err != nil {
			errors = append(errors, fmt.Errorf("error invoking '%s': %v. Out: %s, Err: %s",
				strings.Join(arg, " "), err, stdout, stderr))
		}
		return strings.TrimSpace(stdout)
	}
	info := &SourceInfo{}
	info.Location = git("config", "--get", "remote.origin.url")
	info.Ref = git("rev-parse", "--abbrev-ref", "HEAD")
	info.CommitID = git("rev-parse", "--verify", "HEAD")
	info.AuthorName = git("--no-pager", "show", "-s", "--format=%an", "HEAD")
	info.AuthorEmail = git("--no-pager", "show", "-s", "--format=%ae", "HEAD")
	info.CommitterName = git("--no-pager", "show", "-s", "--format=%cn", "HEAD")
	info.CommitterEmail = git("--no-pager", "show", "-s", "--format=%ce", "HEAD")
	info.Date = git("--no-pager", "show", "-s", "--format=%ad", "HEAD")
	info.Message = git("--no-pager", "show", "-s", "--format=%<(80,trunc)%s", "HEAD")

	return info, errors
}

// command executes an external command in the given directory.
// The command's standard out and error are trimmed and returned as strings
// It may return the type *GitError if the command itself fails.
func command(w io.Writer, name, dir string, env []string, args ...string) (stdout, stderr string, err error) {
	cmdOut := &bytes.Buffer{}
	cmdErr := &bytes.Buffer{}

	glog.V(4).Infof("Executing %s %s", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if w != nil {
		cmd.Stdout = w
	} else {
		cmd.Stdout = cmdOut
	}
	cmd.Stderr = cmdErr
	cmd.Env = env

	if glog.V(8) && env != nil {
		glog.Infof("Environment:\n")
		for _, e := range env {
			glog.Infof("- %s", e)
		}
	}

	err = cmd.Run()
	if err != nil {
		glog.V(4).Infof("Exec error: %v", err)
	}
	if w == nil {
		stdout = strings.TrimFunc(cmdOut.String(), unicode.IsSpace)
		if len(stdout) > 0 {
			glog.V(4).Infof("Out: %s", stdout)
		}
	}
	stderr = strings.TrimFunc(cmdErr.String(), unicode.IsSpace)
	if len(stderr) > 0 {
		glog.V(4).Infof("Err: %s", cmdErr.String())
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		err = &GitError{
			Err:    exitErr,
			Stdout: stdout,
			Stderr: stderr,
		}
	}
	return
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
