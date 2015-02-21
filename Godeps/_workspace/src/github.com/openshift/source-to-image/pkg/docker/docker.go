package docker

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/errors"
)

const (
	ScriptsURL = "STI_SCRIPTS_URL"
	Location   = "STI_LOCATION"
)

// Docker is the interface between STI and the Docker client
// It contains higher level operations called from the STI
// build or usage commands
type Docker interface {
	IsImageInLocalRegistry(name string) (bool, error)
	IsImageOnBuild(string) bool
	RemoveContainer(id string) error
	GetScriptsURL(name string) (string, error)
	RunContainer(opts RunContainerOptions) error
	GetImageID(name string) (string, error)
	CommitContainer(opts CommitContainerOptions) (string, error)
	RemoveImage(name string) error
	PullImage(name string) (*docker.Image, error)
	CheckAndPull(name string) (*docker.Image, error)
	BuildImage(opts BuildImageOptions) error
	GetImageUser(name string) (string, error)
}

// Client contains all methods called on the go Docker
// client.
type Client interface {
	RemoveImage(name string) error
	InspectImage(name string) (*docker.Image, error)
	PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error
	CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error)
	AttachToContainer(opts docker.AttachToContainerOptions) error
	StartContainer(id string, hostConfig *docker.HostConfig) error
	WaitContainer(id string) (int, error)
	RemoveContainer(opts docker.RemoveContainerOptions) error
	CommitContainer(opts docker.CommitContainerOptions) (*docker.Image, error)
	CopyFromContainer(opts docker.CopyFromContainerOptions) error
	BuildImage(opts docker.BuildImageOptions) error
}

type stiDocker struct {
	client Client
}

type PostExecutor interface {
	PostExecute(containerID string, location string) error
}

type PullResult struct {
	OnBuild bool
	Image   *docker.Image
}

// RunContainerOptions are options passed in to the RunContainer method
type RunContainerOptions struct {
	Image           string
	PullImage       bool
	ExternalScripts bool
	ScriptsURL      string
	Location        string
	Command         string
	Env             []string
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
	OnStart         func() error
	PostExec        PostExecutor
}

// CommitContainerOptions are options passed in to the CommitContainer method
type CommitContainerOptions struct {
	ContainerID string
	Repository  string
	Command     []string
	Env         []string
}

// BuildImageOptions are options passed in to the BuildImage method
type BuildImageOptions struct {
	Name   string
	Stdin  io.Reader
	Stdout io.Writer
}

// New creates a new implementation of the STI Docker interface
func New(endpoint string) (Docker, error) {
	client, err := docker.NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	return &stiDocker{
		client: client,
	}, nil
}

// IsImageInLocalRegistry determines whether the supplied image is in the local registry.
func (d *stiDocker) IsImageInLocalRegistry(name string) (bool, error) {
	image, err := d.client.InspectImage(name)

	if image != nil {
		return true, nil
	} else if err == docker.ErrNoSuchImage {
		return false, nil
	}
	return false, err
}

// GetImageUser finds and retrieves the user associated with
// an image if one has been specified
func (d *stiDocker) GetImageUser(name string) (string, error) {
	image, err := d.client.InspectImage(name)
	if err != nil {
		return "", errors.NewInspectImageError(name, err)
	}
	user := image.ContainerConfig.User
	if len(user) == 0 {
		user = image.Config.User
	}
	return user, nil
}

// IsImageOnBuild provides information about whether the Docker image has
// OnBuild instruction recorded in the Image Config.
func (d *stiDocker) IsImageOnBuild(name string) bool {
	image, err := d.client.InspectImage(name)
	if err != nil {
		return false
	}
	return len(image.Config.OnBuild) > 0
}

// CheckAndPull pulls an image into the local registry if not present
// and returns the image metadata
func (d *stiDocker) CheckAndPull(name string) (image *docker.Image, err error) {
	if image, err = d.client.InspectImage(name); err != nil && err != docker.ErrNoSuchImage {
		return nil, errors.NewInspectImageError(name, err)
	}
	if image == nil {
		return d.PullImage(name)
	}

	glog.V(2).Infof("Image %s available locally", name)
	return
}

