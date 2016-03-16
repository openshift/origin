package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
)

// ImageList is a list of Image objects.
type ImageList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`

	// Items is a list of images
	Items []Image `json:"items"`
}

// Image is an immutable representation of a Docker image and metadata at a point in time.
type Image struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// DockerImageReference is the string that can be used to pull this image.
	DockerImageReference string `json:"dockerImageReference,omitempty"`
	// DockerImageMetadata contains metadata about this image
	DockerImageMetadata runtime.RawExtension `json:"dockerImageMetadata,omitempty"`
	// DockerImageMetadataVersion conveys the version of the object, which if empty defaults to "1.0"
	DockerImageMetadataVersion string `json:"dockerImageMetadataVersion,omitempty"`
	// DockerImageManifest is the raw JSON of the manifest
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
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`

	// Items is a list of imageStreams
	Items []ImageStream `json:"items"`
}

// ImageStream stores a mapping of tags to images, metadata overrides that are applied
// when images are tagged in a stream, and an optional reference to a Docker image
// repository on a registry.
type ImageStream struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Spec describes the desired state of this stream
	Spec ImageStreamSpec `json:"spec"`
	// Status describes the current state of this stream
	Status ImageStreamStatus `json:"status,omitempty"`
}

// ImageStreamSpec represents options for ImageStreams.
type ImageStreamSpec struct {
	// DockerImageRepository is optional, if specified this stream is backed by a Docker repository on this server
	DockerImageRepository string `json:"dockerImageRepository,omitempty"`
	// Tags map arbitrary string values to specific image locators
	Tags []TagReference `json:"tags,omitempty"`
}

// TagReference specifies optional annotations for images using this tag and an optional reference to an ImageStreamTag, ImageStreamImage, or DockerImage this tag should track.
type TagReference struct {
	// Name of the tag
	Name string `json:"name"`
	// Annotations associated with images using this tag
	Annotations map[string]string `json:"annotations"`
	// From is a reference to an image stream tag or image stream this tag should track
	From *kapi.ObjectReference `json:"from,omitempty"`
	// Reference states if the tag will be imported. Default value is false, which means the tag will be imported.
	Reference bool `json:"reference,omitempty"`
	// Generation is the image stream generation that updated this tag - setting it to 0 is an indication that the generation must be updated.
	// Legacy clients will send this as nil, which means the client doesn't know or care.
	Generation *int64 `json:"generation"`
	// Import is information that controls how images may be imported by the server.
	ImportPolicy TagImportPolicy `json:"importPolicy,omitempty"`
}

// TagImportPolicy describes the tag import policy
type TagImportPolicy struct {
	// Insecure is true if the server may bypass certificate verification or connect directly over HTTP during image import.
	Insecure bool `json:"insecure,omitempty"`
	// Scheduled indicates to the server that this tag should be periodically checked to ensure it is up to date, and imported
	Scheduled bool `json:"scheduled,omitempty"`
}

// ImageStreamStatus contains information about the state of this image stream.
type ImageStreamStatus struct {
	// DockerImageRepository represents the effective location this stream may be accessed at.
	// May be empty until the server determines where the repository is located
	DockerImageRepository string `json:"dockerImageRepository"`
	// Tags are a historical record of images associated with each tag. The first entry in the
	// TagEvent array is the currently tagged image.
	Tags []NamedTagEventList `json:"tags,omitempty"`
}

// NamedTagEventList relates a tag to its image history.
type NamedTagEventList struct {
	// Tag is the tag for which the history is recorded
	Tag string `json:"tag"`
	// Standard object's metadata.
	Items []TagEvent `json:"items"`
	// Conditions is an array of conditions that apply to the tag event list.
	Conditions []TagEventCondition `json:"conditions,omitempty"`
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
	// LastTransitionTIme is the time the condition transitioned from one status to another.
	LastTransitionTime unversioned.Time `json:"lastTransitionTime,omitempty"`
	// Reason is a brief machine readable explanation for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Message is a human readable description of the details about last transition, complementing reason.
	Message string `json:"message,omitempty"`
	// Generation is the spec tag generation that this status corresponds to
	Generation int64 `json:"generation"`
}

// ImageStreamMapping represents a mapping from a single tag to a Docker image as
// well as the reference to the Docker image stream the image came from.
type ImageStreamMapping struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Image is a Docker image.
	Image Image `json:"image"`
	// Tag is a string value this image can be located with inside the stream.
	Tag string `json:"tag"`
}

