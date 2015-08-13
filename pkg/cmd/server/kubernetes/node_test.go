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

	testCases := []struct {
		chconFound      bool
		chconRunErr     error
		removeVolumeDir bool
	}{
		{chconFound: false, removeVolumeDir: true},
		{chconFound: true, chconRunErr: nil, removeVolumeDir: true},
		{chconFound: true, chconRunErr: errors.New("e"), removeVolumeDir: true},
		{removeVolumeDir: false},
	}

	for i, testCase := range testCases {
		ce := &fakeCommandExecutor{
			commandFound: testCase.chconFound,
			commandErr:   testCase.chconRunErr,
		}
		nc := &NodeConfig{VolumeDir: path.Join(parentDir, "somedir")}

		if testCase.removeVolumeDir {
			if err := os.RemoveAll(nc.VolumeDir); err != nil {
				t.Fatalf("%d: Error removing volume dir: %s", i, err)
			}
		}

		volumePath, pathErr := nc.initializeVolumeDir(ce, nc.VolumeDir)

		if testCase.removeVolumeDir {
			if !ce.lookCalled {
				t.Fatalf("%d: expected look for chcon", i)
			}
			if !testCase.chconFound && ce.runCalled {
				t.Fatalf("%d: unexpected run after chcon not found", i)
			}
			if testCase.chconFound && !ce.runCalled {
				t.Fatalf("%d: expected chcon run", i)
			}
			if pathErr != nil {
				t.Fatalf("%d: unexpected err: %s", i, pathErr)
			}
			if volumePath != nc.VolumeDir {
				t.Fatalf("%d:, expected path(%s) == nc.VolumeDir(%s)", i, volumePath, nc.VolumeDir)
			}
		} else {
			if ce.lookCalled {
				t.Fatalf("%d: unexpected look for chcon with reused volume dir", i)
			}
			if ce.runCalled {
				t.Fatalf("%d: unexpected run for chcon with reused volume dir", i)
			}
		}
	}
}
