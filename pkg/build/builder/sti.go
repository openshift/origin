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
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"

	s2iapi "github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/describe"
	"github.com/openshift/source-to-image/pkg/api/validation"
	s2ibuild "github.com/openshift/source-to-image/pkg/build"
	s2i "github.com/openshift/source-to-image/pkg/build/strategies"
	"github.com/openshift/source-to-image/pkg/scm/git"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
)

// builderFactory is the internal interface to decouple S2I-specific code from Origin builder code
type builderFactory interface {
	// Create S2I Builder based on S2I configuration
	Builder(config *s2iapi.Config, overrides s2ibuild.Overrides) (s2ibuild.Builder, error)
}

// validator is the interval interface to decouple S2I-specific code from Origin builder code
type validator interface {
	// Perform validation of S2I configuration, returns slice of validation errors
	ValidateConfig(config *s2iapi.Config) []validation.ValidationError
}

// runtimeBuilderFactory is the default implementation of stiBuilderFactory
type runtimeBuilderFactory struct{}

// Builder delegates execution to S2I-specific code
func (_ runtimeBuilderFactory) Builder(config *s2iapi.Config, overrides s2ibuild.Overrides) (s2ibuild.Builder, error) {
	return s2i.Strategy(config, overrides)
}

// runtimeConfigValidator is the default implementation of stiConfigValidator
type runtimeConfigValidator struct{}

// ValidateConfig delegates execution to S2I-specific code
func (_ runtimeConfigValidator) ValidateConfig(config *s2iapi.Config) []validation.ValidationError {
	return validation.ValidateConfig(config)
}

// STIBuilder performs an STI build given the build object
type STIBuilder struct {
	builder   builderFactory
	validator validator
	git       git.Git

	dockerClient DockerClient
	dockerSocket string
	build        *api.Build
}

// NewSTIBuilder creates a new STIBuilder instance
func NewSTIBuilder(client DockerClient, dockerSocket string, build *api.Build) *STIBuilder {
	// delegate to internal implementation passing default implementation of builderFactory and validator
	return newSTIBuilder(client, dockerSocket, build, runtimeBuilderFactory{}, runtimeConfigValidator{})

}

// newSTIBuilder is the internal factory function to create STIBuilder based on parameters. Used for testing.
func newSTIBuilder(client DockerClient, dockerSocket string, build *api.Build,
	builder builderFactory, validator validator) *STIBuilder {
	// just create instance
	return &STIBuilder{
		builder:   builder,
		validator: validator,
		git:       git.New(),

		dockerClient: client,
		dockerSocket: dockerSocket,
		build:        build,
	}
}

