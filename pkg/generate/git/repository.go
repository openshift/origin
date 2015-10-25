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
)

// Repository represents a git source repository
type Repository interface {
	GetRootDir(dir string) (string, error)
	GetOriginURL(dir string) (string, bool, error)
	GetRef(dir string) string
	Clone(dir string, url string) error
	CloneBare(dir string, url string) error
	CloneMirror(dir string, url string) error
	Fetch(dir string) error
	Checkout(dir string, ref string) error
	Archive(dir, ref, format string, w io.Writer) error
	Init(dir string, bare bool) error
	AddRemote(dir string, name, url string) error
	AddLocalConfig(dir, name, value string) error
	ShowFormat(dir, commit, format string) (string, error)
}

// execGitFunc is a function that executes a Git command
type execGitFunc func(w io.Writer, dir string, args ...string) (string, string, error)

type repository struct {
	git execGitFunc
}

// NewRepository creates a new Repository for the given directory
func NewRepository() Repository {
	return &repository{
		git: func(w io.Writer, dir string, args ...string) (string, string, error) {
			return command(w, "git", dir, args...)
		},
	}
}

// NewRepositoryForBinary returns a Repository using the specified
// git executable.
func NewRepositoryForBinary(gitBinaryPath string) Repository {
	return &repository{
		git: func(w io.Writer, dir string, args ...string) (string, string, error) {
			return command(w, gitBinaryPath, dir, args...)
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

// Clone clones a remote git repository to a local directory
func (r *repository) Clone(location string, url string) error {
	_, _, err := r.git(nil, "", "clone", "--recursive", url, location)
	return err
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

// command executes an external command in the given directory.
// The command's standard out and error are trimmed and returned as strings
// It may return the type *GitError if the command itself fails.
func command(w io.Writer, name, dir string, args ...string) (stdout, stderr string, err error) {
	cmdOut := &bytes.Buffer{}
	cmdErr := &bytes.Buffer{}

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if w != nil {
		cmd.Stdout = w
	} else {
		cmd.Stdout = cmdOut
	}
	cmd.Stderr = cmdErr

	err = cmd.Run()
	if w == nil {
		stdout = strings.TrimFunc(cmdOut.String(), unicode.IsSpace)
	}
	stderr = strings.TrimFunc(cmdErr.String(), unicode.IsSpace)
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
