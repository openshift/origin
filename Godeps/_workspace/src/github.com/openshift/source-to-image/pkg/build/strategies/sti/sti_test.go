package sti

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	stierr "github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/git"
	"github.com/openshift/source-to-image/pkg/test"
)

type FakeSTI struct {
	CleanupCalled          bool
	PrepareCalled          bool
	SetupRequired          []string
	SetupOptional          []string
	SetupError             error
	ExistsCalled           bool
	ExistsError            error
	BuildRequest           *api.Request
	BuildResult            *api.Result
	SaveArtifactsCalled    bool
	SaveArtifactsError     error
	FetchSourceCalled      bool
	FetchSourceError       error
	ExecuteCommand         string
	ExecuteError           error
	ExpectedError          bool
	LayeredBuildCalled     bool
	LayeredBuildError      error
	PostExecuteLocation    string
	PostExecuteContainerID string
	PostExecuteError       error
}

func newFakeBaseSTI() *STI {
	return &STI{
		request:   &api.Request{},
		result:    &api.Result{},
		docker:    &test.FakeDocker{},
		installer: &test.FakeInstaller{},
		git:       &test.FakeGit{},
		fs:        &test.FakeFileSystem{},
		tar:       &test.FakeTar{},
	}
}

func newFakeSTI(f *FakeSTI) *STI {
	s := &STI{
		request:   &api.Request{},
		result:    &api.Result{},
		docker:    &test.FakeDocker{},
		installer: &test.FakeInstaller{},
		git:       &test.FakeGit{},
		fs:        &test.FakeFileSystem{},
		tar:       &test.FakeTar{},
		preparer:  f,
		artifacts: f,
		scripts:   f,
		garbage:   f,
		layered:   &FakeDockerBuild{f},
	}
	s.source = &git.Clone{s.git, s.fs}
	return s
}

func (f *FakeSTI) Cleanup(*api.Request) {
	f.CleanupCalled = true
}

func (f *FakeSTI) Prepare(*api.Request) error {
	f.PrepareCalled = true
	f.SetupRequired = []string{api.Assemble, api.Run}
	f.SetupOptional = []string{api.SaveArtifacts}
	return nil
}

func (f *FakeSTI) Exists(*api.Request) bool {
	f.ExistsCalled = true
	return true
}

func (f *FakeSTI) Request() *api.Request {
	return f.BuildRequest
}

func (f *FakeSTI) Result() *api.Result {
	return f.BuildResult
}

func (f *FakeSTI) Save(*api.Request) error {
	f.SaveArtifactsCalled = true
	return f.SaveArtifactsError
}

func (f *FakeSTI) fetchSource() error {
	return f.FetchSourceError
}

func (f *FakeSTI) Download(*api.Request) error {
	return nil
}

func (f *FakeSTI) Execute(command string, r *api.Request) error {
	f.ExecuteCommand = command
	return f.ExecuteError
}

func (f *FakeSTI) wasExpectedError(text string) bool {
	return f.ExpectedError
}

func (f *FakeSTI) PostExecute(id, location string) error {
	f.PostExecuteContainerID = id
	f.PostExecuteLocation = location
	return f.PostExecuteError
}

type FakeDockerBuild struct {
	*FakeSTI
}

func (f *FakeDockerBuild) Build(*api.Request) (*api.Result, error) {
	f.LayeredBuildCalled = true
	return nil, f.LayeredBuildError
}

func TestBuild(t *testing.T) {
	incrementalTest := []bool{false, true}
	for _, incremental := range incrementalTest {
		fh := &FakeSTI{
			BuildRequest: &api.Request{Incremental: incremental},
			BuildResult:  &api.Result{},
		}

		builder := newFakeSTI(fh)
		builder.Build(&api.Request{Incremental: incremental})

		// Verify the right scripts were requested
		if !reflect.DeepEqual(fh.SetupRequired, []string{api.Assemble, api.Run}) {
			t.Errorf("Unexpected required scripts requested: %#v", fh.SetupRequired)
		}
		if !reflect.DeepEqual(fh.SetupOptional, []string{api.SaveArtifacts}) {
			t.Errorf("Unexpected optional scripts requested: %#v", fh.SetupOptional)
		}

		// Verify that Exists was called
		if !fh.ExistsCalled {
			t.Errorf("Exists was not called.")
		}

		// Verify that Save was called for an incremental build
		if incremental && !fh.SaveArtifactsCalled {
			t.Errorf("Save artifacts was not called for an incremental build")
		}

		// Verify that Execute was called with the right script
		if fh.ExecuteCommand != api.Assemble {
			t.Errorf("Unexpected execute command: %s", fh.ExecuteCommand)
		}
	}
}

