package git

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/test"
)

func TestCloneWithContext(t *testing.T) {
	fs := &test.FakeFileSystem{}
	gh := New(fs).(*stiGit)
	cr := &test.FakeCmdRunner{}
	gh.CommandRunner = cr
	c := &Clone{gh, fs}

	fakeConfig := &api.Config{
		Source:           "https://foo/bar.git",
		ContextDir:       "subdir",
		Ref:              "ref1",
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
	if !reflect.DeepEqual(cr.Args, []string{"checkout", "ref1"}) {
		t.Errorf("Unexpected command arguments: %#v", cr.Args)
	}
}
