package otbuiltin

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/14rcole/gopopulate"
)

func TestLogSuccess(t *testing.T) {
	// Make a base directory in which all of our test data resides
	baseDir, err := ioutil.TempDir("", "otbuiltin-test-")
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer os.RemoveAll(baseDir)

	// Make a directory in which the repo should exist
	repoDir := path.Join(baseDir, "repo")
	if err := os.Mkdir(repoDir, 0777); err != nil {
		t.Fatalf("%s", err)
	}

	// Initialize the repo
	inited, err := Init(repoDir, NewInitOptions())
	if !inited || err != nil {
		t.Fatal("Cannot test commit: failed to initialize repo")
	}

	// Make a new directory full of random data to commit
	commitDir := path.Join(baseDir, "commit1")
	if err := os.Mkdir(commitDir, 0777); err != nil {
		t.Fatalf("%s", err)
	}
	if err := gopopulate.PopulateDir(commitDir, "rd", 4, 4); err != nil {
		t.Fatalf("%s", err)
	}

	// Test commit
	repo, err := OpenRepo(repoDir)
	if err != nil {
		t.Fatalf("%s", err)
	}

	opts := NewCommitOptions()
	branch := "test-branch"
	_, err = repo.PrepareTransaction()
	if err != nil {
		t.Fatalf("%s", err)
	}

	if _, err := repo.Commit(commitDir, branch, opts); err != nil {
		t.Fatalf("%s", err)
	}

	if _, err = repo.CommitTransaction(); err != nil {
		t.Fatalf("%s", err)
	}

	// Add more files to the commit dir and return
	if err := gopopulate.PopulateDir(commitDir, "rd", 4, 4); err != nil {
		t.Fatalf("%s", err)
	}

	// Get the logs for the branch
	logOpts := NewLogOptions()
	entries, err := Log(repoDir, branch, logOpts)
	if err != nil {
		t.Fatalf("%s", err)
	}

	if len(entries) <= 0 {
		t.Fatal("got no entries")
	}
}
