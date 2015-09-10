package builder

import (
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/util"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/tar"
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
	PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error
}

// pushImage pushes a docker image to the registry specified in its tag.
// The method will retry to push the image when following scenarios occur:
// - Docker registry is down temporarily or permanently
// - other image is being pushed to the registry
// If any other scenario the push will fail, without retries.
func pushImage(client DockerClient, name string, authConfig docker.AuthConfiguration) error {
	repository, tag := docker.ParseRepositoryTag(name)
	opts := docker.PushImageOptions{
		Name: repository,
		Tag:  tag,
	}
	if glog.V(5) {
		opts.OutputStream = os.Stderr
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

		util.HandleError(fmt.Errorf("push for image %s failed, will retry in %s seconds ...", name, DefaultPushRetryDelay))
		glog.Flush()
		time.Sleep(DefaultPushRetryDelay)
	}
	return err
}

func removeImage(client DockerClient, name string) error {
	return client.RemoveImage(name)
}

// buildImage invokes a docker build on a particular directory
func buildImage(client DockerClient, dir string, noCache bool, tag string, tar tar.Tar, pullAuth *docker.AuthConfigurations, forcePull bool) error {
	tarFile, err := tar.CreateTarFile("", dir)
	if err != nil {
		return err
	}
	tarStream, err := os.Open(tarFile)
	if err != nil {
		return err
	}
	defer tarStream.Close()
	opts := docker.BuildImageOptions{
		Name:           tag,
		RmTmpContainer: true,
		OutputStream:   os.Stdout,
		InputStream:    tarStream,
		NoCache:        noCache,
		Pull:           forcePull,
	}
	if pullAuth != nil {
		opts.AuthConfigs = *pullAuth
	}
	return client.BuildImage(opts)
}
