package run

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"sync"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
)

type RunHelper struct {
	client *docker.Client
}

func NewRunHelper(client *docker.Client) *RunHelper {
	return &RunHelper{
		client: client,
	}
}

func (h *RunHelper) New() *Runner {
	return &Runner{
		client:     h.client,
		config:     &docker.Config{},
		hostConfig: &docker.HostConfig{},
	}
}

// Runner is a helper to run new containers on Docker
type Runner struct {
	name            string
	client          *docker.Client
	config          *docker.Config
	hostConfig      *docker.HostConfig
	input           io.Reader
	removeContainer bool
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
		h.hostConfig.PortBindings = map[docker.Port][]docker.PortBinding{}
	}
	containerPort := docker.Port(fmt.Sprintf("%d/tcp", remote))
	binding := docker.PortBinding{
		HostPort: fmt.Sprintf("%d", local),
	}
	h.hostConfig.PortBindings[containerPort] = []docker.PortBinding{binding}

	if h.config.ExposedPorts == nil {
		h.config.ExposedPorts = map[docker.Port]struct{}{}
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

// Input sets an input stream for the Docker run command
func (h *Runner) Input(reader io.Reader) *Runner {
	h.config.OpenStdin = true
	h.config.StdinOnce = true
	h.input = reader
	return h
}

// Start starts the container as a daemon and returns
func (h *Runner) Start() (string, error) {
	id, err := h.Create()
	if err != nil {
		return "", err
	}
	return id, h.startContainer(id)
}

// Output starts the container, waits for it to finish and returns its output
func (h *Runner) Output() (string, string, int, error) {
	stdOut, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	rc, err := h.runWithOutput(h.input, stdOut, errOut)
	return stdOut.String(), errOut.String(), rc, err
}

// CombinedOutput  is the same as Output, except both output and error streams
// are combined into one.
func (h *Runner) CombinedOutput() (string, int, error) {
	out := &bytes.Buffer{}
	rc, err := h.runWithOutput(h.input, out, out)
	return out.String(), rc, err
}

// Run executes the container and waits until it completes
func (h *Runner) Run() (int, error) {
	return h.runWithOutput(h.input, ioutil.Discard, ioutil.Discard)
}

func (h *Runner) Create() (string, error) {
	glog.V(4).Infof("Creating container named %q\nconfig:\n%s\nhost config:\n%s\n", h.name, printConfig(h.config), printHostConfig(h.hostConfig))
	container, err := h.client.CreateContainer(docker.CreateContainerOptions{
		Name:       h.name,
		Config:     h.config,
		HostConfig: h.hostConfig,
	})
	if err != nil {
		return "", errors.NewError("cannot create container using image %s", h.config.Image).WithCause(err)
	}
	glog.V(5).Infof("Container created with id %q", container.ID)
	glog.V(5).Infof("Container: %#v", container)
	return container.ID, nil
}

func (h *Runner) startContainer(id string) error {
	err := h.client.StartContainer(id, nil)
	if err != nil {
		return errors.NewError("cannot start container %s", id).WithCause(err)
	}
	return nil
}

func (h *Runner) runWithOutput(stdIn io.Reader, stdOut, stdErr io.Writer) (int, error) {
	id, err := h.Create()
	if err != nil {
		return 0, err
	}
	if h.removeContainer {
		defer func() {
			glog.V(5).Infof("Deleting container %q", id)
			if err = h.client.RemoveContainer(docker.RemoveContainerOptions{ID: id}); err != nil {
				glog.V(1).Infof("Error deleting container %q: %v", id, err)
			}
		}()
	}
	logOut, logErr := &bytes.Buffer{}, &bytes.Buffer{}
	outStream := io.MultiWriter(stdOut, logOut)
	errStream := io.MultiWriter(stdErr, logErr)
	attached := make(chan struct{})
	attachErr := make(chan error)
	streamingWG := &sync.WaitGroup{}
	streamingWG.Add(1)
	go func() {
		glog.V(5).Infof("Attaching to container %q", id)
		err = h.client.AttachToContainer(docker.AttachToContainerOptions{
			Container:    id,
			Logs:         true,
			Stream:       true,
			Stdout:       true,
			Stderr:       true,
			Stdin:        stdIn != nil,
			OutputStream: outStream,
			ErrorStream:  errStream,
			InputStream:  stdIn,
			Success:      attached,
		})
		if err != nil {
			glog.V(2).Infof("Error occurred while attaching: %v", err)
			attachErr <- err
		}
		streamingWG.Done()
		glog.V(5).Infof("Done attaching to container %q", id)
	}()

	select {
	case <-attached:
		glog.V(5).Infof("Attach is successful.")
	case err = <-attachErr:
		return 0, err
	}
	glog.V(5).Infof("Starting container %q", id)
	err = h.startContainer(id)
	if err != nil {
		glog.V(2).Infof("Error occurred starting container %q: %v", id, err)
		return 0, err
	}
	glog.V(5).Infof("signaling attached channel")
	attached <- struct{}{}
	glog.V(5).Infof("Waiting for container %q", id)
	rc, err := h.client.WaitContainer(id)
	if err != nil {
		glog.V(2).Infof("Error occurred waiting for container %q: %v, rc = %d", id, err, rc)
	}
	glog.V(5).Infof("Done waiting for container %q, rc=%d. Waiting for attach routine", id, rc)
	streamingWG.Wait()
	glog.V(5).Infof("Done waiting for attach routine")
	glog.V(5).Infof("Stdout:\n%s", logOut.String())
	glog.V(5).Infof("Stderr:\n%s", logErr.String())
	if rc != 0 || err != nil {
		return rc, newRunError(rc, err, logOut.Bytes(), logErr.Bytes(), h.config)
	}
	glog.V(4).Infof("Container run successful\n")
	return 0, nil
}

// printConfig prints out the relevant parts of a container's Docker config
func printConfig(c *docker.Config) string {
	out := &bytes.Buffer{}
	fmt.Fprintf(out, "  image: %s\n", c.Image)
	if len(c.Entrypoint) > 0 {
		fmt.Fprintf(out, "  entry point:\n")
		for _, e := range c.Entrypoint {
			fmt.Fprintf(out, "    %s\n", e)
		}
	}
	if len(c.Cmd) > 0 {
		fmt.Fprintf(out, "  command:\n")
		for _, c := range c.Cmd {
			fmt.Fprintf(out, "    %s\n", c)
		}
	}
	if len(c.Env) > 0 {
		fmt.Fprintf(out, "  environment:\n")
		for _, e := range c.Env {
			fmt.Fprintf(out, "    %s\n", e)
		}
	}
	return out.String()
}

func printHostConfig(c *docker.HostConfig) string {
	out := &bytes.Buffer{}
	fmt.Fprintf(out, "  pid mode: %s\n", c.PidMode)
	fmt.Fprintf(out, "  network mode: %s\n", c.NetworkMode)
	if len(c.Binds) > 0 {
		fmt.Fprintf(out, "  volume binds:\n")
		for _, b := range c.Binds {
			fmt.Fprintf(out, "    %s\n", b)
		}
	}
	return out.String()
}
