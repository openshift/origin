package handlers

import (
	"net/http"
	"time"

	"github.com/docker/distribution"
	ctxu "github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/gorilla/handlers"
)

// layerDispatcher uses the request context to build a layerHandler.
func layerDispatcher(ctx *Context, r *http.Request) http.Handler {
	dgst, err := getDigest(ctx)
	if err != nil {

		if err == errDigestNotAvailable {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				ctx.Errors.Push(v2.ErrorCodeDigestInvalid, err)
			})
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx.Errors.Push(v2.ErrorCodeDigestInvalid, err)
		})
	}

	layerHandler := &layerHandler{
		Context: ctx,
		Digest:  dgst,
	}

	return handlers.MethodHandler{
		"GET":  http.HandlerFunc(layerHandler.GetLayer),
		"HEAD": http.HandlerFunc(layerHandler.GetLayer),
	}
}

// layerHandler serves http layer requests.
type layerHandler struct {
	*Context

	Digest digest.Digest
}

// GetLayer fetches the binary data from backend storage returns it in the
// response.
func (lh *layerHandler) GetLayer(w http.ResponseWriter, r *http.Request) {
	var (
		layer distribution.Layer
		err   error
	)

	ctxu.GetLogger(lh).Debug("GetImageLayer")
	layers := lh.Repository.Layers()

	// On NFS, recently pushed layer by another registry instance may be unseen to us
	// for a second. Retry the fetch few times.
Loop:
	for retries := 0; ; retries++ {
		layer, err = layers.Fetch(lh.Digest)
		if err == nil {
			if retries > 0 {
				ctxu.GetLogger(lh).Debugf("(*layerHandler).GetLayer: layer fetched after %d failed attempts", retries)
			}
			break Loop
		}
		switch err.(type) {
		case distribution.ErrUnknownLayer:
			if retries > 10 {
				break Loop
			}
			time.Sleep(100 * time.Millisecond * time.Duration(retries+1))
		default:
			break Loop
		}
	}

	if err != nil {
		switch err := err.(type) {
		case distribution.ErrUnknownLayer:
			w.WriteHeader(http.StatusNotFound)
			lh.Errors.Push(v2.ErrorCodeBlobUnknown, err.FSLayer)
		default:
			lh.Errors.Push(v2.ErrorCodeUnknown, err)
		}
		return
	}

	handler, err := layer.Handler(r)
	if err != nil {
		ctxu.GetLogger(lh).Debugf("unexpected error getting layer HTTP handler: %s", err)
		lh.Errors.Push(v2.ErrorCodeUnknown, err)
		return
	}

	handler.ServeHTTP(w, r)
}
