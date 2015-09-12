package git

import (
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/test"
)

func TestCloneWithContext(t *testing.T) {
	gh := New().(*stiGit)
	cr := &test.FakeCmdRunner{}
	gh.runner = cr
	fs := &test.FakeFileSystem{}
	c := &Clone{gh, fs}

	fakeConfig := &api.Config{
		Source:     "https://foo/bar.git",
		ContextDir: "subdir",
		Ref:        "ref1",
	}
	info, err := c.Download(fakeConfig)
	if err != nil {
		t.Errorf("%v", err)
	}
	if info == nil {
		t.Fatalf("Expected info to be not nil")
	}
	if fs.CopySource != "upload/tmp/subdir/." {
		t.Errorf("The source directory should be 'upload/tmp/subdir', it is %v", fs.CopySource)
	}
	if fs.CopyDest != "upload/src" {
		t.Errorf("The target directory should be 'upload/src', it is %v", fs.CopyDest)
	}
	if fs.RemoveDirName != "upload/tmp" {
		t.Errorf("Expected to remove the upload/tmp directory")
	}
	if !reflect.DeepEqual(cr.Args, []string{"checkout", "ref1"}) {
		t.Errorf("Unexpected command arguments: %#v\n", cr.Args)
	}
}

func TestCloneLocalWithContext(t *testing.T) {
	gh := New().(*stiGit)
	cr := &test.FakeCmdRunner{}
	gh.runner = cr
	fs := &test.FakeFileSystem{ExistsResult: map[string]bool{"source/subdir/.": true}}
	c := &Clone{gh, fs}

	fakeConfig := &api.Config{
		Source:     "source",
		ContextDir: "subdir",
		Ref:        "ref1",
	}
	_, err := c.Download(fakeConfig)
	if err != nil {
		t.Errorf("%v", err)
	}
	if fs.CopySource != "source/subdir/." {
		t.Errorf("The source directory should be 'source/subdir', it is %v", fs.CopySource)
	}
	if fs.CopyDest != "upload/src" {
		t.Errorf("The target directory should be 'upload/src', it is %v", fs.CopyDest)
	}
}
