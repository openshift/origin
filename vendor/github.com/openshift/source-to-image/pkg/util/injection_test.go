package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
)

func TestCreateInjectedFilesRemovalScript(t *testing.T) {
	files := []string{
		"/foo",
		"/bar/bar",
	}
	name, err := CreateInjectedFilesRemovalScript(files, "/tmp/rm-foo")
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

func TestExpandInjectedFiles(t *testing.T) {
	tmp, err := ioutil.TempDir("", "s2i-test-")
	tmpNested, err := ioutil.TempDir(tmp, "nested")
	if err != nil {
		t.Errorf("Unable to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmp)
	list := api.VolumeList{{Source: tmp, Destination: "/foo"}}
	f1, _ := ioutil.TempFile(tmp, "foo")
	f2, _ := ioutil.TempFile(tmpNested, "bar")
	files, err := ExpandInjectedFiles(NewFileSystem(), list)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := []string{"/foo/" + filepath.Base(f1.Name()), "/foo/" + filepath.Base(tmpNested) + "/" + filepath.Base(f2.Name())}
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
