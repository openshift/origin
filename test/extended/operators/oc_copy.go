package operators

import (
	"github.com/docker/distribution"
	digest "github.com/opencontainers/go-digest"
	imageapi "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/dockerv1client"
	imagereference "github.com/openshift/library-go/pkg/image/reference"
)

type CincinnatiMetadata struct {
	Kind string `json:"kind"`

	Version  string   `json:"version"`
	Previous []string `json:"previous"`

	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ReleaseDiff struct {
	From *ReleaseInfo `json:"from"`
	To   *ReleaseInfo `json:"to"`

	ChangedImages    map[string]*ImageReferenceDiff  `json:"changedImages"`
	ChangedManifests map[string]*ReleaseManifestDiff `json:"changedManifests"`
}

type ImageReferenceDiff struct {
	Name string `json:"name"`

	From *imageapi.TagReference `json:"from"`
	To   *imageapi.TagReference `json:"to"`
}

type ReleaseManifestDiff struct {
	Filename string `json:"filename"`

	From []byte `json:"from"`
	To   []byte `json:"to"`
}

type ReleaseInfo struct {
	Image         string                              `json:"image"`
	ImageRef      imagereference.DockerImageReference `json:"-"`
	Digest        digest.Digest                       `json:"digest"`
	ContentDigest digest.Digest                       `json:"contentDigest"`
	// TODO: return the list digest in the future
	// ListDigest    digest.Digest                       `json:"listDigest"`
	Config     *dockerv1client.DockerImageConfig `json:"config"`
	Metadata   *CincinnatiMetadata               `json:"metadata"`
	References *imageapi.ImageStream             `json:"references"`

	ComponentVersions map[string]string `json:"versions"`

	Images map[string]*Image `json:"images"`

	RawMetadata   map[string][]byte `json:"-"`
	ManifestFiles map[string][]byte `json:"-"`
	UnknownFiles  []string          `json:"-"`

	Warnings []string `json:"warnings"`
}

type Image struct {
	Name          string                              `json:"name"`
	Ref           imagereference.DockerImageReference `json:"-"`
	Digest        digest.Digest                       `json:"digest"`
	ContentDigest digest.Digest                       `json:"contentDigest"`
	ListDigest    digest.Digest                       `json:"listDigest"`
	MediaType     string                              `json:"mediaType"`
	Layers        []distribution.Descriptor           `json:"layers"`
	Config        *dockerv1client.DockerImageConfig   `json:"config"`

	Manifest distribution.Manifest `json:"-"`
}
