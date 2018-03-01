package builder

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
)

type FakeDocker struct {
	pushImageFunc   func(opts docker.PushImageOptions, auth docker.AuthConfiguration) error
	pullImageFunc   func(opts docker.PullImageOptions, auth docker.AuthConfiguration) error
	buildImageFunc  func(opts docker.BuildImageOptions) error
	removeImageFunc func(name string) error

	buildImageCalled  bool
	pushImageCalled   bool
	removeImageCalled bool
	errPushImage      error

	callLog []methodCall
}

var _ DockerClient = &FakeDocker{}

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

func fakePullImageFunc(opts docker.PullImageOptions, auth docker.AuthConfiguration) error {
	switch opts.Repository {
	case "repo_test_succ_foo_bar":
		return nil
	case "repo_test_err_exist_foo_bar":
		fooBarRunTimes++
		return errors.New(RetriableErrors[0])
	case "repo_test_err_no_exist_foo_bar":
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
	if d.pullImageFunc != nil {
		return d.pullImageFunc(opts, auth)
	}
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
func (d *FakeDocker) AttachToContainerNonBlocking(opts docker.AttachToContainerOptions) (docker.CloseWaiter, error) {
	return nil, nil
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

	bakRetryCount := DefaultPushOrPullRetryCount
	bakRetryDelay := DefaultPushOrPullRetryDelay

	fakeDocker := NewFakeDockerClient()
	fakeDocker.pushImageFunc = fakePushImageFunc
	testAuth := docker.AuthConfiguration{
		Username:      "username_foo_bar",
		Password:      "password_foo_bar",
		Email:         "email_foo_bar",
		ServerAddress: "serveraddress_foo_bar",
	}

	//make test quickly, and recover the value after testing
	DefaultPushOrPullRetryCount = 2
	defer func() { DefaultPushOrPullRetryCount = bakRetryCount }()
	DefaultPushOrPullRetryDelay = 1
	defer func() { DefaultPushOrPullRetryDelay = bakRetryDelay }()

	//expect succ
	testImageName = "repo_foo_bar:tag_test_succ_foo_bar"
	if _, err := pushImage(fakeDocker, testImageName, testAuth); err != nil {
		t.Errorf("Unexpect push image : %v, want succ", err)
	}

	//expect fail
	testImageName = "repo_foo_bar:tag_test_err_exist_foo_bar"
	_, err := pushImage(fakeDocker, testImageName, testAuth)
	if err == nil {
		t.Errorf("Unexpect push image : %v, want error", err)
	}
	//expect run 3 times
	if fooBarRunTimes != (DefaultPushOrPullRetryCount + 1) {
		t.Errorf("Unexpect run times : %d, we expect run three times", fooBarRunTimes)
	}

	//expect fail
	testImageName = "repo_foo_bar:tag_test_err_no_exist_foo_bar"
	if _, err := pushImage(fakeDocker, testImageName, testAuth); err == nil {
		t.Errorf("Unexpect push image : %v, want error", err)
	}
	defer func() { fooBarRunTimes = 0 }()
}

func TestPullImage(t *testing.T) {
	var testImageName string

	bakRetryCount := DefaultPushOrPullRetryCount
	bakRetryDelay := DefaultPushOrPullRetryDelay

	fakeDocker := NewFakeDockerClient()
	fakeDocker.pullImageFunc = fakePullImageFunc
	testAuth := docker.AuthConfiguration{
		Username:      "username_foo_bar",
		Password:      "password_foo_bar",
		Email:         "email_foo_bar",
		ServerAddress: "serveraddress_foo_bar",
	}

	//make test quickly, and recover the value after testing
	DefaultPushOrPullRetryCount = 2
	defer func() { DefaultPushOrPullRetryCount = bakRetryCount }()
	DefaultPushOrPullRetryDelay = 1
	defer func() { DefaultPushOrPullRetryDelay = bakRetryDelay }()

	//expect succ
	testImageName = "repo_test_succ_foo_bar"
	if err := pullImage(fakeDocker, testImageName, testAuth); err != nil {
		t.Errorf("Unexpect pull image : %v, want succ", err)
	}

	//expect fail
	testImageName = "repo_test_err_exist_foo_bar"
	err := pullImage(fakeDocker, testImageName, testAuth)
	if err == nil {
		t.Errorf("Unexpect pull image : %v, want error", err)
	}
	//expect run 3 times
	if fooBarRunTimes != (DefaultPushOrPullRetryCount + 1) {
		t.Errorf("Unexpect run times : %d, we expect run three times", fooBarRunTimes)
	}

	//expect fail
	testImageName = "repo_test_err_no_exist_foo_bar"
	if err := pullImage(fakeDocker, testImageName, testAuth); err == nil {
		t.Errorf("Unexpect pull image : %v, want error", err)
	}
	defer func() { fooBarRunTimes = 0 }()
}

func TestGetContainerNameOrID(t *testing.T) {
	c := &docker.Container{}
	c.ID = "ID"
	c.Name = "Name"
	ret := getContainerNameOrID(c)
	if ret != c.Name {
		t.Errorf("getContainerNameOrID err. ret is %s", ret)
	}

	c.Name = ""
	ret = getContainerNameOrID(c)
	if ret != c.ID {
		t.Errorf("getContainerNameOrID err.ret is %s", ret)
	}
}

func TestPushWriter(t *testing.T) {
	tests := []struct {
		Writes    []string
		Expected  []progressLine
		ExpectErr string
	}{
		{
			Writes:   []string{"", "\n"},
			Expected: []progressLine{},
		},
		{
			// The writer doesn't know if another write is coming or not so
			// this is not an error.
			Writes:   []string{"{"},
			Expected: []progressLine{},
		},
		{
			Writes:   []string{"{}"},
			Expected: []progressLine{{}},
		},
		{
			Writes:   []string{"{", "}{", "}"},
			Expected: []progressLine{{}, {}},
		},
		{
			Writes:   []string{" ", "{", " ", "}{}{", "}"},
			Expected: []progressLine{{}, {}, {}},
		},
		{
			Writes:   []string{"{}\r\n{}\r\n{}\r\n"},
			Expected: []progressLine{{}, {}, {}},
		},
		{
			Writes: []string{"{\"progress\": \"1\"}\r\n{\"progress\": \"2\"}\r\n{\"progress\": \"3\"}\r\n"},
			Expected: []progressLine{
				{Progress: "1"},
				{Progress: "2"},
				{Progress: "3"},
			},
		},
		{
			Writes:    []string{"}"},
			ExpectErr: "invalid character",
		},
		{
			Writes:    []string{`{"error": "happened"}`},
			ExpectErr: "happened",
		},
		{
			Writes:    []string{`{"status": "good!"}{"`, `error": "front fell off"}`},
			ExpectErr: "front fell off",
		},
		{
			Writes: []string{`{"status": "good!"}{"st`, `atus": `, `"even better"}`},
			Expected: []progressLine{
				{Status: "good!"},
				{Status: "even better"},
			},
		},
	}

main:
	for i, tc := range tests {
		decoded := []progressLine{}
		w := newPushWriter(func(line progressLine) error {
			decoded = append(decoded, line)
			if len(line.Error) > 0 {
				return errors.New(line.Error)
			} else {
				return nil
			}
		})

		for _, part := range tc.Writes {
			n, err := w.Write([]byte(part))

			partLen := len([]byte(part))
			if n != partLen {
				t.Errorf("[%d] Wrote %d bytes but Write() returned %d", i, partLen, n)
				continue main
			}

			if err != nil {
				if len(tc.ExpectErr) > 0 && !strings.Contains(err.Error(), tc.ExpectErr) {
					t.Errorf("[%d] Expected error: %s, got: %s", i, tc.ExpectErr, err)
				}
				if len(tc.ExpectErr) == 0 {
					t.Errorf("[%d] Unexpected error: %s", i, err)
				}
				continue main
			}
		}

		if len(tc.ExpectErr) > 0 {
			t.Errorf("[%d] Expected error %q, got none", i, tc.ExpectErr)
			continue main
		}

		if !reflect.DeepEqual(tc.Expected, decoded) {
			t.Errorf("[%d] Expected: %#v\nGot: %#v\n", i, tc.Expected, decoded)
			continue main
		}
	}
}

func TestPushImageDigests(t *testing.T) {
	tests := []struct {
		Filename  string
		Expected  string
		ExpectErr bool
	}{
		{
			Filename: "docker-push-1.10.txt",
			Expected: "sha256:adc72a07c3a96ffc2201b1d9bf84b8f2416932a8e39000f61d0cda10761f2658",
		},
		{
			Filename: "docker-push-1.12.txt",
			Expected: "sha256:29f5d56d12684887bdfa50dcd29fc31eea4aaf4ad3bec43daf19026a7ce69912",
		},
		{
			Filename: "docker-push-exists.txt",
			Expected: "sha256:29f5d56d12684887bdfa50dcd29fc31eea4aaf4ad3bec43daf19026a7ce69912",
		},
		{
			Filename: "docker-push-0digests.txt",
			Expected: "",
		},
		{
			Filename: "docker-push-2digests.txt",
			Expected: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		{
			Filename:  "docker-push-malformed1.txt",
			ExpectErr: true,
		},
		{
			Filename:  "docker-push-malformed2.txt",
			ExpectErr: true,
		},
		{
			Filename:  "docker-push-malformed3.txt",
			ExpectErr: true,
		},
		{
			Filename: "empty.txt",
			Expected: "",
		},
	}

	for _, tc := range tests {
		fakeDocker := NewFakeDockerClient()
		fakeDocker.pushImageFunc = func(opts docker.PushImageOptions, auth docker.AuthConfiguration) error {
			if opts.OutputStream == nil {
				return fmt.Errorf("Expected OutputStream != nil")
			}

			fh, err := os.Open(filepath.Join("testdata", tc.Filename))
			if err != nil {
				return fmt.Errorf("Cannot open %q: %s", tc.Filename, err)
			}

			_, err = io.Copy(opts.OutputStream, fh)
			if err != nil {
				return fmt.Errorf("[%s] Failed to process output stream for: %s", tc.Filename, err)
			}

			return nil
		}
		testAuth := docker.AuthConfiguration{}

		digest, err := pushImage(fakeDocker, "testimage/"+tc.Filename, testAuth)
		if err != nil && !tc.ExpectErr {
			t.Errorf("[%s] Unexpected error: %v", tc.Filename, err)
			continue
		} else if err == nil && tc.ExpectErr {
			t.Errorf("[%s] Expected error, got success", tc.Filename)
			continue
		}

		if digest != tc.Expected {
			t.Errorf("[%s] Digest mismatch: expected %q, got %q", tc.Filename, tc.Expected, digest)
			continue
		}
	}

}

func TestSimpleProgress(t *testing.T) {
	tests := []struct {
		Filename  string
		Expected  string
		ExpectErr bool
	}{
		{
			Filename: "docker-push-1.10.txt",
			Expected: "(?ms)The push.*Preparing.*Waiting.*Layer.*Pushing.*Pushed.*digest",
		},
		{
			Filename: "docker-push-1.12.txt",
			Expected: "(?ms)The push.*Preparing.*Pushing.*Pushed.*digest",
		},
		{
			Filename: "docker-push-exists.txt",
			Expected: "(?ms)The push.*Preparing.*Layer.*digest",
		},
		{
			Filename: "docker-push-0digests.txt",
			Expected: "(?ms)The push.*Preparing.*Pushing.*Pushed",
		},
		{
			Filename: "docker-push-2digests.txt",
			Expected: "(?ms)The push.*Preparing.*Pushing.*Pushed.*digest",
		},
		{
			Filename:  "docker-push-malformed1.txt",
			Expected:  "(?ms)The push",
			ExpectErr: true,
		},
		{
			Filename:  "docker-push-malformed2.txt",
			Expected:  "(?ms)The push.*Preparing.*Pushing.*Pushed.*digest",
			ExpectErr: true,
		},
		{
			Filename:  "docker-push-malformed3.txt",
			Expected:  "(?ms)The push.*Preparing.*Pushing.*Pushed.*digest",
			ExpectErr: true,
		},
		{
			Filename: "empty.txt",
			Expected: "^$",
		},
	}

	for _, tc := range tests {
		fh, err := os.Open(filepath.Join("testdata", tc.Filename))
		if err != nil {
			t.Errorf("Cannot open %q: %s", tc.Filename, err)
			continue
		}

		output := &bytes.Buffer{}
		writer := newSimpleWriter(output)

		_, err = io.Copy(writer, fh)
		if err != nil && !tc.ExpectErr {
			t.Errorf("Failed to process %q: %s", tc.Filename, err)
			continue
		} else if err == nil && tc.ExpectErr {
			t.Errorf("Expected error for %q, got success", tc.Filename)
		}

		if outputStr := output.String(); !regexp.MustCompile(tc.Expected).MatchString(outputStr) {
			t.Errorf("%s: expected %q, got:\n%s\n", tc.Filename, tc.Expected, outputStr)
			continue
		}
	}
}

var credsRegex = regexp.MustCompile("user:password")
var redactedRegex = regexp.MustCompile("redacted")

func TestSafeForLoggingDockerCreateOptions(t *testing.T) {
	opts := &docker.CreateContainerOptions{
		Config: &docker.Config{

			Env: []string{
				"http_proxy=http://user:password@proxy.com",
				"ignore=http://user:password@proxy.com",
			},
		},
	}
	stripped := SafeForLoggingDockerCreateOptions(opts)
	if credsRegex.MatchString(stripped.Config.Env[0]) {
		t.Errorf("stripped proxy variable %s should not contain credentials", stripped.Config.Env[0])
	}
	if !redactedRegex.MatchString(stripped.Config.Env[0]) {
		t.Errorf("stripped proxy variable %s should contain redacted", stripped.Config.Env[0])
	}
	if !credsRegex.MatchString(stripped.Config.Env[1]) {
		t.Errorf("stripped other variable %s should contain credentials", stripped.Config.Env[1])
	}
	if redactedRegex.MatchString(stripped.Config.Env[1]) {
		t.Errorf("stripped other variable %s should not contain redacted", stripped.Config.Env[1])
	}

	if !credsRegex.MatchString(opts.Config.Env[0]) {
		t.Errorf("original proxy variable %s should contain credentials", opts.Config.Env[0])
	}
	if redactedRegex.MatchString(opts.Config.Env[0]) {
		t.Errorf("original proxy variable %s should not contain redacted", opts.Config.Env[0])
	}
	if !credsRegex.MatchString(opts.Config.Env[1]) {
		t.Errorf("original other variable %s should contain credentials", opts.Config.Env[1])
	}
	if redactedRegex.MatchString(opts.Config.Env[1]) {
		t.Errorf("original other variable %s should not contain redacted", opts.Config.Env[1])
	}
}
