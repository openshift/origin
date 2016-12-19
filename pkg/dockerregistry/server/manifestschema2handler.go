package server

import (
	"encoding/json"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/manifest/schema2"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

func unmarshalManifestSchema2(content []byte) (distribution.Manifest, error) {
	var m schema2.DeserializedManifest
	if err := json.Unmarshal(content, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

type manifestSchema2Handler struct {
	repo     *repository
	manifest *schema2.DeserializedManifest
}

var _ ManifestHandler = &manifestSchema2Handler{}

func (h *manifestSchema2Handler) FillImageMetadata(ctx context.Context, image *imageapi.Image) error {
	configBytes, err := h.repo.Blobs(ctx).Get(ctx, h.manifest.Config.Digest)
	if err != nil {
		context.GetLogger(ctx).Errorf("failed to get image config %s: %v", h.manifest.Config.Digest.String(), err)
		return err
	}
	image.DockerImageConfig = string(configBytes)

	if err := imageapi.ImageWithMetadata(image); err != nil {
		return err
	}

	return nil
}

func (h *manifestSchema2Handler) Manifest() distribution.Manifest {
	return h.manifest
}

func (h *manifestSchema2Handler) Payload() (mediaType string, payload []byte, canonical []byte, err error) {
	mt, p, err := h.manifest.Payload()
	return mt, p, p, err
}
