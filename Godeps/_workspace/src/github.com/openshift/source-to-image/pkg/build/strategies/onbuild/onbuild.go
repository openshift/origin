package onbuild

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/build/strategies/sti"
	"github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/git"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util"
)

// SourceHandler is a wrapper for STI strategy Downloader and Preparer which
// allows to use Download and Prepare functions from the STI strategy.
type SourceHandler struct {
	build.Downloader
	build.Preparer
}

// OnBuild strategy executes the simple Docker build in case the image does not
// support STI scripts but has ONBUILD instructions recorded.
type OnBuild struct {
	docker  docker.Docker
	git     git.Git
	fs      util.FileSystem
	tar     tar.Tar
	source  SourceHandler
	garbage build.Cleaner
}

// New returns a new instance of OnBuild builder
func New(request *api.Request) (*OnBuild, error) {
	dockerHandler, err := docker.New(request.DockerSocket)
	if err != nil {
		return nil, err
	}
	b := &OnBuild{
		docker: dockerHandler,
		git:    git.New(),
		fs:     util.NewFileSystem(),
		tar:    tar.New(),
	}
	// Use STI Prepare() and download the 'run' script optionally.
	request.InstallDestination = "upload/src"
	s, err := sti.New(request)
	s.SetScripts([]api.Script{}, []api.Script{api.Assemble, api.Run})

	b.source = SourceHandler{&git.Clone{b.git, b.fs}, s}
	b.garbage = &build.DefaultCleaner{b.fs, b.docker}
	return b, nil
}

// SourceTar produces a tar archive containing application source and stream it
func (b *OnBuild) SourceTar(request *api.Request) (io.ReadCloser, error) {
	uploadDir := filepath.Join(request.WorkingDir, "upload", "src")
	tarFileName, err := b.tar.CreateTarFile(request.WorkingDir, uploadDir)
	if err != nil {
		return nil, err
	}
	return b.fs.Open(tarFileName)
}

// Build executes the ONBUILD kind of build
func (b *OnBuild) Build(request *api.Request) (*api.Result, error) {
	glog.V(2).Info("Preparing the source code for build")
	// Change the installation directory for this request to store scripts inside
	// the application root directory.
	if err := b.source.Prepare(request); err != nil {
		return nil, err
	}

	glog.V(2).Info("Creating application Dockerfile")
	if err := b.CreateDockerfile(request); err != nil {
		return nil, err
	}

	glog.V(2).Info("Creating application source code image")
	tarStream, err := b.SourceTar(request)
	if err != nil {
		return nil, err
	}
	defer tarStream.Close()

	opts := docker.BuildImageOptions{
		Name:   request.Tag,
		Stdin:  tarStream,
		Stdout: os.Stdout,
	}

	glog.V(2).Info("Building the application source")
	if err := b.docker.BuildImage(opts); err != nil {
		return nil, err
	}

	glog.V(2).Info("Cleaning up temporary containers")
	b.garbage.Cleanup(request)

	imageID, err := b.docker.GetImageID(opts.Name)
	if err != nil {
		return nil, err
	}

	return &api.Result{
		Success:    true,
		WorkingDir: request.WorkingDir,
		ImageID:    imageID,
	}, nil
}

// CreateDockerfile creates the ONBUILD Dockerfile
func (b *OnBuild) CreateDockerfile(request *api.Request) error {
	buffer := bytes.Buffer{}
	uploadDir := filepath.Join(request.WorkingDir, "upload", "src")
	buffer.WriteString(fmt.Sprintf("FROM %s\n", request.BaseImage))
	entrypoint, err := GuessEntrypoint(uploadDir)
	if err != nil {
		return err
	}
	// If there is an assemble script present, run it as part of the build process
	// as the last thing.
	if b.hasAssembleScript(request) {
		buffer.WriteString(fmt.Sprintf("RUN sh assemble\n"))
	}
	// FIXME: This assumes that the WORKDIR is set to the application source root
	//        directory.
	buffer.WriteString(fmt.Sprintf(`ENTRYPOINT ["./%s"]`+"\n", entrypoint))
	return b.fs.WriteFile(filepath.Join(uploadDir, "Dockerfile"), buffer.Bytes())
}

// hasAssembleScript checks if the the assemble script is available
func (b *OnBuild) hasAssembleScript(request *api.Request) bool {
	assemblePath := filepath.Join(request.WorkingDir, "upload", "src", "assemble")
	_, err := os.Stat(assemblePath)
	return err == nil
}
