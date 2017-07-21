package builder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	dockertypes "github.com/docker/engine-api/types"
	docker "github.com/fsouza/go-dockerclient"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/source-to-image/pkg/tar"
	s2iutil "github.com/openshift/source-to-image/pkg/util"

	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/util/interrupt"

	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerclient"
	"github.com/openshift/imagebuilder/imageprogress"
)

var (
	// DefaultPushOrPullRetryCount is the number of retries of pushing or pulling the built Docker image
	// into a configured repository
	DefaultPushOrPullRetryCount = 6
	// DefaultPushOrPullRetryDelay is the time to wait before triggering a push or pull retry
	DefaultPushOrPullRetryDelay = 5 * time.Second
	// RetriableErrors is a set of strings that indicate that an retriable error occurred.
	RetriableErrors = []string{
		"ping attempt failed with error",
		"is already in progress",
		"connection reset by peer",
		"transport closed before response was received",
		"connection refused",
		"no route to host",
		"unexpected end of JSON input",
		"i/o timeout",
	}
)

// DockerClient is an interface to the Docker client that contains
// the methods used by the common builder
type DockerClient interface {
	AttachToContainerNonBlocking(opts docker.AttachToContainerOptions) (docker.CloseWaiter, error)
	BuildImage(opts docker.BuildImageOptions) error
	PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) error
	RemoveImage(name string) error
	CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error)
	DownloadFromContainer(id string, opts docker.DownloadFromContainerOptions) error
	PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error
	RemoveContainer(opts docker.RemoveContainerOptions) error
	InspectImage(name string) (*docker.Image, error)
	StartContainer(id string, hostConfig *docker.HostConfig) error
	WaitContainer(id string) (int, error)
	Logs(opts docker.LogsOptions) error
	TagImage(name string, opts docker.TagImageOptions) error
}

func RetryImageAction(client DockerClient, opts interface{}, authConfig docker.AuthConfiguration) error {
	var err error
	var retriableError = false
	var actionName string

	pullOpt := docker.PullImageOptions{}
	pushOpt := docker.PushImageOptions{}

	for retries := 0; retries <= DefaultPushOrPullRetryCount; retries++ {
		if reflect.TypeOf(opts) == reflect.TypeOf(pullOpt) {
			actionName = "Pull"
			err = client.PullImage(opts.(docker.PullImageOptions), authConfig)
		} else if reflect.TypeOf(opts) == reflect.TypeOf(pushOpt) {
			actionName = "Push"
			err = client.PushImage(opts.(docker.PushImageOptions), authConfig)
		} else {
			return errors.New("not match Pull or Push action")
		}

		if err == nil {
			return nil
		}

		errMsg := fmt.Sprintf("%s", err)
		for _, errorString := range RetriableErrors {
			if strings.Contains(errMsg, errorString) {
				retriableError = true
				break
			}
		}
		if !retriableError {
			return err
		}

		glog.V(0).Infof("Warning: %s failed, retrying in %s ...", actionName, DefaultPushOrPullRetryDelay)
		time.Sleep(DefaultPushOrPullRetryDelay)
	}

	return fmt.Errorf("After retrying %d times, %s image still failed", DefaultPushOrPullRetryCount, actionName)
}

func pullImage(client DockerClient, name string, authConfig docker.AuthConfiguration) error {
	logProgress := func(s string) {
		glog.V(0).Infof("%s", s)
	}
	opts := docker.PullImageOptions{
		Repository:    name,
		OutputStream:  imageprogress.NewPullWriter(logProgress),
		RawJSONStream: true,
	}
	if glog.Is(5) {
		opts.OutputStream = os.Stderr
		opts.RawJSONStream = false
	}
	return RetryImageAction(client, opts, authConfig)
}

// pushImage pushes a docker image to the registry specified in its tag.
// The method will retry to push the image when following scenarios occur:
// - Docker registry is down temporarily or permanently
// - other image is being pushed to the registry
// If any other scenario the push will fail, without retries.
//
// Returns the digest of the docker image in the registry, or empty string in
// case registry didn't send it or we failed to extract it.
func pushImage(client DockerClient, name string, authConfig docker.AuthConfiguration) (string, error) {
	repository, tag := docker.ParseRepositoryTag(name)

	var progressWriter io.Writer
	if glog.Is(5) {
		progressWriter = newSimpleWriter(os.Stderr)
	} else {
		logProgress := func(s string) {
			glog.V(0).Infof("%s", s)
		}
		progressWriter = imageprogress.NewPushWriter(logProgress)
	}
	digestWriter := newDigestWriter()

	opts := docker.PushImageOptions{
		Name:          repository,
		Tag:           tag,
		OutputStream:  io.MultiWriter(progressWriter, digestWriter),
		RawJSONStream: true,
	}

	if err := RetryImageAction(client, opts, authConfig); err != nil {
		return "", err
	}
	return digestWriter.Digest, nil
}

