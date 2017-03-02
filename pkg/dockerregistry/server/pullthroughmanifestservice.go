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
	context.GetLogger(ctx).Debugf("(*pullthroughManifestService).remoteGet: starting with dgst=%s", dgst.String())
	image, is, err := m.repo.getImageOfImageStream(dgst)
	if err != nil {
		return nil, err
	}

	m.pullFromInsecureRegistries = false

	if insecure, ok := is.Annotations[imageapi.InsecureRepositoryAnnotation]; ok {
		m.pullFromInsecureRegistries = insecure == "true"
	}

	ref, err := imageapi.ParseDockerImageReference(image.DockerImageReference)
	if err != nil {
		context.GetLogger(ctx).Errorf("bad DockerImageReference in Image %s/%s@%s: %v", m.repo.namespace, m.repo.name, dgst.String(), err)
		return nil, err
	}
	ref = ref.DockerClientDefaults()

	retriever := getImportContext(ctx, m.repo.registryOSClient, is.Namespace, is.Name)

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

func (m *pullthroughManifestService) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	context.GetLogger(ctx).Debugf("(*pullthroughManifestService).Put: enabling remote blob access check")
	// manifest dependencies (layers and config) may not be stored locally, we need to be able to stat them in remote repositories
	ctx = WithRemoteBlobAccessCheckEnabled(ctx, true)
	return m.ManifestService.Put(ctx, manifest, options...)
}
