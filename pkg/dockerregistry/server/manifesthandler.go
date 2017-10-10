package server

import (
	"errors"
	"fmt"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
)

// ErrNotImplemented is returned by an interface instance that does not implement the method in question.
var ErrNotImplemented = errors.New("not implemented")

// A ManifestHandler defines a common set of operations on all versions of manifest schema.
type ManifestHandler interface {
	// Config returns a blob with image configuration associated with the manifest. This applies only to
	// manifet schema 2.
	Config(ctx context.Context) ([]byte, error)

	// Digest returns manifest's digest.
	Digest() (manifestDigest digest.Digest, err error)

	// Layers returns a list of image layers.
	Layers(ctx context.Context) ([]imageapiv1.ImageLayer, error)

	// Metadata returns image configuration in internal representation.
	Metadata(ctx context.Context) (*imageapi.DockerImage, error)

	// Manifest returns a deserialized manifest object.
	Manifest() distribution.Manifest

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
	case "", schema1.MediaTypeManifest, schema1.MediaTypeSignedManifest:
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
