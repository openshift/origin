package docker

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util"
	"github.com/openshift/source-to-image/pkg/util/interrupt"
)

const (
	// ScriptsURLEnvironment is a deprecated environment variable name that
	// specifies where to look for S2I scripts. Use ScriptsURLLabel instead.
	ScriptsURLEnvironment = "STI_SCRIPTS_URL"
	// LocationEnvironment is a deprecated environment variable name that
	// specifies where to place artifacts in a builder image. Use
	// DestinationLabel instead.
	LocationEnvironment = "STI_LOCATION"

	// ScriptsURLLabel is the name of the Docker image LABEL that tells S2I where
	// to look for the S2I scripts. This label is also copied into the output
	// image.
	// The previous name of this label was 'io.s2i.scripts-url'. This is now
	// deprecated.
	ScriptsURLLabel = api.DefaultNamespace + "scripts-url"
	// DestinationLabel is the name of the Docker image LABEL that tells S2I where
	// to place the artifacts (scripts, sources) in the builder image.
	// The previous name of this label was 'io.s2i.destination'. This is now
	// deprecated
	DestinationLabel = api.DefaultNamespace + "destination"
	// AssembleInputFilesLabel is the name of the Docker image LABEL that tells S2I which
	// files wil be copied from builder to a runtime image.
	AssembleInputFilesLabel = api.DefaultNamespace + "assemble-input-files"

	// DefaultDestination is the destination where the artifacts will be placed
	// if DestinationLabel was not specified.
	DefaultDestination = "/tmp"
	// DefaultTag is the image tag, being applied if none is specified.
	DefaultTag = "latest"

	// DefaultDockerTimeout specifies a timeout for Docker API calls. When this
	// timeout is reached, certain Docker API calls might error out.
	DefaultDockerTimeout = 20 * time.Second
)

// containerNamePrefix prefixes the name of containers launched by S2I. We
// cannot reuse the prefix "k8s" because we don't want the containers to be
// managed by a kubelet.
const containerNamePrefix = "s2i"

// containerName creates names for Docker containers launched by S2I. It is
// meant to resemble Kubernetes' pkg/kubelet/dockertools.BuildDockerName.
func containerName(image string) string {
	uid := fmt.Sprintf("%08x", rand.Uint32())
	// Replace invalid characters for container name with underscores.
	image = strings.Map(func(r rune) rune {
		if ('0' <= r && r <= '9') || ('A' <= r && r <= 'Z') || ('a' <= r && r <= 'z') {
			return r
		}
		return '_'
	}, image)
	return fmt.Sprintf("%s_%s_%s", containerNamePrefix, image, uid)
}

// Docker is the interface between STI and the Docker client
// It contains higher level operations called from the STI
// build or usage commands
type Docker interface {
	IsImageInLocalRegistry(name string) (bool, error)
	IsImageOnBuild(string) bool
	GetOnBuild(string) ([]string, error)
	RemoveContainer(id string) error
	GetScriptsURL(name string) (string, error)
	GetAssembleInputFiles(string) (string, error)
	RunContainer(opts RunContainerOptions) error
	GetImageID(name string) (string, error)
	GetImageWorkdir(name string) (string, error)
	CommitContainer(opts CommitContainerOptions) (string, error)
	RemoveImage(name string) error
	CheckImage(name string) (*docker.Image, error)
	PullImage(name string) (*docker.Image, error)
	CheckAndPullImage(name string) (*docker.Image, error)
	BuildImage(opts BuildImageOptions) error
	GetImageUser(name string) (string, error)
	GetImageEntrypoint(name string) ([]string, error)
	GetLabels(name string) (map[string]string, error)
	UploadToContainer(srcPath, destPath, container string) error
	UploadToContainerWithCallback(srcPath, destPath, container string, walkFn filepath.WalkFunc, modifyInplace bool) error
	DownloadFromContainer(containerPath string, w io.Writer, container string) error
	Ping() error
}

