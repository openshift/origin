package builder

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	s2iapi "github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/describe"
	"github.com/openshift/source-to-image/pkg/api/validation"
	s2ibuild "github.com/openshift/source-to-image/pkg/build"
	s2i "github.com/openshift/source-to-image/pkg/build/strategies"
	"github.com/openshift/source-to-image/pkg/docker"
	s2igit "github.com/openshift/source-to-image/pkg/scm/git"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	"github.com/openshift/origin/pkg/build/builder/timing"
	"github.com/openshift/origin/pkg/build/controller/strategy"
	"github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/generate/git"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// builderFactory is the internal interface to decouple S2I-specific code from Origin builder code
type builderFactory interface {
	// Create S2I Builder based on S2I configuration
	Builder(config *s2iapi.Config, overrides s2ibuild.Overrides) (s2ibuild.Builder, s2iapi.BuildInfo, error)
}

// validator is the interval interface to decouple S2I-specific code from Origin builder code
type validator interface {
	// Perform validation of S2I configuration, returns slice of validation errors
	ValidateConfig(config *s2iapi.Config) []validation.Error
}

// runtimeBuilderFactory is the default implementation of stiBuilderFactory
type runtimeBuilderFactory struct{}

// Builder delegates execution to S2I-specific code
func (_ runtimeBuilderFactory) Builder(config *s2iapi.Config, overrides s2ibuild.Overrides) (s2ibuild.Builder, s2iapi.BuildInfo, error) {
	client, err := docker.NewEngineAPIClient(config.DockerConfig)
	if err != nil {
		return nil, s2iapi.BuildInfo{}, err
	}
	builder, buildInfo, err := s2i.Strategy(client, config, overrides)
	return builder, buildInfo, err
}

// runtimeConfigValidator is the default implementation of stiConfigValidator
type runtimeConfigValidator struct{}

// ValidateConfig delegates execution to S2I-specific code
func (_ runtimeConfigValidator) ValidateConfig(config *s2iapi.Config) []validation.Error {
	return validation.ValidateConfig(config)
}

// S2IBuilder performs an STI build given the build object
type S2IBuilder struct {
	builder      builderFactory
	validator    validator
	gitClient    GitClient
	dockerClient DockerClient
	dockerSocket string
	build        *buildapi.Build
	client       client.BuildInterface
	cgLimits     *s2iapi.CGroupLimits
}

// NewS2IBuilder creates a new STIBuilder instance
func NewS2IBuilder(dockerClient DockerClient, dockerSocket string, buildsClient client.BuildInterface, build *buildapi.Build,
	gitClient GitClient, cgLimits *s2iapi.CGroupLimits) *S2IBuilder {
	// delegate to internal implementation passing default implementation of builderFactory and validator
	return newS2IBuilder(dockerClient, dockerSocket, buildsClient, build, gitClient, runtimeBuilderFactory{}, runtimeConfigValidator{}, cgLimits)
}

// newS2IBuilder is the internal factory function to create STIBuilder based on parameters. Used for testing.
func newS2IBuilder(dockerClient DockerClient, dockerSocket string, buildsClient client.BuildInterface, build *buildapi.Build,
	gitClient GitClient, builder builderFactory, validator validator, cgLimits *s2iapi.CGroupLimits) *S2IBuilder {
	// just create instance
	return &S2IBuilder{
		builder:      builder,
		validator:    validator,
		gitClient:    gitClient,
		dockerClient: dockerClient,
		dockerSocket: dockerSocket,
		build:        build,
		client:       buildsClient,
		cgLimits:     cgLimits,
	}
}

