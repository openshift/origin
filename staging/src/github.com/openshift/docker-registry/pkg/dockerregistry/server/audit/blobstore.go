package audit

import (
	"net/http"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
)

// BlobStore wraps a distribution.BlobStore to track operation result and
// write it in the audit log.
type BlobStore struct {
	store  distribution.BlobStore
	logger *AuditLogger
}

func NewBlobStore(ctx context.Context, store distribution.BlobStore) distribution.BlobStore {
	return &BlobStore{
		store:  store,
		logger: GetLogger(ctx),
	}
}

func (b *BlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	b.logger.Log("BlobStore.Stat")
	desc, err := b.store.Stat(ctx, dgst)
	b.logger.LogResult(err, "BlobStore.Stat")
	return desc, err
}

func (b *BlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	b.logger.Log("BlobStore.Get")
	blob, err := b.store.Get(ctx, dgst)
	b.logger.LogResult(err, "BlobStore.Get")
	return blob, err
}

func (b *BlobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	b.logger.Log("BlobStore.Open")
	reader, err := b.store.Open(ctx, dgst)
	b.logger.LogResult(err, "BlobStore.Open")
	return reader, err
}

func (b *BlobStore) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	b.logger.Log("BlobStore.Put")
	desc, err := b.store.Put(ctx, mediaType, p)
	b.logger.LogResult(err, "BlobStore.Put")
	return desc, err
}

func (b *BlobStore) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	b.logger.Log("BlobStore.Create")
	writer, err := b.store.Create(ctx, options...)
	b.logger.LogResult(err, "BlobStore.Create")
	return &blobWriter{
		BlobWriter: writer,
		logger:     b.logger,
	}, err
}

func (b *BlobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	b.logger.Log("BlobStore.Resume")
	writer, err := b.store.Resume(ctx, id)
	b.logger.LogResult(err, "BlobStore.Resume")
	return writer, err
}

func (b *BlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	b.logger.Log("BlobStore.ServeBlob")
	err := b.store.ServeBlob(ctx, w, req, dgst)
	b.logger.LogResult(err, "BlobStore.ServeBlob")
	return err
}

func (b *BlobStore) Delete(ctx context.Context, dgst digest.Digest) error {
	b.logger.Log("BlobStore.Delete")
	err := b.store.Delete(ctx, dgst)
	b.logger.LogResult(err, "BlobStore.Delete")
	return err
}

type blobWriter struct {
	distribution.BlobWriter
	logger *AuditLogger
}

func (bw *blobWriter) Commit(ctx context.Context, provisional distribution.Descriptor) (canonical distribution.Descriptor, err error) {
	bw.logger.Log("BlobWriter.Commit")
	desc, err := bw.BlobWriter.Commit(ctx, provisional)
	bw.logger.LogResult(err, "BlobWriter.Commit")
	return desc, err
}

func (bw *blobWriter) Cancel(ctx context.Context) error {
	bw.logger.Log("BlobWriter.Cancel")
	err := bw.BlobWriter.Cancel(ctx)
	bw.logger.LogResult(err, "BlobWriter.Cancel")
	return err
}
