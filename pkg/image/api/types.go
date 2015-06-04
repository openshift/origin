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

const (
	// ManagedByOpenShiftAnnotation indicates that an image is managed by OpenShift's registry.
	ManagedByOpenShiftAnnotation = "openshift.io/image.managed"

	// DockerImageRepositoryCheckAnnotation indicates that OpenShift has
	// attempted to import tag and image information from an external Docker
	// image repository.
	DockerImageRepositoryCheckAnnotation = "openshift.io/image.dockerRepositoryCheck"

	// InsecureRepositoryAnnotation may be set true on an image stream to allow insecure access to pull content.
	InsecureRepositoryAnnotation = "openshift.io/image.insecureRepository"
)

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
//
// ImageRepositoryList is DEPRECATED; use ImageStreamList instead.
type ImageRepositoryList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	Items []ImageRepository `json:"items"`
}

// ImageStreamList is a list of ImageStream objects.
type ImageStreamList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	Items []ImageStream `json:"items"`
}

// ImageRepository stores a mapping of tags to images, metadata overrides that are applied
// when images are tagged in a repository, and an optional reference to a Docker image
// repository on a registry.
//
// ImageRepository is DEPRECATED; use ImageStream instead.
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

// ImageStream stores a mapping of tags to images, metadata overrides that are applied
// when images are tagged in a stream, and an optional reference to a Docker image
// repository on a registry.
type ImageStream struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Spec describes the desired state of this stream
	Spec ImageStreamSpec `json:"spec"`
	// Status describes the current state of this stream
	Status ImageStreamStatus `json:"status,omitempty"`
}

// ImageStreamSpec represents options for ImageStreams.
type ImageStreamSpec struct {
	// Optional, if specified this stream is backed by a Docker repository on this server
	DockerImageRepository string `json:"dockerImageRepository"`
	// Tags map arbitrary string values to specific image locators
	Tags map[string]TagReference `json:"tags,omitempty"`
}

// TagReference allows a user to TODO.
type TagReference struct {
	Annotations          map[string]string     `json:"annotations,omitempty"`
	DockerImageReference string                `json:"dockerImageReference,omitempty"`
	From                 *kapi.ObjectReference `json:"from,omitempty"`
}

// ImageRepositoryStatus contains information about the state of this image repository.
//
// ImageRepositoryStatus is DEPRECATED; use ImageStreamStatus instead.
type ImageRepositoryStatus struct {
	// Represents the effective location this repository may be accessed at. May be empty until the server
	// determines where the repository is located
	DockerImageRepository string `json:"dockerImageRepository,omitempty"`
	// A historical record of images associated with each tag. The first entry in the TagEvent array is
	// the currently tagged image.
	Tags map[string]TagEventList `json:"tags,omitempty"`
}

// ImageStreamStatus contains information about the state of this image stream.
type ImageStreamStatus struct {
	// Represents the effective location this stream may be accessed at. May be empty until the server
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
//
// ImageRepositoryMapping is DEPRECATED; use ImageStreamMapping instead.
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

// ImageStreamMapping represents a mapping from a single tag to a Docker image as
// well as the reference to the Docker image repository the image came from.
type ImageStreamMapping struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// The Docker image repository the specified image is located in
	// DEPRECATED: remove once v1beta1 support is dropped
	DockerImageRepository string `json:"dockerImageRepository"`
	// A Docker image.
	Image Image `json:"image"`
	// A string value this image can be located with inside the repository.
	Tag string `json:"tag"`
}

// ImageRepositoryTag exists to allow calls to `osc get imageRepositoryTag ...` to function.
//
// ImageRepositoryTag is DEPRECATED; use ImageStreamTag instead.
type ImageRepositoryTag struct {
	Image `json:",inline"`
}

// ImageStreamTag exists to allow calls to `osc get imageStreamTag ...` to function.
type ImageStreamTag struct {
	Image     `json:",inline"`
	ImageName string
}

// DefaultImageTag is used when an image tag is needed and the configuration does not specify a tag to use.
const DefaultImageTag = "latest"

// ImageStreamImage exists to allow calls to `osc get imageStreamImage ...` to function.
type ImageStreamImage struct {
	Image     `json:",inline"`
	ImageName string
}

// DockerImageReference points to a Docker image.
type DockerImageReference struct {
	Registry  string
	Namespace string
	Name      string
	Tag       string
	ID        string
}
