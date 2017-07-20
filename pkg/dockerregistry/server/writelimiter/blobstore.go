package writelimiter

import (
	"errors"
	"runtime"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"

	"github.com/openshift/origin/pkg/dockerregistry/server/types"
)

// ErrRequistCanceled is returned when the context is canceled
var ErrRequistCanceled = errors.New("request canceled")

type blobWriter struct {
	distribution.BlobWriter
	release LockFinalizer
}

var _ distribution.BlobWriter = blobWriter{}

func (bw blobWriter) Close() error {
	bw.release()
	return bw.BlobWriter.Close()
}

func (bw blobWriter) Commit(ctx context.Context, desc distribution.Descriptor) (distribution.Descriptor, error) {
	bw.release()
	return bw.BlobWriter.Commit(ctx, desc)
}

func (bw blobWriter) Cancel(ctx context.Context) error {
	bw.release()
	return bw.BlobWriter.Cancel(ctx)
}

type blobStore struct {
	distribution.BlobStore
	lock *cancellableLock
}

var _ distribution.BlobStore = blobStore{}

func (bs blobStore) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	release, ok := bs.lock.Acquire(ctx)
	if !ok {
		return nil, ErrRequistCanceled
	}

	bw, err := bs.BlobStore.Create(ctx, options...)
	if err != nil {
		release()
		return bw, err
	}

	blobwriter := blobWriter{
		BlobWriter: bw,
		release:    release,
	}

	// We must be sure that the lock is released even if Close() has not been called.
	// Otherwise, a leak could happen.
	runtime.SetFinalizer(blobwriter, func(*blobWriter) { release() })

	return blobwriter, nil
}

func (bs blobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	release, ok := bs.lock.Acquire(ctx)
	if !ok {
		return nil, ErrRequistCanceled
	}

	bw, err := bs.BlobStore.Resume(ctx, id)
	if err != nil {
		release()
		return bw, err
	}

	blobwriter := &blobWriter{
		BlobWriter: bw,
		release:    release,
	}

	// We must be sure that the lock is released even if Close() has not been called.
	// Otherwise, a leak could happen.
	runtime.SetFinalizer(blobwriter, func(*blobWriter) { release() })

	return blobwriter, nil
}

type blobStoreFactory struct {
	*cancellableLock
}

var _ types.BlobStoreFactory = blobStoreFactory{}

func NewBlobStoreFactory(limit int) types.BlobStoreFactory {
	return blobStoreFactory{
		cancellableLock: newCancellableLock(limit),
	}
}

func (f blobStoreFactory) BlobStore(bs distribution.BlobStore) distribution.BlobStore {
	return blobStore{
		BlobStore: bs,
		lock:      f.cancellableLock,
	}
}
