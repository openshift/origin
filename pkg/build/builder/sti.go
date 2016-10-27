package builder

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	"github.com/openshift/origin/pkg/build/controller/strategy"
	"github.com/openshift/origin/pkg/client"
)

// builderFactory is the internal interface to decouple S2I-specific code from Origin builder code
type builderFactory interface {
	// Create S2I Builder based on S2I configuration
	Builder(config *s2iapi.Config, overrides s2ibuild.Overrides) (s2ibuild.Builder, error)
}

// validator is the interval interface to decouple S2I-specific code from Origin builder code
type validator interface {
	// Perform validation of S2I configuration, returns slice of validation errors
	ValidateConfig(config *s2iapi.Config) []validation.Error
}

// runtimeBuilderFactory is the default implementation of stiBuilderFactory
type runtimeBuilderFactory struct{}

// Builder delegates execution to S2I-specific code
func (_ runtimeBuilderFactory) Builder(config *s2iapi.Config, overrides s2ibuild.Overrides) (s2ibuild.Builder, error) {
	builder, _, err := s2i.Strategy(config, overrides)
	return builder, err
}

// runtimeConfigValidator is the default implementation of stiConfigValidator
type runtimeConfigValidator struct{}

// ValidateConfig delegates execution to S2I-specific code
func (_ runtimeConfigValidator) ValidateConfig(config *s2iapi.Config) []validation.Error {
	return validation.ValidateConfig(config)
}

// S2IBuilder performs an STI build given the build object
type S2IBuilder struct {
	builder   builderFactory
	validator validator
	gitClient GitClient

	dockerClient DockerClient
	dockerSocket string
	build        *api.Build
	client       client.BuildInterface
	cgLimits     *s2iapi.CGroupLimits
}

// NewS2IBuilder creates a new STIBuilder instance
func NewS2IBuilder(dockerClient DockerClient, dockerSocket string, buildsClient client.BuildInterface, build *api.Build, gitClient GitClient, cgLimits *s2iapi.CGroupLimits) *S2IBuilder {
	// delegate to internal implementation passing default implementation of builderFactory and validator
	return newS2IBuilder(dockerClient, dockerSocket, buildsClient, build, gitClient, runtimeBuilderFactory{}, runtimeConfigValidator{}, cgLimits)
}

