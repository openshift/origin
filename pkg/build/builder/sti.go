package builder

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
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
	s2iutil "github.com/openshift/source-to-image/pkg/util"

	buildapiv1 "github.com/openshift/api/build/v1"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	"github.com/openshift/origin/pkg/build/builder/timing"
	builderutil "github.com/openshift/origin/pkg/build/builder/util"
	"github.com/openshift/origin/pkg/build/controller/strategy"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/git"

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
	dockerClient DockerClient
	dockerSocket string
	build        *buildapiv1.Build
	client       buildclientv1.BuildInterface
	cgLimits     *s2iapi.CGroupLimits
}

// NewS2IBuilder creates a new STIBuilder instance
func NewS2IBuilder(dockerClient DockerClient, dockerSocket string, buildsClient buildclientv1.BuildInterface, build *buildapiv1.Build,
	cgLimits *s2iapi.CGroupLimits) *S2IBuilder {
	// delegate to internal implementation passing default implementation of builderFactory and validator
	return newS2IBuilder(dockerClient, dockerSocket, buildsClient, build, runtimeBuilderFactory{}, runtimeConfigValidator{}, cgLimits)
}

// newS2IBuilder is the internal factory function to create STIBuilder based on parameters. Used for testing.
func newS2IBuilder(dockerClient DockerClient, dockerSocket string, buildsClient buildclientv1.BuildInterface, build *buildapiv1.Build,
	builder builderFactory, validator validator, cgLimits *s2iapi.CGroupLimits) *S2IBuilder {
	// just create instance
	return &S2IBuilder{
		builder:      builder,
		validator:    validator,
		dockerClient: dockerClient,
		dockerSocket: dockerSocket,
		build:        build,
		client:       buildsClient,
		cgLimits:     cgLimits,
	}
}

