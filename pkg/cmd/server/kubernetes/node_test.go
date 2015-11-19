package kubernetes

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
)

import "testing"

type fakeCommandExecutor struct {
	commandFound bool
	commandErr   error
	runCalled    bool
	lookCalled   bool
}

func (f *fakeCommandExecutor) LookPath(path string) (string, error) {
	f.lookCalled = true
	if f.commandFound {
		return path, nil
	}
	return "", errors.New("not found")
}

func (f *fakeCommandExecutor) Run(command string, args ...string) error {
	f.runCalled = true
	return f.commandErr
}

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
		chconFound       bool
		chconRunErr      error
		dirAlreadyExists bool
	}{
		"no chcon":                  {chconFound: false},
		"have chcon":                {chconFound: true},
		"chcon error":               {chconFound: true, chconRunErr: errors.New("e")},
		"volume dir already exists": {chconFound: true, dirAlreadyExists: true},
	}

	for name, testCase := range testCases {
		ce := &fakeCommandExecutor{
			commandFound: testCase.chconFound,
			commandErr:   testCase.chconRunErr,
		}

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
		path, err := nc.initializeVolumeDir(ce, nc.VolumeDir)

		if !ce.lookCalled {
			t.Errorf("%s: expected look for chcon", name)
		}
		if !testCase.chconFound && ce.runCalled {
			t.Errorf("%s: unexpected run after chcon not found", name)
		}
		if testCase.chconFound && !ce.runCalled {
			t.Errorf("%s: expected chcon run", name)
		}
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