// Build executes STI build based on configured builder, S2I builder factory and S2I config validator
func (s *STIBuilder) Build() error {
	var push bool

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
		timeout: urlCheckTimeout,

		dir:        srcDir,
		contextDir: contextDir,
		tmpDir:     tmpDir,
	}

	// if there is no output target, set one up so the docker build logic
	// (which requires a tag) will still work, but we won't push it at the end.
	if s.build.Spec.Output.To == nil || len(s.build.Spec.Output.To.Name) == 0 {
		s.build.Spec.Output.To = &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: noOutputDefaultTag,
		}
		push = false
	} else {
		push = true
	}

	tag := s.build.Spec.Output.To.Name
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

	config := &s2iapi.Config{
		WorkingDir:     buildDir,
		DockerConfig:   &s2iapi.DockerConfig{Endpoint: s.dockerSocket},
		DockerCfgPath:  os.Getenv(dockercfg.PullAuthType),
		LabelNamespace: api.DefaultDockerLabelNamespace,

		ScriptsURL: s.build.Spec.Strategy.SourceStrategy.Scripts,

		BuilderImage: s.build.Spec.Strategy.SourceStrategy.From.Name,
		Incremental:  s.build.Spec.Strategy.SourceStrategy.Incremental,
		ForcePull:    s.build.Spec.Strategy.SourceStrategy.ForcePull,

		Environment:       buildEnvVars(s.build),
		DockerNetworkMode: getDockerNetworkMode(),

		Source:     sourceURI.String(),
		Tag:        tag,
		ContextDir: s.build.Spec.Source.ContextDir,
	}

	allowedUIDs := os.Getenv("ALLOWED_UIDS")
	glog.V(2).Infof("The value of ALLOWED_UIDS is [%s]", allowedUIDs)
	if len(allowedUIDs) > 0 {
		err := config.AllowedUIDs.Set(allowedUIDs)
		if err != nil {
			return err
		}
	}

	if errs := s.validator.ValidateConfig(config); len(errs) != 0 {
		var buffer bytes.Buffer
		for _, ve := range errs {
			buffer.WriteString(ve.Error())
			buffer.WriteString(", ")
		}
		return errors.New(buffer.String())
	}

	// If DockerCfgPath is provided in api.Config, then attempt to read the the
	// dockercfg file and get the authentication for pulling the builder image.
	config.PullAuthentication, _ = dockercfg.NewHelper().GetDockerAuth(config.BuilderImage, dockercfg.PullAuthType)
	config.IncrementalAuthentication, _ = dockercfg.NewHelper().GetDockerAuth(tag, dockercfg.PushAuthType)

	glog.V(2).Infof("Creating a new S2I builder with build config: %#v\n", describe.DescribeConfig(config))
	builder, err := s.builder.Builder(config, s2ibuild.Overrides{Downloader: download})
	if err != nil {
		return err
	}

	glog.V(4).Infof("Starting S2I build from %s/%s BuildConfig ...", s.build.Namespace, s.build.Name)

	// Set the HTTP and HTTPS proxies to be used by the S2I build.
	var originalProxies map[string]string
	if git != nil {
		originalProxies = setHTTPProxy(git.HTTPProxy, git.HTTPSProxy)
	}

	if _, err = builder.Build(config); err != nil {
		return err
	}

	// Reset proxies back to their original value.
	resetHTTPProxy(originalProxies)

	if push {
		// Get the Docker push authentication
		pushAuthConfig, authPresent := dockercfg.NewHelper().GetDockerAuth(
			tag,
			dockercfg.PushAuthType,
		)
		if authPresent {
			glog.Infof("Using provided push secret for pushing %s image", tag)
		} else {
			glog.Infof("No push secret provided")
		}
		glog.Infof("Pushing %s image ...", tag)
		if err := pushImage(s.dockerClient, tag, pushAuthConfig); err != nil {
			// write extended error message to assist in problem resolution
			msg := fmt.Sprintf("Failed to push image. Response from registry is: %v", err)
			if authPresent {
				glog.Infof("Registry server Address: %s", pushAuthConfig.ServerAddress)
				glog.Infof("Registry server User Name: %s", pushAuthConfig.Username)
				glog.Infof("Registry server Email: %s", pushAuthConfig.Email)
				passwordPresent := "<<empty>>"
				if len(pushAuthConfig.Password) > 0 {
					passwordPresent = "<<non-empty>>"
				}
				glog.Infof("Registry server Password: %s", passwordPresent)
			}
			return errors.New(msg)
		}
		glog.Infof("Successfully pushed %s", tag)
		glog.Flush()
	}
	return nil
}

type downloader struct {
	s       *STIBuilder
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
	if err := fetchSource(targetDir, d.s.build, d.timeout, d.in, d.s.git); err != nil {
		return nil, err
	}
	// TODO: allow source info to be overriden by build
	sourceInfo := d.s.git.GetInfo(targetDir)
	sourceInfo.ContextDir = config.ContextDir

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

	return sourceInfo, nil
}

// buildEnvVars returns a map with build metadata to be inserted into Docker
// images produced by build. It transforms the output from buildInfo into the
// input format expected by s2iapi.Config.Environment.
// Note that using a map has at least two downsides:
// 1. The order of metadata KeyValue pairs is lost;
// 2. In case of repeated Keys, the last Value takes precedence right here,
//    instead of deferring what to do with repeated environment variables to the
//    Docker runtime.
func buildEnvVars(build *api.Build) map[string]string {
	bi := buildInfo(build)
	envVars := make(map[string]string, len(bi))
	for _, item := range bi {
		envVars[item.Key] = item.Value
	}
	return envVars
}
