package builder

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"k8s.io/kubernetes/pkg/util/interrupt"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"

	"github.com/openshift/source-to-image/pkg/tar"

	"github.com/openshift/imagebuilder/imageprogress"
)

var (
	// DefaultPushRetryCount is the number of retries of pushing the built Docker image
	// into a configured repository
	DefaultPushRetryCount = 6
	// DefaultPushRetryDelay is the time to wait before triggering a push retry
	DefaultPushRetryDelay = 5 * time.Second
	// RetriableErrors is a set of strings that indicate that an retriable error occurred.
	RetriableErrors = []string{
		"ping attempt failed with error",
		"is already in progress",
		"connection reset by peer",
		"transport closed before response was received",
	}
)

// DockerClient is an interface to the Docker client that contains
// the methods used by the common builder
type DockerClient interface {
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
	err := client.PullImage(opts, authConfig)
	if err == nil {
		return nil
	}
	return err
}

// pushImage pushes a docker image to the registry specified in its tag.
// The method will retry to push the image when following scenarios occur:
// - Docker registry is down temporarily or permanently
// - other image is being pushed to the registry
// If any other scenario the push will fail, without retries.
func pushImage(client DockerClient, name string, authConfig docker.AuthConfiguration) error {
	repository, tag := docker.ParseRepositoryTag(name)
	logProgress := func(s string) {
		glog.V(0).Infof("%s", s)
	}
	opts := docker.PushImageOptions{
		Name:          repository,
		Tag:           tag,
		OutputStream:  imageprogress.NewPushWriter(logProgress),
		RawJSONStream: true,
	}
	if glog.Is(5) {
		opts.OutputStream = os.Stderr
		opts.RawJSONStream = false
	}
	var err error
	var retriableError = false

	for retries := 0; retries <= DefaultPushRetryCount; retries++ {
		err = client.PushImage(opts, authConfig)
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

		utilruntime.HandleError(fmt.Errorf("push for image %s failed, will retry in %s ...", name, DefaultPushRetryDelay))
		time.Sleep(DefaultPushRetryDelay)
	}
	return err
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
func dockerRun(client DockerClient, createOpts docker.CreateContainerOptions, logsOpts docker.LogsOptions) error {
	// Create a new container.
	glog.V(4).Infof("Creating container with options {Name:%q Config:%+v HostConfig:%+v} ...", createOpts.Name, createOpts.Config, createOpts.HostConfig)
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
		// Start the container.
		glog.V(4).Infof("Starting container %q ...", containerName)
		if err := client.StartContainer(c.ID, nil); err != nil {
			return fmt.Errorf("start container %q: %v", containerName, err)
		}

		// Stream container logs.
		logsOpts.Container = c.ID
		glog.V(4).Infof("Streaming logs of container %q with options %+v ...", containerName, logsOpts)
		if err := client.Logs(logsOpts); err != nil {
			return fmt.Errorf("streaming logs of %q: %v", containerName, err)
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