// Client contains all methods called on the go Docker
// client.
type Client interface {
	RemoveImage(name string) error
	InspectImage(name string) (*docker.Image, error)
	PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error
	CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error)
	AttachToContainerNonBlocking(opts docker.AttachToContainerOptions) (docker.CloseWaiter, error)
	StartContainer(id string, hostConfig *docker.HostConfig) error
	WaitContainer(id string) (int, error)
	UploadToContainer(id string, opts docker.UploadToContainerOptions) error
	DownloadFromContainer(id string, opts docker.DownloadFromContainerOptions) error
	RemoveContainer(opts docker.RemoveContainerOptions) error
	CommitContainer(opts docker.CommitContainerOptions) (*docker.Image, error)
	CopyFromContainer(opts docker.CopyFromContainerOptions) error
	BuildImage(opts docker.BuildImageOptions) error
	InspectContainer(id string) (*docker.Container, error)
	Ping() error
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
	Env             []string
	// Entrypoint will be used to override the default entrypoint
	// for the image if it has one.  If the image has no entrypoint,
	// this value is ignored.
	Entrypoint       []string
	Stdin            io.Reader
	Stdout           io.Writer
	Stderr           io.Writer
	OnStart          func(containerID string) error
	PostExec         PostExecutor
	TargetImage      bool
	NetworkMode      string
	User             string
	CGroupLimits     *api.CGroupLimits
	CapDrop          []string
	Binds            []string
	Command          string
	CommandOverrides func(originalCmd string) string
	// CommandExplicit provides a full control on the CMD directive.
	// It won't modified in any way and will be passed to the docker as-is.
	// Use this option when you want to use arbitrary command as CMD directive.
	// In this case you can't use Command because 1) it's just a string
	// 2) it will be modified by prepending base dir and cleaned by the path.Join().
	// You also can't use CommandOverrides because 1) it's a string
	// 2) it only gets applied when Command equals to "assemble" or "usage" script
	// AND script is inside of the tar archive.
	CommandExplicit []string
}

// asDockerConfig converts a RunContainerOptions into a Config understood by the
// go-dockerclient library.
func (rco RunContainerOptions) asDockerConfig() docker.Config {
	return docker.Config{
		Image:        getImageName(rco.Image),
		User:         rco.User,
		Env:          rco.Env,
		Entrypoint:   rco.Entrypoint,
		OpenStdin:    rco.Stdin != nil,
		StdinOnce:    rco.Stdin != nil,
		AttachStdout: rco.Stdout != nil,
	}
}

// asDockerHostConfig converts a RunContainerOptions into a HostConfig
// understood by the go-dockerclient library.
func (rco RunContainerOptions) asDockerHostConfig() docker.HostConfig {
	hostConfig := docker.HostConfig{
		CapDrop:         rco.CapDrop,
		PublishAllPorts: rco.TargetImage,
		NetworkMode:     rco.NetworkMode,
		Binds:           rco.Binds,
	}
	if rco.CGroupLimits != nil {
		hostConfig.Memory = rco.CGroupLimits.MemoryLimitBytes
		hostConfig.MemorySwap = rco.CGroupLimits.MemorySwap
		hostConfig.CPUShares = rco.CGroupLimits.CPUShares
		hostConfig.CPUQuota = rco.CGroupLimits.CPUQuota
		hostConfig.CPUPeriod = rco.CGroupLimits.CPUPeriod
	}
	return hostConfig
}

// asDockerCreateContainerOptions converts a RunContainerOptions into a
// CreateContainerOptions understood by the go-dockerclient library.
func (rco RunContainerOptions) asDockerCreateContainerOptions() docker.CreateContainerOptions {
	config := rco.asDockerConfig()
	hostConfig := rco.asDockerHostConfig()
	return docker.CreateContainerOptions{
		Name:       containerName(rco.Image),
		Config:     &config,
		HostConfig: &hostConfig,
	}
}

