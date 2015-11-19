package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/test"
)

func createLocalGitDirectory(t *testing.T) string {
	dir, err := ioutil.TempDir(os.TempDir(), "s2i-test")
	if err != nil {
		t.Error(err)
	}
	os.Mkdir(filepath.Join(dir, ".git"), 0600)
	return dir
}

func TestIsLocalGitRepository(t *testing.T) {
	d := createLocalGitDirectory(t)
	defer os.RemoveAll(d)
	if isLocalGitRepository(d) == false {
		t.Errorf("The %q directory is git repository", d)
	}
	os.RemoveAll(filepath.Join(d, ".git"))
	if isLocalGitRepository(d) == true {
		t.Errorf("The %q directory is not git repository", d)
	}
}

func TestValidCloneSpec(t *testing.T) {
	gitLocalDir := createLocalGitDirectory(t)
	defer os.RemoveAll(gitLocalDir)

	valid := []string{"git@github.com:user/repo.git",
		"git://github.com/user/repo.git",
		"git://github.com/user/repo",
		"http://github.com/user/repo.git",
		"http://github.com/user/repo",
		"https://github.com/user/repo.git",
		"https://github.com/user/repo",
		"file://" + gitLocalDir,
		gitLocalDir,
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

	gh := New()

	for _, scenario := range valid {
		result := gh.ValidCloneSpec(scenario)
		if result == false {
			t.Error(scenario)
		}
	}
	for _, scenario := range invalid {
		result := gh.ValidCloneSpec(scenario)
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

	gh := New()

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
	gh := New().(*stiGit)
	cr := &test.FakeCmdRunner{}
	gh.runner = cr

	return gh, cr
}

func TestGitClone(t *testing.T) {
	gh, ch := getGit()
	err := gh.Clone("source1", "target1", api.CloneConfig{Quiet: true, Recursive: true})
	if err != nil {
		t.Errorf("Unexpected error returned from clone: %v\n", err)
	}
	if ch.Name != "git" {
		t.Errorf("Unexpected command name: %s\n", ch.Name)
	}
	if !reflect.DeepEqual(ch.Args, []string{"clone", "--quiet", "--recursive", "source1", "target1"}) {
		t.Errorf("Unexpected command arguments: %#v\n", ch.Args)
	}
}

func TestGitCloneError(t *testing.T) {
	gh, ch := getGit()
	runErr := fmt.Errorf("Run Error")
	ch.Err = runErr
	err := gh.Clone("source1", "target1", api.CloneConfig{})
	if err != runErr {
		t.Errorf("Unexpected error returned from clone: %v\n", err)
	}
}

func TestGitCheckout(t *testing.T) {
	gh, ch := getGit()
	err := gh.Checkout("repo1", "ref1")
	if err != nil {
		t.Errorf("Unexpected error returned from checkout: %v\n", err)
	}
	if ch.Name != "git" {
		t.Errorf("Unexpected command name: %s\n", ch.Name)
	}
	if !reflect.DeepEqual(ch.Args, []string{"checkout", "ref1"}) {
		t.Errorf("Unexpected command arguments: %#v\n", ch.Args)
	}
	if ch.Opts.Dir != "repo1" {
		t.Errorf("Unexpected value in exec directory: %s\n", ch.Opts.Dir)
	}
}

func TestGitCheckoutError(t *testing.T) {
	gh, ch := getGit()
	runErr := fmt.Errorf("Run Error")
	ch.Err = runErr
	err := gh.Checkout("repo1", "ref1")
	if err != runErr {
		t.Errorf("Unexpected error returned from checkout: %v\n", err)
	}
}
