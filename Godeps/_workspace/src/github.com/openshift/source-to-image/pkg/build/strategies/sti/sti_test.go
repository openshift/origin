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
	BuildRequest           *api.Config
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
		config:    &api.Config{},
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
		config:    &api.Config{},
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

func (f *FakeSTI) Cleanup(*api.Config) {
	f.CleanupCalled = true
}

func (f *FakeSTI) Prepare(*api.Config) error {
	f.PrepareCalled = true
	f.SetupRequired = []string{api.Assemble, api.Run}
	f.SetupOptional = []string{api.SaveArtifacts}
	return nil
}

func (f *FakeSTI) Exists(*api.Config) bool {
	f.ExistsCalled = true
	return true
}

func (f *FakeSTI) Request() *api.Config {
	return f.BuildRequest
}

func (f *FakeSTI) Result() *api.Result {
	return f.BuildResult
}

func (f *FakeSTI) Save(*api.Config) error {
	f.SaveArtifactsCalled = true
	return f.SaveArtifactsError
}

func (f *FakeSTI) fetchSource() error {
	return f.FetchSourceError
}

func (f *FakeSTI) Download(*api.Config) error {
	return nil
}

func (f *FakeSTI) Execute(command string, r *api.Config) error {
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

func (f *FakeDockerBuild) Build(*api.Config) (*api.Result, error) {
	f.LayeredBuildCalled = true
	return nil, f.LayeredBuildError
}

func TestBuild(t *testing.T) {
	incrementalTest := []bool{false, true}
	for _, incremental := range incrementalTest {
		fh := &FakeSTI{
			BuildRequest: &api.Config{Incremental: incremental},
			BuildResult:  &api.Result{},
		}

		builder := newFakeSTI(fh)
		builder.Build(&api.Config{Incremental: incremental})

		// Verify the right scripts were configed
		if !reflect.DeepEqual(fh.SetupRequired, []string{api.Assemble, api.Run}) {
			t.Errorf("Unexpected required scripts configed: %#v", fh.SetupRequired)
		}
		if !reflect.DeepEqual(fh.SetupOptional, []string{api.SaveArtifacts}) {
			t.Errorf("Unexpected optional scripts configed: %#v", fh.SetupOptional)
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
		BuildRequest: &api.Config{
			BuilderImage: "testimage",
		},
		BuildResult:   &api.Result{},
		ExecuteError:  stierr.NewContainerError("", 1, `/bin/sh: tar: not found`),
		ExpectedError: true,
	}
	builder := newFakeSTI(fh)
	builder.Build(&api.Config{BuilderImage: "testimage"})
	// Verify layered build
	if !fh.LayeredBuildCalled {
		t.Errorf("Layered build was not called.")
	}
}

func TestBuildErrorExecute(t *testing.T) {
	fh := &FakeSTI{
		BuildRequest: &api.Config{
			BuilderImage: "testimage",
		},
		BuildResult:   &api.Result{},
		ExecuteError:  errors.New("ExecuteError"),
		ExpectedError: false,
	}
	builder := newFakeSTI(fh)
	_, err := builder.Build(&api.Config{BuilderImage: "testimage"})
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
		config:          &api.Config{},
		result:          &api.Result{},
		callbackInvoker: &test.FakeCallbackInvoker{},
	}

	s.source = &git.Clone{s.git, s.fs}
	return s
}

func TestPostExecute(t *testing.T) {
	type postExecuteTest struct {
		tag              string
		incremental      bool
		previousImageID  string
		scriptsFromImage bool
	}
	testCases := []postExecuteTest{
		// 0: tagged, incremental, without previous image
		{"test/tag", true, "", true},
		// 1: tagged, incremental, with previous image
		{"test/tag", true, "test-image", true},
		// 2: tagged, no incremental, without previous image
		{"test/tag", false, "", true},
		// 3: tagged, no incremental, with previous image
		{"test/tag", false, "test-image", true},

		// 4: no tag, incremental, without previous image
		{"", true, "", false},
		// 5: no tag, incremental, with previous image
		{"", true, "test-image", false},
		// 6: no tag, no incremental, without previous image
		{"", false, "", false},
		// 7: no tag, no incremental, with previous image
		{"", false, "test-image", false},
	}

	for i, tc := range testCases {
		bh := testBuildHandler()
		containerID := "test-container-id"
		bh.result.Messages = []string{"one", "two"}
		bh.config.CallbackURL = "https://my.callback.org/test"
		bh.config.Tag = tc.tag
		bh.config.Incremental = tc.incremental
		dh := bh.docker.(*test.FakeDocker)
		if tc.previousImageID != "" {
			bh.config.RemovePreviousImage = true
			bh.incremental = tc.incremental
			bh.docker.(*test.FakeDocker).GetImageIDResult = tc.previousImageID
		}
		ci := bh.callbackInvoker.(*test.FakeCallbackInvoker)
		if tc.scriptsFromImage {
			bh.scriptsURL = map[string]string{api.Run: "image:///usr/local/sti"}
		}
		err := bh.PostExecute(containerID, "cmd1")
		if err != nil {
			t.Errorf("(%d) Unexpected error from postExecute: %v", i, err)
		}
		// Ensure CommitContainer was called with the right parameters
		expectedCmd := []string{"cmd1/scripts/" + api.Run}
		if tc.scriptsFromImage {
			expectedCmd = []string{"/usr/local/sti/" + api.Run}
		}
		if !reflect.DeepEqual(dh.CommitContainerOpts.Command, expectedCmd) {
			t.Errorf("(%d) Unexpected commit container command: %#v", i, dh.CommitContainerOpts.Command)
		}
		if dh.CommitContainerOpts.Repository != tc.tag {
			t.Errorf("(%d) Unexpected tag commited, expected %s, got %s", i, tc.tag, dh.CommitContainerOpts.Repository)
		}
		// Ensure image removal when incremental and previousImageID present
		if tc.incremental && tc.previousImageID != "" {
			if dh.RemoveImageName != "test-image" {
				t.Errorf("(%d) Previous image was not removed: %s", i, dh.RemoveImageName)
			}
		} else {
			if dh.RemoveImageName != "" {
				t.Errorf("(%d) Unexpected image removed: %s", i, dh.RemoveImageName)
			}
		}
		// Ensure Callback was called
		if ci.CallbackURL != bh.config.CallbackURL {
			t.Errorf("(%d) Unexpected callbackURL, expected %s, got %", i, bh.config.CallbackURL, ci.CallbackURL)
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
		bh.config.WorkingDir = "/working-dir"
		bh.config.Incremental = ti.incremental
		bh.config.ForcePull = true
		bh.installedScripts = map[string]bool{api.SaveArtifacts: ti.scriptInstalled}
		bh.docker.(*test.FakeDocker).PullResult = ti.previousImage

		incremental := bh.Exists(bh.config)
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
	bh.config.WorkingDir = "/working-dir"
	bh.config.Tag = "image/tag"
	fs := bh.fs.(*test.FakeFileSystem)
	fd := bh.docker.(*test.FakeDocker)
	th := bh.tar.(*test.FakeTar)
	err := bh.Save(bh.config)
	if err != nil {
		t.Errorf("Unexpected error when saving artifacts: %v", err)
	}
	expectedArtifactDir := "/working-dir/upload/artifacts"
	if fs.MkdirDir != expectedArtifactDir {
		t.Errorf("Mkdir was not called with the expected directory: %s",
			fs.MkdirDir)
	}
	if fd.RunContainerOpts.Image != bh.config.Tag {
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
			err := bh.Save(bh.config)
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
	err := bh.Save(bh.config)
	if err != expected {
		t.Errorf("Unexpected error returned from saveArtifacts: %v", err)
	}
}

func TestSaveArtifactsExtractError(t *testing.T) {
	bh := testBuildHandler()
	th := bh.tar.(*test.FakeTar)
	expected := fmt.Errorf("extract error")
	th.ExtractTarError = expected
	err := bh.Save(bh.config)
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

		bh.config.WorkingDir = "/working-dir"
		gh.ValidCloneSpecResult = ft.validCloneSpec
		if ft.refSpecified {
			bh.config.Ref = "a-branch"
		}
		bh.config.Source = "a-repo-source"
		expectedTargetDir := "/working-dir/upload/src"
		bh.source.Download(bh.config)
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
			if fh.CopySource != "a-repo-source/." {
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
	err := rh.Prepare(rh.config)
	if err != nil {
		t.Errorf("An error occurred setting up the config handler: %v", err)
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
	err := rh.Prepare(rh.config)
	if err == nil || err.Error() != "WorkingDirError" {
		t.Errorf("An error was expected for WorkingDir, but got different: %v", err)
	}
}

func TestPrepareErrorMkdirAll(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	rh.fs.(*test.FakeFileSystem).MkdirAllError = errors.New("MkdirAllError")
	err := rh.Prepare(rh.config)
	if err == nil || err.Error() != "MkdirAllError" {
		t.Errorf("An error was expected for MkdirAll, but got different: %v", err)
	}
}

func TestPrepareErrorRequiredDownloadAndInstall(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	rh.SetScripts([]string{api.Assemble, api.Run}, []string{api.SaveArtifacts})
	rh.installer.(*test.FakeInstaller).Error = fmt.Errorf("%v", api.Assemble)
	err := rh.Prepare(rh.config)
	if err == nil || err.Error() != api.Assemble {
		t.Errorf("An error was expected for required DownloadAndInstall, but got different: %v", err)
	}
}

func TestPrepareErrorOptionalDownloadAndInstall(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	rh.SetScripts([]string{api.Assemble, api.Run}, []string{api.SaveArtifacts})
	err := rh.Prepare(rh.config)
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
	rh.config.Environment = testEnv
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
	rh.config.WorkingDir = "/working-dir"
	rh.config.BuilderImage = "test/image"
	rh.config.ForcePull = true
	th := rh.tar.(*test.FakeTar)
	th.CreateTarResult = "/working-dir/test.tar"
	fd := rh.docker.(*test.FakeDocker)
	fd.RunContainerContainerID = "1234"
	fd.RunContainerCmd = []string{"one", "two"}

	err := rh.Execute("test-command", rh.config)
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

	if ro.Image != rh.config.BuilderImage {
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
	err := rh.Execute("test-command", rh.config)
	if err == nil || err.Error() != "CreateTarError" {
		t.Errorf("An error was expected for CreateTarFile, but got different: %v", err)
	}
}

func TestExecuteErrorOpenTarFile(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	rh.fs.(*test.FakeFileSystem).OpenError = errors.New("OpenTarError")
	err := rh.Execute("test-command", rh.config)
	if err == nil || err.Error() != "OpenTarError" {
		t.Errorf("An error was expected for OpenTarFile, but got different: %v", err)
	}
}

func TestCleanup(t *testing.T) {
	rh := newFakeBaseSTI()

	rh.config.WorkingDir = "/working-dir"
	preserve := []bool{false, true}
	for _, p := range preserve {
		rh.config.PreserveWorkingDir = p
		rh.fs = &test.FakeFileSystem{}
		rh.garbage = &build.DefaultCleaner{rh.fs, rh.docker}
		rh.garbage.Cleanup(rh.config)
		removedDir := rh.fs.(*test.FakeFileSystem).RemoveDirName
		if p && removedDir != "" {
			t.Errorf("Expected working directory to be preserved, but it was removed.")
		} else if !p && removedDir == "" {
			t.Errorf("Expected working directory to be removed, but it was preserved.")
		}
	}
}
