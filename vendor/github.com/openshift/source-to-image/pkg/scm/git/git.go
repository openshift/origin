package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	log "github.com/golang/glog"

	"github.com/openshift/source-to-image/pkg/util/cmd"
	"github.com/openshift/source-to-image/pkg/util/cygpath"
	"github.com/openshift/source-to-image/pkg/util/fs"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"
)

var glog = utilglog.StderrLog
var lsTreeRegexp = regexp.MustCompile("([0-7]{6}) [^ ]+ [0-9a-f]{40}\t(.*)")

// Git is an interface used by main STI code to extract/checkout git repositories
type Git interface {
	Clone(source *URL, target string, opts CloneConfig) error
	Checkout(repo, ref string) error
	SubmoduleUpdate(repo string, init, recursive bool) error
	LsTree(repo, ref string, recursive bool) ([]os.FileInfo, error)
	GetInfo(string) *SourceInfo
}

// New returns a new instance of the default implementation of the Git interface
func New(fs fs.FileSystem, runner cmd.CommandRunner) Git {
	return &stiGit{
		FileSystem:    fs,
		CommandRunner: runner,
	}
}

type stiGit struct {
	fs.FileSystem
	cmd.CommandRunner
}

func cloneConfigToArgs(opts CloneConfig) []string {
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

// followGitSubmodule looks at a .git /file/ and tries to retrieve from inside
// it the gitdir value, which is supposed to indicate the location of the
// corresponding .git /directory/.  Note: the gitdir value should point directly
// to the corresponding .git directory even in the case of nested submodules.
func followGitSubmodule(fs fs.FileSystem, gitPath string) (string, error) {
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

			fi, err := fs.Stat(newGitPath)
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

// IsLocalNonBareGitRepository returns true if dir hosts a non-bare git
// repository, i.e. it contains a ".git" subdirectory or file (submodule case).
func IsLocalNonBareGitRepository(fs fs.FileSystem, dir string) (bool, error) {
	_, err := fs.Stat(filepath.Join(dir, ".git"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// LocalNonBareGitRepositoryIsEmpty returns true if the non-bare git repository
// at dir has no refs or objects.  It also handles the case of dir being a
// checked out git submodule.
func LocalNonBareGitRepositoryIsEmpty(fs fs.FileSystem, dir string) (bool, error) {
	gitPath := filepath.Join(dir, ".git")

	fi, err := fs.Stat(gitPath)
	if err != nil {
		return false, err
	}

	if !fi.IsDir() {
		gitPath, err = followGitSubmodule(fs, gitPath)
		if err != nil {
			return false, err
		}
	}

	// Search for any file in .git/{objects,refs}.  We don't just search the
	// base .git directory because of the hook samples that are normally
	// generated with `git init`
	found := false
	for _, dir := range []string{"objects", "refs"} {
		err := fs.Walk(filepath.Join(gitPath, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				found = true
			}

			if found {
				return filepath.SkipDir
			}

			return nil
		})

		if err != nil {
			return false, err
		}

		if found {
			return false, nil
		}
	}

	return true, nil
}

// HasGitBinary checks if the 'git' binary is available on the system
func HasGitBinary() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// Clone clones a git repository to a specific target directory.
func (h *stiGit) Clone(src *URL, target string, c CloneConfig) error {
	var err error

	source := *src

	if cygpath.UsingCygwinGit {
		if source.IsLocal() {
			source.URL.Path, err = cygpath.ToSlashCygwin(source.LocalPath())
			if err != nil {
				return err
			}
		}

		target, err = cygpath.ToSlashCygwin(target)
		if err != nil {
			return err
		}
	}

	cloneArgs := append([]string{"clone"}, cloneConfigToArgs(c)...)
	cloneArgs = append(cloneArgs, []string{source.StringNoFragment(), target}...)
	stderr := &bytes.Buffer{}
	opts := cmd.CommandOpts{Stderr: stderr}
	err = h.RunWithOptions(opts, "git", cloneArgs...)
	if err != nil {
		glog.Errorf("Clone failed: source %s, target %s, with output %q", source, target, stderr.String())
		return err
	}
	return nil
}

// Checkout checks out a specific branch reference of a given git repository
func (h *stiGit) Checkout(repo, ref string) error {
	opts := cmd.CommandOpts{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Dir:    repo,
	}
	if log.V(1) {
		return h.RunWithOptions(opts, "git", "checkout", ref)
	}
	return h.RunWithOptions(opts, "git", "checkout", "--quiet", ref)
}

// SubmoduleInit initializes/clones submodules
func (h *stiGit) SubmoduleInit(repo string) error {
	opts := cmd.CommandOpts{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Dir:    repo,
	}
	return h.RunWithOptions(opts, "git", "submodule", "init")
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

	opts := cmd.CommandOpts{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Dir:    repo,
	}
	return h.RunWithOptions(opts, "git", updateArgs...)
}

// LsTree returns a slice of os.FileInfo objects populated with the paths and
// file modes of files known to Git.  This is used on Windows systems where the
// executable mode metadata is lost on git checkout.
func (h *stiGit) LsTree(repo, ref string, recursive bool) ([]os.FileInfo, error) {
	args := []string{"ls-tree", ref}
	if recursive {
		args = append(args, "-r")
	}

	opts := cmd.CommandOpts{
		Dir: repo,
	}

	r, err := h.StartWithStdoutPipe(opts, "git", args...)
	if err != nil {
		return nil, err
	}

	submodules := []string{}
	rv := []os.FileInfo{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		m := lsTreeRegexp.FindStringSubmatch(text)
		if m == nil {
			return nil, fmt.Errorf("unparsable response %q from git ls-files", text)
		}
		mode, _ := strconv.ParseInt(m[1], 8, 0)
		path := m[2]
		if recursive && mode == 0160000 { // S_IFGITLINK
			submodules = append(submodules, filepath.Join(repo, path))
			continue
		}
		rv = append(rv, &fs.FileInfo{FileMode: os.FileMode(mode), FileName: path})
	}
	err = scanner.Err()
	if err != nil {
		h.Wait()
		return nil, err
	}

	err = h.Wait()
	if err != nil {
		return nil, err
	}

	for _, submodule := range submodules {
		rrv, err := h.LsTree(submodule, "HEAD", recursive)
		if err != nil {
			return nil, err
		}
		rv = append(rv, rrv...)
	}

	return rv, nil
}

// GetInfo retrieves the information about the source code and commit
func (h *stiGit) GetInfo(repo string) *SourceInfo {
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
	return &SourceInfo{
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
