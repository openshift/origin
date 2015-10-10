package scm

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

func TestDownloaderForSource(t *testing.T) {
	gitLocalDir := createLocalGitDirectory(t)
	defer os.RemoveAll(gitLocalDir)
	localDir, _ := ioutil.TempDir(os.TempDir(), "s2i-test")
	defer os.RemoveAll(localDir)

	tc := map[string]string{
		// Valid GIT clone specs
		"git://github.com/bar":       "git.Clone",
		"https://github.com/bar":     "git.Clone",
		"git@github.com:foo/bar.git": "git.Clone",
		// Non-existing local path (it is not git repository, so it is file
		// download)
		"file://foo/bar": "error",
		"/foo/bar":       "error",
		// Local directory with valid GIT repository
		gitLocalDir:             "git.Clone",
		"file://" + gitLocalDir: "git.Clone",
		// Local directory that exists but it is not GIT repository
		localDir:             "file.File",
		"file://" + localDir: "file.File",
		".":                  "file.File",
		"foo://github.com/bar": "error",
		"./foo":                "error",
	}

	for s, expected := range tc {
		r, _, err := DownloaderForSource(s)
		if err != nil {
			if expected != "error" {
				t.Errorf("Unexpected error %q for %q, expected %q", err, s, expected)
			}
			continue
		}

		expected = "*" + expected
		if reflect.TypeOf(r).String() != expected {
			t.Errorf("Expected %q for %q, got %q", expected, s, reflect.TypeOf(r).String())
		}
	}
}

func TestDownloaderForSourceOnRelativeGit(t *testing.T) {
	gitLocalDir := createLocalGitDirectory(t)
	defer os.RemoveAll(gitLocalDir)
	os.Chdir(gitLocalDir)
	r, s, err := DownloaderForSource(".")
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
