package git

import (
	"bufio"
	"io"
	"io/ioutil"
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
	Clone(source, target string, opts api.CloneConfig) error
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

var allowedSchemes = []string{"git", "http", "https", "file", "ssh"}

func cloneConfigToArgs(opts api.CloneConfig) []string {
	result := []string{}
	if opts.Quiet {
		result = append(result, "--quiet")
	}
	if opts.Recursive {
		result = append(result, "--recursive")
	}
	return result
}

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
func (h *stiGit) Clone(source, target string, c api.CloneConfig) error {

	// NOTE, we don NOT pass in both stdout and stderr, because
	// with running with --quiet, and no output heading to stdout, hangs were occurring with the coordination
	// of underlying channel management in the Go layer when dealing with the Go Cmd wrapper around
	// git, sending of stdout/stderr to the Pipes created here, and the glog routines sent to pipeToLog
	//
	// It was agreed that we wanted to keep --quiet and no stdout output ....leaving stderr only since
	// --quiet does not surpress that anyway reduced the frequency of the hang, but it still occurred.
	// the pipeToLog method has been left for now for historical purposes, but if this implemenetation
	// of git clone holds, we'll want to delete that at some point.

	cloneArgs := append([]string{"clone"}, cloneConfigToArgs(c)...)
	cloneArgs = append(cloneArgs, []string{source, target}...)
	errReader, errWriter, _ := os.Pipe()
	opts := util.CommandOpts{Stderr: errWriter}
	err := h.runner.RunWithOptions(opts, "git", cloneArgs...)
	errWriter.Close()
	if err != nil {
		out, _ := ioutil.ReadAll(errReader)
		// If we captured errors via stderr, print them out.
		if len(out) > 0 {
			glog.Errorf("Clone failed: %s", out)
		}
		return err
	}
	return nil
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
		CommitID: git("rev-parse", "--verify", "HEAD"),
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
