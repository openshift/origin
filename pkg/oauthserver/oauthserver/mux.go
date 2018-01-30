package oauthserver

import (
	"net/http"
)

// mux  is a standard mux interface for HTTP
type mux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}
