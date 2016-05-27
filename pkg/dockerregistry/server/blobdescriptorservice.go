package server

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/middleware/registry"
	"github.com/docker/distribution/registry/storage"
)

func init() {
	middleware.RegisterOptions(storage.BlobDescriptorServiceFactory(&blobDescriptorServiceFactory{}))
}

// blobDescriptorServiceFactory needs to be able to work with blobs
// directly without using links. This allows us to ignore the distribution
// of blobs between repositories.
type blobDescriptorServiceFactory struct{}

func (bf *blobDescriptorServiceFactory) BlobAccessController(svc distribution.BlobDescriptorService) distribution.BlobDescriptorService {
	return &blobDescriptorService{svc}
}

type blobDescriptorService struct {
	distribution.BlobDescriptorService
}

func (bs *blobDescriptorService) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	return dockerRegistry.BlobStatter().Stat(ctx, dgst)
}
