package docker

import (
	"io"
	"strings"
	"sync"

	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	"github.com/openshift/source-to-image/pkg/sti/errors"
)

// Docker is the interface between STI and the Docker client
// It contains higher level operations called from the STI
// build or usage commands
type Docker interface {
	IsImageInLocalRegistry(name string) (bool, error)
	RemoveContainer(id string) error
	GetDefaultScriptsURL(name string) (string, error)
	RunContainer(opts RunContainerOptions) error
	GetImageID(name string) (string, error)
	CommitContainer(opts CommitContainerOptions) (string, error)
	RemoveImage(name string) error
	PullImage(name string) error
	CheckAndPull(name string) (*docker.Image, error)
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
}

type stiDocker struct {
	client Client
}

type postExecutor interface {
	PostExecute(containerID string, cmd []string) error
}

// RunContainerOptions are options passed in to the RunContainer method
type RunContainerOptions struct {
	Image        string
	PullImage    bool
	OverwriteCmd bool
	Command      string
	Env          []string
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	OnStart      func() error
	PostExec     postExecutor
}

// CommitContainerOptions are options passed in to the CommitContainer method
type CommitContainerOptions struct {
	ContainerID string
	Repository  string
	Command     []string
	Env         []string
}

// NewDocker creates a new implementation of the STI Docker interface
func NewDocker(endpoint string) (Docker, error) {
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

// CheckAndPull pulls an image into the local registry if not present
// and returns the image metadata
func (d *stiDocker) CheckAndPull(name string) (image *docker.Image, err error) {
	if image, err = d.client.InspectImage(name); err != nil && err != docker.ErrNoSuchImage {
		return nil, errors.NewInspectImageError(name, err)
	}
	if image == nil {
		if err = d.PullImage(name); err != nil {
			return nil, err
		}
		if image, err = d.client.InspectImage(name); err != nil {
			return nil, errors.NewInspectImageError(name, err)
		}
	} else {
		glog.V(2).Infof("Image %s available locally", name)
	}
	return
}

// PullImage pulls an image into the local registry
func (d *stiDocker) PullImage(name string) (err error) {
	glog.V(1).Infof("Pulling image %s", name)
	// TODO: Add authentication support
	if err = d.client.PullImage(docker.PullImageOptions{Repository: name},
		docker.AuthConfiguration{}); err != nil {
		return errors.NewPullImageError(name, err)
	}
	return nil
}

// RemoveContainer removes a container and its associated volumes.
func (d *stiDocker) RemoveContainer(id string) error {
	return d.client.RemoveContainer(docker.RemoveContainerOptions{id, true, true})
}

// GetDefaultUrl finds a script URL in the given image's metadata
func (d *stiDocker) GetDefaultScriptsURL(name string) (string, error) {
	imageMetadata, err := d.CheckAndPull(name)
	if err != nil {
		return "", err
	}
	defaultScriptsURL := ""
	env := append(imageMetadata.ContainerConfig.Env, imageMetadata.Config.Env...)
	for _, v := range env {
		if strings.HasPrefix(v, "STI_SCRIPTS_URL=") {
			defaultScriptsURL = strings.TrimSpace((v[len("STI_SCRIPTS_URL="):]))
			break
		}
	}
	if len(defaultScriptsURL) == 0 {
		glog.Warningf("Image does not contain default script URL")
	} else {
		glog.V(2).Infof("Image contains default script URL '%s'", defaultScriptsURL)
	}
	return defaultScriptsURL, nil
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

	cmd := imageMetadata.Config.Cmd
	if opts.OverwriteCmd {
		cmd[len(cmd)-1] = opts.Command
	} else {
		cmd = append(cmd, opts.Command)
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

	// If attaching both stdin and stdout, attach stdout in
	// a second goroutine
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
		return errors.NewContainerError(container.Name, exitCode, nil)
	}

	if opts.PostExec != nil {
		glog.V(2).Infof("Invoking postExecution function")
		if err = opts.PostExec.PostExecute(container.ID, imageMetadata.Config.Cmd); err != nil {
			return err
		}
	}
	return nil
}

// GetImageID retrives the ID of the image identified by name
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
		glog.V(2).Infof("Commiting container with config: %+v", config)
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
