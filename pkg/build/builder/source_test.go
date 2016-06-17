package builder

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/generate/git"
)

func TestCheckRemoteGit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	gitRepo := git.NewRepositoryWithEnv([]string{"GIT_ASKPASS=true"})

	var err error
	err = checkRemoteGit(gitRepo, server.URL, 10*time.Second)
	switch v := err.(type) {
	case gitAuthError:
	default:
		t.Errorf("expected gitAuthError, got %q", v)
	}

	err = checkRemoteGit(gitRepo, "https://github.com/openshift/origin", 10*time.Second)
	if err != nil {
		t.Errorf("unexpected error %q", err)
	}
}

type testGitRepo struct {
	Name      string
	Path      string
	Files     []string
	Submodule *testGitRepo
}

func initializeTestGitRepo(name string) (*testGitRepo, error) {
	repo := &testGitRepo{Name: name}
	dir, err := ioutil.TempDir("", "test-"+repo.Name)
	if err != nil {
		return repo, err
	}
	repo.Path = dir
	tmpfn := filepath.Join(dir, "initial-file")
	if err := ioutil.WriteFile(tmpfn, []byte("test"), 0666); err != nil {
		return repo, fmt.Errorf("unable to create temporary file")
	}
	repo.Files = append(repo.Files, tmpfn)
	initCmd := exec.Command("git", "init")
	initCmd.Dir = dir
	if out, err := initCmd.CombinedOutput(); err != nil {
		return repo, fmt.Errorf("unable to initialize repository: %q", out)
	}

	configEmailCmd := exec.Command("git", "config", "user.email", "me@example.com")
	configEmailCmd.Dir = dir
	if out, err := configEmailCmd.CombinedOutput(); err != nil {
		return repo, fmt.Errorf("unable to set git email prefs: %q", out)
	}
	configNameCmd := exec.Command("git", "config", "user.name", "Me Myself")
	configNameCmd.Dir = dir
	if out, err := configNameCmd.CombinedOutput(); err != nil {
		return repo, fmt.Errorf("unable to set git name prefs: %q", out)
	}

	return repo, nil
}

func (r *testGitRepo) addSubmodule() error {
	subRepo, err := initializeTestGitRepo("submodule")
	if err != nil {
		return err
	}
	if err := subRepo.addCommit(); err != nil {
		return err
	}
	subCmd := exec.Command("git", "submodule", "add", "file://"+subRepo.Path, "sub")
	subCmd.Dir = r.Path
	if out, err := subCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("unable to add submodule: %q", out)
	}
	r.Submodule = subRepo
	return nil
}

// getRef returns the sha256 of the commit specified by the negative offset.
// The '0' is the current HEAD.
func (r *testGitRepo) getRef(offset int) (string, error) {
	q := ""
	for i := offset; i != 0; i++ {
		q += "^"
	}
	refCmd := exec.Command("git", "rev-parse", "HEAD"+q)
	refCmd.Dir = r.Path
	if out, err := refCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("unable to checkout %d offset: %q", offset, out)
	} else {
		return strings.TrimSpace(string(out)), nil
	}
}

func (r *testGitRepo) createBranch(name string) error {
	refCmd := exec.Command("git", "checkout", "-b", name)
	refCmd.Dir = r.Path
	if out, err := refCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("unable to checkout new branch: %q", out)
	}
	return nil
}

func (r *testGitRepo) switchBranch(name string) error {
	refCmd := exec.Command("git", "checkout", name)
	refCmd.Dir = r.Path
	if out, err := refCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("unable to checkout branch: %q", out)
	}
	return nil
}

func (r *testGitRepo) cleanup() {
	os.RemoveAll(r.Path)
	if r.Submodule != nil {
		os.RemoveAll(r.Submodule.Path)
	}
}

func (r *testGitRepo) addCommit() error {
	f, err := ioutil.TempFile(r.Path, "")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(f.Name(), []byte("test"), 0666); err != nil {
		return fmt.Errorf("unable to create temporary file %q", f.Name())
	}
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = r.Path
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("unable to add files to repo: %q", out)
	}
	commitCmd := exec.Command("git", "commit", "-a", "-m", "test commit")
	commitCmd.Dir = r.Path
	out, err := commitCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to commit: %q", out)
	}
	r.Files = append(r.Files, f.Name())
	return nil
}

