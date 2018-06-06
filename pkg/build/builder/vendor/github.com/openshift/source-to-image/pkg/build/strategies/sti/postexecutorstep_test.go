package sti

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/docker"
)

func TestStorePreviousImageStep(t *testing.T) {
	testCases := []struct {
		imageIDError             error
		expectedPreviousImageID  string
		expectedPreviousImageTag string
	}{
		{
			imageIDError:             nil,
			expectedPreviousImageID:  "12345",
			expectedPreviousImageTag: "0.1",
		},
		{
			imageIDError:             fmt.Errorf("fail"),
			expectedPreviousImageID:  "",
			expectedPreviousImageTag: "0.1",
		},
	}

	for _, testCase := range testCases {

		builder := newFakeBaseSTI()
		builder.incremental = true
		builder.config.RemovePreviousImage = true
		builder.config.Tag = testCase.expectedPreviousImageTag

		fakeDocker := builder.docker.(*docker.FakeDocker)
		fakeDocker.GetImageIDResult = testCase.expectedPreviousImageID
		fakeDocker.GetImageIDError = testCase.imageIDError

		step := &storePreviousImageStep{builder: builder, docker: fakeDocker}

		ctx := &postExecutorStepContext{}

		if err := step.execute(ctx); err != nil {
			t.Fatalf("should exit without error, but it returned %v", err)
		}

		if fakeDocker.GetImageIDImage != testCase.expectedPreviousImageTag {
			t.Errorf("should invoke fakeDocker.GetImageID(%q) but invoked with %q", testCase.expectedPreviousImageTag, fakeDocker.GetImageIDImage)
		}

		if ctx.previousImageID != testCase.expectedPreviousImageID {
			t.Errorf("should set previousImageID field to %q but it's %q", testCase.expectedPreviousImageID, ctx.previousImageID)
		}
	}
}

func TestRemovePreviousImageStep(t *testing.T) {
	testCases := []struct {
		removeImageError        error
		expectedPreviousImageID string
	}{
		{
			removeImageError:        nil,
			expectedPreviousImageID: "",
		},
		{
			removeImageError:        nil,
			expectedPreviousImageID: "12345",
		},
		{
			removeImageError:        fmt.Errorf("fail"),
			expectedPreviousImageID: "12345",
		},
	}

	for _, testCase := range testCases {

		builder := newFakeBaseSTI()
		builder.incremental = true
		builder.config.RemovePreviousImage = true

		fakeDocker := builder.docker.(*docker.FakeDocker)
		fakeDocker.RemoveImageError = testCase.removeImageError

		step := &removePreviousImageStep{builder: builder, docker: fakeDocker}

		ctx := &postExecutorStepContext{previousImageID: testCase.expectedPreviousImageID}

		if err := step.execute(ctx); err != nil {
			t.Fatalf("should exit without error, but it returned %v", err)
		}

		if fakeDocker.RemoveImageName != testCase.expectedPreviousImageID {
			t.Errorf("should invoke fakeDocker.RemoveImage(%q) but invoked with %q", testCase.expectedPreviousImageID, fakeDocker.RemoveImageName)
		}
	}
}

