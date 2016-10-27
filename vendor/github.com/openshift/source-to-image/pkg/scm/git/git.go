package git

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/util"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"
)

var glog = utilglog.StderrLog

// Git is an interface used by main STI code to extract/checkout git repositories
type Git interface {
	ValidCloneSpec(source string) (bool, error)
	ValidCloneSpecRemoteOnly(source string) bool
	MungeNoProtocolURL(source string, url *url.URL) error
	Clone(source, target string, opts api.CloneConfig) error
	Checkout(repo, ref string) error
	SubmoduleUpdate(repo string, init, recursive bool) error
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

var gitSSHURLUser = regexp.MustCompile(`^([\w\-_\.]+)$`)
var gitSSHURLIPv4 = regexp.MustCompile(`^([\w\.\-_]+)$`)         // should cover textual hostnames and x.x.x.x
var gitSSHURLIPv6 = regexp.MustCompile(`^(\[[a-fA-F0-9\:]+\])$`) // should cover [hex:hex ... :hex]
var gitSSHURLPathRef = regexp.MustCompile(`^([\w\.\-_+\/\\]+)$`)

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
func (h *stiGit) ValidCloneSpec(source string) (bool, error) {
	details, _, err := ParseFile(source)
	if err != nil {
		return false, err
	}

	if details.FileExists && !details.BadRef {
		return true, nil
	}

	return h.ValidCloneSpecRemoteOnly(source), nil
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

	details, mods, err := ParseFile(source)
	if err != nil {
		return err
	}

	if details.BadRef {
		return fmt.Errorf("bad reference following # in %s", source)
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
		return nil, fmt.Errorf("url source %s mistakingly interpreted as protocol %s by golang", source, uri.Scheme)
	}

	// now see if an invalid protocol is specified
	if !stringInSlice(uri.Scheme, allowedSchemes) {
		return nil, fmt.Errorf("unsupported protocol specfied:  %s", uri.Scheme)
	}

	// have a valid protocol, return success
	return uri, nil
}

// mimic the path munging (make paths absolute to fix file:// with non-absolute paths) done when scm.go:DownloderForSource valided git file urls
func makePathAbsolute(source string) string {
	glog.V(4).Infof("makePathAbsolute %s", source)
	if !strings.HasPrefix(source, "/") {
		if absolutePath, err := filepath.Abs(source); err == nil {
			glog.V(4).Infof("makePathAbsolute new path %s err %v", absolutePath, err)
			return absolutePath
		}
	}
	return source
}

// ParseFile will see if the input string is a valid file location, where
// file names have a great deal of flexibility and can even match
// expect git clone spec syntax; it also provides details if the file://
// proto was explicitly specified, if we should use OS copy vs. the git
// binary, and if a frag/ref has a bad format
func ParseFile(source string) (*FileProtoDetails, *URLMods, error) {
	// Checking to see if the user included a "file://" in the call
	protoSpecified := false
	if strings.HasPrefix(source, "file://") && len(source) > 7 {
		protoSpecified = true
	}

	refSpecified := false
	path, ref := "", ""
	if strings.LastIndex(source, "#") != -1 {
		refSpecified = true

		segments := strings.SplitN(source, "#", 2)
		path = segments[0]
		ref = segments[1]
	} else {
		path = source
	}

	// in each valid case, like the prior logic in scm.go did, we'll make the
	// paths absolute and prepend file:// to the path which callers should
	// switch to
	if doesExist(path) {

		// Is there even a valid .git repository?
		isValidGit, err := isValidGitRepository(path)
		hasGit := false
		if isValidGit {
			hasGit = hasGitBinary()
		}

		if err != nil || !isValidGit || !hasGit {
			details := &FileProtoDetails{
				UseCopy:        true,
				FileExists:     true,
				BadRef:         false,
				ProtoSpecified: protoSpecified,
			}
			mods := &URLMods{
				Scheme: "file",
				Path:   makePathAbsolute(strings.TrimPrefix(path, "file://")),
				Ref:    ref,
			}
			return details, mods, err
		}

		// Check is the #ref is valid
		badRef := refSpecified && !gitSSHURLPathRef.MatchString(ref)

		details := &FileProtoDetails{
			BadRef:         badRef,
			FileExists:     true,
			ProtoSpecified: protoSpecified,
			// this value doesn't really matter, we should not proceed if the git ref is bad
			// but let's fallback to "copy" mode if the ref is invalid.
			UseCopy: badRef,
		}

		mods := &URLMods{
			Scheme: "file",
			Path:   makePathAbsolute(strings.TrimPrefix(path, "file://")),
			Ref:    ref,
		}
		return details, mods, nil
	}

	// File does not exist, return bad
	details := &FileProtoDetails{
		UseCopy:        false,
		FileExists:     false,
		BadRef:         false,
		ProtoSpecified: protoSpecified,
	}
	return details, nil, nil
}

// ParseSSH will see if the input string is a valid git clone spec
// which follows the rules for using the ssh protocol either with or without
// the ssh:// prefix
func ParseSSH(source string) (*URLMods, error) {
	// if not ssh protcol, return bad
	if strings.Index(source, "://") != -1 && !strings.HasPrefix(source, "ssh") {
		return nil, fmt.Errorf("not ssh protocol: %s", source)
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
			if !gitSSHURLUser.MatchString(user) {
				return nil, fmt.Errorf("invalid user name provided: %s from %s", user, source)
			}

			// because of ipv6, need to redo last index of :
			lastColonIdx = strings.LastIndex(host, ":")
			if lastColonIdx != -1 {
				path = host[lastColonIdx+1:]
				host = host[0:lastColonIdx]
			} else {
				return nil, fmt.Errorf("invalid git ssh clone spec, the @ precedes the last: %s", source)
			}
		} else {
			host = source[0:lastColonIdx]
			path = source[lastColonIdx+1:]
		}

		// bad host, either ipv6 or ipv4
		if !gitSSHURLIPv6.MatchString(host) && !gitSSHURLIPv4.MatchString(host) {
			return nil, fmt.Errorf("invalid host provided: %s from %s", host, source)
		}

		segments := strings.SplitN(path, "#", 2)
		if len(segments) == 2 {
			path, ref = segments[0], segments[1]

			// bad ref/frag
			if !gitSSHURLPathRef.MatchString(ref) {
				return nil, fmt.Errorf("invalid reference provided: %s from %s", ref, source)
			}
		}

		// bad path
		if !gitSSHURLPathRef.MatchString(path) {
			return nil, fmt.Errorf("invalid path provided: %s from %s", path, source)
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
	return nil, fmt.Errorf("unable to parse ssh git clone specification:  %s", source)
}

// followGitSubmodule looks at a .git /file/ and tries to retrieve from inside
// it the gitdir value, which is supposed to indicate the location of the
// corresponding .git /directory/.  Note: the gitdir value should point directly
// to the corresponding .git directory even in the case of nested submodules.
func followGitSubmodule(gitPath string) (string, error) {
	f, err := os.Open(gitPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if sc.Scan() {
		s := sc.Text()

		if strings.HasPrefix(s, "gitdir: ") {
			newGitPath := s[8:]

			if !filepath.IsAbs(newGitPath) {
				newGitPath = filepath.Join(filepath.Dir(gitPath), newGitPath)
			}

			fi, err := os.Stat(newGitPath)
			if err != nil && !os.IsNotExist(err) {
				return "", err
			}
			if os.IsNotExist(err) || !fi.IsDir() {
				return "", fmt.Errorf("gitdir link in .git file %q is invalid", gitPath)
			}
			return newGitPath, nil
		}
	}

	return "", fmt.Errorf("unable to parse .git file %q", gitPath)
}

// isValidGitRepository checks to see if there is a git repository in the
// directory and if the repository is valid -- i.e. it has remotes or commits
func isValidGitRepository(dir string) (bool, error) {
	gitPath := filepath.Join(strings.TrimPrefix(dir, "file://"), ".git")

	fi, err := os.Stat(gitPath)
	if os.IsNotExist(err) {
		// The directory is not a git repo, no error
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if !fi.IsDir() {
		gitPath, err = followGitSubmodule(gitPath)
		if err != nil {
			return false, err
		}
	}

	// Search the content of the .git directory for content
	directories := [2]string{
		filepath.Join(gitPath, "objects"),
		filepath.Join(gitPath, "refs"),
	}

	// For the directories we search, if the git repo has been used, there will
	// be some file.  We don't just search the base git repository because of the
	// hook samples that are normally generated with `git init`
	isEmpty := true
	for _, dir := range directories {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			// If we find a file, the git directory is "not empty"
			// We're looking for object blobs, and ref files
			if info != nil && !info.IsDir() {
				isEmpty = false
				return filepath.SkipDir
			}

			return err
		})

		if err != nil && err != filepath.SkipDir {
			// There is a .git, but we've encountered an error
			return true, err
		}

		if !isEmpty {
			return true, nil
		}
	}

	// Since we know there's a .git directory, but there is nothing in it, we
	// throw an error
	return true, errors.NewEmptyGitRepositoryError(dir)
}

// doesExist checks if the path exists, removing file:// if needed for OS FS check
func doesExist(dir string) bool {
	_, err := os.Stat(strings.TrimPrefix(dir, "file://"))
	return err == nil
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
	// --quiet does not suppress that anyway reduced the frequency of the hang, but it still occurred.
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
			glog.Errorf("Clone failed: source %s, target %s,  with output %s", source, target, out)
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

// SubmoduleInit initializes/clones submodules
func (h *stiGit) SubmoduleInit(repo string) error {
	opts := util.CommandOpts{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Dir:    repo,
	}
	return h.runner.RunWithOptions(opts, "git", "submodule", "init")
}

// SubmoduleUpdate checks out submodules to their correct version.
// Optionally also inits submodules, optionally operates recursively.
func (h *stiGit) SubmoduleUpdate(repo string, init, recursive bool) error {
	updateArgs := []string{"submodule", "update"}
	if init {
		updateArgs = append(updateArgs, "--init")
	}
	if recursive {
		updateArgs = append(updateArgs, "--recursive")
	}

	opts := util.CommandOpts{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Dir:    repo,
	}
	return h.runner.RunWithOptions(opts, "git", updateArgs...)
}

// GetInfo retrieves the information about the source code and commit
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
		Location:       git("config", "--get", "remote.origin.url"),
		Ref:            git("rev-parse", "--abbrev-ref", "HEAD"),
		CommitID:       git("rev-parse", "--verify", "HEAD"),
		AuthorName:     git("--no-pager", "show", "-s", "--format=%an", "HEAD"),
		AuthorEmail:    git("--no-pager", "show", "-s", "--format=%ae", "HEAD"),
		CommitterName:  git("--no-pager", "show", "-s", "--format=%cn", "HEAD"),
		CommitterEmail: git("--no-pager", "show", "-s", "--format=%ce", "HEAD"),
		Date:           git("--no-pager", "show", "-s", "--format=%ad", "HEAD"),
		Message:        git("--no-pager", "show", "-s", "--format=%<(80,trunc)%s", "HEAD"),
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
