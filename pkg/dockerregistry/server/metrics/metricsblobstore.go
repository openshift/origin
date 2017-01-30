package metrics

import (
	"net/http"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
)

// BlobStore wraps a distribution.BlobStore to collect statistics
type BlobStore struct {
	Store    distribution.BlobStore
	Reponame string
}

var _ distribution.BlobStore = &BlobStore{}

func (b *BlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	defer NewTimer(RegistryAPIRequests, []string{"blobstore.stat", b.Reponame}).Stop()
	return b.Store.Stat(ctx, dgst)
}

func (b *BlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	defer NewTimer(RegistryAPIRequests, []string{"blobstore.get", b.Reponame}).Stop()
	return b.Store.Get(ctx, dgst)
}

func (b *BlobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	defer NewTimer(RegistryAPIRequests, []string{"blobstore.open", b.Reponame}).Stop()
	return b.Store.Open(ctx, dgst)
}

func (b *BlobStore) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	defer NewTimer(RegistryAPIRequests, []string{"blobstore.put", b.Reponame}).Stop()
	return b.Store.Put(ctx, mediaType, p)
}

func (b *BlobStore) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	defer NewTimer(RegistryAPIRequests, []string{"blobstore.create", b.Reponame}).Stop()

	writer, err := b.Store.Create(ctx, options...)

	return &metricsBlobWriter{
		BlobWriter: writer,
		reponame:   b.Reponame,
	}, err
}

func (b *BlobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	defer NewTimer(RegistryAPIRequests, []string{"blobstore.resume", b.Reponame}).Stop()
	return b.Store.Resume(ctx, id)
}

func (b *BlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	defer NewTimer(RegistryAPIRequests, []string{"blobstore.serveblob", b.Reponame}).Stop()
	return b.Store.ServeBlob(ctx, w, req, dgst)
}

func (b *BlobStore) Delete(ctx context.Context, dgst digest.Digest) error {
	defer NewTimer(RegistryAPIRequests, []string{"blobstore.delete", b.Reponame}).Stop()
	return b.Store.Delete(ctx, dgst)
}

type metricsBlobWriter struct {
	distribution.BlobWriter
	reponame string
}

func (bw *metricsBlobWriter) Commit(ctx context.Context, provisional distribution.Descriptor) (canonical distribution.Descriptor, err error) {
	defer NewTimer(RegistryAPIRequests, []string{"blobwriter.commit", bw.reponame}).Stop()
	return bw.BlobWriter.Commit(ctx, provisional)
}

func (bw *metricsBlobWriter) Cancel(ctx context.Context) error {
	defer NewTimer(RegistryAPIRequests, []string{"blobwriter.cancel", bw.reponame}).Stop()
	return bw.BlobWriter.Cancel(ctx)
}
