package docker

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/errors"
	"os"
	"os/signal"
)

const (
	// Deprecated environment variable name, specifying where to look for the S2I scripts.
	// It is now being replaced with ScriptsURLLabel.
	ScriptsURLEnvironment = "STI_SCRIPTS_URL"
	// Deprecated environment variable name, specifying where to place artifacts in
	// builder image. It is now being replaced with DestinationLabel.
	LocationEnvironment = "STI_LOCATION"

	// ScriptsURLLabel is the name of the Docker image LABEL that tells S2I where
	// to look for the S2I scripts. This label is also copied into the ouput
	// image.
	// The previous name of this label was 'io.s2i.scripts-url'. This is now
	// deprecated.
	ScriptsURLLabel = api.DefaultNamespace + "scripts-url"
	// DestinationLabel is the name of the Docker image LABEL that tells S2I where
	// to place the artifacts (scripts, sources) in the builder image.
	// The previous name of this label was 'io.s2i.destination'. This is now
	// deprecated
	DestinationLabel = api.DefaultNamespace + "destination"

	// DefaultDestination is the destination where the artifacts will be placed
	// if DestinationLabel was not specified.
	DefaultDestination = "/tmp"
	// DefaultTag is the image tag, being applied if none is specified.
	DefaultTag = "latest"
)

// Docker is the interface between STI and the Docker client
// It contains higher level operations called from the STI
// build or usage commands
type Docker interface {
	IsImageInLocalRegistry(name string) (bool, error)
	IsImageOnBuild(string) bool
	GetOnBuild(string) ([]string, error)
	RemoveContainer(id string) error
	GetScriptsURL(name string) (string, error)
	RunContainer(opts RunContainerOptions) error
	GetImageID(name string) (string, error)
	CommitContainer(opts CommitContainerOptions) (string, error)
	RemoveImage(name string) error
	CheckImage(name string) (*docker.Image, error)
	PullImage(name string) (*docker.Image, error)
	CheckAndPullImage(name string) (*docker.Image, error)
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
	InspectContainer(id string) (*docker.Container, error)
}

type stiDocker struct {
	client   Client
	pullAuth docker.AuthConfiguration
}

type PostExecutor interface {
	PostExecute(containerID, destination string) error
}

type PullResult struct {
	OnBuild bool
	Image   *docker.Image
}

// RunContainerOptions are options passed in to the RunContainer method
type RunContainerOptions struct {
	Image           string
	PullImage       bool
	PullAuth        docker.AuthConfiguration
	ExternalScripts bool
	ScriptsURL      string
	Destination     string
	Command         string
	Env             []string
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
	OnStart         func() error
	PostExec        PostExecutor
	TargetImage     bool
}

// CommitContainerOptions are options passed in to the CommitContainer method
type CommitContainerOptions struct {
	ContainerID string
	Repository  string
	Command     []string
	Env         []string
	Labels      map[string]string
}

// BuildImageOptions are options passed in to the BuildImage method
type BuildImageOptions struct {
	Name   string
	Stdin  io.Reader
	Stdout io.Writer
}

// New creates a new implementation of the STI Docker interface
func New(config *api.DockerConfig, auth docker.AuthConfiguration) (Docker, error) {
	var client *docker.Client
	var err error
	if config.CertFile != "" && config.KeyFile != "" && config.CAFile != "" {
		client, err = docker.NewTLSClient(
			config.Endpoint,
			config.CertFile,
			config.KeyFile,
			config.CAFile)
	} else {
		client, err = docker.NewClient(config.Endpoint)
	}
	if err != nil {
		return nil, err
	}
	return &stiDocker{
		client:   client,
		pullAuth: auth,
	}, nil
}

// IsImageInLocalRegistry determines whether the supplied image is in the local registry.
func (d *stiDocker) IsImageInLocalRegistry(name string) (bool, error) {
	name = getImageName(name)
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
	name = getImageName(name)
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
	onbuild, err := d.GetOnBuild(name)
	return err == nil && len(onbuild) > 0
}

