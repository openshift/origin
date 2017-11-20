package cmux_test

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strings"

	"github.com/cockroachdb/cmux"
)

type recursiveHTTPHandler struct{}

func (h *recursiveHTTPHandler) ServeHTTP(w http.ResponseWriter,
	r *http.Request) {

	fmt.Fprintf(w, "example http response")
}

func recursiveServeHTTP(l net.Listener) {
	s := &http.Server{
		Handler: &recursiveHTTPHandler{},
	}
	if err := s.Serve(l); err != cmux.ErrListenerClosed {
		panic(err)
	}
}

func tlsListener(l net.Listener) net.Listener {
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
	return tlsl
}

type RecursiveRPCRcvr struct{}

func (r *RecursiveRPCRcvr) Cube(i int, j *int) error {
	*j = i * i
	return nil
}

func recursiveServeRPC(l net.Listener) {
	s := rpc.NewServer()
	if err := s.Register(&RecursiveRPCRcvr{}); err != nil {
		panic(err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			if err != cmux.ErrListenerClosed {
				panic(err)
			}
			return
		}
		go s.ServeConn(conn)
	}
}

// This is an example for serving HTTP, HTTPS, and GoRPC/TLS on the same port.
func Example_recursiveCmux() {
	// Create the TCP listener.
	l, err := net.Listen("tcp", "127.0.0.1:50051")
	if err != nil {
		log.Panic(err)
	}

	// Create a mux.
	tcpm := cmux.New(l)

	// We first match on HTTP 1.1 methods.
	httpl := tcpm.Match(cmux.HTTP1Fast())

	// If not matched, we assume that its TLS.
	tlsl := tcpm.Match(cmux.Any())
	tlsl = tlsListener(tlsl)

	// Now, we build another mux recursively to match HTTPS and GoRPC.
	// You can use the same trick for SSH.
	tlsm := cmux.New(tlsl)
	httpsl := tlsm.Match(cmux.HTTP1Fast())
	gorpcl := tlsm.Match(cmux.Any())
	go recursiveServeHTTP(httpl)
	go recursiveServeHTTP(httpsl)
	go recursiveServeRPC(gorpcl)

	go func() {
		if err := tlsm.Serve(); err != cmux.ErrListenerClosed {
			panic(err)
		}
	}()
	if err := tcpm.Serve(); !strings.Contains(err.Error(), "use of closed network connection") {
		panic(err)
	}
}
