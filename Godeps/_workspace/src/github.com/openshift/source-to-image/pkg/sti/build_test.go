package sti

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/sti/errors"
	"github.com/openshift/source-to-image/pkg/sti/test"
)

type FakeBuildHandler struct {
	CleanupCalled              bool
	SetupRequired              []string
	SetupOptional              []string
	SetupError                 error
	DetermineIncrementalCalled bool
	DetermineIncrementalError  error
	BuildRequest               *STIRequest
	BuildResult                *STIResult
	SaveArtifactsCalled        bool
	SaveArtifactsError         error
	FetchSourceCalled          bool
	FetchSourceError           error
	ExecuteCommand             string
	ExecuteError               error
}

func (f *FakeBuildHandler) cleanup() {
	f.CleanupCalled = true
}

func (f *FakeBuildHandler) setup(required []string, optional []string) error {
	f.SetupRequired = required
	f.SetupOptional = optional
	return f.SetupError
}

func (f *FakeBuildHandler) determineIncremental() error {
	f.DetermineIncrementalCalled = true
	return f.DetermineIncrementalError
}

func (f *FakeBuildHandler) Request() *STIRequest {
	return f.BuildRequest
}

func (f *FakeBuildHandler) Result() *STIResult {
	return f.BuildResult
}

func (f *FakeBuildHandler) saveArtifacts() error {
	f.SaveArtifactsCalled = true
	return f.SaveArtifactsError
}

func (f *FakeBuildHandler) fetchSource() error {
	return f.FetchSourceError
}

func (f *FakeBuildHandler) execute(command string) error {
	f.ExecuteCommand = command
	return f.ExecuteError
}

func TestBuild(t *testing.T) {
	incrementalTest := []bool{false, true}
	for _, incremental := range incrementalTest {

		fh := &FakeBuildHandler{
			BuildRequest: &STIRequest{incremental: incremental},
			BuildResult:  &STIResult{},
		}
		builder := Builder{
			handler: fh,
		}
		builder.Build()

		// Verify the right scripts were requested
		if !reflect.DeepEqual(fh.SetupRequired, []string{"assemble", "run"}) {
			t.Errorf("Unexpected required scripts requested: %#v", fh.SetupRequired)
		}
		if !reflect.DeepEqual(fh.SetupOptional, []string{"save-artifacts"}) {
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
		if fh.ExecuteCommand != "assemble" {
			t.Errorf("Unexpected execute command: %s", fh.ExecuteCommand)
		}
	}
}

func testBuildHandler() *buildHandler {
	requestHandler := &requestHandler{
		docker:    &test.FakeDocker{},
		installer: &test.FakeInstaller{},
		fs:        &test.FakeFileSystem{},
		tar:       &test.FakeTar{},

		request: &STIRequest{},
		result:  &STIResult{},
	}
	buildHandler := &buildHandler{
		requestHandler:  requestHandler,
		git:             &test.FakeGit{},
		callbackInvoker: &test.FakeCallbackInvoker{},
	}

	return buildHandler
}

func TestPostExecute(t *testing.T) {
	incrementalTest := []bool{true, false}
	for _, incremental := range incrementalTest {
		previousImageIdTest := []string{"", "test-image"}
		for _, previousImageId := range previousImageIdTest {
			bh := testBuildHandler()
			bh.result.Messages = []string{"one", "two"}
			bh.request.CallbackUrl = "https://my.callback.org/test"
			bh.request.Tag = "test/tag"
			dh := bh.docker.(*test.FakeDocker)
			bh.request.incremental = incremental
			if previousImageId != "" {
				bh.request.RemovePreviousImage = true
				bh.docker.(*test.FakeDocker).GetImageIdResult = previousImageId
			}
			err := bh.PostExecute("test-container-id", []string{"cmd1", "arg1"})
			if err != nil {
				t.Errorf("Unexpected errror from postExecute: %v", err)
			}
			// Ensure CommitContainer was called with the right parameters
			if !reflect.DeepEqual(dh.CommitContainerOpts.Command,
				[]string{"cmd1", "arg1", "run"}) {
				t.Errorf("Unexpected commit container command: %#v",
					dh.CommitContainerOpts.Command)
			}
			if dh.CommitContainerOpts.Repository != bh.request.Tag {
				t.Errorf("Unexpected tag commited: %s",
					dh.CommitContainerOpts.Repository)
			}

			if incremental && previousImageId != "" {
				if dh.RemoveImageName != "test-image" {
					t.Errorf("Previous image was not removed: %s",
						dh.RemoveImageName)
				}
			} else {
				if dh.RemoveImageName != "" {
					t.Errorf("Unexpected image removed: %s",
						dh.RemoveImageName)
				}
			}

		}
	}
}

func TestDetermineIncremental(t *testing.T) {
	type incrementalTest struct {
		clean        bool
		inLocal      bool
		scriptExists bool
		expected     bool
	}

	tests := []incrementalTest{
		{
			clean:        false,
			inLocal:      true,
			scriptExists: true,
			expected:     true,
		},
		{
			clean:        true,
			inLocal:      true,
			scriptExists: true,
			expected:     false,
		},
		{
			clean:        false,
			inLocal:      false,
			scriptExists: true,
			expected:     false,
		},
		{
			clean:        false,
			inLocal:      true,
			scriptExists: false,
			expected:     false,
		},
	}

	for _, ti := range tests {
		bh := testBuildHandler()
		bh.request.workingDir = "/working-dir"
		bh.request.Clean = ti.clean
		bh.docker.(*test.FakeDocker).LocalRegistryResult = ti.inLocal
		if ti.scriptExists {
			bh.fs.(*test.FakeFileSystem).ExistsResult = map[string]bool{
				"/working-dir/upload/scripts/save-artifacts": true,
			}
		}
		bh.determineIncremental()
		if bh.request.incremental != ti.expected {
			t.Errorf("Unexpected incremental result: %v. Expected: %v",
				bh.request.incremental, ti.expected)
		}
		if !ti.clean && ti.inLocal {
			scriptChecked := bh.fs.(*test.FakeFileSystem).ExistsFile[0]
			expectedScript := "/working-dir/upload/scripts/save-artifacts"
			if scriptChecked != expectedScript {
				t.Errorf("Unexpected script checked. Actual: %s. Expected: %s",
					scriptChecked, expectedScript)
			}
		}
	}
}

func TestSaveArtifacts(t *testing.T) {
	bh := testBuildHandler()
	bh.request.workingDir = "/working-dir"
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
		errors.StiContainerError{-1},
	}
	expected := []error{
		tests[0],
		errors.ErrSaveArtifactsFailed,
	}
	// test with tar extract error or not
	tarError := []bool{true, false}
	for i, _ := range tests {
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

		bh.request.workingDir = "/working-dir"
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