// GetOnBuild returns the set of ONBUILD Dockerfile commands to execute
// for the given image
func (d *stiDocker) GetOnBuild(name string) ([]string, error) {
	name = getImageName(name)
	image, err := d.client.InspectImage(name)
	if err != nil {
		return nil, err
	}
	return image.Config.OnBuild, nil
}

// CheckAndPullImage pulls an image into the local registry if not present
// and returns the image metadata
func (d *stiDocker) CheckAndPullImage(name string) (*docker.Image, error) {
	name = getImageName(name)
	image, err := d.CheckImage(name)
	if err != nil && err.(errors.Error).Details != docker.ErrNoSuchImage {
		return nil, err
	}
	if image == nil {
		return d.PullImage(name)
	}

	glog.V(2).Infof("Image %s available locally", name)
	return image, nil
}

// CheckImage checks image from the local registry.
func (d *stiDocker) CheckImage(name string) (*docker.Image, error) {
	name = getImageName(name)
	image, err := d.client.InspectImage(name)
	if err != nil {
		return nil, errors.NewInspectImageError(name, err)
	}
	return image, nil
}

// PullImage pulls an image into the local registry
func (d *stiDocker) PullImage(name string) (*docker.Image, error) {
	name = getImageName(name)
	glog.V(1).Infof("Pulling image %s", name)
	// TODO: Add authentication support
	if err := d.client.PullImage(docker.PullImageOptions{Repository: name}, d.pullAuth); err != nil {
		glog.V(3).Infof("An error was received from the PullImage call: %v", err)
		return nil, errors.NewPullImageError(name, err)
	}
	image, err := d.client.InspectImage(name)
	if err != nil {
		return nil, errors.NewInspectImageError(name, err)
	}
	return image, nil
}

// RemoveContainer removes a container and its associated volumes.
func (d *stiDocker) RemoveContainer(id string) error {
	opts := docker.RemoveContainerOptions{
		ID:            id,
		RemoveVolumes: true,
		Force:         true,
	}
	return d.client.RemoveContainer(opts)
}

// getImageName checks the image name and adds DefaultTag if none is specified
func getImageName(name string) string {
	_, tag := docker.ParseRepositoryTag(name)
	if len(tag) == 0 {
		return strings.Join([]string{name, DefaultTag}, ":")
	}

	return name
}

// getLabel gets label's value from the image metadata
func getLabel(image *docker.Image, name string) string {
	if value, ok := image.Config.Labels[name]; ok {
		return value
	}
	if value, ok := image.ContainerConfig.Labels[name]; ok {
		return value
	}

	return ""
}

// getVariable gets environment variable's value from the image metadata
func getVariable(image *docker.Image, name string) string {
	envName := name + "="
	env := append(image.ContainerConfig.Env, image.Config.Env...)
	for _, v := range env {
		if strings.HasPrefix(v, envName) {
			return strings.TrimSpace((v[len(envName):]))
		}
	}

	return ""
}

// GetScriptsURL finds a scripts-url label in the given image's metadata
func (d *stiDocker) GetScriptsURL(image string) (string, error) {
	imageMetadata, err := d.CheckAndPullImage(image)
	if err != nil {
		return "", err
	}

	return getScriptsURL(imageMetadata), nil
}

