package builder

import (
	"errors"
	"reflect"
	"testing"

	"github.com/fsouza/go-dockerclient"
)

type FakeDocker struct {
	pushImageFunc   func(opts docker.PushImageOptions, auth docker.AuthConfiguration) error
	buildImageFunc  func(opts docker.BuildImageOptions) error
	removeImageFunc func(name string) error

	buildImageCalled  bool
	pushImageCalled   bool
	removeImageCalled bool
	errPushImage      error

	callLog []methodCall
}

type methodCall struct {
	methodName string
	args       []interface{}
}

func NewFakeDockerClient() *FakeDocker {
	return &FakeDocker{}
}

var fooBarRunTimes = 0

func fakePushImageFunc(opts docker.PushImageOptions, auth docker.AuthConfiguration) error {
	switch opts.Tag {
	case "tag_test_succ_foo_bar":
		return nil
	case "tag_test_err_exist_foo_bar":
		fooBarRunTimes++
		return errors.New(RetriableErrors[0])
	case "tag_test_err_no_exist_foo_bar":
		return errors.New("no_exist_err_foo_bar")
	}
	return nil
}
func (d *FakeDocker) BuildImage(opts docker.BuildImageOptions) error {
	if d.buildImageFunc != nil {
		return d.buildImageFunc(opts)
	}
	return nil
}
func (d *FakeDocker) PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) error {
	d.pushImageCalled = true
	if d.pushImageFunc != nil {
		return d.pushImageFunc(opts, auth)
	}
	return d.errPushImage
}
func (d *FakeDocker) RemoveImage(name string) error {
	if d.removeImageFunc != nil {
		return d.removeImageFunc(name)
	}
	return nil
}
func (d *FakeDocker) CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error) {
	return &docker.Container{}, nil
}
func (d *FakeDocker) DownloadFromContainer(id string, opts docker.DownloadFromContainerOptions) error {
	return nil
}
func (d *FakeDocker) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error {
	return nil
}
func (d *FakeDocker) RemoveContainer(opts docker.RemoveContainerOptions) error {
	return nil
}
func (d *FakeDocker) InspectImage(name string) (*docker.Image, error) {
	return &docker.Image{}, nil
}
func (d *FakeDocker) StartContainer(id string, hostConfig *docker.HostConfig) error {
	return nil
}
func (d *FakeDocker) WaitContainer(id string) (int, error) {
	return 0, nil
}
func (d *FakeDocker) Logs(opts docker.LogsOptions) error {
	return nil
}
func (d *FakeDocker) TagImage(name string, opts docker.TagImageOptions) error {
	d.callLog = append(d.callLog, methodCall{"TagImage", []interface{}{name, opts}})
	return nil
}

func TestDockerPush(t *testing.T) {
	verifyFunc := func(opts docker.PushImageOptions, auth docker.AuthConfiguration) error {
		if opts.Name != "test/image" {
			t.Errorf("Unexpected image name: %s", opts.Name)
		}
		return nil
	}
	fd := &FakeDocker{pushImageFunc: verifyFunc}
	pushImage(fd, "test/image", docker.AuthConfiguration{})
}

func TestTagImage(t *testing.T) {
	tests := []struct {
		old, new, newRepo, newTag string
	}{
		{"test/image", "new/image:tag", "new/image", "tag"},
		{"test/image:1.0", "new-name", "new-name", ""},
	}
	for _, tt := range tests {
		dockerClient := &FakeDocker{}
		tagImage(dockerClient, tt.old, tt.new)
		got := dockerClient.callLog
		tagOpts := docker.TagImageOptions{
			Repo:  tt.newRepo,
			Tag:   tt.newTag,
			Force: true,
		}
		want := []methodCall{
			{"TagImage", []interface{}{tt.old, tagOpts}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("dockerClient called with %#v, want %#v", got, want)
		}
	}
}

func TestPushImage(t *testing.T) {
	var testImageName string

	bakRetryCount := DefaultPushRetryCount
	bakRetryDelay := DefaultPushRetryDelay

	fakeDocker := NewFakeDockerClient()
	fakeDocker.pushImageFunc = fakePushImageFunc
	testAuth := docker.AuthConfiguration{
		Username:      "usernname_foo_bar",
		Password:      "password_foo_bar",
		Email:         "email_foo_bar",
		ServerAddress: "serveraddress_foo_bar",
	}

	//make test quickly, and recover the value after testing
	DefaultPushRetryCount = 2
	defer func() { DefaultPushRetryCount = bakRetryCount }()
	DefaultPushRetryDelay = 1
	defer func() { DefaultPushRetryDelay = bakRetryDelay }()

	//expect succ
	testImageName = "repo_foo_bar:tag_test_succ_foo_bar"
	if err := pushImage(fakeDocker, testImageName, testAuth); err != nil {
		t.Errorf("Unexpect push image : %v, want succ", err)
	}

	//expect fail
	testImageName = "repo_foo_bar:tag_test_err_exist_foo_bar"
	err := pushImage(fakeDocker, testImageName, testAuth)
	if err == nil {
		t.Errorf("Unexpect push image : %v, want error", err)
	}
	//expect run 3 times
	if fooBarRunTimes != (DefaultPushRetryCount + 1) {
		t.Errorf("Unexpect run times : %d, we expect run three times", fooBarRunTimes)
	}

	//expect fail
	testImageName = "repo_foo_bar:tag_test_err_no_exist_foo_bar"
	if err := pushImage(fakeDocker, testImageName, testAuth); err == nil {
		t.Errorf("Unexpect push image : %v, want error", err)
	}
	defer func() { fooBarRunTimes = 0 }()
}
