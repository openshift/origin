package docker

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/openshift/source-to-image/pkg/sti/docker/test"
	"github.com/openshift/source-to-image/pkg/sti/errors"
)

func getDocker(client DockerClient) *stiDocker {
	return &stiDocker{
		client: client,
	}
}

func TestIsImageInLocalRegistry(t *testing.T) {
	type testDef struct {
		docker         test.FakeDockerClient
		expectedResult bool
		expectedError  error
	}
	otherError := fmt.Errorf("Other")
	tests := map[string]testDef{
		"ImageFound":    {test.FakeDockerClient{Image: &docker.Image{}}, true, nil},
		"ImageNotFound": {test.FakeDockerClient{InspectImageErr: []error{docker.ErrNoSuchImage}}, false, nil},
		"ErrorOccurred": {test.FakeDockerClient{InspectImageErr: []error{otherError}}, false, otherError},
	}

	imageName := "a_test_image"

	for test, def := range tests {
		dh := getDocker(&def.docker)
		result, err := dh.IsImageInLocalRegistry(imageName)
		if result != def.expectedResult {
			t.Errorf("Test - %s: Expected result: %v. Got: %v", test, def.expectedResult, result)
		}
		if err != def.expectedError {
			t.Errorf("Test - %s: Expected error: %v. Got: %v", test, def.expectedError, err)
		}
		if def.docker.InspectImageName[0] != imageName {
			t.Errorf("Docker inspect called with unexpected image name: %s\n",
				def.docker.InspectImageName)
		}
	}
}

func TestCheckAndPull(t *testing.T) {
	type testDef struct {
		docker              test.FakeDockerClient
		expectedImage       *docker.Image
		expectedError       error
		expectedPullOptions docker.PullImageOptions
	}
	image := &docker.Image{}
	imageName := "test_image"
	imageExistsTest := testDef{
		docker: test.FakeDockerClient{
			InspectImageErr:    []error{nil},
			InspectImageResult: []*docker.Image{image},
		},
		expectedImage: image,
	}
	imagePulledTest := testDef{
		docker: test.FakeDockerClient{
			InspectImageErr:    []error{docker.ErrNoSuchImage, nil},
			InspectImageResult: []*docker.Image{nil, image},
		},
		expectedImage:       image,
		expectedPullOptions: docker.PullImageOptions{Repository: imageName},
	}
	inspectErrorTest := testDef{
		docker: test.FakeDockerClient{
			InspectImageErr:    []error{docker.ErrConnectionRefused},
			InspectImageResult: []*docker.Image{nil},
		},
		expectedImage: nil,
		expectedError: errors.ErrPullImageFailed,
	}
	pullErrorTest := testDef{
		docker: test.FakeDockerClient{
			PullImageErr:       docker.ErrConnectionRefused,
			InspectImageErr:    []error{nil},
			InspectImageResult: []*docker.Image{nil},
		},
		expectedImage:       nil,
		expectedError:       errors.ErrPullImageFailed,
		expectedPullOptions: docker.PullImageOptions{Repository: imageName},
	}
	errorAfterPullTest := testDef{
		docker: test.FakeDockerClient{
			InspectImageErr:    []error{docker.ErrNoSuchImage, docker.ErrNoSuchImage},
			InspectImageResult: []*docker.Image{nil, image},
		},
		expectedImage:       nil,
		expectedError:       docker.ErrNoSuchImage,
		expectedPullOptions: docker.PullImageOptions{Repository: imageName},
	}
	tests := map[string]testDef{
		"ImageExists":    imageExistsTest,
		"ImagePulled":    imagePulledTest,
		"InspectError":   inspectErrorTest,
		"PullError":      pullErrorTest,
		"ErrorAfterPull": errorAfterPullTest,
	}

	for test, def := range tests {
		dh := getDocker(&def.docker)
		resultImage, resultErr := dh.CheckAndPull(imageName)
		if resultImage != def.expectedImage {
			t.Errorf("%s: Unexpected image result -- %v", test, resultImage)
		}
		if resultErr != def.expectedError {
			t.Errorf("%s: Unexpected error result -- %v", test, resultErr)
		}
		pullOpts := def.docker.PullImageOpts
		if !reflect.DeepEqual(pullOpts, def.expectedPullOptions) {
			t.Errorf("%s: Unexpected pull image opts -- %v", test, pullOpts)
		}
	}
}

