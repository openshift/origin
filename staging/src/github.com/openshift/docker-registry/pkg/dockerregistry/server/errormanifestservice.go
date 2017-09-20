package server

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
)

// errorManifestService wraps a distribution.ManifestService for a particular repo.
// before delegating, it ensures auth completed and there were no errors relevant to the repo.
type errorManifestService struct {
	manifests distribution.ManifestService
	repo      *repository
}

var _ distribution.ManifestService = &errorManifestService{}

func (em *errorManifestService) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	if err := em.repo.checkPendingErrors(ctx); err != nil {
		return false, err
	}
	return em.manifests.Exists(ctx, dgst)
}

func (em *errorManifestService) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	if err := em.repo.checkPendingErrors(ctx); err != nil {
		return nil, err
	}
	return em.manifests.Get(ctx, dgst, options...)
}

func (em *errorManifestService) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	if err := em.repo.checkPendingErrors(ctx); err != nil {
		return "", err
	}
	return em.manifests.Put(ctx, manifest, options...)
}

func (em *errorManifestService) Delete(ctx context.Context, dgst digest.Digest) error {
	if err := em.repo.checkPendingErrors(ctx); err != nil {
		return err
	}
	return em.manifests.Delete(ctx, dgst)
}
