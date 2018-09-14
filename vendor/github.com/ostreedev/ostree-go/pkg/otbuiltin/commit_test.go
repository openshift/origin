package otbuiltin

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/14rcole/gopopulate"
)

func TestCommitSuccess(t *testing.T) {
	// Make a base directory in which all of our test data resides
	baseDir, err := ioutil.TempDir("", "otbuiltin-test-")
	if err != nil {
		t.Errorf("%s", err)
		return
	}
	defer os.RemoveAll(baseDir)

	// Make a directory in which the repo should exist
	repoDir := path.Join(baseDir, "repo")
	err = os.Mkdir(repoDir, 0777)
	if err != nil {
		t.Errorf("%s", err)
		return
	}

	// Initialize the repo
	inited, err := Init(repoDir, NewInitOptions())
	if !inited || err != nil {
		fmt.Println("Cannot test commit: failed to initialize repo")
		return
	}

	// Make a new directory full of random data to commit
	commitDir := path.Join(baseDir, "commit1")
	err = os.Mkdir(commitDir, 0777)
	if err != nil {
		t.Errorf("%s", err)
		return
	}
	err = gopopulate.PopulateDir(commitDir, "rd", 4, 4)
	if err != nil {
		t.Errorf("%s", err)
		return
	}

	// Test commit
	repo, err := OpenRepo(repoDir)
	if err != nil {
		t.Errorf("%s", err)
	}
	opts := NewCommitOptions()
	branch := "test-branch"
	_, err = repo.PrepareTransaction()
	if err != nil {
		t.Errorf("%s", err)
	}
	ret, err := repo.Commit(commitDir, branch, opts)
	if err != nil {
		t.Errorf("%s", err)
	} else {
		fmt.Println(ret)
	}
	_, err = repo.CommitTransaction()
	if err != nil {
		t.Errorf("%s", err)
	}
}

func TestCommitTreeSuccess(t *testing.T) {
	// Make a base directory in which all of our test data resides
	baseDir, err := ioutil.TempDir("", "otbuiltin-test-")
	if err != nil {
		t.Fatalf("failed to create tempdir: %s", err)
	}
	defer os.RemoveAll(baseDir)

	// Make a directory in which the repo should exist
	repoDir := path.Join(baseDir, "repo")
	if err := os.MkdirAll(repoDir, 0777); err != nil {
		t.Fatalf("failed to create repodir at %q: %s", repoDir, err)
	}

	// Initialize the repo
	initOk, err := Init(repoDir, NewInitOptions())
	if err != nil {
		t.Fatalf("failed to initialize the repo: %s", err)
	}
	if !initOk {
		t.Fatal("failed to initialize repo")
	}

	// Make a new directory full of random data to commit
	commitDir := path.Join(baseDir, "commit1")
	tarPath := path.Join(baseDir, "tree.tar")
	if err := os.Mkdir(commitDir, 0777); err != nil {
		t.Fatalf("failed to make random data dir at %q: %s", commitDir, err)
	}
	if err := gopopulate.PopulateDir(commitDir, "rd", 4, 4); err != nil {
		t.Fatalf("failed to populate dir: %s", err)
	}
	if err := gopopulate.Tar(commitDir, tarPath); err != nil {
		t.Fatalf("failed to tar populated dir: %s", err)
	}

	// Test commit
	repo, err := OpenRepo(repoDir)
	if err != nil {
		t.Fatalf("failed to open repo at %q: %s", repoDir, err)
	}
	branch := "test-branch"
	opts := NewCommitOptions()
	opts.Subject = "blob"
	opts.Tree = []string{"tar=" + tarPath}
	opts.TarAutoCreateParents = true
	if _, err := repo.PrepareTransaction(); err != nil {
		t.Fatalf("failed to prepare transaction: %s", err)
	}

	// TODO(lucab): investigate flake and re-enable commit.
	t.Skip("flake: fchown EPERM")

	if _, err := repo.Commit("", branch, opts); err != nil {
		t.Fatalf("failed to commit: %s", err)
	}

	if _, err = repo.CommitTransaction(); err != nil {
		t.Fatalf("failed to commit transaction: %s", err)
	}
}

func TestCommitTreeParentSuccess(t *testing.T) {
	// Make a base directory in which all of our test data resides
	baseDir, err := ioutil.TempDir("", "otbuiltin-test-")
	if err != nil {
		t.Fatalf("failed to create tempdir: %s", err)
	}
	defer os.RemoveAll(baseDir)

	// Make a directory in which the repo should exist
	repoDir := path.Join(baseDir, "repo")
	if err = os.MkdirAll(repoDir, 0777); err != nil {
		t.Fatalf("failed to create repodir at %q: %s", repoDir, err)
	}

	// Initialize the repo
	initOk, err := Init(repoDir, NewInitOptions())
	if err != nil {
		t.Fatalf("failed to initialize the repo: %s", err)
	}
	if !initOk {
		t.Fatal("failed to initialize repo")
	}

	// Make a new directory full of random data to commit
	commitDir := path.Join(baseDir, "commit1")
	tarPath := path.Join(baseDir, "tree.tar")
	if err := os.Mkdir(commitDir, 0777); err != nil {
		t.Fatalf("failed to make random data dir at %q: %s", commitDir, err)
	}
	if err := gopopulate.PopulateDir(commitDir, "rd", 4, 4); err != nil {
		t.Fatalf("failed to populate dir: %s", err)
	}
	if err := gopopulate.Tar(commitDir, tarPath); err != nil {
		t.Fatalf("failed to tar populated dir: %s", err)
	}

	// Create a test commit
	repo, err := OpenRepo(repoDir)
	if err != nil {
		t.Fatalf("failed to open repo at %q: %s", repoDir, err)
	}
	opts := NewCommitOptions()
	opts.Subject = "blob"
	opts.Tree = []string{"tar=" + tarPath}
	opts.TarAutoCreateParents = true
	branch := "test-branch"

	// TODO(lucab): investigate flake and re-enable commit.
	t.Skip("flake: fchown EPERM")

	// Commit a first time
	if _, err := repo.PrepareTransaction(); err != nil {
		t.Fatalf("failed to prepare transaction: %s", err)
	}
	parentChecksum, err := repo.Commit("", branch, opts)
	if err != nil {
		t.Fatalf("failed to commit: %s", err)
	}
	if _, err := repo.CommitTransaction(); err != nil {
		t.Fatalf("failed to commit transaction: %s", err)
	}

	// Commit again, this time with a parent checksum
	if _, err := repo.PrepareTransaction(); err != nil {
		t.Fatalf("failed to prepare transaction: %s", err)
	}
	opts.Parent = parentChecksum
	if _, err := repo.Commit("", branch, opts); err != nil {
		t.Fatalf("failed to commit with parent: %s", err)
	}
	if _, err := repo.CommitTransaction(); err != nil {
		t.Fatalf("failed to commit transaction: %s", err)
	}
}
