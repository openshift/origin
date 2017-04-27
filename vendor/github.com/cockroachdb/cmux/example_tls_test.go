package cmux_test

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/cockroachdb/cmux"
)

type anotherHTTPHandler struct{}

func (h *anotherHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "example http response")
}

func serveHTTP1(l net.Listener) {
	s := &http.Server{
		Handler: &anotherHTTPHandler{},
	}
	if err := s.Serve(l); err != cmux.ErrListenerClosed {
		panic(err)
	}
}

func serveHTTPS(l net.Listener) {
	// Load certificates.
	certificate, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Panic(err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		Rand:         rand.Reader,
	}

	// Create TLS listener.
	tlsl := tls.NewListener(l, config)

	// Serve HTTP over TLS.
	serveHTTP1(tlsl)
}

// This is an example for serving HTTP and HTTPS on the same port.
func Example_bothHTTPAndHTTPS() {
	// Create the TCP listener.
	l, err := net.Listen("tcp", "127.0.0.1:50051")
	if err != nil {
		log.Panic(err)
	}

	// Create a mux.
	m := cmux.New(l)

	// We first match on HTTP 1.1 methods.
	httpl := m.Match(cmux.HTTP1Fast())

	// If not matched, we assume that its TLS.
	//
	// Note that you can take this listener, do TLS handshake and
	// create another mux to multiplex the connections over TLS.
	tlsl := m.Match(cmux.Any())

	go serveHTTP1(httpl)
	go serveHTTPS(tlsl)

	if err := m.Serve(); !strings.Contains(err.Error(), "use of closed network connection") {
		panic(err)
	}
}
