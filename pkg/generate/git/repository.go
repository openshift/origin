package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"unicode"
)

// execCmdFunc is a function that executes an external command
type execCmdFunc func(dir, name string, args ...string) (string, string, error)

// Repository represents a git source repository
type Repository interface {
	GetRootDir(dir string) (string, error)
	GetOriginURL(dir string) (string, bool, error)
	GetRef(dir string) string
	Clone(dir string, url string) error
	Checkout(dir string, ref string) error
}

type repository struct {
	exec execCmdFunc
}

// NewRepository creates a new Repository for the given directory
func NewRepository() Repository {
	return &repository{
		exec: execCmd,
	}
}

// GetRootDir obtains the directory root for a Git repository
func (r *repository) GetRootDir(location string) (string, error) {
	dir, _, err := r.exec(location, "git", "rev-parse", "--git-dir")
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
	text, _, err := r.exec(location, "git", "config", "--get-regexp", "^remote\\..*\\.url$")
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
	branch, _, err := r.exec(location, "git", "symbolic-ref", "-q", "--short", "HEAD")
	if err != nil {
		branch = ""
	}
	return branch
}

// Clone clones a remote git repository to a local directory
func (r *repository) Clone(location string, url string) error {
	_, _, err := r.exec("", "git", "clone", "--recursive", url, location)
	return err
}

// Checkout switches to the given ref for the git repository
func (r *repository) Checkout(location string, ref string) error {
	_, _, err := r.exec(location, "git", "checkout", ref)
	return err
}

// execCmd executes an external command in the given directory.
// The command's standard out and error are trimmed and returned as strings
func execCmd(dir, name string, args ...string) (stdout, stderr string, err error) {
	cmdOut := &bytes.Buffer{}
	cmdErr := &bytes.Buffer{}

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = cmdOut
	cmd.Stderr = cmdErr

	err = cmd.Run()
	stdout = strings.TrimFunc(cmdOut.String(), unicode.IsSpace)
	stderr = strings.TrimFunc(cmdErr.String(), unicode.IsSpace)
	return
}