func removeImage(client DockerClient, name string) error {
	return client.RemoveImage(name)
}

// buildImage invokes a docker build on a particular directory
func buildImage(client DockerClient, dir string, tar tar.Tar, opts *docker.BuildImageOptions) error {
	// TODO: be able to pass a stream directly to the Docker build to avoid the double temp hit
	if opts == nil {
		return fmt.Errorf("%s", "build image options nil")
	}
	r, w := io.Pipe()
	go func() {
		defer utilruntime.HandleCrash()
		defer w.Close()
		if err := tar.CreateTarStream(dir, false, w); err != nil {
			w.CloseWithError(err)
		}
	}()
	defer w.Close()
	opts.InputStream = r
	glog.V(5).Infof("Invoking Docker build to create %q", opts.Name)
	return client.BuildImage(*opts)
}

// buildDirectImage invokes a docker build on a particular directory using imagebuilder
func buildDirectImage(dir string, ignoreFailures bool, opts *docker.BuildImageOptions) error {
	glog.V(5).Infof("Invoking imagebuilder to create %q in dir %s with Dockerfile %s", opts.Name, dir, opts.Dockerfile)

	e := dockerclient.NewClientExecutor(nil)
	e.Directory = dir
	e.Tag = opts.Name

	e.IgnoreUnrecognizedInstructions = ignoreFailures
	e.StrictVolumeOwnership = !ignoreFailures
	e.HostConfig = &docker.HostConfig{
		NetworkMode: opts.NetworkMode,
		CPUShares:   opts.CPUShares,
		CPUPeriod:   opts.CPUPeriod,
		CPUSetCPUs:  opts.CPUSetCPUs,
		CPUQuota:    opts.CPUQuota,
		Memory:      opts.Memory,
		MemorySwap:  opts.Memswap,
	}

	e.Out, e.ErrOut = opts.OutputStream, opts.OutputStream

	// use a keyring
	keys := make(credentialprovider.DockerConfig)
	for k, v := range opts.AuthConfigs.Configs {
		keys[k] = credentialprovider.DockerConfigEntry{
			Username: v.Username,
			Password: v.Password,
			Email:    v.Email,
		}
	}
	keyring := credentialprovider.BasicDockerKeyring{}
	keyring.Add(keys)
	e.AuthFn = func(name string) ([]dockertypes.AuthConfig, bool) {
		authConfs, found := keyring.Lookup(name)
		var out []dockertypes.AuthConfig
		for _, conf := range authConfs {
			out = append(out, conf.AuthConfig)
		}
		return out, found
	}

	e.LogFn = func(format string, args ...interface{}) {
		if glog.Is(3) {
			glog.Infof("Builder: "+format, args...)
		} else {
			fmt.Fprintf(e.ErrOut, "--> %s\n", fmt.Sprintf(format, args...))
		}
	}

	arguments := make(map[string]string)
	for _, arg := range opts.BuildArgs {
		arguments[arg.Name] = arg.Value
	}

	if err := e.DefaultExcludes(); err != nil {
		return fmt.Errorf("error: Could not parse default .dockerignore: %v", err)
	}

	client, err := dockerclient.NewClientFromEnv()
	if err != nil {
		return fmt.Errorf("error: No connection to Docker available: %v", err)
	}
	e.Client = client

	releaseFn := func() {
		for _, err := range e.Release() {
			fmt.Fprintf(e.ErrOut, "error: Unable to clean up build: %v\n", err)
		}
	}
	return interrupt.New(nil, releaseFn).Run(func() error {
		b, node, err := imagebuilder.NewBuilderForFile(filepath.Join(dir, opts.Dockerfile), arguments)
		if err != nil {
			return err
		}
		if err := e.Prepare(b, node); err != nil {
			return err
		}
		if err := e.Execute(b, node); err != nil {
			return err
		}
		return e.Commit(b)
	})
}