func TestLayeredBuild(t *testing.T) {
	fh := &FakeSTI{
		BuildRequest: &api.Request{
			BaseImage: "testimage",
		},
		BuildResult:   &api.Result{},
		ExecuteError:  stierr.NewContainerError("", 1, `/bin/sh: tar: not found`),
		ExpectedError: true,
	}
	builder := newFakeSTI(fh)
	builder.Build(&api.Request{BaseImage: "testimage"})
	// Verify layered build
	if !fh.LayeredBuildCalled {
		t.Errorf("Layered build was not called.")
	}
}

func TestBuildErrorExecute(t *testing.T) {
	fh := &FakeSTI{
		BuildRequest: &api.Request{
			BaseImage: "testimage",
		},
		BuildResult:   &api.Result{},
		ExecuteError:  errors.New("ExecuteError"),
		ExpectedError: false,
	}
	builder := newFakeSTI(fh)
	_, err := builder.Build(&api.Request{BaseImage: "testimage"})
	if err == nil || err.Error() != "ExecuteError" {
		t.Errorf("An error was expected, but got different %v", err)
	}
}

func TestWasExpectedError(t *testing.T) {
	type expErr struct {
		text     string
		expected bool
	}

	tests := []expErr{
		{ // 0 - tar error
			text:     `/bin/sh: tar: not found`,
			expected: true,
		},
		{ // 1 - tar error
			text:     `/bin/sh: tar: command not found`,
			expected: true,
		},
		{ // 2 - /bin/sh error
			text:     `exec: "/bin/sh": stat /bin/sh: no such file or directory`,
			expected: true,
		},
		{ // 3 - non container error
			text:     "other error",
			expected: false,
		},
	}

	for i, ti := range tests {
		result := isMissingRequirements(ti.text)
		if result != ti.expected {
			t.Errorf("(%d) Unexpected result: %v. Expected: %v", i, result, ti.expected)
		}
	}
}

func testBuildHandler() *STI {
	s := &STI{
		docker:          &test.FakeDocker{},
		installer:       &test.FakeInstaller{},
		git:             &test.FakeGit{},
		fs:              &test.FakeFileSystem{},
		tar:             &test.FakeTar{},
		request:         &api.Request{},
		result:          &api.Result{},
		callbackInvoker: &test.FakeCallbackInvoker{},
	}

	s.source = &git.Clone{s.git, s.fs}
	return s
}

func TestPostExecute(t *testing.T) {
	incrementalTest := []bool{true, false}
	for _, incremental := range incrementalTest {
		previousImageIDTest := []string{"", "test-image"}
		for _, previousImageID := range previousImageIDTest {
			bh := testBuildHandler()
			bh.result.Messages = []string{"one", "two"}
			bh.request.CallbackURL = "https://my.callback.org/test"
			bh.request.Tag = "test/tag"
			dh := bh.docker.(*test.FakeDocker)
			bh.request.Incremental = incremental
			if previousImageID != "" {
				bh.request.RemovePreviousImage = true
				bh.incremental = incremental
				bh.docker.(*test.FakeDocker).GetImageIDResult = previousImageID
			}
			err := bh.PostExecute("test-container-id", "cmd1")
			if err != nil {
				t.Errorf("Unexpected errror from postExecute: %v", err)
			}
			// Ensure CommitContainer was called with the right parameters
			if !reflect.DeepEqual(dh.CommitContainerOpts.Command, []string{"cmd1/run"}) {
				t.Errorf("Unexpected commit container command: %#v", dh.CommitContainerOpts.Command)
			}
			if dh.CommitContainerOpts.Repository != bh.request.Tag {
				t.Errorf("Unexpected tag commited: %s", dh.CommitContainerOpts.Repository)
			}

			if incremental && previousImageID != "" {
				if dh.RemoveImageName != "test-image" {
					t.Errorf("Previous image was not removed: %s", dh.RemoveImageName)
				}
			} else {
				if dh.RemoveImageName != "" {
					t.Errorf("Unexpected image removed: %s", dh.RemoveImageName)
				}
			}

		}
	}
}

