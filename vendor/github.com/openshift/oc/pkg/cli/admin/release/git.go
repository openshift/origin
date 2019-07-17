package release

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog"
)

// git is a wrapper to invoke git safely, similar to
// github.com/openshift/library-go/pkg/git but giving access to lower level
// calls. Consider improving pkg/git in the future.
type git struct {
	path string
}

var noSuchRepo = errors.New("location is not a git repo")

func (g *git) exec(command ...string) (string, error) {
	buf := &bytes.Buffer{}
	bufErr := &bytes.Buffer{}
	cmd := exec.Command("git", command...)
	cmd.Dir = g.path
	cmd.Stdout = buf
	cmd.Stderr = bufErr
	klog.V(5).Infof("Executing git: %v\n", cmd.Args)
	err := cmd.Run()
	if err != nil {
		return bufErr.String(), err
	}
	return buf.String(), nil
}

func (g *git) streamExec(out, errOut io.Writer, command ...string) error {
	cmd := exec.Command("git", command...)
	cmd.Dir = g.path
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

func (g *git) ChangeContext(path string) (*git, error) {
	location := &git{path: path}
	if errOut, err := location.exec("rev-parse", "--git-dir"); err != nil {
		if strings.Contains(strings.ToLower(errOut), "not a git repository") {
			return location, noSuchRepo
		}
		return location, err
	}
	return location, nil
}

func (g *git) Clone(repository string, out, errOut io.Writer) error {
	cmd := exec.Command("git", "clone", repository, g.path)
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

func (g *git) parent() *git {
	return &git{path: filepath.Dir(g.path)}
}

func (g *git) basename() string {
	return filepath.Base(g.path)
}

func (g *git) CheckoutCommit(repo, commit string) error {
	_, err := g.exec("checkout", commit)
	if err == nil {
		return nil
	}

	// try to fetch by URL
	klog.V(4).Infof("failed to checkout: %v", err)
	if err := ensureFetchedRemoteForRepo(g, repo); err == nil {
		if _, err := g.exec("checkout", commit); err == nil {
			return nil
		}
	} else {
		klog.V(4).Infof("failed to fetch: %v", err)
	}

	return fmt.Errorf("could not locate commit %s", commit)
}

var reMatch = regexp.MustCompile(`^([a-zA-Z0-9\-\_]+)@([^:]+):(.+)$`)

func sourceLocationAsURL(location string) (*url.URL, error) {
	if matches := reMatch.FindStringSubmatch(location); matches != nil {
		return &url.URL{Scheme: "git", User: url.UserPassword(matches[1], ""), Host: matches[2], Path: matches[3]}, nil
	}
	return url.Parse(location)
}

func sourceLocationAsRelativePath(dir, location string) (string, error) {
	u, err := sourceLocationAsURL(location)
	if err != nil {
		return "", err
	}
	gitPath := u.Path
	if strings.HasSuffix(gitPath, ".git") {
		gitPath = strings.TrimSuffix(gitPath, ".git")
	}
	gitPath = path.Clean(gitPath)
	basePath := filepath.Join(dir, u.Host, filepath.FromSlash(gitPath))
	return basePath, nil
}

type MergeCommit struct {
	CommitDate time.Time

	Commit        string
	ParentCommits []string

	PullRequest int
	Bug         int

	Subject string
}

func gitOutputToError(err error, out string) error {
	out = strings.TrimSpace(out)
	if strings.HasPrefix(out, "fatal: ") {
		out = strings.TrimPrefix(out, "fatal: ")
	}
	if len(out) == 0 {
		return err
	}
	return fmt.Errorf(out)
}

func mergeLogForRepo(g *git, repo string, from, to string) ([]MergeCommit, error) {
	if from == to {
		return nil, nil
	}

	rePR, err := regexp.Compile(`^Merge pull request #(\d+) from`)
	if err != nil {
		return nil, err
	}
	reBug, err := regexp.Compile(`^Bug (\d+)\s*(-|:)\s*`)
	if err != nil {
		return nil, err
	}

	args := []string{"log", "--merges", "--topo-order", "-z", "--pretty=format:%H %P%x1E%ct%x1E%s%x1E%b", fmt.Sprintf("%s..%s", from, to)}
	out, err := g.exec(args...)
	if err != nil {
		// retry once if there's a chance we haven't fetched the latest commits
		if !strings.Contains(out, "Invalid revision range") {
			return nil, gitOutputToError(err, out)
		}
		if _, err := g.exec("fetch", "--all"); err != nil {
			return nil, gitOutputToError(err, out)
		}
		if _, err := g.exec("cat-file", "-e", from+"^{commit}"); err != nil {
			return nil, fmt.Errorf("from commit %s does not exist", from)
		}
		if _, err := g.exec("cat-file", "-e", to+"^{commit}"); err != nil {
			return nil, fmt.Errorf("to commit %s does not exist", to)
		}
		out, err = g.exec(args...)
		if err != nil {
			return nil, gitOutputToError(err, out)
		}
	}

	if klog.V(5) {
		klog.Infof("Got commit info:\n%s", strconv.Quote(out))
	}

	var commits []MergeCommit
	if len(out) == 0 {
		return nil, nil
	}
	for _, entry := range strings.Split(out, "\x00") {
		records := strings.Split(entry, "\x1e")
		if len(records) != 4 {
			return nil, fmt.Errorf("unexpected git log output width %d columns", len(records))
		}
		unixTS, err := strconv.ParseInt(records[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unexpected timestamp: %v", err)
		}
		commitValues := strings.Split(records[0], " ")

		mergeCommit := MergeCommit{
			CommitDate:    time.Unix(unixTS, 0).UTC(),
			Commit:        commitValues[0],
			ParentCommits: commitValues[1:],
		}

		msg := records[3]
		if m := reBug.FindStringSubmatch(msg); m != nil {
			mergeCommit.Subject = msg[len(m[0]):]
			mergeCommit.Bug, err = strconv.Atoi(m[1])
			if err != nil {
				return nil, fmt.Errorf("could not extract bug number from %q: %v", msg, err)
			}
		} else {
			mergeCommit.Subject = msg
		}
		mergeCommit.Subject = strings.TrimSpace(mergeCommit.Subject)
		mergeCommit.Subject = strings.SplitN(mergeCommit.Subject, "\n", 2)[0]

		mergeMsg := records[2]
		if m := rePR.FindStringSubmatch(mergeMsg); m != nil {
			mergeCommit.PullRequest, err = strconv.Atoi(m[1])
			if err != nil {
				return nil, fmt.Errorf("could not extract PR number from %q: %v", mergeMsg, err)
			}
		} else {
			klog.V(2).Infof("Omitted commit %s which has no pull-request", mergeCommit.Commit)
			continue
		}
		if len(mergeCommit.Subject) == 0 {
			mergeCommit.Subject = "Merge"
		}

		commits = append(commits, mergeCommit)
	}

	return commits, nil
}

// ensureCloneForRepo ensures that the repo exists on disk, is cloned, and has remotes for
// both repo and alternateRepos defined. The remotes for alternateRepos will be file system
// relative to avoid cloning repos twice.
func ensureCloneForRepo(dir string, repo string, alternateRepos []string, out, errOut io.Writer) (*git, error) {
	basePath, err := sourceLocationAsRelativePath(dir, repo)
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("Ensure repo is cloned at %s pointing to %s", basePath, repo)
	fi, err := os.Stat(basePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.MkdirAll(basePath, 0777); err != nil {
			return nil, err
		}
	} else {
		if !fi.IsDir() {
			return nil, fmt.Errorf("repo path %s is not a directory", basePath)
		}
	}
	cloner := &git{}
	extractedRepo, err := cloner.ChangeContext(basePath)
	if err != nil {
		if err != noSuchRepo {
			return nil, err
		}
		klog.V(2).Infof("Cloning %s ...", repo)
		if err := extractedRepo.Clone(repo, out, errOut); err != nil {
			return nil, err
		}
	} else {
		if err := ensureRemoteForRepo(extractedRepo, repo); err != nil {
			return nil, err
		}
	}

	for _, altRepo := range alternateRepos {
		if altRepo == repo {
			continue
		}
		if err := ensureRemoteForRepo(extractedRepo, altRepo); err != nil {
			return nil, err
		}
	}

	return extractedRepo, nil
}

func remoteNameForRepo(repo string) string {
	sum := md5.Sum([]byte(repo))
	repoName := fmt.Sprintf("up-%s", base64.RawURLEncoding.EncodeToString(sum[:])[:10])
	return repoName
}

func ensureRemoteForRepo(g *git, repo string) error {
	repoName := remoteNameForRepo(repo)
	if out, err := g.exec("remote", "add", repoName, repo); err != nil && !strings.Contains(out, "already exists") {
		return gitOutputToError(err, out)
	}
	return nil
}

func ensureFetchedRemoteForRepo(g *git, repo string) error {
	repoName := remoteNameForRepo(repo)
	if out, err := g.exec("remote", "add", repoName, repo); err != nil && !strings.Contains(out, "already exists") {
		return gitOutputToError(err, out)
	}
	if out, err := g.exec("fetch", repoName); err != nil {
		return gitOutputToError(err, out)
	}
	return nil
}
