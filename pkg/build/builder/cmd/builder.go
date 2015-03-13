package cmd

import (
	"os"

	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/build/api"
	bld "github.com/openshift/origin/pkg/build/builder"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
	image "github.com/openshift/origin/pkg/image/api"
)

const DefaultDockerEndpoint = "unix:///var/run/docker.sock"
const DockerCfgFile = ".dockercfg"

type builder interface {
	Build() error
}
type factoryFunc func(
	client bld.DockerClient,
	dockerSocket string,
	authConfig docker.AuthConfiguration,
	authPresent bool,
	build *api.Build) builder

func run(builderFactory factoryFunc) {
	client, endpoint, err := dockerutil.NewHelper().GetClient()
	if err != nil {
		glog.Fatalf("Error obtaining docker client: %v", err)
	}
	buildStr := os.Getenv("BUILD")
	build := api.Build{}
	if err := latest.Codec.DecodeInto([]byte(buildStr), &build); err != nil {
		glog.Fatalf("Unable to parse build: %v", err)
	}

	var (
		authcfg     docker.AuthConfiguration
		authPresent bool
	)
	output := true
	if len(build.Parameters.Output.DockerImageReference) == 0 {
		if build.Parameters.Output.To != nil {
			glog.Fatalf("Cannot determine an output image reference. Make sure a registry service is running.")
		}
		output = false
	}
	if output {
		ref, err := image.ParseDockerImageReference(build.Parameters.Output.DockerImageReference)
		if err != nil {
			glog.Fatalf("Build output does not have a valid Docker image reference: %v", err)
		}
		authcfg, authPresent = dockercfg.NewHelper().GetDockerAuth(ref.Registry)
	}
	b := builderFactory(client, endpoint, authcfg, authPresent, &build)
	if err = b.Build(); err != nil {
		glog.Fatalf("Build error: %v", err)
	}
	if !output {
		glog.Warning("Build does not have an Output defined, no output image was pushed to a registry.")
	}

}

// RunDockerBuild creates a docker builder and runs its build
func RunDockerBuild() {
	run(func(client bld.DockerClient, sock string, auth docker.AuthConfiguration, present bool, build *api.Build) builder {
		return bld.NewDockerBuilder(client, auth, present, build)
	})
}

// RunSTIBuild creates a STI builder and runs its build
func RunSTIBuild() {
	run(func(client bld.DockerClient, sock string, auth docker.AuthConfiguration, present bool, build *api.Build) builder {
		return bld.NewSTIBuilder(client, sock, auth, present, build)
	})
}
