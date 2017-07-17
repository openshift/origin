package sti

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/build/strategies/layered"
	dockerpkg "github.com/openshift/source-to-image/pkg/docker"
	s2ierr "github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/ignore"
	"github.com/openshift/source-to-image/pkg/scm"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/scripts"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util"
	"github.com/openshift/source-to-image/pkg/util/cmd"
	"github.com/openshift/source-to-image/pkg/util/fs"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"
	utilstatus "github.com/openshift/source-to-image/pkg/util/status"
)

var (
	glog = utilglog.StderrLog

	// List of directories that needs to be present inside working dir
	workingDirs = []string{
		api.UploadScripts,
		api.Source,
		api.DefaultScripts,
		api.UserScripts,
	}

	errMissingRequirements = errors.New("missing requirements")
)

// STI strategy executes the S2I build.
// For more details about S2I, visit https://github.com/openshift/source-to-image
type STI struct {
	config                 *api.Config
	result                 *api.Result
	postExecutor           dockerpkg.PostExecutor
	installer              scripts.Installer
	runtimeInstaller       scripts.Installer
	git                    git.Git
	fs                     fs.FileSystem
	tar                    tar.Tar
	docker                 dockerpkg.Docker
	incrementalDocker      dockerpkg.Docker
	runtimeDocker          dockerpkg.Docker
	callbackInvoker        util.CallbackInvoker
	requiredScripts        []string
	optionalScripts        []string
	optionalRuntimeScripts []string
	externalScripts        map[string]bool
	installedScripts       map[string]bool
	scriptsURL             map[string]string
	incremental            bool
	sourceInfo             *git.SourceInfo
	env                    []string
	newLabels              map[string]string

	// Interfaces
	preparer  build.Preparer
	ignorer   build.Ignorer
	artifacts build.IncrementalBuilder
	scripts   build.ScriptsHandler
	source    build.Downloader
	garbage   build.Cleaner
	layered   build.Builder

	// post executors steps
	postExecutorStage            int
	postExecutorFirstStageSteps  []postExecutorStep
	postExecutorSecondStageSteps []postExecutorStep
	postExecutorStepsContext     *postExecutorStepContext
}

// New returns the instance of STI builder strategy for the given config.
// If the layeredBuilder parameter is specified, then the builder provided will
// be used for the case that the base Docker image does not have 'tar' or 'bash'
// installed.
func New(client dockerpkg.Client, config *api.Config, fs fs.FileSystem, overrides build.Overrides) (*STI, error) {
	excludePattern, err := regexp.Compile(config.ExcludeRegExp)
	if err != nil {
		return nil, err
	}

	docker := dockerpkg.New(client, config.PullAuthentication)
	var incrementalDocker dockerpkg.Docker
	if config.Incremental {
		incrementalDocker = dockerpkg.New(client, config.IncrementalAuthentication)
	}

	inst := scripts.NewInstaller(
		config.BuilderImage,
		config.ScriptsURL,
		config.ScriptDownloadProxyConfig,
		docker,
		config.PullAuthentication,
		fs,
	)
	tarHandler := tar.New(fs)
	tarHandler.SetExclusionPattern(excludePattern)

	builder := &STI{
		installer:              inst,
		config:                 config,
		docker:                 docker,
		incrementalDocker:      incrementalDocker,
		git:                    git.New(fs, cmd.NewCommandRunner()),
		fs:                     fs,
		tar:                    tarHandler,
		callbackInvoker:        util.NewCallbackInvoker(),
		requiredScripts:        []string{api.Assemble, api.Run},
		optionalScripts:        []string{api.SaveArtifacts},
		optionalRuntimeScripts: []string{api.AssembleRuntime},
		externalScripts:        map[string]bool{},
		installedScripts:       map[string]bool{},
		scriptsURL:             map[string]string{},
		newLabels:              map[string]string{},
	}

	if len(config.RuntimeImage) > 0 {
		builder.runtimeDocker = dockerpkg.New(client, config.RuntimeAuthentication)

		builder.runtimeInstaller = scripts.NewInstaller(
			config.RuntimeImage,
			config.ScriptsURL,
			config.ScriptDownloadProxyConfig,
			builder.runtimeDocker,
			config.RuntimeAuthentication,
			builder.fs,
		)
	}

	// The sources are downloaded using the Git downloader.
	// TODO: Add more SCM in future.
	// TODO: explicit decision made to customize processing for usage specifically vs.
	// leveraging overrides; also, we ultimately want to simplify s2i usage a good bit,
	// which would lead to replacing this quick short circuit (so this change is tactical)
	builder.source = overrides.Downloader
	if builder.source == nil && !config.Usage {
		downloader, err := scm.DownloaderForSource(builder.fs, config.Source, config.ForceCopy)
		if err != nil {
			return nil, err
		}
		builder.source = downloader
	}
	builder.garbage = build.NewDefaultCleaner(builder.fs, builder.docker)

	builder.layered, err = layered.New(client, config, builder.fs, builder, overrides)
	if err != nil {
		return nil, err
	}

	// Set interfaces
	builder.preparer = builder
	// later on, if we support say .gitignore func in addition to .dockerignore
	// func, setting ignorer will be based on config setting
	builder.ignorer = &ignore.DockerIgnorer{}
	builder.artifacts = builder
	builder.scripts = builder
	builder.postExecutor = builder
	builder.initPostExecutorSteps()

	return builder, nil
}

