package scm

import (
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/openshift/source-to-image/pkg/test"
	"github.com/openshift/source-to-image/pkg/util"
)

func TestDownloaderForSource(t *testing.T) {
	gitLocalDir := test.CreateLocalGitDirectory(t)
	defer os.RemoveAll(gitLocalDir)
	localDir, _ := ioutil.TempDir(os.TempDir(), "localdir-s2i-test")
	defer os.RemoveAll(localDir)

	tc := map[string]string{
		// Valid Git clone specs
		"git://github.com/bar":       "git.Clone",
		"https://github.com/bar":     "git.Clone",
		"git@github.com:foo/bar.git": "git.Clone",
		// Non-existing local path (it is not git repository, so it is file
		// download)
		"file://foo/bar": "error",
		"/foo/bar":       "error",
		// Local directory with valid Git repository
		gitLocalDir:             "git.Clone",
		"file://" + gitLocalDir: "git.Clone",
		// Local directory that exists but it is not Git repository
		localDir:               "file.File",
		"file://" + localDir:   "file.File",
		"foo://github.com/bar": "error",
		"./foo":                "error",
		// Empty source string
		"": "empty.Noop",
	}

	for s, expected := range tc {
		r, filePathUpdate, err := DownloaderForSource(util.NewFileSystem(), s, false)
		if err != nil {
			if expected != "error" {
				t.Errorf("Unexpected error %q for %q, expected %q", err, s, expected)
			}
			continue
		}

		if s == gitLocalDir || s == localDir {
			if !strings.HasPrefix(filePathUpdate, "file://") {
				t.Errorf("input %s should have produced a file path update starting with file:// but produced:  %s", s, filePathUpdate)
			}
		}

		expected = "*" + expected
		if reflect.TypeOf(r).String() != expected {
			t.Errorf("Expected %q for %q, got %q", expected, s, reflect.TypeOf(r).String())
		}
	}
}

func TestDownloaderForSourceOnRelativeGit(t *testing.T) {
	gitLocalDir := test.CreateLocalGitDirectory(t)
	defer os.RemoveAll(gitLocalDir)
	os.Chdir(gitLocalDir)
	r, s, err := DownloaderForSource(util.NewFileSystem(), ".", false)
	if err != nil {
		t.Errorf("Unexpected error %q for %q, expected %q", err, ".", "git.Clone")
	}
	if !strings.HasPrefix(s, "file://") {
		t.Errorf("Expected source to have 'file://' prefix, it is %q", s)
	}
	if reflect.TypeOf(r).String() != "*git.Clone" {
		t.Errorf("Expected %q for %q, got %q", "*git.Clone", ".", reflect.TypeOf(r).String())
	}
}

func TestDownloaderForceCopy(t *testing.T) {
	gitLocalDir := test.CreateLocalGitDirectory(t)
	defer os.RemoveAll(gitLocalDir)
	os.Chdir(gitLocalDir)
	r, s, err := DownloaderForSource(util.NewFileSystem(), ".", true)
	if err != nil {
		t.Errorf("Unexpected error %q for %q, expected %q", err, ".", "*file.File")
	}
	if !strings.HasPrefix(s, "file://") {
		t.Errorf("Expected source to have 'file://' prefix, it is %q", s)
	}
	if reflect.TypeOf(r).String() != "*file.File" {
		t.Errorf("Expected %q for %q, got %q", "*file.File", ".", reflect.TypeOf(r).String())
	}
}