// PullImage pulls an image into the local registry
func (d *stiDocker) PullImage(name string) (image *docker.Image, err error) {
	glog.V(1).Infof("Pulling image %s", name)
	// TODO: Add authentication support
	if err = d.client.PullImage(docker.PullImageOptions{Repository: name},
		docker.AuthConfiguration{}); err != nil {
		glog.V(3).Infof("An error was received from the PullImage call: %v", err)
		return nil, errors.NewPullImageError(name, err)
	}
	if image, err = d.client.InspectImage(name); err != nil {
		return nil, errors.NewInspectImageError(name, err)
	}
	return
}

// RemoveContainer removes a container and its associated volumes.
func (d *stiDocker) RemoveContainer(id string) error {
	return d.client.RemoveContainer(docker.RemoveContainerOptions{id, true, true})
}

// getVariable gets environment variable's value from the image metadata
func (d *stiDocker) getVariable(image *docker.Image, name string) string {
	envName := name + "="
	env := append(image.ContainerConfig.Env, image.Config.Env...)
	for _, v := range env {
		if strings.HasPrefix(v, envName) {
			return strings.TrimSpace((v[len(envName):]))
		}
	}

	return ""
}

// GetScriptsURL finds a STI_SCRIPTS_URL in the given image's metadata
func (d *stiDocker) GetScriptsURL(image string) (string, error) {
	imageMetadata, err := d.CheckAndPull(image)
	if err != nil {
		return "", err
	}

	scriptsURL := d.getVariable(imageMetadata, ScriptsURL)
	if len(scriptsURL) == 0 {
		glog.Warningf("Image does not contain a value for the STI_SCRIPTS_URL environment variable")
	} else {
		glog.V(2).Infof("Image contains STI_SCRIPTS_URL set to '%s'", scriptsURL)
	}

	return scriptsURL, nil
}

