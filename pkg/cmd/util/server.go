package util

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// Copied from http package to support Listen and ListenTLS functions
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

// Listen updates the server's config and starts a net.Listener identically to http.Server.ListenAndServe().
// The listener can then be passed to http.Server.Serve(). Establishing the listener synchronously, then calling
// http.Server.Serve() in a goroutine ensures subsequent connections can connect immediately.
func Listen(srv *http.Server) (net.Listener, error) {
	addr := srv.Addr
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return tcpKeepAliveListener{ln.(*net.TCPListener)}, nil
}

// ListenTLS updates the server's config and starts a net.Listener identically to http.Server.ListenAndServeTLS().
// The listener can then be passed to http.Server.Serve(). Establishing the listener synchronously, then calling
// http.Server.Serve() in a goroutine ensures subsequent connections can connect immediately.
func ListenTLS(srv *http.Server, certFile, keyFile string) (net.Listener, error) {
	addr := srv.Addr
	if addr == "" {
		addr = ":https"
	}
	config := &tls.Config{}
	if srv.TLSConfig != nil {
		*config = *srv.TLSConfig
	}
	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}

	var err error
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	tlsListener := tls.NewListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, config)
	return tlsListener, nil
}
