package server

import (
	"fmt"
	"net/http"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	ctxu "github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/handlers"
	"github.com/docker/distribution/registry/storage"
	storagedriver "github.com/docker/distribution/registry/storage/driver"
	gorillahandlers "github.com/gorilla/handlers"
)

// BlobDispatcher takes the request context and builds the appropriate handler
// for handling blob requests.
func BlobDispatcher(ctx *handlers.Context, r *http.Request) http.Handler {
	reference := ctxu.GetStringValue(ctx, "vars.digest")
	dgst, _ := digest.ParseDigest(reference)

	blobHandler := &blobHandler{
		Context: ctx,
		Digest:  dgst,
	}

	return gorillahandlers.MethodHandler{
		"DELETE": http.HandlerFunc(blobHandler.Delete),
	}
}

// blobHandler handles http operations on blobs.
type blobHandler struct {
	*handlers.Context

	Digest digest.Digest
}

// Delete deletes the blob from the storage backend.
func (bh *blobHandler) Delete(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	if len(bh.Digest) == 0 {
		bh.Errors = append(bh.Errors, v2.ErrorCodeBlobUnknown)
		return
	}

	bd, err := storage.RegistryBlobDeleter(bh.Namespace())
	if err != nil {
		bh.Errors = append(bh.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}

	err = bd.Delete(bh, bh.Digest)
	if ignoreNotFoundError(bh.Context, err, fmt.Sprintf("error deleting blob %q", bh.Digest)) == nil {
		w.WriteHeader(http.StatusNoContent)
	}
}

// LayerDispatcher takes the request context and builds the appropriate handler
// for handling layer requests.
func LayerDispatcher(ctx *handlers.Context, r *http.Request) http.Handler {
	reference := ctxu.GetStringValue(ctx, "vars.digest")
	dgst, _ := digest.ParseDigest(reference)

	layerHandler := &layerHandler{
		Context: ctx,
		Digest:  dgst,
	}

	return gorillahandlers.MethodHandler{
		"DELETE": http.HandlerFunc(layerHandler.Delete),
	}
}

// layerHandler handles http operations on layers.
type layerHandler struct {
	*handlers.Context

	Digest digest.Digest
}

// Delete deletes the layer link from the repository from the storage backend.
func (lh *layerHandler) Delete(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	if len(lh.Digest) == 0 {
		lh.Errors = append(lh.Errors, v2.ErrorCodeBlobUnknown)
		return
	}

	err := lh.Repository.Blobs(lh).Delete(lh, lh.Digest)
	if ignoreNotFoundError(lh.Context, err, fmt.Sprintf("error unlinking layer %q from repo %q", lh.Digest, lh.Repository.Name())) == nil {
		w.WriteHeader(http.StatusNoContent)
	}
}

// ManifestDispatcher takes the request context and builds the appropriate
// handler for handling manifest requests.
func ManifestDispatcher(ctx *handlers.Context, r *http.Request) http.Handler {
	reference := ctxu.GetStringValue(ctx, "vars.digest")
	dgst, _ := digest.ParseDigest(reference)

	manifestHandler := &manifestHandler{
		Context: ctx,
		Digest:  dgst,
	}

	return gorillahandlers.MethodHandler{
		"DELETE": http.HandlerFunc(manifestHandler.Delete),
	}
}

// manifestHandler handles http operations on mainfests.
type manifestHandler struct {
	*handlers.Context

	Digest digest.Digest
}

// Delete deletes the manifest information from the repository from the storage
// backend.
func (mh *manifestHandler) Delete(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	if len(mh.Digest) == 0 {
		mh.Errors = append(mh.Errors, v2.ErrorCodeManifestUnknown)
		return
	}

	manService, err := mh.Repository.Manifests(mh)
	if err != nil {
		mh.Errors = append(mh.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}

	err = manService.Delete(mh.Digest)
	if ignoreNotFoundError(mh.Context, err, fmt.Sprintf("error deleting repo %q, manifest %q", mh.Repository.Name(), mh.Digest)) == nil {
		w.WriteHeader(http.StatusNoContent)
	}
}

// ignoreNotFoundError logs and ignores unknown manifest or blob errors. All
// the other errors will be appended to a list of context errors and returned.
// In case of unexpected error, unknownErrorDetail will be used to create
// ErrorCodeUnknown error with the original err appended.
func ignoreNotFoundError(ctx *handlers.Context, err error, unknownErrorDetail string) error {
	if err != nil {
		switch t := err.(type) {
		case storagedriver.PathNotFoundError:
		case errcode.Error:
			if t.Code != v2.ErrorCodeBlobUnknown {
				ctx.Errors = append(ctx.Errors, err)
				return err
			}
		default:
			if err != distribution.ErrBlobUnknown {
				err = errcode.ErrorCodeUnknown.WithDetail(fmt.Sprintf("%s: %v", unknownErrorDetail, err))
				ctx.Errors = append(ctx.Errors, err)
				return err
			}
		}
		context.GetLogger(ctx).Infof("%T: ignoring %T error: %v", ctx, err, err)
	}

	return nil
}
