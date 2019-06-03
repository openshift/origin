package dockerv1client

import (
	docker "github.com/fsouza/go-dockerclient"
)

// convertLegacyConfigToDockerConfig converts the docker012 (legacy) config to docker10 config.
func convertLegacyConfigToDockerConfig(config *docker.Config, dockerConfig *DockerConfig) {
	dockerConfig.Hostname = config.Hostname
	dockerConfig.Domainname = config.Domainname
	dockerConfig.User = config.User
	dockerConfig.Memory = config.Memory
	dockerConfig.MemorySwap = config.MemorySwap
	dockerConfig.CPUShares = config.CPUShares
	dockerConfig.CPUSet = config.CPUSet
	dockerConfig.AttachStdin = config.AttachStdin
	dockerConfig.AttachStdout = config.AttachStdout
	dockerConfig.AttachStderr = config.AttachStderr
	dockerConfig.PortSpecs = config.PortSpecs

	if dockerConfig.ExposedPorts == nil {
		dockerConfig.ExposedPorts = map[string]struct{}{}
	}
	for portName, port := range config.ExposedPorts {
		dockerConfig.ExposedPorts[string(portName)] = port
	}

	dockerConfig.Tty = config.Tty
	dockerConfig.OpenStdin = config.OpenStdin
	dockerConfig.StdinOnce = config.StdinOnce
	dockerConfig.Env = config.Env
	dockerConfig.Cmd = config.Cmd
	dockerConfig.DNS = config.DNS
	dockerConfig.Image = config.Image
	dockerConfig.Volumes = config.Volumes
	dockerConfig.VolumesFrom = config.VolumesFrom
	dockerConfig.WorkingDir = config.WorkingDir
	dockerConfig.Entrypoint = config.Entrypoint
	dockerConfig.NetworkDisabled = config.NetworkDisabled
	dockerConfig.SecurityOpts = config.SecurityOpts
	dockerConfig.OnBuild = config.OnBuild
	dockerConfig.Labels = config.Labels
}