// ImageStreamTag represents an Image that is retrieved by tag name from an ImageStream.
type ImageStreamTag struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Tag is the spec tag associated with this image stream tag, and it may be null
	// if only pushes have occured to this image stream.
	Tag *TagReference `json:"tag"`

	// Generation is the current generation of the tagged image - if tag is provided
	// and this value is not equal to the tag generation, a user has requested an
	// import that has not completed, or Conditions will be filled out indicating any
	// error.
	Generation int64 `json:"generation"`

	// Conditions is an array of conditions that apply to the image stream tag.
	Conditions []TagEventCondition `json:"conditions,omitempty"`

	// Image associated with the ImageStream and tag.
	Image Image `json:"image"`
}

// ImageStreamTagList is a list of ImageStreamTag objects.
type ImageStreamTagList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`

	// Items is the list of image stream tags
	Items []ImageStreamTag `json:"items"`
}

// ImageStreamImage represents an Image that is retrieved by image name from an ImageStream.
type ImageStreamImage struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Image associated with the ImageStream and image name.
	Image Image `json:"image"`
}

// DockerImageReference points to a Docker image.
type DockerImageReference struct {
	// Registry is the registry that contains the Docker image
	Registry string
	// Namespace is the namespace that contains the Docker image
	Namespace string
	// Name is the name of the Docker image
	Name string
	// Tag is which tag of the Docker image is being referenced
	Tag string
	// ID is the identifier for the Docker image
	ID string
}

// ImageStreamImport imports an image from remote repositories into OpenShift.
type ImageStreamImport struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Spec is a description of the images that the user wishes to import
	Spec ImageStreamImportSpec `json:"spec"`
	// Status is the the result of importing the image
	Status ImageStreamImportStatus `json:"status"`
}

// ImageStreamImportSpec defines what images should be imported.
type ImageStreamImportSpec struct {
	// Import indicates whether to perform an import - if so, the specified tags are set on the spec
	// and status of the image stream defined by the type meta.
	Import bool `json:"import"`
	// Repository is an optional import of an entire Docker image repository. A maximum limit on the
	// number of tags imported this way is imposed by the server.
	Repository *RepositoryImportSpec `json:"repository,omitempty"`
	// Images are a list of individual images to import.
	Images []ImageImportSpec `json:"images,omitempty"`
}

// ImageStreamImportStatus contains information about the status of an image stream import.
type ImageStreamImportStatus struct {
	// Import is the image stream that was successfully updated or created when 'to' was set.
	Import *ImageStream `json:"import,omitempty"`
	// Repository is set if spec.repository was set to the outcome of the import
	Repository *RepositoryImportStatus `json:"repository,omitempty"`
	// Images is set with the result of importing spec.images
	Images []ImageImportStatus `json:"images,omitempty"`
}

// RepositoryImportSpec describes a request to import images from a Docker image repository.
type RepositoryImportSpec struct {
	// From is the source for the image repository to import; only kind DockerImage and a name of a Docker image repository is allowed
	From kapi.ObjectReference `json:"from"`

	// ImportPolicy is the policy controlling how the image is imported
	ImportPolicy TagImportPolicy `json:"importPolicy,omitempty"`
	// IncludeManifest determines if the manifest for each image is returned in the response
	IncludeManifest bool `json:"includeManifest,omitempty"`
}

// RepositoryImportStatus describes the result of an image repository import
type RepositoryImportStatus struct {
	// Status reflects whether any failure occurred during import
	Status unversioned.Status `json:"status,omitempty"`
	// Images is a list of images successfully retrieved by the import of the repository.
	Images []ImageImportStatus `json:"images,omitempty"`
	// AdditionalTags are tags that exist in the repository but were not imported because
	// a maximum limit of automatic imports was applied.
	AdditionalTags []string `json:"additionalTags,omitempty"`
}

// ImageImportSpec describes a request to import a specific image.
type ImageImportSpec struct {
	// From is the source of an image to import; only kind DockerImage is allowed
	From kapi.ObjectReference `json:"from"`
	// To is a tag in the current image stream to assign the imported image to, if name is not specified the default tag from from.name will be used
	To *kapi.LocalObjectReference `json:"to,omitempty"`

	// ImportPolicy is the policy controlling how the image is imported
	ImportPolicy TagImportPolicy `json:"importPolicy,omitempty"`
	// IncludeManifest determines if the manifest for each image is returned in the response
	IncludeManifest bool `json:"includeManifest,omitempty"`
}

// ImageImportStatus describes the result of an image import.
type ImageImportStatus struct {
	// Status is the status of the image import, including errors encountered while retrieving the image
	Status unversioned.Status `json:"status"`
	// Image is the metadata of that image, if the image was located
	Image *Image `json:"image,omitempty"`
	// Tag is the tag this image was located under, if any
	Tag string `json:"tag,omitempty"`
}