// Build processes a Request and returns a *api.Result and an error.
// An error represents a failure performing the build rather than a failure
// of the build itself.  Callers should check the Success field of the result
// to determine whether a build succeeded or not.
func (builder *STI) Build(config *api.Config) (*api.Result, error) {
	builder.result = &api.Result{}

	if len(builder.config.CallbackURL) > 0 {
		defer func() {
			builder.result.Messages = builder.callbackInvoker.ExecuteCallback(
				builder.config.CallbackURL,
				builder.result.Success,
				builder.postExecutorStepsContext.labels,
				builder.result.Messages,
			)
		}()
	}
	defer builder.garbage.Cleanup(config)

	glog.V(1).Infof("Preparing to build %s", config.Tag)
	if err := builder.preparer.Prepare(config); err != nil {
		return builder.result, err
	}

	if builder.incremental = builder.artifacts.Exists(config); builder.incremental {
		tag := firstNonEmpty(config.IncrementalFromTag, config.Tag)
		glog.V(1).Infof("Existing image for tag %s detected for incremental build", tag)
	} else {
		glog.V(1).Info("Clean build will be performed")
	}

	glog.V(2).Infof("Performing source build from %s", config.Source)
	if builder.incremental {
		if err := builder.artifacts.Save(config); err != nil {
			glog.Warning("Clean build will be performed because of error saving previous build artifacts")
			glog.V(2).Infof("error: %v", err)
		}
	}

	if len(config.AssembleUser) > 0 {
		glog.V(1).Infof("Running %q in %q as %q user", api.Assemble, config.Tag, config.AssembleUser)
	} else {
		glog.V(1).Infof("Running %q in %q", api.Assemble, config.Tag)
	}
	startTime := time.Now()
	if err := builder.scripts.Execute(api.Assemble, config.AssembleUser, config); err != nil {
		if err == errMissingRequirements {
			glog.V(1).Info("Image is missing basic requirements (sh or tar), layered build will be performed")
			return builder.layered.Build(config)
		}
		if e, ok := err.(s2ierr.ContainerError); ok {
			if !isMissingRequirements(e.Output) {
				builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
					utilstatus.ReasonAssembleFailed,
					utilstatus.ReasonMessageAssembleFailed,
				)
				return builder.result, err
			}
			glog.V(1).Info("Image is missing basic requirements (sh or tar), layered build will be performed")
			buildResult, err := builder.layered.Build(config)
			return buildResult, err
		}

		return builder.result, err
	}
	builder.result.BuildInfo.Stages = api.RecordStageAndStepInfo(builder.result.BuildInfo.Stages, api.StageAssemble, api.StepAssembleBuildScripts, startTime, time.Now())
	builder.result.Success = true

	return builder.result, nil
}

