package writelimiter

import (
	"errors"
	"sync"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"

	"github.com/openshift/origin/pkg/dockerregistry/server/types"
)

// ErrResourcesExhausted is returned when no more BlobWriters can be created
// because of the limit.
var ErrResourcesExhausted = errors.New("writers limit is reached")

type writerBouncer struct {
	sem  *semaphore
	once sync.Once
}

func newWriterBouncer(sem *semaphore) (*writerBouncer, error) {
	if !sem.TryDown() {
		return nil, ErrResourcesExhausted
	}
	return &writerBouncer{
		sem: sem,
	}, nil
}

func (g *writerBouncer) Release() {
	g.once.Do(g.sem.Up)
}

type blobWriter struct {
	distribution.BlobWriter
	bouncer *writerBouncer
}

var _ distribution.BlobWriter = blobWriter{}

func (bw blobWriter) Close() error {
	bw.bouncer.Release()
	return bw.BlobWriter.Close()
}

func (bw blobWriter) Commit(ctx context.Context, desc distribution.Descriptor) (distribution.Descriptor, error) {
	bw.bouncer.Release()
	return bw.BlobWriter.Commit(ctx, desc)
}

func (bw blobWriter) Cancel(ctx context.Context) error {
	bw.bouncer.Release()
	return bw.BlobWriter.Cancel(ctx)
}

type blobStore struct {
	distribution.BlobStore
	sem *semaphore
}

var _ distribution.BlobStore = blobStore{}

func (bs blobStore) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	bouncer, err := newWriterBouncer(bs.sem)
	if err != nil {
		return nil, err
	}

	bw, err := bs.BlobStore.Create(ctx, options...)
	if err != nil {
		bouncer.Release()
		return bw, err
	}

	return blobWriter{
		BlobWriter: bw,
		bouncer:    bouncer,
	}, nil
}

func (bs blobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	bouncer, err := newWriterBouncer(bs.sem)
	if err != nil {
		return nil, err
	}

	bw, err := bs.BlobStore.Resume(ctx, id)
	if err != nil {
		bouncer.Release()
		return bw, err
	}

	return blobWriter{
		BlobWriter: bw,
		bouncer:    bouncer,
	}, nil
}

type blobStoreFactory struct {
	sem *semaphore
}

var _ types.BlobStoreFactory = blobStoreFactory{}

// NewBlobStoreFactory creates a factory of BlobStores which together can
// create no more than limit BlobWriters at a time.
func NewBlobStoreFactory(limit int) types.BlobStoreFactory {
	return blobStoreFactory{
		sem: newSemaphore(limit),
	}
}

func (f blobStoreFactory) BlobStore(bs distribution.BlobStore) distribution.BlobStore {
	return blobStore{
		BlobStore: bs,
		sem:       f.sem,
	}
}
