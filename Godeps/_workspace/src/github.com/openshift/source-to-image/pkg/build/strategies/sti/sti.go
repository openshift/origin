package sti

import (
	"bufio"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/build/strategies/layered"
	dockerpkg "github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/ignore"
	"github.com/openshift/source-to-image/pkg/scm"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/scripts"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util"
)

var (
	// List of directories that needs to be present inside working dir
	workingDirs = []string{
		api.UploadScripts,
		api.Source,
		api.DefaultScripts,
		api.UserScripts,
	}
)

// STI strategy executes the STI build.
// For more details about STI, visit https://github.com/openshift/source-to-image
type STI struct {
	config            *api.Config
	result            *api.Result
	postExecutor      dockerpkg.PostExecutor
	installer         scripts.Installer
	git               git.Git
	fs                util.FileSystem
	tar               tar.Tar
	docker            dockerpkg.Docker
	incrementalDocker dockerpkg.Docker
	callbackInvoker   util.CallbackInvoker
	requiredScripts   []string
	optionalScripts   []string
	externalScripts   map[string]bool
	installedScripts  map[string]bool
	scriptsURL        map[string]string
	incremental       bool
	sourceInfo        *api.SourceInfo

	// Interfaces
	preparer  build.Preparer
	ignorer   build.Ignorer
	artifacts build.IncrementalBuilder
	scripts   build.ScriptsHandler
	source    build.Downloader
	garbage   build.Cleaner
	layered   build.Builder
}

// New returns the instance of STI builder strategy for the given config.
// If the layeredBuilder parameter is specified, then the builder provided will
// be used for the case that the base Docker image does not have 'tar' or 'bash'
// installed.
func New(req *api.Config, overrides build.Overrides) (*STI, error) {
	docker, err := dockerpkg.New(req.DockerConfig, req.PullAuthentication)
	if err != nil {
		return nil, err
	}
	var incrementalDocker dockerpkg.Docker
	if req.Incremental {
		incrementalDocker, err = dockerpkg.New(req.DockerConfig, req.IncrementalAuthentication)
		if err != nil {
			return nil, err
		}
	}

	inst := scripts.NewInstaller(req.BuilderImage, req.ScriptsURL, docker, req.PullAuthentication)

	b := &STI{
		installer:         inst,
		config:            req,
		docker:            docker,
		incrementalDocker: incrementalDocker,
		git:               git.New(),
		fs:                util.NewFileSystem(),
		tar:               tar.New(),
		callbackInvoker:   util.NewCallbackInvoker(),
		requiredScripts:   []string{api.Assemble, api.Run},
		optionalScripts:   []string{api.SaveArtifacts},
		externalScripts:   map[string]bool{},
		installedScripts:  map[string]bool{},
		scriptsURL:        map[string]string{},
	}

	// The sources are downloaded using the GIT downloader.
	// TODO: Add more SCM in future.
	b.source = overrides.Downloader
	if b.source == nil {
		downloader, sourceURL, err := scm.DownloaderForSource(req.Source)
		if err != nil {
			return nil, err
		}
		b.source = downloader
		req.Source = sourceURL
	}

	b.garbage = &build.DefaultCleaner{b.fs, b.docker}
	b.layered, err = layered.New(req, b, overrides)

	// Set interfaces
	b.preparer = b
	// later on, if we support say .gitignore func in addition to .dockerignore func, setting
	// ignorer will be based on config setting
	b.ignorer = &ignore.DockerIgnorer{}
	b.artifacts = b
	b.scripts = b
	b.postExecutor = b
	return b, err
}

// Build processes a Request and returns a *api.Result and an error.
// An error represents a failure performing the build rather than a failure
// of the build itself.  Callers should check the Success field of the result
// to determine whether a build succeeded or not.
func (b *STI) Build(config *api.Config) (*api.Result, error) {
	defer b.garbage.Cleanup(config)

	glog.V(1).Infof("Preparing to build %s", config.Tag)
	if err := b.preparer.Prepare(config); err != nil {
		return nil, err
	}

	if b.incremental = b.artifacts.Exists(config); b.incremental {
		glog.V(1).Infof("Existing image for tag %s detected for incremental build", config.Tag)
	} else {
		glog.V(1).Infof("Clean build will be performed")
	}

	glog.V(2).Infof("Performing source build from %s", config.Source)
	if b.incremental {
		if err := b.artifacts.Save(config); err != nil {
			glog.Warningf("Clean build will be performed because of error saving previous build artifacts")
			if glog.V(2) {
				glog.Infof("ERROR: %v", err)
			}
		}
	}

	glog.V(1).Infof("Running S2I script in %s", config.Tag)
	if err := b.scripts.Execute(api.Assemble, config); err != nil {
		switch e := err.(type) {
		case errors.ContainerError:
			if !isMissingRequirements(e.Output) {
				return nil, err
			}
			glog.V(1).Info("Image is missing basic requirements (sh or tar), layered build will be performed")
			return b.layered.Build(config)
		default:
			return nil, err
		}
	}

	return b.result, nil
}

