package release

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
)

// git is a wrapper to invoke git safely, similar to
// github.com/openshift/library-go/pkg/git but giving access to lower level
// calls. Consider improving pkg/git in the future.
type git struct {
	path string
}

var (
	noSuchRepo = errors.New("location is not a git repo")
	rePR       = regexp.MustCompile(`^Merge pull request #(\d+) from`)
	reBug      = regexp.MustCompile(`^Bug (\d+)\s*(-|:)\s*`)
	reIssue    = regexp.MustCompile(`^Issue:\s*(\S*)\s*$`)
)

func (g *git) exec(command ...string) (string, error) {
	buf := &bytes.Buffer{}
	bufErr := &bytes.Buffer{}
	cmd := exec.Command("git", append([]string{"-C", g.path}, command...)...)
	glog.V(5).Infof("Executing git: %v\n", cmd.Args)
	cmd.Stdout = buf
	cmd.Stderr = bufErr
	err := cmd.Run()
	if err != nil {
		return bufErr.String(), err
	}
	return buf.String(), nil
}

func (g *git) streamExec(out, errOut io.Writer, command ...string) error {
	cmd := exec.Command("git", append([]string{"--git-dir", filepath.Join(g.path, ".git")}, command...)...)
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

func (g *git) ChangeContext(path string) (*git, error) {
	location := &git{path: path}
	if errOut, err := location.exec("rev-parse", "--git-dir"); err != nil {
		if strings.Contains(errOut, "not a git repository") {
			return location, noSuchRepo
		}
		return location, err
	}
	return location, nil
}

func (g *git) Clone(repository string, out, errOut io.Writer) error {
	return (&git{}).streamExec(out, errOut, "clone", repository, g.path)
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
	if _, err := g.exec("fetch", repo); err == nil {
		if _, err := g.exec("checkout", commit); err == nil {
			return nil
		}
	}

	// TODO: what if that transport URL does not exist?

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

type commit struct {
	// Merkle graph information
	Hash          string
	Parents       []string
	KnownChildren []string

	// Local Merkle-graph node information
	Subject       string
	CommitterDate time.Time

	// Extracted metadata
	PullRequest int
	Issues      []*issue
}

func (cmt *commit) process(body string) error {
	if len(cmt.Parents) > 1 { // merge commit
		if m := rePR.FindStringSubmatch(cmt.Subject); m != nil {
			var err error
			cmt.PullRequest, err = strconv.Atoi(m[1])
			if err != nil {
				return fmt.Errorf("could not extract PR number from %q: %v", cmt.Subject, err)
			}
		}

		cmt.Subject = strings.SplitN(strings.TrimSpace(body), "\n", 2)[0]
	}

	if m := reBug.FindStringSubmatch(cmt.Subject); m != nil {
		bug, err := strconv.Atoi(m[1])
		if err != nil {
			return fmt.Errorf("could not extract bug number from %q: %v", cmt.Subject, err)
		}
		cmt.Subject = cmt.Subject[len(m[0]):]
		cmt.Issues = append(cmt.Issues, &issue{
			Store: "rhbz",
			ID:    bug,
			URI:   fmt.Sprintf("https://bugzilla.redhat.com/show_bug.cgi?id=%d", bug),
		})
	}

	cmd := exec.Command("git", "interpret-trailers", "--parse")
	cmd.Stdin = bytes.NewBufferString(body)
	glog.V(5).Infof("Executing git: %v (on %s)\n", cmd.Args, cmt.Hash)
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(output), "\n") {
		if len(line) == 0 {
			continue
		}

		if m := reIssue.FindStringSubmatch(line); m != nil {
			issue, err := issueFromURI(m[1])
			if err != nil {
				return err
			}

			cmt.Issues = append(cmt.Issues, issue)
		}
	}

	return nil
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

func firstParentLogForRepo(g *git, from, to string) ([]*commit, error) {
	if from == to {
		return nil, nil
	}

	args := []string{"log", "--topo-order", "-z", "--format=%H %P%x1E%ct%x1E%s%x1E%b", fmt.Sprintf("%s..%s", from, to)}
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

	if glog.V(5) {
		glog.Infof("Got commit info:\n%s", strconv.Quote(out))
	}

	if len(out) == 0 {
		return nil, nil
	}

	var tip *commit
	commitMap := map[string]*commit{}
	for _, entry := range strings.Split(out, "\x00") {
		if entry == "" {
			continue
		}

		records := strings.Split(entry, "\x1e")
		if len(records) != 4 {
			return nil, fmt.Errorf("unexpected git log output width %d columns", len(records))
		}

		commitHashes := strings.Split(records[0], " ")
		unixTS, err := strconv.ParseInt(records[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unexpected timestamp: %v", err)
		}

		cmt := &commit{
			CommitterDate: time.Unix(unixTS, 0).UTC(),
			Hash:          commitHashes[0],
			Parents:       commitHashes[1:],
			Subject:       records[2],
		}
		for _, potentialChild := range commitMap {
			for _, hash := range potentialChild.Parents {
				if hash == cmt.Hash {
					cmt.KnownChildren = append(cmt.KnownChildren, potentialChild.Hash)
					break
				}
			}
		}

		commitMap[cmt.Hash] = cmt
		if tip == nil {
			tip = cmt
		}

		body := records[3]
		err = cmt.process(body)
		if err != nil {
			return nil, err
		}
	}

	exists := struct{}{}
	firstParents := map[string]struct{}{}
	commits := []*commit{}
	for cmt := tip; cmt != nil; cmt = commitMap[cmt.Parents[0]] { // [0] is ok, because there will be no orphans in a from..to log
		commits = append(commits, cmt)
		firstParents[cmt.Hash] = exists
	}

	// collect issue information from commits outside the first-parent chain
	for _, cmt := range commitMap {
		if len(cmt.Issues) == 0 {
			continue
		}

		if _, ok := firstParents[cmt.Hash]; ok {
			continue
		}

		// figure out which first-parent is closest
		var closestFirstParent *commit
		for generation := cmt.KnownChildren; closestFirstParent == nil && len(generation) > 0; {
			nextGeneration := []string{}
			for _, hash := range generation {
				child := commitMap[hash]
				if _, ok := firstParents[hash]; ok {
					closestFirstParent = child
					break
				}
				for _, grandChild := range child.KnownChildren {
					nextGeneration = append(nextGeneration, grandChild)
				}
			}

			generation = nextGeneration
		}

		if closestFirstParent == nil {
			return nil, fmt.Errorf("no first-parent found for %s", cmt.Hash)
		}

		for _, issue := range cmt.Issues {
			alreadyOnClosestFirstParent := false
			for _, mergeIssue := range closestFirstParent.Issues {
				if mergeIssue.URI == issue.URI {
					alreadyOnClosestFirstParent = true
					break
				}
			}

			if !alreadyOnClosestFirstParent {
				closestFirstParent.Issues = append(closestFirstParent.Issues, issue)
			}
		}
	}

	for _, cmt := range commits {
		sort.Slice(cmt.Issues, func(i, j int) bool {
			if cmt.Issues[i].Store != cmt.Issues[j].Store {
				return cmt.Issues[i].Store < cmt.Issues[j].Store
			}

			if cmt.Issues[i].ID != cmt.Issues[j].ID {
				return cmt.Issues[i].ID < cmt.Issues[j].ID
			}

			return cmt.Issues[i].URI < cmt.Issues[j].URI
		})
	}

	return commits, nil
}
