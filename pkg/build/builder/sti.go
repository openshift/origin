package builder

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	image "github.com/openshift/origin/pkg/image/api"
	stiapi "github.com/openshift/source-to-image/pkg/api"
	sti "github.com/openshift/source-to-image/pkg/build/strategies"

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
	request := &stiapi.Request{
		BaseImage:    s.build.Parameters.Strategy.STIStrategy.From.Name,
		DockerSocket: s.dockerSocket,
		Source:       s.build.Parameters.Source.Git.URI,
		ContextDir:   s.build.Parameters.Source.ContextDir,
		Tag:          tag,
		ScriptsURL:   s.build.Parameters.Strategy.STIStrategy.Scripts,
		Environment:  getBuildEnvVars(s.build),
		Incremental:  s.build.Parameters.Strategy.STIStrategy.Incremental,
	}

	if s.build.Parameters.Revision != nil && s.build.Parameters.Revision.Git != nil &&
		s.build.Parameters.Revision.Git.Commit != "" {
		request.Ref = s.build.Parameters.Revision.Git.Commit
	} else if s.build.Parameters.Source.Git.Ref != "" {
		request.Ref = s.build.Parameters.Source.Git.Ref
	}
	glog.V(2).Infof("Creating a new STI builder with build request: %#v\n", request)
	builder, err := sti.GetStrategy(request)
	if err != nil {
		return err
	}
	defer removeImage(s.dockerClient, tag)
	if _, err = builder.Build(request); err != nil {
		return err
	}
	dockerImageRef := s.build.Parameters.Output.DockerImageReference
	if len(dockerImageRef) != 0 {
		ref, err := image.ParseDockerImageReference(dockerImageRef)
		if err != nil {
			glog.Fatalf("Build output does not have a valid Docker image reference: %v", err)
		}
		// Get the Docker push authentication
		pushAuthConfig, authPresent := dockercfg.NewHelper().GetDockerAuth(
			ref.Registry,
			dockercfg.PushAuthType,
		)
		if authPresent {
			glog.V(3).Infof("Using Docker authentication provided")
			s.auth = pushAuthConfig
		}
		glog.Infof("Pushing %s image ...", dockerImageRef)
		if err := pushImage(s.dockerClient, tag, s.auth); err != nil {
			glog.Errorf("Failed to push image: %v", err)
			return nil
		}
		glog.Infof("Successfully pushed %s", dockerImageRef)
	}
	return nil
}
