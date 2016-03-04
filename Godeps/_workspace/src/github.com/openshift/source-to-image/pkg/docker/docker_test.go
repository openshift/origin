package docker

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/docker/test"
	"github.com/openshift/source-to-image/pkg/errors"

	docker "github.com/fsouza/go-dockerclient"
)

func getDocker(client Client) *stiDocker {
	return &stiDocker{
		client:   client,
		pullAuth: docker.AuthConfiguration{},
	}
}

func TestIsImageInLocalRegistry(t *testing.T) {
	type testDef struct {
		imageName         string
		docker            test.FakeDockerClient
		expectedImageName string
		expectedResult    bool
		expectedError     error
	}
	otherError := fmt.Errorf("Other")
	tests := map[string]testDef{
		"ImageFound":    {"a_test_image", test.FakeDockerClient{Image: &docker.Image{}}, "a_test_image:latest", true, nil},
		"ImageNotFound": {"a_test_image:sometag", test.FakeDockerClient{InspectImageErr: []error{docker.ErrNoSuchImage}}, "a_test_image:sometag", false, nil},
		"ErrorOccurred": {"a_test_image", test.FakeDockerClient{InspectImageErr: []error{otherError}}, "a_test_image:latest", false, otherError},
	}

	for test, def := range tests {
		dh := getDocker(&def.docker)
		result, err := dh.IsImageInLocalRegistry(def.imageName)
		if result != def.expectedResult {
			t.Errorf("Test - %s: Expected result: %v. Got: %v", test, def.expectedResult, result)
		}
		if err != def.expectedError {
			t.Errorf("Test - %s: Expected error: %v. Got: %v", test, def.expectedError, err)
		}
		if def.docker.InspectImageName[0] != def.expectedImageName {
			t.Errorf("Docker inspect called with unexpected image name: %s\n",
				def.docker.InspectImageName)
		}
	}
}