// getScriptsURL finds a scripts url label in the image metadata
func getScriptsURL(image *docker.Image) string {
	scriptsURL := getLabel(image, ScriptsURLLabel)

	// For backward compatibility, support the old label schema
	if len(scriptsURL) == 0 {
		scriptsURL = getLabel(image, "io.s2i.scripts-url")
		if len(scriptsURL) > 0 {
			glog.Warningf("The 'io.s2i.scripts-url' label is deprecated. Use %q instead.", ScriptsURLLabel)
		}
	}
	if len(scriptsURL) == 0 {
		scriptsURL = getVariable(image, ScriptsURLEnvironment)
		if len(scriptsURL) != 0 {
			glog.Warningf("BuilderImage uses deprecated environment variable %s, please migrate it to %s label instead!",
				ScriptsURLEnvironment, ScriptsURLLabel)
		}
	}
	if len(scriptsURL) == 0 {
		glog.Warningf("Image does not contain a value for the %s label", ScriptsURLLabel)
	} else {
		glog.V(2).Infof("Image contains %s set to '%s'", ScriptsURLLabel, scriptsURL)
	}

	return scriptsURL
}

// getDestination finds a destination label in the image metadata
func getDestination(image *docker.Image) string {
	if val := getLabel(image, DestinationLabel); len(val) != 0 {
		return val
	}
	// For backward compatibility, support the old label schema
	if val := getLabel(image, "io.s2i.destination"); len(val) != 0 {
		glog.Warningf("The 'io.s2i.destination' label is deprecated. Use %q instead.", DestinationLabel)
		return val
	}
	if val := getVariable(image, LocationEnvironment); len(val) != 0 {
		glog.Warningf("BuilderImage uses deprecated environment variable %s, please migrate it to %s label instead!",
			LocationEnvironment, DestinationLabel)
		return val
	}

	// default directory if none is specified
	return DefaultDestination
}

// this funtion simply abstracts out the tar related processing that was originally inline in RunContainer()
func runContainerTar(opts RunContainerOptions, config docker.Config, imageMetadata *docker.Image) (docker.Config, string) {
	tarDestination := ""
	if opts.TargetImage {
		return config, tarDestination
	}

	// base directory for all STI commands
	var commandBaseDir string
	// untar operation destination directory
	tarDestination = opts.Destination
	if len(tarDestination) == 0 {
		tarDestination = getDestination(imageMetadata)
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
			scriptsURL = getScriptsURL(imageMetadata)
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
	config.Cmd = cmd
	return config, tarDestination
}

// this funtion simply abstracts out the running of the newly produced image when the --run=true option is provided
func runContainerDockerRun(container *docker.Container, d *stiDocker, image string) {
	cont, icerr := d.client.InspectContainer(container.ID)
	liveports := "\n\nPort Bindings:  "
	if icerr == nil {
		//Ports is of the follwing type:  map[docker.Port][]docker.PortBinding
		for port, bindings := range cont.NetworkSettings.Ports {
			liveports = liveports + "\n  Container Port:  " + port.Port()
			liveports = liveports + "\n        Protocol:  " + port.Proto()
			liveports = liveports + "\n        Public Host / Port Mappings:"
			for _, binding := range bindings {
				liveports = liveports + "\n            IP: " + binding.HostIP + " Port: " + binding.HostPort
			}
		}
		liveports = liveports + "\n"
	}

	glog.Infof("\n\n\n\n\nThe image %s has been started in container %s as a result of the --run=true option.  The container's stdout/stderr will be redirected to this command's glog output to help you validate its behavior.  You can also inspect the container with docker commands if you like.  If the container is set up to stay running, you will have to Ctrl-C to exit this command, which should also stop the container %s.  This particular invocation attempts to run with the port mappings %+v \n\n\n\n\n", image, container.ID, container.ID, liveports)

	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for range signalChan {
			glog.V(2).Info("\nReceived an interrupt, stopping services...\n")
			cleanupDone <- true
		}
	}()
	<-cleanupDone
}

// this funtion simply abstracts out the first phase of attaching to the container that was originally in line with the RunContainer() method
func runContainerAttachOne(attached chan struct{}, container *docker.Container, opts RunContainerOptions, d *stiDocker) sync.WaitGroup {
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

	if opts.Stderr != nil {
		attachOpts.ErrorStream = opts.Stderr
		attachOpts.Stderr = true
	}

	wg := sync.WaitGroup{}
	go func() {
		wg.Add(1)
		defer wg.Done()
		if err := d.client.AttachToContainer(attachOpts); err != nil {
			glog.Errorf("Unable to attach container with %v", attachOpts)
		}
	}()
	return wg
}