// asDockerAttachToContainerOptions converts a RunContainerOptions into a
// AttachToContainerOptions understood by the go-dockerclient library.
func (rco RunContainerOptions) asDockerAttachToContainerOptions() docker.AttachToContainerOptions {
	return docker.AttachToContainerOptions{
		InputStream:  rco.Stdin,
		OutputStream: rco.Stdout,
		ErrorStream:  rco.Stderr,

		Stdin:  rco.Stdin != nil,
		Stdout: rco.Stdout != nil,
		Stderr: rco.Stderr != nil,

		Logs:   rco.Stdout != nil,
		Stream: rco.Stdout != nil,
	}
}

// CommitContainerOptions are options passed in to the CommitContainer method
type CommitContainerOptions struct {
	ContainerID string
	Repository  string
	User        string
	Command     []string
	Env         []string
	Entrypoint  []string
	Labels      map[string]string
}

// BuildImageOptions are options passed in to the BuildImage method
type BuildImageOptions struct {
	Name         string
	Stdin        io.Reader
	Stdout       io.Writer
	CGroupLimits *api.CGroupLimits
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

// GetImageWorkdir returns the WORKDIR property for the given image name.
// When the WORKDIR is not set or empty, return "/" instead.
func (d *stiDocker) GetImageWorkdir(name string) (string, error) {
	image, err := d.client.InspectImage(name)
	if err != nil {
		return "", err
	}
	workdir := image.Config.WorkingDir
	if len(workdir) == 0 {
		// This is a default destination used by UploadToContainer when the WORKDIR
		// is not set or it is empty. To show user where the injections will end up,
		// we set this to "/".
		workdir = "/"
	}
	return workdir, nil
}

// GetImageEntrypoint returns the ENTRYPOINT property for the given image name.
func (d *stiDocker) GetImageEntrypoint(name string) ([]string, error) {
	image, err := d.client.InspectImage(name)
	if err != nil {
		return nil, err
	}
	return image.Config.Entrypoint, nil
}

// UploadToContainer uploads artifacts to the container.
func (d *stiDocker) UploadToContainer(src, dest, container string) error {
	makeFileWorldWritable := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip chmod if on windows OS and for symlinks
		if runtime.GOOS == "windows" || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		mode := os.FileMode(0666)
		if info.IsDir() {
			mode = 0777
		}
		return os.Chmod(path, mode)
	}
	return d.UploadToContainerWithCallback(src, dest, container, makeFileWorldWritable, false)
}

// UploadToContainerWithCallback uploads artifacts to the container.
// If the source is a directory, then all files and sub-folders are copied into
// the destination (which has to be directory as well).
// If the source is a single file, then the file copied into destination (which
// has to be full path to a file inside the container).
// If the destination path is empty or set to ".", then we will try to figure
// out the WORKDIR of the image that the container was created from and use that
// as a destination. If the WORKDIR is not set, then we copy files into "/"
// folder (docker upload default).
func (d *stiDocker) UploadToContainerWithCallback(src, dest, container string, walkFn filepath.WalkFunc, modifyInplace bool) error {
	path := filepath.Dir(dest)
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	info, _ := f.Stat()
	defer f.Close()
	t := tar.New()
	r, w := io.Pipe()
	if info.IsDir() {
		path = dest
		go func() {
			defer w.Close()
			if err := t.StreamDirAsTarWithCallback(src, w, walkFn, modifyInplace); err != nil {
				glog.V(0).Infof("error: Uploading directory to container failed: %v", err)
			}
		}()
	} else {
		go func() {
			defer w.Close()
			if err := t.StreamFileAsTarWithCallback(src, filepath.Base(dest), w, walkFn, modifyInplace); err != nil {
				glog.V(0).Infof("error: Uploading files to container failed: %v", err)
			}
		}()
	}
	glog.V(3).Infof("Uploading %q to %q ...", src, path)
	opts := docker.UploadToContainerOptions{Path: path, InputStream: r}
	return d.client.UploadToContainer(container, opts)
}

