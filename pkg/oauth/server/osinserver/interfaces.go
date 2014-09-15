package osinserver

import (
	"net/http"

	"github.com/RangelReale/osin"
)

// mux is an object that can register http handlers.
type Mux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

type AuthorizeHandler interface {
	HandleAuthorize(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) (handled bool)
}

type AuthorizeHandlerFunc func(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) bool

func (f AuthorizeHandlerFunc) HandleAuthorize(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) bool {
	return f(ar, w, r)
}

type AuthorizeHandlers []AuthorizeHandler

func (all AuthorizeHandlers) HandleAuthorize(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) bool {
	for _, h := range all {
		if h.HandleAuthorize(ar, w, r) {
			return true
		}
	}
	return false
}

type AccessHandler interface {
	HandleAccess(ar *osin.AccessRequest, w http.ResponseWriter, r *http.Request)
}

type AccessHandlerFunc func(ar *osin.AccessRequest, w http.ResponseWriter, r *http.Request)

func (f AccessHandlerFunc) HandleAccess(ar *osin.AccessRequest, w http.ResponseWriter, r *http.Request) {
	f(ar, w, r)
}

type AccessHandlers []AccessHandler

func (all AccessHandlers) HandleAccess(ar *osin.AccessRequest, w http.ResponseWriter, r *http.Request) {
	for _, h := range all {
		h.HandleAccess(ar, w, r)
	}
}
