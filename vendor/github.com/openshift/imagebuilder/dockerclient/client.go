package dockerclient

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/docker/docker/builder/dockerfile/parser"
	dockertypes "github.com/docker/engine-api/types"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/archive"
	"github.com/fsouza/go-dockerclient/external/github.com/docker/docker/pkg/fileutils"
	"github.com/golang/glog"

	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/imageprogress"
)

// Mount represents a binding between the current system and the destination client
type Mount struct {
	SourcePath      string
	DestinationPath string
}

// ClientExecutor can run Docker builds from a Docker client.
type ClientExecutor struct {
	// Client is a client to a Docker daemon.
	Client *docker.Client
	// Directory is the context directory to build from, will use
	// the current working directory if not set.
	Directory string
	// Excludes are a list of file patterns that should be excluded
	// from the context. Will be set to the contents of the
	// .dockerignore file if nil.
	Excludes []string
	// Tag is an optional value to tag the resulting built image.
	Tag string
	// AllowPull when set will pull images that are not present on
	// the daemon.
	AllowPull bool
	// IgnoreUnrecognizedInstructions, if true, allows instructions
	// that are not yet supported to be ignored (will be printed)
	IgnoreUnrecognizedInstructions bool
	// TransientMounts are a set of mounts from outside the build
	// to the inside that will not be part of the final image. Any
	// content created inside the mount's destinationPath will be
	// omitted from the final image.
	TransientMounts []Mount

	Out, ErrOut io.Writer

	// Container is optional and can be set to a container to use as
	// the execution environment for a build.
	Container *docker.Container
	// Command, if set, will be used as the entrypoint for the new
	// container. This is ignored if Container is set.
	Command []string
	// Image is optional and may be set to control which image is used
	// as a base for this build. Otherwise the FROM value from the
	// Dockerfile is read (will be pulled if not locally present).
	Image *docker.Image

	// AuthFn will handle authenticating any docker pulls if Image
	// is set to nil.
	AuthFn func(name string) ([]dockertypes.AuthConfig, bool)
	// HostConfig is used to start the container (if necessary).
	HostConfig *docker.HostConfig
	// LogFn is an optional command to log information to the end user
	LogFn func(format string, args ...interface{})
}

// NewClientExecutor creates a client executor.
func NewClientExecutor(client *docker.Client) *ClientExecutor {
	return &ClientExecutor{
		Client: client,
		LogFn:  func(string, ...interface{}) {},
	}
}

