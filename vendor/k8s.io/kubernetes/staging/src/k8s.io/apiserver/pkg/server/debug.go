package server

import (
	"bufio"
	"bytes"
	"net"
	"net/http"

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

		if !final.shouldLog() {
			return
		}

		log(final.code, final.buf.String(), final.Header(), r, w)
	})
}

func log(args ...interface{}) {
	klog.ErrorDepth(1, "ENJ:\n", config.Sdump(args...))
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
	code int
	buf  bytes.Buffer
}

func (a *auditResponseWriter) Write(bs []byte) (int, error) {
	_, _ = a.buf.Write(bs)
	return a.ResponseWriter.Write(bs)
}

func (a *auditResponseWriter) WriteHeader(code int) {
	a.ResponseWriter.WriteHeader(code)
	a.code = code
}

var unauthorizedMsg = []byte(`nauthorized`)

func (a *auditResponseWriter) shouldLog() bool {
	if a.code == http.StatusUnauthorized {
		return true
	}

	return bytes.Contains(a.buf.Bytes(), unauthorizedMsg)
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
