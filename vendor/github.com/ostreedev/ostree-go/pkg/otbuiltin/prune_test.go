package otbuiltin

import (
	"os"
	"testing"
	//"strconv"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/14rcole/gopopulate"
)

func TestPruneNoPrunePass(t *testing.T) {
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

	//Make a new directory full of random data to commit
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

	//Test commit
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

	// Now let's do some pruning!
	pruneOpts := NewPruneOptions()
	pruneOpts.NoPrune = true
	ret, err = Prune(repoDir, pruneOpts)
	if err != nil {
		t.Errorf("%s", err)
	} else {
		fmt.Println(ret)
	}
}

func TestPruneNoPruneFail(t *testing.T) {
}

func TestPrunePass(t *testing.T) {
}

func TestPruneFail(t *testing.T) {
}