func TestCommitImageStep(t *testing.T) {

	testCases := []struct {
		embeddedScript   bool
		destination      string
		expectedImageCmd string
	}{
		{
			embeddedScript:   false,
			destination:      "/path/to/location",
			expectedImageCmd: "/path/to/location/scripts/run",
		},
		{
			embeddedScript:   true,
			destination:      "image:///usr/bin/run.sh",
			expectedImageCmd: "/usr/bin/run.sh",
		},
	}

	for _, testCase := range testCases {

		expectedEnv := []string{"BUILD_LOGLEVEL"}
		expectedContainerID := "container-yyyy"
		expectedImageID := "image-xxx"
		expectedImageTag := "v1"
		expectedImageUser := "jboss"
		expectedEntrypoint := []string{"test_entrypoint"}

		displayName := "MyApp"
		description := "My Application is awesome!"

		baseImageLabels := make(map[string]string)
		baseImageLabels["vendor"] = "CentOS"

		configLabels := make(map[string]string)
		configLabels["distribution-scope"] = "private"

		expectedLabels := make(map[string]string)
		expectedLabels["io.k8s.description"] = description
		expectedLabels["io.k8s.display-name"] = displayName
		expectedLabels["vendor"] = "CentOS"
		expectedLabels["distribution-scope"] = "private"

		builder := newFakeBaseSTI()
		builder.config.DisplayName = displayName
		builder.config.Description = description
		builder.config.Tag = expectedImageTag
		builder.config.Labels = configLabels
		builder.env = expectedEnv

		fakeDocker := builder.docker.(*docker.FakeDocker)
		fakeDocker.CommitContainerResult = expectedImageID
		fakeDocker.GetImageUserResult = expectedImageUser
		fakeDocker.GetImageEntrypointResult = expectedEntrypoint
		fakeDocker.Labels = baseImageLabels

		ctx := &postExecutorStepContext{
			containerID: expectedContainerID,
		}

		if testCase.embeddedScript {
			builder.scriptsURL = make(map[string]string)
			builder.scriptsURL["run"] = testCase.destination
		} else {
			ctx.destination = testCase.destination
		}

		step := &commitImageStep{builder: builder, docker: fakeDocker}

		if err := step.execute(ctx); err != nil {
			t.Fatalf("should exit without error, but it returned %v", err)
		}

		if ctx.imageID != expectedImageID {
			t.Errorf("should set ImageID field to %q but it's %q", expectedImageID, ctx.imageID)
		}

		commitOpts := fakeDocker.CommitContainerOpts

		if len(commitOpts.Command) != 1 {
			t.Errorf("should commit container with Command: %q, but committed with %q", testCase.expectedImageCmd, commitOpts.Command)

		} else if commitOpts.Command[0] != testCase.expectedImageCmd {
			t.Errorf("should commit container with Command: %q, but committed with %q", testCase.expectedImageCmd, commitOpts.Command[0])
		}

		if !reflect.DeepEqual(commitOpts.Env, expectedEnv) {
			t.Errorf("should commit container with Env: %v, but committed with %v", expectedEnv, commitOpts.Env)
		}

		if commitOpts.ContainerID != expectedContainerID {
			t.Errorf("should commit container with ContainerID: %q, but committed with %q", expectedContainerID, commitOpts.ContainerID)
		}

		if commitOpts.Repository != expectedImageTag {
			t.Errorf("should commit container with Repository: %q, but committed with %q", expectedImageTag, commitOpts.Repository)
		}

		if commitOpts.User != expectedImageUser {
			t.Errorf("should commit container with User: %q, but committed with %q", expectedImageUser, commitOpts.User)
		}

		if !reflect.DeepEqual(commitOpts.Entrypoint, expectedEntrypoint) {
			t.Errorf("should commit container with Entrypoint: %q, but committed with %q", expectedEntrypoint, commitOpts.Entrypoint)
		}

		if !reflect.DeepEqual(commitOpts.Labels, expectedLabels) {
			t.Errorf("should commit container with Labels: %v, but committed with %v", expectedLabels, commitOpts.Labels)
		}
	}
}

func TestDownloadFilesFromBuilderImageStep(t *testing.T) {
	// FIXME
}

func TestStartRuntimeImageAndUploadFilesStep(t *testing.T) {
	// FIXME
}

func TestReportSuccessStep(t *testing.T) {
	builder := newFakeBaseSTI()
	step := &reportSuccessStep{builder: builder}
	ctx := &postExecutorStepContext{imageID: "my-app"}

	if err := step.execute(ctx); err != nil {
		t.Fatalf("should exit without error, but it returned %v", err)
	}

	if builder.result.Success != true {
		t.Errorf("should set Success field to 'true' but it's %v", builder.result.Success)
	}

	if builder.result.ImageID != ctx.imageID {
		t.Errorf("should set ImageID field to %q but it's %q", ctx.imageID, builder.result.ImageID)
	}
}
