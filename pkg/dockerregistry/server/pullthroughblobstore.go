package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
)

// pullthroughBlobStore wraps a distribution.BlobStore and allows remote repositories to serve blobs from remote
// repositories.
type pullthroughBlobStore struct {
	distribution.BlobStore

	repo *repository
}

var _ distribution.BlobStore = &pullthroughBlobStore{}

// Stat makes a local check for the blob, then falls through to the other servers referenced by
// the image stream and looks for those that have the layer.
func (r *pullthroughBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	// check the local store for the blob
	desc, err := r.BlobStore.Stat(ctx, dgst)
	switch {
	case err == distribution.ErrBlobUnknown:
		// continue on to the code below and look up the blob in a remote store since it is not in
		// the local store
	case err != nil:
		context.GetLogger(ctx).Errorf("Failed to find blob %q: %#v", dgst.String(), err)
		fallthrough
	default:
		return desc, err
	}

	remoteGetter, found := RemoteBlobGetterFrom(r.repo.ctx)
	if !found {
		context.GetLogger(ctx).Errorf("pullthroughBlobStore.Stat: failed to retrieve remote getter from context")
		return distribution.Descriptor{}, distribution.ErrBlobUnknown
	}

	return remoteGetter.Stat(ctx, dgst)
}

// ServeBlob attempts to serve the requested digest onto w, using a remote proxy store if necessary.
func (r *pullthroughBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	// This call should be done without BlobGetterService in the context.
	err := r.BlobStore.ServeBlob(ctx, w, req, dgst)
	switch {
	case err == distribution.ErrBlobUnknown:
		// continue on to the code below and look up the blob in a remote store since it is not in
		// the local store
	case err != nil:
		context.GetLogger(ctx).Errorf("Failed to find blob %q: %#v", dgst.String(), err)
		fallthrough
	default:
		return err
	}

	remoteGetter, found := RemoteBlobGetterFrom(r.repo.ctx)
	if !found {
		context.GetLogger(ctx).Errorf("pullthroughBlobStore.ServeBlob: failed to retrieve remote getter from context")
		return distribution.ErrBlobUnknown
	}

	desc, err := remoteGetter.Stat(ctx, dgst)
	if err != nil {
		context.GetLogger(ctx).Errorf("failed to stat digest %q: %v", dgst.String(), err)
		return err
	}

	remoteReader, err := remoteGetter.Open(ctx, dgst)
	if err != nil {
		context.GetLogger(ctx).Errorf("failure to open remote store for digest %q: %v", dgst.String(), err)
		return err
	}
	defer remoteReader.Close()

	setResponseHeaders(w, desc.Size, desc.MediaType, dgst)

	context.GetLogger(ctx).Infof("serving blob %s of type %s %d bytes long", dgst.String(), desc.MediaType, desc.Size)
	http.ServeContent(w, req, desc.Digest.String(), time.Time{}, remoteReader)
	return nil
}

// Get attempts to fetch the requested blob by digest using a remote proxy store if necessary.
func (r *pullthroughBlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	data, originalErr := r.BlobStore.Get(ctx, dgst)
	if originalErr == nil {
		return data, nil
	}

	remoteGetter, found := RemoteBlobGetterFrom(r.repo.ctx)
	if !found {
		context.GetLogger(ctx).Errorf("pullthroughBlobStore.Get: failed to retrieve remote getter from context")
		return nil, originalErr
	}

	return remoteGetter.Get(ctx, dgst)
}

// setResponseHeaders sets the appropriate content serving headers
func setResponseHeaders(w http.ResponseWriter, length int64, mediaType string, digest digest.Digest) {
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Docker-Content-Digest", digest.String())
	w.Header().Set("Etag", digest.String())
}
