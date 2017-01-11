package server

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// pullthroughManifestService wraps a distribution.ManifestService
// repositories. Since the manifest is no longer stored in the Image
// the docker-registry must pull through requests to manifests as well
// as to blobs.
type pullthroughManifestService struct {
	distribution.ManifestService

	repo                       *repository
	pullFromInsecureRegistries bool
}

var _ distribution.ManifestService = &pullthroughManifestService{}

func (m *pullthroughManifestService) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	manifest, err := m.ManifestService.Get(ctx, dgst, options...)
	switch err.(type) {
	case distribution.ErrManifestUnknownRevision:
		break
	case nil:
		return manifest, nil
	default:
		return nil, err
	}

	return m.remoteGet(ctx, dgst, options...)
}

func (m *pullthroughManifestService) remoteGet(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	isi, err := m.repo.getImageStreamImage(dgst)
	if err != nil {
		context.GetLogger(ctx).Errorf("error retrieving ImageStreamImage %s/%s@%s: %v", m.repo.namespace, m.repo.name, dgst.String(), err)
		return nil, err
	}

	m.pullFromInsecureRegistries = false

	if insecure, ok := isi.Annotations[imageapi.InsecureRepositoryAnnotation]; ok {
		m.pullFromInsecureRegistries = insecure == "true"
	}

	ref, err := imageapi.ParseDockerImageReference(isi.Image.DockerImageReference)
	if err != nil {
		context.GetLogger(ctx).Errorf("bad DockerImageReference in Image %s/%s@%s: %v", m.repo.namespace, m.repo.name, dgst.String(), err)
		return nil, err
	}
	ref = ref.DockerClientDefaults()

	retriever := m.repo.importContext()

	repo, err := retriever.Repository(ctx, ref.RegistryURL(), ref.RepositoryName(), m.pullFromInsecureRegistries)
	if err != nil {
		context.GetLogger(ctx).Errorf("error getting remote repository for image %q: %v", ref.Exact(), err)
		return nil, err
	}

	pullthroughManifestService, err := repo.Manifests(ctx)
	if err != nil {
		context.GetLogger(ctx).Errorf("error getting remote manifests for image %q: %v", ref.Exact(), err)
		return nil, err
	}

	manifest, err := pullthroughManifestService.Get(ctx, dgst)
	switch err.(type) {
	case nil:
		m.repo.rememberLayersOfManifest(dgst, manifest, ref.Exact())
	case distribution.ErrManifestUnknownRevision:
		break
	default:
		context.GetLogger(ctx).Errorf("error getting manifest from remote location %q: %v", ref.Exact(), err)
	}

	return manifest, err
}
