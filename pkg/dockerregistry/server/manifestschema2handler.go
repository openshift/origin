package server

import (
	"encoding/json"
	"errors"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/manifest/schema2"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

var (
	errMissingURL    = errors.New("missing URL on layer")
	errUnexpectedURL = errors.New("unexpected URL on layer")
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

func (h *manifestSchema2Handler) Verify(ctx context.Context, skipDependencyVerification bool) error {
	var errs distribution.ErrManifestVerification

	if skipDependencyVerification {
		return nil
	}

	// we want to verify that referenced blobs exist locally - thus using upstream repository object directly
	repo := h.repo.Repository

	target := h.manifest.Target()
	_, err := repo.Blobs(ctx).Stat(ctx, target.Digest)
	if err != nil {
		if err != distribution.ErrBlobUnknown {
			errs = append(errs, err)
		}

		// On error here, we always append unknown blob errors.
		errs = append(errs, distribution.ErrManifestBlobUnknown{Digest: target.Digest})
	}

	for _, fsLayer := range h.manifest.References() {
		var err error
		if fsLayer.MediaType != schema2.MediaTypeForeignLayer {
			if len(fsLayer.URLs) == 0 {
				_, err = repo.Blobs(ctx).Stat(ctx, fsLayer.Digest)
			} else {
				err = errUnexpectedURL
			}
		} else {
			// Clients download this layer from an external URL, so do not check for
			// its presense.
			if len(fsLayer.URLs) == 0 {
				err = errMissingURL
			}
		}
		if err != nil {
			if err != distribution.ErrBlobUnknown {
				errs = append(errs, err)
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
