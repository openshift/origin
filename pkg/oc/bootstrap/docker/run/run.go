package run

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

type RunHelper struct {
	client       dockerhelper.Interface
	dockerHelper *dockerhelper.Helper
}

func NewRunHelper(dockerHelper *dockerhelper.Helper) *RunHelper {
	return &RunHelper{
		client:       dockerHelper.Client(),
		dockerHelper: dockerHelper,
	}
}

func (h *RunHelper) New() *Runner {
	return &Runner{
		client:       h.client,
		dockerHelper: h.dockerHelper,
		config:       &container.Config{},
		hostConfig:   &container.HostConfig{},
	}
}

// Runner is a helper to run new containers on Docker
type Runner struct {
	name            string
	client          dockerhelper.Interface
	dockerHelper    *dockerhelper.Helper
	config          *container.Config
	hostConfig      *container.HostConfig
	removeContainer bool
	copies          map[string][]byte
}

// Name sets the name of the container to create
func (h *Runner) Name(name string) *Runner {
	h.name = name
	return h
}

// Image sets the image to run
func (h *Runner) Image(image string) *Runner {
	h.config.Image = image
	return h
}

func (h *Runner) PortForward(local, remote int) *Runner {
	if h.hostConfig.PortBindings == nil {
		h.hostConfig.PortBindings = nat.PortMap{}
	}
	containerPort := nat.Port(fmt.Sprintf("%d/tcp", remote))
	binding := nat.PortBinding{
		HostPort: fmt.Sprintf("%d", local),
	}
	h.hostConfig.PortBindings[containerPort] = []nat.PortBinding{binding}

	if h.config.ExposedPorts == nil {
		h.config.ExposedPorts = map[nat.Port]struct{}{}
	}
	h.config.ExposedPorts[containerPort] = struct{}{}
	return h
}

// Entrypoint sets the entrypoint to use when running
func (h *Runner) Entrypoint(cmd ...string) *Runner {
	h.config.Entrypoint = cmd
	return h
}

// Command sets the command to run
func (h *Runner) Command(cmd ...string) *Runner {
	h.config.Cmd = cmd
	return h
}

func (h *Runner) Copy(contents map[string][]byte) *Runner {
	h.copies = contents
	return h
}

// HostPid tells Docker to run using the host's pid namespace
func (h *Runner) HostPid() *Runner {
	h.hostConfig.PidMode = "host"
	return h
}

// HostNetwork tells Docker to run using the host's Network namespace
func (h *Runner) HostNetwork() *Runner {
	h.hostConfig.NetworkMode = "host"
	return h
}

// Bind tells Docker to bind host dirs to container dirs
func (h *Runner) Bind(binds ...string) *Runner {
	h.hostConfig.Binds = append(h.hostConfig.Binds, binds...)
	return h
}

// Env tells Docker to add environment variables to the container getting started
func (h *Runner) Env(env ...string) *Runner {
	h.config.Env = append(h.config.Env, env...)
	return h
}

// Privileged tells Docker to run the container as privileged
func (h *Runner) Privileged() *Runner {
	h.hostConfig.Privileged = true
	return h
}

// DiscardContainer if true will cause the container to be removed when done executing.
// Will be ignored in the case of Start
func (h *Runner) DiscardContainer() *Runner {
	h.removeContainer = true
	return h
}

// User sets the username or UID to use when running the container.
// Will be ignored if empty string
func (h *Runner) User(user string) *Runner {
	if strings.TrimSpace(user) != "" {
		h.config.User = user
	}
	return h
}

func (h *Runner) DNS(address ...string) *Runner {
	h.hostConfig.DNS = address
	return h
}

// Start starts the container as a daemon and returns
func (h *Runner) Start() (string, error) {
	id, err := h.Create()
	if err != nil {
		return "", err
	}
	if err := h.copy(id); err != nil {
		return id, err
	}
	return id, h.startContainer(id)
}

// Output starts the container, waits for it to finish and returns its output
func (h *Runner) Output() (string, string, string, int, error) {
	return h.runWithOutput()
}

// Run executes the container and waits until it completes
func (h *Runner) Run() (string, int, error) {
	containerId, _, _, rc, err := h.runWithOutput()
	return containerId, rc, err
}

func (h *Runner) Create() (string, error) {
	if h.hostConfig.Privileged {
		userNsMode, err := h.dockerHelper.UserNamespaceEnabled()
		if err != nil {
			return "", err
		}
		if userNsMode {
			h.hostConfig.UsernsMode = "host"
		}
	}
	glog.V(4).Infof("Creating container named %q\nconfig:\n%s\nhost config:\n%s\n", h.name, printConfig(h.config), printHostConfig(h.hostConfig))
	response, err := h.client.ContainerCreate(h.config, h.hostConfig, nil, h.name)
	if err != nil {
		return "", errors.NewError("cannot create container using image %s", h.config.Image).WithCause(err)
	}
	glog.V(5).Infof("Container created with id %q", response.ID)
	if len(response.Warnings) > 0 {
		glog.V(5).Infof("Warnings from container creation:  %v", response.Warnings)
	}
	return response.ID, nil
}

