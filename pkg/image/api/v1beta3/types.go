package v1beta3

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1beta3"
	"k8s.io/kubernetes/pkg/runtime"
)

// ImageList is a list of Image objects.
type ImageList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`

	Items []Image `json:"items"`
}

// Image is an immutable representation of a Docker image and metadata at a point in time.
type Image struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// The string that can be used to pull this image.
	DockerImageReference string `json:"dockerImageReference,omitempty"`
	// Metadata about this image
	DockerImageMetadata runtime.RawExtension `json:"dockerImageMetadata,omitempty"`
	// This attribute conveys the version of the object, which if empty defaults to "1.0"
	DockerImageMetadataVersion string `json:"dockerImageMetadataVersion,omitempty"`
	// The raw JSON of the manifest
	DockerImageManifest string `json:"dockerImageManifest,omitempty"`
	// DockerImageLayers represents the layers in the image. May not be set if the image does not define that data.
	DockerImageLayers []ImageLayer `json:"dockerImageLayers"`
}

// ImageLayer represents a single layer of the image. Some images may have multiple layers. Some may have none.
type ImageLayer struct {
	// Name of the layer as defined by the underlying store.
	Name string `json:"name"`
	// Size of the layer as defined by the underlying store.
	Size int64 `json:"size"`
}

// ImageStreamList is a list of ImageStream objects.
type ImageStreamList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`

	Items []ImageStream `json:"items"`
}

// ImageStream stores a mapping of tags to images, metadata overrides that are applied
// when images are tagged in a stream, and an optional reference to a Docker image
// repository on a registry.
type ImageStream struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// Spec describes the desired state of this stream
	Spec ImageStreamSpec `json:"spec"`
	// Status describes the current state of this stream
	Status ImageStreamStatus `json:"status,omitempty"`
}

// ImageStreamSpec represents options for ImageStreams.
type ImageStreamSpec struct {
	// Optional, if specified this stream is backed by a Docker repository on this server
	DockerImageRepository string `json:"dockerImageRepository,omitempty"`
	// Tags map arbitrary string values to specific image locators
	Tags []TagReference `json:"tags,omitempty"`
}

// TagReference specifies optional annotations for images using this tag and an optional reference to an ImageStreamTag, ImageStreamImage, or DockerImage this tag should track.
type TagReference struct {
	Name        string                `json:"name"`
	Annotations map[string]string     `json:"annotations"`
	From        *kapi.ObjectReference `json:"from,omitempty"`
	// Reference states if the tag will be imported. Default value is false, which means the tag will be imported.
	Reference bool `json:"reference,omitempty"`
	// Generation is the image stream generation that updated this tag - setting it to 0 is an indication that the generation must be updated.
	// Legacy clients will send this as nil, which means the client doesn't know or care.
	Generation *int64 `json:"generation"`
	// Import is information that controls how images may be imported by the server.
	ImportPolicy TagImportPolicy `json:"importPolicy,omitempty"`
}

type TagImportPolicy struct {
	// Insecure is true if the server may bypass certificate verification or connect directly over HTTP during image import.
	Insecure bool `json:"insecure,omitempty"`
	// Scheduled indicates to the server that this tag should be periodically checked to ensure it is up to date, and imported
	Scheduled bool `json:"scheduled,omitempty"`
}

// ImageStreamStatus contains information about the state of this image stream.
type ImageStreamStatus struct {
	// Represents the effective location this stream may be accessed at. May be empty until the server
	// determines where the repository is located
	DockerImageRepository string `json:"dockerImageRepository"`
	// A historical record of images associated with each tag. The first entry in the TagEvent array is
	// the currently tagged image.
	Tags []NamedTagEventList `json:"tags,omitempty"`
}

// NamedTagEventList relates a tag to its image history.
type NamedTagEventList struct {
	Tag   string     `json:"tag"`
	Items []TagEvent `json:"items"`
	// Conditions is an array of conditions that apply to the tag event list.
	Conditions []TagEventCondition `json:"conditions"`
}

// TagEvent is used by ImageStreamStatus to keep a historical record of images associated with a tag.
type TagEvent struct {
	// Created holds the time the TagEvent was created
	Created unversioned.Time `json:"created"`
	// DockerImageReference is the string that can be used to pull this image
	DockerImageReference string `json:"dockerImageReference"`
	// Image is the image
	Image string `json:"image"`
	// Generation is the spec tag generation that resulted in this tag being updated
	Generation int64 `json:"generation"`
}

type TagEventConditionType string

// These are valid conditions of TagEvents.
const (
	// ImportSuccess with status False means the import of the specific tag failed
	ImportSuccess TagEventConditionType = "ImportSuccess"
)

// TagEventCondition contains condition information for a tag event.
type TagEventCondition struct {
	// Type of tag event condition, currently only ImportSuccess
	Type TagEventConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status kapi.ConditionStatus `json:"status"`
	// Last time the condition transit from one status to another.
	LastTransitionTime unversioned.Time `json:"lastTransitionTime,omitempty"`
	// (brief) reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
	// Generation is the spec tag generation that this status corresponds to
	Generation int64 `json:"generation"`
}

// ImageStreamMapping represents a mapping from a single tag to a Docker image as
// well as the reference to the Docker image repository the image came from.
type ImageStreamMapping struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// A Docker image.
	Image Image `json:"image"`
	// A string value this image can be located with inside the repository.
	Tag string `json:"tag"`
}

// ImageStreamTag represents an Image that is retrieved by tag name from an ImageStream.
type ImageStreamTag struct {
	Image     `json:",inline"`
	ImageName string `json:"imageName"`
}

// ImageStreamTagList is a list of ImageStreamTag objects.
type ImageStreamTagList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`

	Items []ImageStreamTag `json:"items"`
}

// ImageStreamImage represents an Image that is retrieved by image name from an ImageStream.
type ImageStreamImage struct {
	Image     `json:",inline"`
	ImageName string `json:"imageName"`
}

// DockerImageReference points to a Docker image.
type DockerImageReference struct {
	Registry  string
	Namespace string
	Name      string
	Tag       string
	ID        string
}
