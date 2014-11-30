package v1beta1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/fsouza/go-dockerclient"
)

// ImageList is a list of Image objects.
type ImageList struct {
	kapi.TypeMeta `json:",inline" yaml:",inline"`
	kapi.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Items []Image `json:"items" yaml:"items"`
}

// Image is an immutable representation of a Docker image and metadata at a point in time.
type Image struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// The string that can be used to pull this image.
	DockerImageReference string `json:"dockerImageReference,omitempty" yaml:"dockerImageReference,omitempty"`
	// Metadata about this image
	DockerImageMetadata docker.Image `json:"dockerImageMetadata,omitempty" yaml:"dockerImageMetadata,omitempty"`
}

// ImageRepositoryList is a list of ImageRepository objects.
type ImageRepositoryList struct {
	kapi.TypeMeta `json:",inline" yaml:",inline"`
	kapi.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Items []ImageRepository `json:"items" yaml:"items"`
}

// ImageRepository stores a mapping of tags to images, metadata overrides that are applied
// when images are tagged in a repository, and an optional reference to a Docker image
// repository on a registry.
type ImageRepository struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Optional, if specified this repository is backed by a Docker repository on this server
	DockerImageRepository string `json:"dockerImageRepository,omitempty" yaml:"dockerImageRepository,omitempty"`
	// Tags map arbitrary string values to specific image locators
	Tags map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`

	// Status describes the current state of this repository
	Status ImageRepositoryStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// ImageRepositoryStatus contains information about the state of this image repository.
type ImageRepositoryStatus struct {
	// Represents the effective location this repository may be accessed at. May be empty until the server
	// determines where the repository is located
	DockerImageRepository string `json:"dockerImageRepository,omitempty" yaml:"dockerImageRepository,omitempty"`
}

// TODO add metadata overrides

// ImageRepositoryMapping represents a mapping from a single tag to a Docker image as
// well as the reference to the Docker image repository the image came from.
type ImageRepositoryMapping struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// The Docker image repository the specified image is located in
	DockerImageRepository string `json:"dockerImageRepository" yaml:"dockerImageRepository"`
	// A Docker image.
	Image Image `json:"image" yaml:"image"`
	// A string value this image can be located with inside the repository.
	Tag string `json:"tag" yaml:"tag"`
}
