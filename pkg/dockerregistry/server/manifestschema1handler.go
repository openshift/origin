package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/reference"
	"github.com/docker/libtrust"

	"k8s.io/apimachinery/pkg/util/sets"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
)

// ErrNoManifestMetadata is an error informing about invalid manifest that lacks metadata.
var ErrNoManifestMetadata = errors.New("no manifest metadata found")

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
	return &sm, nil
}

type manifestSchema1Handler struct {
	repo         *repository
	manifest     *schema1.SignedManifest
	cachedLayers []imageapiv1.ImageLayer
}

var _ ManifestHandler = &manifestSchema1Handler{}

func (h *manifestSchema1Handler) Config(ctx context.Context) ([]byte, error) {
	return nil, nil
}

func (h *manifestSchema1Handler) Digest() (digest.Digest, error) {
	return digest.FromBytes(h.manifest.Canonical), nil
}

func (h *manifestSchema1Handler) Layers(ctx context.Context) ([]imageapiv1.ImageLayer, error) {
	if h.cachedLayers == nil {
		var sizeContainer = imageapi.DockerV1CompatibilityImageSize{}

		layers := make([]imageapiv1.ImageLayer, len(h.manifest.FSLayers))
		for hi, li := 0, len(h.manifest.FSLayers)-1; hi < len(h.manifest.FSLayers) && li >= 0; hi, li = hi+1, li-1 {
			layer := &layers[li]
			sizeContainer.Size = 0
			if hi < len(h.manifest.History) {
				if err := json.Unmarshal([]byte(h.manifest.History[hi].V1Compatibility), &sizeContainer); err != nil {
					sizeContainer.Size = 0
				}
			}
			if err := h.updateLayerMetadata(ctx, layer, &h.manifest.FSLayers[hi], sizeContainer.Size); err != nil {
				return nil, err
			}
		}

		h.cachedLayers = layers
	}

	layers := make([]imageapiv1.ImageLayer, len(h.cachedLayers))
	for i, l := range h.cachedLayers {
		layers[i] = l
	}

	return layers, nil
}

func (h *manifestSchema1Handler) Manifest() distribution.Manifest {
	return h.manifest
}

func (h *manifestSchema1Handler) Metadata(ctx context.Context) (*imageapi.DockerImage, error) {
	if len(h.manifest.History) == 0 {
		// should never have an empty history, but just in case...
		return nil, ErrNoManifestMetadata
	}

	v1Metadata := imageapi.DockerV1CompatibilityImage{}
	if err := json.Unmarshal([]byte(h.manifest.History[0].V1Compatibility), &v1Metadata); err != nil {
		return nil, err
	}

	var (
		dockerImageSize int64
		layerSet        = sets.NewString()
	)

	layers, err := h.Layers(ctx)
	if err != nil {
		return nil, err
	}
	for _, layer := range layers {
		if !layerSet.Has(layer.Name) {
			dockerImageSize += layer.LayerSize
			layerSet.Insert(layer.Name)
		}
	}

	meta := &imageapi.DockerImage{}
	meta.ID = v1Metadata.ID
	meta.Parent = v1Metadata.Parent
	meta.Comment = v1Metadata.Comment
	meta.Created = v1Metadata.Created
	meta.Container = v1Metadata.Container
	meta.ContainerConfig = v1Metadata.ContainerConfig
	meta.DockerVersion = v1Metadata.DockerVersion
	meta.Author = v1Metadata.Author
	meta.Config = v1Metadata.Config
	meta.Architecture = v1Metadata.Architecture
	meta.Size = dockerImageSize

	return meta, nil
}

func (h *manifestSchema1Handler) Signatures(ctx context.Context) ([][]byte, error) {
	return h.manifest.Signatures()
}

func (h *manifestSchema1Handler) Payload() (mediaType string, payload []byte, canonical []byte, err error) {
	mt, payload, err := h.manifest.Payload()
	return mt, payload, h.manifest.Canonical, err
}

func (h *manifestSchema1Handler) Verify(ctx context.Context, skipDependencyVerification bool) error {
	var errs distribution.ErrManifestVerification

	// we want to verify that referenced blobs exist locally or accessible over
	// pullthroughBlobStore. The base image of this image can be remote repository
	// and since we use pullthroughBlobStore all the layer existence checks will be
	// successful. This means that the docker client will not attempt to send them
	// to us as it will assume that the registry has them.
	repo := h.repo

	if len(path.Join(h.repo.config.registryAddr, h.manifest.Name)) > reference.NameTotalLengthMax {
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

func (h *manifestSchema1Handler) updateLayerMetadata(
	ctx context.Context,
	layerMetadata *imageapiv1.ImageLayer,
	manifestLayer *schema1.FSLayer,
	size int64,
) error {
	layerMetadata.Name = manifestLayer.BlobSum.String()
	layerMetadata.MediaType = schema1.MediaTypeManifestLayer
	if size > 0 {
		layerMetadata.LayerSize = size
		return nil
	}

	desc, err := h.repo.Blobs(ctx).Stat(ctx, digest.Digest(layerMetadata.Name))
	if err != nil {
		context.GetLogger(ctx).Errorf("failed to stat blob %s", layerMetadata.Name)
		return err
	}
	layerMetadata.LayerSize = desc.Size
	return nil
}