// Build executes S2I build based on configured builder, S2I builder factory
// and S2I config validator
func (s *S2IBuilder) Build() error {

	var err error
	ctx := timing.NewContext(context.Background())
	defer func() {
		s.build.Status.Stages = timing.AppendStageAndStepInfo(s.build.Status.Stages, timing.GetStages(ctx))
		HandleBuildStatusUpdate(s.build, s.client, nil)
	}()

	if s.build.Spec.Strategy.SourceStrategy == nil {
		return errors.New("the source to image builder must be used with the source strategy")
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

	sourceInfo, err := readSourceInfo()
	if err != nil {
		return fmt.Errorf("error reading git source info: %v", err)
	}
	var s2iSourceInfo *s2igit.SourceInfo
	if sourceInfo != nil {
		s2iSourceInfo = toS2ISourceInfo(sourceInfo)
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
			buildutil.SafeForLoggingURL(scriptDownloadProxyConfig.HTTPProxy),
			buildutil.SafeForLoggingURL(scriptDownloadProxyConfig.HTTPSProxy),
		)
	}

	var incremental bool
	if s.build.Spec.Strategy.SourceStrategy.Incremental != nil {
		incremental = *s.build.Spec.Strategy.SourceStrategy.Incremental
	}

	srcDir := buildutil.InputContentPath
	contextDir := ""
	if len(s.build.Spec.Source.ContextDir) != 0 {
		contextDir = filepath.Clean(s.build.Spec.Source.ContextDir)
		if contextDir == "." || contextDir == "/" {
			contextDir = ""
		}
	}

	networkMode, resolvConfHostPath, err := getContainerNetworkConfig()
	if err != nil {
		return err
	}

	config := &s2iapi.Config{
		// Save some processing time by not cleaning up (the container will go away anyway)
		PreserveWorkingDir: true,
		WorkingDir:         "/tmp",
		DockerConfig:       &s2iapi.DockerConfig{Endpoint: s.dockerSocket},
		DockerCfgPath:      os.Getenv(dockercfg.PullAuthType),
		LabelNamespace:     builderutil.DefaultDockerLabelNamespace,

		ScriptsURL: s.build.Spec.Strategy.SourceStrategy.Scripts,

		BuilderImage:       s.build.Spec.Strategy.SourceStrategy.From.Name,
		Incremental:        incremental,
		IncrementalFromTag: pushTag,

		Environment:       buildEnvVars(s.build, sourceInfo),
		Labels:            s2iBuildLabels(s.build, sourceInfo),
		DockerNetworkMode: s2iapi.DockerNetworkMode(networkMode),

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

	if len(resolvConfHostPath) != 0 {
		cmd := exec.Command("chcon", "system_u:object_r:svirt_sandbox_file_t:s0", "/etc/resolv.conf")
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("unable to set permissions on /etc/resolv.conf: %v", err)
		}
		config.BuildVolumes = []string{fmt.Sprintf("%s:/etc/resolv.conf", resolvConfHostPath)}
	}

	if s.build.Spec.Strategy.SourceStrategy.ForcePull {
		glog.V(4).Infof("With force pull true, setting policies to %s", s2iapi.PullAlways)
		config.BuilderPullPolicy = s2iapi.PullAlways
	} else {
		glog.V(4).Infof("With force pull false, setting policies to %s", s2iapi.PullIfNotPresent)
		config.BuilderPullPolicy = s2iapi.PullIfNotPresent
	}
	config.PreviousImagePullPolicy = s2iapi.PullAlways

	allowedUIDs := os.Getenv(builderutil.AllowedUIDs)
	glog.V(4).Infof("The value of %s is [%s]", builderutil.AllowedUIDs, allowedUIDs)
	if len(allowedUIDs) > 0 {
		err = config.AllowedUIDs.Set(allowedUIDs)
		if err != nil {
			return err
		}
	}
	dropCaps := os.Getenv(builderutil.DropCapabilities)
	glog.V(4).Infof("The value of %s is [%s]", builderutil.DropCapabilities, dropCaps)
	if len(dropCaps) > 0 {
		config.DropCapabilities = strings.Split(dropCaps, ",")
	}

	// If DockerCfgPath is provided in buildapiv1.Config, then attempt to read the
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
		redactedConfig := SafeForLoggingS2IConfig(config)
		glog.V(4).Infof("Creating a new S2I builder with config: %#v\n", describe.Config(client, redactedConfig))
	}
	builder, buildInfo, err := s.builder.Builder(config, s2ibuild.Overrides{Downloader: nil})
	if err != nil {
		s.build.Status.Phase = buildapiv1.BuildPhaseFailed
		s.build.Status.Reason, s.build.Status.Message = convertS2IFailureType(
			buildInfo.FailureReason.Reason,
			buildInfo.FailureReason.Message,
		)
		HandleBuildStatusUpdate(s.build, s.client, nil)
		return err
	}

	glog.V(4).Infof("Starting S2I build from %s/%s BuildConfig ...", s.build.Namespace, s.build.Name)
	startTime := metav1.Now()
	result, err := builder.Build(config)

	for _, stage := range result.BuildInfo.Stages {
		for _, step := range stage.Steps {
			timing.RecordNewStep(ctx, buildapiv1.StageName(stage.Name), buildapiv1.StepName(step.Name), metav1.NewTime(step.StartTime), metav1.NewTime(step.StartTime.Add(time.Duration(step.DurationMilliseconds)*time.Millisecond)))
		}
	}

	if err != nil {
		s.build.Status.Phase = buildapiv1.BuildPhaseFailed
		s.build.Status.Reason, s.build.Status.Message = convertS2IFailureType(
			result.BuildInfo.FailureReason.Reason,
			result.BuildInfo.FailureReason.Message,
		)

		HandleBuildStatusUpdate(s.build, s.client, nil)
		return err
	}

	cName := containerName("s2i", s.build.Name, s.build.Namespace, "post-commit")
	err = execPostCommitHook(ctx, s.dockerClient, s.build.Spec.PostCommit, buildTag, cName)

	if err != nil {
		s.build.Status.Phase = buildapiv1.BuildPhaseFailed
		s.build.Status.Reason = buildapiv1.StatusReasonPostCommitHookFailed
		s.build.Status.Message = builderutil.StatusMessagePostCommitHookFailed
		HandleBuildStatusUpdate(s.build, s.client, nil)
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

		timing.RecordNewStep(ctx, buildapiv1.StagePushImage, buildapiv1.StepPushImage, startTime, metav1.Now())

		if err != nil {
			s.build.Status.Phase = buildapiv1.BuildPhaseFailed
			s.build.Status.Reason = buildapiv1.StatusReasonPushImageToRegistryFailed
			s.build.Status.Message = builderutil.StatusMessagePushImageToRegistryFailed
			HandleBuildStatusUpdate(s.build, s.client, nil)
			return reportPushFailure(err, authPresent, pushAuthConfig)
		}

		if len(digest) > 0 {
			s.build.Status.Output.To = &buildapiv1.BuildStatusOutputTo{
				ImageDigest: digest,
			}
			HandleBuildStatusUpdate(s.build, s.client, nil)
		}
		glog.V(0).Infof("Push successful")
	}
	return nil
}

