package sti

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/sti/api"
	stierr "github.com/openshift/source-to-image/pkg/sti/errors"
	"github.com/openshift/source-to-image/pkg/sti/test"
)

type FakeBuildHandler struct {
	CleanupCalled              bool
	SetupRequired              []api.Script
	SetupOptional              []api.Script
	SetupError                 error
	DetermineIncrementalCalled bool
	DetermineIncrementalError  error
	BuildRequest               *api.Request
	BuildResult                *api.Result
	SaveArtifactsCalled        bool
	SaveArtifactsError         error
	FetchSourceCalled          bool
	FetchSourceError           error
	ExecuteCommand             api.Script
	ExecuteError               error
	ExpectedError              bool
	LayeredBuildCalled         bool
	LayeredBuildError          error
}

func (f *FakeBuildHandler) cleanup() {
	f.CleanupCalled = true
}

func (f *FakeBuildHandler) setup(required []api.Script, optional []api.Script) error {
	f.SetupRequired = required
	f.SetupOptional = optional
	return f.SetupError
}

func (f *FakeBuildHandler) determineIncremental() error {
	f.DetermineIncrementalCalled = true
	return f.DetermineIncrementalError
}

func (f *FakeBuildHandler) Request() *api.Request {
	return f.BuildRequest
}

func (f *FakeBuildHandler) Result() *api.Result {
	return f.BuildResult
}

func (f *FakeBuildHandler) saveArtifacts() error {
	f.SaveArtifactsCalled = true
	return f.SaveArtifactsError
}

func (f *FakeBuildHandler) fetchSource() error {
	return f.FetchSourceError
}

func (f *FakeBuildHandler) execute(command api.Script) error {
	f.ExecuteCommand = command
	return f.ExecuteError
}

func (f *FakeBuildHandler) wasExpectedError(text string) bool {
	return f.ExpectedError
}

func (f *FakeBuildHandler) build() error {
	f.LayeredBuildCalled = true
	return f.LayeredBuildError
}

func TestBuild(t *testing.T) {
	incrementalTest := []bool{false, true}
	for _, incremental := range incrementalTest {

		fh := &FakeBuildHandler{
			BuildRequest: &api.Request{Incremental: incremental},
			BuildResult:  &api.Result{},
		}
		builder := Builder{
			handler: fh,
		}
		builder.Build()

		// Verify the right scripts were requested
		if !reflect.DeepEqual(fh.SetupRequired, []api.Script{api.Assemble, api.Run}) {
			t.Errorf("Unexpected required scripts requested: %#v", fh.SetupRequired)
		}
		if !reflect.DeepEqual(fh.SetupOptional, []api.Script{api.SaveArtifacts}) {
			t.Errorf("Unexpected optional scripts requested: %#v", fh.SetupOptional)
		}

		// Verify that determineIncremental was called
		if !fh.DetermineIncrementalCalled {
			t.Errorf("Determine incremental was not called.")
		}

		// Verify that saveartifacts was called for an incremental build
		if incremental && !fh.SaveArtifactsCalled {
			t.Errorf("SaveArtifacts was not called for an incremental build")
		}

		// Verify that execute was called with the right script
		if fh.ExecuteCommand != api.Assemble {
			t.Errorf("Unexpected execute command: %s", fh.ExecuteCommand)
		}
	}
}

func TestLayeredBuild(t *testing.T) {
	fh := &FakeBuildHandler{
		BuildRequest: &api.Request{
			BaseImage: "testimage",
		},
		BuildResult:   &api.Result{},
		ExecuteError:  stierr.NewContainerError("", 1, `/bin/sh: tar: not found`),
		ExpectedError: true,
	}
	builder := Builder{
		handler: fh,
	}
	builder.Build()
	// Verify layered build
	if !fh.LayeredBuildCalled {
		t.Errorf("Layered build was not called.")
	}
}

func TestBuildErrorExecute(t *testing.T) {
	fh := &FakeBuildHandler{
		BuildRequest: &api.Request{
			BaseImage: "testimage",
		},
		BuildResult:   &api.Result{},
		ExecuteError:  errors.New("ExecuteError"),
		ExpectedError: false,
	}
	builder := Builder{
		handler: fh,
	}
	_, err := builder.Build()
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
		bh := &buildHandler{}
		result := bh.wasExpectedError(ti.text)
		if result != ti.expected {
			t.Errorf("(%d) Unexpected result: %v. Expected: %v", i, result, ti.expected)
		}
	}
}

func testBuildHandler() *buildHandler {
	requestHandler := &requestHandler{
		docker:    &test.FakeDocker{},
		installer: &test.FakeInstaller{},
		git:       &test.FakeGit{},
		fs:        &test.FakeFileSystem{},
		tar:       &test.FakeTar{},

		request: &api.Request{},
		result:  &api.Result{},
	}
	buildHandler := &buildHandler{
		requestHandler:  requestHandler,
		callbackInvoker: &test.FakeCallbackInvoker{},
	}

	return buildHandler
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

func TestDetermineIncremental(t *testing.T) {
	type incrementalTest struct {
		// clean flag was passed
		clean bool
		// previous image existence
		previousImage bool
		// script download happened -> external scripts
		scriptDownload bool
		// script exists
		scriptExists bool
		// expected result
		expected bool
	}

	tests := []incrementalTest{
		// 0: external, downloaded scripts and previously image available
		{false, true, true, true, true},

		// 1: previous image, script downloaded but no save-artifacts
		{false, true, true, false, false},

		// 2-9: clean build - should always return false no matter what other flags are
		{true, false, false, false, false},
		{true, false, false, true, false},
		{true, false, true, false, false},
		{true, false, true, true, false},
		{true, true, false, false, false},
		{true, true, false, true, false},
		{true, true, true, false, false},
		{true, true, true, true, false},

		// 10-17: no previous image - should always return false not matter what other flags are
		{false, false, false, false, false},
		{false, false, false, true, false},
		{false, false, true, false, false},
		{false, false, true, true, false},
		{true, false, false, false, false},
		{true, false, false, true, false},
		{true, false, true, false, false},
		{true, false, true, true, false},

		// 18-19: previous image, script inside the image, its existence does not matter
		{false, true, false, true, true},
		{false, true, false, false, true},
	}

	for i, ti := range tests {
		bh := testBuildHandler()
		bh.request.WorkingDir = "/working-dir"
		bh.request.Clean = ti.clean
		bh.request.ExternalOptionalScripts = ti.scriptDownload
		bh.docker.(*test.FakeDocker).LocalRegistryResult = ti.previousImage
		if ti.scriptExists {
			bh.fs.(*test.FakeFileSystem).ExistsResult = map[string]bool{
				"/working-dir/upload/scripts/save-artifacts": true,
			}
		}
		bh.determineIncremental()
		if bh.request.Incremental != ti.expected {
			t.Errorf("(%d) Unexpected incremental result: %v. Expected: %v",
				i, bh.request.Incremental, ti.expected)
		}
		if !ti.clean && ti.previousImage && ti.scriptDownload {
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
	err := bh.saveArtifacts()
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
		stierr.NewSaveArtifactsError("", tests[1]),
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
			err := bh.saveArtifacts()
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
	err := bh.saveArtifacts()
	if err != expected {
		t.Errorf("Unexpected error returned from saveArtifacts: %v", err)
	}
}

func TestSaveArtifactsExtractError(t *testing.T) {
	bh := testBuildHandler()
	th := bh.tar.(*test.FakeTar)
	expected := fmt.Errorf("extract error")
	th.ExtractTarError = expected
	err := bh.saveArtifacts()
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
		bh.fetchSource()
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