// Prepare prepares the source code and tar for build.
// NOTE: this func serves both the sti and onbuild strategies, as the OnBuild
// struct Build func leverages the STI struct Prepare func directly below.
func (builder *STI) Prepare(config *api.Config) error {
	var err error
	if builder.result == nil {
		builder.result = &api.Result{}
	}

	if len(config.WorkingDir) == 0 {
		if config.WorkingDir, err = builder.fs.CreateWorkingDirectory(); err != nil {
			builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonFSOperationFailed,
				utilstatus.ReasonMessageFSOperationFailed,
			)
			return err
		}
	}

	builder.result.WorkingDir = config.WorkingDir

	if len(config.RuntimeImage) > 0 {
		startTime := time.Now()
		dockerpkg.GetRuntimeImage(config, builder.runtimeDocker)
		builder.result.BuildInfo.Stages = api.RecordStageAndStepInfo(builder.result.BuildInfo.Stages, api.StagePullImages, api.StepPullRuntimeImage, startTime, time.Now())

		if err != nil {
			builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonPullRuntimeImageFailed,
				utilstatus.ReasonMessagePullRuntimeImageFailed,
			)
			glog.Errorf("Unable to pull runtime image %q: %v", config.RuntimeImage, err)
			return err
		}

		// user didn't specify mapping, let's take it from the runtime image then
		if len(builder.config.RuntimeArtifacts) == 0 {
			var mapping string
			mapping, err = builder.docker.GetAssembleInputFiles(config.RuntimeImage)
			if err != nil {
				builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
					utilstatus.ReasonInvalidArtifactsMapping,
					utilstatus.ReasonMessageInvalidArtifactsMapping,
				)
				return err
			}
			if len(mapping) == 0 {
				builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
					utilstatus.ReasonGenericS2IBuildFailed,
					utilstatus.ReasonMessageGenericS2iBuildFailed,
				)
				return errors.New("no runtime artifacts to copy were specified")
			}
			for _, value := range strings.Split(mapping, ";") {
				if err = builder.config.RuntimeArtifacts.Set(value); err != nil {
					builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
						utilstatus.ReasonGenericS2IBuildFailed,
						utilstatus.ReasonMessageGenericS2iBuildFailed,
					)
					return fmt.Errorf("could not  parse %q label with value %q on image %q: %v",
						dockerpkg.AssembleInputFilesLabel, mapping, config.RuntimeImage, err)
				}
			}
		}
		// we're validating values here to be sure that we're handling both of the cases of the invocation:
		// from main() and as a method from OpenShift
		for _, volumeSpec := range builder.config.RuntimeArtifacts {
			var volumeErr error

			switch {
			case !path.IsAbs(filepath.ToSlash(volumeSpec.Source)):
				volumeErr = fmt.Errorf("invalid runtime artifacts mapping: %q -> %q: source must be an absolute path", volumeSpec.Source, volumeSpec.Destination)
			case path.IsAbs(volumeSpec.Destination):
				volumeErr = fmt.Errorf("invalid runtime artifacts mapping: %q -> %q: destination must be a relative path", volumeSpec.Source, volumeSpec.Destination)
			case strings.HasPrefix(volumeSpec.Destination, ".."):
				volumeErr = fmt.Errorf("invalid runtime artifacts mapping: %q -> %q: destination cannot start with '..'", volumeSpec.Source, volumeSpec.Destination)
			default:
				continue
			}
			if volumeErr != nil {
				builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
					utilstatus.ReasonInvalidArtifactsMapping,
					utilstatus.ReasonMessageInvalidArtifactsMapping,
				)
				return volumeErr
			}
		}
	}

	// Setup working directories
	for _, v := range workingDirs {
		if err = builder.fs.MkdirAllWithPermissions(filepath.Join(config.WorkingDir, v), 0755); err != nil {
			builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonFSOperationFailed,
				utilstatus.ReasonMessageFSOperationFailed,
			)
			return err
		}
	}

	// fetch sources, for their .s2i/bin might contain s2i scripts
	if config.Source != nil {
		if builder.sourceInfo, err = builder.source.Download(config); err != nil {
			builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonFetchSourceFailed,
				utilstatus.ReasonMessageFetchSourceFailed,
			)
			return err
		}
		if config.SourceInfo != nil {
			builder.sourceInfo = config.SourceInfo
		}
	}

	// get the scripts
	required, err := builder.installer.InstallRequired(builder.requiredScripts, config.WorkingDir)
	if err != nil {
		builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
			utilstatus.ReasonInstallScriptsFailed,
			utilstatus.ReasonMessageInstallScriptsFailed,
		)
		return err
	}
	optional := builder.installer.InstallOptional(builder.optionalScripts, config.WorkingDir)

	requiredAndOptional := append(required, optional...)

	if len(config.RuntimeImage) > 0 && builder.runtimeInstaller != nil {
		optionalRuntime := builder.runtimeInstaller.InstallOptional(builder.optionalRuntimeScripts, config.WorkingDir)
		requiredAndOptional = append(requiredAndOptional, optionalRuntime...)
	}

	// If a ScriptsURL was specified, but no scripts were downloaded from it, throw an error
	if len(config.ScriptsURL) > 0 {
		failedCount := 0
		for _, result := range requiredAndOptional {
			if includes(result.FailedSources, scripts.ScriptURLHandler) {
				failedCount++
			}
		}
		if failedCount == len(requiredAndOptional) {
			builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonScriptsFetchFailed,
				utilstatus.ReasonMessageScriptsFetchFailed,
			)
			return fmt.Errorf("could not download any scripts from URL %v", config.ScriptsURL)
		}
	}

	for _, r := range requiredAndOptional {
		if r.Error != nil {
			glog.Warningf("Error getting %v from %s: %v", r.Script, r.URL, r.Error)
			continue
		}

		builder.externalScripts[r.Script] = r.Downloaded
		builder.installedScripts[r.Script] = r.Installed
		builder.scriptsURL[r.Script] = r.URL
	}

	// see if there is a .s2iignore file, and if so, read in the patterns an then
	// search and delete on
	return builder.ignorer.Ignore(config)
}

