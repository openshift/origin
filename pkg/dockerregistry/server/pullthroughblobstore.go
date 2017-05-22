package server

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
)

// pullthroughBlobStore wraps a distribution.BlobStore and allows remote repositories to serve blobs from remote
// repositories.
type pullthroughBlobStore struct {
	distribution.BlobStore

	repo   *repository
	mirror bool
}

var _ distribution.BlobStore = &pullthroughBlobStore{}

// Stat makes a local check for the blob, then falls through to the other servers referenced by
// the image stream and looks for those that have the layer.
func (pbs *pullthroughBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	context.GetLogger(ctx).Debugf("(*pullthroughBlobStore).Stat: starting with dgst=%s", dgst.String())

	// check the local store for the blob
	desc, err := pbs.BlobStore.Stat(ctx, dgst)
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

	return pbs.repo.remoteBlobGetter.Stat(ctx, dgst)
}

// ServeBlob attempts to serve the requested digest onto w, using a remote proxy store if necessary.
// Important! This function is called for GET and HEAD requests. Docker client uses[1] HEAD request
// to check existence of a layer. If the layer with the digest is available, this function MUST return
// success response with no actual body content.
// [1] https://docs.docker.com/registry/spec/api/#existing-layers
func (pbs *pullthroughBlobStore) ServeBlob(ctx context.Context, rw http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	context.GetLogger(ctx).Debugf("(*pullthroughBlobStore).ServeBlob: starting with dgst=%s", dgst.String())
	// This call should be done without BlobGetterService in the context.
	err := pbs.BlobStore.ServeBlob(ctx, rw, req, dgst)
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

	var bw distribution.BlobWriter

	if pbs.mirror {
		bw, err = pbs.BlobStore.Create(ctx)
		if err != nil {
			return err
		}

		rw = &blobResponseMirrorer{
			ResponseWriter: rw,
			writer:         io.MultiWriter(rw, bw),
		}
	}

	desc, err := pbs.copyContent(ctx, rw, req, dgst, pbs.repo.remoteBlobGetter)
	if pbs.mirror {
		if err == nil {
			bw.Commit(ctx, desc)
		} else {
			bw.Cancel(ctx)
		}
	}
	return err
}

// Get attempts to fetch the requested blob by digest using a remote proxy store if necessary.
func (pbs *pullthroughBlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	context.GetLogger(ctx).Debugf("(*pullthroughBlobStore).Get: starting with dgst=%s", dgst.String())
	data, originalErr := pbs.BlobStore.Get(ctx, dgst)
	if originalErr == nil {
		return data, nil
	}

	return pbs.repo.remoteBlobGetter.Get(ctx, dgst)
}

// setResponseHeaders sets the appropriate content serving headers
func setResponseHeaders(w http.ResponseWriter, length int64, mediaType string, digest digest.Digest) {
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Docker-Content-Digest", digest.String())
	w.Header().Set("Etag", digest.String())
}

// serveRemoteContent tries to use http.ServeContent for remote content.
func (pbs *pullthroughBlobStore) serveRemoteContent(rw http.ResponseWriter, req *http.Request, desc distribution.Descriptor, remoteReader io.ReadSeeker) (bool, error) {
	// Set the appropriate content serving headers.
	setResponseHeaders(rw, desc.Size, desc.MediaType, desc.Digest)

	// Fallback to Copy if request wasn't given.
	if req == nil {
		return false, nil
	}

	// Check whether remoteReader is seekable. The remoteReader' Seek method must work: ServeContent uses
	// a seek to the end of the content to determine its size.
	if _, err := remoteReader.Seek(0, os.SEEK_END); err != nil {
		// The remoteReader isn't seekable. It means that the remote response under the hood of remoteReader
		// doesn't contain any Content-Range or Content-Length headers. In this case we need to rollback to
		// simple Copy.
		return false, nil
	}

	// Move pointer back to begin.
	if _, err := remoteReader.Seek(0, os.SEEK_SET); err != nil {
		return false, err
	}

	http.ServeContent(rw, req, desc.Digest.String(), time.Time{}, remoteReader)

	return true, nil
}

// copyContent attempts to load and serve the provided blob.
func (pbs *pullthroughBlobStore) copyContent(ctx context.Context, writer http.ResponseWriter, req *http.Request, dgst digest.Digest, store BlobGetterService) (distribution.Descriptor, error) {
	desc, err := store.Stat(ctx, dgst)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	remoteReader, err := store.Open(ctx, dgst)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	contentHandled, err := pbs.serveRemoteContent(writer, req, desc, remoteReader)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	if contentHandled {
		return desc, nil
	}

	writer.Header().Set("Content-Length", fmt.Sprintf("%d", desc.Size))

	if _, err = io.CopyN(writer, remoteReader, desc.Size); err != nil {
		return distribution.Descriptor{}, err
	}

	return desc, nil
}

// blobResponseMirrorer wraps a http.ResponseWriter to duplicates its writes to multiple streams.
type blobResponseMirrorer struct {
	http.ResponseWriter

	writer io.Writer
}

func (brm *blobResponseMirrorer) Write(p []byte) (int, error) {
	return brm.writer.Write(p)
}
