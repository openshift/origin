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

	vacuum := storage.NewVacuum(bh.Context, dockerStorageDriver)

	err := vacuum.RemoveBlob(bh.Digest.String())
	if err != nil {
		// ignore not found error
		switch t := err.(type) {
		case storagedriver.PathNotFoundError:
		case errcode.Error:
			if t.Code != v2.ErrorCodeBlobUnknown {
				bh.Errors = append(bh.Errors, err)
				return
			}
		default:
			if err != distribution.ErrBlobUnknown {
				detail := fmt.Sprintf("error deleting blob %q: %v", bh.Digest, err)
				err = errcode.ErrorCodeUnknown.WithDetail(detail)
				bh.Errors = append(bh.Errors, err)
				return
			}
		}
		context.GetLogger(bh).Infof("blobHandler: ignoring %T error: %v", err, err)
	}

	w.WriteHeader(http.StatusNoContent)
}
