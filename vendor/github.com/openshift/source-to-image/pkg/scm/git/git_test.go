package git

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	s2ierr "github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/test"
	"github.com/openshift/source-to-image/pkg/util"
)

func TestIsValidGitRepository(t *testing.T) {
	fs := util.NewFileSystem()

	d := test.CreateLocalGitDirectory(t)
	defer os.RemoveAll(d)

	// We have a .git that is populated
	// Should return true with no error
	ok, err := isValidGitRepository(fs, d)
	if !ok {
		t.Errorf("The %q directory is git repository", d)
	}

	if err != nil {
		t.Errorf("isValidGitRepository returned an unexpected error: %q", err.Error())
	}

	d = test.CreateEmptyLocalGitDirectory(t)
	defer os.RemoveAll(d)

	// There are no tracking objects in the .git repository
	// Should return true with an EmptyGitRepositoryError
	ok, err = isValidGitRepository(fs, d)
	if !ok {
		t.Errorf("The %q directory is a git repository, but is empty", d)
	}

	if err != nil {
		var e s2ierr.Error
		if e, ok = err.(s2ierr.Error); !ok || e.ErrorCode != s2ierr.EmptyGitRepositoryError {
			t.Errorf("isValidGitRepository returned an unexpected error: %q, expecting EmptyGitRepositoryError", err.Error())
		}
	} else {
		t.Errorf("isValidGitRepository returned no error, expecting EmptyGitRepositoryError")
	}

	d = filepath.Join(d, ".git")

	// There is no .git in the provided directory
	// Should return false with no error
	if ok, err = isValidGitRepository(fs, d); ok {
		t.Errorf("The %q directory is not git repository", d)
	}

	if err != nil {
		t.Errorf("isValidGitRepository returned an unexpected error: %q", err.Error())
	}

	d = test.CreateLocalGitDirectoryWithSubmodule(t)
	defer os.RemoveAll(d)

	ok, err = isValidGitRepository(fs, filepath.Join(d, "submodule"))
	if !ok || err != nil {
		t.Errorf("Expected isValidGitRepository to return true, nil on submodule; got %v, %v", ok, err)
	}
}

func TestValidCloneSpec(t *testing.T) {
	gitLocalDir := test.CreateLocalGitDirectory(t)
	defer os.RemoveAll(gitLocalDir)

	valid := []string{"git@github.com:user/repo.git",
		"git://github.com/user/repo.git",
		"git://github.com/user/repo",
		"http://github.com/user/repo.git",
		"http://github.com/user/repo",
		"https://github.com/user/repo.git",
		"https://github.com/user/repo",
		"file://" + gitLocalDir,
		"file://" + gitLocalDir + "#master",
		gitLocalDir,
		gitLocalDir + "#master",
		"git@192.168.122.1:repositories/authooks",
		"mbalazs@build.ulx.hu:/var/git/eap-ulx.git",
		"ssh://git@[2001:db8::1]/repository.git",
		"ssh://git@mydomain.com:8080/foo/bar",
		"git@[2001:db8::1]:repository.git",
		"git@[2001:db8::1]:/repository.git",
		"g_m@foo.bar:foo/bar",
		"g-m@foo.bar:foo/bar",
		"g.m@foo.bar:foo/bar",
		"github.com:openshift/origin.git",
		"http://github.com/user/repo#1234",
	}
	invalid := []string{"g&m@foo.bar:foo.bar",
		"~git@test.server/repo.git",
		"some/lo:cal/path", // a local path that does not exist, but "some/lo" is not a valid host name
		"http://github.com/user/repo#%%%%",
	}

	gh := New(util.NewFileSystem())

	for _, scenario := range valid {
		result, _ := gh.ValidCloneSpec(scenario)
		if result == false {
			t.Error(scenario)
		}
	}
	for _, scenario := range invalid {
		result, _ := gh.ValidCloneSpec(scenario)
		if result {
			t.Error(scenario)
		}
	}
}

func TestValidCloneSpecRemoteOnly(t *testing.T) {
	valid := []string{"git://github.com/user/repo.git",
		"git://github.com/user/repo",
		"http://github.com/user/repo.git",
		"http://github.com/user/repo",
		"https://github.com/user/repo.git",
		"https://github.com/user/repo",
		"ssh://git@[2001:db8::1]/repository.git",
		"ssh://git@mydomain.com:8080/foo/bar",
		"git@github.com:user/repo.git",
		"git@192.168.122.1:repositories/authooks",
		"mbalazs@build.ulx.hu:/var/git/eap-ulx.git",
		"git@[2001:db8::1]:repository.git",
		"git@[2001:db8::1]:/repository.git",
	}
	invalid := []string{"file:///home/user/code/repo.git",
		"/home/user/code/repo.git",
	}

	gh := New(util.NewFileSystem())

	for _, scenario := range valid {
		result := gh.ValidCloneSpecRemoteOnly(scenario)
		if result == false {
			t.Error(scenario)
		}
	}
	for _, scenario := range invalid {
		result := gh.ValidCloneSpecRemoteOnly(scenario)
		if result {
			t.Error(scenario)
		}
	}
}

func getGit() (*stiGit, *test.FakeCmdRunner) {
	gh := New(&test.FakeFileSystem{}).(*stiGit)
	cr := &test.FakeCmdRunner{}
	gh.CommandRunner = cr

	return gh, cr
}

func TestGitClone(t *testing.T) {
	gh, ch := getGit()
	err := gh.Clone("source1", "target1", api.CloneConfig{Quiet: true, Recursive: true})
	if err != nil {
		t.Errorf("Unexpected error returned from clone: %v", err)
	}
	if ch.Name != "git" {
		t.Errorf("Unexpected command name: %q", ch.Name)
	}
	if !reflect.DeepEqual(ch.Args, []string{"clone", "--quiet", "--recursive", "source1", "target1"}) {
		t.Errorf("Unexpected command arguments: %#v", ch.Args)
	}
}

func TestGitCloneError(t *testing.T) {
	gh, ch := getGit()
	runErr := fmt.Errorf("Run Error")
	ch.Err = runErr
	err := gh.Clone("source1", "target1", api.CloneConfig{})
	if err != runErr {
		t.Errorf("Unexpected error returned from clone: %v", err)
	}
}

func TestGitCheckout(t *testing.T) {
	gh, ch := getGit()
	err := gh.Checkout("repo1", "ref1")
	if err != nil {
		t.Errorf("Unexpected error returned from checkout: %v", err)
	}
	if ch.Name != "git" {
		t.Errorf("Unexpected command name: %q", ch.Name)
	}
	if !reflect.DeepEqual(ch.Args, []string{"checkout", "ref1"}) {
		t.Errorf("Unexpected command arguments: %#v", ch.Args)
	}
	if ch.Opts.Dir != "repo1" {
		t.Errorf("Unexpected value in exec directory: %q", ch.Opts.Dir)
	}
}

func TestGitCheckoutError(t *testing.T) {
	gh, ch := getGit()
	runErr := fmt.Errorf("Run Error")
	ch.Err = runErr
	err := gh.Checkout("repo1", "ref1")
	if err != runErr {
		t.Errorf("Unexpected error returned from checkout: %v", err)
	}
}
