package sti

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"regexp/syntax"
	"strings"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/docker"
	s2ierr "github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/ignore"
	"github.com/openshift/source-to-image/pkg/scm/empty"
	"github.com/openshift/source-to-image/pkg/scm/file"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/test"
	"github.com/openshift/source-to-image/pkg/util"
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
	DownloadError          error
	SaveArtifactsCalled    bool
	SaveArtifactsError     error
	FetchSourceCalled      bool
	FetchSourceError       error
	ExecuteCommand         string
	ExecuteUser            string
	ExecuteError           error
	ExpectedError          bool
	LayeredBuildCalled     bool
	LayeredBuildError      error
	PostExecuteDestination string
	PostExecuteContainerID string
	PostExecuteError       error
}

func newFakeBaseSTI() *STI {
	return &STI{
		config:    &api.Config{},
		result:    &api.Result{},
		docker:    &docker.FakeDocker{},
		installer: &test.FakeInstaller{},
		git:       &test.FakeGit{},
		fs:        &test.FakeFileSystem{},
		tar:       &test.FakeTar{},
	}
}

func newFakeSTI(f *FakeSTI) *STI {
	s := &STI{
		config:        &api.Config{},
		result:        &api.Result{},
		docker:        &docker.FakeDocker{},
		runtimeDocker: &docker.FakeDocker{},
		installer:     &test.FakeInstaller{},
		git:           &test.FakeGit{},
		fs:            &test.FakeFileSystem{},
		tar:           &test.FakeTar{},
		preparer:      f,
		ignorer:       &ignore.DockerIgnorer{},
		artifacts:     f,
		scripts:       f,
		garbage:       f,
		layered:       &FakeDockerBuild{f},
	}
	s.source = &git.Clone{Git: s.git, FileSystem: s.fs}
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

func (f *FakeSTI) Download(*api.Config) (*api.SourceInfo, error) {
	return nil, f.DownloadError
}

func (f *FakeSTI) Execute(command string, user string, r *api.Config) error {
	f.ExecuteCommand = command
	f.ExecuteUser = user
	return f.ExecuteError
}

func (f *FakeSTI) wasExpectedError(text string) bool {
	return f.ExpectedError
}

func (f *FakeSTI) PostExecute(id, destination string) error {
	f.PostExecuteContainerID = id
	f.PostExecuteDestination = destination
	return f.PostExecuteError
}

type FakeDockerBuild struct {
	*FakeSTI
}

func (f *FakeDockerBuild) Build(*api.Config) (*api.Result, error) {
	f.LayeredBuildCalled = true
	return &api.Result{}, f.LayeredBuildError
}

func TestDefaultSource(t *testing.T) {
	config := &api.Config{
		Source:       "file://.",
		DockerConfig: &api.DockerConfig{Endpoint: "unix:///var/run/docker.sock"},
	}
	client, err := docker.NewEngineAPIClient(config.DockerConfig)
	if err != nil {
		t.Fatal(err)
	}
	sti, err := New(client, config, util.NewFileSystem(), build.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if config.Source == "" {
		t.Errorf("Config.Source not set: %v", config.Source)
	}
	if _, ok := sti.source.(*file.File); !ok || sti.source == nil {
		t.Errorf("Source interface not set: %#v", sti.source)
	}
}

func TestEmptySource(t *testing.T) {
	config := &api.Config{
		Source:       "",
		DockerConfig: &api.DockerConfig{Endpoint: "unix:///var/run/docker.sock"},
	}
	client, err := docker.NewEngineAPIClient(config.DockerConfig)
	if err != nil {
		t.Fatal(err)
	}
	sti, err := New(client, config, util.NewFileSystem(), build.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if config.Source != "" {
		t.Errorf("Config.Source unexpectantly changed: %v", config.Source)
	}
	if _, ok := sti.source.(*empty.Noop); !ok || sti.source == nil {
		t.Errorf("Source interface not set: %#v", sti.source)
	}
}

func TestOverrides(t *testing.T) {
	fd := &FakeSTI{}
	client, err := docker.NewEngineAPIClient(&api.DockerConfig{Endpoint: "unix:///var/run/docker.sock"})
	if err != nil {
		t.Fatal(err)
	}
	sti, err := New(client,
		&api.Config{
			DockerConfig: &api.DockerConfig{Endpoint: "unix:///var/run/docker.sock"},
		},
		util.NewFileSystem(),
		build.Overrides{
			Downloader: fd,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if sti.source != fd {
		t.Errorf("Override of downloader not set: %#v", sti)
	}
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
		BuildResult: &api.Result{
			BuildInfo: api.BuildInfo{
				Stages: []api.StageInfo{},
			},
		},
		ExecuteError:  errMissingRequirements,
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
		docker:            &docker.FakeDocker{},
		incrementalDocker: &docker.FakeDocker{},
		installer:         &test.FakeInstaller{},
		git:               &test.FakeGit{},
		fs:                &test.FakeFileSystem{ExistsResult: map[string]bool{filepath.FromSlash("a-repo-source"): true}},
		tar:               &test.FakeTar{},
		config:            &api.Config{},
		result:            &api.Result{},
		callbackInvoker:   &test.FakeCallbackInvoker{},
	}
	s.source = &git.Clone{Git: s.git, FileSystem: s.fs}
	s.initPostExecutorSteps()
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
		bh.config.Tag = tc.tag
		bh.config.Incremental = tc.incremental
		dh := bh.docker.(*docker.FakeDocker)
		if tc.previousImageID != "" {
			bh.config.RemovePreviousImage = true
			bh.incremental = tc.incremental
			bh.docker.(*docker.FakeDocker).GetImageIDResult = tc.previousImageID
		}
		if tc.scriptsFromImage {
			bh.scriptsURL = map[string]string{api.Run: "image:///usr/libexec/s2i/run"}
		}
		err := bh.PostExecute(containerID, "cmd1")
		if err != nil {
			t.Errorf("(%d) Unexpected error from postExecute: %v", i, err)
		}
		// Ensure CommitContainer was called with the right parameters
		expectedCmd := []string{"cmd1/scripts/" + api.Run}
		if tc.scriptsFromImage {
			expectedCmd = []string{"/usr/libexec/s2i/" + api.Run}
		}
		if !reflect.DeepEqual(dh.CommitContainerOpts.Command, expectedCmd) {
			t.Errorf("(%d) Unexpected commit container command: %#v, expected %q", i, dh.CommitContainerOpts.Command, expectedCmd)
		}
		if dh.CommitContainerOpts.Repository != tc.tag {
			t.Errorf("(%d) Unexpected tag committed, expected %q, got %q", i, tc.tag, dh.CommitContainerOpts.Repository)
		}
		// Ensure image removal when incremental and previousImageID present
		if tc.incremental && tc.previousImageID != "" {
			if dh.RemoveImageName != "test-image" {
				t.Errorf("(%d) Previous image was not removed: %q", i, dh.RemoveImageName)
			}
		} else {
			if dh.RemoveImageName != "" {
				t.Errorf("(%d) Unexpected image removed: %s", i, dh.RemoveImageName)
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
		bh.config.WorkingDir = "/working-dir"
		bh.config.Incremental = ti.incremental
		bh.config.BuilderPullPolicy = api.PullAlways
		bh.installedScripts = map[string]bool{api.SaveArtifacts: ti.scriptInstalled}
		bh.incrementalDocker.(*docker.FakeDocker).PullResult = ti.previousImage
		bh.config.DockerConfig = &api.DockerConfig{Endpoint: "http://localhost:4243"}
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
	fd := bh.docker.(*docker.FakeDocker)
	th := bh.tar.(*test.FakeTar)
	err := bh.Save(bh.config)
	if err != nil {
		t.Errorf("Unexpected error when saving artifacts: %v", err)
	}
	expectedArtifactDir := "/working-dir/upload/artifacts"
	if filepath.ToSlash(fs.MkdirDir) != expectedArtifactDir {
		t.Errorf("Mkdir was not called with the expected directory: %s",
			fs.MkdirDir)
	}
	if fd.RunContainerOpts.Image != bh.config.Tag {
		t.Errorf("Unexpected image sent to RunContainer: %s",
			fd.RunContainerOpts.Image)
	}
	if filepath.ToSlash(th.ExtractTarDir) != expectedArtifactDir || th.ExtractTarReader == nil {
		t.Errorf("ExtractTar was not called with the expected parameters.")
	}
}

func TestSaveArtifactsCustomTag(t *testing.T) {
	bh := testBuildHandler()
	bh.config.WorkingDir = "/working-dir"
	bh.config.IncrementalFromTag = "custom/tag"
	bh.config.Tag = "image/tag"
	fs := bh.fs.(*test.FakeFileSystem)
	fd := bh.docker.(*docker.FakeDocker)
	th := bh.tar.(*test.FakeTar)
	err := bh.Save(bh.config)
	if err != nil {
		t.Errorf("Unexpected error when saving artifacts: %v", err)
	}
	expectedArtifactDir := "/working-dir/upload/artifacts"
	if filepath.ToSlash(fs.MkdirDir) != expectedArtifactDir {
		t.Errorf("Mkdir was not called with the expected directory: %s",
			fs.MkdirDir)
	}
	if fd.RunContainerOpts.Image != bh.config.IncrementalFromTag {
		t.Errorf("Unexpected image sent to RunContainer: %s",
			fd.RunContainerOpts.Image)
	}
	if filepath.ToSlash(th.ExtractTarDir) != expectedArtifactDir || th.ExtractTarReader == nil {
		t.Errorf("ExtractTar was not called with the expected parameters.")
	}
}

func TestSaveArtifactsRunError(t *testing.T) {
	tests := []error{
		fmt.Errorf("Run error"),
		s2ierr.NewContainerError("", -1, ""),
	}
	expected := []error{
		tests[0],
		s2ierr.NewSaveArtifactsError("", "", tests[1]),
	}
	// test with tar extract error or not
	tarError := []bool{true, false}
	for i := range tests {
		for _, te := range tarError {
			bh := testBuildHandler()
			fd := bh.docker.(*docker.FakeDocker)
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
	fd := bh.docker.(*docker.FakeDocker)
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
		refSpecified     bool
		checkoutExpected bool
	}

	tests := []fetchTest{
		// 0
		{
			refSpecified:     false,
			checkoutExpected: false,
		},
		// 1
		{
			refSpecified:     true,
			checkoutExpected: true,
		},
	}

	for testNum, ft := range tests {
		bh := testBuildHandler()
		gh := bh.git.(*test.FakeGit)

		bh.config.WorkingDir = "/working-dir"
		gh.ValidCloneSpecResult = true
		if ft.refSpecified {
			bh.config.Ref = "a-branch"
		}
		bh.config.Source = "a-repo-source"

		expectedTargetDir := "/working-dir/upload/src"
		_, e := bh.source.Download(bh.config)
		if e != nil {
			t.Errorf("Unexpected error %v [%d]", e, testNum)
		}
		if gh.CloneSource != "a-repo-source" {
			t.Errorf("Clone was not called with the expected source. Got %s, expected %s [%d]", gh.CloneSource, "a-source-repo-source", testNum)
		}
		if filepath.ToSlash(gh.CloneTarget) != expectedTargetDir {
			t.Errorf("Unexpected target directory for clone operation. Got %s, expected %s [%d]", gh.CloneTarget, expectedTargetDir, testNum)
		}
		if ft.checkoutExpected {
			if gh.CheckoutRef != "a-branch" {
				t.Errorf("Checkout was not called with the expected branch. Got %s, expected %s [%d]", gh.CheckoutRef, "a-branch", testNum)
			}
			if filepath.ToSlash(gh.CheckoutRepo) != expectedTargetDir {
				t.Errorf("Unexpected target repository for checkout operation. Got %s, expected %s [%d]", gh.CheckoutRepo, expectedTargetDir, testNum)
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
		expected = append(expected, filepath.FromSlash("/working-dir/"+dir))
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

func TestPrepareUseCustomRuntimeArtifacts(t *testing.T) {
	expectedMapping := filepath.FromSlash("/src") + ":dst"

	builder := newFakeSTI(&FakeSTI{})

	config := builder.config
	config.RuntimeImage = "my-app"
	config.RuntimeArtifacts.Set(expectedMapping)

	if err := builder.Prepare(config); err != nil {
		t.Fatalf("Prepare() unexpectedly failed with error: %v", err)
	}

	if actualMapping := config.RuntimeArtifacts.String(); actualMapping != expectedMapping {
		t.Errorf("Prepare() shouldn't change mapping, but it was modified from %v to %v", expectedMapping, actualMapping)
	}
}

func TestPrepareFailForEmptyRuntimeArtifacts(t *testing.T) {
	builder := newFakeSTI(&FakeSTI{})

	docker := builder.docker.(*docker.FakeDocker)
	docker.AssembleInputFilesResult = ""

	config := builder.config
	config.RuntimeImage = "my-app"

	if len(config.RuntimeArtifacts) > 0 {
		t.Fatalf("RuntimeArtifacts must be empty by default")
	}

	err := builder.Prepare(config)
	if err == nil {
		t.Errorf("Prepare() should fail but it didn't")

	} else if expectedError := "no runtime artifacts to copy"; !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Prepare() should fail with error that contains text %q but failed with error: %q", expectedError, err)
	}
}

func TestPrepareRuntimeArtifactsValidation(t *testing.T) {
	testCases := []struct {
		mapping       string
		expectedError string
	}{
		{
			mapping:       "src:dst",
			expectedError: "source must be an absolute path",
		},
		{
			mapping:       "/src:/dst",
			expectedError: "destination must be a relative path",
		},
		{
			mapping:       "/src:../dst",
			expectedError: "destination cannot start with '..'",
		},
	}

	for _, testCase := range testCases {
		for _, mappingFromUser := range []bool{true, false} {
			builder := newFakeSTI(&FakeSTI{})

			config := builder.config
			config.RuntimeImage = "my-app"

			if mappingFromUser {
				config.RuntimeArtifacts.Set(testCase.mapping)
			} else {
				docker := builder.docker.(*docker.FakeDocker)
				docker.AssembleInputFilesResult = testCase.mapping
			}

			err := builder.Prepare(config)
			if err == nil {
				t.Errorf("Prepare() should fail but it didn't")

			} else if !strings.Contains(err.Error(), testCase.expectedError) {
				t.Errorf("Prepare() should fail to validate mapping %q with error that contains text %q but failed with error: %q", testCase.mapping, testCase.expectedError, err)
			}
		}
	}
}

func TestPrepareSetRuntimeArtifacts(t *testing.T) {
	for _, mapping := range []string{filepath.FromSlash("/src") + ":dst", filepath.FromSlash("/src1") + ":dst1;" + filepath.FromSlash("/src1") + ":dst1"} {
		expectedMapping := strings.Replace(mapping, ";", ",", -1)

		builder := newFakeSTI(&FakeSTI{})

		docker := builder.docker.(*docker.FakeDocker)
		docker.AssembleInputFilesResult = mapping

		config := builder.config
		config.RuntimeImage = "my-app"

		if len(config.RuntimeArtifacts) > 0 {
			t.Fatalf("RuntimeArtifacts must be empty by default")
		}

		if err := builder.Prepare(config); err != nil {
			t.Fatalf("Prepare() unexpectedly failed with error: %v", err)
		}

		if actualMapping := config.RuntimeArtifacts.String(); actualMapping != expectedMapping {
			t.Errorf("Prepare() shouldn't change mapping, but it was modified from %v to %v", expectedMapping, actualMapping)
		}
	}
}

func TestPrepareDownloadAssembleRuntime(t *testing.T) {
	installer := &test.FakeInstaller{}

	builder := newFakeSTI(&FakeSTI{})
	builder.runtimeInstaller = installer
	builder.optionalRuntimeScripts = []string{api.AssembleRuntime}

	config := builder.config
	config.RuntimeImage = "my-app"
	config.RuntimeArtifacts.Set("/src:dst")

	if err := builder.Prepare(config); err != nil {
		t.Fatalf("Prepare() unexpectedly failed with error: %v", err)
	}

	if len(installer.Scripts) != 1 || installer.Scripts[0][0] != api.AssembleRuntime {
		t.Errorf("Prepare() should download %q script but it downloaded %v", api.AssembleRuntime, installer.Scripts)
	}
}

func TestExecuteOK(t *testing.T) {
	rh := newFakeBaseSTI()
	pe := &FakeSTI{}
	rh.postExecutor = pe
	rh.config.WorkingDir = "/working-dir"
	rh.config.BuilderImage = "test/image"
	rh.config.BuilderPullPolicy = api.PullAlways
	rh.config.Environment = api.EnvironmentList{
		api.EnvironmentSpec{
			Name:  "Key1",
			Value: "Value1",
		},
		api.EnvironmentSpec{
			Name:  "Key2",
			Value: "Value2",
		},
	}
	expectedEnv := []string{"Key1=Value1", "Key2=Value2"}
	th := rh.tar.(*test.FakeTar)
	th.CreateTarResult = "/working-dir/test.tar"
	fd := rh.docker.(*docker.FakeDocker)
	fd.RunContainerContainerID = "1234"
	fd.RunContainerCmd = []string{"one", "two"}

	err := rh.Execute("test-command", "foo", rh.config)
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	th = rh.tar.(*test.FakeTar).Copy()
	if th.CreateTarBase != "" {
		t.Errorf("Unexpected tar base directory: %s", th.CreateTarBase)
	}
	if filepath.ToSlash(th.CreateTarDir) != "/working-dir/upload" {
		t.Errorf("Unexpected tar directory: %s", th.CreateTarDir)
	}
	fh, ok := rh.fs.(*test.FakeFileSystem)
	if !ok {
		t.Fatalf("Unable to convert %v to FakeFilesystem", rh.fs)
	}
	if fh.OpenFile != "" {
		t.Fatalf("Unexpected file opened: %s", fh.OpenFile)
	}
	if fh.OpenFileResult != nil {
		t.Errorf("Tar file was opened.")
	}
	ro := fd.RunContainerOpts

	if ro.User != "foo" {
		t.Errorf("Expected user to be foo, got %q", ro.User)
	}

	if ro.Image != rh.config.BuilderImage {
		t.Errorf("Unexpected Image passed to RunContainer")
	}
	if _, ok := ro.Stdin.(*io.PipeReader); !ok {
		t.Errorf("Unexpected input stream: %#v", ro.Stdin)
	}
	if ro.PullImage {
		t.Errorf("PullImage is true for RunContainer, should be false")
	}
	if ro.Command != "test-command" {
		t.Errorf("Unexpected command passed to RunContainer: %s",
			ro.Command)
	}
	if pe.PostExecuteContainerID != "1234" {
		t.Errorf("PostExecutor not called with expected ID: %s",
			pe.PostExecuteContainerID)
	}
	if !reflect.DeepEqual(ro.Env, expectedEnv) {
		t.Errorf("Unexpected container environment passed to RunContainer: %v, should be %v", ro.Env, expectedEnv)
	}
	if !reflect.DeepEqual(pe.PostExecuteDestination, "test-command") {
		t.Errorf("PostExecutor not called with expected command: %s", pe.PostExecuteDestination)
	}
}

func TestExecuteRunContainerError(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	fd := rh.docker.(*docker.FakeDocker)
	runContainerError := fmt.Errorf("an error")
	fd.RunContainerError = runContainerError
	err := rh.Execute("test-command", "", rh.config)
	if err != runContainerError {
		t.Errorf("Did not get expected error, got %v", err)
	}
}

func TestExecuteErrorCreateTarFile(t *testing.T) {
	rh := newFakeSTI(&FakeSTI{})
	rh.tar.(*test.FakeTar).CreateTarError = errors.New("CreateTarError")
	err := rh.Execute("test-command", "", rh.config)
	if err == nil || err.Error() != "CreateTarError" {
		t.Errorf("An error was expected for CreateTarFile, but got different: %#v", err)
	}
}

func TestCleanup(t *testing.T) {
	rh := newFakeBaseSTI()

	rh.config.WorkingDir = "/working-dir"
	preserve := []bool{false, true}
	for _, p := range preserve {
		rh.config.PreserveWorkingDir = p
		rh.fs = &test.FakeFileSystem{}
		rh.garbage = build.NewDefaultCleaner(rh.fs, rh.docker)
		rh.garbage.Cleanup(rh.config)
		removedDir := rh.fs.(*test.FakeFileSystem).RemoveDirName
		if p && removedDir != "" {
			t.Errorf("Expected working directory to be preserved, but it was removed.")
		} else if !p && removedDir == "" {
			t.Errorf("Expected working directory to be removed, but it was preserved.")
		}
	}
}

func TestNewWithInvalidExcludeRegExp(t *testing.T) {
	_, err := New(nil, &api.Config{
		DockerConfig:  docker.GetDefaultDockerConfig(),
		ExcludeRegExp: "(",
	}, nil, build.Overrides{})
	if syntaxErr, ok := err.(*syntax.Error); ok && syntaxErr.Code != syntax.ErrMissingParen {
		t.Errorf("expected regexp compilation error, got %v", err)
	}
}
