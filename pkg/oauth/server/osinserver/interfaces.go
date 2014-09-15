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
	HandleAuthorize(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) bool
}

type AccessHandler interface {
	HandleAccess(ar *osin.AccessRequest, w http.ResponseWriter, r *http.Request) bool
}

type AuthorizeHandlerFunc func(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) bool

func (f AuthorizeHandlerFunc) HandleAuthorize(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) bool {
	return f(ar, w, r)
}

type AccessHandlerFunc func(ar *osin.AccessRequest, w http.ResponseWriter, r *http.Request) bool

func (f AccessHandlerFunc) HandleAccess(ar *osin.AccessRequest, w http.ResponseWriter, r *http.Request) bool {
	return f(ar, w, r)
}
