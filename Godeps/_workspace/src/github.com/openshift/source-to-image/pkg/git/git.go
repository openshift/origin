package git

import (
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/openshift/source-to-image/pkg/util"
)

// Git is an interface used by main STI code to extract/checkout git repositories
type Git interface {
	ValidCloneSpec(source string) bool
	Clone(source, target string) error
	Checkout(repo, ref string) error
}

// NewGit returns a new instance of the default implementation of the Git interface
func New() Git {
	return &stiGit{
		runner: util.NewCommandRunner(),
	}
}

type stiGit struct {
	runner util.CommandRunner
}

var gitSshURLExp = regexp.MustCompile(`\A([\w\d\-_\.+]+@[\w\d\-_\.+]+:[\w\d\-_\.+%/]+\.git)$`)

var allowedSchemes = []string{"git", "http", "https", "file"}

func stringInSlice(s string, slice []string) bool {
	for _, element := range slice {
		if s == element {
			return true
		}
	}

	return false
}

// ValidCloneSpec determines if the given string reference points to a valid git
// repository
func (h *stiGit) ValidCloneSpec(source string) bool {
	url, err := url.Parse(source)
	if err != nil {
		return false
	}

	if stringInSlice(url.Scheme, allowedSchemes) {
		return true
	}

	// support 'git@' ssh urls and local protocol without 'file://' scheme
	return url.Scheme == "" && (strings.HasSuffix(source, ".git") ||
		(strings.HasPrefix(source, "git@") && gitSshURLExp.MatchString(source)))
}

// Clone clones a git repository to a specific target directory
func (h *stiGit) Clone(source, target string) error {
	opts := util.CommandOpts{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	return h.runner.RunWithOptions(opts, "git", "clone", "--recursive", source, target)
}

// Checkout checks out a specific branch reference of a given git repository
func (h *stiGit) Checkout(repo, ref string) error {
	opts := util.CommandOpts{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Dir:    repo,
	}
	return h.runner.RunWithOptions(opts, "git", "checkout", ref)
}