// Build is a helper method to perform a Docker build against the
// provided Docker client. It will load the image if not specified,
// create a container if one does not already exist, and start a
// container if the Dockerfile contains RUN commands. It will cleanup
// any containers it creates directly, and set the e.Image.ID field
// to the generated image.
func (e *ClientExecutor) Build(r io.Reader, args map[string]string) error {
	b := imagebuilder.NewBuilder()
	b.Args = args

	if e.Excludes == nil {
		excludes, err := imagebuilder.ParseDockerignore(e.Directory)
		if err != nil {
			return err
		}
		e.Excludes = append(excludes, ".dockerignore")
	}

	// TODO: check the Docker daemon version (1.20 is required for Upload)

	node, err := parser.Parse(r)
	if err != nil {
		return err
	}

	// identify the base image
	from, err := b.From(node)
	if err != nil {
		return err
	}
	// load the image
	if e.Image == nil {
		if from == imagebuilder.NoBaseImageSpecifier {
			if runtime.GOOS == "windows" {
				return fmt.Errorf("building from scratch images is not supported")
			}
			from, err = e.CreateScratchImage()
			if err != nil {
				return fmt.Errorf("unable to create a scratch image for this build: %v", err)
			}
			defer e.CleanupImage(from)
		}
		glog.V(4).Infof("Retrieving image %q", from)
		e.Image, err = e.LoadImage(from)
		if err != nil {
			return err
		}
	}

	// update the builder with any information from the image, including ONBUILD
	// statements
	if err := b.FromImage(e.Image, node); err != nil {
		return err
	}

	b.RunConfig.Image = from
	e.LogFn("FROM %s", from)
	glog.V(4).Infof("step: FROM %s", from)

	var sharedMount string

	// create a container to execute in, if necessary
	mustStart := b.RequiresStart(node)
	if e.Container == nil {
		opts := docker.CreateContainerOptions{
			Config: &docker.Config{
				Image: from,
			},
		}
		if mustStart {
			// Transient mounts only make sense on images that will be running processes
			if len(e.TransientMounts) > 0 {
				volumeName, err := randSeq(imageSafeCharacters, 24)
				if err != nil {
					return err
				}
				v, err := e.Client.CreateVolume(docker.CreateVolumeOptions{Name: volumeName})
				if err != nil {
					return fmt.Errorf("unable to create volume to mount secrets: %v", err)
				}
				defer e.cleanupVolume(volumeName)
				sharedMount = v.Mountpoint
				opts.HostConfig = &docker.HostConfig{
					Binds: []string{sharedMount + ":/tmp/__temporarymount"},
				}
			}

			// TODO: windows support
			if len(e.Command) > 0 {
				opts.Config.Cmd = e.Command
				opts.Config.Entrypoint = nil
			} else {
				// TODO; replace me with a better default command
				opts.Config.Cmd = []string{"sleep 86400"}
				opts.Config.Entrypoint = []string{"/bin/sh", "-c"}
			}
		}
		if len(opts.Config.Cmd) == 0 {
			opts.Config.Entrypoint = []string{"/bin/sh", "-c", "# NOP"}
		}
		container, err := e.Client.CreateContainer(opts)
		if err != nil {
			return fmt.Errorf("unable to create build container: %v", err)
		}
		e.Container = container

		// if we create the container, take responsibilty for cleaning up
		defer e.Cleanup()
	}

	// copy any source content into the temporary mount path
	if mustStart && len(e.TransientMounts) > 0 {
		var copies []imagebuilder.Copy
		for i, mount := range e.TransientMounts {
			source := mount.SourcePath
			copies = append(copies, imagebuilder.Copy{
				Src:  source,
				Dest: []string{path.Join("/tmp/__temporarymount", strconv.Itoa(i))},
			})
		}
		if err := e.Copy(copies...); err != nil {
			return fmt.Errorf("unable to copy build context into container: %v", err)
		}
	}

	// TODO: lazy start
	if mustStart && !e.Container.State.Running {
		var hostConfig docker.HostConfig
		if e.HostConfig != nil {
			hostConfig = *e.HostConfig
		}

		// mount individual items temporarily
		for i, mount := range e.TransientMounts {
			if len(sharedMount) == 0 {
				return fmt.Errorf("no mount point available for temporary mounts")
			}
			hostConfig.Binds = append(
				hostConfig.Binds,
				fmt.Sprintf("%s:%s:%s", path.Join(sharedMount, strconv.Itoa(i)), mount.DestinationPath, "ro"),
			)
		}

		hc := &hostConfig
		if len(hostConfig.Binds) == 0 {
			// don't set HostConfig if we have no need for binds
			hc = nil
		}
		if err := e.Client.StartContainer(e.Container.ID, hc); err != nil {
			return fmt.Errorf("unable to start build container: %v", err)
		}
		// TODO: is this racy? may have to loop wait in the actual run step
	}

	for _, child := range node.Children {
		step := b.Step()
		if err := step.Resolve(child); err != nil {
			return err
		}
		glog.V(4).Infof("step: %s", step.Original)
		if e.LogFn != nil {
			e.LogFn(step.Original)
		}
		if err := b.Run(step, e); err != nil {
			return err
		}
	}

	if mustStart {
		glog.V(4).Infof("Stopping container %s ...", e.Container.ID)
		if err := e.Client.StopContainer(e.Container.ID, 0); err != nil {
			return fmt.Errorf("unable to stop build container: %v", err)
		}
	}

	config := b.Config()
	var repository, tag string
	if len(e.Tag) > 0 {
		repository, tag = docker.ParseRepositoryTag(e.Tag)
		glog.V(4).Infof("Committing built container %s as image %q: %#v", e.Container.ID, e.Tag, config)
		if e.LogFn != nil {
			e.LogFn("Committing changes to %s ...", e.Tag)
		}
	} else {
		glog.V(4).Infof("Committing built container %s: %#v", e.Container.ID, config)
		if e.LogFn != nil {
			e.LogFn("Committing changes ...")
		}
	}

	image, err := e.Client.CommitContainer(docker.CommitContainerOptions{
		Author:     b.Author,
		Container:  e.Container.ID,
		Run:        config,
		Repository: repository,
		Tag:        tag,
	})
	if err != nil {
		return fmt.Errorf("unable to commit build container: %v", err)
	}
	e.Image = image
	glog.V(4).Infof("Committed %s to %s", e.Container.ID, e.Image.ID)
	if e.LogFn != nil {
		e.LogFn("Done")
	}
	return nil
}

