package v1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// ImageList is a list of Image objects.
type ImageList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of images
	Items []Image `json:"items" description:"list of image objects"`
}

// Image is an immutable representation of a Docker image and metadata at a point in time.
type Image struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// DockerImageReference is the string that can be used to pull this image.
	DockerImageReference string `json:"dockerImageReference,omitempty" description:"string that can be used to pull this image"`
	// DockerImageMetadata contains metadata about this image
	DockerImageMetadata runtime.RawExtension `json:"dockerImageMetadata,omitempty" description:"metadata about this image"`
	// DockerImageMetadataVersion conveys the version of the object, which if empty defaults to "1.0"
	DockerImageMetadataVersion string `json:"dockerImageMetadataVersion,omitempty" description:"conveys version of the object, if empty defaults to '1.0'"`
	// DockerImageManifest is the raw JSON of the manifest
	DockerImageManifest string `json:"dockerImageManifest,omitempty" description:"raw JSON of the manifest"`
}

// ImageStreamList is a list of ImageStream objects.
type ImageStreamList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of imageStreams
	Items []ImageStream `json:"items" description:"list of image stream objects"`
}

// ImageStream stores a mapping of tags to images, metadata overrides that are applied
// when images are tagged in a stream, and an optional reference to a Docker image
// repository on a registry.
type ImageStream struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Spec describes the desired state of this stream
	Spec ImageStreamSpec `json:"spec" description:"desired state of the stream"`
	// Status describes the current state of this stream
	Status ImageStreamStatus `json:"status,omitempty" description:"current state of the stream as observed by the system"`
}

// ImageStreamSpec represents options for ImageStreams.
type ImageStreamSpec struct {
	// DockerImageRepository is optional, if specified this stream is backed by a Docker repository on this server
	DockerImageRepository string `json:"dockerImageRepository,omitempty" description:"optional field if specified this stream is backed by a Docker repository on this server"`
	// Tags map arbitrary string values to specific image locators
	Tags []NamedTagReference `json:"tags,omitempty" description:"map arbitrary string values to specific image locators"`
}

// NamedTagReference specifies optional annotations for images using this tag and an optional reference to an ImageStreamTag, ImageStreamImage, or DockerImage this tag should track.
type NamedTagReference struct {
	// Name of the tag
	Name string `json:"name" description:"name of tag"`
	// Annotations associated with images using this tag
	Annotations map[string]string `json:"annotations,omitempty" description:"annotations associated with images using this tag"`
	// From is a reference to an image stream tag or image stream this tag should track
	From *kapi.ObjectReference `json:"from,omitempty" description:"a reference to an image stream tag or image stream this tag should track"`
}

// ImageStreamStatus contains information about the state of this image stream.
type ImageStreamStatus struct {
	// DockerImageRepository represents the effective location this stream may be accessed at.
	// May be empty until the server determines where the repository is located
	DockerImageRepository string `json:"dockerImageRepository" description:"represents the effective location this stream may be accessed at, may be empty until the server determines where the repository is located"`
	// Tags are a historical record of images associated with each tag. The first entry in the
	// TagEvent array is the currently tagged image.
	Tags []NamedTagEventList `json:"tags,omitempty" description:"historical record of images associated with each tag, the first entry is the currently tagged image"`
}

// NamedTagEventList relates a tag to its image history.
type NamedTagEventList struct {
	Tag   string     `json:"tag" description:"the tag"`
	Items []TagEvent `json:"items" description:"list of tag events related to the tag"`
}

// TagEvent is used by ImageStreamStatus to keep a historical record of images associated with a tag.
type TagEvent struct {
	// Created holds the time the TagEvent was created
	Created util.Time `json:"created" description:"when the event was created"`
	// DockerImageReference is the string that can be used to pull this image
	DockerImageReference string `json:"dockerImageReference" description:"the string that can be used to pull this image"`
	// Image is the image
	Image string `json:"image" description:"the image"`
}

// ImageStreamMapping represents a mapping from a single tag to a Docker image as
// well as the reference to the Docker image stream the image came from.
type ImageStreamMapping struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Image is a Docker image.
	Image Image `json:"image" description:"a Docker image"`
	// Tag is a string value this image can be located with inside the stream.
	Tag string `json:"tag" description:"string value this image can be located with inside the stream"`
}

// ImageStreamTag represents an Image that is retrieved by tag name from an ImageStream.
type ImageStreamTag struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Image associated with the ImageStream and tag.
	Image Image `json:"image" description:"the image associated with the ImageStream and tag"`
}

// ImageStreamImage represents an Image that is retrieved by image name from an ImageStream.
type ImageStreamImage struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Image associated with the ImageStream and image name.
	Image Image `json:"image" description:"the image associated with the ImageStream and image name"`
}

// DockerImageReference points to a Docker image.
type DockerImageReference struct {
	Registry  string
	Namespace string
	Name      string
	Tag       string
	ID        string
}
