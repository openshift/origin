package util

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
)

// TryListen tries to open a connection on the given port and returns true if it succeeded.
func TryListen(network, hostPort string) (bool, error) {
	l, err := net.Listen(network, hostPort)
	if err != nil {
		glog.V(5).Infof("Failure while checking listen on %s: %v", err)
		return false, err
	}
	defer l.Close()
	return true, nil
}

type connPair struct {
	conn net.Conn
	err  error
}

type multiListener struct {
	listeners []net.Listener
	accept    chan connPair
	stop      chan struct{}
	once      sync.Once
	wg        sync.WaitGroup
}

func NewMultiListener(listeners ...net.Listener) net.Listener {
	l := &multiListener{
		listeners: listeners,
		accept:    make(chan connPair),
		stop:      make(chan struct{}),
	}
	for i := range listeners {
		l.wg.Add(1)
		go l.Run(listeners[i])
	}
	return l
}

func (l *multiListener) Run(listener net.Listener) {
	defer l.wg.Done()
	for {
		select {
		case _, ok := <-l.stop:
			if !ok {
				return
			}
		default:
		}
		c, err := listener.Accept()
		l.accept <- connPair{conn: c, err: err}
	}
}

func (l *multiListener) Accept() (c net.Conn, err error) {
	pair, ok := <-l.accept
	if !ok {
		return nil, fmt.Errorf("listener closed")
	}
	return pair.conn, pair.err
}

// Close invokes close on each listener in the list, then signals to the
// accept workers to close, starts a goroutine to drain any new accepts
// while the workers complete, waits for the workers to exit, and finally
// returns. Each listener will only be closed once.
func (l *multiListener) Close() error {
	var firstErr error
	l.once.Do(func() {
		for i, li := range l.listeners {
			if i == 0 {
				firstErr = li.Close()
			} else {
				li.Close()
			}
		}
		close(l.stop)
		go func() {
			// drain any connections that may not have been accepted yet
			for pair := range l.accept {
				if pair.conn != nil {
					pair.conn.Close()
				}
			}
		}()
		l.wg.Wait()
		close(l.accept)
	})
	return firstErr
}

func (l *multiListener) Addr() net.Addr {
	return l.listeners[0].Addr()
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
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

// ListenAndServe starts a server that listens on the provided TCP mode (as supported
// by net.Listen)
func ListenAndServe(srv *http.Server, network string) error {
	addr := srv.Addr
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen(network, addr)
	if err != nil {
		return err
	}
	ln = tcpKeepAliveListener{ln.(*net.TCPListener)}
	return srv.Serve(ln)
}

// AddCertKeyToTLSConfig loads an X509 certificate into the provided tls config.
func AddCertKeyToTLSConfig(config *tls.Config, certFile, keyFile string) error {
	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	config.Certificates = []tls.Certificate{cert}
	return nil
}

// ListenAndServeTLS starts a server that listens on the provided TCP mode (as supported
// by net.Listen).
func ListenAndServeTLS(srv *http.Server, network string) error {
	addr := srv.Addr
	if addr == "" {
		addr = ":https"
	}

	ln, err := net.Listen(network, addr)
	if err != nil {
		return err
	}
	ln = tcpKeepAliveListener{ln.(*net.TCPListener)}

	tlsListener := tls.NewListener(ln, srv.TLSConfig)
	return srv.Serve(tlsListener)
}

// WaitForSuccessfulDial attempts to connect to the given address, closing and returning nil on the first successful connection.
func WaitForSuccessfulDial(https bool, network, address string, timeout, interval time.Duration, retries int) error {
	var (
		conn net.Conn
		err  error
	)
	for i := 0; i <= retries; i++ {
		dialer := net.Dialer{Timeout: timeout}
		if https {
			conn, err = tls.DialWithDialer(&dialer, network, address, &tls.Config{InsecureSkipVerify: true})
		} else {
			conn, err = dialer.Dial(network, address)
		}
		if err != nil {
			glog.V(5).Infof("Got error %#v, trying again: %#v\n", err, address)
			time.Sleep(interval)
			continue
		}
		conn.Close()
		return nil
	}
	return err
}

// TransportFor returns an http.Transport for the given ca and client cert (which may be empty strings)
func TransportFor(ca string, certFile string, keyFile string) (http.RoundTripper, error) {
	if len(ca) == 0 && len(certFile) == 0 && len(keyFile) == 0 {
		return http.DefaultTransport, nil
	}

	if (len(certFile) == 0) != (len(keyFile) == 0) {
		return nil, errors.New("certFile and keyFile must be specified together")
	}

	// Copy default transport
	transport := *http.DefaultTransport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{}

	if len(ca) != 0 {
		roots, err := CertPoolFromFile(ca)
		if err != nil {
			return nil, fmt.Errorf("error loading cert pool from ca file %s: %v", ca, err)
		}
		transport.TLSClientConfig.RootCAs = roots
	}

	if len(certFile) != 0 {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("error loading x509 keypair from cert file %s and key file %s: %v", certFile, keyFile, err)
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}

	return &transport, nil
}

// GetCertificateFunc returns a function that can be used in tls.Config#GetCertificate
// Returns nil if len(certs) == 0
func GetCertificateFunc(certs map[string]*tls.Certificate) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	if len(certs) == 0 {
		return nil
	}
	// Replica of tls.Config#getCertificate logic
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if clientHello == nil {
			return nil, nil
		}

		name := clientHello.ServerName
		name = strings.ToLower(name)
		name = strings.TrimRight(name, ".")
		for _, candidate := range HostnameMatchSpecCandidates(name) {
			if cert, ok := certs[candidate]; ok {
				return cert, nil
			}
		}
		return nil, nil
	}
}

// HostnameMatchSpecCandidates returns a list of match specs that would match the provided hostname
// Returns nil if len(hostname) == 0
func HostnameMatchSpecCandidates(hostname string) []string {
	if len(hostname) == 0 {
		return nil
	}

	// Exact match has priority
	candidates := []string{hostname}

	// Replace successive labels in the name with wildcards, to require an exact match on number of
	// path segments, because certificates cannot wildcard multiple levels of subdomains
	//
	// This is primarily to be consistent with tls.Config#getCertificate implementation
	//
	// It using a cert signed for *.foo.example.com and *.bar.example.com by specifying the name *.*.example.com
	labels := strings.Split(hostname, ".")
	for i := range labels {
		labels[i] = "*"
		candidates = append(candidates, strings.Join(labels, "."))
	}
	return candidates
}

// HostnameMatches returns true if the given hostname is matched by the given matchSpec
func HostnameMatches(hostname string, matchSpec string) bool {
	return sets.NewString(HostnameMatchSpecCandidates(hostname)...).Has(matchSpec)
}