// Cleanup will remove the container that created the build.
func (e *ClientExecutor) Cleanup() error {
	if e.Container == nil {
		return nil
	}
	err := e.Client.RemoveContainer(docker.RemoveContainerOptions{
		ID:            e.Container.ID,
		RemoveVolumes: true,
		Force:         true,
	})
	if _, ok := err.(*docker.NoSuchContainer); err != nil && !ok {
		return fmt.Errorf("unable to cleanup build container: %v", err)
	}
	e.Container = nil
	return nil
}

// CreateScratchImage creates a new, zero byte layer that is identical to "scratch"
// except that the resulting image will have two layers.
func (e *ClientExecutor) CreateScratchImage() (string, error) {
	random, err := randSeq(imageSafeCharacters, 24)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("scratch%s", random)

	buf := &bytes.Buffer{}
	w := tar.NewWriter(buf)
	w.Close()

	return name, e.Client.ImportImage(docker.ImportImageOptions{
		Repository:  name,
		Source:      "-",
		InputStream: buf,
	})
}

// imageSafeCharacters are characters allowed to be part of a Docker image name.
const imageSafeCharacters = "abcdefghijklmnopqrstuvwxyz0123456789"

// randSeq returns a sequence of random characters drawn from source. It returns
// an error if cryptographic randomness is not available or source is more than 255
// characters.
func randSeq(source string, n int) (string, error) {
	if len(source) > 255 {
		return "", fmt.Errorf("source must be less than 256 bytes long")
	}
	random := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", err
	}
	for i := range random {
		random[i] = source[random[i]%byte(len(source))]
	}
	return string(random), nil
}

// cleanupVolume attempts to remove the provided volume
func (e *ClientExecutor) cleanupVolume(name string) error {
	return e.Client.RemoveVolume(name)
}

// CleanupImage attempts to remove the provided image.
func (e *ClientExecutor) CleanupImage(name string) error {
	return e.Client.RemoveImage(name)
}

// LoadImage checks the client for an image matching from. If not found,
// attempts to pull the image and then tries to inspect again.
func (e *ClientExecutor) LoadImage(from string) (*docker.Image, error) {
	image, err := e.Client.InspectImage(from)
	if err == nil {
		return image, nil
	}
	if err != docker.ErrNoSuchImage {
		return nil, err
	}

	if !e.AllowPull {
		glog.V(4).Infof("image %s did not exist", from)
		return nil, docker.ErrNoSuchImage
	}

	repository, tag := docker.ParseRepositoryTag(from)
	if len(tag) == 0 {
		tag = "latest"
	}

	glog.V(4).Infof("attempting to pull %s with auth from repository %s:%s", from, repository, tag)

	// TODO: we may want to abstract looping over multiple credentials
	auth, _ := e.AuthFn(repository)
	if len(auth) == 0 {
		auth = append(auth, dockertypes.AuthConfig{})
	}

	if e.LogFn != nil {
		e.LogFn("Image %s was not found, pulling ...", from)
	}

	var lastErr error
	outputProgress := func(s string) {
		e.LogFn("%s", s)
	}
	for _, config := range auth {
		// TODO: handle IDs?
		pullImageOptions := docker.PullImageOptions{
			Repository:    repository,
			Tag:           tag,
			OutputStream:  imageprogress.NewPullWriter(outputProgress),
			RawJSONStream: true,
		}
		if glog.V(5) {
			pullImageOptions.OutputStream = os.Stderr
			pullImageOptions.RawJSONStream = false
		}
		authConfig := docker.AuthConfiguration{Username: config.Username, ServerAddress: config.ServerAddress, Password: config.Password}
		if err = e.Client.PullImage(pullImageOptions, authConfig); err == nil {
			break
		}
		lastErr = err
		continue
	}
	if lastErr != nil {
		return nil, fmt.Errorf("unable to pull image (from: %s, tag: %s): %v", repository, tag, lastErr)
	}

	return e.Client.InspectImage(from)
}

func (e *ClientExecutor) UnrecognizedInstruction(step *imagebuilder.Step) error {
	if e.IgnoreUnrecognizedInstructions {
		e.LogFn("warning: Unknown instruction: %s", strings.ToUpper(step.Command))
		return nil
	}
	return fmt.Errorf("Unknown instruction: %s", strings.ToUpper(step.Command))
}