// Build executes STI build based on configured builder, S2I builder factory
// and S2I config validator
func (s *S2IBuilder) Build() error {

	var err error
	ctx := timing.NewContext(context.Background())
	defer func() {
		s.build.Status.Stages = buildapi.AppendStageAndStepInfo(s.build.Status.Stages, timing.GetStages(ctx))
		handleBuildStatusUpdate(s.build, s.client, nil)
	}()

	if s.build.Spec.Strategy.SourceStrategy == nil {
		return errors.New("the source to image builder must be used with the source strategy")
	}

	buildDir, err := ioutil.TempDir("", "s2i-build")
	if err != nil {
		return err
	}
	srcDir := filepath.Join(buildDir, s2iapi.Source)
	if err = os.MkdirAll(srcDir, os.ModePerm); err != nil {
		return err
	}

	var push bool
	// if there is no output target, set one up so the docker build logic
	// (which requires a tag) will still work, but we won't push it at the end.
	if s.build.Spec.Output.To == nil || len(s.build.Spec.Output.To.Name) == 0 {
		s.build.Status.OutputDockerImageReference = s.build.Name
	} else {
		push = true
	}
	pushTag := s.build.Status.OutputDockerImageReference

	// fetch source
	sourceInfo, err := fetchSource(ctx, s.dockerClient, srcDir, s.build, initialURLCheckTimeout, os.Stdin, s.gitClient)

	if err != nil {
		switch err.(type) {
		case contextDirNotFoundError:
			s.build.Status.Phase = buildapi.BuildPhaseFailed
			s.build.Status.Reason = buildapi.StatusReasonInvalidContextDirectory
			s.build.Status.Message = buildapi.StatusMessageInvalidContextDirectory
		default:
			s.build.Status.Phase = buildapi.BuildPhaseFailed
			s.build.Status.Reason = buildapi.StatusReasonFetchSourceFailed
			s.build.Status.Message = buildapi.StatusMessageFetchSourceFailed
		}
		handleBuildStatusUpdate(s.build, s.client, nil)
		return err
	}

	contextDir := ""
	if len(s.build.Spec.Source.ContextDir) > 0 {
		contextDir = filepath.Clean(s.build.Spec.Source.ContextDir)
		if contextDir == "." || contextDir == "/" {
			contextDir = ""
		}
		if len(contextDir) > 0 {
			// if we're building out of a context dir, we need to use a different working
			// directory from where we put the source code because s2i is going to copy
			// from the context dir to the workdir, and if the workdir is a parent of the
			// context dir, we end up with a directory that contains 2 copies of the
			// input source code.
			buildDir, err = ioutil.TempDir("", "s2i-build-context")
		}
		if sourceInfo != nil {
			sourceInfo.ContextDir = s.build.Spec.Source.ContextDir
		}
	}

	var s2iSourceInfo *s2igit.SourceInfo
	if sourceInfo != nil {
		s2iSourceInfo = &sourceInfo.SourceInfo
		revision := updateBuildRevision(s.build, sourceInfo)
		handleBuildStatusUpdate(s.build, s.client, revision)
	}

	injections := s2iapi.VolumeList{}
	for _, s := range s.build.Spec.Source.Secrets {
		glog.V(3).Infof("Injecting secret %q into a build into %q", s.Secret.Name, filepath.Clean(s.DestinationDir))
		secretSourcePath := filepath.Join(strategy.SecretBuildSourceBaseMountPath, s.Secret.Name)
		injections = append(injections, s2iapi.VolumeSpec{
			Source:      secretSourcePath,
			Destination: s.DestinationDir,
		})
	}

	buildTag := randomBuildTag(s.build.Namespace, s.build.Name)
	scriptDownloadProxyConfig, err := scriptProxyConfig(s.build)
	if err != nil {
		return err
	}
	if scriptDownloadProxyConfig != nil {
		glog.V(0).Infof("Using HTTP proxy %v and HTTPS proxy %v for script download",
			util.SafeForLoggingURL(scriptDownloadProxyConfig.HTTPProxy),
			util.SafeForLoggingURL(scriptDownloadProxyConfig.HTTPSProxy),
		)
	}

	var incremental bool
	if s.build.Spec.Strategy.SourceStrategy.Incremental != nil {
		incremental = *s.build.Spec.Strategy.SourceStrategy.Incremental
	}
	config := &s2iapi.Config{
		// Save some processing time by not cleaning up (the container will go away anyway)
		PreserveWorkingDir: true,
		WorkingDir:         buildDir,
		DockerConfig:       &s2iapi.DockerConfig{Endpoint: s.dockerSocket},
		DockerCfgPath:      os.Getenv(dockercfg.PullAuthType),
		LabelNamespace:     buildapi.DefaultDockerLabelNamespace,

		ScriptsURL: s.build.Spec.Strategy.SourceStrategy.Scripts,

		BuilderImage:       s.build.Spec.Strategy.SourceStrategy.From.Name,
		Incremental:        incremental,
		IncrementalFromTag: pushTag,

		Environment:       buildEnvVars(s.build, sourceInfo),
		Labels:            buildLabels(s.build),
		DockerNetworkMode: getDockerNetworkMode(),

		Source:     &s2igit.URL{URL: url.URL{Path: srcDir}, Type: s2igit.URLTypeLocal},
		ContextDir: contextDir,
		SourceInfo: s2iSourceInfo,
		ForceCopy:  true,
		Injections: injections,

		Tag: buildTag,

		CGroupLimits:              s.cgLimits,
		ScriptDownloadProxyConfig: scriptDownloadProxyConfig,
		BlockOnBuild:              true,
	}

	if s.build.Spec.Strategy.SourceStrategy.ForcePull {
		glog.V(4).Infof("With force pull true, setting policies to %s", s2iapi.PullAlways)
		config.BuilderPullPolicy = s2iapi.PullAlways
		config.RuntimeImagePullPolicy = s2iapi.PullAlways
	} else {
		glog.V(4).Infof("With force pull false, setting policies to %s", s2iapi.PullIfNotPresent)
		config.BuilderPullPolicy = s2iapi.PullIfNotPresent
		config.RuntimeImagePullPolicy = s2iapi.PullIfNotPresent
	}
	config.PreviousImagePullPolicy = s2iapi.PullAlways

	allowedUIDs := os.Getenv(buildapi.AllowedUIDs)
	glog.V(4).Infof("The value of %s is [%s]", buildapi.AllowedUIDs, allowedUIDs)
	if len(allowedUIDs) > 0 {
		err = config.AllowedUIDs.Set(allowedUIDs)
		if err != nil {
			return err
		}
	}
	dropCaps := os.Getenv(buildapi.DropCapabilities)
	glog.V(4).Infof("The value of %s is [%s]", buildapi.DropCapabilities, dropCaps)
	if len(dropCaps) > 0 {
		config.DropCapabilities = strings.Split(dropCaps, ",")
	}

	if s.build.Spec.Strategy.SourceStrategy.RuntimeImage != nil {
		runtimeImageName := s.build.Spec.Strategy.SourceStrategy.RuntimeImage.Name
		config.RuntimeImage = runtimeImageName
		t, _ := dockercfg.NewHelper().GetDockerAuth(runtimeImageName, dockercfg.PullAuthType)
		config.RuntimeAuthentication = s2iapi.AuthConfig{Username: t.Username, Password: t.Password, Email: t.Email, ServerAddress: t.ServerAddress}
		config.RuntimeArtifacts = copyToVolumeList(s.build.Spec.Strategy.SourceStrategy.RuntimeArtifacts)
	}
	// If DockerCfgPath is provided in buildapi.Config, then attempt to read the
	// dockercfg file and get the authentication for pulling the builder image.
	t, _ := dockercfg.NewHelper().GetDockerAuth(config.BuilderImage, dockercfg.PullAuthType)
	config.PullAuthentication = s2iapi.AuthConfig{Username: t.Username, Password: t.Password, Email: t.Email, ServerAddress: t.ServerAddress}
	t, _ = dockercfg.NewHelper().GetDockerAuth(pushTag, dockercfg.PushAuthType)
	config.IncrementalAuthentication = s2iapi.AuthConfig{Username: t.Username, Password: t.Password, Email: t.Email, ServerAddress: t.ServerAddress}

	if errs := s.validator.ValidateConfig(config); len(errs) != 0 {
		var buffer bytes.Buffer
		for _, ve := range errs {
			buffer.WriteString(ve.Error())
			buffer.WriteString(", ")
		}
		return errors.New(buffer.String())
	}

	client, err := docker.NewEngineAPIClient(config.DockerConfig)
	if err != nil {
		return err
	}
	if glog.Is(4) {
		redactedConfig := util.SafeForLoggingS2IConfig(config)
		glog.V(4).Infof("Creating a new S2I builder with config: %#v\n", describe.Config(client, redactedConfig))
	}
	builder, buildInfo, err := s.builder.Builder(config, s2ibuild.Overrides{Downloader: nil})
	if err != nil {
		s.build.Status.Phase = buildapi.BuildPhaseFailed
		s.build.Status.Reason, s.build.Status.Message = convertS2IFailureType(
			buildInfo.FailureReason.Reason,
			buildInfo.FailureReason.Message,
		)
		handleBuildStatusUpdate(s.build, s.client, nil)
		return err
	}

	glog.V(4).Infof("Starting S2I build from %s/%s BuildConfig ...", s.build.Namespace, s.build.Name)
	startTime := metav1.Now()
	result, err := builder.Build(config)

	for _, stage := range result.BuildInfo.Stages {
		for _, step := range stage.Steps {
			timing.RecordNewStep(ctx, buildapi.StageName(stage.Name), buildapi.StepName(step.Name), metav1.NewTime(step.StartTime), metav1.NewTime(step.StartTime.Add(time.Duration(step.DurationMilliseconds)*time.Millisecond)))
		}
	}

	if err != nil {
		s.build.Status.Phase = buildapi.BuildPhaseFailed
		s.build.Status.Reason, s.build.Status.Message = convertS2IFailureType(
			result.BuildInfo.FailureReason.Reason,
			result.BuildInfo.FailureReason.Message,
		)

		handleBuildStatusUpdate(s.build, s.client, nil)
		return err
	}

	cName := containerName("s2i", s.build.Name, s.build.Namespace, "post-commit")
	startTime = metav1.Now()
	err = execPostCommitHook(s.dockerClient, s.build.Spec.PostCommit, buildTag, cName)

	timing.RecordNewStep(ctx, buildapi.StagePostCommit, buildapi.StepExecPostCommitHook, startTime, metav1.Now())

	if err != nil {
		s.build.Status.Phase = buildapi.BuildPhaseFailed
		s.build.Status.Reason = buildapi.StatusReasonPostCommitHookFailed
		s.build.Status.Message = buildapi.StatusMessagePostCommitHookFailed
		handleBuildStatusUpdate(s.build, s.client, nil)
		return err
	}

	if push {
		if err = tagImage(s.dockerClient, buildTag, pushTag); err != nil {
			return err
		}
	}

	if err = removeImage(s.dockerClient, buildTag); err != nil {
		glog.V(0).Infof("warning: Failed to remove temporary build tag %v: %v", buildTag, err)
	}

	if push {
		// Get the Docker push authentication
		pushAuthConfig, authPresent := dockercfg.NewHelper().GetDockerAuth(
			pushTag,
			dockercfg.PushAuthType,
		)
		if authPresent {
			glog.V(3).Infof("Using provided push secret for pushing %s image", pushTag)
		} else {
			glog.V(3).Infof("No push secret provided")
		}
		glog.V(0).Infof("\nPushing image %s ...", pushTag)
		startTime = metav1.Now()
		digest, err := pushImage(s.dockerClient, pushTag, pushAuthConfig)

		timing.RecordNewStep(ctx, buildapi.StagePushImage, buildapi.StepPushImage, startTime, metav1.Now())

		if err != nil {
			s.build.Status.Phase = buildapi.BuildPhaseFailed
			s.build.Status.Reason = buildapi.StatusReasonPushImageToRegistryFailed
			s.build.Status.Message = buildapi.StatusMessagePushImageToRegistryFailed
			handleBuildStatusUpdate(s.build, s.client, nil)
			return reportPushFailure(err, authPresent, pushAuthConfig)
		}

		if len(digest) > 0 {
			s.build.Status.Output.To = &buildapi.BuildStatusOutputTo{
				ImageDigest: digest,
			}
			handleBuildStatusUpdate(s.build, s.client, nil)
		}
		glog.V(0).Infof("Push successful")
	}
	return nil
}

