package onbuild

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/build/strategies/sti"
	"github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/ignore"
	"github.com/openshift/source-to-image/pkg/scm"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/scripts"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util/cmd"
	"github.com/openshift/source-to-image/pkg/util/fs"
	utilstatus "github.com/openshift/source-to-image/pkg/util/status"
)

// OnBuild strategy executes the simple Docker build in case the image does not
// support STI scripts but has ONBUILD instructions recorded.
type OnBuild struct {
	docker  docker.Docker
	git     git.Git
	fs      fs.FileSystem
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
func New(client docker.Client, config *api.Config, fs fs.FileSystem, overrides build.Overrides) (*OnBuild, error) {
	dockerHandler := docker.New(client, config.PullAuthentication)
	builder := &OnBuild{
		docker: dockerHandler,
		git:    git.New(fs, cmd.NewCommandRunner()),
		fs:     fs,
		tar:    tar.New(fs),
	}
	// Use STI Prepare() and download the 'run' script optionally.
	s, err := sti.New(client, config, fs, overrides)
	if err != nil {
		return nil, err
	}
	s.SetScripts([]string{}, []string{api.Assemble, api.Run})

	downloader := overrides.Downloader
	if downloader == nil {
		downloader, err = scm.DownloaderForSource(builder.fs, config.Source, config.ForceCopy)
		if err != nil {
			return nil, err
		}
	}

	builder.source = onBuildSourceHandler{
		Downloader: downloader,
		Preparer:   s,
		Ignorer:    &ignore.DockerIgnorer{},
	}

	builder.garbage = build.NewDefaultCleaner(builder.fs, builder.docker)
	return builder, nil
}

// Build executes the ONBUILD kind of build
func (builder *OnBuild) Build(config *api.Config) (*api.Result, error) {
	buildResult := &api.Result{}

	if config.BlockOnBuild {
		buildResult.BuildInfo.FailureReason = utilstatus.NewFailureReason(
			utilstatus.ReasonOnBuildForbidden,
			utilstatus.ReasonMessageOnBuildForbidden,
		)
		return buildResult, fmt.Errorf("builder image uses ONBUILD instructions but ONBUILD is not allowed")
	}
	glog.V(2).Info("Preparing the source code for build")
	// Change the installation directory for this config to store scripts inside
	// the application root directory.
	if err := builder.source.Prepare(config); err != nil {
		return buildResult, err
	}

	// If necessary, copy the STI scripts into application root directory
	builder.copySTIScripts(config)

	glog.V(2).Info("Creating application Dockerfile")
	if err := builder.CreateDockerfile(config); err != nil {
		buildResult.BuildInfo.FailureReason = utilstatus.NewFailureReason(
			utilstatus.ReasonDockerfileCreateFailed,
			utilstatus.ReasonMessageDockerfileCreateFailed,
		)
		return buildResult, err
	}

	glog.V(2).Info("Creating application source code image")
	tarStream := builder.tar.CreateTarStreamReader(filepath.Join(config.WorkingDir, "upload", "src"), false)
	defer tarStream.Close()

	outReader, outWriter := io.Pipe()
	go io.Copy(os.Stdout, outReader)

	opts := docker.BuildImageOptions{
		Name:         config.Tag,
		Stdin:        tarStream,
		Stdout:       outWriter,
		CGroupLimits: config.CGroupLimits,
	}

	glog.V(2).Info("Building the application source")
	if err := builder.docker.BuildImage(opts); err != nil {
		buildResult.BuildInfo.FailureReason = utilstatus.NewFailureReason(
			utilstatus.ReasonDockerImageBuildFailed,
			utilstatus.ReasonMessageDockerImageBuildFailed,
		)
		return buildResult, err
	}

	glog.V(2).Info("Cleaning up temporary containers")
	builder.garbage.Cleanup(config)

	var imageID string
	var err error
	if len(opts.Name) > 0 {
		if imageID, err = builder.docker.GetImageID(opts.Name); err != nil {
			buildResult.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonGenericS2IBuildFailed,
				utilstatus.ReasonMessageGenericS2iBuildFailed,
			)
			return buildResult, err
		}
	}

	return &api.Result{
		Success:    true,
		WorkingDir: config.WorkingDir,
		ImageID:    imageID,
	}, nil
}

// CreateDockerfile creates the ONBUILD Dockerfile
func (builder *OnBuild) CreateDockerfile(config *api.Config) error {
	buffer := bytes.Buffer{}
	uploadDir := filepath.Join(config.WorkingDir, "upload", "src")
	buffer.WriteString(fmt.Sprintf("FROM %s\n", config.BuilderImage))
	entrypoint, err := GuessEntrypoint(builder.fs, uploadDir)
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
	if builder.hasAssembleScript(config) {
		buffer.WriteString("RUN sh assemble\n")
	}
	// FIXME: This assumes that the WORKDIR is set to the application source root
	//        directory.
	buffer.WriteString(fmt.Sprintf(`ENTRYPOINT ["./%s"]`+"\n", entrypoint))
	return builder.fs.WriteFile(filepath.Join(uploadDir, "Dockerfile"), buffer.Bytes())
}

func (builder *OnBuild) copySTIScripts(config *api.Config) {
	scriptsPath := filepath.Join(config.WorkingDir, "upload", "scripts")
	sourcePath := filepath.Join(config.WorkingDir, "upload", "src")
	if _, err := builder.fs.Stat(filepath.Join(scriptsPath, api.Run)); err == nil {
		glog.V(3).Info("Found S2I 'run' script, copying to application source dir")
		builder.fs.Copy(filepath.Join(scriptsPath, api.Run), filepath.Join(sourcePath, api.Run))
	}
	if _, err := builder.fs.Stat(filepath.Join(scriptsPath, api.Assemble)); err == nil {
		glog.V(3).Info("Found S2I 'assemble' script, copying to application source dir")
		builder.fs.Copy(filepath.Join(scriptsPath, api.Assemble), filepath.Join(sourcePath, api.Assemble))
	}
}

// hasAssembleScript checks if the the assemble script is available
func (builder *OnBuild) hasAssembleScript(config *api.Config) bool {
	assemblePath := filepath.Join(config.WorkingDir, "upload", "src", api.Assemble)
	_, err := builder.fs.Stat(assemblePath)
	return err == nil
}
