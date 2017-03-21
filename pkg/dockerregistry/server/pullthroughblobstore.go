package server

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
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
func (pbs *pullthroughBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	context.GetLogger(ctx).Debugf("(*pullthroughBlobStore).ServeBlob: starting with dgst=%s", dgst.String())
	// This call should be done without BlobGetterService in the context.
	err := pbs.BlobStore.ServeBlob(ctx, w, req, dgst)
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

	remoteGetter := pbs.repo.remoteBlobGetter

	// store the content locally if requested, but ensure only one instance at a time
	// is storing to avoid excessive local writes
	if pbs.mirror {
		mu.Lock()
		if _, ok := inflight[dgst]; ok {
			mu.Unlock()
			context.GetLogger(ctx).Infof("Serving %q while mirroring in background", dgst)
			_, err := pbs.copyContent(remoteGetter, ctx, dgst, w, req)
			return err
		}
		inflight[dgst] = struct{}{}
		mu.Unlock()

		go func(dgst digest.Digest) {
			context.GetLogger(ctx).Infof("Start background mirroring of %q", dgst)
			if err := pbs.storeLocal(remoteGetter, ctx, dgst); err != nil {
				context.GetLogger(ctx).Errorf("Error committing to storage: %s", err.Error())
			}
			context.GetLogger(ctx).Infof("Completed mirroring of %q", dgst)
		}(dgst)
	}

	_, err = pbs.copyContent(remoteGetter, ctx, dgst, w, req)
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
func serveRemoteContent(rw http.ResponseWriter, req *http.Request, desc distribution.Descriptor, remoteReader io.ReadSeeker) (bool, error) {
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

// inflight tracks currently downloading blobs
var inflight = make(map[digest.Digest]struct{})

// mu protects inflight
var mu sync.Mutex

// copyContent attempts to load and serve the provided blob. If req != nil and writer is an instance of http.ResponseWriter,
// response headers will be set and range requests honored.
func (pbs *pullthroughBlobStore) copyContent(store BlobGetterService, ctx context.Context, dgst digest.Digest, writer io.Writer, req *http.Request) (distribution.Descriptor, error) {
	desc, err := store.Stat(ctx, dgst)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	remoteReader, err := store.Open(ctx, dgst)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	rw, ok := writer.(http.ResponseWriter)
	if ok {
		contentHandled, err := serveRemoteContent(rw, req, desc, remoteReader)
		if err != nil {
			return distribution.Descriptor{}, err
		}

		if contentHandled {
			return desc, nil
		}

		rw.Header().Set("Content-Length", fmt.Sprintf("%d", desc.Size))
	}

	if _, err = io.CopyN(writer, remoteReader, desc.Size); err != nil {
		return distribution.Descriptor{}, err
	}
	return desc, nil
}

// storeLocal retrieves the named blob from the provided store and writes it into the local store.
func (pbs *pullthroughBlobStore) storeLocal(remoteGetter BlobGetterService, ctx context.Context, dgst digest.Digest) error {
	defer func() {
		mu.Lock()
		delete(inflight, dgst)
		mu.Unlock()
	}()

	var desc distribution.Descriptor
	var err error
	var bw distribution.BlobWriter

	bw, err = pbs.BlobStore.Create(ctx)
	if err != nil {
		return err
	}

	desc, err = pbs.copyContent(remoteGetter, ctx, dgst, bw, nil)
	if err != nil {
		return err
	}

	_, err = bw.Commit(ctx, desc)
	if err != nil {
		return err
	}

	return nil
}
