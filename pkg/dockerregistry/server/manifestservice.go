package server

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/api/errcode"
	regapi "github.com/docker/distribution/registry/api/v2"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"

	imageapi "github.com/openshift/origin/pkg/image/api"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

var _ distribution.ManifestService = &manifestService{}

type manifestService struct {
	ctx       context.Context
	repo      *repository
	manifests distribution.ManifestService

	// acceptschema2 allows to refuse the manifest schema version 2
	acceptschema2 bool
}

// Exists returns true if the manifest specified by dgst exists.
func (m *manifestService) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	context.GetLogger(ctx).Debugf("(*manifestService).Exists")

	image, _, err := m.repo.getImageOfImageStream(dgst)
	if err != nil {
		return false, err
	}
	return image != nil, nil
}

// Get retrieves the manifest with digest `dgst`.
func (m *manifestService) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	context.GetLogger(ctx).Debugf("(*manifestService).Get")

	image, _, err := m.repo.getImageOfImageStream(dgst)
	if err != nil {
		return nil, err
	}

	ref := imageapi.DockerImageReference{
		Namespace: m.repo.namespace,
		Name:      m.repo.name,
		Registry:  m.repo.registryAddr,
	}
	if isImageManaged(image) {
		// Reference without a registry part refers to repository containing locally managed images.
		// Such an entry is retrieved, checked and set by blobDescriptorService operating only on local blobs.
		ref.Registry = ""
	} else {
		// Repository with a registry points to remote repository. This is used by pullthrough middleware.
		ref = ref.DockerClientDefaults().AsRepository()
	}

	manifest, err := m.manifests.Get(withRepository(ctx, m.repo), dgst, options...)
	switch err.(type) {
	case distribution.ErrManifestUnknownRevision:
		break
	case nil:
		m.repo.rememberLayersOfManifest(dgst, manifest, ref.Exact())
		m.migrateManifest(withRepository(ctx, m.repo), image, dgst, manifest, true)
		return manifest, nil
	default:
		context.GetLogger(m.ctx).Errorf("unable to get manifest from storage: %v", err)
		return nil, err
	}

	if len(image.DockerImageManifest) == 0 {
		// We don't have the manifest in the storage and we don't have the manifest
		// inside the image so there is no point to continue.
		return nil, distribution.ErrManifestUnknownRevision{
			Name:     m.repo.Named().Name(),
			Revision: dgst,
		}
	}

	manifest, err = m.repo.manifestFromImageWithCachedLayers(image, ref.Exact())
	if err == nil {
		m.migrateManifest(withRepository(ctx, m.repo), image, dgst, manifest, false)
	}

	return manifest, err
}

