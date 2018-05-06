package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/util/fs"
)

func TestCreateTruncateFilesScript(t *testing.T) {
	files := []string{
		"/foo",
		"/bar/bar",
	}
	name, err := CreateTruncateFilesScript(files, "/tmp/rm-foo")
	defer os.Remove(name)
	if err != nil {
		t.Errorf("Unexpected error: %v", name)
	}
	_, err = os.Stat(name)
	if err != nil {
		t.Errorf("Expected file %q to exists, got: %v", name, err)
	}
	data, err := ioutil.ReadFile(name)
	if err != nil {
		t.Errorf("Unable to read %q: %v", name, err)
	}
	if !strings.Contains(string(data), fmt.Sprintf("truncate -s0 %q", "/foo")) {
		t.Errorf("Expected script to contain truncate -s0 \"/foo\", got: %q", string(data))
	}
	if !strings.Contains(string(data), fmt.Sprintf("truncate -s0 %q", "/tmp/rm-foo")) {
		t.Errorf("Expected script to truncate itself, got: %q", string(data))
	}
}

func TestListFilesToTruncate(t *testing.T) {
	tmp, err := ioutil.TempDir("", "s2i-test-")
	tmpKeep, err := ioutil.TempDir("", "s2i-test-")
	tmpNested, err := ioutil.TempDir(tmp, "nested")
	if err != nil {
		t.Errorf("Unable to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmp)
	defer os.RemoveAll(tmpKeep)
	list := api.VolumeList{
		{Source: tmp, Destination: "/foo"},
		{Source: tmpKeep, Destination: "/this", Keep: true},
	}
	f1, _ := ioutil.TempFile(tmp, "foo")
	f2, _ := ioutil.TempFile(tmpNested, "bar")
	ioutil.TempFile(tmpKeep, "that")
	files, err := ListFilesToTruncate(fs.NewFileSystem(), list)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := []string{"/foo/" + filepath.Base(f1.Name()), "/foo/" + filepath.Base(tmpNested) + "/" + filepath.Base(f2.Name())}
	if len(expected) != len(files) {
		t.Errorf("Expected %d files in resulting list, got %+v", len(expected), files)
	}
	for _, exp := range expected {
		found := false
		for _, f := range files {
			if f == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected %q in resulting file list, got %+v", exp, files)
		}
	}
}
