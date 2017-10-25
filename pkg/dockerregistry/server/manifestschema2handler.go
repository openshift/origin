package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema2"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
)

var (
	errMissingURL    = errors.New("missing URL on layer")
	errUnexpectedURL = errors.New("unexpected URL on layer")
)

func unmarshalManifestSchema2(content []byte) (distribution.Manifest, error) {
	var deserializedManifest schema2.DeserializedManifest
	if err := json.Unmarshal(content, &deserializedManifest); err != nil {
		return nil, err
	}

	if !reflect.DeepEqual(deserializedManifest.Versioned, schema2.SchemaVersion) {
		return nil, fmt.Errorf("unexpected manifest schema version=%d, mediaType=%q",
			deserializedManifest.SchemaVersion,
			deserializedManifest.MediaType)
	}

	return &deserializedManifest, nil
}

type manifestSchema2Handler struct {
	repo         *repository
	manifest     *schema2.DeserializedManifest
	cachedConfig []byte
}

var _ ManifestHandler = &manifestSchema2Handler{}

func (h *manifestSchema2Handler) Config(ctx context.Context) ([]byte, error) {
	if h.cachedConfig == nil {
		blob, err := h.repo.Blobs(ctx).Get(ctx, h.manifest.Config.Digest)
		if err != nil {
			context.GetLogger(ctx).Errorf("failed to get manifest config: %v", err)
			return nil, err
		}
		h.cachedConfig = blob
	}

	return h.cachedConfig, nil
}

func (h *manifestSchema2Handler) Digest() (digest.Digest, error) {
	_, p, err := h.manifest.Payload()
	if err != nil {
		return "", err
	}
	return digest.FromBytes(p), nil
}

func (h *manifestSchema2Handler) Manifest() distribution.Manifest {
	return h.manifest
}

func (h *manifestSchema2Handler) Layers(ctx context.Context) (string, []imageapiv1.ImageLayer, error) {
	layers := make([]imageapiv1.ImageLayer, len(h.manifest.Layers))
	for i, layer := range h.manifest.Layers {
		layers[i].Name = layer.Digest.String()
		layers[i].LayerSize = layer.Size
		layers[i].MediaType = layer.MediaType
	}
	return imageapi.DockerImageLayersOrderAscending, layers, nil
}

func (h *manifestSchema2Handler) Payload() (mediaType string, payload []byte, canonical []byte, err error) {
	mt, p, err := h.manifest.Payload()
	return mt, p, p, err
}

func (h *manifestSchema2Handler) verifyLayer(ctx context.Context, fsLayer distribution.Descriptor) error {
	if fsLayer.MediaType == schema2.MediaTypeForeignLayer {
		// Clients download this layer from an external URL, so do not check for
		// its presense.
		if len(fsLayer.URLs) == 0 {
			return errMissingURL
		}
		return nil
	}

	if len(fsLayer.URLs) != 0 {
		return errUnexpectedURL
	}

	desc, err := h.repo.Blobs(ctx).Stat(ctx, fsLayer.Digest)
	if err != nil {
		return err
	}

	if fsLayer.Size != desc.Size {
		return ErrManifestBlobBadSize{
			Digest:         fsLayer.Digest,
			ActualSize:     desc.Size,
			SizeInManifest: fsLayer.Size,
		}
	}

	return nil
}

func (h *manifestSchema2Handler) Verify(ctx context.Context, skipDependencyVerification bool) error {
	var errs distribution.ErrManifestVerification

	if skipDependencyVerification {
		return nil
	}

	// we want to verify that referenced blobs exist locally or accessible over
	// pullthroughBlobStore. The base image of this image can be remote repository
	// and since we use pullthroughBlobStore all the layer existence checks will be
	// successful. This means that the docker client will not attempt to send them
	// to us as it will assume that the registry has them.

	for _, fsLayer := range h.manifest.References() {
		if err := h.verifyLayer(ctx, fsLayer); err != nil {
			if err != distribution.ErrBlobUnknown {
				errs = append(errs, err)
				continue
			}

			// On error here, we always append unknown blob errors.
			errs = append(errs, distribution.ErrManifestBlobUnknown{Digest: fsLayer.Digest})
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}
