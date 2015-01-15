package sti

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"github.com/openshift/source-to-image/pkg/sti/api"
	"github.com/openshift/source-to-image/pkg/sti/test"
)

func testRequestHandler() *requestHandler {
	return &requestHandler{
		docker:    &test.FakeDocker{},
		installer: &test.FakeInstaller{},
		git:       &test.FakeGit{},
		fs:        &test.FakeFileSystem{},
		tar:       &test.FakeTar{},

		request: &api.Request{},
		result:  &api.Result{},
	}
}

func TestSetupOK(t *testing.T) {
	rh := testRequestHandler()
	rh.fs.(*test.FakeFileSystem).WorkingDirResult = "/working-dir"
	err := rh.setup([]api.Script{api.Assemble, api.Run}, []api.Script{api.SaveArtifacts})
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
	if !reflect.DeepEqual(scripts[0], []api.Script{api.Assemble, api.Run}) {
		t.Errorf("Unexpected set of required scripts: %#v", scripts[0])
	}
	if !reflect.DeepEqual(scripts[1], []api.Script{api.SaveArtifacts}) {
		t.Errorf("Unexpected set of optional scripts: %#v", scripts[1])
	}
}

func TestSetupErrorCreatingWorkingDir(t *testing.T) {
	rh := testRequestHandler()
	rh.fs.(*test.FakeFileSystem).WorkingDirError = errors.New("WorkingDirError")
	err := rh.setup([]api.Script{api.Assemble, api.Run}, []api.Script{api.SaveArtifacts})
	if err == nil || err.Error() != "WorkingDirError" {
		t.Errorf("An error was expected for WorkingDir, but got different: %v", err)
	}
}

func TestSetupErrorMkdirAll(t *testing.T) {
	rh := testRequestHandler()
	rh.fs.(*test.FakeFileSystem).MkdirAllError = errors.New("MkdirAllError")
	err := rh.setup([]api.Script{api.Assemble, api.Run}, []api.Script{api.SaveArtifacts})
	if err == nil || err.Error() != "MkdirAllError" {
		t.Errorf("An error was expected for MkdirAll, but got different: %v", err)
	}
}

func TestSetupErrorRequiredDownloadAndInstall(t *testing.T) {
	rh := testRequestHandler()
	rh.installer.(*test.FakeInstaller).ErrScript = api.Assemble
	err := rh.setup([]api.Script{api.Assemble, api.Run}, []api.Script{api.SaveArtifacts})
	if err == nil || err.Error() != string(api.Assemble) {
		t.Errorf("An error was expected for required DownloadAndInstall, but got different: %v", err)
	}
}

func TestSetupErrorOptionalDownloadAndInstall(t *testing.T) {
	rh := testRequestHandler()
	rh.installer.(*test.FakeInstaller).ErrScript = api.SaveArtifacts
	err := rh.setup([]api.Script{api.Assemble, api.Run}, []api.Script{api.SaveArtifacts})
	if err != nil {
		t.Errorf("Unexpected error when downloading optional scripts: %v", err)
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
	location    string
	err         error
}

func (f *FakePostExecutor) PostExecute(id string, location string) error {
	fmt.Errorf("Post execute called!!!!")
	f.containerID = id
	f.location = location
	return f.err
}

func TestExecuteOK(t *testing.T) {
	rh := testRequestHandler()
	pe := &FakePostExecutor{}
	rh.postExecutor = pe
	rh.request.WorkingDir = "/working-dir"
	rh.request.BaseImage = "test/image"
	rh.request.ForcePull = true
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
	if !reflect.DeepEqual(pe.location, "test-command") {
		t.Errorf("PostExecutor not called with expected command: %s", pe.location)
	}
}

func TestExecuteErrorCreateTarFile(t *testing.T) {
	rh := testRequestHandler()
	rh.tar.(*test.FakeTar).CreateTarError = errors.New("CreateTarError")
	err := rh.execute("test-command")
	if err == nil || err.Error() != "CreateTarError" {
		t.Errorf("An error was expected for CreateTarFile, but got different: %v", err)
	}
}

func TestExecuteErrorOpenTarFile(t *testing.T) {
	rh := testRequestHandler()
	rh.fs.(*test.FakeFileSystem).OpenError = errors.New("OpenTarError")
	err := rh.execute("test-command")
	if err == nil || err.Error() != "OpenTarError" {
		t.Errorf("An error was expected for OpenTarFile, but got different: %v", err)
	}
}

func TestBuildOK(t *testing.T) {
	rh := testRequestHandler()
	rh.request.BaseImage = "test/image"
	err := rh.build()
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	if !rh.request.LayeredBuild {
		t.Errorf("Expected LayeredBuild to be true!")
	}
	if m, _ := regexp.MatchString(`test/image-\d+`, rh.request.BaseImage); !m {
		t.Errorf("Expected BaseImage test/image-withnumbers, but got %s", rh.request.BaseImage)
	}
	if rh.request.ExternalRequiredScripts {
		t.Errorf("Expected ExternalRequiredScripts to be false!")
	}
	if rh.request.ScriptsURL != "image:///tmp/scripts" {
		t.Error("Expected ScriptsURL image:///tmp/scripts, but got %s", rh.request.ScriptsURL)
	}
	if rh.request.Location != "/tmp/src" {
		t.Errorf("Expected Location /tmp/src, but got %s", rh.request.Location)
	}
}

func TestBuildErrorWriteDockerfile(t *testing.T) {
	rh := testRequestHandler()
	rh.fs.(*test.FakeFileSystem).WriteFileError = errors.New("WriteDockerfileError")
	err := rh.build()
	if err == nil || err.Error() != "WriteDockerfileError" {
		t.Error("An error was expected for WriteDockerfile, but got different: %v", err)
	}
}

func TestBuildErrorCreateTarFile(t *testing.T) {
	rh := testRequestHandler()
	rh.tar.(*test.FakeTar).CreateTarError = errors.New("CreateTarError")
	err := rh.build()
	if err == nil || err.Error() != "CreateTarError" {
		t.Error("An error was expected for CreateTar, but got different: %v", err)
	}
}

func TestBuildErrorOpenTarFile(t *testing.T) {
	rh := testRequestHandler()
	rh.fs.(*test.FakeFileSystem).OpenError = errors.New("OpenTarError")
	err := rh.build()
	if err == nil || err.Error() != "OpenTarError" {
		t.Errorf("An error was expected for OpenTarFile, but got different: %v", err)
	}
}

func TestBuildErrorBuildImage(t *testing.T) {
	rh := testRequestHandler()
	rh.docker.(*test.FakeDocker).BuildImageError = errors.New("BuildImageError")
	err := rh.build()
	if err == nil || err.Error() != "BuildImageError" {
		t.Errorf("An error was expected for BuildImage, but got different: %v", err)
	}
}

func TestCleanup(t *testing.T) {
	rh := testRequestHandler()
	rh.request.WorkingDir = "/working-dir"
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
