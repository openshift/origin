package cmd

import (
	"encoding/json"
	"log"
	"os"

	"github.com/fsouza/go-dockerclient"
	"github.com/openshift/origin/pkg/build/api"
	bld "github.com/openshift/origin/pkg/build/builder"
	"github.com/openshift/origin/pkg/build/builder/cmd/dockercfg"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
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
		log.Fatalf("Error obtaining docker client: %v", err)
	}
	buildStr := os.Getenv("BUILD")
	build := api.Build{}
	err = json.Unmarshal([]byte(buildStr), &build)
	if err != nil {
		log.Fatalf("Unable to parse build: %v", err)
	}
	authcfg, authPresent := dockercfg.NewHelper().GetDockerAuth(build.Parameters.Output.Registry)
	b := builderFactory(client, endpoint, authcfg, authPresent, &build)
	if err = b.Build(); err != nil {
		log.Fatalf("Build error: %v", err)
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