// RunContainer creates and starts a container using the image specified in the options with the ability
// to stream input or output
func (d *stiDocker) RunContainer(opts RunContainerOptions) (err error) {
	// get info about the specified image
	var imageMetadata *docker.Image
	if opts.PullImage {
		imageMetadata, err = d.CheckAndPull(opts.Image)
	} else {
		imageMetadata, err = d.client.InspectImage(opts.Image)
	}
	if err != nil {
		glog.Errorf("Unable to get image metadata for %s: %v", opts.Image, err)
		return err
	}

	// base directory for all STI commands
	var commandBaseDir string
	// untar operation destination directory
	tarDestination := opts.Location
	if len(tarDestination) == 0 {
		if val := d.getVariable(imageMetadata, Location); len(val) != 0 {
			tarDestination = val
		} else {
			// default directory if none is specified
			tarDestination = "/tmp"
		}
	}
	if opts.ExternalScripts {
		// for external scripts we must always append 'scripts' because this is
		// the default subdirectory inside tar for them
		commandBaseDir = filepath.Join(tarDestination, "scripts")
		glog.V(2).Infof("Both scripts and untarred source will be placed in '%s'", tarDestination)
	} else {
		// for internal scripts we can have separate path for scripts and untar operation destination
		scriptsURL := opts.ScriptsURL
		if len(scriptsURL) == 0 {
			scriptsURL = d.getVariable(imageMetadata, ScriptsURL)
		}
		commandBaseDir = strings.TrimPrefix(scriptsURL, "image://")
		glog.V(2).Infof("Base directory for STI scripts is '%s'. Untarring destination is '%s'.",
			commandBaseDir, tarDestination)
	}

	cmd := []string{filepath.Join(commandBaseDir, string(opts.Command))}
	// when calling assemble script with Stdin parameter set (the tar file)
	// we need to first untar the whole archive and only then call the assemble script
	if opts.Stdin != nil && (opts.Command == api.Assemble || opts.Command == api.Usage) {
		cmd = []string{"/bin/sh", "-c", fmt.Sprintf("tar -C %s -xf - && %s",
			tarDestination, filepath.Join(commandBaseDir, string(opts.Command)))}
	}
	config := docker.Config{
		Image: opts.Image,
		Cmd:   cmd,
	}

	if opts.Env != nil {
		config.Env = opts.Env
	}
	if opts.Stdin != nil {
		config.OpenStdin = true
		config.StdinOnce = true
	}
	if opts.Stdout != nil {
		config.AttachStdout = true
	}

	glog.V(2).Infof("Creating container using config: %+v", config)
	container, err := d.client.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})
	if err != nil {
		return err
	}
	defer d.RemoveContainer(container.ID)

	glog.V(2).Infof("Attaching to container")
	attached := make(chan struct{})
	attachOpts := docker.AttachToContainerOptions{
		Container: container.ID,
		Success:   attached,
		Stream:    true,
	}
	if opts.Stdin != nil {
		attachOpts.InputStream = opts.Stdin
		attachOpts.Stdin = true
	} else if opts.Stdout != nil {
		attachOpts.OutputStream = opts.Stdout
		attachOpts.Stdout = true
	}

	wg := sync.WaitGroup{}
	go func() {
		wg.Add(1)
		d.client.AttachToContainer(attachOpts)
		wg.Done()
	}()
	attached <- <-attached

	// If attaching both stdin and stdout or stderr, attach stdout and stderr in
	// a second goroutine
	// TODO remove this goroutine when docker 1.4 will be in broad usage,
	// see: https://github.com/docker/docker/commit/f936a10d8048f471d115978472006e1b58a7c67d
	if opts.Stdin != nil && opts.Stdout != nil {
		attached2 := make(chan struct{})
		attachOpts2 := docker.AttachToContainerOptions{
			Container:    container.ID,
			Success:      attached2,
			Stream:       true,
			OutputStream: opts.Stdout,
			Stdout:       true,
		}
		if opts.Stderr != nil {
			attachOpts2.Stderr = true
			attachOpts2.ErrorStream = opts.Stderr
		}
		go func() {
			wg.Add(1)
			d.client.AttachToContainer(attachOpts2)
			wg.Done()
		}()
		attached2 <- <-attached2
	}

	glog.V(2).Infof("Starting container")
	if err = d.client.StartContainer(container.ID, nil); err != nil {
		return err
	}
	if opts.OnStart != nil {
		if err = opts.OnStart(); err != nil {
			return err
		}
	}

	glog.V(2).Infof("Waiting for container")
	exitCode, err := d.client.WaitContainer(container.ID)
	wg.Wait()
	if err != nil {
		return err
	}
	glog.V(2).Infof("Container exited")

	if exitCode != 0 {
		return errors.NewContainerError(container.Name, exitCode, "")
	}
	if opts.PostExec != nil {
		glog.V(2).Infof("Invoking postExecution function")
		if err = opts.PostExec.PostExecute(container.ID, commandBaseDir); err != nil {
			return err
		}
	}
	return nil
}

// GetImageID retrieves the ID of the image identified by name
func (d *stiDocker) GetImageID(name string) (string, error) {
	image, err := d.client.InspectImage(name)
	if err != nil {
		return "", err
	}
	return image.ID, nil
}

// CommitContainer commits a container to an image with a specific tag.
// The new image ID is returned
func (d *stiDocker) CommitContainer(opts CommitContainerOptions) (string, error) {
	repository, tag := docker.ParseRepositoryTag(opts.Repository)
	dockerOpts := docker.CommitContainerOptions{
		Container:  opts.ContainerID,
		Repository: repository,
		Tag:        tag,
	}
	if opts.Command != nil {
		config := docker.Config{
			Cmd: opts.Command,
			Env: opts.Env,
		}
		dockerOpts.Run = &config
		glog.V(2).Infof("Committing container with config: %+v", config)
	}

	image, err := d.client.CommitContainer(dockerOpts)
	if err == nil && image != nil {
		return image.ID, nil
	}
	return "", err
}

// RemoveImage removes the image with specified ID
func (d *stiDocker) RemoveImage(imageID string) error {
	return d.client.RemoveImage(imageID)
}

// BuildImage builds the image according to specified options
func (d *stiDocker) BuildImage(opts BuildImageOptions) error {
	dockerOpts := docker.BuildImageOptions{
		Name:                opts.Name,
		NoCache:             true,
		SuppressOutput:      false,
		RmTmpContainer:      true,
		ForceRmTmpContainer: true,
		InputStream:         opts.Stdin,
		OutputStream:        opts.Stdout,
	}
	return d.client.BuildImage(dockerOpts)
}