type downloader struct {
	sourceInfo *s2igit.SourceInfo
}

// Download no-ops (because we already downloaded the source to the right location)
// and returns the previously computed sourceInfo for the source.
func (d *downloader) Download(config *s2iapi.Config) (*s2igit.SourceInfo, error) {
	config.WorkingSourceDir = config.Source.LocalPath()

	return d.sourceInfo, nil
}

// buildEnvVars returns a map with build metadata to be inserted into Docker
// images produced by build. It transforms the output from buildInfo into the
// input format expected by s2iapi.Config.Environment.
// Note that using a map has at least two downsides:
// 1. The order of metadata KeyValue pairs is lost;
// 2. In case of repeated Keys, the last Value takes precedence right here,
//    instead of deferring what to do with repeated environment variables to the
//    Docker runtime.
func buildEnvVars(build *buildapi.Build, sourceInfo *git.SourceInfo) s2iapi.EnvironmentList {
	bi := buildInfo(build, sourceInfo)
	envVars := &s2iapi.EnvironmentList{}
	for _, item := range bi {
		envVars.Set(fmt.Sprintf("%s=%s", item.Key, item.Value))
	}
	return *envVars
}

func buildLabels(build *buildapi.Build) map[string]string {
	labels := make(map[string]string)
	addBuildLabels(labels, build)
	for _, lbl := range build.Spec.Output.ImageLabels {
		labels[lbl.Name] = lbl.Value
	}
	return labels
}