func TestUnqualifiedClone(t *testing.T) {
	repo, err := initializeTestGitRepo("unqualified")
	defer repo.cleanup()
	if err != nil {
		t.Errorf("%v", err)
	}
	if err := repo.addSubmodule(); err != nil {
		t.Errorf("%v", err)
	}
	// add two commits to check that shallow clone take account
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	destDir, err := ioutil.TempDir("", "clone-dest-")
	defer os.RemoveAll(destDir)
	client := git.NewRepositoryWithEnv([]string{})
	source := &api.GitBuildSource{URI: "file://" + repo.Path}
	revision := api.SourceRevision{Git: &api.GitSourceRevision{}}
	if _, err = extractGitSource(client, source, &revision, destDir, 10*time.Second); err != nil {
		t.Errorf("%v", err)
	}
	for _, f := range repo.Files {
		if _, err := os.Stat(filepath.Join(destDir, path.Base(f))); os.IsNotExist(err) {
			t.Errorf("unable to find repository file %q", path.Base(f))
		}
	}
	if _, err := os.Stat(filepath.Join(destDir, "sub")); os.IsNotExist(err) {
		t.Errorf("unable to find submodule dir")
	}
	for _, f := range repo.Submodule.Files {
		if _, err := os.Stat(filepath.Join(destDir, "sub/"+path.Base(f))); os.IsNotExist(err) {
			t.Errorf("unable to find submodule repository file %q", path.Base(f))
		}
	}
}

func TestCloneFromRef(t *testing.T) {
	repo, err := initializeTestGitRepo("commit")
	defer repo.cleanup()
	if err != nil {
		t.Errorf("%v", err)
	}
	if err := repo.addSubmodule(); err != nil {
		t.Errorf("%v", err)
	}
	// add two commits to check that shallow clone take account
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	destDir, err := ioutil.TempDir("", "commit-dest-")
	defer os.RemoveAll(destDir)
	client := git.NewRepositoryWithEnv([]string{})
	firstCommitRef, err := repo.getRef(-1)
	if err != nil {
		t.Errorf("%v", err)
	}
	source := &api.GitBuildSource{
		URI: "file://" + repo.Path,
		Ref: firstCommitRef,
	}
	revision := api.SourceRevision{Git: &api.GitSourceRevision{}}
	if _, err = extractGitSource(client, source, &revision, destDir, 10*time.Second); err != nil {
		t.Errorf("%v", err)
	}
	for _, f := range repo.Files[:len(repo.Files)-1] {
		if _, err := os.Stat(filepath.Join(destDir, path.Base(f))); os.IsNotExist(err) {
			t.Errorf("unable to find repository file %q", path.Base(f))
		}
	}
	if _, err := os.Stat(filepath.Join(destDir, path.Base(repo.Files[len(repo.Files)-1]))); !os.IsNotExist(err) {
		t.Errorf("last file should not exists in this checkout")
	}
	if _, err := os.Stat(filepath.Join(destDir, "sub")); os.IsNotExist(err) {
		t.Errorf("unable to find submodule dir")
	}
	for _, f := range repo.Submodule.Files {
		if _, err := os.Stat(filepath.Join(destDir, "sub/"+path.Base(f))); os.IsNotExist(err) {
			t.Errorf("unable to find submodule repository file %q", path.Base(f))
		}
	}
}

func TestCloneFromBranch(t *testing.T) {
	repo, err := initializeTestGitRepo("branch")
	defer repo.cleanup()
	if err != nil {
		t.Errorf("%v", err)
	}
	if err := repo.addSubmodule(); err != nil {
		t.Errorf("%v", err)
	}
	// add two commits to check that shallow clone take account
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	if err := repo.createBranch("test"); err != nil {
		t.Errorf("%v", err)
	}
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	if err := repo.switchBranch("master"); err != nil {
		t.Errorf("%v", err)
	}
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	destDir, err := ioutil.TempDir("", "branch-dest-")
	defer os.RemoveAll(destDir)
	client := git.NewRepositoryWithEnv([]string{})
	source := &api.GitBuildSource{
		URI: "file://" + repo.Path,
		Ref: "test",
	}
	revision := api.SourceRevision{Git: &api.GitSourceRevision{}}
	if _, err = extractGitSource(client, source, &revision, destDir, 10*time.Second); err != nil {
		t.Errorf("%v", err)
	}
	for _, f := range repo.Files[:len(repo.Files)-1] {
		if _, err := os.Stat(filepath.Join(destDir, path.Base(f))); os.IsNotExist(err) {
			t.Errorf("file %q should not exists in the test branch", f)
		}
	}
	if _, err := os.Stat(filepath.Join(destDir, path.Base(repo.Files[len(repo.Files)-1]))); !os.IsNotExist(err) {
		t.Errorf("last file should not exists in the test branch")
	}
	if _, err := os.Stat(filepath.Join(destDir, "sub")); os.IsNotExist(err) {
		t.Errorf("unable to find submodule dir")
	}
	for _, f := range repo.Submodule.Files {
		if _, err := os.Stat(filepath.Join(destDir, "sub/"+path.Base(f))); os.IsNotExist(err) {
			t.Errorf("unable to find submodule repository file %q", path.Base(f))
		}
	}
}