// DownloadFromContainer downloads file (or directory) from the container.
func (d *stiDocker) DownloadFromContainer(containerPath string, w io.Writer, container string) error {
	opts := docker.DownloadFromContainerOptions{Path: containerPath, OutputStream: w}
	return d.client.DownloadFromContainer(container, opts)
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
		glog.V(4).Infof("error inspecting image %s: %v", name, err)
		return "", errors.NewInspectImageError(name, err)
	}
	user := image.ContainerConfig.User
	if len(user) == 0 {
		user = image.Config.User
	}
	return user, nil
}

// Ping determines if the Docker daemon is reachable
func (d *stiDocker) Ping() error {
	return d.client.Ping()
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
	displayName := name

	if !glog.Is(3) {
		// For less verbose log levels (less than 3), shorten long iamge names like:
		//     "centos/php-56-centos7@sha256:51c3e2b08bd9fadefccd6ec42288680d6d7f861bdbfbd2d8d24960621e4e27f5"
		// to include just enough characters to differentiate the build from others in the docker repository:
		//     "centos/php-56-centos7@sha256:51c3e2b08bd..."
		// 18 characters is somewhat arbitrary, but should be enough to avoid a name collision.
		split := strings.Split(name, "@")
		if len(split) > 1 && len(split[1]) > 18 {
			displayName = split[0] + "@" + split[1][:18] + "..."
		}
	}

	image, err := d.CheckImage(name)
	if err != nil && err.(errors.Error).Details != docker.ErrNoSuchImage {
		return nil, err
	}
	if image == nil {
		glog.V(1).Infof("Image %q not available locally, pulling ...", displayName)
		return d.PullImage(name)
	}

	glog.V(3).Infof("Using locally available image %q", displayName)
	return image, nil
}

// CheckImage checks image from the local registry.
func (d *stiDocker) CheckImage(name string) (*docker.Image, error) {
	name = getImageName(name)
	image, err := d.client.InspectImage(name)
	if err != nil {
		glog.V(4).Infof("error inspecting image %s: %v", name, err)
		return nil, errors.NewInspectImageError(name, err)
	}
	return image, nil
}

