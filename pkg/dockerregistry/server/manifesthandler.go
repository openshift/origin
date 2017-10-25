package server

import (
	"fmt"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"

	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
)

// A ManifestHandler defines a common set of operations on all versions of manifest schema.
type ManifestHandler interface {
	// Config returns a blob with image configuration associated with the manifest. This applies only to
	// manifet schema 2.
	Config(ctx context.Context) ([]byte, error)

	// Digest returns manifest's digest.
	Digest() (manifestDigest digest.Digest, err error)

	// Manifest returns a deserialized manifest object.
	Manifest() distribution.Manifest

	// Layers returns image layers and a value for the dockerLayersOrder annotation.
	Layers(ctx context.Context) (order string, layers []imageapiv1.ImageLayer, err error)

	// Payload returns manifest's media type, complete payload with signatures and canonical payload without
	// signatures or an error if the information could not be fetched.
	Payload() (mediaType string, payload []byte, canonical []byte, err error)

	// Verify returns an error if the contained manifest is not valid or has missing dependencies.
	Verify(ctx context.Context, skipDependencyVerification bool) error
}

// NewManifestHandler creates a manifest handler for the given manifest.
func NewManifestHandler(repo *repository, manifest distribution.Manifest) (ManifestHandler, error) {
	switch t := manifest.(type) {
	case *schema1.SignedManifest:
		return &manifestSchema1Handler{repo: repo, manifest: t}, nil
	case *schema2.DeserializedManifest:
		return &manifestSchema2Handler{repo: repo, manifest: t}, nil
	default:
		return nil, fmt.Errorf("unsupported manifest type %T", manifest)
	}
}

// NewManifestHandlerFromImage creates a new manifest handler for a manifest stored in the given image.
func NewManifestHandlerFromImage(repo *repository, image *imageapiv1.Image) (ManifestHandler, error) {
	var (
		manifest distribution.Manifest
		err      error
	)

	switch image.DockerImageManifestMediaType {
	case "", schema1.MediaTypeManifest:
		manifest, err = unmarshalManifestSchema1([]byte(image.DockerImageManifest), image.DockerImageSignatures)
	case schema2.MediaTypeManifest:
		manifest, err = unmarshalManifestSchema2([]byte(image.DockerImageManifest))
	default:
		return nil, fmt.Errorf("unsupported manifest media type %s", image.DockerImageManifestMediaType)
	}

	if err != nil {
		return nil, err
	}

	return NewManifestHandler(repo, manifest)
}
