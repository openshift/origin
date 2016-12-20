package server

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/reference"
	"github.com/docker/libtrust"

	"k8s.io/kubernetes/pkg/util/sets"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

func unmarshalManifestSchema1(content []byte, signatures [][]byte) (distribution.Manifest, error) {
	// prefer signatures from the manifest
	if _, err := libtrust.ParsePrettySignature(content, "signatures"); err == nil {
		sm := schema1.SignedManifest{Canonical: content}
		if err = json.Unmarshal(content, &sm); err == nil {
			return &sm, nil
		}
	}

	jsig, err := libtrust.NewJSONSignature(content, signatures...)
	if err != nil {
		return nil, err
	}

	// Extract the pretty JWS
	content, err = jsig.PrettySignature("signatures")
	if err != nil {
		return nil, err
	}

	var sm schema1.SignedManifest
	if err = json.Unmarshal(content, &sm); err != nil {
		return nil, err
	}
	return &sm, err
}

type manifestSchema1Handler struct {
	repo     *repository
	manifest *schema1.SignedManifest
}

var _ ManifestHandler = &manifestSchema1Handler{}

func (h *manifestSchema1Handler) FillImageMetadata(ctx context.Context, image *imageapi.Image) error {
	signatures, err := h.manifest.Signatures()
	if err != nil {
		return err
	}

	for _, signDigest := range signatures {
		image.DockerImageSignatures = append(image.DockerImageSignatures, signDigest)
	}

	if err := imageapi.ImageWithMetadata(image); err != nil {
		return err
	}

	refs := h.manifest.References()

	blobSet := sets.NewString()
	image.DockerImageMetadata.Size = int64(0)

	blobs := h.repo.Blobs(ctx)
	for i := range image.DockerImageLayers {
		layer := &image.DockerImageLayers[i]
		// DockerImageLayers represents h.manifest.Manifest.FSLayers in reversed order
		desc, err := blobs.Stat(ctx, refs[len(image.DockerImageLayers)-i-1].Digest)
		if err != nil {
			context.GetLogger(ctx).Errorf("failed to stat blob %s of image %s", layer.Name, image.DockerImageReference)
			return err
		}
		if layer.MediaType == "" {
			if desc.MediaType != "" {
				layer.MediaType = desc.MediaType
			} else {
				layer.MediaType = schema1.MediaTypeManifestLayer
			}
		}
		layer.LayerSize = desc.Size
		// count empty layer just once (empty layer may actually have non-zero size)
		if !blobSet.Has(layer.Name) {
			image.DockerImageMetadata.Size += desc.Size
			blobSet.Insert(layer.Name)
		}
	}

	return nil
}

func (h *manifestSchema1Handler) Manifest() distribution.Manifest {
	return h.manifest
}

func (h *manifestSchema1Handler) Payload() (mediaType string, payload []byte, canonical []byte, err error) {
	mt, payload, err := h.manifest.Payload()
	return mt, payload, h.manifest.Canonical, err
}

func (h *manifestSchema1Handler) Verify(ctx context.Context, skipDependencyVerification bool) error {
	var errs distribution.ErrManifestVerification

	// we want to verify that referenced blobs exist locally - thus using upstream repository object directly
	repo := h.repo.Repository

	if len(path.Join(h.repo.registryAddr, h.manifest.Name)) > reference.NameTotalLengthMax {
		errs = append(errs,
			distribution.ErrManifestNameInvalid{
				Name:   h.manifest.Name,
				Reason: fmt.Errorf("<registry-host>/<manifest-name> must not be more than %d characters", reference.NameTotalLengthMax),
			})
	}

	if !reference.NameRegexp.MatchString(h.manifest.Name) {
		errs = append(errs,
			distribution.ErrManifestNameInvalid{
				Name:   h.manifest.Name,
				Reason: fmt.Errorf("invalid manifest name format"),
			})
	}

	if len(h.manifest.History) != len(h.manifest.FSLayers) {
		errs = append(errs, fmt.Errorf("mismatched history and fslayer cardinality %d != %d",
			len(h.manifest.History), len(h.manifest.FSLayers)))
	}

	if _, err := schema1.Verify(h.manifest); err != nil {
		switch err {
		case libtrust.ErrMissingSignatureKey, libtrust.ErrInvalidJSONContent, libtrust.ErrMissingSignatureKey:
			errs = append(errs, distribution.ErrManifestUnverified{})
		default:
			if err.Error() == "invalid signature" {
				errs = append(errs, distribution.ErrManifestUnverified{})
			} else {
				errs = append(errs, err)
			}
		}
	}

	if skipDependencyVerification {
		if len(errs) > 0 {
			return errs
		}
		return nil
	}

	for _, fsLayer := range h.manifest.References() {
		_, err := repo.Blobs(ctx).Stat(ctx, fsLayer.Digest)
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
