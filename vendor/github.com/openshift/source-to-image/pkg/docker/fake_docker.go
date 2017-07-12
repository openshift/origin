package docker

import (
	"errors"
	"io"
	"io/ioutil"

	dockertypes "github.com/docker/engine-api/types"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util/fs"
)

// FakeDocker provides a fake docker interface
type FakeDocker struct {
	LocalRegistryImage           string
	LocalRegistryResult          bool
	LocalRegistryError           error
	RemoveContainerID            string
	RemoveContainerError         error
	DefaultURLImage              string
	DefaultURLResult             string
	DefaultURLError              error
	AssembleInputFilesResult     string
	AssembleInputFilesError      error
	RunContainerOpts             RunContainerOptions
	RunContainerError            error
	RunContainerErrorBeforeStart bool
	RunContainerContainerID      string
	RunContainerCmd              []string
	GetImageIDImage              string
	GetImageIDResult             string
	GetImageIDError              error
	GetImageUserImage            string
	GetImageUserResult           string
	GetImageUserError            error
	GetImageEntrypointResult     []string
	GetImageEntrypointError      error
	CommitContainerOpts          CommitContainerOptions
	CommitContainerResult        string
	CommitContainerError         error
	RemoveImageName              string
	RemoveImageError             error
	BuildImageOpts               BuildImageOptions
	BuildImageError              error
	PullResult                   bool
	PullError                    error
	OnBuildImage                 string
	OnBuildResult                []string
	OnBuildError                 error
	IsOnBuildResult              bool
	IsOnBuildImage               string
	Labels                       map[string]string
	LabelsError                  error
}

// IsImageInLocalRegistry checks if the image exists in the fake local registry
func (f *FakeDocker) IsImageInLocalRegistry(imageName string) (bool, error) {
	f.LocalRegistryImage = imageName
	return f.LocalRegistryResult, f.LocalRegistryError
}

// IsImageOnBuild  returns true if the builder has onbuild instructions
func (f *FakeDocker) IsImageOnBuild(imageName string) bool {
	f.IsOnBuildImage = imageName
	return f.IsOnBuildResult
}

// Version returns information of the docker client and server host
func (f *FakeDocker) Version() (dockertypes.Version, error) {
	return dockertypes.Version{}, nil
}

// GetImageWorkdir returns the workdir
func (f *FakeDocker) GetImageWorkdir(name string) (string, error) {
	return "/", nil
}

// GetOnBuild returns the list of onbuild instructions for the given image
func (f *FakeDocker) GetOnBuild(imageName string) ([]string, error) {
	f.OnBuildImage = imageName
	return f.OnBuildResult, f.OnBuildError
}

// RemoveContainer removes a fake Docker container
func (f *FakeDocker) RemoveContainer(id string) error {
	f.RemoveContainerID = id
	return f.RemoveContainerError
}

// KillContainer kills a fake container
func (f *FakeDocker) KillContainer(id string) error {
	return nil
}

// GetScriptsURL returns a default STI scripts URL
func (f *FakeDocker) GetScriptsURL(image string) (string, error) {
	f.DefaultURLImage = image
	return f.DefaultURLResult, f.DefaultURLError
}

// GetAssembleInputFiles finds a io.openshift.s2i.assemble-input-files label on the given image.
func (f *FakeDocker) GetAssembleInputFiles(image string) (string, error) {
	return f.AssembleInputFilesResult, f.AssembleInputFilesError
}

// RunContainer runs a fake Docker container
func (f *FakeDocker) RunContainer(opts RunContainerOptions) error {
	f.RunContainerOpts = opts
	if f.RunContainerErrorBeforeStart {
		return f.RunContainerError
	}
	if opts.Stdout != nil {
		opts.Stdout.Close()
	}
	if opts.Stderr != nil {
		opts.Stderr.Close()
	}
	if opts.OnStart != nil {
		if err := opts.OnStart(""); err != nil {
			return err
		}
	}
	if opts.Stdin != nil {
		_, err := io.Copy(ioutil.Discard, opts.Stdin)
		if err != nil {
			return err
		}
	}
	if opts.PostExec != nil {
		opts.PostExec.PostExecute(f.RunContainerContainerID, string(opts.Command))
	}
	return f.RunContainerError
}

// UploadToContainer uploads artifacts to the container.
func (f *FakeDocker) UploadToContainer(fs fs.FileSystem, srcPath, destPath, container string) error {
	return nil
}

// UploadToContainerWithTarWriter uploads artifacts to the container.
func (f *FakeDocker) UploadToContainerWithTarWriter(fs fs.FileSystem, srcPath, destPath, container string, makeTarWriter func(io.Writer) tar.Writer) error {
	return errors.New("not implemented")
}

// DownloadFromContainer downloads file (or directory) from the container.
func (f *FakeDocker) DownloadFromContainer(containerPath string, w io.Writer, container string) error {
	return errors.New("not implemented")
}

// GetImageID returns a fake Docker image ID
func (f *FakeDocker) GetImageID(image string) (string, error) {
	f.GetImageIDImage = image
	return f.GetImageIDResult, f.GetImageIDError
}

// GetImageUser returns a fake user
func (f *FakeDocker) GetImageUser(image string) (string, error) {
	f.GetImageUserImage = image
	return f.GetImageUserResult, f.GetImageUserError
}

// GetImageEntrypoint returns an empty entrypoint
func (f *FakeDocker) GetImageEntrypoint(image string) ([]string, error) {
	return f.GetImageEntrypointResult, f.GetImageEntrypointError
}

// CommitContainer commits a fake Docker container
func (f *FakeDocker) CommitContainer(opts CommitContainerOptions) (string, error) {
	f.CommitContainerOpts = opts
	return f.CommitContainerResult, f.CommitContainerError
}

// RemoveImage removes a fake Docker image
func (f *FakeDocker) RemoveImage(name string) error {
	f.RemoveImageName = name
	return f.RemoveImageError
}

// CheckImage checks image in local registry
func (f *FakeDocker) CheckImage(name string) (*api.Image, error) {
	return nil, nil
}

// PullImage pulls a fake docker image
func (f *FakeDocker) PullImage(imageName string) (*api.Image, error) {
	if f.PullResult {
		return &api.Image{}, nil
	}
	return nil, f.PullError
}

// CheckAndPullImage pulls a fake docker image
func (f *FakeDocker) CheckAndPullImage(name string) (*api.Image, error) {
	if f.PullResult {
		return &api.Image{}, nil
	}
	return nil, f.PullError
}

// BuildImage builds image
func (f *FakeDocker) BuildImage(opts BuildImageOptions) error {
	f.BuildImageOpts = opts
	if opts.Stdin != nil {
		_, err := io.Copy(ioutil.Discard, opts.Stdin)
		if err != nil {
			return err
		}
	}
	return f.BuildImageError
}

// GetLabels returns the labels of the image
func (f *FakeDocker) GetLabels(name string) (map[string]string, error) {
	return f.Labels, f.LabelsError
}

// CheckReachable returns if the Docker daemon is reachable from s2i
func (f *FakeDocker) CheckReachable() error {
	return nil
}