// Prepare prepares the source code and tar for build
// NOTE, this func serves both the sti and onbuild strategies, as the OnBuild
// struct Build func leverages the STI struct Prepare func directly below
func (b *STI) Prepare(config *api.Config) error {
	var err error
	if len(config.WorkingDir) == 0 {
		if config.WorkingDir, err = b.fs.CreateWorkingDirectory(); err != nil {
			return err
		}
	}

	b.result = &api.Result{
		Success:    false,
		WorkingDir: config.WorkingDir,
	}

	// Setup working directories
	for _, v := range workingDirs {
		if err := b.fs.MkdirAll(filepath.Join(config.WorkingDir, v)); err != nil {
			return err
		}
	}

	// fetch sources, for theirs .sti/bin might contain sti scripts
	if len(config.Source) > 0 {
		if b.sourceInfo, err = b.source.Download(config); err != nil {
			return err
		}
	}

	// get the scripts
	required, err := b.installer.InstallRequired(b.requiredScripts, config.WorkingDir)
	if err != nil {
		return err
	}
	optional := b.installer.InstallOptional(b.optionalScripts, config.WorkingDir)

	for _, r := range append(required, optional...) {
		if r.Error == nil {
			glog.V(1).Infof("Using %v from %s", r.Script, r.URL)
			b.externalScripts[r.Script] = r.Downloaded
			b.installedScripts[r.Script] = r.Installed
			b.scriptsURL[r.Script] = r.URL
		} else {
			glog.Warningf("Error getting %v from %s: %v", r.Script, r.URL, r.Error)
		}
	}

	// see if there is a .s2iignore file, and if so, read in the patterns an then search and delete on
	return b.ignorer.Ignore(config)
}

// SetScripts allows to override default required and optional scripts
func (b *STI) SetScripts(required, optional []string) {
	b.requiredScripts = required
	b.optionalScripts = optional
}

func mergeLabels(newLabels, existingLabels map[string]string) map[string]string {
	if existingLabels == nil {
		return newLabels
	}
	result := map[string]string{}
	for k, v := range existingLabels {
		result[k] = v
	}
	for k, v := range newLabels {
		result[k] = v
	}
	return result
}

// PostExecute allows to execute post-build actions after the Docker build
// finishes.
func (b *STI) PostExecute(containerID, location string) error {
	var (
		err             error
		previousImageID string
	)

	if b.incremental && b.config.RemovePreviousImage {
		if previousImageID, err = b.docker.GetImageID(b.config.Tag); err != nil {
			glog.Errorf("Error retrieving previous image's metadata: %v", err)
		}
	}

	env, err := scripts.GetEnvironment(b.config)
	if err != nil {
		glog.V(1).Infof("No .sti/environment provided (%v)", err)
	}

	buildEnv := append(scripts.ConvertEnvironment(env), b.generateConfigEnv()...)

	runCmd := b.scriptsURL[api.Run]
	if strings.HasPrefix(runCmd, "image://") {
		// scripts from inside of the image, we need to strip the image part
		runCmd = filepath.Join(strings.TrimPrefix(runCmd, "image://"), api.Run)
	} else {
		// external scripts, in which case we're taking the directory to which they
		// were extracted and append scripts dir and name
		runCmd = filepath.Join(location, "scripts", api.Run)
	}
	existingLabels, err := b.docker.GetLabels(b.config.BuilderImage)
	if err != nil {
		glog.Errorf("Unable to read existing labels from current builder image %s", b.config.BuilderImage)
	}

	opts := dockerpkg.CommitContainerOptions{
		Command:     append([]string{}, runCmd),
		Env:         buildEnv,
		ContainerID: containerID,
		Repository:  b.config.Tag,
		Labels:      mergeLabels(util.GenerateOutputImageLabels(b.sourceInfo, b.config), existingLabels),
	}

	imageID, err := b.docker.CommitContainer(opts)
	if err != nil {
		return errors.NewBuildError(b.config.Tag, err)
	}

	b.result.Success = true
	b.result.ImageID = imageID

	if len(b.config.Tag) > 0 {
		glog.V(1).Infof("Successfully built %s", b.config.Tag)
	} else {
		glog.V(1).Infof("Successfully built %s", imageID)
	}

	if b.incremental && b.config.RemovePreviousImage && previousImageID != "" {
		glog.V(1).Infof("Removing previously-tagged image %s", previousImageID)
		if err = b.docker.RemoveImage(previousImageID); err != nil {
			glog.Errorf("Unable to remove previous image: %v", err)
		}
	}

	if b.config.CallbackURL != "" {
		b.result.Messages = b.callbackInvoker.ExecuteCallback(b.config.CallbackURL,
			b.result.Success, b.result.Messages)
	}

	return nil
}

