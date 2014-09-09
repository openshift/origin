package v1beta1

import (
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta1"
	"github.com/fsouza/go-dockerclient"
)

// ImageList is a list of Image objects.
type ImageList struct {
	kubeapi.JSONBase `json:",inline" yaml:",inline"`
	Items            []Image `json:"items,omitempty" yaml:"items,omitempty"`
}

// Image is an immutable representation of a Docker image and metadata at a point in time.
type Image struct {
	kubeapi.JSONBase     `json:",inline" yaml:",inline"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	DockerImageReference string            `json:"dockerImageReference,omitempty" yaml:"dockerImageReference,omitempty"`
	Metadata             docker.Image      `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ImageRepositoryList is a list of ImageRepository objects.
type ImageRepositoryList struct {
	kubeapi.JSONBase `json:",inline" yaml:",inline"`
	Items            []ImageRepository `json:"items,omitempty" yaml:"items,omitempty"`
}

// ImageRepository stores a mapping of tags to images, metadata overrides that are applied
// when images are tagged in a repository, and an optional reference to a Docker image
// repository on a registry.
type ImageRepository struct {
	kubeapi.JSONBase      `json:",inline" yaml:",inline"`
	Labels                map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	DockerImageRepository string            `json:"dockerImageRepository,omitempty" yaml:"dockerImageRepository,omitempty"`
	Tags                  map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// TODO add metadata overrides

// ImageRepositoryMapping represents a mapping from a single tag to a Docker image as
// well as the reference to the Docker image repository the image came from.
type ImageRepositoryMapping struct {
	kubeapi.JSONBase      `json:",inline" yaml:",inline"`
	DockerImageRepository string `json:"dockerImageRepository" yaml:"dockerImageRepository"`
	Image                 Image  `json:"image" yaml:"image"`
	Tag                   string `json:"tag" yaml:"tag"`
}
