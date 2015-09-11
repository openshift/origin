package git

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/test"
)

func TestValidCloneSpec(t *testing.T) {
	scenarios := []string{"git@github.com:user/repo.git",
		"git://github.com/user/repo.git",
		"git://github.com/user/repo",
		"http://github.com/user/repo.git",
		"http://github.com/user/repo",
		"https://github.com/user/repo.git",
		"https://github.com/user/repo",
		"file:///home/user/code/repo.git",
		"/home/user/code/repo.git",
	}

	gh := New()

	for _, scenario := range scenarios {
		result := gh.ValidCloneSpec(scenario)
		if result == false {
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