// tagImage uses the dockerClient to tag a Docker image with name. It is a
// helper to facilitate the usage of dockerClient.TagImage, because the former
// requires the name to be split into more explicit parts.
func tagImage(dockerClient DockerClient, image, name string) error {
	repo, tag := docker.ParseRepositoryTag(name)
	return dockerClient.TagImage(image, docker.TagImageOptions{
		Repo: repo,
		Tag:  tag,
		// We need to set Force to true to update the tag even if it
		// already exists. This is the same behavior as `docker build -t
		// tag .`.
		Force: true,
	})
}

// dockerRun mimics the 'docker run --rm' CLI command. It uses the Docker Remote
// API to create and start a container and stream its logs. The container is
// removed after it terminates.
func dockerRun(client DockerClient, createOpts docker.CreateContainerOptions, attachOpts docker.AttachToContainerOptions) error {
	// Create a new container.
	// First strip any inlined proxy credentials from the *proxy* env variables,
	// before logging the env variables.
	if glog.Is(4) {
		redactedOpts := SafeForLoggingDockerCreateOptions(&createOpts)
		glog.V(4).Infof("Creating container with options {Name:%q Config:%+v HostConfig:%+v} ...", redactedOpts.Name, redactedOpts.Config, redactedOpts.HostConfig)
	}
	c, err := client.CreateContainer(createOpts)
	if err != nil {
		return fmt.Errorf("create container %q: %v", createOpts.Name, err)
	}

	containerName := getContainerNameOrID(c)

	removeContainer := func() {
		glog.V(4).Infof("Removing container %q ...", containerName)
		if err := client.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID}); err != nil {
			glog.V(0).Infof("warning: Failed to remove container %q: %v", containerName, err)
		} else {
			glog.V(4).Infof("Removed container %q", containerName)
		}
	}
	startWaitContainer := func() error {
		// Changed to use attach call instead of logs call to stream stdout/stderr
		// during execution to avoid race condition
		// https://github.com/docker/docker/issues/31323 .
		// Using attach call is also racy in docker versions which don't carry
		// https://github.com/docker/docker/pull/30446 .
		// In RHEL, docker >= 1.12.6-10.el7.x86_64 should be OK.

		// Attach to the container.
		success := make(chan struct{})
		attachOpts.Container = c.ID
		attachOpts.Success = success
		glog.V(4).Infof("Attaching to container %q with options %+v ...", containerName, attachOpts)
		wc, err := client.AttachToContainerNonBlocking(attachOpts)
		if err != nil {
			return fmt.Errorf("attach container %q: %v", containerName, err)
		}
		defer wc.Close()

		select {
		case <-success:
			close(success)
		case <-time.After(120 * time.Second):
			return fmt.Errorf("attach container %q: timeout waiting for success signal", containerName)
		}

		// Start the container.
		glog.V(4).Infof("Starting container %q ...", containerName)
		if err := client.StartContainer(c.ID, nil); err != nil {
			return fmt.Errorf("start container %q: %v", containerName, err)
		}

		// Wait for streaming to finish.
		glog.V(4).Infof("Waiting for streaming to finish ...")
		if err := wc.Wait(); err != nil {
			return fmt.Errorf("container %q streaming: %v", containerName, err)
		}

		// Return an error if the exit code of the container is non-zero.
		glog.V(4).Infof("Waiting for container %q to stop ...", containerName)
		exitCode, err := client.WaitContainer(c.ID)
		if err != nil {
			return fmt.Errorf("waiting for container %q to stop: %v", containerName, err)
		}
		if exitCode != 0 {
			return fmt.Errorf("container %q returned non-zero exit code: %d", containerName, exitCode)
		}
		return nil
	}
	// the interrupt handler acts as a super-defer which will guarantee removeContainer is executed
	// either when startWaitContainer finishes, or when a SIGQUIT/SIGINT/SIGTERM is received.
	return interrupt.New(nil, removeContainer).Run(startWaitContainer)
}

func getContainerNameOrID(c *docker.Container) string {
	if c.Name != "" {
		return c.Name
	}
	return c.ID
}

type progressLine struct {
	Status   string      `json:"status,omitempty"`
	Progress string      `json:"progress,omitempty"`
	Error    string      `json:"error,omitempty"`
	Stream   string      `json:"stream,omitempty"`
	Aux      progressAux `json:"aux,omitempty"`
}

