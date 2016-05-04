package sti

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
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

	inst := scripts.NewInstaller(req.BuilderImage, req.ScriptsURL, req.ScriptDownloadProxyConfig, docker, req.PullAuthentication)
	tarHandler := tar.New()
	tarHandler.SetExclusionPattern(regexp.MustCompile(req.ExcludeRegExp))

	builder := &STI{
		installer:         inst,
		config:            req,
		docker:            docker,
		incrementalDocker: incrementalDocker,
		git:               git.New(),
		fs:                util.NewFileSystem(),
		tar:               tarHandler,
		callbackInvoker:   util.NewCallbackInvoker(),
		requiredScripts:   []string{api.Assemble, api.Run},
		optionalScripts:   []string{api.SaveArtifacts},
		externalScripts:   map[string]bool{},
		installedScripts:  map[string]bool{},
		scriptsURL:        map[string]string{},
	}

	// The sources are downloaded using the Git downloader.
	// TODO: Add more SCM in future.
	// TODO: explicit decision made to customize processing for usage specifically vs.
	// leveraging overrides; also, we ultimately want to simplify s2i usage a good bit,
	// which would lead to replacing this quick short circuit (so this change is tactical)
	builder.source = overrides.Downloader
	if builder.source == nil && !req.Usage {
		downloader, sourceURL, err := scm.DownloaderForSource(req.Source, req.ForceCopy)
		if err != nil {
			return nil, err
		}
		builder.source = downloader
		req.Source = sourceURL
	}

	builder.garbage = &build.DefaultCleaner{builder.fs, builder.docker}
	builder.layered, err = layered.New(req, builder, overrides)

	// Set interfaces
	builder.preparer = builder
	// later on, if we support say .gitignore func in addition to .dockerignore func, setting
	// ignorer will be based on config setting
	builder.ignorer = &ignore.DockerIgnorer{}
	builder.artifacts = builder
	builder.scripts = builder
	builder.postExecutor = builder
	return builder, err
}

// Build processes a Request and returns a *api.Result and an error.
// An error represents a failure performing the build rather than a failure
// of the build itself.  Callers should check the Success field of the result
// to determine whether a build succeeded or not.
func (builder *STI) Build(config *api.Config) (*api.Result, error) {
	defer builder.garbage.Cleanup(config)

	glog.V(1).Infof("Preparing to build %s", config.Tag)
	if err := builder.preparer.Prepare(config); err != nil {
		return nil, err
	}

	if builder.incremental = builder.artifacts.Exists(config); builder.incremental {
		tag := config.IncrementalFromTag
		if len(tag) == 0 {
			tag = config.Tag
		}
		glog.V(1).Infof("Existing image for tag %s detected for incremental build", tag)
	} else {
		glog.V(1).Infof("Clean build will be performed")
	}

	glog.V(2).Infof("Performing source build from %s", config.Source)
	if builder.incremental {
		if err := builder.artifacts.Save(config); err != nil {
			glog.Warningf("Clean build will be performed because of error saving previous build artifacts")
			glog.V(2).Infof("error: %v", err)
		}
	}

	if len(config.AssembleUser) > 0 {
		glog.V(1).Infof("Running %q in %q as %q user", api.Assemble, config.Tag, config.AssembleUser)
	} else {
		glog.V(1).Infof("Running %q in %q", api.Assemble, config.Tag)
	}
	if err := builder.scripts.Execute(api.Assemble, config.AssembleUser, config); err != nil {
		switch e := err.(type) {
		case errors.ContainerError:
			if !isMissingRequirements(e.Output) {
				return nil, err
			}
			glog.V(1).Info("Image is missing basic requirements (sh or tar), layered build will be performed")
			return builder.layered.Build(config)
		default:
			return nil, err
		}
	}

	return builder.result, nil
}