// scriptProxyConfig determines a proxy configuration for downloading
// scripts from a URL. For now, it uses environment variables passed in
// the strategy's environment. There is no preference given to either lowercase
// or uppercase form of the variable.
func scriptProxyConfig(build *buildapi.Build) (*s2iapi.ProxyConfig, error) {
	httpProxy := ""
	httpsProxy := ""
	for _, env := range build.Spec.Strategy.SourceStrategy.Env {
		switch env.Name {
		case "HTTP_PROXY", "http_proxy":
			httpProxy = env.Value
		case "HTTPS_PROXY", "https_proxy":
			httpsProxy = env.Value
		}
	}
	if len(httpProxy) == 0 && len(httpsProxy) == 0 {
		return nil, nil
	}
	config := &s2iapi.ProxyConfig{}
	if len(httpProxy) > 0 {
		proxyURL, err := util.ParseProxyURL(httpProxy)
		if err != nil {
			return nil, err
		}
		config.HTTPProxy = proxyURL
	}
	if len(httpsProxy) > 0 {
		proxyURL, err := util.ParseProxyURL(httpsProxy)
		if err != nil {
			return nil, err
		}
		config.HTTPSProxy = proxyURL
	}
	return config, nil
}

// copyToVolumeList copies the artifacts set in the build config to the
// VolumeList struct in the s2iapi.Config
func copyToVolumeList(artifactsMapping []buildapi.ImageSourcePath) (volumeList s2iapi.VolumeList) {
	for _, mappedPath := range artifactsMapping {
		volumeList = append(volumeList, s2iapi.VolumeSpec{
			Source:      mappedPath.SourcePath,
			Destination: mappedPath.DestinationDir,
		})
	}
	return
}

func convertS2IFailureType(reason s2iapi.StepFailureReason, message s2iapi.StepFailureMessage) (buildapi.StatusReason, string) {
	return buildapi.StatusReason(reason), fmt.Sprintf("%s", message)
}
