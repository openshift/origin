package server

import (
	"bufio"
	"net"
	"net/http"
	"sync"

	"github.com/davecgh/go-spew/spew"

	"k8s.io/klog"
)

var config = spew.ConfigState{Indent: "\t", MaxDepth: 5, DisableMethods: true}

func withDebug(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := decorateResponseWriter(w)

		handler.ServeHTTP(d, r)

		var final *auditResponseWriter
		switch t := d.(type) {
		case *auditResponseWriter:
			final = t
		case *fancyResponseWriterDelegator:
			final = t.auditResponseWriter
		default:
			panic(config.Sdump("ENJ:", "unknown type:", d))
		}

		if !final.shouldLog {
			return
		}

		log(r, w)
	})
}

func log(args ...interface{}) {
	klog.ErrorDepth(1, "ENJ:", config.Sdump(args...))
}

func decorateResponseWriter(responseWriter http.ResponseWriter) http.ResponseWriter {
	delegate := &auditResponseWriter{
		ResponseWriter: responseWriter,
	}

	// check if the ResponseWriter we're wrapping is the fancy one we need
	// or if the basic is sufficient
	_, cn := responseWriter.(http.CloseNotifier)
	_, fl := responseWriter.(http.Flusher)
	_, hj := responseWriter.(http.Hijacker)
	if cn && fl && hj {
		return &fancyResponseWriterDelegator{delegate}
	}
	return delegate
}

var _ http.ResponseWriter = &auditResponseWriter{}

// auditResponseWriter intercepts WriteHeader, sets it in the event. If the sink is set, it will
// create immediately an event (for long running requests).
type auditResponseWriter struct {
	http.ResponseWriter
	once      sync.Once
	shouldLog bool
}

func (a *auditResponseWriter) Write(bs []byte) (int, error) {
	return a.ResponseWriter.Write(bs)
}

func (a *auditResponseWriter) WriteHeader(code int) {
	a.ResponseWriter.WriteHeader(code)
	if code == http.StatusUnauthorized {
		a.once.Do(func() {
			a.shouldLog = true
		})
	}
}

// fancyResponseWriterDelegator implements http.CloseNotifier, http.Flusher and
// http.Hijacker which are needed to make certain http operation (e.g. watch, rsh, etc)
// working.
type fancyResponseWriterDelegator struct {
	*auditResponseWriter
}

func (f *fancyResponseWriterDelegator) CloseNotify() <-chan bool {
	return f.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (f *fancyResponseWriterDelegator) Flush() {
	f.ResponseWriter.(http.Flusher).Flush()
}

func (f *fancyResponseWriterDelegator) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return f.ResponseWriter.(http.Hijacker).Hijack()
}

var _ http.CloseNotifier = &fancyResponseWriterDelegator{}
var _ http.Flusher = &fancyResponseWriterDelegator{}
var _ http.Hijacker = &fancyResponseWriterDelegator{}
