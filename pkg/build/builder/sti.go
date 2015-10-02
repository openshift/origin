package builder

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/golang/glog"
	stiapi "github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/describe"
	"github.com/openshift/source-to-image/pkg/api/validation"
	"github.com/openshift/source-to-image/pkg/build"
	sti "github.com/openshift/source-to-image/pkg/build/strategies"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
)

// internal interface to decouple S2I-specific code from Origin builder code
type stiBuilderFactory interface {
	// retrieve S2I Builder based on S2I configuration
	GetStrategy(config *stiapi.Config) (build.Builder, error)
}

// interval interface to decouple S2I-specific code from Origin builder code
type stiConfigValidator interface {
	ValidateConfig(config *stiapi.Config) []validation.ValidationError
}

// default implementation of stiBuilderFactory
type runtimeBuilderFactory struct{}

// default implementation of stiConfigValidator
type runtimeConfigValidator struct{}

// default implementation of stiBuildFactory.GetStrategy method. Just delegates to S2I-specific code
func (_ runtimeBuilderFactory) GetStrategy(config *stiapi.Config) (build.Builder, error) {
	return sti.GetStrategy(config)
}

// default implementation of stiConfigValidator.ValidateConfig method. Just delegates to S2I-specific code
func (_ runtimeConfigValidator) ValidateConfig(config *stiapi.Config) []validation.ValidationError {
	return validation.ValidateConfig(config)
}

// STIBuilder performs an STI build given the build object
type STIBuilder struct {
	dockerClient    DockerClient
	dockerSocket    string
	build           *api.Build
	builderFactory  stiBuilderFactory
	configValidator stiConfigValidator
}

// NewSTIBuilder creates a new STIBuilder instance
func NewSTIBuilder(client DockerClient, dockerSocket string, build *api.Build) *STIBuilder {
	// delegate to internal implementation passing default implementation of stiBuilderFactory and stiConfigValidator
	return newSTIBuilder(client, dockerSocket, build,
		new(runtimeBuilderFactory), new(runtimeConfigValidator))

}

// internal factory function to create STIBuilder based on arameters. Used for testing.
func newSTIBuilder(client DockerClient, dockerSocket string, build *api.Build,
	builderFactory stiBuilderFactory, configValidator stiConfigValidator) *STIBuilder {
	return &STIBuilder{
		dockerClient:    client,
		dockerSocket:    dockerSocket,
		build:           build,
		builderFactory:  builderFactory,
		configValidator: configValidator,
	}
}

// Build executes the STI build
func (s *STIBuilder) Build() error {
	// delegate to internal method
	return internalBuild(s, s.builderFactory, s.configValidator)
}

// executes STI build based on configured builder, S2I builder factory and S2I config validator
func internalBuild(s *STIBuilder,
	builderFactory stiBuilderFactory,
	configValidator stiConfigValidator) error {

	var push bool

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

	config := &stiapi.Config{
		BuilderImage:   s.build.Spec.Strategy.SourceStrategy.From.Name,
		DockerConfig:   &stiapi.DockerConfig{Endpoint: s.dockerSocket},
		Source:         s.build.Spec.Source.Git.URI,
		ContextDir:     s.build.Spec.Source.ContextDir,
		DockerCfgPath:  os.Getenv(dockercfg.PullAuthType),
		Tag:            tag,
		ScriptsURL:     s.build.Spec.Strategy.SourceStrategy.Scripts,
		Environment:    buildEnvVars(s.build),
		LabelNamespace: api.DefaultDockerLabelNamespace,
		Incremental:    s.build.Spec.Strategy.SourceStrategy.Incremental,
		ForcePull:      s.build.Spec.Strategy.SourceStrategy.ForcePull,
	}
	if s.build.Spec.Revision != nil && s.build.Spec.Revision.Git != nil &&
		s.build.Spec.Revision.Git.Commit != "" {
		config.Ref = s.build.Spec.Revision.Git.Commit
	} else if s.build.Spec.Source.Git.Ref != "" {
		config.Ref = s.build.Spec.Source.Git.Ref
	}

	allowedUIDs := os.Getenv("ALLOWED_UIDS")
	glog.V(2).Infof("The value of ALLOWED_UIDS is [%s]", allowedUIDs)
	if len(allowedUIDs) > 0 {
		err := config.AllowedUIDs.Set(allowedUIDs)
		if err != nil {
			return err
		}
	}

	if errs := configValidator.ValidateConfig(config); len(errs) != 0 {
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
	builder, err := builderFactory.GetStrategy(config)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Starting S2I build from %s/%s BuildConfig ...", s.build.Namespace, s.build.Name)

	// Set the HTTP and HTTPS proxies to be used by the S2I build.
	originalProxies := setHTTPProxy(s.build.Spec.Source.Git.HTTPProxy, s.build.Spec.Source.Git.HTTPSProxy)

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
				glog.Infof("Registry server address: %s", passwordPresent)
			}
			//glog.Info("
			return errors.New(msg)
		}
		glog.Infof("Successfully pushed %s", tag)
		glog.Flush()
	}
	return nil
}

// buildEnvVars returns a map with build metadata to be inserted into Docker
// images produced by build. It transforms the output from buildInfo into the
// input format expected by stiapi.Config.Environment.
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
