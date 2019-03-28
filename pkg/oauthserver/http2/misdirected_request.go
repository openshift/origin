package http2

import (
	"bytes"
	"net/http"
	"net/url"

	"k8s.io/klog"

	"github.com/openshift/origin/pkg/oauthserver/server/redirect"
)

// This package handles an edge case in our passthrough route handling of HTTP2 endpoints.
//
// Per Dan Mace (@ironcladlou):
//
// Given the following conditions:
//
// * A wildcard ingress certificate (eg. *.apps.openshift.example.com)
// * A DNS wildcard (eg. *.apps.openshift.example.com) with A records resolving to a static set of ingress load balancer IPs
// * A passthrough route to an HTTP2 server (eg. auth.apps.openshift.example.com)
// * A an edge or reencrypted route to a server in the same subdomain (eg. console.apps.openshift.example.com)
// * An HTTP2 client which coalesces connections (eg. Chrome/Firefox)
//
// It's possible for packets destined for a proxy-terminated route (eg. console) to be misdirected to the passthrough/HTTP2 route (eg. auth).
//
// In brief, a connection to the passthrough/HTTP2 server may be reused by the client for packets destined for other
// servers for which the wildcard certificate is valid. Because both route host names are valid for the certificate
// and resolve to the same IPs through DNS, an existing HTTP2 server connection is considered reusable for the other
// servers' packets. However, because the HTTP2 connection at the proxy is coupled to the HTTP2 server through the
// initial SNI header from a TLS handshake, and the packets coming through are opaque and cannot be disambiguated by
// the proxy, the packets cannot be identified by the proxy as misdirected and are all forwarded to the HTTP2 server.
//
// Solutions could be:
//
// 1. Discontinue use of HTTP2 for these services
// 2. Implement mutual TLS in the ingress controller to enable terminating TLS at the proxy for the auth server
// 3. Implement HTTP 421 misdirected request support at the auth server to hint clients to stop reusing the connection for the request authority
//
// Our current recommendation in the short term is (3), and longer term we would like to implement mTLS (2) for ease of use.
//
// References:
//
// * https://httpwg.org/specs/rfc7540.html#reuse
// * https://httpwg.org/specs/rfc7540.html#MisdirectedRequest
// * https://daniel.haxx.se/blog/2016/08/18/http2-connection-coalescing

const (
	// Explicitly limit to a single concurrent open stream per client in
	// an attempt to reduce the chance that we get misdirected requests.
	MaxStreamsPerConnection = 1

	http2ProtoMajor = 2

	responseMisdirectedRequest = `
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
    <meta http-equiv="refresh" content="0" />
  </head>
  <body>
    <p>misdirected request</p>
  </body>
</html>`
)

func WithMisdirectedRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// happy path
		if !isMisdirectedRequest(r) {
			handler.ServeHTTP(w, r)

			// stop processing the request
			return
		}

		// log only the safe metadata for this misdirected request
		klog.Infof("misdirected request detected from %s to %s %s %d instead of %s",
			r.RemoteAddr, r.Method, r.Host, r.ContentLength, r.TLS.ServerName)

		// send the client a graceful close connection via GOAWAY
		w.Header().Set("Connection", "close")

		// set 421 code so that well behaved clients retry the request
		w.WriteHeader(http.StatusMisdirectedRequest)

		// try to force a browser refresh for misbehaving clients
		_, _ = w.Write([]byte(responseMisdirectedRequest))
	})
}

func isMisdirectedRequest(r *http.Request) bool {
	// ignore non-HTTP2 requests
	if r.ProtoMajor != http2ProtoMajor {
		return false
	}

	// check if have a value for the :authority pseudo header field
	// see golang.org/x/net/http2/server.go serverConn.newWriterAndRequest / newWriterAndRequestNoBody
	host := r.Host
	if len(host) == 0 {
		return false
	}

	// ignore non-TLS requests
	tls := r.TLS
	if tls == nil {
		return false
	}

	// check if we know the server name requested by the client
	serverName := tls.ServerName
	if len(serverName) == 0 {
		return false
	}

	// if :authority does not match server name, this request is not meant for us
	return host != serverName
}

// buffer is a ResponseWriter wrapper that allows us to delay calls to
// WriteHeader and Write for the purpose of allowing Header to be mutated
type buffer struct {
	w    http.ResponseWriter
	code int
	buf  bytes.Buffer
}

func (b *buffer) Header() http.Header {
	return b.w.Header()
}

func (b *buffer) WriteHeader(statusCode int) {
	b.code = statusCode
}

func (b *buffer) Write(p []byte) (int, error) {
	return b.buf.Write(p)
}

func WithHTTP2ConnectionClose(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// determine if we need to delay response writing
		if !canMisdirectRequest(r) {
			handler.ServeHTTP(w, r)

			// if we did not delay response writing, we are done
			return
		}

		// delay WriteHeader and Write calls
		b := &buffer{w: w}

		// run the input handler first and let it modify the response
		handler.ServeHTTP(b, r)

		// aggressively close the connection when we are using HTTP2 by
		// sending the client a graceful close connection via GOAWAY
		// this combined with MaxStreamsPerConnection should limit the
		// number of misdirected requests for 302 code exchanges
		if shouldCloseConnection(b, r) {
			b.Header().Set("Connection", "close")
		}

		// now that we are done mutating the header, write the code ...
		if b.code != 0 {
			b.w.WriteHeader(b.code)
		}

		// ... and the response
		if b.buf.Len() != 0 {
			_, _ = b.w.Write(b.buf.Bytes())
		}
	})
}

func canMisdirectRequest(r *http.Request) bool {
	// ignore non-HTTP2 requests
	if r.ProtoMajor != http2ProtoMajor {
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

func shouldCloseConnection(w *buffer, r *http.Request) bool {
	// see if we are trying to redirect the client (a favorite of OAuth)
	location := w.Header().Get("Location")
	if len(location) == 0 {
		// fail closed and assume we are done with an OAuth flow
		// we will end up closing connections early for grant flows,
		// but it is better than reusing connections incorrectly
		return true
	}

	// no need to close the connection if were are redirecting to ourselves
	if redirect.IsServerRelativeURL(location) {
		return false
	}

	// check if we know the server name requested by the client
	serverName := r.TLS.ServerName
	if len(serverName) == 0 {
		return true // fail closed
	}

	u, err := url.Parse(location)
	if err != nil {
		return true // fail closed
	}

	// no need to close the connection if were are redirecting to ourselves
	if u.Host == serverName || u.Hostname() == serverName {
		return false
	}

	// external redirect, close the connection to prevent
	// misdirected requests on other components hosted in the cluster
	return true
}
