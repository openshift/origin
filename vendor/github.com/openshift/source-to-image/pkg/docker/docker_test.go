package docker

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	dockertest "github.com/openshift/source-to-image/pkg/docker/test"
	"github.com/openshift/source-to-image/pkg/test"

	dockertypes "github.com/docker/engine-api/types"
	dockercontainer "github.com/docker/engine-api/types/container"
	dockerstrslice "github.com/docker/engine-api/types/strslice"
)

func TestContainerName(t *testing.T) {
	rand.Seed(0)
	got := containerName("sub.domain.com:5000/repo:tag@sha256:ffffff")
	want := "s2i_sub_domain_com_5000_repo_tag_sha256_ffffff_f1f85ff5"
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func getDocker(client Client) *stiDocker {
	return &stiDocker{
		client:   client,
		pullAuth: dockertypes.AuthConfig{},
	}
}

func TestRemoveContainer(t *testing.T) {
	fakeDocker := dockertest.NewFakeDockerClient()
	dh := getDocker(fakeDocker)
	containerID := "testContainerId"
	fakeDocker.Containers[containerID] = dockercontainer.Config{}
	err := dh.RemoveContainer(containerID)
	if err != nil {
		t.Errorf("%+v", err)
	}
	expectedCalls := []string{"remove"}
	if !reflect.DeepEqual(fakeDocker.Calls, expectedCalls) {
		t.Errorf("Expected fakeDocker.Calls %v, got %v", expectedCalls, fakeDocker.Calls)
	}
}

func TestCommitContainer(t *testing.T) {
	type commitTest struct {
		containerID     string
		containerTag    string
		expectedImageID string
		expectedError   error
	}

	tests := map[string]commitTest{
		"valid": {
			containerID:     "test-container-id",
			containerTag:    "test-container-tag",
			expectedImageID: "test-container-tag",
		},
		"error": {
			containerID:     "test-container-id",
			containerTag:    "test-container-tag",
			expectedImageID: "test-container-tag",
			expectedError:   fmt.Errorf("Test error"),
		},
	}

	for desc, tst := range tests {
		opt := CommitContainerOptions{
			ContainerID: tst.containerID,
			Repository:  tst.containerTag,
		}
		param := dockertypes.ContainerCommitOptions{
			Reference: tst.containerTag,
		}
		resp := dockertypes.ContainerCommitResponse{
			ID: tst.expectedImageID,
		}
		fakeDocker := &dockertest.FakeDockerClient{
			ContainerCommitResponse: resp,
			ContainerCommitErr:      tst.expectedError,
		}
		dh := getDocker(fakeDocker)

		imageID, err := dh.CommitContainer(opt)
		if err != tst.expectedError {
			t.Errorf("test case %s: Unexpected error returned: %v", desc, err)
		}
		if tst.containerID != fakeDocker.ContainerCommitID {
			t.Errorf("test case %s: Commit container called with unexpected container id: %s and %+v", desc, tst.containerID, fakeDocker.ContainerCommitID)
		}
		if !reflect.DeepEqual(param, fakeDocker.ContainerCommitOptions) {
			t.Errorf("test case %s: Commit container called with unexpected parameters: %+v and %+v", desc, param, fakeDocker.ContainerCommitOptions)
		}
		if tst.expectedError == nil && imageID != tst.expectedImageID {
			t.Errorf("test case %s: Did not return the correct image id: %s", desc, imageID)
		}
	}
}

func TestCopyToContainer(t *testing.T) {
	type copyToTest struct {
		containerID string
		src         string
		destPath    string
	}

	tests := map[string]copyToTest{
		"valid": {
			containerID: "test-container-id",
			src:         "foo",
		},
		"error": {
			containerID: "test-container-id",
			src:         "badsource",
		},
	}

	for desc, tst := range tests {
		var tempDir, fileName string
		var err error
		var file *os.File
		if len(tst.src) > 0 {
			tempDir, err = ioutil.TempDir("", tst.src)
			defer os.RemoveAll(tempDir)
			fileName = filepath.Join(tempDir, "bar")
			if err = os.MkdirAll(filepath.Dir(fileName), 0700); err == nil {
				file, err = os.Create(fileName)
				if err == nil {
					defer file.Close()
					file.WriteString("asdf")
				}
			}
		}
		if err != nil {
			t.Fatalf("Error creating src test files: %v", err)
		}
		fakeDocker := &dockertest.FakeDockerClient{
			CopyToContainerContent: file,
		}
		dh := getDocker(fakeDocker)

		err = dh.UploadToContainer(&test.FakeFileSystem{}, fileName, fileName, tst.containerID)
		// the error we are inducing will prevent call into engine-api
		if len(tst.src) > 0 {
			if err != nil {
				t.Errorf("test case %s: Unexpected error returned: %v", desc, err)
			}
			if tst.containerID != fakeDocker.CopyToContainerID {
				t.Errorf("test case %s: copy to container called with unexpected id: %s and %s", desc, tst.containerID, fakeDocker.CopyToContainerID)
			}
		} else {
			if err == nil {
				t.Errorf("test case %s: Unexpected error returned: %v", desc, err)
			}
			if len(fakeDocker.CopyToContainerID) > 0 {
				t.Errorf("test case %s: copy to container called with unexpected id: %s and %s", desc, tst.containerID, fakeDocker.CopyToContainerID)
			}
		}
		// the directory of our file gets passed down to the engine-api method
		if tempDir != fakeDocker.CopyToContainerPath {
			t.Errorf("test case %s: copy to container called with unexpected path: %s and %s", desc, tempDir, fakeDocker.CopyToContainerPath)
		}
		// reflect.DeepEqual does not help here cause the reader is transformed prior to calling the engine-api stack, so just make sure it is no nil
		if file != nil && fakeDocker.CopyToContainerContent == nil {
			t.Errorf("test case %s: copy to container content was not passed through", desc)
		}
	}
}

func TestCopyFromContainer(t *testing.T) {
	type copyFromTest struct {
		containerID   string
		srcPath       string
		expectedError error
	}

	tests := map[string]copyFromTest{
		"valid": {
			containerID: "test-container-id",
			srcPath:     "/foo/bar",
		},
		"error": {
			containerID:   "test-container-id",
			srcPath:       "/foo/bar",
			expectedError: fmt.Errorf("Test error"),
		},
	}

	for desc, tst := range tests {
		buffer := bytes.NewBuffer([]byte(""))
		fakeDocker := &dockertest.FakeDockerClient{
			CopyFromContainerErr: tst.expectedError,
		}
		dh := getDocker(fakeDocker)

		err := dh.DownloadFromContainer(tst.srcPath, buffer, tst.containerID)
		if err != tst.expectedError {
			t.Errorf("test case %s: Unexpected error returned: %v", desc, err)
		}
		if fakeDocker.CopyFromContainerID != tst.containerID {
			t.Errorf("test case %s: Unexpected container id: %s and %s", desc, tst.containerID, fakeDocker.CopyFromContainerID)
		}
		if fakeDocker.CopyFromContainerPath != tst.srcPath {
			t.Errorf("test case %s: Unexpected container id: %s and %s", desc, tst.srcPath, fakeDocker.CopyFromContainerPath)
		}
	}
}

func TestImageBuild(t *testing.T) {
	type waitTest struct {
		imageID       string
		expectedError error
	}

	tests := map[string]waitTest{
		"valid": {
			imageID: "test-container-id",
		},
		"error": {
			imageID:       "test-container-id",
			expectedError: fmt.Errorf("Test error"),
		},
	}

	for desc, tst := range tests {
		fakeDocker := &dockertest.FakeDockerClient{
			BuildImageErr: tst.expectedError,
		}
		dh := getDocker(fakeDocker)
		opts := BuildImageOptions{
			Name: tst.imageID,
		}

		err := dh.BuildImage(opts)
		if err != tst.expectedError {
			t.Errorf("test case %s: Unexpected error returned: %v", desc, err)
		}
		if len(fakeDocker.BuildImageOpts.Tags) != 1 || fakeDocker.BuildImageOpts.Tags[0] != tst.imageID {
			t.Errorf("test case %s: Unexpected container id: %s and %+v", desc, tst.imageID, fakeDocker.BuildImageOpts.Tags)
		}
	}
}

func TestGetScriptsURL(t *testing.T) {
	type urltest struct {
		image      dockertypes.ImageInspect
		result     string
		calls      []string
		inspectErr error
	}
	tests := map[string]urltest{
		"not present": {
			calls: []string{"inspect_image"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{
					Env:    []string{"Env1=value1"},
					Labels: map[string]string{},
				},
				Config: &dockercontainer.Config{
					Env:    []string{"Env2=value2"},
					Labels: map[string]string{},
				},
			},
			result: "",
		},

		"env in containerConfig": {
			calls: []string{"inspect_image"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{
					Env: []string{"Env1=value1", ScriptsURLEnvironment + "=test_url_value"},
				},
				Config: &dockercontainer.Config{},
			},
			result: "",
		},

		"env in image config": {
			calls: []string{"inspect_image"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config: &dockercontainer.Config{
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
			calls: []string{"inspect_image"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{
					Labels: map[string]string{ScriptsURLLabel: "test_url_value"},
				},
				Config: &dockercontainer.Config{},
			},
			result: "",
		},

		"label in image config": {
			calls: []string{"inspect_image"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config: &dockercontainer.Config{
					Labels: map[string]string{ScriptsURLLabel: "test_url_value_2"},
				},
			},
			result: "test_url_value_2",
		},

		"inspect error": {
			calls:      []string{"inspect_image", "pull"},
			image:      dockertypes.ImageInspect{},
			inspectErr: fmt.Errorf("Inspect error"),
		},
	}
	for desc, tst := range tests {
		fakeDocker := dockertest.NewFakeDockerClient()
		dh := getDocker(fakeDocker)
		tst.image.ID = "test/image:latest"
		if tst.inspectErr != nil {
			fakeDocker.PullFail = tst.inspectErr
		} else {
			fakeDocker.Images = map[string]dockertypes.ImageInspect{tst.image.ID: tst.image}
		}
		url, err := dh.GetScriptsURL(tst.image.ID)

		if !reflect.DeepEqual(fakeDocker.Calls, tst.calls) {
			t.Errorf("%s: Expected fakeDocker.Calls %v, got %v", desc, tst.calls, fakeDocker.Calls)
		}
		if err != nil && tst.inspectErr == nil {
			t.Errorf("%s: Unexpected error returned: %v", desc, err)
		}
		if tst.inspectErr == nil && url != tst.result {
			//t.Errorf("%s: Unexpected result. Expected: %s Actual: %s",
			//	desc, tst.result, url)
		}
	}
}

func TestRunContainer(t *testing.T) {
	type runtest struct {
		calls            []string
		image            dockertypes.ImageInspect
		cmd              string
		externalScripts  bool
		paramScriptsURL  string
		paramDestination string
		cmdExpected      []string
	}

	tests := map[string]runtest{
		"default": {
			calls: []string{"inspect_image", "inspect_image", "inspect_image", "create", "attach", "start", "remove"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config:          &dockercontainer.Config{},
			},
			cmd:             api.Assemble,
			externalScripts: true,
			cmdExpected:     []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /tmp -xf - && /tmp/scripts/%s", api.Assemble)},
		},
		"paramDestination": {
			calls: []string{"inspect_image", "inspect_image", "inspect_image", "create", "attach", "start", "remove"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config:          &dockercontainer.Config{},
			},
			cmd:              api.Assemble,
			externalScripts:  true,
			paramDestination: "/opt/test",
			cmdExpected:      []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /opt/test -xf - && /opt/test/scripts/%s", api.Assemble)},
		},
		"paramDestination&paramScripts": {
			calls: []string{"inspect_image", "inspect_image", "inspect_image", "create", "attach", "start", "remove"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config:          &dockercontainer.Config{},
			},
			cmd:              api.Assemble,
			externalScripts:  true,
			paramDestination: "/opt/test",
			paramScriptsURL:  "http://my.test.url/test?param=one",
			cmdExpected:      []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /opt/test -xf - && /opt/test/scripts/%s", api.Assemble)},
		},
		"scriptsInsideImageEnvironment": {
			calls: []string{"inspect_image", "inspect_image", "inspect_image", "create", "attach", "start", "remove"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config: &dockercontainer.Config{
					Env: []string{ScriptsURLEnvironment + "=image:///opt/bin/"},
				},
			},
			cmd:             api.Assemble,
			externalScripts: false,
			cmdExpected:     []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /tmp -xf - && /opt/bin/%s", api.Assemble)},
		},
		"scriptsInsideImageLabel": {
			calls: []string{"inspect_image", "inspect_image", "inspect_image", "create", "attach", "start", "remove"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config: &dockercontainer.Config{
					Labels: map[string]string{ScriptsURLLabel: "image:///opt/bin/"},
				},
			},
			cmd:             api.Assemble,
			externalScripts: false,
			cmdExpected:     []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /tmp -xf - && /opt/bin/%s", api.Assemble)},
		},
		"scriptsInsideImageEnvironmentWithParamDestination": {
			calls: []string{"inspect_image", "inspect_image", "inspect_image", "create", "attach", "start", "remove"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config: &dockercontainer.Config{
					Env: []string{ScriptsURLEnvironment + "=image:///opt/bin"},
				},
			},
			cmd:              api.Assemble,
			externalScripts:  false,
			paramDestination: "/opt/sti",
			cmdExpected:      []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /opt/sti -xf - && /opt/bin/%s", api.Assemble)},
		},
		"scriptsInsideImageLabelWithParamDestination": {
			calls: []string{"inspect_image", "inspect_image", "inspect_image", "create", "attach", "start", "remove"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config: &dockercontainer.Config{
					Labels: map[string]string{ScriptsURLLabel: "image:///opt/bin"},
				},
			},
			cmd:              api.Assemble,
			externalScripts:  false,
			paramDestination: "/opt/sti",
			cmdExpected:      []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /opt/sti -xf - && /opt/bin/%s", api.Assemble)},
		},
		"paramDestinationFromImageEnvironment": {
			calls: []string{"inspect_image", "inspect_image", "inspect_image", "create", "attach", "start", "remove"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config: &dockercontainer.Config{
					Env: []string{LocationEnvironment + "=/opt", ScriptsURLEnvironment + "=http://my.test.url/test?param=one"},
				},
			},
			cmd:             api.Assemble,
			externalScripts: true,
			cmdExpected:     []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /opt -xf - && /opt/scripts/%s", api.Assemble)},
		},
		"paramDestinationFromImageLabel": {
			calls: []string{"inspect_image", "inspect_image", "inspect_image", "create", "attach", "start", "remove"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config: &dockercontainer.Config{
					Labels: map[string]string{DestinationLabel: "/opt", ScriptsURLLabel: "http://my.test.url/test?param=one"},
				},
			},
			cmd:             api.Assemble,
			externalScripts: true,
			cmdExpected:     []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /opt -xf - && /opt/scripts/%s", api.Assemble)},
		},
		"usageCommand": {
			calls: []string{"inspect_image", "inspect_image", "inspect_image", "create", "attach", "start", "remove"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config:          &dockercontainer.Config{},
			},
			cmd:             api.Usage,
			externalScripts: true,
			cmdExpected:     []string{"/bin/sh", "-c", fmt.Sprintf("tar -C /tmp -xf - && /tmp/scripts/%s", api.Usage)},
		},
		"otherCommand": {
			calls: []string{"inspect_image", "inspect_image", "inspect_image", "create", "attach", "start", "remove"},
			image: dockertypes.ImageInspect{
				ContainerConfig: &dockercontainer.Config{},
				Config:          &dockercontainer.Config{},
			},
			cmd:             api.Run,
			externalScripts: true,
			cmdExpected:     []string{fmt.Sprintf("/tmp/scripts/%s", api.Run)},
		},
	}

	for desc, tst := range tests {
		fakeDocker := dockertest.NewFakeDockerClient()
		dh := getDocker(fakeDocker)
		tst.image.ID = "test/image:latest"
		fakeDocker.Images = map[string]dockertypes.ImageInspect{tst.image.ID: tst.image}
		if len(fakeDocker.Containers) > 0 {
			t.Errorf("newly created fake client should have empty container map: %+v", fakeDocker.Containers)
		}

		err := dh.RunContainer(RunContainerOptions{
			Image:           "test/image",
			PullImage:       true,
			ExternalScripts: tst.externalScripts,
			ScriptsURL:      tst.paramScriptsURL,
			Destination:     tst.paramDestination,
			Command:         tst.cmd,
			Env:             []string{"Key1=Value1", "Key2=Value2"},
			Stdin:           ioutil.NopCloser(os.Stdin),
		})
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", desc, err)
		}

		// container ID will be random, so don't look up directly ... just get the 1 entry which should be there
		if len(fakeDocker.Containers) != 1 {
			t.Errorf("fake container map should only have 1 entry: %+v", fakeDocker.Containers)
		}

		for _, container := range fakeDocker.Containers {
			// Validate the Container parameters
			if container.Image != "test/image:latest" {
				t.Errorf("%s: Unexpected create config image: %s", desc, container.Image)
			}
			if !reflect.DeepEqual(container.Cmd, dockerstrslice.StrSlice(tst.cmdExpected)) {
				t.Errorf("%s: Unexpected create config command: %#v instead of %q", desc, container.Cmd, strings.Join(tst.cmdExpected, " "))
			}
			if !reflect.DeepEqual(container.Env, []string{"Key1=Value1", "Key2=Value2"}) {
				t.Errorf("%s: Unexpected create config env: %#v", desc, container.Env)
			}
			if !reflect.DeepEqual(fakeDocker.Calls, tst.calls) {
				t.Errorf("%s: Expected fakeDocker.Calls %v, got %v", desc, tst.calls, fakeDocker.Calls)
			}
		}
	}
}

func TestGetImageID(t *testing.T) {
	fakeDocker := dockertest.NewFakeDockerClient()
	dh := getDocker(fakeDocker)
	image := dockertypes.ImageInspect{ID: "test-abcd:latest"}
	fakeDocker.Images = map[string]dockertypes.ImageInspect{image.ID: image}
	id, err := dh.GetImageID("test-abcd")
	expectedCalls := []string{"inspect_image"}
	if !reflect.DeepEqual(fakeDocker.Calls, expectedCalls) {
		t.Errorf("Expected fakeDocker.Calls %v, got %v", expectedCalls, fakeDocker.Calls)
	}
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	} else if id != image.ID {
		t.Errorf("Unexpected image id returned: %s", id)
	}
}

func TestRemoveImage(t *testing.T) {
	fakeDocker := dockertest.NewFakeDockerClient()
	dh := getDocker(fakeDocker)
	image := dockertypes.ImageInspect{ID: "test-abcd"}
	fakeDocker.Images = map[string]dockertypes.ImageInspect{image.ID: image}
	err := dh.RemoveImage("test-abcd")
	if err != nil {
		t.Errorf("Unexpected error removing image: %s", err)
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
