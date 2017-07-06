package file

import (
	"path/filepath"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/test"
)

func TestDownload(t *testing.T) {
	fs := &test.FakeFileSystem{}
	f := &File{fs}

	config := &api.Config{
		Source: "file:///foo",
	}
	info, err := f.Download(config)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if fs.CopySource != "/foo" {
		t.Errorf("Unexpected fs.CopySource %s", fs.CopySource)
	}
	if info.Location != config.Source[7:] || info.ContextDir != config.ContextDir {
		t.Errorf("Unexpected info")
	}
}

func TestDownloadWithContext(t *testing.T) {
	fs := &test.FakeFileSystem{}
	f := &File{fs}

	config := &api.Config{
		Source:     "file:///foo",
		ContextDir: "bar",
	}
	info, err := f.Download(config)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if filepath.ToSlash(fs.CopySource) != "/foo/bar" {
		t.Errorf("Unexpected fs.CopySource %s", fs.CopySource)
	}
	if info.Location != config.Source[7:] || info.ContextDir != config.ContextDir {
		t.Errorf("Unexpected info")
	}
}