// PullImage pulls an image into the local registry
func (d *stiDocker) PullImage(name string) (*docker.Image, error) {
	name = getImageName(name)
	glog.V(1).Infof("Pulling Docker image %s ...", name)
	// TODO: Add authentication support
	if err := d.client.PullImage(docker.PullImageOptions{Repository: name}, d.pullAuth); err != nil {
		glog.V(3).Infof("An error was received from the PullImage call: %v", err)
		return nil, errors.NewPullImageError(name, err)
	}
	image, err := d.client.InspectImage(name)
	if err != nil {
		glog.V(4).Infof("error inspecting image %s: %v", name, err)
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

// GetLabels retrieves the labels of the given image.
func (d *stiDocker) GetLabels(name string) (map[string]string, error) {
	name = getImageName(name)
	image, err := d.client.InspectImage(name)
	if err != nil {
		glog.V(4).Infof("error inspecting image %s: %v", name, err)
		return nil, errors.NewInspectImageError(name, err)
	}
	return image.Config.Labels, nil

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

// GetScriptsURL finds a scripts-url label on the given image.
func (d *stiDocker) GetScriptsURL(image string) (string, error) {
	imageMetadata, err := d.CheckAndPullImage(image)
	if err != nil {
		return "", err
	}

	return getScriptsURL(imageMetadata), nil
}

// GetAssembleInputFiles finds a io.openshift.s2i.assemble-input-files label on the given image.
func (d *stiDocker) GetAssembleInputFiles(image string) (string, error) {
	imageMetadata, err := d.CheckAndPullImage(image)
	if err != nil {
		return "", err
	}

	label := getLabel(imageMetadata, AssembleInputFilesLabel)
	if len(label) == 0 {
		glog.V(0).Infof("warning: Image %q does not contain a value for the %s label", image, AssembleInputFilesLabel)
	} else {
		glog.V(3).Infof("Image %q contains %s set to %q", image, AssembleInputFilesLabel, label)
	}
	return label, nil
}

// getScriptsURL finds a scripts url label in the image metadata
func getScriptsURL(image *docker.Image) string {
	scriptsURL := getLabel(image, ScriptsURLLabel)

	// For backward compatibility, support the old label schema
	if len(scriptsURL) == 0 {
		scriptsURL = getLabel(image, "io.s2i.scripts-url")
		if len(scriptsURL) > 0 {
			glog.V(0).Infof("warning: Image %s uses deprecated label 'io.s2i.scripts-url', please migrate it to %s instead!",
				image.ID, ScriptsURLLabel)
		}
	}
	if len(scriptsURL) == 0 {
		scriptsURL = getVariable(image, ScriptsURLEnvironment)
		if len(scriptsURL) != 0 {
			glog.V(0).Infof("warning: Image %s uses deprecated environment variable %s, please migrate it to %s label instead!",
				image.ID, ScriptsURLEnvironment, ScriptsURLLabel)
		}
	}
	if len(scriptsURL) == 0 {
		glog.V(0).Infof("warning: Image %s does not contain a value for the %s label", image.ID, ScriptsURLLabel)
	} else {
		glog.V(2).Infof("Image %s contains %s set to %q", image.ID, ScriptsURLLabel, scriptsURL)
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
		glog.V(0).Infof("warning: Image %s uses deprecated label 'io.s2i.destination', please migrate it to %s instead!",
			image.ID, DestinationLabel)
		return val
	}
	if val := getVariable(image, LocationEnvironment); len(val) != 0 {
		glog.V(0).Infof("warning: Image %s uses deprecated environment variable %s, please migrate it to %s label instead!",
			image.ID, LocationEnvironment, DestinationLabel)
		return val
	}

	// default directory if none is specified
	return DefaultDestination
}

func constructCommand(opts RunContainerOptions, imageMetadata *docker.Image, tarDestination string) []string {
	// base directory for all S2I commands
	commandBaseDir := determineCommandBaseDir(opts, imageMetadata, tarDestination)

	// NOTE: We use path.Join instead of filepath.Join to avoid converting the
	// path to UNC (Windows) format as we always run this inside container.
	binaryToRun := path.Join(commandBaseDir, opts.Command)

	// when calling assemble script with Stdin parameter set (the tar file)
	// we need to first untar the whole archive and only then call the assemble script
	if opts.Stdin != nil && (opts.Command == api.Assemble || opts.Command == api.Usage) {
		untarAndRun := fmt.Sprintf("tar -C %s -xf - && %s", tarDestination, binaryToRun)

		resultedCommand := untarAndRun
		if opts.CommandOverrides != nil {
			resultedCommand = opts.CommandOverrides(untarAndRun)
		}
		return []string{"/bin/sh", "-c", resultedCommand}
	}

	return []string{binaryToRun}
}

func determineTarDestinationDir(opts RunContainerOptions, imageMetadata *docker.Image) string {
	if len(opts.Destination) != 0 {
		return opts.Destination
	}
	return getDestination(imageMetadata)
}

func determineCommandBaseDir(opts RunContainerOptions, imageMetadata *docker.Image, tarDestination string) string {
	if opts.ExternalScripts {
		// for external scripts we must always append 'scripts' because this is
		// the default subdirectory inside tar for them
		// NOTE: We use path.Join instead of filepath.Join to avoid converting the
		// path to UNC (Windows) format as we always run this inside container.
		glog.V(2).Infof("Both scripts and untarred source will be placed in '%s'", tarDestination)
		return path.Join(tarDestination, "scripts")
	}

	// for internal scripts we can have separate path for scripts and untar operation destination
	scriptsURL := opts.ScriptsURL
	if len(scriptsURL) == 0 {
		scriptsURL = getScriptsURL(imageMetadata)
	}

	commandBaseDir := strings.TrimPrefix(scriptsURL, "image://")
	glog.V(2).Infof("Base directory for S2I scripts is '%s'. Untarring destination is '%s'.",
		commandBaseDir, tarDestination)

	return commandBaseDir
}

// dumpContainerInfo dumps information about a running container (port/IP/etc).
func dumpContainerInfo(container *docker.Container, d *stiDocker, image string) {
	cont, icerr := d.client.InspectContainer(container.ID)
	liveports := "\n\nPort Bindings:  "
	if icerr == nil {
		// Ports is of the following type:  map[docker.Port][]docker.PortBinding
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
	glog.V(0).Infof("\n\n\n\n\nThe image %s has been started in container %s as a result of the --run=true option.  The container's stdout/stderr will be redirected to this command's glog output to help you validate its behavior.  You can also inspect the container with docker commands if you like.  If the container is set up to stay running, you will have to Ctrl-C to exit this command, which should also stop the container %s.  This particular invocation attempts to run with the port mappings %+v \n\n\n\n\n", image, container.ID, container.ID, liveports)
}

// RunContainer creates and starts a container using the image specified in opts
// with the ability to stream input and/or output.
func (d *stiDocker) RunContainer(opts RunContainerOptions) error {
	createOpts := opts.asDockerCreateContainerOptions()

	// get info about the specified image
	image := createOpts.Config.Image
	var (
		imageMetadata *docker.Image
		err           error
	)
	if opts.PullImage {
		imageMetadata, err = d.CheckAndPullImage(image)
	} else {
		imageMetadata, err = d.client.InspectImage(image)
	}

	// if the original image has no entrypoint, and the run invocation
	// is trying to set an entrypoint, ignore it.  We only want to
	// set the entrypoint if we need to override a default entrypoint
	// in the image.  This allows us to still work with a minimal image
	// that does not contain "/usr/bin/env" since we don't attempt to override
	// the entrypoint.
	if len(opts.Entrypoint) != 0 {
		entrypoint, err := d.GetImageEntrypoint(image)
		if err != nil {
			return err
		}
		if len(entrypoint) == 0 {
			opts.Entrypoint = nil
		}
	}

	if err != nil {
		glog.V(0).Infof("error: Unable to get image metadata for %s: %v", image, err)
		return err
	}

	// tarDestination will be passed as location to PostExecute function
	// and will be used as the prefix for the CMD (scripts/run)
	var tarDestination string

	var cmd []string
	if !opts.TargetImage {
		if len(opts.CommandExplicit) != 0 {
			cmd = opts.CommandExplicit
		} else {
			tarDestination = determineTarDestinationDir(opts, imageMetadata)
			cmd = constructCommand(opts, imageMetadata, tarDestination)
		}
		glog.V(5).Infof("Setting %q command for container ...", strings.Join(cmd, " "))
	}
	createOpts.Config.Cmd = cmd

	// Create a new container.
	glog.V(2).Infof("Creating container with options {Name:%q Config:%+v HostConfig:%+v} ...", createOpts.Name, createOpts.Config, createOpts.HostConfig)
	var container *docker.Container
	if err = util.TimeoutAfter(DefaultDockerTimeout, "timeout after waiting %v for Docker to create container", func() error {
		var createErr error
		container, createErr = d.client.CreateContainer(createOpts)
		return createErr
	}); err != nil {
		return err
	}

	containerName := containerNameOrID(container)

	// Container was created, so we defer its removal, and also remove it if we get a SIGINT/SIGTERM/SIGQUIT/SIGHUP.
	removeContainer := func() {
		glog.V(4).Infof("Removing container %q ...", containerName)
		if err := d.RemoveContainer(container.ID); err != nil {
			glog.V(0).Infof("warning: Failed to remove container %q: %v", containerName, err)
		} else {
			glog.V(4).Infof("Removed container %q", containerName)
		}
	}
	dumpStack := func(signal os.Signal) {
		if signal == syscall.SIGQUIT {
			buf := make([]byte, 1<<16)
			runtime.Stack(buf, true)
			fmt.Printf("%s", buf)
		}
		os.Exit(2)
	}
	return interrupt.New(dumpStack, removeContainer).Run(func() error {
		// Attach to the container.
		glog.V(2).Infof("Attaching to container %q ...", containerName)
		attachOpts := opts.asDockerAttachToContainerOptions()
		attachOpts.Container = container.ID
		if _, err = d.client.AttachToContainerNonBlocking(attachOpts); err != nil {
			glog.V(0).Infof("error: Unable to attach to container %q with options %+v: %v", containerName, attachOpts, err)
			return err
		}

		// Start the container.
		glog.V(2).Infof("Starting container %q ...", containerName)
		if err := util.TimeoutAfter(DefaultDockerTimeout, "timeout after waiting %v for Docker to start container", func() error {
			return d.client.StartContainer(container.ID, nil)
		}); err != nil {
			return err
		}

		// Run OnStart hook if defined. OnStart might block, so we run it in a
		// new goroutine, and wait for it to be done later on.
		onStartDone := make(chan error, 1)
		if opts.OnStart != nil {
			go func() {
				onStartDone <- opts.OnStart(container.ID)
			}()
		}

		if opts.TargetImage {
			// When TargetImage is true, we're dealing with an invocation of `s2i build ... --run`
			// so this will, e.g., run a web server and block until the user interrupts it (or
			// the container exits normally).  dump port/etc information for the user.
			dumpContainerInfo(container, d, image)
		}
		// Return an error if the exit code of the container is
		// non-zero.
		glog.V(4).Infof("Waiting for container %q to stop ...", containerName)
		exitCode, err := d.client.WaitContainer(container.ID)
		if err != nil {
			return fmt.Errorf("waiting for container %q to stop: %v", containerName, err)
		}
		if exitCode != 0 {
			return errors.NewContainerError(container.Name, exitCode, "")
		}

		// FIXME: If Stdout or Stderr can be closed, close it to notify that
		// there won't be any more writes. This is a hack to close the write
		// half of a pipe so that the read half sees io.EOF.
		// In particular, this is needed to eventually terminate code that runs
		// on OnStart and blocks reading from the pipe.
		if c, ok := opts.Stdout.(io.Closer); ok {
			c.Close()
		}
		if c, ok := opts.Stderr.(io.Closer); ok {
			c.Close()
		}

		// OnStart must be done before we move on.
		if opts.OnStart != nil {
			if err = <-onStartDone; err != nil {
				return err
			}
		}
		// Run PostExec hook if defined.
		if opts.PostExec != nil {
			glog.V(2).Infof("Invoking PostExecute function")
			if err = opts.PostExec.PostExecute(container.ID, tarDestination); err != nil {
				return err
			}
		}

		return nil
	})

}

// containerName returns the name of a container or its ID if the name is empty.
// Useful for identifying a container in logs.
func containerNameOrID(c *docker.Container) string {
	if c.Name != "" {
		return c.Name
	}
	return c.ID
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
	if opts.Command != nil || opts.Entrypoint != nil {
		config := docker.Config{
			Cmd:        opts.Command,
			Entrypoint: opts.Entrypoint,
			Env:        opts.Env,
			Labels:     opts.Labels,
			User:       opts.User,
		}
		dockerOpts.Run = &config
		glog.V(2).Infof("Committing container with dockerOpts: %+v, config: %+v", dockerOpts, config)
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
	if opts.CGroupLimits != nil {
		dockerOpts.Memory = opts.CGroupLimits.MemoryLimitBytes
		dockerOpts.Memswap = opts.CGroupLimits.MemorySwap
		dockerOpts.CPUShares = opts.CGroupLimits.CPUShares
		dockerOpts.CPUPeriod = opts.CGroupLimits.CPUPeriod
		dockerOpts.CPUQuota = opts.CGroupLimits.CPUQuota
	}
	glog.V(2).Infof("Building container using config: %+v", dockerOpts)
	return d.client.BuildImage(dockerOpts)
}