// SetScripts allows to override default required and optional scripts
func (builder *STI) SetScripts(required, optional []string) {
	builder.requiredScripts = required
	builder.optionalScripts = optional
}

// PostExecute allows to execute post-build actions after the Docker
// container execution finishes.
func (builder *STI) PostExecute(containerID, destination string) error {
	builder.postExecutorStepsContext.containerID = containerID
	builder.postExecutorStepsContext.destination = destination

	stageSteps := builder.postExecutorFirstStageSteps
	if builder.postExecutorStage > 0 {
		stageSteps = builder.postExecutorSecondStageSteps
	}

	for _, step := range stageSteps {
		if err := step.execute(builder.postExecutorStepsContext); err != nil {
			glog.V(0).Info("error: Execution of post execute step failed")
			return err
		}
	}

	return nil
}

func createBuildEnvironment(config *api.Config) []string {
	env, err := scripts.GetEnvironment(config)
	if err != nil {
		glog.V(3).Infof("No user environment provided (%v)", err)
	}

	return append(scripts.ConvertEnvironmentList(env), scripts.ConvertEnvironmentList(config.Environment)...)
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

	tag := firstNonEmpty(config.IncrementalFromTag, config.Tag)

	startTime := time.Now()
	result, err := dockerpkg.PullImage(tag, builder.incrementalDocker, policy)
	builder.result.BuildInfo.Stages = api.RecordStageAndStepInfo(builder.result.BuildInfo.Stages, api.StagePullImages, api.StepPullPreviousImage, startTime, time.Now())

	if err != nil {
		builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
			utilstatus.ReasonPullPreviousImageFailed,
			utilstatus.ReasonMessagePullPreviousImageFailed,
		)
		glog.V(2).Infof("Unable to pull previously built image %q: %v", tag, err)
		return false
	}

	return result.Image != nil && builder.installedScripts[api.SaveArtifacts]
}