func TestRemoveContainer(t *testing.T) {
	fakeDocker := &test.FakeDockerClient{}
	dh := getDocker(fakeDocker)
	containerId := "testContainerId"
	expectedOpts := docker.RemoveContainerOptions{
		ID:            containerId,
		RemoveVolumes: true,
		Force:         true,
	}
	dh.RemoveContainer(containerId)
	if !reflect.DeepEqual(expectedOpts, fakeDocker.RemoveContainerOpts) {
		t.Errorf("Unexpected removeContainerOpts. Expected: %#v, Got: %#v",
			expectedOpts, fakeDocker.RemoveContainerOpts)
	}
}

func TestCommitContainer(t *testing.T) {
	expectedImageID := "test-1234"
	fakeDocker := &test.FakeDockerClient{Image: &docker.Image{ID: expectedImageID}}
	dh := getDocker(fakeDocker)
	containerId := "test-container-id"
	containerTag := "test-container-tag"

	opt := CommitContainerOptions{
		ContainerID: containerId,
		Repository:  containerTag,
	}

	imageID, err := dh.CommitContainer(opt)
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	if imageID != expectedImageID {
		t.Errorf("Did not return the correct image id: %s", imageID)
	}
}

func TestCommitContainerError(t *testing.T) {
	expectedErr := fmt.Errorf("Test error")
	fakeDocker := &test.FakeDockerClient{CommitContainerErr: expectedErr}
	dh := getDocker(fakeDocker)
	containerId := "test-container-id"
	containerTag := "test-container-tag"

	opt := CommitContainerOptions{
		ContainerID: containerId,
		Repository:  containerTag,
	}

	_, err := dh.CommitContainer(opt)

	expectedOpts := docker.CommitContainerOptions{
		Container:  containerId,
		Repository: containerTag,
	}
	if !reflect.DeepEqual(expectedOpts, fakeDocker.CommitContainerOpts) {
		t.Errorf("Commit container called with unexpected parameters: %#v", fakeDocker.CommitContainerOpts)
	}
	if err != expectedErr {
		t.Errorf("Unexpected error returned: %v", err)
	}
}

func TestGetDefaultUrl(t *testing.T) {
	type urltest struct {
		image       docker.Image
		result      string
		inspectErr  error
		errExpected bool
	}
	tests := map[string]urltest{
		"not present": urltest{
			image: docker.Image{
				ContainerConfig: docker.Config{
					Env: []string{"Env1=value1"},
				},
				Config: &docker.Config{
					Env: []string{"Env2=value2"},
				},
			},
			result: "",
		},

		"in containerConfig": urltest{
			image: docker.Image{
				ContainerConfig: docker.Config{
					Env: []string{"Env1=value1", "STI_SCRIPTS_URL=test_url_value"},
				},
				Config: &docker.Config{},
			},
			result: "test_url_value",
		},

		"in image config": urltest{
			image: docker.Image{
				ContainerConfig: docker.Config{},
				Config: &docker.Config{
					Env: []string{
						"Env1=value1",
						"STI_SCRIPTS_URL=test_url_value_2",
						"Env2=value2",
					},
				},
			},
			result: "test_url_value_2",
		},

		"contains =": urltest{
			image: docker.Image{
				ContainerConfig: docker.Config{
					Env: []string{"STI_SCRIPTS_URL=http://my.test.url/test?param=one"},
				},
				Config: &docker.Config{},
			},
			result: "http://my.test.url/test?param=one",
		},

		"inspect error": urltest{
			image:       docker.Image{},
			inspectErr:  fmt.Errorf("Inspect error"),
			errExpected: true,
		},
	}
	for desc, tst := range tests {
		fakeDocker := &test.FakeDockerClient{
			InspectImageResult: []*docker.Image{&tst.image},
		}
		if tst.inspectErr != nil {
			fakeDocker.InspectImageErr = []error{tst.inspectErr}
		}
		dh := getDocker(fakeDocker)
		url, err := dh.GetDefaultScriptsUrl("test/image")
		if err != nil && !tst.errExpected {
			t.Errorf("%s: Unexpected error returned: %v", desc, err)
		} else if err == nil && tst.errExpected {
			t.Errorf("%s: Expected error. Did not get one.", desc)
		}
		if !tst.errExpected && url != tst.result {
			t.Errorf("%s: Unexpected result. Expected: %s Actual: %s",
				desc, tst.result, url)
		}
	}
}

