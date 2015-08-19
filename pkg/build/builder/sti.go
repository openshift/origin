package builder

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/golang/glog"
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
	var push bool

	// if there is no output target, set one up so the docker build logic
	// will still work, but we won't push it at the end.
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
		BuilderImage:  s.build.Spec.Strategy.SourceStrategy.From.Name,
		DockerConfig:  &stiapi.DockerConfig{Endpoint: s.dockerSocket},
		Source:        s.build.Spec.Source.Git.URI,
		ContextDir:    s.build.Spec.Source.ContextDir,
		DockerCfgPath: os.Getenv(dockercfg.PullAuthType),
		Tag:           tag,
		ScriptsURL:    s.build.Spec.Strategy.SourceStrategy.Scripts,
		Environment:   getBuildEnvVars(s.build),
		Incremental:   s.build.Spec.Strategy.SourceStrategy.Incremental,
		ForcePull:     s.build.Spec.Strategy.SourceStrategy.ForcePull,
	}
	if s.build.Spec.Revision != nil && s.build.Spec.Revision.Git != nil &&
		s.build.Spec.Revision.Git.Commit != "" {
		config.Ref = s.build.Spec.Revision.Git.Commit
	} else if s.build.Spec.Source.Git.Ref != "" {
		config.Ref = s.build.Spec.Source.Git.Ref
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

	origProxy := make(map[string]string)
	var setHttp, setHttps bool
	// set the http proxy to be used by the git clone performed by S2I
	if len(s.build.Spec.Source.Git.HTTPSProxy) != 0 {
		glog.V(2).Infof("Setting https proxy variables for Git to %s", s.build.Spec.Source.Git.HTTPSProxy)
		origProxy["HTTPS_PROXY"] = os.Getenv("HTTPS_PROXY")
		origProxy["https_proxy"] = os.Getenv("https_proxy")
		os.Setenv("HTTPS_PROXY", s.build.Spec.Source.Git.HTTPSProxy)
		os.Setenv("https_proxy", s.build.Spec.Source.Git.HTTPSProxy)
		setHttps = true
	}
	if len(s.build.Spec.Source.Git.HTTPProxy) != 0 {
		glog.V(2).Infof("Setting http proxy variables for Git to %s", s.build.Spec.Source.Git.HTTPProxy)
		origProxy["HTTP_PROXY"] = os.Getenv("HTTP_PROXY")
		origProxy["http_proxy"] = os.Getenv("http_proxy")
		os.Setenv("HTTP_PROXY", s.build.Spec.Source.Git.HTTPProxy)
		os.Setenv("http_proxy", s.build.Spec.Source.Git.HTTPProxy)
		setHttp = true
	}

	if _, err = builder.Build(config); err != nil {
		return err
	}

	// reset http proxy env variables to original value
	if setHttps {
		glog.V(4).Infof("Resetting HTTPS_PROXY variable for Git to %s", origProxy["HTTPS_PROXY"])
		os.Setenv("HTTPS_PROXY", origProxy["HTTPS_PROXY"])
		glog.V(4).Infof("Resetting https_proxy variable for Git to %s", origProxy["https_proxy"])
		os.Setenv("https_proxy", origProxy["https_proxy"])
	}
	if setHttp {
		glog.V(4).Infof("Resetting HTTP_PROXY variable for Git to %s", origProxy["HTTP_PROXY"])
		os.Setenv("HTTP_PROXY", origProxy["HTTP_PROXY"])
		glog.V(4).Infof("Resetting http_proxy variable for Git to %s", origProxy["http_proxy"])
		os.Setenv("http_proxy", origProxy["http_proxy"])
	}

	if push {
		// Get the Docker push authentication
		pushAuthConfig, authPresent := dockercfg.NewHelper().GetDockerAuth(
			tag,
			dockercfg.PushAuthType,
		)
		if authPresent {
			glog.Infof("Using provided push secret for pushing %s image", tag)
			s.auth = pushAuthConfig
		}
		glog.Infof("Pushing %s image ...", tag)
		if err := pushImage(s.dockerClient, tag, s.auth); err != nil {
			return fmt.Errorf("Failed to push image: %v", err)
		}
		glog.Infof("Successfully pushed %s", tag)
		glog.Flush()
	}
	return nil
}
