package origin

import (
	"net/http"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

type handlerWrapper interface {
	Wrap(http.Handler) http.Handler
}

// handlerWrapperMux wraps all handlers before registering them in the contained mux
type handlerWrapperMux struct {
	mux     cmdutil.Mux
	wrapper handlerWrapper
}

var _ = cmdutil.Mux(&handlerWrapperMux{})

func (m *handlerWrapperMux) Handle(pattern string, handler http.Handler) {
	m.mux.Handle(pattern, m.wrapper.Wrap(handler))
}
func (m *handlerWrapperMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.mux.Handle(pattern, m.wrapper.Wrap(http.HandlerFunc(handler)))
}
