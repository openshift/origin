package git

import (
	"bufio"
	"errors"
	"fmt"
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
	ValidCloneSpecRemoteOnly(source string) bool
	MungeNoProtocolURL(source string, url *url.URL) error
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

// URLMods encapsulates potential changes to similarly named fields in the URL struct defined in golang
// when a protocol is not explicitly specified in a clone spec (which is possible for certain file:// and ssh:// permutations)
type URLMods struct {
	Scheme string
	User   string
	Host   string
	Path   string
	// Corresponds to Fragment in the URL struct, but we use Ref since URL Fragments are used and git commit refs
	Ref string
}

// FileProtoDetails encapsulates certain determinations from examining a given clone spec under the assumption
// that it is a local file protocol clone spec
type FileProtoDetails struct {
	// Using the clone spec as a literal file path, does it actually exis
	FileExists bool
	// Use OS level file copy commands instead of the git binary
	UseCopy bool
	// Did the clone spec have the prefix of file://
	ProtoSpecified bool
	// Was the text for a fragment/ref that follows the last # incorrect
	BadRef bool
}

var gitSshURLUser = regexp.MustCompile(`^([\w\-_\.]+)$`)
var gitSshURLIPv4 = regexp.MustCompile(`^([\w\.\-_]+)$`)         // should cover textual hostnames and x.x.x.x
var gitSshURLIPv6 = regexp.MustCompile(`^(\[[a-fA-F0-9\:]+\])$`) // should cover [hex:hex ... :hex]
var gitSshURLPathRef = regexp.MustCompile(`^([\w\.\-_+\/\\]+)$`)

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
	details, _ := ParseFile(source)
	if details.FileExists && !details.BadRef {
		return true
	}

	return h.ValidCloneSpecRemoteOnly(source)
}

// ValidCloneSpecRemoteOnly determines if the given string reference points to a valid remote git
// repository; valid local repository specs will result in a return of false
func (h *stiGit) ValidCloneSpecRemoteOnly(source string) bool {
	_, err := ParseSSH(source)
	if err == nil {
		return true
	}

	url, err := ParseURL(source)
	if url != nil && err == nil && url.Scheme != "file" {
		return true
	}

	return false
}

// MungeNoProtocolURL will take a URL returned from net/url.Parse and make
// corrections to the URL when no protocol is specified in the Scheme, where there
// are valid protocol-less git url spec formats that result in either file:// or ssh:// protocol usage;
// if an explicit protocol is already specified, then the
// URL is left unchaged and the method simply returns with no error reported,
// since the URL is
func (h *stiGit) MungeNoProtocolURL(source string, uri *url.URL) error {
	if uri == nil {
		return nil
	}

	// only deal with protocol-less url's
	if uri.Scheme != "" {
		return nil
	}

	details, mods := ParseFile(source)

	if details.BadRef {
		return errors.New(fmt.Sprintf("bad reference following # in %s", source))
	}
	if !details.FileExists {
		mods2, err := ParseSSH(source)
		if err != nil {
			glog.Errorf("ssh git clone spec error: %v", err)
			return err
		}
		mods = mods2
	}

	// update the either file or ssh url accordingly
	if mods != nil {
		if len(mods.User) > 0 {
			uri.User = url.User(mods.User)
		}
		if len(mods.Scheme) > 0 {
			uri.Scheme = mods.Scheme
		}
		if len(mods.Host) > 0 {
			uri.Host = mods.Host
		}
		if len(mods.Path) > 0 {
			uri.Path = mods.Path
		}
		if len(mods.Ref) > 0 {
			uri.Fragment = mods.Ref
		}
	}
	return nil
}

// ParseURL sees if the spec is properly handled by golang's URL parser,
// with a valid, explicit protocol specified,
// and deals with some of the known issues we have seen with the logic
func ParseURL(source string) (*url.URL, error) {
	uri, err := url.Parse(source)
	if err != nil {
		return nil, err
	}

	// to work around the url.Parse bug, deal with the fact it seemed to split on just :
	if strings.Index(source, "://") == -1 && uri.Scheme != "" {
		uri.Scheme = ""
		return nil, errors.New(fmt.Sprintf("url source %s mistakingly interpreted as protocol %s by golang", source, uri.Scheme))
	}

	// now see if an invalid protocol is specified
	if !stringInSlice(uri.Scheme, allowedSchemes) {
		return nil, errors.New(fmt.Sprintf("unsupported protocol specfied:  %s", uri.Scheme))
	}

	// have a valid protocol, return sucess
	return uri, nil
}

