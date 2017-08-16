package server

import (
	"encoding/json"
	"errors"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema2"

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

	return &deserializedManifest, nil
}

type manifestSchema2Handler struct {
	repo     *repository
	manifest *schema2.DeserializedManifest
}

var _ ManifestHandler = &manifestSchema2Handler{}

func (h *manifestSchema2Handler) FillImageMetadata(ctx context.Context, image *imageapiv1.Image) error {
	// The manifest.Config references a configuration object for a container by its digest.
	// It needs to be fetched in order to fill an image object metadata below.
	configBytes, err := h.repo.Blobs(ctx).Get(ctx, h.manifest.Config.Digest)
	if err != nil {
		context.GetLogger(ctx).Errorf("failed to get image config %s: %v", h.manifest.Config.Digest.String(), err)
		return err
	}
	image.DockerImageConfig = string(configBytes)

	// We need to populate the image metadata using the manifest.
	return imageMetadataFromManifest(image)
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

	// we want to verify that referenced blobs exist locally or accessible over
	// pullthroughBlobStore. The base image of this image can be remote repository
	// and since we use pullthroughBlobStore all the layer existence checks will be
	// successful. This means that the docker client will not attempt to send them
	// to us as it will assume that the registry has them.
	repo := h.repo

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

func (h *manifestSchema2Handler) Digest() (digest.Digest, error) {
	_, p, err := h.manifest.Payload()
	if err != nil {
		return "", err
	}
	return digest.FromBytes(p), nil
}
