package metrics

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
)

// ManifestService wraps a distribution.ManifestService to collect statistics
type ManifestService struct {
	Manifests distribution.ManifestService
	Reponame  string
}

var _ distribution.ManifestService = &ManifestService{}

func (m *ManifestService) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	defer NewTimer(RegistryAPIRequests, []string{"manifestservice.exists", m.Reponame}).Stop()
	return m.Manifests.Exists(ctx, dgst)
}

func (m *ManifestService) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	defer NewTimer(RegistryAPIRequests, []string{"manifestservice.get", m.Reponame}).Stop()
	return m.Manifests.Get(ctx, dgst, options...)
}

func (m *ManifestService) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	defer NewTimer(RegistryAPIRequests, []string{"manifestservice.put", m.Reponame}).Stop()
	return m.Manifests.Put(ctx, manifest, options...)
}

func (m *ManifestService) Delete(ctx context.Context, dgst digest.Digest) error {
	defer NewTimer(RegistryAPIRequests, []string{"manifestservice.delete", m.Reponame}).Stop()
	return m.Manifests.Delete(ctx, dgst)
}
