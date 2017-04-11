package audit

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
)

// ManifestService wraps a distribution.ManifestService to track operation result and
// write it in the audit log.
type ManifestService struct {
	manifests distribution.ManifestService
	logger    *AuditLogger
}

func NewManifestService(ctx context.Context, manifests distribution.ManifestService) distribution.ManifestService {
	return &ManifestService{
		manifests: manifests,
		logger:    GetLogger(ctx),
	}
}

func (m *ManifestService) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	m.logger.Log("ManifestService.Exists")
	exists, err := m.manifests.Exists(ctx, dgst)
	m.logger.LogResult(err, "ManifestService.Exists")
	return exists, err
}

func (m *ManifestService) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	m.logger.Log("ManifestService.Get")
	manifest, err := m.manifests.Get(ctx, dgst, options...)
	m.logger.LogResult(err, "ManifestService.Get")
	return manifest, err
}

func (m *ManifestService) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	m.logger.Log("ManifestService.Put")
	dgst, err := m.manifests.Put(ctx, manifest, options...)
	m.logger.LogResult(err, "ManifestService.Put")
	return dgst, err
}

func (m *ManifestService) Delete(ctx context.Context, dgst digest.Digest) error {
	m.logger.Log("ManifestService.Delete")
	err := m.manifests.Delete(ctx, dgst)
	m.logger.LogResult(err, "ManifestService.Delete")
	return err
}