func TestExists(t *testing.T) {
	type incrementalTest struct {
		// incremental flag was passed
		incremental bool
		// previous image existence
		previousImage bool
		// script installed
		scriptInstalled bool
		// expected result
		expected bool
	}

	tests := []incrementalTest{
		// 0-1: incremental, no image, no matter what with scripts
		{true, false, false, false},
		{true, false, true, false},

		// 2: incremental, previous image, no scripts
		{true, true, false, false},
		// 3: incremental, previous image, scripts installed
		{true, true, true, true},

		// 4-7: no incremental build - should always return false no matter what other flags are
		{false, false, false, false},
		{false, false, true, false},
		{false, true, false, false},
		{false, true, true, false},
	}

	for i, ti := range tests {
		bh := testBuildHandler()
		bh.request.WorkingDir = "/working-dir"
		bh.request.Incremental = ti.incremental
		bh.installedScripts = map[string]bool{api.SaveArtifacts: ti.scriptInstalled}
		bh.docker.(*test.FakeDocker).PullResult = ti.previousImage

		incremental := bh.Exists(bh.request)
		if incremental != ti.expected {
			t.Errorf("(%d) Unexpected incremental result: %v. Expected: %v",
				i, incremental, ti.expected)
		}
		if ti.incremental && ti.previousImage && ti.scriptInstalled {
			if len(bh.fs.(*test.FakeFileSystem).ExistsFile) == 0 {
				continue
			}
			scriptChecked := bh.fs.(*test.FakeFileSystem).ExistsFile[0]
			expectedScript := "/working-dir/upload/scripts/save-artifacts"
			if scriptChecked != expectedScript {
				t.Errorf("(%d) Unexpected script checked. Actual: %s. Expected: %s",
					i, scriptChecked, expectedScript)
			}
		}
	}
}

func TestSaveArtifacts(t *testing.T) {
	bh := testBuildHandler()
	bh.request.WorkingDir = "/working-dir"
	bh.request.Tag = "image/tag"
	fs := bh.fs.(*test.FakeFileSystem)
	fd := bh.docker.(*test.FakeDocker)
	th := bh.tar.(*test.FakeTar)
	err := bh.Save(bh.request)
	if err != nil {
		t.Errorf("Unexpected error when saving artifacts: %v", err)
	}
	expectedArtifactDir := "/working-dir/upload/artifacts"
	if fs.MkdirDir != expectedArtifactDir {
		t.Errorf("Mkdir was not called with the expected directory: %s",
			fs.MkdirDir)
	}
	if fd.RunContainerOpts.Image != bh.request.Tag {
		t.Errorf("Unexpected image sent to RunContainer: %s",
			fd.RunContainerOpts.Image)
	}
	if th.ExtractTarDir != expectedArtifactDir || th.ExtractTarReader == nil {
		t.Errorf("ExtractTar was not called with the expected parameters.")
	}
}

func TestSaveArtifactsRunError(t *testing.T) {
	tests := []error{
		fmt.Errorf("Run error"),
		stierr.NewContainerError("", -1, ""),
	}
	expected := []error{
		tests[0],
		stierr.NewSaveArtifactsError("", "", tests[1]),
	}
	// test with tar extract error or not
	tarError := []bool{true, false}
	for i := range tests {
		for _, te := range tarError {
			bh := testBuildHandler()
			fd := bh.docker.(*test.FakeDocker)
			th := bh.tar.(*test.FakeTar)
			fd.RunContainerError = tests[i]
			if te {
				th.ExtractTarError = fmt.Errorf("tar error")
			}
			err := bh.Save(bh.request)
			if !te && err != expected[i] {
				t.Errorf("Unexpected error returned from saveArtifacts: %v", err)
			} else if te && err != th.ExtractTarError {
				t.Errorf("Expected tar error. Got %v", err)
			}
		}
	}
}

