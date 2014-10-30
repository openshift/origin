package builder

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/source-to-image/pkg/sti"
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
	request := &sti.STIRequest{
		BaseImage:    s.build.Parameters.Strategy.STIStrategy.BuilderImage,
		DockerSocket: s.dockerSocket,
		Source:       s.build.Parameters.Source.Git.URI,
		Tag:          imageTag(s.build),
		Environment:  getBuildEnvVars(s.build),
	}
	if s.build.Parameters.Revision != nil && s.build.Parameters.Revision.Git != nil &&
		s.build.Parameters.Revision.Git.Commit != "" {
		request.Ref = s.build.Parameters.Revision.Git.Commit
	} else if s.build.Parameters.Source.Git.Ref != "" {
		request.Ref = s.build.Parameters.Source.Git.Ref
	}
	builder, err := sti.NewBuilder(request)
	if err != nil {
		return err
	}
	if _, err = builder.Build(); err != nil {
		return err
	}
	if s.build.Parameters.Output.Registry != "" || s.authPresent {
		return pushImage(s.dockerClient, imageTag(s.build), s.auth)
	}
	return nil
}