// Exists determines if the current build supports incremental workflow.
// It checks if the previous image exists in the system and if so, then it
// verifies that the save-artifacts script is present.
func (b *STI) Exists(config *api.Config) bool {
	if !config.Incremental {
		return false
	}

	// can only do incremental build if runtime image exists, so always pull image
	previousImageExists, _ := b.docker.IsImageInLocalRegistry(config.Tag)
	if !previousImageExists || config.ForcePull {
		if image, _ := b.incrementalDocker.PullImage(config.Tag); image != nil {
			previousImageExists = true
		}
	}

	return previousImageExists && b.installedScripts[api.SaveArtifacts]
}

// Save extracts and restores the build artifacts from the previous build to a
// current build.
func (b *STI) Save(config *api.Config) (err error) {
	artifactTmpDir := filepath.Join(config.WorkingDir, "upload", "artifacts")
	if err = b.fs.Mkdir(artifactTmpDir); err != nil {
		return err
	}

	image := config.Tag
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	defer errReader.Close()
	defer errWriter.Close()
	glog.V(1).Infof("Saving build artifacts from image %s to path %s", image, artifactTmpDir)
	extractFunc := func() error {
		defer outReader.Close()
		return b.tar.ExtractTarStream(artifactTmpDir, outReader)
	}

	opts := dockerpkg.RunContainerOptions{
		Image:           image,
		ExternalScripts: b.externalScripts[api.SaveArtifacts],
		ScriptsURL:      config.ScriptsURL,
		Destination:     config.Destination,
		Command:         api.SaveArtifacts,
		Stdout:          outWriter,
		Stderr:          errWriter,
		OnStart:         extractFunc,
	}

	go dockerpkg.StreamContainerIO(errReader, nil, glog.Error)
	err = b.docker.RunContainer(opts)

	if e, ok := err.(errors.ContainerError); ok {
		return errors.NewSaveArtifactsError(image, e.Output, err)
	}
	return err
}

// Execute runs the specified STI script in the builder image.
func (b *STI) Execute(command string, config *api.Config) error {
	glog.V(2).Infof("Using image name %s", config.BuilderImage)

	env, err := scripts.GetEnvironment(config)
	if err != nil {
		glog.V(1).Infof("No .sti/environment provided (%v)", err)
	}

	buildEnv := append(scripts.ConvertEnvironment(env), b.generateConfigEnv()...)

	errOutput := ""
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	defer outReader.Close()
	defer outWriter.Close()
	defer errReader.Close()
	defer errWriter.Close()
	externalScripts := b.externalScripts[command]
	// if LayeredBuild is called then all the scripts will be placed inside the image
	if config.LayeredBuild {
		externalScripts = false
	}
	opts := dockerpkg.RunContainerOptions{
		Image:           config.BuilderImage,
		Stdout:          outWriter,
		Stderr:          errWriter,
		PullImage:       config.ForcePull,
		ExternalScripts: externalScripts,
		ScriptsURL:      config.ScriptsURL,
		Destination:     config.Destination,
		Command:         command,
		Env:             buildEnv,
		PostExec:        b.postExecutor,
	}

	if !config.LayeredBuild {
		wg := sync.WaitGroup{}
		wg.Add(1)
		uploadDir := filepath.Join(config.WorkingDir, "upload")

		// TODO: be able to pass a stream directly to the Docker build to avoid the double temp hit
		r, w := io.Pipe()
		go func() {
			var err error
			defer func() {
				w.CloseWithError(err)
				if r := recover(); r != nil {
					glog.Errorf("recovered panic: %#v", r)
				}
				wg.Done()
			}()
			err = b.tar.CreateTarStream(uploadDir, false, w)
		}()

		opts.Stdin = r
		defer wg.Wait()
	}

	go func(reader io.Reader) {
		scanner := bufio.NewReader(reader)
		for {
			text, err := scanner.ReadString('\n')
			if err != nil {
				// we're ignoring ErrClosedPipe, as this is information
				// the docker container ended streaming logs
				if glog.V(2) && err != io.ErrClosedPipe {
					glog.Errorf("Error reading docker stdout, %v", err)
				}
				break
			}
			if glog.V(2) || config.Quiet != true || command == api.Usage {
				glog.Info(text)
			}
		}
	}(outReader)

	go dockerpkg.StreamContainerIO(errReader, &errOutput, glog.Error)

	err = b.docker.RunContainer(opts)
	if e, ok := err.(errors.ContainerError); ok {
		return errors.NewContainerError(config.BuilderImage, e.ErrorCode, errOutput)
	}
	return err
}

func (b *STI) generateConfigEnv() (configEnv []string) {
	if len(b.config.Environment) > 0 {
		for key, val := range b.config.Environment {
			configEnv = append(configEnv, key+"="+val)
		}
	}
	return
}

func isMissingRequirements(text string) bool {
	tar, _ := regexp.MatchString(`.*tar.*not found`, text)
	sh, _ := regexp.MatchString(`.*/bin/sh.*no such file or directory`, text)
	return tar || sh
}
