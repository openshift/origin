package builder

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/golang/glog"
	image "github.com/openshift/origin/pkg/image/api"
	stiapi "github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/describe"
	"github.com/openshift/source-to-image/pkg/api/validation"
	sti "github.com/openshift/source-to-image/pkg/build/strategies"
	stidocker "github.com/openshift/source-to-image/pkg/docker"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
)

// STIBuilder performs an STI build given the build object
type STIBuilder struct {
	dockerClient DockerClient
	dockerSocket string
	authPresent  bool
	auth         docker.AuthConfiguration
	build        *api.Build
}

// NewSTIBuilder creates a new STIBuilder instance
func NewSTIBuilder(client DockerClient, dockerSocket string, authCfg docker.AuthConfiguration, authPresent bool, build *api.Build) *STIBuilder {
	return &STIBuilder{
		dockerClient: client,
		dockerSocket: dockerSocket,
		authPresent:  authPresent,
		auth:         authCfg,
		build:        build,
	}
}

// Build executes the STI build
func (s *STIBuilder) Build() error {
	tag := s.build.Parameters.Output.DockerImageReference
	config := &stiapi.Config{
		BuilderImage:  s.build.Parameters.Strategy.SourceStrategy.From.Name,
		DockerConfig:  &stiapi.DockerConfig{Endpoint: s.dockerSocket},
		Source:        s.build.Parameters.Source.Git.URI,
		ContextDir:    s.build.Parameters.Source.ContextDir,
		DockerCfgPath: os.Getenv(dockercfg.PullAuthType),
		Tag:           tag,
		ScriptsURL:    s.build.Parameters.Strategy.SourceStrategy.Scripts,
		Environment:   getBuildEnvVars(s.build),
		Incremental:   s.build.Parameters.Strategy.SourceStrategy.Incremental,
	}

	if s.build.Parameters.Revision != nil && s.build.Parameters.Revision.Git != nil &&
		s.build.Parameters.Revision.Git.Commit != "" {
		config.Ref = s.build.Parameters.Revision.Git.Commit
	} else if s.build.Parameters.Source.Git.Ref != "" {
		config.Ref = s.build.Parameters.Source.Git.Ref
	}

	if errs := validation.ValidateConfig(config); len(errs) != 0 {
		var buffer bytes.Buffer
		for _, ve := range errs {
			buffer.WriteString(ve.Error())
			buffer.WriteString(", ")
		}
		return errors.New(buffer.String())
	}

	// If DockerCfgPath is provided in api.Config, then attempt to read the the
	// dockercfg file and get the authentication for pulling the builder image.
	if r, err := os.Open(config.DockerCfgPath); err == nil {
		config.PullAuthentication = stidocker.GetImageRegistryAuth(r, config.BuilderImage)
		glog.Infof("Using provided pull secret for pulling %s image", config.BuilderImage)
	}
	glog.V(2).Infof("Creating a new S2I builder with build config: %#v\n", describe.DescribeConfig(config))
	builder, err := sti.GetStrategy(config)
	if err != nil {
		return err
	}
	defer removeImage(s.dockerClient, tag)
	glog.V(4).Infof("Starting S2I build from %s/%s BuildConfig ...", s.build.Namespace, s.build.Name)
	if _, err = builder.Build(config); err != nil {
		return err
	}
	dockerImageRef := s.build.Parameters.Output.DockerImageReference
	if len(dockerImageRef) != 0 {
		ref, err := image.ParseDockerImageReference(dockerImageRef)
		if err != nil {
			glog.Fatalf("Build %s/%s output does not have a valid DockerImageReference: %v", s.build.Namespace, s.build.Name, err)
		}
		// Get the Docker push authentication
		pushAuthConfig, authPresent := dockercfg.NewHelper().GetDockerAuth(
			ref.Registry,
			dockercfg.PushAuthType,
		)
		if authPresent {
			glog.Infof("Using provided push secret for pushing %s image", config.BuilderImage)
			s.auth = pushAuthConfig
		}
		glog.Infof("Pushing %s image ...", dockerImageRef)
		if err := pushImage(s.dockerClient, tag, s.auth); err != nil {
			return fmt.Errorf("Failed to push image: %v", err)
		}
		glog.Infof("Successfully pushed %s", dockerImageRef)
		glog.Flush()
	}
	return nil
}