func TestSaveArtifactsErrorBeforeStart(t *testing.T) {
	bh := testBuildHandler()
	fd := bh.docker.(*test.FakeDocker)
	expected := fmt.Errorf("run error")
	fd.RunContainerError = expected
	fd.RunContainerErrorBeforeStart = true
	err := bh.Save(bh.request)
	if err != expected {
		t.Errorf("Unexpected error returned from saveArtifacts: %v", err)
	}
}

func TestSaveArtifactsExtractError(t *testing.T) {
	bh := testBuildHandler()
	th := bh.tar.(*test.FakeTar)
	expected := fmt.Errorf("extract error")
	th.ExtractTarError = expected
	err := bh.Save(bh.request)
	if err != expected {
		t.Errorf("Unexpected error returned from saveArtifacts: %v", err)
	}
}

func TestFetchSource(t *testing.T) {
	type fetchTest struct {
		validCloneSpec   bool
		refSpecified     bool
		cloneExpected    bool
		checkoutExpected bool
		copyExpected     bool
	}

	tests := []fetchTest{
		{
			validCloneSpec:   false,
			refSpecified:     false,
			cloneExpected:    false,
			checkoutExpected: false,
			copyExpected:     true,
		},
		{
			validCloneSpec:   true,
			refSpecified:     false,
			cloneExpected:    true,
			checkoutExpected: false,
			copyExpected:     false,
		},
		{
			validCloneSpec:   true,
			refSpecified:     true,
			cloneExpected:    true,
			checkoutExpected: true,
			copyExpected:     false,
		},
	}

	for _, ft := range tests {
		bh := testBuildHandler()
		gh := bh.git.(*test.FakeGit)
		fh := bh.fs.(*test.FakeFileSystem)

		bh.request.WorkingDir = "/working-dir"
		gh.ValidCloneSpecResult = ft.validCloneSpec
		if ft.refSpecified {
			bh.request.Ref = "a-branch"
		}
		bh.request.Source = "a-repo-source"
		expectedTargetDir := "/working-dir/upload/src"
		bh.source.Download(bh.request)
		if ft.cloneExpected {
			if gh.CloneSource != "a-repo-source" {
				t.Errorf("Clone was not called with the expected source.")
			}
			if gh.CloneTarget != expectedTargetDir {
				t.Errorf("Unexpected target dirrectory for clone operation.")
			}
		}
		if ft.checkoutExpected {
			if gh.CheckoutRef != "a-branch" {
				t.Errorf("Checkout was not called with the expected branch.")
			}
			if gh.CheckoutRepo != expectedTargetDir {
				t.Errorf("Unexpected target repository for checkout operation.")
			}
		}
		if ft.copyExpected {
			if fh.CopySource != "a-repo-source" {
				t.Errorf("Copy was not called with the expected source.")
			}
			if fh.CopyDest != expectedTargetDir {
				t.Errorf("Unexpected target director for copy operation.")
			}
		}
	}
}

func TestPrepareOK(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	rh.SetScripts([]string{api.Assemble, api.Run}, []string{api.SaveArtifacts})
	rh.fs.(*test.FakeFileSystem).WorkingDirResult = "/working-dir"
	err := rh.Prepare(rh.request)
	if err != nil {
		t.Errorf("An error occurred setting up the request handler: %v", err)
	}
	if !rh.fs.(*test.FakeFileSystem).WorkingDirCalled {
		t.Errorf("Working directory was not created.")
	}
	var expected []string
	for _, dir := range workingDirs {
		expected = append(expected, "/working-dir/"+dir)
	}
	mkdirs := rh.fs.(*test.FakeFileSystem).MkdirAllDir
	if !reflect.DeepEqual(mkdirs, expected) {
		t.Errorf("Unexpected set of MkdirAll calls: %#v", mkdirs)
	}
	scripts := rh.installer.(*test.FakeInstaller).Scripts
	if !reflect.DeepEqual(scripts[0], []string{api.Assemble, api.Run}) {
		t.Errorf("Unexpected set of required scripts: %#v", scripts[0])
	}
	if !reflect.DeepEqual(scripts[1], []string{api.SaveArtifacts}) {
		t.Errorf("Unexpected set of optional scripts: %#v", scripts[1])
	}
}