// newS2IBuilder is the internal factory function to create STIBuilder based on parameters. Used for testing.
func newS2IBuilder(dockerClient DockerClient, dockerSocket string, buildsClient client.BuildInterface, build *api.Build,
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

// Build executes STI build based on configured builder, S2I builder factory and S2I config validator
func (s *S2IBuilder) Build() error {
	if s.build.Spec.Strategy.SourceStrategy == nil {
		return errors.New("the source to image builder must be used with the source strategy")
	}

	contextDir := filepath.Clean(s.build.Spec.Source.ContextDir)
	if contextDir == "." || contextDir == "/" {
		contextDir = ""
	}
	buildDir, err := ioutil.TempDir("", "s2i-build")
	if err != nil {
		return err
	}
	srcDir := filepath.Join(buildDir, s2iapi.Source)
	if err := os.MkdirAll(srcDir, os.ModePerm); err != nil {
		return err
	}
	tmpDir := filepath.Join(buildDir, "tmp")
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return err
	}

	download := &downloader{
		s:       s,
		in:      os.Stdin,
		timeout: initialURLCheckTimeout,

		dir:        srcDir,
		contextDir: contextDir,
		tmpDir:     tmpDir,
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
	git := s.build.Spec.Source.Git

	var ref string
	if s.build.Spec.Revision != nil && s.build.Spec.Revision.Git != nil &&
		len(s.build.Spec.Revision.Git.Commit) != 0 {
		ref = s.build.Spec.Revision.Git.Commit
	} else if git != nil && len(git.Ref) != 0 {
		ref = git.Ref
	}

	sourceURI := &url.URL{
		Scheme:   "file",
		Path:     srcDir,
		Fragment: ref,
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
			scriptDownloadProxyConfig.HTTPProxy,
			scriptDownloadProxyConfig.HTTPSProxy)
	}

	var incremental bool
	if s.build.Spec.Strategy.SourceStrategy.Incremental != nil {
		incremental = *s.build.Spec.Strategy.SourceStrategy.Incremental
	}
	config := &s2iapi.Config{
		WorkingDir:     buildDir,
		DockerConfig:   &s2iapi.DockerConfig{Endpoint: s.dockerSocket},
		DockerCfgPath:  os.Getenv(dockercfg.PullAuthType),
		LabelNamespace: api.DefaultDockerLabelNamespace,

		ScriptsURL: s.build.Spec.Strategy.SourceStrategy.Scripts,

		BuilderImage:       s.build.Spec.Strategy.SourceStrategy.From.Name,
		Incremental:        incremental,
		IncrementalFromTag: pushTag,

		Environment:       buildEnvVars(s.build),
		Labels:            buildLabels(s.build),
		DockerNetworkMode: getDockerNetworkMode(),

		Source:                    sourceURI.String(),
		Tag:                       buildTag,
		ContextDir:                s.build.Spec.Source.ContextDir,
		CGroupLimits:              s.cgLimits,
		Injections:                injections,
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

	allowedUIDs := os.Getenv(api.AllowedUIDs)
	glog.V(4).Infof("The value of %s is [%s]", api.AllowedUIDs, allowedUIDs)
	if len(allowedUIDs) > 0 {
		err := config.AllowedUIDs.Set(allowedUIDs)
		if err != nil {
			return err
		}
	}
	dropCaps := os.Getenv(api.DropCapabilities)
	glog.V(4).Infof("The value of %s is [%s]", api.DropCapabilities, dropCaps)
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
	// If DockerCfgPath is provided in api.Config, then attempt to read the
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

	glog.V(4).Infof("Creating a new S2I builder with build config: %#v\n", describe.Config(config))
	builder, err := s.builder.Builder(config, s2ibuild.Overrides{Downloader: download})
	if err != nil {
		return err
	}

	glog.V(4).Infof("Starting S2I build from %s/%s BuildConfig ...", s.build.Namespace, s.build.Name)

	if _, err = builder.Build(config); err != nil {
		return err
	}

	cname := containerName("s2i", s.build.Name, s.build.Namespace, "post-commit")
	if err := execPostCommitHook(s.dockerClient, s.build.Spec.PostCommit, buildTag, cname); err != nil {
		return err
	}

	if push {
		if err := tagImage(s.dockerClient, buildTag, pushTag); err != nil {
			return err
		}
	}

	if err := removeImage(s.dockerClient, buildTag); err != nil {
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
		if err := pushImage(s.dockerClient, pushTag, pushAuthConfig); err != nil {
			return reportPushFailure(err, authPresent, pushAuthConfig)
		}
		glog.V(0).Infof("Push successful")
	}
	return nil
}

type downloader struct {
	s       *S2IBuilder
	in      io.Reader
	timeout time.Duration

	dir        string
	contextDir string
	tmpDir     string
}

func (d *downloader) Download(config *s2iapi.Config) (*s2iapi.SourceInfo, error) {
	var targetDir string
	if len(d.contextDir) > 0 {
		targetDir = d.tmpDir
	} else {
		targetDir = d.dir
	}

	// fetch source
	sourceInfo, err := fetchSource(d.s.dockerClient, targetDir, d.s.build, d.timeout, d.in, d.s.gitClient)
	if err != nil {
		return nil, err
	}
	if sourceInfo != nil {
		updateBuildRevision(d.s.client, d.s.build, sourceInfo)
	}
	if sourceInfo != nil {
		sourceInfo.ContextDir = config.ContextDir
	}

	// if a context dir is provided, move the context dir contents into the src location
	if len(d.contextDir) > 0 {
		srcDir := filepath.Join(targetDir, d.contextDir)
		if err := os.Remove(d.dir); err != nil {
			return nil, err
		}
		if err := os.Rename(srcDir, d.dir); err != nil {
			return nil, err
		}
	}
	if sourceInfo != nil {
		return &sourceInfo.SourceInfo, nil
	}
	return nil, nil
}

// buildEnvVars returns a map with build metadata to be inserted into Docker
// images produced by build. It transforms the output from buildInfo into the
// input format expected by s2iapi.Config.Environment.
// Note that using a map has at least two downsides:
// 1. The order of metadata KeyValue pairs is lost;
// 2. In case of repeated Keys, the last Value takes precedence right here,
//    instead of deferring what to do with repeated environment variables to the
//    Docker runtime.
func buildEnvVars(build *api.Build) s2iapi.EnvironmentList {
	bi := buildInfo(build)
	envVars := &s2iapi.EnvironmentList{}
	for _, item := range bi {
		envVars.Set(fmt.Sprintf("%s=%s", item.Key, item.Value))
	}
	return *envVars
}

func buildLabels(build *api.Build) map[string]string {
	labels := make(map[string]string)
	for _, lbl := range build.Spec.Output.ImageLabels {
		labels[lbl.Name] = lbl.Value
	}
	return labels
}

// scriptProxyConfig determines a proxy configuration for downloading
// scripts from a URL. For now, it uses environment variables passed in
// the strategy's environment. There is no preference given to either lowercase
// or uppercase form of the variable.
func scriptProxyConfig(build *api.Build) (*s2iapi.ProxyConfig, error) {
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
		proxyURL, err := url.Parse(httpProxy)
		if err != nil {
			return nil, err
		}
		config.HTTPProxy = proxyURL
	}
	if len(httpsProxy) > 0 {
		proxyURL, err := url.Parse(httpsProxy)
		if err != nil {
			return nil, err
		}
		config.HTTPSProxy = proxyURL
	}
	return config, nil
}

// copyToVolumeList copies the artifacts set in the build config to the
// VolumeList struct in the s2iapi.Config
func copyToVolumeList(artifactsMapping []api.ImageSourcePath) (volumeList s2iapi.VolumeList) {
	for _, mappedPath := range artifactsMapping {
		volumeList = append(volumeList, s2iapi.VolumeSpec{
			Source:      mappedPath.SourcePath,
			Destination: mappedPath.DestinationDir,
		})
	}
	return
}