// ParseFile will see if the input string is a valid file location, where
// file names have a great deal of flexibility and can even match
// expect git clone spec syntax; it also provides details if the file://
// proto was explicitly specified, if we should use OS copy vs. the git
// binary, and if a frag/ref has a bad format
func ParseFile(source string) (details *FileProtoDetails, mods *URLMods) {
	protoSpecified := false
	if strings.HasPrefix(source, "file://") && len(source) > 7 {
		protoSpecified = true
	}

	// if source, minus potential file:// prefix, exists as is, denote
	// and return
	if doesExist(source) {
		details = &FileProtoDetails{
			FileExists:     true,
			UseCopy:        !isLocalGitRepository(source) || !hasGitBinary(),
			ProtoSpecified: protoSpecified,
			BadRef:         false,
		}
		mods = &URLMods{
			Scheme: "file",
			Path:   strings.TrimPrefix(source, "file://"),
		}
		return
	}

	// need to see if this was a file://<valid file>#ref
	if protoSpecified && strings.LastIndex(source, "#") != -1 {
		segments := strings.SplitN(source, "#", 2)
		// given last index check above, segement should
		// be of len 2
		path, ref := segments[0], segments[1]
		// file does not exist, return bad
		if !doesExist(path) {
			details = &FileProtoDetails{
				UseCopy:        false,
				FileExists:     false,
				ProtoSpecified: protoSpecified,
				BadRef:         false,
			}
			mods = nil
			return
		}
		// if ref/frag bad, return bad
		if !gitSshURLPathRef.MatchString(ref) {
			details = &FileProtoDetails{
				UseCopy:        false,
				FileExists:     true,
				ProtoSpecified: protoSpecified,
				BadRef:         true,
			}
			mods = nil
			return
		}

		// return good
		details = &FileProtoDetails{
			UseCopy:        !isLocalGitRepository(source) || !hasGitBinary(),
			FileExists:     true,
			ProtoSpecified: protoSpecified,
			BadRef:         false,
		}
		mods = &URLMods{
			Scheme: "file",
			Path:   path,
			Ref:    ref,
		}
		return
	}

	// general return bad
	details = &FileProtoDetails{
		UseCopy:        false,
		FileExists:     false,
		BadRef:         false,
		ProtoSpecified: protoSpecified,
	}
	return
}

// ParseSSH will see if the input string is a valid git clone spec
// which follows the rules for using the ssh protocol either with or without
// the ssh:// prefix
func ParseSSH(source string) (*URLMods, error) {
	// if not ssh protcol, return bad
	if strings.Index(source, "://") != -1 && !strings.HasPrefix(source, "ssh") {
		return nil, errors.New(fmt.Sprintf("not ssh protocol: %s", source))
	}

	lastColonIdx := strings.LastIndex(source, ":")
	atSignPresent := strings.Index(source, "@") != -1
	if lastColonIdx != -1 {
		host, path, user, ref := "", "", "", ""

		if atSignPresent {
			segments := strings.SplitN(source, "@", 2)
			// with index check above, can assume there are 2 segments
			user, host = segments[0], segments[1]

			// bad user, return
			if !gitSshURLUser.MatchString(user) {
				return nil, errors.New(fmt.Sprintf("invalid user name provided: %s from %s", user, source))
			}

			// because of ipv6, need to redo last index of :
			lastColonIdx = strings.LastIndex(host, ":")
			if lastColonIdx != -1 {
				path = host[lastColonIdx+1:]
				host = host[0:lastColonIdx]
			} else {
				return nil, errors.New(fmt.Sprintf("invalid git ssh clone spec, the @ precedes the last: %s", source))
			}
		} else {
			host = source[0:lastColonIdx]
			path = source[lastColonIdx+1:]
		}

		// bad host, either ipv6 or ipv4
		if !gitSshURLIPv6.MatchString(host) && !gitSshURLIPv4.MatchString(host) {
			return nil, errors.New(fmt.Sprintf("invalid host provided: %s from %s", host, source))
		}

		segments := strings.SplitN(path, "#", 2)
		if len(segments) == 2 {
			path, ref = segments[0], segments[1]

			// bad ref/frag
			if !gitSshURLPathRef.MatchString(ref) {
				return nil, errors.New(fmt.Sprintf("invalid reference provided: %s from %s", ref, source))
			}
		}

		// bad path
		if !gitSshURLPathRef.MatchString(path) {
			return nil, errors.New(fmt.Sprintf("invalid path provided: %s from ", path, source))
		}

		// return good
		return &URLMods{
			Scheme: "ssh",
			User:   user,
			Host:   host,
			Path:   path,
			Ref:    ref,
		}, nil
	}
	return nil, errors.New(fmt.Sprintf("unable to parse ssh git clone specification:  %s", source))
}

// isLocalGitRepository checks if the specified directory has .git subdirectory (it
// is a GIT repository)
func isLocalGitRepository(dir string) bool {
	_, err := os.Stat(fmt.Sprintf("%s/.git", strings.TrimPrefix(dir, "file://")))
	return !(err != nil && os.IsNotExist(err))
}

// doesExist checks if the path exists, removing file:// if needed for OS FS check
func doesExist(dir string) bool {
	_, err := os.Stat(strings.TrimPrefix(dir, "file://"))
	return !(err != nil && os.IsNotExist(err))
}

// hasGitBinary checks if the 'git' binary is available on the system
func hasGitBinary() bool {
	_, err := exec.LookPath("git")
	return err == nil
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
