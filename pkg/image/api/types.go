package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// ImageList is a list of Image objects.
type ImageList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	Items []Image `json:"items"`
}

// Image is an immutable representation of a Docker image and metadata at a point in time.
type Image struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// The string that can be used to pull this image.
	DockerImageReference string `json:"dockerImageReference,omitempty"`
	// Metadata about this image
	DockerImageMetadata DockerImage `json:"dockerImageMetadata,omitempty"`
	// This attribute conveys the version of docker metadata the JSON should be stored in, which if empty defaults to "1.0"
	DockerImageMetadataVersion string `json:"dockerImageMetadataVersion,omitempty"`
	// The raw JSON of the manifest
	DockerImageManifest string `json:"rawManifest,omitempty"`
}

// ImageRepositoryList is a list of ImageRepository objects.
type ImageRepositoryList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	Items []ImageRepository `json:"items"`
}

// ImageRepository stores a mapping of tags to images, metadata overrides that are applied
// when images are tagged in a repository, and an optional reference to a Docker image
// repository on a registry.
type ImageRepository struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Optional, if specified this repository is backed by a Docker repository on this server
	DockerImageRepository string `json:"dockerImageRepository,omitempty"`
	// Tags map arbitrary string values to specific image locators
	Tags map[string]string `json:"tags,omitempty"`

	// Status describes the current state of this repository
	Status ImageRepositoryStatus `json:"status,omitempty"`
}

// ImageRepositoryStatus contains information about the state of this image repository.
type ImageRepositoryStatus struct {
	// Represents the effective location this repository may be accessed at. May be empty until the server
	// determines where the repository is located
	DockerImageRepository string `json:"dockerImageRepository,omitempty"`
	// A historical record of images associated with each tag. The first entry in the TagEvent array is
	// the currently tagged image.
	Tags map[string]TagEventList `json:"tags,omitempty"`
}

// TagEventList contains a historical record of images associated with a tag.
type TagEventList struct {
	Items []TagEvent `json:"items"`
}

// TagEvent is used by ImageRepositoryStatus to keep a historical record of images associated with a tag.
type TagEvent struct {
	// When the TagEvent was created
	Created util.Time `json:"created"`
	// The string that can be used to pull this image
	DockerImageReference string `json:"dockerImageReference"`
	// The image
	Image string `json:"image"`
}

// ImageRepositoryMapping represents a mapping from a single tag to a Docker image as
// well as the reference to the Docker image repository the image came from.
type ImageRepositoryMapping struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// The Docker image repository the specified image is located in
	DockerImageRepository string `json:"dockerImageRepository"`
	// A Docker image.
	Image Image `json:"image"`
	// A string value this image can be located with inside the repository.
	Tag string `json:"tag"`
}
