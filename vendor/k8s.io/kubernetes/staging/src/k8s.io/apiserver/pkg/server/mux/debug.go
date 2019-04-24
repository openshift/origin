package mux

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
	"runtime/debug"

	"github.com/davecgh/go-spew/spew"

	"k8s.io/klog"
)

var config = spew.ConfigState{Indent: "\t", MaxDepth: 5, DisableMethods: true}

func WithDebug(handler http.Handler) http.Handler {
	if _, ok := handler.(debugHandlerFunc); ok {
		return handler
	}

	return debugHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := decorateResponseWriter(w)

		handler.ServeHTTP(d, r)

		var final *auditResponseWriter
		switch t := d.(type) {
		case *auditResponseWriter:
			final = t
		case *fancyResponseWriterDelegator:
			final = t.auditResponseWriter
		case *closeResponseWriterDelegator:
			final = t.auditResponseWriter
		case *hijacResponseWriterDelegator:
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

type debugHandlerFunc func(http.ResponseWriter, *http.Request)

func (f debugHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f(w, r)
}

func log(args ...interface{}) {
	klog.ErrorDepth(1, "ENJ:\n", config.Sdump(args...))
	debug.PrintStack()
	klog.Flush()
}

func decorateResponseWriter(responseWriter http.ResponseWriter) http.ResponseWriter {
	delegate := &auditResponseWriter{
		ResponseWriter: responseWriter,
	}

	// check if the ResponseWriter we're wrapping is the fancy one we need
	// or if the basic is sufficient
	_, cn := responseWriter.(http.CloseNotifier)
	_, hj := responseWriter.(http.Hijacker)
	switch {
	case cn && hj:
		return &fancyResponseWriterDelegator{delegate}
	case cn:
		return &closeResponseWriterDelegator{delegate}
	case hj:
		return &hijacResponseWriterDelegator{delegate}
	default:
		return delegate
	}
}

var _ http.ResponseWriter = &auditResponseWriter{}
var _ http.Flusher = &auditResponseWriter{}

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

func (a *auditResponseWriter) Flush() {
	if fl, ok := a.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

var unauthorizedMsg1 = []byte(`{\"kind\":\"Status\",\"apiVersion\":\"v1\",\"metadata\":{},\"status\":\"Failure\",\"message\":\"Unauthorized\",\"reason\":\"Unauthorized\",\"code\":401}`)
var unauthorizedMsg2 = []byte(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"Unauthorized","reason":"Unauthorized","code":401}`)

func (a *auditResponseWriter) shouldLog() bool {
	if a.code == http.StatusUnauthorized {
		return true
	}

	return bytes.Contains(a.buf.Bytes(), unauthorizedMsg1) || bytes.Contains(a.buf.Bytes(), unauthorizedMsg2)
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

func (f *fancyResponseWriterDelegator) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return f.ResponseWriter.(http.Hijacker).Hijack()
}

var _ http.CloseNotifier = &fancyResponseWriterDelegator{}
var _ http.Flusher = &fancyResponseWriterDelegator{}
var _ http.Hijacker = &fancyResponseWriterDelegator{}

type closeResponseWriterDelegator struct {
	*auditResponseWriter
}

func (f *closeResponseWriterDelegator) CloseNotify() <-chan bool {
	return f.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

type hijacResponseWriterDelegator struct {
	*auditResponseWriter
}

func (f *hijacResponseWriterDelegator) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return f.ResponseWriter.(http.Hijacker).Hijack()
}
