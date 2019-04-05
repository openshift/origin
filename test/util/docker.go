package util

import (
	dockerClient "github.com/fsouza/go-dockerclient"
)

// newDockerClient creates a docker client using the env var DOCKER_ENDPOINT or, if not supplied, uses the default
// docker endpoint /var/run/docker.sock
func NewDockerClient() (*dockerClient.Client, error) {
	return dockerClient.NewClientFromEnv()
}
