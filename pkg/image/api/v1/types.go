package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
)

// ImageList is a list of Image objects.
type ImageList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`

	// Items is a list of images
	Items []Image `json:"items" description:"list of image objects"`
}

// Image is an immutable representation of a Docker image and metadata at a point in time.
type Image struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// DockerImageReference is the string that can be used to pull this image.
	DockerImageReference string `json:"dockerImageReference,omitempty" description:"string that can be used to pull this image"`
	// DockerImageMetadata contains metadata about this image
	DockerImageMetadata runtime.RawExtension `json:"dockerImageMetadata,omitempty" description:"metadata about this image"`
	// DockerImageMetadataVersion conveys the version of the object, which if empty defaults to "1.0"
	DockerImageMetadataVersion string `json:"dockerImageMetadataVersion,omitempty" description:"conveys version of the object, if empty defaults to '1.0'"`
	// DockerImageManifest is the raw JSON of the manifest
	DockerImageManifest string `json:"dockerImageManifest,omitempty" description:"raw JSON of the manifest"`
	// DockerImageLayers represents the layers in the image. May not be set if the image does not define that data.
	DockerImageLayers []ImageLayer `json:"dockerImageLayers" description:"a list of the image layers from lowest to highest"`
}

// ImageLayer represents a single layer of the image. Some images may have multiple layers. Some may have none.
type ImageLayer struct {
	// Name of the layer as defined by the underlying store.
	Name string `json:"name" description:"the name of the layer (blob, in Docker parlance)"`
	// Size of the layer as defined by the underlying store.
	Size int64 `json:"size" description:"size of the layer in bytes"`
}

// ImageStreamList is a list of ImageStream objects.
type ImageStreamList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`

	// Items is a list of imageStreams
	Items []ImageStream `json:"items" description:"list of image stream objects"`
}

// ImageStream stores a mapping of tags to images, metadata overrides that are applied
// when images are tagged in a stream, and an optional reference to a Docker image
// repository on a registry.
type ImageStream struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

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
	Tags []TagReference `json:"tags,omitempty" description:"map arbitrary string values to specific image locators"`
}

// TagReference specifies optional annotations for images using this tag and an optional reference to an ImageStreamTag, ImageStreamImage, or DockerImage this tag should track.
type TagReference struct {
	// Name of the tag
	Name string `json:"name" description:"name of tag"`
	// Annotations associated with images using this tag
	Annotations map[string]string `json:"annotations,omitempty" description:"annotations associated with images using this tag"`
	// From is a reference to an image stream tag or image stream this tag should track
	From *kapi.ObjectReference `json:"from,omitempty" description:"a reference to an image stream tag or image stream this tag should track"`
	// Reference states if the tag will be imported. Default value is false, which means the tag will be imported.
	Reference bool `json:"reference,omitempty" description:"if true consider this tag a reference only and do not attempt to import metadata about the image"`
	// Generation is the image stream generation that updated this tag - setting it to 0 is an indication that the generation must be updated.
	// Legacy clients will send this as nil, which means the client doesn't know or care.
	Generation *int64 `json:"generation" description:"the generation of the image stream this was updated to"`
	// Import is information that controls how images may be imported by the server.
	ImportPolicy TagImportPolicy `json:"importPolicy,omitempty" description:"attributes controlling how this reference is imported"`
}

type TagImportPolicy struct {
	// Insecure is true if the server may bypass certificate verification or connect directly over HTTP during image import.
	Insecure bool `json:"insecure,omitempty" description:"if true, the server may bypass certificate verification or connect directly over HTTP during image import"`
	// Scheduled indicates to the server that this tag should be periodically checked to ensure it is up to date, and imported
	Scheduled bool `json:"scheduled,omitempty" description:"if true, the server will periodically check to ensure this tag is up to date"`
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
	// Conditions is an array of conditions that apply to the tag event list.
	Conditions []TagEventCondition `json:"conditions,omitempty" description:"the set of conditions that apply to this tag"`
}

// TagEvent is used by ImageStreamStatus to keep a historical record of images associated with a tag.
type TagEvent struct {
	// Created holds the time the TagEvent was created
	Created unversioned.Time `json:"created" description:"when the event was created"`
	// DockerImageReference is the string that can be used to pull this image
	DockerImageReference string `json:"dockerImageReference" description:"the string that can be used to pull this image"`
	// Image is the image
	Image string `json:"image" description:"the image"`
	// Generation is the spec tag generation that resulted in this tag being updated
	Generation int64 `json:"generation" description:"the generation of the image stream spec tag this tag event represents"`
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
	Type TagEventConditionType `json:"type" description:"type of tag event condition, currently only ImportSuccess"`
	// Status of the condition, one of True, False, Unknown.
	Status kapi.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`
	// LastTransitionTIme is the time the condition transitioned from one status to another.
	LastTransitionTime unversioned.Time `json:"lastTransitionTime,omitempty" description:"last time the condition transitioned from one status to another"`
	// Reason is a brief machine readable explanation for the condition's last transition.
	Reason string `json:"reason,omitempty" description:"machine-readable reason for the last condition transition"`
	// Message is a human readable description of the details about last transition, complementing reason.
	Message string `json:"message,omitempty" description:"human-readable message indicating details of the last transition"`
	// Generation is the spec tag generation that this status corresponds to
	Generation int64 `json:"generation" description:"the generation of the image stream spec tag this condition represents"`
}

// ImageStreamMapping represents a mapping from a single tag to a Docker image as
// well as the reference to the Docker image stream the image came from.
type ImageStreamMapping struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// Image is a Docker image.
	Image Image `json:"image" description:"a Docker image"`
	// Tag is a string value this image can be located with inside the stream.
	Tag string `json:"tag" description:"string value this image can be located with inside the stream"`
}

// ImageStreamTag represents an Image that is retrieved by tag name from an ImageStream.
type ImageStreamTag struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// Image associated with the ImageStream and tag.
	Image Image `json:"image" description:"the image associated with the ImageStream and tag"`
}

// ImageStreamTagList is a list of ImageStreamTag objects.
type ImageStreamTagList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`

	Items []ImageStreamTag `json:"items" description:"list of image stream tag objects"`
}