// buildEnvVars returns a map with build metadata to be inserted into Docker
// images produced by build. It transforms the output from buildInfo into the
// input format expected by s2iapi.Config.Environment.
// Note that using a map has at least two downsides:
// 1. The order of metadata KeyValue pairs is lost;
// 2. In case of repeated Keys, the last Value takes precedence right here,
//    instead of deferring what to do with repeated environment variables to the
//    Docker runtime.
func buildEnvVars(build *buildapiv1.Build, sourceInfo *git.SourceInfo) s2iapi.EnvironmentList {
	bi := buildInfo(build, sourceInfo)
	envVars := &s2iapi.EnvironmentList{}
	for _, item := range bi {
		envVars.Set(fmt.Sprintf("%s=%s", item.Key, item.Value))
	}
	return *envVars
}

// s2iBuildLabels returns a slice of KeyValue pairs in a format that appendLabel can
// consume.
func s2iBuildLabels(build *buildapiv1.Build, sourceInfo *git.SourceInfo) map[string]string {
	labels := map[string]string{}
	if sourceInfo == nil {
		sourceInfo = &git.SourceInfo{}
	}
	if len(build.Spec.Source.ContextDir) > 0 {
		sourceInfo.ContextDir = build.Spec.Source.ContextDir
	}

	labels = s2iutil.GenerateLabelsFromSourceInfo(labels, toS2ISourceInfo(sourceInfo), builderutil.DefaultDockerLabelNamespace)
	if build != nil && build.Spec.Source.Git != nil && build.Spec.Source.Git.Ref != "" {
		// override the io.openshift.build.commit.ref label to match what we
		// were originally told to check out, as well as the
		// OPENSHIFT_BUILD_REFERENCE environment variable.  This can sometimes
		// differ from git's view (see PotentialPRRetryAsFetch for details).
		labels[builderutil.DefaultDockerLabelNamespace+"build.commit.ref"] = build.Spec.Source.Git.Ref
	}

	// override autogenerated labels
	for _, lbl := range build.Spec.Output.ImageLabels {
		labels[lbl.Name] = lbl.Value
	}
	return labels
}

// scriptProxyConfig determines a proxy configuration for downloading
// scripts from a URL. For now, it uses environment variables passed in
// the strategy's environment. There is no preference given to either lowercase
// or uppercase form of the variable.
func scriptProxyConfig(build *buildapiv1.Build) (*s2iapi.ProxyConfig, error) {
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
		proxyURL, err := buildutil.ParseProxyURL(httpProxy)
		if err != nil {
			return nil, err
		}
		config.HTTPProxy = proxyURL
	}
	if len(httpsProxy) > 0 {
		proxyURL, err := buildutil.ParseProxyURL(httpsProxy)
		if err != nil {
			return nil, err
		}
		config.HTTPSProxy = proxyURL
	}
	return config, nil
}

// copyToVolumeList copies the artifacts set in the build config to the
// VolumeList struct in the s2iapi.Config
func copyToVolumeList(artifactsMapping []buildapiv1.ImageSourcePath) (volumeList s2iapi.VolumeList) {
	for _, mappedPath := range artifactsMapping {
		volumeList = append(volumeList, s2iapi.VolumeSpec{
			Source:      mappedPath.SourcePath,
			Destination: mappedPath.DestinationDir,
		})
	}
	return
}

func convertS2IFailureType(reason s2iapi.StepFailureReason, message s2iapi.StepFailureMessage) (buildapiv1.StatusReason, string) {
	return buildapiv1.StatusReason(reason), string(message)
}