// Put creates or updates the named manifest.
func (m *manifestService) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	context.GetLogger(ctx).Debugf("(*manifestService).Put")

	mh, err := NewManifestHandler(m.repo, manifest)
	if err != nil {
		return "", regapi.ErrorCodeManifestInvalid.WithDetail(err)
	}
	mediaType, payload, canonical, err := mh.Payload()
	if err != nil {
		return "", regapi.ErrorCodeManifestInvalid.WithDetail(err)
	}

	// this is fast to check, let's do it before verification
	if !m.acceptschema2 && mediaType == schema2.MediaTypeManifest {
		return "", regapi.ErrorCodeManifestInvalid.WithDetail(fmt.Errorf("manifest V2 schema 2 not allowed"))
	}

	// in order to stat the referenced blobs, repository need to be set on the context
	if err := mh.Verify(withRepository(ctx, m.repo), false); err != nil {
		return "", err
	}

	_, err = m.manifests.Put(withRepository(ctx, m.repo), manifest, options...)
	if err != nil {
		return "", err
	}

	// Calculate digest
	dgst := digest.FromBytes(canonical)

	// Upload to openshift
	ism := imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: m.repo.namespace,
			Name:      m.repo.name,
		},
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: dgst.String(),
				Annotations: map[string]string{
					imageapi.ManagedByOpenShiftAnnotation: "true",
				},
			},
			DockerImageReference:         fmt.Sprintf("%s/%s/%s@%s", m.repo.registryAddr, m.repo.namespace, m.repo.name, dgst.String()),
			DockerImageManifest:          string(payload),
			DockerImageManifestMediaType: mediaType,
		},
	}

	for _, option := range options {
		if opt, ok := option.(distribution.WithTagOption); ok {
			ism.Tag = opt.Tag
			break
		}
	}

	if err = mh.FillImageMetadata(ctx, &ism.Image); err != nil {
		return "", err
	}

	// Remove the raw manifest as it's very big and this leads to a large memory consumption in etcd.
	ism.Image.DockerImageManifest = ""
	ism.Image.DockerImageConfig = ""

	if err = m.repo.registryOSClient.ImageStreamMappings(m.repo.namespace).Create(&ism); err != nil {
		// if the error was that the image stream wasn't found, try to auto provision it
		statusErr, ok := err.(*kerrors.StatusError)
		if !ok {
			context.GetLogger(ctx).Errorf("error creating ImageStreamMapping: %s", err)
			return "", err
		}

		if quotautil.IsErrorQuotaExceeded(statusErr) {
			context.GetLogger(ctx).Errorf("denied creating ImageStreamMapping: %v", statusErr)
			return "", distribution.ErrAccessDenied
		}

		status := statusErr.ErrStatus
		if status.Code != http.StatusNotFound ||
			(strings.ToLower(status.Details.Kind) != "imagestream" /*pre-1.2*/ && strings.ToLower(status.Details.Kind) != "imagestreams") ||
			status.Details.Name != m.repo.name {
			context.GetLogger(ctx).Errorf("error creating ImageStreamMapping: %s", err)
			return "", err
		}

		if _, err := m.repo.createImageStream(ctx); err != nil {
			if e, ok := err.(errcode.Error); ok && e.ErrorCode() == errcode.ErrorCodeUnknown {
				// TODO: convert statusErr to distribution error
				return "", statusErr
			}
			return "", err
		}

		// try to create the ISM again
		if err := m.repo.registryOSClient.ImageStreamMappings(m.repo.namespace).Create(&ism); err != nil {
			if quotautil.IsErrorQuotaExceeded(err) {
				context.GetLogger(ctx).Errorf("denied a creation of ImageStreamMapping: %v", err)
				return "", distribution.ErrAccessDenied
			}
			context.GetLogger(ctx).Errorf("error creating ImageStreamMapping: %s", err)
			return "", err
		}
	}

	return dgst, nil
}

// Delete deletes the manifest with digest `dgst`. Note: Image resources
// in OpenShift are deleted via 'oadm prune images'. This function deletes
// the content related to the manifest in the registry's storage (signatures).
func (m *manifestService) Delete(ctx context.Context, dgst digest.Digest) error {
	context.GetLogger(ctx).Debugf("(*manifestService).Delete")
	return m.manifests.Delete(withRepository(ctx, m.repo), dgst)
}

// manifestInflight tracks currently downloading manifests
var manifestInflight = make(map[digest.Digest]struct{})

// manifestInflightSync protects manifestInflight
var manifestInflightSync sync.Mutex

func (m *manifestService) migrateManifest(ctx context.Context, image *imageapi.Image, dgst digest.Digest, manifest distribution.Manifest, isLocalStored bool) {
	// Everything in its place and nothing to do.
	if isLocalStored && len(image.DockerImageManifest) == 0 {
		return
	}
	manifestInflightSync.Lock()
	if _, ok := manifestInflight[dgst]; ok {
		manifestInflightSync.Unlock()
		return
	}
	manifestInflight[dgst] = struct{}{}
	manifestInflightSync.Unlock()

	go m.storeManifestLocally(ctx, image, dgst, manifest, isLocalStored)
}

func (m *manifestService) storeManifestLocally(ctx context.Context, image *imageapi.Image, dgst digest.Digest, manifest distribution.Manifest, isLocalStored bool) {
	defer func() {
		manifestInflightSync.Lock()
		delete(manifestInflight, dgst)
		manifestInflightSync.Unlock()
	}()

	if !isLocalStored {
		if _, err := m.manifests.Put(ctx, manifest); err != nil {
			context.GetLogger(ctx).Errorf("unable to put manifest to storage: %v", err)
			return
		}
	}

	if len(image.DockerImageManifest) == 0 {
		return
	}

	image.DockerImageManifest = ""
	image.DockerImageConfig = ""

	if _, err := m.repo.updateImage(image); err != nil {
		context.GetLogger(ctx).Errorf("error updating Image: %v", err)
	}
}
