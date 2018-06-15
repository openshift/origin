package git

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/scm/git"
	testcmd "github.com/openshift/source-to-image/pkg/test/cmd"
	testfs "github.com/openshift/source-to-image/pkg/test/fs"
)

func TestCloneWithContext(t *testing.T) {
	fs := &testfs.FakeFileSystem{}
	cr := &testcmd.FakeCmdRunner{}
	gh := git.New(fs, cr)
	c := &Clone{gh, fs}

	fakeConfig := &api.Config{
		Source:           git.MustParse("https://foo/bar.git#ref1"),
		ContextDir:       "subdir",
		IgnoreSubmodules: true,
	}
	info, err := c.Download(fakeConfig)
	if err != nil {
		t.Errorf("%v", err)
	}
	if info == nil {
		t.Fatalf("Expected info to be not nil")
	}
	if filepath.ToSlash(fs.CopySource) != "upload/tmp/subdir" {
		t.Errorf("The source directory should be 'upload/tmp/subdir', it is %v", fs.CopySource)
	}
	if filepath.ToSlash(fs.CopyDest) != "upload/src" {
		t.Errorf("The target directory should be 'upload/src', it is %v", fs.CopyDest)
	}
	if filepath.ToSlash(fs.RemoveDirName) != "upload/tmp" {
		t.Errorf("Expected to remove the upload/tmp directory")
	}
	if !reflect.DeepEqual(cr.Args, []string{"checkout", "--quiet", "ref1"}) {
		t.Errorf("Unexpected command arguments: %#v", cr.Args)
	}
}
