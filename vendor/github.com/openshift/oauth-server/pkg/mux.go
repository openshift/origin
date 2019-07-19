package oauthserver

import "net/http"

type Mux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

type Endpoints interface {
	// Install registers one or more http.Handlers into the given mux.
	// It is expected that the provided prefix will serve all operations.
	// prefix MUST NOT end in a slash.
	Install(mux Mux, prefix string)
}