type progressAux struct {
	Tag    string `json:"Tag"`
	Digest string `json:"Digest"`
	Size   int64  `json:"Size"`
}

type pushWriterCallback func(progressLine) error

// pushWriter is an io.Writer which consumes a stream of json messages returned
// by docker client when it pushes image to registry. It calls the provided
// callback for each decoded JSON object.
type pushWriter struct {
	buf      *bytes.Buffer
	callback pushWriterCallback
}

func newPushWriter(cb pushWriterCallback) *pushWriter {
	return &pushWriter{
		buf:      &bytes.Buffer{},
		callback: cb,
	}
}

func (t *pushWriter) Write(data []byte) (int, error) {
	n, err := t.buf.Write(data)
	if err != nil {
		return n, err
	}
	dec := json.NewDecoder(t.buf)

	for {
		// save the not yet parsed input so we can restore it in case it
		// contains part of valid JSON
		savedBuf, err := ioutil.ReadAll(dec.Buffered())
		if err != nil {
			return n, err
		}
		savedBuf = append(savedBuf, t.buf.Bytes()...)

		// try decoding a value
		line := &progressLine{}
		err = dec.Decode(line)

		switch err {
		// decoded a value, pass it to callback
		case nil:
			if callbackErr := t.callback(*line); callbackErr != nil {
				return n, callbackErr
			}
		// no more values
		case io.EOF:
			return n, nil
		// there's no whole JSON but we consumed bytes that might be part of
		// one - restore the saved buffer
		case io.ErrUnexpectedEOF:
			t.buf = bytes.NewBuffer(savedBuf)
			return n, nil
		// actual error happened
		default:
			return n, err
		}
	}
}

// newSimpleWriter creates an io.Writer which consumes a stream of json
// messages returned by docker client when it pushes image to registry. It
// writes simple human-readable indication of the push progress to the output
// io.Writer. The output format mimics what go-dockerclient writes when called
// with RawJSONStream=false.
func newSimpleWriter(output io.Writer) io.Writer {
	return newPushWriter(func(line progressLine) error {
		if len(line.Stream) > 0 {
			fmt.Fprint(output, line.Stream)
		} else if len(line.Progress) > 0 {
			fmt.Fprintf(output, "%s %s\r", line.Status, line.Progress)
		} else if len(line.Error) > 0 {
			return errors.New(line.Error)
		}
		if len(line.Status) > 0 {
			fmt.Fprintln(output, line.Status)
		}
		return nil
	})
}

// digestWriter consumes stream of json messages from docker client push
// operation and looks for digest of the pushed image.
type digestWriter struct {
	*pushWriter
	Digest string
}

func newDigestWriter() *digestWriter {
	dw := digestWriter{}
	dw.pushWriter = newPushWriter(func(line progressLine) error {
		if len(line.Error) > 0 {
			return errors.New(line.Error)
		}
		if len(dw.Digest) == 0 && len(line.Aux.Digest) > 0 {
			dw.Digest = line.Aux.Digest
		}
		return nil
	})
	return &dw
}

// GetDockerClient returns a valid Docker client, the address of the client, or an error
// if the client couldn't be created.
func GetDockerClient() (client *docker.Client, endpoint string, err error) {
	client, err = docker.NewClientFromEnv()
	if len(os.Getenv("DOCKER_HOST")) > 0 {
		endpoint = os.Getenv("DOCKER_HOST")
	} else {
		endpoint = "unix:///var/run/docker.sock"
	}
	return
}

// SafeForLoggingDockerConfig returns a copy of a docker config struct
// where any proxy credentials in the env section of the config
// have been redacted.
func SafeForLoggingDockerConfig(config *docker.Config) *docker.Config {
	origEnv := config.Env
	newConfig := *config
	newConfig.Env = s2iutil.SafeForLoggingEnv(origEnv)
	return &newConfig
}

// SafeForLoggingDockerCreateOptions returns a copy of a docker
// create container options struct where any proxy credentials in the env section of
// the config have been redacted.
func SafeForLoggingDockerCreateOptions(opts *docker.CreateContainerOptions) *docker.CreateContainerOptions {
	origConfig := opts.Config
	newOpts := *opts
	newOpts.Config = SafeForLoggingDockerConfig(origConfig)
	return &newOpts
}