// this funtion simply abstracts out the second phase of attaching to the container that was originally in line with the RunContainer() method
func runContainerAttachTwo(attached2 chan struct{}, container *docker.Container, opts RunContainerOptions, d *stiDocker, wg sync.WaitGroup) {
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
		defer wg.Done()
		if err := d.client.AttachToContainer(attachOpts2); err != nil {
			glog.Errorf("Unable to attach container with %v", attachOpts2)
		}
	}()
}

// this funtion simply abstracts out the waiting on the container hosting the builder function that was originally in line with the RunContainer() method
func runContainerWait(wg sync.WaitGroup, d *stiDocker, container *docker.Container) error {
	glog.V(2).Infof("Waiting for container")
	exitCode, err := d.client.WaitContainer(container.ID)
	glog.V(2).Infof("Container wait returns with %d and %v\n", exitCode, err)

	wg.Wait()
	if err != nil {
		return err
	}

	glog.V(2).Infof("Container exited")
	if exitCode != 0 {
		return errors.NewContainerError(container.Name, exitCode, "")
	}
	return nil
}

// RunContainer creates and starts a container using the image specified in the options with the ability
// to stream input or output
func (d *stiDocker) RunContainer(opts RunContainerOptions) (err error) {
	// get info about the specified image
	image := getImageName(opts.Image)
	var imageMetadata *docker.Image
	if opts.PullImage {
		imageMetadata, err = d.CheckAndPullImage(image)
	} else {
		imageMetadata, err = d.client.InspectImage(image)
	}
	if err != nil {
		glog.Errorf("Unable to get image metadata for %s: %v", image, err)
		return err
	}

	config := docker.Config{
		Image: image,
	}

	config, tarDestination := runContainerTar(opts, config, imageMetadata)

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
	ccopts := docker.CreateContainerOptions{Name: "", Config: &config}
	if opts.TargetImage {
		ccopts.HostConfig = &docker.HostConfig{PublishAllPorts: true}
	}
	container, err := d.client.CreateContainer(ccopts)
	if err != nil {
		return err
	}
	defer d.RemoveContainer(container.ID)

	glog.V(2).Infof("Attaching to container")
	// creating / piping the channels in runContainerAttachOne lead to unintended hangs
	attached := make(chan struct{})
	wg := runContainerAttachOne(attached, container, opts, d)
	attached <- <-attached

	// If attaching both stdin and stdout or stderr, attach stdout and stderr in
	// a second goroutine
	// TODO remove this goroutine when docker 1.4 will be in broad usage,
	// see: https://github.com/docker/docker/commit/f936a10d8048f471d115978472006e1b58a7c67d
	if opts.Stdin != nil && opts.Stdout != nil {
		// creating / piping the channels in runContainerAttachTwo lead to unintended hangs
		attached2 := make(chan struct{})
		runContainerAttachTwo(attached2, container, opts, d, wg)
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

	if opts.TargetImage {

		runContainerDockerRun(container, d, image)

	} else {
		werr := runContainerWait(wg, d, container)
		if werr != nil {
			return werr
		}
	}

	if opts.PostExec != nil {
		glog.V(2).Infof("Invoking postExecution function")
		if err = opts.PostExec.PostExecute(container.ID, tarDestination); err != nil {
			return err
		}
	}
	return nil
}

// GetImageID retrieves the ID of the image identified by name
func (d *stiDocker) GetImageID(name string) (string, error) {
	name = getImageName(name)
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
			Cmd:    opts.Command,
			Env:    opts.Env,
			Labels: opts.Labels,
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