func (h *Runner) copy(id string) error {
	if len(h.copies) == 0 {
		return nil
	}
	archive := streamingArchive(h.copies)
	defer archive.Close()
	err := h.client.CopyToContainer(id, "/", archive, types.CopyToContainerOptions{})
	return err
}

// streamingArchive returns a ReadCloser containing a tar archive with contents serialized as files.
func streamingArchive(contents map[string][]byte) io.ReadCloser {
	r, w := io.Pipe()
	go func() {
		archive := tar.NewWriter(w)
		for k, v := range contents {
			if err := archive.WriteHeader(&tar.Header{
				Name:     k,
				Mode:     0644,
				Size:     int64(len(v)),
				Typeflag: tar.TypeReg,
			}); err != nil {
				w.CloseWithError(err)
				return
			}
			if _, err := archive.Write(v); err != nil {
				w.CloseWithError(err)
				return
			}
		}
		archive.Close()
		w.Close()
	}()
	return r
}

func (h *Runner) startContainer(id string) error {
	err := h.client.ContainerStart(id)
	if err != nil {
		return errors.NewError("cannot start container %s", id).WithCause(err)
	}
	return nil
}

func (h *Runner) runWithOutput() (string, string, string, int, error) {
	id, err := h.Create()
	if err != nil {
		return "", "", "", 0, err
	}
	if h.removeContainer {
		defer func() {
			glog.V(5).Infof("Deleting container %q", id)
			if err = h.client.ContainerRemove(id, types.ContainerRemoveOptions{}); err != nil {
				glog.V(2).Infof("Error deleting container %q: %v", id, err)
			}
		}()
	}

	if err := h.copy(id); err != nil {
		return id, "", "", 0, err
	}

	glog.V(5).Infof("Starting container %q", id)
	err = h.startContainer(id)
	if err != nil {
		glog.V(2).Infof("Error occurred starting container %q: %v", id, err)
		return id, "", "", 0, err
	}

	glog.V(5).Infof("Waiting for container %q", id)
	rc, err := h.client.ContainerWait(id)
	if err != nil {
		glog.V(2).Infof("Error occurred waiting for container %q: %v", id, err)
		return id, "", "", 0, err
	}
	glog.V(5).Infof("Done waiting for container %q, rc=%d", id, rc)

	// changed to only reading logs after execution instead of streaming
	// stdout/stderr to avoid race condition in (at least) docker 1.10-1.14-dev:
	// https://github.com/docker/docker/issues/29285
	glog.V(5).Infof("Reading logs from container %q", id)
	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	err = h.client.ContainerLogs(id, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true}, stdOut, stdErr)
	if err != nil {
		glog.V(2).Infof("Error occurred while reading logs: %v", err)
		return id, "", "", 0, err
	}
	glog.V(5).Infof("Done reading logs from container %q", id)

	glog.V(5).Infof("Stdout:\n%s", stdOut.String())
	glog.V(5).Infof("Stderr:\n%s", stdErr.String())
	if rc != 0 || err != nil {
		return id, stdOut.String(), stdErr.String(), rc, newRunError(rc, err, stdOut.String(), stdErr.String(), h.config)
	}
	glog.V(4).Infof("Container run successful\n")
	return id, stdOut.String(), stdErr.String(), rc, nil
}

// printConfig prints out the relevant parts of a container's Docker config
func printConfig(c *container.Config) string {
	out := &bytes.Buffer{}
	fmt.Fprintf(out, "  image: %s\n", c.Image)
	if len(c.Entrypoint) > 0 {
		fmt.Fprintln(out, "  entry point:")
		for _, e := range c.Entrypoint {
			fmt.Fprintf(out, "    %s\n", e)
		}
	}
	if len(c.Cmd) > 0 {
		fmt.Fprintln(out, "  command:")
		for _, c := range c.Cmd {
			fmt.Fprintf(out, "    %s\n", c)
		}
	}
	if len(c.Env) > 0 {
		fmt.Fprintln(out, "  environment:")
		for _, e := range c.Env {
			fmt.Fprintf(out, "    %s\n", e)
		}
	}
	return out.String()
}

func printHostConfig(c *container.HostConfig) string {
	out := &bytes.Buffer{}
	fmt.Fprintf(out, "  pid mode: %s\n", c.PidMode)
	fmt.Fprintf(out, "  user mode: %s\n", c.UsernsMode)
	fmt.Fprintf(out, "  network mode: %s\n", c.NetworkMode)
	if len(c.DNS) > 0 {
		fmt.Fprintln(out, "  DNS:")
		for _, h := range c.DNS {
			fmt.Fprintf(out, "    %s\n", h)
		}
	}
	if len(c.Binds) > 0 {
		fmt.Fprintln(out, "  volume binds:")
		for _, b := range c.Binds {
			fmt.Fprintf(out, "    %s\n", b)
		}
	}
	return out.String()
}
