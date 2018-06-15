package oauthserver

import (
	"net/http"
)

type handlerWrapper interface {
	Wrap(http.Handler) http.Handler
}

// handlerWrapperMux wraps all handlers before registering them in the contained mux
type handlerWrapperMux struct {
	mux     mux
	wrapper handlerWrapper
}

var _ = mux(&handlerWrapperMux{})

func (m *handlerWrapperMux) Handle(pattern string, handler http.Handler) {
	m.mux.Handle(pattern, m.wrapper.Wrap(handler))
}
func (m *handlerWrapperMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.mux.Handle(pattern, m.wrapper.Wrap(http.HandlerFunc(handler)))
}