func TestRunContainer(t *testing.T) {
	fakeDocker := &test.FakeDockerClient{
		InspectImageResult: []*docker.Image{
			&docker.Image{
				Config: &docker.Config{
					Cmd: []string{"test", "command"},
				},
			},
		},
		Container: &docker.Container{
			ID: "12345-test",
		},
		AttachToContainerSleep: 200 * time.Millisecond,
	}
	dh := getDocker(fakeDocker)
	err := dh.RunContainer(RunContainerOptions{
		Image:     "test/image",
		PullImage: true,
		Command:   "assemble",
		Env:       []string{"Key1=Value1", "Key2=Value2"},
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stdout,
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// Validate the CreateContainer parameters
	createConfig := fakeDocker.CreateContainerOpts.Config
	if createConfig.Image != "test/image" {
		t.Errorf("Unexpected create config image: %s", createConfig.Image)
	}
	if !reflect.DeepEqual(createConfig.Cmd, []string{"test", "command", "assemble"}) {
		t.Errorf("Unexpected create config command: %#v", createConfig.Cmd)
	}
	if !reflect.DeepEqual(createConfig.Env, []string{"Key1=Value1", "Key2=Value2"}) {
		t.Errorf("Unexpected create config env: %#v", createConfig.Env)
	}
	if !createConfig.OpenStdin || !createConfig.StdinOnce {
		t.Errorf("Unexpected stdin flags for createConfig: OpenStdin - %v"+
			" StdinOnce - %v", createConfig.OpenStdin, createConfig.StdinOnce)
	}

	// Verify that remove container was called
	if fakeDocker.RemoveContainerOpts.ID != "12345-test" {
		t.Errorf("RemoveContainer was not called with the expected container ID")
	}

	// Verify that AttachToContainer was called twice (Stdin/Stdout)
	if len(fakeDocker.AttachToContainerOpts) != 2 {
		t.Errorf("AttachToContainer was not called the expected number of times.")
	}
	// Make sure AttachToContainer was not called with both Stdin & Stdout
	for _, opt := range fakeDocker.AttachToContainerOpts {
		if opt.InputStream != nil && (opt.OutputStream != nil || opt.ErrorStream != nil) {
			t.Errorf("AttachToContainer was called with both Stdin and Stdout: %#v", opt)
		}
		if opt.Stdin && (opt.Stdout || opt.Stderr) {
			t.Errorf("AttachToContainer was called with both Stdin and Stdout flags: %#v", opt)
		}
	}
}

func TestGetImageID(t *testing.T) {
	fakeDocker := &test.FakeDockerClient{
		InspectImageResult: []*docker.Image{&docker.Image{ID: "test-abcd"}},
	}
	dh := getDocker(fakeDocker)
	id, err := dh.GetImageId("test/image")
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	} else if id != "test-abcd" {
		t.Errorf("Unexpected image id returned: %s", id)
	}
}

func TestGetImageIDError(t *testing.T) {
	expected := fmt.Errorf("Image Error")
	fakeDocker := &test.FakeDockerClient{
		InspectImageErr: []error{expected},
	}
	dh := getDocker(fakeDocker)
	id, err := dh.GetImageId("test/image")
	if err != expected {
		t.Errorf("Unexpected error returned: %v", err)
	}
	if id != "" {
		t.Errorf("Unexpected image id returned: %s", id)
	}
}

func TestRemoveImage(t *testing.T) {
	fakeDocker := &test.FakeDockerClient{}
	dh := getDocker(fakeDocker)
	err := dh.RemoveImage("test-image-id")
	if err != nil {
		t.Errorf("Unexpected error removing image: %s", err)
	}
	if fakeDocker.RemoveImageName != "test-image-id" {
		t.Errorf("Unexpected image removed: %s", fakeDocker.RemoveImageName)
	}
}
