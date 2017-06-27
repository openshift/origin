package node

import (
	"io/ioutil"
	"os"
	"path"
)

import "testing"

func TestInitializeVolumeDir(t *testing.T) {
	parentDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Unable to create parent temp dir: %s", err)
	}
	if err := os.MkdirAll(parentDir, 0750); err != nil {
		t.Fatalf("Error creating volume parent dir: %s", err)
	}
	defer os.RemoveAll(parentDir)

	volumeDir := path.Join(parentDir, "somedir")

	testCases := map[string]struct {
		dirAlreadyExists bool
	}{
		"volume dir does not exist": {dirAlreadyExists: false},
		"volume dir already exists": {dirAlreadyExists: true},
	}

	for name, testCase := range testCases {
		if testCase.dirAlreadyExists {
			if err := os.MkdirAll(volumeDir, 0750); err != nil {
				t.Fatalf("%s: error creating volume dir: %v", name, err)
			}
		} else {
			if err := os.RemoveAll(volumeDir); err != nil {
				t.Fatalf("%s: error removing volume dir: %v", name, err)
			}
		}

		nc := &NodeConfig{VolumeDir: volumeDir}
		path, err := nc.initializeVolumeDir(nc.VolumeDir)

		if err != nil {
			t.Errorf("%s: unexpected err: %s", name, err)
		}
		if path != nc.VolumeDir {
			t.Errorf("%s:, expected path(%s) == nc.VolumeDir(%s)", name, path, nc.VolumeDir)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s: expected volume dir to exist: %v", name, err)
		}
	}
}
