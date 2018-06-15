package file

import (
	"path/filepath"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/scm/git"
	testfs "github.com/openshift/source-to-image/pkg/test/fs"
)

func TestDownload(t *testing.T) {
	fs := &testfs.FakeFileSystem{}
	f := &File{fs}

	config := &api.Config{
		Source: git.MustParse("/foo"),
	}
	info, err := f.Download(config)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if fs.CopySource != "/foo" {
		t.Errorf("Unexpected fs.CopySource %s", fs.CopySource)
	}
	if info.Location != config.Source.URL.Path || info.ContextDir != config.ContextDir {
		t.Errorf("Unexpected info")
	}
}

func TestDownloadWithContext(t *testing.T) {
	fs := &testfs.FakeFileSystem{}
	f := &File{fs}

	config := &api.Config{
		Source:     git.MustParse("/foo"),
		ContextDir: "bar",
	}
	info, err := f.Download(config)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if filepath.ToSlash(fs.CopySource) != "/foo/bar" {
		t.Errorf("Unexpected fs.CopySource %s", fs.CopySource)
	}
	if info.Location != config.Source.URL.Path || info.ContextDir != config.ContextDir {
		t.Errorf("Unexpected info")
	}
}