// Save extracts and restores the build artifacts from the previous build to
// the current build.
func (builder *STI) Save(config *api.Config) (err error) {
	artifactTmpDir := filepath.Join(config.WorkingDir, "upload", "artifacts")
	if builder.result == nil {
		builder.result = &api.Result{}
	}

	if err = builder.fs.Mkdir(artifactTmpDir); err != nil {
		builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
			utilstatus.ReasonFSOperationFailed,
			utilstatus.ReasonMessageFSOperationFailed,
		)
		return err
	}

	image := firstNonEmpty(config.IncrementalFromTag, config.Tag)

	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	glog.V(1).Infof("Saving build artifacts from image %s to path %s", image, artifactTmpDir)
	extractFunc := func(string) error {
		startTime := time.Now()
		extractErr := builder.tar.ExtractTarStream(artifactTmpDir, outReader)
		io.Copy(ioutil.Discard, outReader) // must ensure reader from container is drained
		builder.result.BuildInfo.Stages = api.RecordStageAndStepInfo(builder.result.BuildInfo.Stages, api.StageRetrieve, api.StepRetrievePreviousArtifacts, startTime, time.Now())
		return extractErr
	}

	user := config.AssembleUser
	if len(user) == 0 {
		user, err = builder.docker.GetImageUser(image)
		if err != nil {
			builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonGenericS2IBuildFailed,
				utilstatus.ReasonMessageGenericS2iBuildFailed,
			)
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

	dockerpkg.StreamContainerIO(errReader, nil, func(s string) { glog.Info(s) })
	err = builder.docker.RunContainer(opts)
	if e, ok := err.(s2ierr.ContainerError); ok {
		err = s2ierr.NewSaveArtifactsError(image, e.Output, err)
	}

	builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
		utilstatus.ReasonGenericS2IBuildFailed,
		utilstatus.ReasonMessageGenericS2iBuildFailed,
	)
	return err
}

