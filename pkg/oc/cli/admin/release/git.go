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

var noSuchRepo = errors.New("location is not a git repo")

func (g *git) exec(command ...string) (string, error) {
	buf := &bytes.Buffer{}
	bufErr := &bytes.Buffer{}
	cmd := exec.Command("git", command...)
	cmd.Dir = g.path
	cmd.Stdout = buf
	cmd.Stderr = bufErr
	glog.V(5).Infof("Executing git: %v\n", cmd.Args)
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
	Hash    string
	Parents []string

	// Local Merkle-graph node information
	Subject       string
	CommitterDate time.Time

	// Extracted metadata
	PullRequest int
	Issues      []*issue
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

func mergeLogForRepo(g *git, from, to string) ([]*commit, error) {
	if from == to {
		return nil, nil
	}

	rePR, err := regexp.Compile(`^Merge pull request #(\d+) from`)
	if err != nil {
		return nil, err
	}
	reBug, err := regexp.Compile(`^Bug\s([0-9,\s]*)\s*(-|:)\s*`)
	if err != nil {
		return nil, err
	}

	args := []string{"log", "--merges", "--topo-order", "-z", "--pretty=format:%H %P%x1E%ct%x1E%s%x1E%b", "--reverse", fmt.Sprintf("%s..%s", from, to)}
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

	commits := []*commit{}
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

		body := records[3]
		if m := rePR.FindStringSubmatch(cmt.Subject); m != nil {
			cmt.PullRequest, err = strconv.Atoi(m[1])
			if err != nil {
				return nil, fmt.Errorf("could not extract PR number from %q: %v", cmt.Subject, err)
			}
		}

		cmt.Subject = strings.SplitN(strings.TrimSpace(body), "\n", 2)[0]

		if m := reBug.FindStringSubmatch(cmt.Subject); m != nil {
			for _, idString := range strings.Split(m[1], ",") {
				bugID, err := strconv.Atoi(strings.Trim(idString, " \t"))
				if err != nil {
					return nil, fmt.Errorf("could not extract bug number from %q: %v", cmt.Subject, err)
				}

				cmt.Issues = append(cmt.Issues, &issue{
					ID:  bugID,
					URI: fmt.Sprintf("https://bugzilla.redhat.com/show_bug.cgi?id=%d", bugID),
				})
			}
			cmt.Subject = cmt.Subject[len(m[0]):]
		}

		commits = append(commits, cmt)
	}

	for _, cmt := range commits {
		sort.Slice(cmt.Issues, func(i, j int) bool {
			if cmt.Issues[i].ID != cmt.Issues[j].ID {
				return cmt.Issues[i].ID < cmt.Issues[j].ID
			}

			return cmt.Issues[i].URI < cmt.Issues[j].URI
		})
	}

	return commits, nil
}
