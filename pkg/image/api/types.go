package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
)

// ImageList is a list of Image objects.
type ImageList struct {
	kapi.TypeMeta
	kapi.ListMeta

	Items []Image
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

	// DefaultImageTag is used when an image tag is needed and the configuration does not specify a tag to use.
	DefaultImageTag = "latest"
)

// ImageStatus is information about the current status of an Image.
type ImageStatus struct {
	// Phase is the current lifecycle phase of the image.
	Phase string
}

// These are the valid phases of an image.
const (
	// ImageActive means the image is available for use in the system
	ImageAvailable string = "Available"
	// ImagePurging means the image is going to be deleted during next run of registry's pruner
	ImagePurging string = "Purging"
)

// Image is an immutable representation of a Docker image and metadata at a point in time.
type Image struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// The string that can be used to pull this image.
	DockerImageReference string
	// Metadata about this image
	DockerImageMetadata DockerImage
	// This attribute conveys the version of docker metadata the JSON should be stored in, which if empty defaults to "1.0"
	DockerImageMetadataVersion string
	// The raw JSON of the manifest
	DockerImageManifest string
	// Finalizers is an opaque list of values that must be empty to permanently remove object from storage
	Finalizers []kapi.FinalizerName
	// Status describes the current status of an Image
	Status ImageStatus
}

// ImageStreamList is a list of ImageStream objects.
type ImageStreamList struct {
	kapi.TypeMeta
	kapi.ListMeta

	Items []ImageStream
}

// ImageStream stores a mapping of tags to images, metadata overrides that are applied
// when images are tagged in a stream, and an optional reference to a Docker image
// repository on a registry.
type ImageStream struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Spec describes the desired state of this stream
	Spec ImageStreamSpec
	// Status describes the current state of this stream
	Status ImageStreamStatus
}

// ImageStreamSpec represents options for ImageStreams.
type ImageStreamSpec struct {
	// Optional, if specified this stream is backed by a Docker repository on this server
	DockerImageRepository string
	// Tags map arbitrary string values to specific image locators
	Tags map[string]TagReference
}

// TagReference specifies optional annotations for images using this tag and an optional reference to an ImageStreamTag, ImageStreamImage, or DockerImage this tag should track.
type TagReference struct {
	// Optional; if specified, annotations that are applied to images retrieved via ImageStreamTags.
	Annotations map[string]string
	// Optional; if specified, a reference to another image that this tag should point to. Valid values are ImageStreamTag, ImageStreamImage, and DockerImage.
	From *kapi.ObjectReference
}

// ImageStreamStatus contains information about the state of this image stream.
type ImageStreamStatus struct {
	// DockerImageRepository represents the effective location this stream may be accessed at. May be empty until the server
	// determines where the repository is located
	DockerImageRepository string
	// A historical record of images associated with each tag. The first entry in the TagEvent array is
	// the currently tagged image.
	Tags map[string]TagEventList
}

// TagEventList contains a historical record of images associated with a tag.
type TagEventList struct {
	Items []TagEvent
}

// TagEvent is used by ImageRepositoryStatus to keep a historical record of images associated with a tag.
type TagEvent struct {
	// When the TagEvent was created
	Created util.Time
	// The string that can be used to pull this image
	DockerImageReference string
	// The image
	Image string
}

// ImageStreamMapping represents a mapping from a single tag to a Docker image as
// well as the reference to the Docker image repository the image came from.
type ImageStreamMapping struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// The Docker image repository the specified image is located in
	// DEPRECATED: remove once v1beta1 support is dropped
	DockerImageRepository string
	// A Docker image.
	Image Image
	// A string value this image can be located with inside the repository.
	Tag string
}

type ImageStreamTag struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// The Image associated with the ImageStream and tag.
	Image Image
}

// ImageStreamImageList is a list of image stream image objects.
type ImageStreamImageList struct {
	kapi.TypeMeta
	kapi.ListMeta

	Items []ImageStreamImage
}

// ImageStreamImage represents an Image that is retrieved by image name from an ImageStream.
type ImageStreamImage struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// The Image associated with the ImageStream and image name.
	Image Image
}

// ImageStreamDeletionList is a list of image stream deletion objects.
type ImageStreamDeletionList struct {
	kapi.TypeMeta
	kapi.ListMeta

	Items []ImageStreamDeletion
}

// ImageStreamDeletion represents an ImageStream that have been deleted from
// etcd store and is awaiting a garbage collection in internal registry.
type ImageStreamDeletion struct {
	kapi.TypeMeta
	kapi.ObjectMeta
}

// DockerImageReference points to a Docker image.
type DockerImageReference struct {
	Registry  string
	Namespace string
	Name      string
	Tag       string
	ID        string
}
