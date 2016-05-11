// Package server wraps repository and blob store objects of docker/distribution upstream. Most significantly,
// the wrappers cause manifests to be stored in OpenShift's etcd store instead of registry's storage.
// Registry's middleware API is utilized to register the object factories.
//
// Module with quotaRestrictedBlobStore defines a wrapper for upstream blob store that does an image quota
// check before committing image layer to a registry. Master server contains admission check that will refuse
// the manifest if the image exceeds whatever quota set. But the check occurs too late (after the layers are
// written). This addition allows us to refuse the layers and thus keep the storage clean.
//
// *Note*: Here, we take into account just a single layer, not the image as a whole because the layers are
// uploaded before the manifest. This leads to a situation where several layers can be written until a big
// enough layer will be received that exceeds the limit.
package server

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"

	kapi "k8s.io/kubernetes/pkg/api"

	imageadmission "github.com/openshift/origin/pkg/image/admission"
)

// quotaRestrictedBlobStore wraps upstream blob store with a guard preventing big layers exceeding image quotas
// from being saved.
type quotaRestrictedBlobStore struct {
	distribution.BlobStore

	repo *repository
}

var _ distribution.BlobStore = &quotaRestrictedBlobStore{}

// Create wraps returned blobWriter with quota guard wrapper.
func (bs *quotaRestrictedBlobStore) Create(ctx context.Context) (distribution.BlobWriter, error) {
	context.GetLogger(ctx).Debug("(*quotaRestrictedBlobStore).Create: starting")

	bw, err := bs.BlobStore.Create(ctx)
	if err != nil {
		return nil, err
	}

	repo := (*bs.repo)
	repo.ctx = ctx
	return &quotaRestrictedBlobWriter{
		BlobWriter: bw,
		repo:       &repo,
	}, nil
}

// Resume wraps returned blobWriter with quota guard wrapper.
func (bs *quotaRestrictedBlobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	context.GetLogger(ctx).Debug("(*quotaRestrictedBlobStore).Resume: starting")

	bw, err := bs.BlobStore.Resume(ctx, id)
	if err != nil {
		return nil, err
	}

	repo := (*bs.repo)
	repo.ctx = ctx
	return &quotaRestrictedBlobWriter{
		BlobWriter: bw,
		repo:       &repo,
	}, nil
}

// quotaRestrictedBlobWriter wraps upstream blob writer with a guard preventig big layers exceeding image
// quotas from being written.
type quotaRestrictedBlobWriter struct {
	distribution.BlobWriter

	repo *repository
}

func (bw *quotaRestrictedBlobWriter) Commit(ctx context.Context, provisional distribution.Descriptor) (canonical distribution.Descriptor, err error) {
	context.GetLogger(ctx).Debug("(*quotaRestrictedBlobWriter).Commit: starting")

	if err := admitBlobWrite(ctx, bw.repo, provisional.Size); err != nil {
		return distribution.Descriptor{}, err
	}

	return bw.BlobWriter.Commit(ctx, provisional)
}

// admitBlobWrite checks whether the blob does not exceed image limit ranges if set. Returns ErrAccessDenied
// error if the limit is exceeded.
func admitBlobWrite(ctx context.Context, repo *repository, size int64) error {
	if size < 1 {
		return nil
	}

	limitranges, err := repo.limitClient.LimitRanges(repo.namespace).List(kapi.ListOptions{})
	if err != nil {
		context.GetLogger(ctx).Errorf("Failed to list limitranges: %v", err)
		return err
	}

	for _, limitrange := range limitranges.Items {
		for _, limit := range limitrange.Spec.Limits {
			if err := imageadmission.AdmitImage(size, limit); err != nil {
				context.GetLogger(ctx).Errorf("Refusing to write blob exceeding limit range: %s", err.Error())
				return distribution.ErrAccessDenied
			}
		}
	}

	return nil
}