// Prepare prepares the source code and tar for build
// NOTE, this func serves both the sti and onbuild strategies, as the OnBuild
// struct Build func leverages the STI struct Prepare func directly below
func (builder *STI) Prepare(config *api.Config) error {
	var err error
	if len(config.WorkingDir) == 0 {
		if config.WorkingDir, err = builder.fs.CreateWorkingDirectory(); err != nil {
			return err
		}
	}

	builder.result = &api.Result{
		Success:    false,
		WorkingDir: config.WorkingDir,
	}

	// Setup working directories
	for _, v := range workingDirs {
		if err := builder.fs.MkdirAll(filepath.Join(config.WorkingDir, v)); err != nil {
			return err
		}
	}

	// fetch sources, for their .sti/bin might contain sti scripts
	if len(config.Source) > 0 {
		if builder.sourceInfo, err = builder.source.Download(config); err != nil {
			return err
		}
	}

	// get the scripts
	required, err := builder.installer.InstallRequired(builder.requiredScripts, config.WorkingDir)
	if err != nil {
		return err
	}
	optional := builder.installer.InstallOptional(builder.optionalScripts, config.WorkingDir)

	// If a ScriptsURL was specified, but no scripts were downloaded from it, throw an error
	if len(config.ScriptsURL) > 0 {
		failedCount := 0
		for _, result := range required {
			if includes(result.FailedSources, scripts.ScriptURLHandler) {
				failedCount++
			}
		}
		for _, result := range optional {
			if includes(result.FailedSources, scripts.ScriptURLHandler) {
				failedCount++
			}
		}
		if failedCount == len(required)+len(optional) {
			return fmt.Errorf("Could not download any scripts from URL %v", config.ScriptsURL)
		}
	}

	for _, r := range append(required, optional...) {
		if r.Error == nil {
			builder.externalScripts[r.Script] = r.Downloaded
			builder.installedScripts[r.Script] = r.Installed
			builder.scriptsURL[r.Script] = r.URL
		} else {
			glog.Warningf("Error getting %v from %s: %v", r.Script, r.URL, r.Error)
		}
	}

	// see if there is a .s2iignore file, and if so, read in the patterns an then search and delete on
	return builder.ignorer.Ignore(config)
}

