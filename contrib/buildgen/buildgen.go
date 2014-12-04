// buildgen is a command line tool to generate a OpenShift
// build configuration using a source repository
//
// Build configs can then be edited by a human
//
// Example usage:
//
// $ buildgen https://github.com/openshift/ruby-hello-world.git > ruby-docker-bldcfg.json
// Generates a Docker build configuration with an output image named ruby-hello-world
//
// $ buildgen https://github.com/pmorie/simple-ruby.git \
//   openshift/ruby-20-centos -output=test/my-ruby-test > ruby-sti-bldcfg.json
// Generates a STI build configuration with an output image named test/my-ruby-test

package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	api "github.com/openshift/origin/pkg/build/api/v1beta1"

	"gopkg.in/v1/yaml"
)

const usage = "usage: buildgen [-json|-yaml] -output=name -type=[sti|docker] [src] [sti-builder-image]"

var generateJSON = flag.Bool("json", true, "generate json manifest")
var generateYAML = flag.Bool("yaml", false, "generate yaml manifest")
var outputName = flag.String("output", "", "set pod name")
var buildType = flag.String("type", "", "set build type")

func inferNameFromRepository(uri string) string {
	parts := strings.Split(uri, "/")
	lastSegment := parts[len(parts)-1]
	parts = strings.Split(lastSegment, ".")
	return parts[0]
}

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatal(usage)
	}
	if *outputName == "" {
		*outputName = inferNameFromRepository(flag.Arg(0))
	}
	if *buildType == "" {
		if flag.NArg() == 1 {
			*buildType = "docker"
		}
		if flag.NArg() == 2 {
			*buildType = "sti"
		}
	}
	var buildcfg *api.BuildConfig = &api.BuildConfig{
		TypeMeta: kapi.TypeMeta{
			Kind:       "BuildConfig",
			APIVersion: "v1beta1",
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					URI: flag.Arg(0),
				},
			},
			Output: api.BuildOutput{
				ImageTag: *outputName,
			},
		},
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GithubWebHookType,
				GithubWebHook: &api.WebHookTrigger{
					Secret: "secret101",
				},
			},
			{
				Type: api.GenericWebHookType,
				GenericWebHook: &api.WebHookTrigger{
					Secret: "secret101",
				},
			},
		},
	}

	switch *buildType {
	case "docker":
		buildcfg.Parameters.Strategy = api.BuildStrategy{
			Type: api.DockerBuildStrategyType,
		}

	case "sti":
		if flag.NArg() < 2 {
			log.Printf("Missing STI builder image name")
			log.Fatal(usage)
		}
		buildcfg.Parameters.Strategy = api.BuildStrategy{
			Type: api.STIBuildStrategyType,
			STIStrategy: &api.STIBuildStrategy{
				BuilderImage: flag.Arg(1),
			},
		}
	}

	if *generateJSON {
		bs, err := json.MarshalIndent(buildcfg, "", "  ")
		if err != nil {
			log.Fatalf("failed to render JSON build config: %v", err)
		}
		os.Stdout.Write(bs)
	}
	if *generateYAML {
		bs, err := yaml.Marshal(buildcfg)
		if err != nil {
			log.Fatalf("failed to render YAML build config: %v", err)
		}
		os.Stdout.Write(bs)
	}
}
