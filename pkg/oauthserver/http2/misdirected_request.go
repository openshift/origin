package http2

import "net/http"

func WithHTTP2ConnectionClose(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// aggressively close the connection when we are using HTTP2 by
		// sending the client a graceful close connection via GOAWAY
		// this will prevent misdirected requests for 302 code exchanges
		if canMisdirectRequest(r) {
			w.Header().Set("Connection", "close")
		}
		handler.ServeHTTP(w, r)
	})
}

func canMisdirectRequest(r *http.Request) bool {
	// ignore non-HTTP2 requests
	if r.ProtoMajor != 2 {
		return false
	}

	// ignore non-TLS requests
	tls := r.TLS
	if tls == nil {
		return false
	}

	// terrible things can happen now
	return true
}