// ImageStreamImage represents an Image that is retrieved by image name from an ImageStream.
type ImageStreamImage struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

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

// ImageStreamImport imports an image from remote repositories into OpenShift.
type ImageStreamImport struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty" description:"metadata about the image stream, name is required"`

	Spec   ImageStreamImportSpec   `json:"spec" description:"description of the images that the user wishes to import"`
	Status ImageStreamImportStatus `json:"status" description:"the result of importing the image"`
}

// ImageStreamImportSpec defines what images should be imported.
type ImageStreamImportSpec struct {
	// Import indicates whether to perform an import - if so, the specified tags are set on the spec
	// and status of the image stream defined by the type meta.
	Import bool `json:"import" description:"if true, the images will be imported to the server and the resulting image stream will be returned in status.import"`
	// Repository is an optional import of an entire Docker image repository. A maximum limit on the
	// number of tags imported this way is imposed by the server.
	Repository *RepositoryImportSpec `json:"repository,omitempty" description:"if specified, import a single Docker repository's tags to this image stream"`
	// Images are a list of individual images to import.
	Images []ImageImportSpec `json:"images,omitempty" description:"a list of images to import into this image stream"`
}

// ImageStreamImportStatus contains information about the status of an image stream import.
type ImageStreamImportStatus struct {
	// Import is the image stream that was successfully updated or created when 'to' was set.
	Import *ImageStream `json:"import,omitempty" description:"if the user requested any images be imported, this field will be set with the successful image stream create or update"`
	// Repository is set if spec.repository was set to the outcome of the import
	Repository *RepositoryImportStatus `json:"repository,omitempty" description:"status of the attempt to import a repository"`
	// Images is set with the result of importing spec.images
	Images []ImageImportStatus `json:"images,omitempty" description:"status of the attempt to import images"`
}

// RepositoryImportSpec describes a request to import images from a Docker image repository.
type RepositoryImportSpec struct {
	From kapi.ObjectReference `json:"from" description:"the source for the image repository to import; only kind DockerImage and a name of a Docker image repository is allowed"`

	ImportPolicy    TagImportPolicy `json:"importPolicy,omitempty" description:"policy controlling how the image is imported"`
	IncludeManifest bool            `json:"includeManifest,omitempty" description:"if true, return the manifest for each image in the response"`
}

// RepositoryImportStatus describes the result of an image repository import
type RepositoryImportStatus struct {
	// Status reflects whether any failure occurred during import
	Status unversioned.Status `json:"status,omitempty" description:"the result of the import attempt, will include a reason and message if the repository could not be imported"`
	// Images is a list of images successfully retrieved by the import of the repository.
	Images []ImageImportStatus `json:"images,omitempty" description:"a list of the images retrieved by the import of the repository"`
	// AdditionalTags are tags that exist in the repository but were not imported because
	// a maximum limit of automatic imports was applied.
	AdditionalTags []string `json:"additionalTags,omitempty" description:"a list of additional tags on the repository that were not retrieved"`
}

// ImageImportSpec describes a request to import a specific image.
type ImageImportSpec struct {
	From kapi.ObjectReference       `json:"from" description:"the source of an image to import; only kind DockerImage is allowed"`
	To   *kapi.LocalObjectReference `json:"to,omitempty" description:"a tag in the current image stream to assign the imported image to, if name is not specified the default tag from from.name will be used"`

	ImportPolicy    TagImportPolicy `json:"importPolicy,omitempty" description:"policy controlling how the image is imported"`
	IncludeManifest bool            `json:"includeManifest,omitempty" description:"if true, return the manifest for this image in the response"`
}

// ImageImportStatus describes the result of an image import.
type ImageImportStatus struct {
	Status unversioned.Status `json:"status" description:"the status of the image import, including errors encountered while retrieving the image"`
	Image  *Image             `json:"image,omitempty" description:"if the image was located, the metadata of that image"`
	Tag    string             `json:"tag,omitempty" description:"the tag this image was located under, if any"`
}