// Run executes a single Run command against the current container using exec().
// Since exec does not allow ENV or WORKINGDIR to be set, we force the execution of
// the user command into a shell and perform those operations before. Since RUN
// requires /bin/sh, we can use both 'cd' and 'export'.
func (e *ClientExecutor) Run(run imagebuilder.Run, config docker.Config) error {
	args := make([]string, len(run.Args))
	copy(args, run.Args)

	if runtime.GOOS == "windows" {
		if len(config.WorkingDir) > 0 {
			args[0] = fmt.Sprintf("cd %s && %s", imagebuilder.BashQuote(config.WorkingDir), args[0])
		}
		// TODO: implement windows ENV
		args = append([]string{"cmd", "/S", "/C"}, args...)
	} else {
		if len(config.WorkingDir) > 0 {
			args[0] = fmt.Sprintf("cd %s && %s", imagebuilder.BashQuote(config.WorkingDir), args[0])
		}
		if len(config.Env) > 0 {
			args[0] = imagebuilder.ExportEnv(config.Env) + args[0]
		}
		args = append([]string{"/bin/sh", "-c"}, args...)
	}

	config.Cmd = args

	exec, err := e.Client.CreateExec(docker.CreateExecOptions{
		Cmd:          config.Cmd,
		Container:    e.Container.ID,
		AttachStdout: true,
		AttachStderr: true,
		User:         config.User,
	})
	if err != nil {
		return err
	}
	if err := e.Client.StartExec(exec.ID, docker.StartExecOptions{
		OutputStream: e.Out,
		ErrorStream:  e.ErrOut,
	}); err != nil {
		return err
	}
	status, err := e.Client.InspectExec(exec.ID)
	if err != nil {
		return err
	}
	if status.ExitCode != 0 {
		return fmt.Errorf("running '%s' failed with exit code %d", strings.Join(args, " "), status.ExitCode)
	}
	return nil
}

func (e *ClientExecutor) Copy(copies ...imagebuilder.Copy) error {
	container := e.Container
	for _, c := range copies {
		// TODO: reuse source
		for _, dst := range c.Dest {
			glog.V(4).Infof("Archiving %s %t", c.Src, c.Download)
			r, closer, err := e.Archive(c.Src, dst, c.Download, c.Download)
			if err != nil {
				return err
			}
			glog.V(5).Infof("Uploading to %s at %s", container.ID, dst)
			err = e.Client.UploadToContainer(container.ID, docker.UploadToContainerOptions{
				InputStream: r,
				Path:        "/",
			})
			if err := closer.Close(); err != nil {
				glog.Errorf("Error while closing stream container copy stream %s: %v", container.ID, err)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type closers []func() error

func (c closers) Close() error {
	var lastErr error
	for _, fn := range c {
		if err := fn(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (e *ClientExecutor) Archive(src, dst string, allowDecompression, allowDownload bool) (io.Reader, io.Closer, error) {
	var closer closers
	var base string
	var infos []CopyInfo
	var err error
	if isURL(src) {
		if !allowDownload {
			return nil, nil, fmt.Errorf("source can't be a URL")
		}
		infos, base, err = DownloadURL(src, dst)
		if len(base) > 0 {
			closer = append(closer, func() error { return os.RemoveAll(base) })
		}
	} else {
		if filepath.IsAbs(src) {
			base = filepath.Dir(src)
			src, err = filepath.Rel(base, src)
			if err != nil {
				return nil, nil, err
			}
		} else {
			base = e.Directory
		}
		infos, err = CalcCopyInfo(src, base, allowDecompression, true)
	}
	if err != nil {
		closer.Close()
		return nil, nil, err
	}

	options := archiveOptionsFor(infos, dst, e.Excludes)

	glog.V(4).Infof("Tar of directory %s %#v", base, options)
	rc, err := archive.TarWithOptions(base, options)
	closer = append(closer, rc.Close)
	return rc, closer, err
}

func archiveOptionsFor(infos []CopyInfo, dst string, excludes []string) *archive.TarOptions {
	dst = trimLeadingPath(dst)
	patterns, patDirs, _, _ := fileutils.CleanPatterns(excludes)
	options := &archive.TarOptions{}
	for _, info := range infos {
		if ok, _ := fileutils.OptimizedMatches(info.Path, patterns, patDirs); ok {
			continue
		}
		options.IncludeFiles = append(options.IncludeFiles, info.Path)
		if len(dst) == 0 {
			continue
		}
		if options.RebaseNames == nil {
			options.RebaseNames = make(map[string]string)
		}
		if info.FromDir || strings.HasSuffix(dst, "/") || strings.HasSuffix(dst, "/.") || dst == "." {
			if strings.HasSuffix(info.Path, "/") {
				options.RebaseNames[info.Path] = dst
			} else {
				options.RebaseNames[info.Path] = path.Join(dst, path.Base(info.Path))
			}
		} else {
			options.RebaseNames[info.Path] = dst
		}
	}
	options.ExcludePatterns = excludes
	return options
}
