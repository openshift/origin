package server

import (
	"fmt"
	"testing"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/reference"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

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
	client            *testclient.Fake
	enablePullThrough bool
	blobs             distribution.BlobStore
}

func newTestRepository(
	t *testing.T,
	namespace, repo string,
	opts testRepositoryOptions,
) *repository {
	cachedLayers, err := newDigestToRepositoryCache(10)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	isGetter := &cachedImageStreamGetter{
		ctx:          ctx,
		namespace:    namespace,
		name:         repo,
		isNamespacer: opts.client,
	}

	repoName := fmt.Sprintf("%s/%s", namespace, repo)
	named, err := reference.ParseNamed(repoName)
	if err != nil {
		t.Fatal(err)
	}

	r := &repository{
		Repository: &testRepository{
			name:  named,
			blobs: opts.blobs,
		},

		ctx:               ctx,
		namespace:         namespace,
		name:              repo,
		pullthrough:       opts.enablePullThrough,
		cachedLayers:      cachedLayers,
		registryOSClient:  opts.client,
		imageStreamGetter: isGetter,
		cachedImages:      make(map[digest.Digest]*imageapi.Image),
	}

	if opts.enablePullThrough {
		r.remoteBlobGetter = NewBlobGetterService(
			namespace,
			repo,
			defaultBlobRepositoryCacheTTL,
			isGetter.get,
			opts.client,
			cachedLayers,
		)
	}

	return r
}
