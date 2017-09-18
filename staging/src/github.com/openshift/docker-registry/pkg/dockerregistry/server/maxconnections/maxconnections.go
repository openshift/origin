package maxconnections

import "net/http"

// See tooManyRequests from k8s.io/apiserver/pkg/server/filters/maxinflight.go
func defaultOverloadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", "1") // Docker ignores this header, though.
	http.Error(w, "Too many requests, please try again later.", http.StatusTooManyRequests)
}

// Handler implements the http.Handler interface.
type Handler struct {
	limiter Limiter
	handler http.Handler

	OverloadHandler http.Handler
}

// New returns an http.Handler that uses limiter to control h invocation.
// If limiter prohibits starting a new handler, OverloadHandler will be
// invoked.
func New(limiter Limiter, h http.Handler) *Handler {
	return &Handler{
		limiter:         limiter,
		handler:         h,
		OverloadHandler: http.HandlerFunc(defaultOverloadHandler),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.limiter.Start(r.Context()) {
		h.OverloadHandler.ServeHTTP(w, r)
		return
	}
	defer h.limiter.Done()
	h.handler.ServeHTTP(w, r)
}
