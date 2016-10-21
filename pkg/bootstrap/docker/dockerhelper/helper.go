package dockerhelper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/blang/semver"
	dockerclient "github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types/registry"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/openshift/imagebuilder/imageprogress"
	starterrors "github.com/openshift/origin/pkg/bootstrap/docker/errors"
)

const openShiftInsecureCIDR = "172.30.0.0/16"

// Helper provides utility functions to help with Docker
type Helper struct {
	client          *docker.Client
	engineAPIClient *dockerclient.Client
}

// NewHelper creates a new Helper
func NewHelper(client *docker.Client, engineAPIClient *dockerclient.Client) *Helper {
	return &Helper{
		client:          client,
		engineAPIClient: engineAPIClient,
	}
}

type RegistryConfig struct {
	InsecureRegistryCIDRs []string
}

func hasCIDR(cidr string, listOfCIDRs []*registry.NetIPNet) bool {
	glog.V(5).Infof("Looking for %q in %#v", cidr, listOfCIDRs)
	for _, candidate := range listOfCIDRs {
		candidateStr := (*net.IPNet)(candidate).String()
		if candidateStr == cidr {
			glog.V(5).Infof("Found %q", cidr)
			return true
		}
	}
	glog.V(5).Infof("Did not find %q", cidr)
	return false
}

// HasInsecureRegistryArg checks whether the docker daemon is configured with
// the appropriate insecure registry argument
func (h *Helper) HasInsecureRegistryArg() (bool, error) {
	glog.V(5).Infof("Retrieving Docker daemon info")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if h.engineAPIClient == nil {
		return false, fmt.Errorf("the Docker engine API client is not initialized")
	}
	info, err := h.engineAPIClient.Info(ctx)
	defer cancel()
	if err != nil {
		glog.V(2).Infof("Could not retrieve Docker info: %v", err)
		return false, err
	}
	glog.V(5).Infof("Docker daemon info: %#v", info)
	registryConfig := info.RegistryConfig
	if err != nil {
		return false, err
	}
	return hasCIDR(openShiftInsecureCIDR, registryConfig.InsecureRegistryCIDRs), nil
}

var (
	fedoraPackage = regexp.MustCompile("\\.fc[0-9_]*\\.")
	rhelPackage   = regexp.MustCompile("\\.el[0-9_]*\\.")
)

// Version returns the Docker version and whether it is a Red Hat distro version
func (h *Helper) Version() (*semver.Version, bool, error) {
	glog.V(5).Infof("Retrieving Docker version")
	env, err := h.client.Version()
	if err != nil {
		glog.V(2).Infof("Error retrieving version: %v", err)
		return nil, false, err
	}
	glog.V(5).Infof("Docker version results: %#v", env)
	versionStr := env.Get("Version")
	if len(versionStr) == 0 {
		return nil, false, errors.New("did not get a version")
	}
	glog.V(5).Infof("Version: %s", versionStr)
	dockerVersion, err := semver.Parse(versionStr)
	if err != nil {
		glog.V(2).Infof("Error parsing Docker version %q", versionStr)
		return nil, false, err
	}
	isRedHat := false
	packageVersion := env.Get("PkgVersion")
	if len(packageVersion) > 0 {
		isRedHat = fedoraPackage.MatchString(packageVersion) || rhelPackage.MatchString(packageVersion)
	}
	return &dockerVersion, isRedHat, nil
}

// CheckAndPull checks whether a Docker image exists. If not, it pulls it.
func (h *Helper) CheckAndPull(image string, out io.Writer) error {
	glog.V(5).Infof("Inspecting Docker image %q", image)
	imageMeta, err := h.client.InspectImage(image)
	if err == nil {
		glog.V(5).Infof("Image %q found: %#v", image, imageMeta)
		return nil
	}
	if err != docker.ErrNoSuchImage {
		return starterrors.NewError("unexpected error inspecting image %s", image).WithCause(err)
	}
	glog.V(5).Infof("Image %q not found. Pulling", image)
	fmt.Fprintf(out, "Pulling image %s\n", image)
	logProgress := func(s string) {
		fmt.Fprintf(out, "%s\n", s)
	}
	outputStream := imageprogress.NewPullWriter(logProgress)
	if glog.V(5) {
		outputStream = out
	}
	err = h.client.PullImage(docker.PullImageOptions{
		Repository:    image,
		RawJSONStream: bool(!glog.V(5)),
		OutputStream:  outputStream,
	}, docker.AuthConfiguration{})
	if err != nil {
		return starterrors.NewError("error pulling Docker image %s", image).WithCause(err)
	}
	fmt.Fprintf(out, "Image pull complete\n")
	return nil
}

// GetContainerState returns whether a container exists and if it does whether it's running
func (h *Helper) GetContainerState(id string) (container *docker.Container, running bool, err error) {
	glog.V(5).Infof("Inspecting docker container %q", id)
	container, err = h.client.InspectContainer(id)
	if err != nil {
		if _, notFound := err.(*docker.NoSuchContainer); notFound {
			glog.V(5).Infof("Container %q was not found", id)
			err = nil
			return
		}
		glog.V(5).Infof("An error occurred inspecting container %q: %v", id, err)
		return
	}
	running = container.State.Running
	glog.V(5).Infof("Container inspect result: %#v", container)
	glog.V(5).Infof("Container running = %v", running)
	return
}

// RemoveContainer removes the container with the given id
func (h *Helper) RemoveContainer(id string) error {
	glog.V(5).Infof("Removing container %q", id)
	err := h.client.RemoveContainer(docker.RemoveContainerOptions{
		ID: id,
	})
	if err != nil {
		return starterrors.NewError("cannot delete container %s", id).WithCause(err)
	}
	glog.V(5).Infof("Removed container %q", id)
	return nil
}

// HostIP returns the IP of the Docker host if connecting via TCP
func (h *Helper) HostIP() string {
	// By default, if the Docker client uses tcp, then use the Docker daemon's address
	endpoint := h.client.Endpoint()
	u, err := url.Parse(endpoint)
	if err == nil && (u.Scheme == "tcp" || u.Scheme == "http" || u.Scheme == "https") {
		glog.V(2).Infof("Using the Docker host %s for the server IP", endpoint)
		if host, _, serr := net.SplitHostPort(u.Host); serr == nil {
			return host
		}
		return u.Host
	}
	glog.V(5).Infof("Cannot use Docker endpoint (%s) because it is not using one of the following protocols: tcp, http, https", endpoint)
	return ""
}

func (h *Helper) ContainerLog(container string, numLines int) string {
	output := &bytes.Buffer{}
	err := h.client.Logs(docker.LogsOptions{
		Container:    container,
		Tail:         strconv.Itoa(numLines),
		OutputStream: output,
		ErrorStream:  output,
		Stdout:       true,
		Stderr:       true,
	})
	if err != nil {
		glog.V(1).Infof("Error getting container %q log: %v", container, err)
	}
	return output.String()
}

func (h *Helper) StopAndRemoveContainer(container string) error {
	err := h.client.StopContainer(container, 10)
	if err != nil {
		glog.V(2).Infof("Cannot stop container %s: %v", container, err)
	}
	return h.RemoveContainer(container)
}

func (h *Helper) ListContainerNames() ([]string, error) {
	containers, err := h.client.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, c := range containers {
		if len(c.Names) > 0 {
			names = append(names, c.Names[0])
		}
	}
	return names, nil
}
