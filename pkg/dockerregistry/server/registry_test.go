package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/storage"
	"github.com/docker/distribution/registry/storage/cache"
	"github.com/docker/distribution/registry/storage/cache/memory"
	"github.com/docker/distribution/registry/storage/driver"
	"github.com/docker/distribution/registry/storage/driver/inmemory"

	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"

	"github.com/openshift/origin/pkg/dockerregistry/server/client"
	registryclient "github.com/openshift/origin/pkg/dockerregistry/server/client"
	"github.com/openshift/origin/pkg/dockerregistry/server/configuration"
)

type testRegistry struct {
	distribution.Namespace
	app                    *App
	osClient               registryclient.Interface
	pullthrough            bool
	blobrepositorycachettl time.Duration
}

var _ distribution.Namespace = &testRegistry{}

func newTestRegistry(
	ctx context.Context,
	osClient registryclient.Interface,
	storageDriver driver.StorageDriver,
	blobrepositorycachettl time.Duration,
	pullthrough bool,
	useBlobDescriptorCacheProvider bool,
) (*testRegistry, error) {
	if storageDriver == nil {
		storageDriver = inmemory.New()
	}

	opts := []storage.RegistryOption{
		storage.BlobDescriptorServiceFactory(&blobDescriptorServiceFactory{}),
		storage.EnableDelete,
		storage.EnableRedirect,
	}
	if useBlobDescriptorCacheProvider {
		cacheProvider := cache.BlobDescriptorCacheProvider(memory.NewInMemoryBlobDescriptorCacheProvider())
		opts = append(opts, storage.BlobDescriptorCacheProvider(cacheProvider))
	}

	reg, err := storage.NewRegistry(ctx, storageDriver, opts...)
	if err != nil {
		return nil, err
	}

	cachedLayers, err := newDigestToRepositoryCache(defaultDigestToRepositoryCacheSize)
	if err != nil {
		return nil, err
	}

	app := &App{
		extraConfig:  &configuration.Configuration{},
		driver:       storageDriver,
		registry:     reg,
		cachedLayers: cachedLayers,
		quotaEnforcing: &quotaEnforcingConfig{
			enforcementEnabled: false,
		},
	}

	return &testRegistry{
		Namespace:              reg,
		app:                    app,
		osClient:               osClient,
		blobrepositorycachettl: blobrepositorycachettl,
		pullthrough:            pullthrough,
	}, nil
}

func (reg *testRegistry) Repository(ctx context.Context, ref reference.Named) (distribution.Repository, error) {
	repo, err := reg.Namespace.Repository(ctx, ref)
	if err != nil {
		return nil, err
	}

	parts := strings.SplitN(ref.Name(), "/", 3)
	if len(parts) != 2 {
		return nil, fmt.Errorf("failed to parse repository name %q", ref.Name())
	}

	nm, name := parts[0], parts[1]

	isGetter := &cachedImageStreamGetter{
		ctx:          ctx,
		namespace:    nm,
		name:         name,
		isNamespacer: reg.osClient,
	}

	r := &repository{
		Repository: repo,

		ctx:              ctx,
		app:              reg.app,
		registryOSClient: reg.osClient,
		namespace:        nm,
		name:             name,
		config: repositoryConfig{
			registryAddr:           "localhost:5000",
			blobRepositoryCacheTTL: reg.blobrepositorycachettl,
			pullthrough:            reg.pullthrough,
		},
		imageStreamGetter: isGetter,
		cachedImages:      make(map[digest.Digest]*imageapiv1.Image),
		cachedLayers:      reg.app.cachedLayers,
	}

	if reg.pullthrough {
		r.remoteBlobGetter = NewBlobGetterService(
			nm,
			name,
			defaultBlobRepositoryCacheTTL,
			isGetter.get,
			reg.osClient,
			reg.app.cachedLayers)
	}

	return r, nil
}

type testRepository struct {
	distribution.Repository

	name  reference.Named
	blobs distribution.BlobStore
}

func (r *testRepository) Named() reference.Named {
	return r.name
}

func (r *testRepository) Blobs(ctx context.Context) distribution.BlobStore {
	return r.blobs
}

type testRepositoryOptions struct {
	client            client.Interface
	enablePullThrough bool
	blobs             distribution.BlobStore
}

func newTestRepository(
	ctx context.Context,
	t *testing.T,
	namespace, repoName string,
	opts testRepositoryOptions,
) *repository {
	reg, err := newTestRegistry(ctx, opts.client, nil, 0, opts.enablePullThrough, false)
	if err != nil {
		t.Fatal(err)
	}

	named, err := reference.ParseNamed(fmt.Sprintf("%s/%s", namespace, repoName))
	if err != nil {
		t.Fatal(err)
	}

	repo, err := reg.Repository(ctx, named)
	if err != nil {
		t.Fatal(err)
	}

	r := repo.(*repository)
	// TODO(dmage): can we avoid this replacement?
	r.Repository = &testRepository{
		name:  named,
		blobs: opts.blobs,
	}
	return r
}
