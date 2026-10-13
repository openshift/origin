package dockerv1client

import "github.com/openshift/api/image/docker10"

// Convert_DockerV1CompatibilityImage_to_DockerImageConfig takes a Docker registry digest
// (schema 2.1) and converts it to the external API version of Image.
func Convert_DockerV1CompatibilityImage_to_DockerImageConfig(in *DockerV1CompatibilityImage, out *DockerImageConfig) error {
	*out = DockerImageConfig{
		ID:              in.ID,
		Parent:          in.Parent,
		Comment:         in.Comment,
		Created:         in.Created,
		Container:       in.Container,
		DockerVersion:   in.DockerVersion,
		Author:          in.Author,
		Architecture:    in.Architecture,
		Size:            in.Size,
		OS:              "linux",
		ContainerConfig: in.ContainerConfig,
	}
	if in.Config != nil {
		out.Config = &docker10.DockerConfig{}
		*out.Config = *in.Config
	}
	return nil
}
