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
	"github.com/openshift/source-to-image/pkg/ignore"
	"github.com/openshift/source-to-image/pkg/scripts"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util"
)

// OnBuild strategy executes the simple Docker build in case the image does not
// support STI scripts but has ONBUILD instructions recorded.
type OnBuild struct {
	docker  docker.Docker
	git     git.Git
	fs      util.FileSystem
	tar     tar.Tar
	source  build.SourceHandler
	garbage build.Cleaner
}

type onBuildSourceHandler struct {
	build.Downloader
	build.Preparer
	build.Ignorer
}

// New returns a new instance of OnBuild builder
func New(config *api.Config) (*OnBuild, error) {
	dockerHandler, err := docker.New(config.DockerConfig, config.PullAuthentication)
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
	s, err := sti.New(config)
	s.SetScripts([]string{}, []string{api.Assemble, api.Run})

	b.source = onBuildSourceHandler{
		&git.Clone{b.git, b.fs},
		s,
		&ignore.DockerIgnorer{},
	}
	b.garbage = &build.DefaultCleaner{b.fs, b.docker}
	return b, nil
}

// SourceTar produces a tar archive containing application source and stream it
func (b *OnBuild) SourceTar(config *api.Config) (io.ReadCloser, error) {
	uploadDir := filepath.Join(config.WorkingDir, "upload", "src")
	tarFileName, err := b.tar.CreateTarFile(config.WorkingDir, uploadDir)
	if err != nil {
		return nil, err
	}
	return b.fs.Open(tarFileName)
}

// Build executes the ONBUILD kind of build
func (b *OnBuild) Build(config *api.Config) (*api.Result, error) {
	glog.V(2).Info("Preparing the source code for build")
	// Change the installation directory for this config to store scripts inside
	// the application root directory.
	if err := b.source.Prepare(config); err != nil {
		return nil, err
	}

	// If necessary, copy the STI scripts into application root directory
	b.copySTIScripts(config)

	glog.V(2).Info("Creating application Dockerfile")
	if err := b.CreateDockerfile(config); err != nil {
		return nil, err
	}

	glog.V(2).Info("Creating application source code image")
	tarStream, err := b.SourceTar(config)
	if err != nil {
		return nil, err
	}
	defer tarStream.Close()

	opts := docker.BuildImageOptions{
		Name:   config.Tag,
		Stdin:  tarStream,
		Stdout: os.Stdout,
	}

	glog.V(2).Info("Building the application source")
	if err := b.docker.BuildImage(opts); err != nil {
		return nil, err
	}

	glog.V(2).Info("Cleaning up temporary containers")
	b.garbage.Cleanup(config)

	var imageID string

	if len(opts.Name) > 0 {
		if imageID, err = b.docker.GetImageID(opts.Name); err != nil {
			return nil, err
		}
	}

	return &api.Result{
		Success:    true,
		WorkingDir: config.WorkingDir,
		ImageID:    imageID,
	}, nil
}

// CreateDockerfile creates the ONBUILD Dockerfile
func (b *OnBuild) CreateDockerfile(config *api.Config) error {
	buffer := bytes.Buffer{}
	uploadDir := filepath.Join(config.WorkingDir, "upload", "src")
	buffer.WriteString(fmt.Sprintf("FROM %s\n", config.BuilderImage))
	entrypoint, err := GuessEntrypoint(b.fs, uploadDir)
	if err != nil {
		return err
	}
	env, err := scripts.GetEnvironment(config)
	if err != nil {
		glog.V(1).Infof("Environment: %v", err)
	} else {
		buffer.WriteString(scripts.ConvertEnvironmentToDocker(env))
	}
	// If there is an assemble script present, run it as part of the build process
	// as the last thing.
	if b.hasAssembleScript(config) {
		buffer.WriteString(fmt.Sprintf("RUN sh assemble\n"))
	}
	// FIXME: This assumes that the WORKDIR is set to the application source root
	//        directory.
	buffer.WriteString(fmt.Sprintf(`ENTRYPOINT ["./%s"]`+"\n", entrypoint))
	return b.fs.WriteFile(filepath.Join(uploadDir, "Dockerfile"), buffer.Bytes())
}

func (b *OnBuild) copySTIScripts(config *api.Config) {
	scriptsPath := filepath.Join(config.WorkingDir, "upload", "scripts")
	sourcePath := filepath.Join(config.WorkingDir, "upload", "src")
	if _, err := b.fs.Stat(filepath.Join(scriptsPath, api.Run)); err == nil {
		glog.V(3).Infof("Found S2I 'run' script, copying to application source dir")
		b.fs.Copy(filepath.Join(scriptsPath, api.Run), sourcePath)
	}
	if _, err := b.fs.Stat(filepath.Join(scriptsPath, api.Assemble)); err == nil {
		glog.V(3).Infof("Found S2I 'assemble' script, copying to application source dir")
		b.fs.Copy(filepath.Join(scriptsPath, api.Assemble), sourcePath)
	}
}

// hasAssembleScript checks if the the assemble script is available
func (b *OnBuild) hasAssembleScript(config *api.Config) bool {
	assemblePath := filepath.Join(config.WorkingDir, "upload", "src", "assemble")
	_, err := b.fs.Stat(assemblePath)
	return err == nil
}
