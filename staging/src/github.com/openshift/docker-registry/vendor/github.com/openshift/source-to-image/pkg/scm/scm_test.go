package scm

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/util/fs"
)

func TestDownloaderForSource(t *testing.T) {
	gitLocalDir, err := git.CreateLocalGitDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(gitLocalDir)
	localDir, _ := ioutil.TempDir(os.TempDir(), "localdir-s2i-test")
	defer os.RemoveAll(localDir)

	tc := map[*git.URL]string{
		// Valid Git clone specs
		git.MustParse("git://github.com/bar"):       "git.Clone",
		git.MustParse("https://github.com/bar"):     "git.Clone",
		git.MustParse("git@github.com:foo/bar.git"): "git.Clone",
		// Local directory with valid Git repository
		git.MustParse(gitLocalDir):                                "git.Clone",
		git.MustParse("file:///" + filepath.ToSlash(gitLocalDir)): "git.Clone",
		// Local directory that exists but it is not Git repository
		git.MustParse(localDir):                                "file.File",
		git.MustParse("file:///" + filepath.ToSlash(localDir)): "file.File",
		// Empty source string
		nil: "empty.Noop",
	}

	for s, expected := range tc {
		r, err := DownloaderForSource(fs.NewFileSystem(), s, false)
		if err != nil {
			t.Errorf("Unexpected error %q for %q, expected %q", err, s, expected)
			continue
		}

		expected = "*" + expected
		if reflect.TypeOf(r).String() != expected {
			t.Errorf("Expected %q for %q, got %q", expected, s, reflect.TypeOf(r).String())
		}
	}
}

func TestDownloaderForSourceOnRelativeGit(t *testing.T) {
	gitLocalDir, err := git.CreateLocalGitDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(gitLocalDir)
	os.Chdir(gitLocalDir)
	r, err := DownloaderForSource(fs.NewFileSystem(), git.MustParse("."), false)
	if err != nil {
		t.Errorf("Unexpected error %q for %q, expected %q", err, ".", "git.Clone")
	}
	if reflect.TypeOf(r).String() != "*git.Clone" {
		t.Errorf("Expected %q for %q, got %q", "*git.Clone", ".", reflect.TypeOf(r).String())
	}
}

func TestDownloaderForceCopy(t *testing.T) {
	gitLocalDir, err := git.CreateLocalGitDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(gitLocalDir)
	os.Chdir(gitLocalDir)
	r, err := DownloaderForSource(fs.NewFileSystem(), git.MustParse("."), true)
	if err != nil {
		t.Errorf("Unexpected error %q for %q, expected %q", err, ".", "*file.File")
	}
	if reflect.TypeOf(r).String() != "*file.File" {
		t.Errorf("Expected %q for %q, got %q", "*file.File", ".", reflect.TypeOf(r).String())
	}
}
