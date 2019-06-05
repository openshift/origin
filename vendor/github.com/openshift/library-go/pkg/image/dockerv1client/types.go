package dockerv1client

// TODO: Move these to openshift/api

// DockerImageManifest represents the Docker v2 image format.
type DockerImageManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType,omitempty"`

	// schema1
	Name         string          `json:"name"`
	Tag          string          `json:"tag"`
	Architecture string          `json:"architecture"`
	FSLayers     []DockerFSLayer `json:"fsLayers"`
	History      []DockerHistory `json:"history"`

	// schema2
	Layers []Descriptor `json:"layers"`
	Config Descriptor   `json:"config"`
}

// DockerFSLayer is a container struct for BlobSums defined in an image manifest
type DockerFSLayer struct {
	// DockerBlobSum is the tarsum of the referenced filesystem image layer
	// TODO make this digest.Digest once docker/distribution is in Godeps
	DockerBlobSum string `json:"blobSum"`
}

// DockerHistory stores unstructured v1 compatibility information
type DockerHistory struct {
	// DockerV1Compatibility is the raw v1 compatibility information
	DockerV1Compatibility string `json:"v1Compatibility"`
}

// Descriptor describes targeted content. Used in conjunction with a blob
// store, a descriptor can be used to fetch, store and target any kind of
// blob. The struct also describes the wire protocol format. Fields should
// only be added but never changed.
type Descriptor struct {
	// MediaType describe the type of the content. All text based formats are
	// encoded as utf-8.
	MediaType string `json:"mediaType,omitempty"`

	// Size in bytes of content.
	Size int64 `json:"size,omitempty"`

	// Digest uniquely identifies the content. A byte stream can be verified
	// against against this digest.
	Digest string `json:"digest,omitempty"`
}