func TestPrepareErrorCreatingWorkingDir(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	rh.fs.(*test.FakeFileSystem).WorkingDirError = errors.New("WorkingDirError")
	err := rh.Prepare(rh.request)
	if err == nil || err.Error() != "WorkingDirError" {
		t.Errorf("An error was expected for WorkingDir, but got different: %v", err)
	}
}

func TestPrepareErrorMkdirAll(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	rh.fs.(*test.FakeFileSystem).MkdirAllError = errors.New("MkdirAllError")
	err := rh.Prepare(rh.request)
	if err == nil || err.Error() != "MkdirAllError" {
		t.Errorf("An error was expected for MkdirAll, but got different: %v", err)
	}
}

func TestPrepareErrorRequiredDownloadAndInstall(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	rh.SetScripts([]string{api.Assemble, api.Run}, []string{api.SaveArtifacts})
	rh.installer.(*test.FakeInstaller).Error = fmt.Errorf("%v", api.Assemble)
	err := rh.Prepare(rh.request)
	if err == nil || err.Error() != api.Assemble {
		t.Errorf("An error was expected for required DownloadAndInstall, but got different: %v", err)
	}
}

func TestPrepareErrorOptionalDownloadAndInstall(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	rh.SetScripts([]string{api.Assemble, api.Run}, []string{api.SaveArtifacts})
	err := rh.Prepare(rh.request)
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
	rh := newFakeSTI(&FakeSTI{})
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

func TestExecuteOK(t *testing.T) {
	rh := newFakeBaseSTI()
	pe := &FakeSTI{}
	rh.postExecutor = pe
	rh.request.WorkingDir = "/working-dir"
	rh.request.BaseImage = "test/image"
	rh.request.ForcePull = true
	th := rh.tar.(*test.FakeTar)
	th.CreateTarResult = "/working-dir/test.tar"
	fd := rh.docker.(*test.FakeDocker)
	fd.RunContainerContainerID = "1234"
	fd.RunContainerCmd = []string{"one", "two"}

	err := rh.Execute("test-command", rh.request)
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	if th.CreateTarBase != "/working-dir" {
		t.Errorf("Unexpected tar base directory: %s", th.CreateTarBase)
	}
	if th.CreateTarDir != "/working-dir/upload" {
		t.Errorf("Unexpected tar directory: %s", th.CreateTarDir)
	}
	fh, ok := rh.fs.(*test.FakeFileSystem)
	if !ok {
		t.Fatalf("Unable to convert %v to FakeFilesystem", rh.fs)
	}
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
	if pe.PostExecuteContainerID != "1234" {
		t.Errorf("PostExecutor not called with expected ID: %s",
			pe.PostExecuteContainerID)
	}
	if !reflect.DeepEqual(pe.PostExecuteLocation, "test-command") {
		t.Errorf("PostExecutor not called with expected command: %s", pe.PostExecuteLocation)
	}
}

func TestExecuteErrorCreateTarFile(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	rh.tar.(*test.FakeTar).CreateTarError = errors.New("CreateTarError")
	err := rh.Execute("test-command", rh.request)
	if err == nil || err.Error() != "CreateTarError" {
		t.Errorf("An error was expected for CreateTarFile, but got different: %v", err)
	}
}

func TestExecuteErrorOpenTarFile(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	rh.fs.(*test.FakeFileSystem).OpenError = errors.New("OpenTarError")
	err := rh.Execute("test-command", rh.request)
	if err == nil || err.Error() != "OpenTarError" {
		t.Errorf("An error was expected for OpenTarFile, but got different: %v", err)
	}
}

func TestCleanup(t *testing.T) {
	rh := newFakeBaseSTI()

	rh.request.WorkingDir = "/working-dir"
	preserve := []bool{false, true}
	for _, p := range preserve {
		rh.request.PreserveWorkingDir = p
		rh.fs = &test.FakeFileSystem{}
		rh.garbage = &build.DefaultCleaner{rh.fs, rh.docker}
		rh.garbage.Cleanup(rh.request)
		removedDir := rh.fs.(*test.FakeFileSystem).RemoveDirName
		if p && removedDir != "" {
			t.Errorf("Expected working directory to be preserved, but it was removed.")
		} else if !p && removedDir == "" {
			t.Errorf("Expected working directory to be removed, but it was preserved.")
		}
	}
}
