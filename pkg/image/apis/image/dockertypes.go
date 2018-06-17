package image

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/api/image/docker10"
	public "github.com/openshift/origin/pkg/image/apis/image/docker10"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DockerImage is the type representing a docker image and its various properties when
// retrieved from the Docker client API.
type DockerImage = docker10.DockerImage

// DockerConfig is the list of configuration options used when creating a container.
type DockerConfig = docker10.DockerConfig

// Convert_public_to_api_DockerImage ensures that out has all of the fields set from in or returns
// an error.
func Convert_public_to_api_DockerImage(in *public.DockerImage, out *docker10.DockerImage) error {
	*out = docker10.DockerImage{
		ID:            in.ID,
		Parent:        in.Parent,
		Comment:       in.Comment,
		Created:       metav1.Time{Time: in.Created},
		Container:     in.Container,
		DockerVersion: in.DockerVersion,
		Author:        in.Author,
		Architecture:  in.Architecture,
		Size:          in.Size,
	}
	if err := Convert_public_to_api_DockerConfig(&in.ContainerConfig, &out.ContainerConfig); err != nil {
		return err
	}
	if in.Config != nil {
		out.Config = &docker10.DockerConfig{}
		if err := Convert_public_to_api_DockerConfig(in.Config, out.Config); err != nil {
			return err
		}
	}
	return nil
}

// Convert_imageconfig_to_api_DockerImage takes a Docker registry digest (schema 2.1) and converts it
// to the external API version of Image.
func Convert_compatibility_to_api_DockerImage(in *public.DockerV1CompatibilityImage, out *docker10.DockerImage) error {
	*out = docker10.DockerImage{
		ID:            in.ID,
		Parent:        in.Parent,
		Comment:       in.Comment,
		Created:       metav1.Time{Time: in.Created},
		Container:     in.Container,
		DockerVersion: in.DockerVersion,
		Author:        in.Author,
		Architecture:  in.Architecture,
		Size:          in.Size,
	}
	if err := Convert_public_to_api_DockerConfig(&in.ContainerConfig, &out.ContainerConfig); err != nil {
		return err
	}
	if in.Config != nil {
		out.Config = &docker10.DockerConfig{}
		if err := Convert_public_to_api_DockerConfig(in.Config, out.Config); err != nil {
			return err
		}
	}
	return nil
}

// Convert_imageconfig_to_api_DockerImage takes a Docker registry digest (schema 2.2) and converts it
// to the external API version of Image.
func Convert_imageconfig_to_api_DockerImage(in *public.DockerImageConfig, out *docker10.DockerImage) error {
	*out = docker10.DockerImage{
		ID:            in.ID,
		Parent:        in.Parent,
		Comment:       in.Comment,
		Created:       metav1.Time{Time: in.Created},
		Container:     in.Container,
		DockerVersion: in.DockerVersion,
		Author:        in.Author,
		Architecture:  in.Architecture,
		Size:          in.Size,
	}
	if err := Convert_public_to_api_DockerConfig(&in.ContainerConfig, &out.ContainerConfig); err != nil {
		return err
	}
	if in.Config != nil {
		out.Config = &docker10.DockerConfig{}
		if err := Convert_public_to_api_DockerConfig(in.Config, out.Config); err != nil {
			return err
		}
	}
	return nil
}

// Convert_api_to_public_DockerImage ensures that out has all of the fields set from in or returns
// an error.
func Convert_api_to_public_DockerImage(in *docker10.DockerImage, out *public.DockerImage) error {
	*out = public.DockerImage{
		ID:            in.ID,
		Parent:        in.Parent,
		Comment:       in.Comment,
		Created:       in.Created.Time,
		Container:     in.Container,
		DockerVersion: in.DockerVersion,
		Author:        in.Author,
		Architecture:  in.Architecture,
		Size:          in.Size,
	}
	if err := Convert_api_to_public_DockerConfig(&in.ContainerConfig, &out.ContainerConfig); err != nil {
		return err
	}
	if in.Config != nil {
		out.Config = &public.DockerConfig{}
		if err := Convert_api_to_public_DockerConfig(in.Config, out.Config); err != nil {
			return err
		}
	}
	return nil
}

// Convert_public_to_api_DockerConfig ensures that out has all of the fields set from in or returns
// an error.
func Convert_public_to_api_DockerConfig(in *public.DockerConfig, out *docker10.DockerConfig) error {
	*out = docker10.DockerConfig{
		Hostname:        in.Hostname,
		Domainname:      in.Domainname,
		User:            in.User,
		Memory:          in.Memory,
		MemorySwap:      in.MemorySwap,
		CPUShares:       in.CPUShares,
		CPUSet:          in.CPUSet,
		AttachStdin:     in.AttachStdin,
		AttachStdout:    in.AttachStdout,
		AttachStderr:    in.AttachStderr,
		PortSpecs:       in.PortSpecs,
		ExposedPorts:    in.ExposedPorts,
		Tty:             in.Tty,
		OpenStdin:       in.OpenStdin,
		StdinOnce:       in.StdinOnce,
		Env:             in.Env,
		Cmd:             in.Cmd,
		DNS:             in.DNS,
		Image:           in.Image,
		Volumes:         in.Volumes,
		VolumesFrom:     in.VolumesFrom,
		WorkingDir:      in.WorkingDir,
		Entrypoint:      in.Entrypoint,
		NetworkDisabled: in.NetworkDisabled,
		SecurityOpts:    in.SecurityOpts,
		OnBuild:         in.OnBuild,
		Labels:          in.Labels,
	}
	return nil
}

// Convert_api_to_public_DockerConfig ensures that out has all of the fields set from in or returns
// an error.
func Convert_api_to_public_DockerConfig(in *docker10.DockerConfig, out *public.DockerConfig) error {
	*out = public.DockerConfig{
		Hostname:        in.Hostname,
		Domainname:      in.Domainname,
		User:            in.User,
		Memory:          in.Memory,
		MemorySwap:      in.MemorySwap,
		CPUShares:       in.CPUShares,
		CPUSet:          in.CPUSet,
		AttachStdin:     in.AttachStdin,
		AttachStdout:    in.AttachStdout,
		AttachStderr:    in.AttachStderr,
		PortSpecs:       in.PortSpecs,
		ExposedPorts:    in.ExposedPorts,
		Tty:             in.Tty,
		OpenStdin:       in.OpenStdin,
		StdinOnce:       in.StdinOnce,
		Env:             in.Env,
		Cmd:             in.Cmd,
		DNS:             in.DNS,
		Image:           in.Image,
		Volumes:         in.Volumes,
		VolumesFrom:     in.VolumesFrom,
		WorkingDir:      in.WorkingDir,
		Entrypoint:      in.Entrypoint,
		NetworkDisabled: in.NetworkDisabled,
		SecurityOpts:    in.SecurityOpts,
		OnBuild:         in.OnBuild,
		Labels:          in.Labels,
	}
	return nil
}