// SetScripts allows to override default required and optional scripts
func (builder *STI) SetScripts(required, optional []string) {
	builder.requiredScripts = required
	builder.optionalScripts = optional
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
func (builder *STI) PostExecute(containerID, location string) error {
	var (
		err             error
		previousImageID string
	)

	if builder.incremental && builder.config.RemovePreviousImage {
		if previousImageID, err = builder.docker.GetImageID(builder.config.Tag); err != nil {
			glog.Errorf("Error retrieving previous image's metadata: %v", err)
		}
	}

	env, err := scripts.GetEnvironment(builder.config)
	if err != nil {
		glog.V(1).Infof("No user environment provided (%v)", err)
	}

	buildEnv := append(scripts.ConvertEnvironment(env), builder.generateConfigEnv()...)

	runCmd := builder.scriptsURL[api.Run]
	if strings.HasPrefix(runCmd, "image://") {
		// scripts from inside of the image, we need to strip the image part
		// NOTE: We use path.Join instead of filepath.Join to avoid converting the
		// path to UNC (Windows) format as we always run this inside container.
		runCmd = strings.TrimPrefix(runCmd, "image://")
	} else {
		// external scripts, in which case we're taking the directory to which they
		// were extracted and append scripts dir and name
		runCmd = path.Join(location, "scripts", api.Run)
	}
	existingLabels, err := builder.docker.GetLabels(builder.config.BuilderImage)
	if err != nil {
		glog.Errorf("Unable to read existing labels from current builder image %s", builder.config.BuilderImage)
	}

	buildImageUser, err := builder.docker.GetImageUser(builder.config.BuilderImage)
	if err != nil {
		return err
	}

	resultLabels := mergeLabels(util.GenerateOutputImageLabels(builder.sourceInfo, builder.config), existingLabels)
	opts := dockerpkg.CommitContainerOptions{
		Command:     append([]string{}, runCmd),
		Env:         buildEnv,
		ContainerID: containerID,
		Repository:  builder.config.Tag,
		User:        buildImageUser,
		Labels:      resultLabels,
	}

	imageID, err := builder.docker.CommitContainer(opts)
	if err != nil {
		return errors.NewBuildError(builder.config.Tag, err)
	}

	builder.result.Success = true
	builder.result.ImageID = imageID

	if len(builder.config.Tag) > 0 {
		glog.V(1).Infof("Successfully built %s", builder.config.Tag)
	} else {
		glog.V(1).Infof("Successfully built %s", imageID)
	}

	if builder.incremental && builder.config.RemovePreviousImage && previousImageID != "" {
		glog.V(1).Infof("Removing previously-tagged image %s", previousImageID)
		if err = builder.docker.RemoveImage(previousImageID); err != nil {
			glog.Errorf("Unable to remove previous image: %v", err)
		}
	}

	if builder.config.CallbackURL != "" {
		builder.result.Messages = builder.callbackInvoker.ExecuteCallback(builder.config.CallbackURL,
			builder.result.Success, resultLabels, builder.result.Messages)
	}

	return nil
}

// Exists determines if the current build supports incremental workflow.
// It checks if the previous image exists in the system and if so, then it
// verifies that the save-artifacts script is present.
func (builder *STI) Exists(config *api.Config) bool {
	if !config.Incremental {
		return false
	}

	policy := config.PreviousImagePullPolicy
	if len(policy) == 0 {
		policy = api.DefaultPreviousImagePullPolicy
	}

	tag := config.IncrementalFromTag
	if len(tag) == 0 {
		tag = config.Tag
	}

	result, err := dockerpkg.PullImage(tag, builder.incrementalDocker, policy, false)
	if err != nil {
		glog.V(2).Infof("Unable to pull previously built image %q: %v", tag, err)
		return false
	}

	return result.Image != nil && builder.installedScripts[api.SaveArtifacts]
}

// Save extracts and restores the build artifacts from the previous build to a
// current build.
func (builder *STI) Save(config *api.Config) (err error) {
	artifactTmpDir := filepath.Join(config.WorkingDir, "upload", "artifacts")
	if err = builder.fs.Mkdir(artifactTmpDir); err != nil {
		return err
	}

	image := config.IncrementalFromTag
	if len(image) == 0 {
		image = config.Tag
	}
	outReader, outWriter := io.Pipe()
	defer outReader.Close()
	defer outWriter.Close()
	errReader, errWriter := io.Pipe()
	defer errReader.Close()
	defer errWriter.Close()
	glog.V(1).Infof("Saving build artifacts from image %s to path %s", image, artifactTmpDir)
	extractFunc := func(string) error {
		return builder.tar.ExtractTarStream(artifactTmpDir, outReader)
	}

	user := config.AssembleUser
	if len(user) == 0 {
		user, err = builder.docker.GetImageUser(image)
		if err != nil {
			return err
		}
		glog.V(3).Infof("The assemble user is not set, defaulting to %q user", user)
	} else {
		glog.V(3).Infof("Using assemble user %q to extract artifacts", user)
	}

	opts := dockerpkg.RunContainerOptions{
		Image:           image,
		User:            user,
		ExternalScripts: builder.externalScripts[api.SaveArtifacts],
		ScriptsURL:      config.ScriptsURL,
		Destination:     config.Destination,
		PullImage:       false,
		Command:         api.SaveArtifacts,
		Stdout:          outWriter,
		Stderr:          errWriter,
		OnStart:         extractFunc,
		NetworkMode:     string(config.DockerNetworkMode),
		CGroupLimits:    config.CGroupLimits,
		CapDrop:         config.DropCapabilities,
	}

	go dockerpkg.StreamContainerIO(errReader, nil, glog.Error)
	err = builder.docker.RunContainer(opts)
	if e, ok := err.(errors.ContainerError); ok {
		return errors.NewSaveArtifactsError(image, e.Output, err)
	}
	return err
}

// Execute runs the specified STI script in the builder image.
func (builder *STI) Execute(command string, user string, config *api.Config) error {
	glog.V(2).Infof("Using image name %s", config.BuilderImage)

	env, err := scripts.GetEnvironment(config)
	if err != nil {
		glog.V(1).Infof("No user environment provided (%v)", err)
	}

	buildEnv := append(scripts.ConvertEnvironment(env), builder.generateConfigEnv()...)

	errOutput := ""
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	defer outReader.Close()
	defer outWriter.Close()
	defer errReader.Close()
	defer errWriter.Close()
	externalScripts := builder.externalScripts[command]
	// if LayeredBuild is called then all the scripts will be placed inside the image
	if config.LayeredBuild {
		externalScripts = false
	}

	opts := dockerpkg.RunContainerOptions{
		Image:  config.BuilderImage,
		Stdout: outWriter,
		Stderr: errWriter,
		// The PullImage is false because the PullImage function should be called
		// before we run the container
		PullImage:       false,
		ExternalScripts: externalScripts,
		ScriptsURL:      config.ScriptsURL,
		Destination:     config.Destination,
		Command:         command,
		Env:             buildEnv,
		User:            user,
		PostExec:        builder.postExecutor,
		NetworkMode:     string(config.DockerNetworkMode),
		CGroupLimits:    config.CGroupLimits,
		CapDrop:         config.DropCapabilities,
	}

	// If there are injections specified, override the original assemble script
	// and wait till all injections are uploaded into the container that runs the
	// assemble script.
	injectionComplete := make(chan struct{})
	var injectionError error
	if len(config.Injections) > 0 && command == api.Assemble {
		workdir, err := builder.docker.GetImageWorkdir(config.BuilderImage)
		if err != nil {
			return err
		}
		util.FixInjectionsWithRelativePath(workdir, &config.Injections)
		injectedFiles, err := util.ExpandInjectedFiles(config.Injections)
		if err != nil {
			return err
		}
		rmScript, err := util.CreateInjectedFilesRemovalScript(injectedFiles, "/tmp/rm-injections")
		if err != nil {
			return err
		}
		defer os.Remove(rmScript)
		opts.CommandOverrides = func(cmd string) string {
			return fmt.Sprintf("while [ ! -f %q ]; do sleep 0.5; done; %s; result=$?; source %[1]s; exit $result",
				"/tmp/rm-injections", cmd)
		}
		originalOnStart := opts.OnStart
		opts.OnStart = func(containerID string) error {
			defer close(injectionComplete)
			if err != nil {
				injectionError = err
				return err
			}
			glog.V(2).Info("starting the injections uploading ...")
			for _, s := range config.Injections {
				if err := builder.docker.UploadToContainer(s.SourcePath, s.DestinationDir, containerID); err != nil {
					injectionError = util.HandleInjectionError(s, err)
					return err
				}
			}
			if err := builder.docker.UploadToContainer(rmScript, "/tmp/rm-injections", containerID); err != nil {
				injectionError = util.HandleInjectionError(api.InjectPath{SourcePath: rmScript, DestinationDir: "/tmp/rm-injections"}, err)
				return err
			}
			if originalOnStart != nil {
				return originalOnStart(containerID)
			}
			return nil
		}
	} else {
		close(injectionComplete)
	}

	wg := sync.WaitGroup{}
	if !config.LayeredBuild {
		wg.Add(1)
		uploadDir := filepath.Join(config.WorkingDir, "upload")
		// TODO: be able to pass a stream directly to the Docker build to avoid the double temp hit
		r, w := io.Pipe()
		go func() {
			// Wait for the injections to complete and check the error. Do not start
			// streaming the sources when the injection failed.
			<-injectionComplete
			if injectionError != nil {
				wg.Done()
				return
			}
			glog.V(2).Info("starting the source uploading ...")
			var err error
			defer func() {
				w.CloseWithError(err)
				if r := recover(); r != nil {
					glog.Errorf("recovered panic: %#v", r)
				}
				wg.Done()
			}()
			err = builder.tar.CreateTarStream(uploadDir, false, w)
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
				if glog.V(2) && err != io.ErrClosedPipe && err != io.EOF {
					glog.Errorf("Error reading docker stdout, %v", err)
				}
				break
			}
			// Nothing is printed when the quiet option is set
			if config.Quiet {
				continue
			}
			// The log level > 3 forces to use glog instead of printing to stdout
			if glog.V(3) {
				glog.Info(text)
				continue
			}
			fmt.Fprintf(os.Stdout, "%s\n", strings.TrimSpace(text))
		}
	}(outReader)

	go dockerpkg.StreamContainerIO(errReader, &errOutput, glog.Error)

	err = builder.docker.RunContainer(opts)
	if util.IsTimeoutError(err) {
		// Cancel waiting for source input if the container timeouts
		wg.Done()
	}
	if e, ok := err.(errors.ContainerError); ok {
		return errors.NewContainerError(config.BuilderImage, e.ErrorCode, errOutput)
	}
	return err
}

func (builder *STI) generateConfigEnv() (configEnv []string) {
	for _, e := range builder.config.Environment {
		configEnv = append(configEnv, strings.Join([]string{e.Name, e.Value}, "="))
	}
	return
}

func isMissingRequirements(text string) bool {
	tar, _ := regexp.MatchString(`.*tar.*not found`, text)
	sh, _ := regexp.MatchString(`.*/bin/sh.*no such file or directory`, text)
	return tar || sh
}

func includes(arr []string, str string) bool {
	for _, s := range arr {
		if s == str {
			return true
		}
	}
	return false
}