func TestCheckAndPullImage(t *testing.T) {
	type testDef struct {
		imageName           string
		docker              test.FakeDockerClient
		expectedImage       *docker.Image
		expectedError       int
		expectedPullOptions docker.PullImageOptions
	}
	image := &docker.Image{}
	imageExistsTest := testDef{
		imageName: "test_image",
		docker: test.FakeDockerClient{
			InspectImageErr:    []error{nil},
			InspectImageResult: []*docker.Image{image},
		},
		expectedImage: image,
	}
	imagePulledTest := testDef{
		imageName: "test_image",
		docker: test.FakeDockerClient{
			InspectImageErr:    []error{docker.ErrNoSuchImage, nil},
			InspectImageResult: []*docker.Image{nil, image},
		},
		expectedImage:       image,
		expectedPullOptions: docker.PullImageOptions{Repository: "test_image:" + DefaultTag},
	}
	inspectErrorTest := testDef{
		imageName: "test_image",
		docker: test.FakeDockerClient{
			InspectImageErr:    []error{docker.ErrConnectionRefused},
			InspectImageResult: []*docker.Image{nil},
		},
		expectedImage: nil,
		expectedError: errors.InspectImageError,
	}
	pullErrorTest := testDef{
		imageName: "test_image",
		docker: test.FakeDockerClient{
			PullImageErr:       docker.ErrConnectionRefused,
			InspectImageErr:    []error{nil},
			InspectImageResult: []*docker.Image{nil},
		},
		expectedImage:       nil,
		expectedError:       errors.PullImageError,
		expectedPullOptions: docker.PullImageOptions{Repository: "test_image:" + DefaultTag},
	}
	errorAfterPullTest := testDef{
		imageName: "test_image:testtag",
		docker: test.FakeDockerClient{
			InspectImageErr:    []error{docker.ErrNoSuchImage, docker.ErrNoSuchImage},
			InspectImageResult: []*docker.Image{nil, image},
		},
		expectedImage:       nil,
		expectedError:       errors.InspectImageError,
		expectedPullOptions: docker.PullImageOptions{Repository: "test_image:testtag"},
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
		resultImage, resultErr := dh.CheckAndPullImage(def.imageName)
		if resultImage != def.expectedImage {
			t.Errorf("%s: Unexpected image result -- %v", test, resultImage)
		}
		if e, ok := resultErr.(errors.Error); def.expectedError != 0 && (!ok || e.ErrorCode != def.expectedError) {
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
	containerID := "testContainerId"
	expectedOpts := docker.RemoveContainerOptions{
		ID:            containerID,
		RemoveVolumes: true,
		Force:         true,
	}
	dh.RemoveContainer(containerID)
	if !reflect.DeepEqual(expectedOpts, fakeDocker.RemoveContainerOpts) {
		t.Errorf("Unexpected removeContainerOpts. Expected: %#v, Got: %#v",
			expectedOpts, fakeDocker.RemoveContainerOpts)
	}
}

func TestCommitContainer(t *testing.T) {
	expectedImageID := "test-1234"
	fakeDocker := &test.FakeDockerClient{Image: &docker.Image{ID: expectedImageID}}
	dh := getDocker(fakeDocker)
	containerID := "test-container-id"
	containerTag := "test-container-tag"

	opt := CommitContainerOptions{
		ContainerID: containerID,
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
	containerID := "test-container-id"
	containerTag := "test-container-tag"

	opt := CommitContainerOptions{
		ContainerID: containerID,
		Repository:  containerTag,
	}

	_, err := dh.CommitContainer(opt)

	expectedOpts := docker.CommitContainerOptions{
		Container:  containerID,
		Repository: containerTag,
	}
	if !reflect.DeepEqual(expectedOpts, fakeDocker.CommitContainerOpts) {
		t.Errorf("Commit container called with unexpected parameters: %#v", fakeDocker.CommitContainerOpts)
	}
	if err != expectedErr {
		t.Errorf("Unexpected error returned: %v", err)
	}
}

func TestGetScriptsURL(t *testing.T) {
	type urltest struct {
		image       docker.Image
		result      string
		inspectErr  error
		errExpected bool
	}
	tests := map[string]urltest{
		"not present": {
			image: docker.Image{
				ContainerConfig: docker.Config{
					Env:    []string{"Env1=value1"},
					Labels: map[string]string{},
				},
				Config: &docker.Config{
					Env:    []string{"Env2=value2"},
					Labels: map[string]string{},
				},
			},
			result: "",
		},

		"env in containerConfig": {
			image: docker.Image{
				ContainerConfig: docker.Config{
					Env: []string{"Env1=value1", ScriptsURLEnvironment + "=test_url_value"},
				},
				Config: &docker.Config{},
			},
			result: "test_url_value",
		},

		"env in image config": {
			image: docker.Image{
				ContainerConfig: docker.Config{},
				Config: &docker.Config{
					Env: []string{
						"Env1=value1",
						ScriptsURLEnvironment + "=test_url_value_2",
						"Env2=value2",
					},
				},
			},
			result: "test_url_value_2",
		},

		"label in containerConfig": {
			image: docker.Image{
				ContainerConfig: docker.Config{
					Labels: map[string]string{ScriptsURLLabel: "test_url_value"},
				},
				Config: &docker.Config{},
			},
			result: "test_url_value",
		},

		"label in image config": {
			image: docker.Image{
				ContainerConfig: docker.Config{},
				Config: &docker.Config{
					Labels: map[string]string{ScriptsURLLabel: "test_url_value_2"},
				},
			},
			result: "test_url_value_2",
		},

		"inspect error": {
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
		url, err := dh.GetScriptsURL("test/image")
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
	type runtest struct {
		image            docker.Image
		cmd              string
		externalScripts  bool
		paramScriptsURL  string
		paramDestination string
		cmdExpected      []string
	}

	tests := map[string]runtest{
		"default": {
			image: docker.Image{
				ContainerConfig: docker.Config{},
				Config:          &docker.Config{},
			},
			cmd:             api.Assemble,
			externalScripts: true,
			cmdExpected:     []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /tmp -xf - && /tmp/scripts/%s", api.Assemble)},
		},
		"paramDestination": {
			image: docker.Image{
				ContainerConfig: docker.Config{},
				Config:          &docker.Config{},
			},
			cmd:              api.Assemble,
			externalScripts:  true,
			paramDestination: "/opt/test",
			cmdExpected:      []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /opt/test -xf - && /opt/test/scripts/%s", api.Assemble)},
		},
		"paramDestination&paramScripts": {
			image: docker.Image{
				ContainerConfig: docker.Config{},
				Config:          &docker.Config{},
			},
			cmd:              api.Assemble,
			externalScripts:  true,
			paramDestination: "/opt/test",
			paramScriptsURL:  "http://my.test.url/test?param=one",
			cmdExpected:      []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /opt/test -xf - && /opt/test/scripts/%s", api.Assemble)},
		},
		"scriptsInsideImageEnvironment": {
			image: docker.Image{
				ContainerConfig: docker.Config{
					Env: []string{ScriptsURLEnvironment + "=image:///opt/bin/"},
				},
				Config: &docker.Config{},
			},
			cmd:             api.Assemble,
			externalScripts: false,
			cmdExpected:     []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /tmp -xf - && /opt/bin/%s", api.Assemble)},
		},
		"scriptsInsideImageLabel": {
			image: docker.Image{
				ContainerConfig: docker.Config{
					Labels: map[string]string{ScriptsURLLabel: "image:///opt/bin/"},
				},
				Config: &docker.Config{},
			},
			cmd:             api.Assemble,
			externalScripts: false,
			cmdExpected:     []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /tmp -xf - && /opt/bin/%s", api.Assemble)},
		},
		"scriptsInsideImageEnvironmentWithParamDestination": {
			image: docker.Image{
				ContainerConfig: docker.Config{
					Env: []string{ScriptsURLEnvironment + "=image:///opt/bin"},
				},
				Config: &docker.Config{},
			},
			cmd:              api.Assemble,
			externalScripts:  false,
			paramDestination: "/opt/sti",
			cmdExpected:      []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /opt/sti -xf - && /opt/bin/%s", api.Assemble)},
		},
		"scriptsInsideImageLabelWithParamDestination": {
			image: docker.Image{
				ContainerConfig: docker.Config{
					Labels: map[string]string{ScriptsURLLabel: "image:///opt/bin"},
				},
				Config: &docker.Config{},
			},
			cmd:              api.Assemble,
			externalScripts:  false,
			paramDestination: "/opt/sti",
			cmdExpected:      []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /opt/sti -xf - && /opt/bin/%s", api.Assemble)},
		},
		"paramDestinationFromImageEnvironment": {
			image: docker.Image{
				ContainerConfig: docker.Config{
					Env: []string{LocationEnvironment + "=/opt", ScriptsURLEnvironment + "=http://my.test.url/test?param=one"},
				},
				Config: &docker.Config{},
			},
			cmd:             api.Assemble,
			externalScripts: true,
			cmdExpected:     []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /opt -xf - && /opt/scripts/%s", api.Assemble)},
		},
		"paramDestinationFromImageLabel": {
			image: docker.Image{
				ContainerConfig: docker.Config{
					Labels: map[string]string{DestinationLabel: "/opt", ScriptsURLLabel: "http://my.test.url/test?param=one"},
				},
				Config: &docker.Config{},
			},
			cmd:             api.Assemble,
			externalScripts: true,
			cmdExpected:     []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /opt -xf - && /opt/scripts/%s", api.Assemble)},
		},
		"usageCommand": {
			image: docker.Image{
				ContainerConfig: docker.Config{},
				Config:          &docker.Config{},
			},
			cmd:             api.Usage,
			externalScripts: true,
			cmdExpected:     []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /tmp -xf - && /tmp/scripts/%s", api.Usage)},
		},
		"otherCommand": {
			image: docker.Image{
				ContainerConfig: docker.Config{},
				Config:          &docker.Config{},
			},
			cmd:             api.Run,
			externalScripts: true,
			cmdExpected:     []string{fmt.Sprintf("/tmp/scripts/%s", api.Run)},
		},
	}

	for desc, tst := range tests {
		fakeDocker := &test.FakeDockerClient{
			InspectImageResult: []*docker.Image{&tst.image},
			Container: &docker.Container{
				ID: "12345-test",
			},
			AttachToContainerSleep: 200 * time.Millisecond,
		}
		dh := getDocker(fakeDocker)
		err := dh.RunContainer(RunContainerOptions{
			Image:           "test/image",
			PullImage:       true,
			ExternalScripts: tst.externalScripts,
			ScriptsURL:      tst.paramScriptsURL,
			Destination:     tst.paramDestination,
			Command:         tst.cmd,
			Env:             []string{"Key1=Value1", "Key2=Value2"},
			Stdin:           os.Stdin,
			Stdout:          os.Stdout,
			Stderr:          os.Stdout,
		})
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", desc, err)
		}
		// Validate the CreateContainer parameters
		createConfig := fakeDocker.CreateContainerOpts.Config
		if createConfig.Image != "test/image:latest" {
			t.Errorf("%s: Unexpected create config image: %s", desc, createConfig.Image)
		}
		if !reflect.DeepEqual(createConfig.Cmd, tst.cmdExpected) {
			t.Errorf("%s: Unexpected create config command: %#v", desc, createConfig.Cmd)
		}
		if !reflect.DeepEqual(createConfig.Env, []string{"Key1=Value1", "Key2=Value2"}) {
			t.Errorf("%s: Unexpected create config env: %#v", desc, createConfig.Env)
		}
		if !createConfig.OpenStdin || !createConfig.StdinOnce {
			t.Errorf("%s: Unexpected stdin flags for createConfig: OpenStdin - %v"+
				" StdinOnce - %v", desc, createConfig.OpenStdin, createConfig.StdinOnce)
		}

		// Verify that remove container was called
		if fakeDocker.RemoveContainerOpts.ID != "12345-test" {
			t.Errorf("%s: RemoveContainer was not called with the expected container ID", desc)
		}

		// Verify that AttachToContainer was called twice (Stdin/Stdout)
		if len(fakeDocker.AttachToContainerOpts) != 1 {
			t.Errorf("%s: AttachToContainer was not called the expected number of times.", desc)
		}
		// Make sure AttachToContainer was not called with both Stdin & Stdout
		for _, opt := range fakeDocker.AttachToContainerOpts {
			if opt.InputStream == nil || opt.OutputStream == nil {
				t.Errorf("%s: AttachToContainer was not called with both Stdin and Stdout: %#v", desc, opt)
			}
			if !opt.Stdin || !opt.Stdout {
				t.Errorf("%s: AttachToContainer was not called with both Stdin and Stdout flags: %#v", desc, opt)
			}
		}
	}
}

func TestGetImageID(t *testing.T) {
	fakeDocker := &test.FakeDockerClient{
		InspectImageResult: []*docker.Image{{ID: "test-abcd"}},
	}
	dh := getDocker(fakeDocker)
	id, err := dh.GetImageID("test/image")
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
	id, err := dh.GetImageID("test/image")
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

func TestGetImageName(t *testing.T) {
	type runtest struct {
		name     string
		expected string
	}
	tests := []runtest{
		{"test/image", "test/image:latest"},
		{"test/image:latest", "test/image:latest"},
		{"test/image:tag", "test/image:tag"},
		{"repository/test/image", "repository/test/image:latest"},
		{"repository/test/image:latest", "repository/test/image:latest"},
		{"repository/test/image:tag", "repository/test/image:tag"},
	}

	for _, tc := range tests {
		if e, a := tc.expected, getImageName(tc.name); e != a {
			t.Errorf("Expected image name %s, but got %s!", e, a)
		}
	}
}
