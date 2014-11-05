package sti

import (
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/sti/test"
)

func testRequestHandler() *requestHandler {
	return &requestHandler{
		docker:    &test.FakeDocker{},
		installer: &test.FakeInstaller{},
		fs:        &test.FakeFileSystem{},
		tar:       &test.FakeTar{},

		request: &STIRequest{},
		result:  &STIResult{},
	}
}

func TestSetup(t *testing.T) {
	rh := testRequestHandler()
	rh.fs.(*test.FakeFileSystem).WorkingDirResult = "/working-dir"
	err := rh.setup([]string{"required1", "required2"}, []string{"optional1", "optional2"})
	if err != nil {
		t.Errorf("An error occurred setting up the request handler: %v", err)
	}
	if !rh.fs.(*test.FakeFileSystem).WorkingDirCalled {
		t.Errorf("Working directory was not created.")
	}
	mkdirs := rh.fs.(*test.FakeFileSystem).MkdirAllDir
	if !reflect.DeepEqual(
		mkdirs,
		[]string{"/working-dir/upload/scripts",
			"/working-dir/downloads/scripts",
			"/working-dir/downloads/defaultScripts"}) {
		t.Errorf("Unexpected set of MkdirAll calls: %#v", mkdirs)
	}
	requiredFlags := rh.installer.(*test.FakeInstaller).Required
	if !reflect.DeepEqual(requiredFlags, []bool{true, false}) {
		t.Errorf("Unexpected set of required flags: %#v", requiredFlags)
	}
	scripts := rh.installer.(*test.FakeInstaller).Scripts
	if !reflect.DeepEqual(scripts[0], []string{"required1", "required2"}) {
		t.Errorf("Unexpected set of required scripts: %#v", scripts[0])
	}
	if !reflect.DeepEqual(scripts[1], []string{"optional1", "optional2"}) {
		t.Errorf("Unexpected set of optional scripts: %#v", scripts[1])
	}
}

func equalArrayContents(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, e := range a {
		found := false
		for _, f := range b {
			if f == e {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestGenerateConfigEnv(t *testing.T) {
	rh := testRequestHandler()
	testEnv := map[string]string{
		"Key1": "Value1",
		"Key2": "Value2",
		"Key3": "Value3",
	}
	rh.request.Environment = testEnv
	result := rh.generateConfigEnv()
	expected := []string{"Key1=Value1", "Key2=Value2", "Key3=Value3"}
	if !equalArrayContents(result, expected) {
		t.Errorf("Unexpected result. Expected: %#v. Actual: %#v",
			expected, result)
	}
}

type FakePostExecutor struct {
	containerID string
	cmd         []string
}

func (f *FakePostExecutor) PostExecute(id string, cmd []string) error {
	f.containerID = id
	f.cmd = cmd
	return nil
}

func TestExecute(t *testing.T) {
	rh := testRequestHandler()
	pe := &FakePostExecutor{}
	rh.postExecutor = pe
	rh.request.workingDir = "/working-dir"
	rh.request.BaseImage = "test/image"
	th := rh.tar.(*test.FakeTar)
	th.CreateTarResult = "/working-dir/test.tar"
	fd := rh.docker.(*test.FakeDocker)
	fd.RunContainerContainerID = "1234"
	fd.RunContainerCmd = []string{"one", "two"}

	err := rh.execute("test-command")
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	if !rh.result.Success {
		t.Errorf("Execute was not successful.")
	}
	if th.CreateTarBase != "/working-dir" {
		t.Errorf("Unexpected tar base directory: %s", th.CreateTarBase)
	}
	if th.CreateTarDir != "/working-dir/upload" {
		t.Errorf("Unexpected tar directory: %s", th.CreateTarDir)
	}
	fh := rh.fs.(*test.FakeFileSystem)
	if fh.OpenFile != "/working-dir/test.tar" {
		t.Errorf("Unexpected file opened: %s", fh.OpenFile)
	}
	if !fh.OpenFileResult.CloseCalled {
		t.Errorf("Tar file was not closed.")
	}
	ro := fd.RunContainerOpts

	if ro.Image != rh.request.BaseImage {
		t.Errorf("Unexpected Image passed to RunContainer")
	}
	if ro.Stdin != fh.OpenFileResult {
		t.Errorf("Unexpected input stream: %#v", fd.RunContainerOpts.Stdin)
	}
	if !ro.PullImage {
		t.Errorf("PullImage is not true for RunContainer")
	}
	if ro.Command != "test-command" {
		t.Errorf("Unexpected command passed to RunContainer: %s",
			fd.RunContainerOpts.Command)
	}
	if pe.containerID != "1234" {
		t.Errorf("PostExecutor not called with expected ID: %s",
			pe.containerID)
	}
	if !reflect.DeepEqual(pe.cmd, []string{"one", "two", "test-command"}) {
		t.Errorf("PostExecutor not called with expected command: %#v", pe.cmd)
	}
}

func TestCleanup(t *testing.T) {
	rh := testRequestHandler()
	rh.request.workingDir = "/working-dir"
	preserve := []bool{false, true}
	for _, p := range preserve {
		rh.request.PreserveWorkingDir = p
		rh.fs = &test.FakeFileSystem{}
		rh.cleanup()
		removedDir := rh.fs.(*test.FakeFileSystem).RemoveDirName
		if p && removedDir != "" {
			t.Errorf("Expected working directory to be preserved, but it was removed.")
		} else if !p && removedDir == "" {
			t.Errorf("Expected working directory to be removed, but it was preserved.")
		}
	}
}
