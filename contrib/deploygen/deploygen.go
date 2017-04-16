// deploygen is a command line tool to bootstrap a OpenShift
// deployment configuration using image metadata
//
// Example usage:
//
// $ docker pull google/nodejs-hello
// $ deploygen google/nodejs-hello > nodejs-deploy.json

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	dockerclient "github.com/fsouza/go-dockerclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api/v1beta1"

	"gopkg.in/v1/yaml"
)

const usage = "usage: deploygen -name=[deployment-name] username/image1 ... username/imageN"

var generateJSON = flag.Bool("json", true, "generate json manifest")
var generateYAML = flag.Bool("yaml", false, "generate yaml manifest")
var podName = flag.String("name", "", "set deployment name")

func generateManifest(dockerHost string, podName string, imageNames []string) (*kapi.ContainerManifest, error) {
	docker, err := dockerclient.NewClient(dockerHost)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %q: %v", dockerHost, err)
	}

	podContainers := []kapi.Container{}

	for _, imageName := range imageNames {
		parts, baseName, _ := parseDockerImage(imageName)
		container := kapi.Container{
			Name:  baseName,
			Image: imageName,
		}

		img, err := docker.InspectImage(imageName)
		if err != nil {
			log.Fatalf("failed to inspect image %q: %v", imageName, err)
		}
		for p := range img.Config.ExposedPorts {
			port, err := strconv.Atoi(p.Port())
			if err != nil {
				log.Fatalf("failed to parse port %q: %v", parts[0], err)
			}
			container.Ports = append(container.Ports, kapi.Port{
				Name:          strings.Join([]string{baseName, p.Proto(), p.Port()}, "-"),
				ContainerPort: port,
				Protocol:      kapi.Protocol(strings.ToUpper(p.Proto())),
			})
		}
		podContainers = append(podContainers, container)
	}

	manifest := &kapi.ContainerManifest{
		Version:    "v1beta1",
		ID:         podName + "-pod",
		Containers: podContainers,
		RestartPolicy: kapi.RestartPolicy{
			Always: &kapi.RestartPolicyAlways{},
		},
	}

	return manifest, nil
}

// parseDockerImage split a docker image name of the form [REGISTRYHOST/][USERNAME/]NAME[:TAG]
// Returns array of images name parts, base image name, and tag
func parseDockerImage(imageName string) (parts []string, baseName string, tag string) {
	// Parse docker image name
	// IMAGE: [REGISTRYHOST/][USERNAME/]NAME[:TAG]
	// NAME: [a-z0-9-_.]
	imageName, tag = dockerclient.ParseRepositoryTag(imageName)
	parts = strings.Split(imageName, "/")
	baseName = parts[len(parts)-1]
	return
}

func imageDeploymentTriggers(manifest *kapi.ContainerManifest) []deployapi.DeploymentTriggerPolicy {
	result := []deployapi.DeploymentTriggerPolicy{}
	for _, container := range manifest.Containers {
		repository, tag := dockerclient.ParseRepositoryTag(container.Image)
		trigger := deployapi.DeploymentTriggerPolicy{
			Type: deployapi.DeploymentTriggerOnImageChange,
			ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:      true,
				ContainerNames: []string{container.Name},
				RepositoryName: repository,
				Tag:            tag,
			},
		}
		result = append(result, trigger)
	}
	return result
}

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatal(usage)
	}
	if *podName == "" {
		if flag.NArg() > 1 {
			log.Fatal(usage)
		}
		_, *podName, _ = parseDockerImage(flag.Arg(0))
	}

	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		log.Fatalf("DOCKER_HOST is not set")
	}

	manifest, err := generateManifest(dockerHost, *podName, flag.Args())
	if err != nil {
		log.Fatalf("%v", err)
	}
	deployment := deployapi.DeploymentConfig{
		TypeMeta: kapi.TypeMeta{
			Kind:       "DeploymentConfig",
			APIVersion: "v1beta1",
		},
		Template: deployapi.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerState{
				Replicas:        1,
				ReplicaSelector: map[string]string{"deployment": *podName + "-deployment"},
				PodTemplate: kapi.PodTemplate{
					DesiredState: kapi.PodState{
						Manifest: *manifest,
					},
					Labels: map[string]string{"deployment": *podName + "-deployment"},
				},
			},
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
			},
		},
		Triggers: imageDeploymentTriggers(manifest),
	}
	if *generateJSON {
		bs, err := json.MarshalIndent(deployment, "", "  ")
		if err != nil {
			log.Fatalf("failed to render JSON deployment config: %v", err)
		}
		os.Stdout.Write(bs)
	}
	if *generateYAML {
		bs, err := yaml.Marshal(deployment)
		if err != nil {
			log.Fatalf("failed to render YAML deployment config: %v", err)
		}
		os.Stdout.Write(bs)
	}
}
