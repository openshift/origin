package git

import (
	"bufio"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/util"
)

// Git is an interface used by main STI code to extract/checkout git repositories
type Git interface {
	ValidCloneSpec(source string) bool
	Clone(source, target string) error
	Checkout(repo, ref string) error
	GetInfo(string) *api.SourceInfo
}

// New returns a new instance of the default implementation of the Git interface
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
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	defer func() {
		outReader.Close()
		outWriter.Close()
		errReader.Close()
		errWriter.Close()
	}()
	opts := util.CommandOpts{
		Stdout: outWriter,
		Stderr: errWriter,
	}
	go pipeToLog(outReader, glog.Info)
	go pipeToLog(errReader, glog.Error)
	return h.runner.RunWithOptions(opts, "git", "clone", "--quiet", "--recursive", source, target)
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

// GetInfo retrieves the informations about the source code and commit
func (h *stiGit) GetInfo(repo string) *api.SourceInfo {
	git := func(arg ...string) string {
		command := exec.Command("git", arg...)
		command.Dir = repo
		out, err := command.CombinedOutput()
		if err != nil {
			glog.V(1).Infof("Error executing 'git %#v': %s (%v)", arg, out, err)
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	return &api.SourceInfo{
		Location: git("config", "--get", "remote.origin.url"),
		Ref:      git("rev-parse", "--abbrev-ref", "HEAD"),
		CommitID: git("rev-parse", "--short", "--verify", "HEAD"),
		Author:   git("--no-pager", "show", "-s", "--format=%an <%ae>", "HEAD"),
		Date:     git("--no-pager", "show", "-s", "--format=%ad", "HEAD"),
		Message:  git("--no-pager", "show", "-s", "--format=%<(80,trunc)%s", "HEAD"),
	}
}

func pipeToLog(reader io.Reader, log func(...interface{})) {
	scanner := bufio.NewReader(reader)
	for {
		if text, err := scanner.ReadString('\n'); err != nil {
			if err != io.ErrClosedPipe {
				glog.Errorf("Error reading stdout, %v", err)
			}
			break
		} else {
			log(text)
		}
	}
}
