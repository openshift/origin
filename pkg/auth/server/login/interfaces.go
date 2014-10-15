package login

import "net/http"

// mux is an object that can register http handlers.
type Mux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

type CSRF interface {
	Generate() (string, error)
	Check(string) (bool, error)
}