// Execute runs the specified STI script in the builder image.
func (builder *STI) Execute(command string, user string, config *api.Config) error {
	glog.V(2).Infof("Using image name %s", config.BuilderImage)

	// Ensure that the builder image is present in the local Docker daemon.
	// The image should have been pulled when the strategy was created, so
	// this should be a quick inspect of the existing image. However, if
	// the image has been deleted since the strategy was created, this will ensure
	// it exists before executing a script on it.
	builder.docker.CheckAndPullImage(config.BuilderImage)

	// we can't invoke this method before (for example in New() method)
	// because of later initialization of config.WorkingDir
	builder.env = createBuildEnvironment(config)

	errOutput := ""
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
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
		Env:             builder.env,
		User:            user,
		PostExec:        builder.postExecutor,
		NetworkMode:     string(config.DockerNetworkMode),
		CGroupLimits:    config.CGroupLimits,
		CapDrop:         config.DropCapabilities,
		Binds:           config.BuildVolumes,
	}

	// If there are injections specified, override the original assemble script
	// and wait till all injections are uploaded into the container that runs the
	// assemble script.
	injectionError := make(chan error)
	if len(config.Injections) > 0 && command == api.Assemble {
		workdir, err := builder.docker.GetImageWorkdir(config.BuilderImage)
		if err != nil {
			builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonGenericS2IBuildFailed,
				utilstatus.ReasonMessageGenericS2iBuildFailed,
			)
			return err
		}
		config.Injections = util.FixInjectionsWithRelativePath(workdir, config.Injections)
		injectedFiles, err := util.ExpandInjectedFiles(builder.fs, config.Injections)
		if err != nil {
			builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonInstallScriptsFailed,
				utilstatus.ReasonMessageInstallScriptsFailed,
			)
			return err
		}
		rmScript, err := util.CreateInjectedFilesRemovalScript(injectedFiles, "/tmp/rm-injections")
		if len(rmScript) != 0 {
			defer os.Remove(rmScript)
		}
		if err != nil {
			builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonGenericS2IBuildFailed,
				utilstatus.ReasonMessageGenericS2iBuildFailed,
			)
			return err
		}
		opts.CommandOverrides = func(cmd string) string {
			return fmt.Sprintf("while [ ! -f %q ]; do sleep 0.5; done; %s; result=$?; . %[1]s; exit $result",
				"/tmp/rm-injections", cmd)
		}
		originalOnStart := opts.OnStart
		opts.OnStart = func(containerID string) error {
			defer close(injectionError)
			glog.V(2).Info("starting the injections uploading ...")
			for _, s := range config.Injections {
				if err := builder.docker.UploadToContainer(builder.fs, s.Source, s.Destination, containerID); err != nil {
					injectionError <- util.HandleInjectionError(s, err)
					return err
				}
			}
			if err := builder.docker.UploadToContainer(builder.fs, rmScript, "/tmp/rm-injections", containerID); err != nil {
				injectionError <- util.HandleInjectionError(api.VolumeSpec{Source: rmScript, Destination: "/tmp/rm-injections"}, err)
				return err
			}
			if originalOnStart != nil {
				return originalOnStart(containerID)
			}
			return nil
		}
	} else {
		close(injectionError)
	}

	if !config.LayeredBuild {
		r, w := io.Pipe()
		opts.Stdin = r

		go func() {
			// Wait for the injections to complete and check the error. Do not start
			// streaming the sources when the injection failed.
			if <-injectionError != nil {
				w.Close()
				return
			}
			glog.V(2).Info("starting the source uploading ...")
			uploadDir := filepath.Join(config.WorkingDir, "upload")
			w.CloseWithError(builder.tar.CreateTarStream(uploadDir, false, w))
		}()
	}

	dockerpkg.StreamContainerIO(outReader, nil, func(s string) {
		if !config.Quiet {
			glog.Info(strings.TrimSpace(s))
		}
	})

	c := dockerpkg.StreamContainerIO(errReader, &errOutput, func(s string) { glog.Info(s) })

	err := builder.docker.RunContainer(opts)
	if err != nil {
		// Must wait for StreamContainerIO goroutine above to exit before reading errOutput.
		<-c

		if isMissingRequirements(errOutput) {
			err = errMissingRequirements
		} else if e, ok := err.(s2ierr.ContainerError); ok {
			err = s2ierr.NewContainerError(config.BuilderImage, e.ErrorCode, errOutput)
		}
	}

	return err
}

func (builder *STI) initPostExecutorSteps() {
	builder.postExecutorStepsContext = &postExecutorStepContext{}
	if len(builder.config.RuntimeImage) == 0 {
		builder.postExecutorFirstStageSteps = []postExecutorStep{
			&storePreviousImageStep{
				builder: builder,
				docker:  builder.docker,
			},
			&commitImageStep{
				image:   builder.config.BuilderImage,
				builder: builder,
				docker:  builder.docker,
				fs:      builder.fs,
				tar:     builder.tar,
			},
			&reportSuccessStep{
				builder: builder,
			},
			&removePreviousImageStep{
				builder: builder,
				docker:  builder.docker,
			},
		}
	} else {
		builder.postExecutorFirstStageSteps = []postExecutorStep{
			&downloadFilesFromBuilderImageStep{
				builder: builder,
				docker:  builder.docker,
				fs:      builder.fs,
				tar:     builder.tar,
			},
			&startRuntimeImageAndUploadFilesStep{
				builder: builder,
				docker:  builder.docker,
				fs:      builder.fs,
			},
		}
		builder.postExecutorSecondStageSteps = []postExecutorStep{
			&commitImageStep{
				image:   builder.config.RuntimeImage,
				builder: builder,
				docker:  builder.docker,
			},
			&reportSuccessStep{
				builder: builder,
			},
		}
	}
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

func firstNonEmpty(args ...string) string {
	for _, value := range args {
		if len(value) > 0 {
			return value
		}
	}
	return ""
}
