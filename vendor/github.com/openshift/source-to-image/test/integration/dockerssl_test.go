// +build integration

package integration

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/docker"
)

const (
	tcpListener  = "127.0.0.1:8080"
	tlsListener  = "127.0.0.1:8443"
	unixListener = "/tmp/test.sock"
)

var serverCert tls.Certificate
var caPool *x509.CertPool

func init() {
	var err error
	serverCert, err = tls.LoadX509KeyPair("testdata/127.0.0.1.crt", "testdata/127.0.0.1.key")
	if err != nil {
		panic(err)
	}

	ca, err := ioutil.ReadFile("testdata/ca.crt")
	if err != nil {
		panic(err)
	}

	caPool = x509.NewCertPool()
	caPool.AppendCertsFromPEM(ca)

	log.SetOutput(ioutil.Discard)
}

type Server struct {
	l net.Listener
	c chan struct{}
}

func (s *Server) Close() {
	s.l.Close()
	<-s.c
}

func (s *Server) serveFakeDockerAPIServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/version", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})
	hs := http.Server{Handler: mux}
	// Disable keepalives in order to prevent an explosion in the number of
	// goroutines that makes stack traces noisy.  TODO: when using Go 1.8,
	// http.Server.Shutdown() should do this for us.
	hs.SetKeepAlivesEnabled(false)
	hs.Serve(s.l)
	s.c <- struct{}{}
}

func serveTLS(t *testing.T, config *tls.Config) *Server {
	config.Certificates = []tls.Certificate{serverCert}

	l, err := tls.Listen("tcp", tlsListener, config)
	if err != nil {
		t.Fatal(err)
	}

	s := &Server{l: l, c: make(chan struct{})}
	go s.serveFakeDockerAPIServer()

	return s
}

func serveTCP(t *testing.T) *Server {
	l, err := net.Listen("tcp", tcpListener)
	if err != nil {
		t.Fatal(err)
	}

	s := &Server{l: l, c: make(chan struct{})}
	go s.serveFakeDockerAPIServer()

	return s
}

func serveUNIX(t *testing.T) *Server {
	os.Remove(unixListener)

	l, err := net.Listen("unix", unixListener)
	if err != nil {
		t.Fatal(err)
	}

	s := &Server{l: l, c: make(chan struct{})}
	go s.serveFakeDockerAPIServer()

	return s
}

func runTest(t *testing.T, config *api.DockerConfig, expectedSuccess bool) {
	client, err := docker.NewEngineAPIClient(config)
	if err != nil {
		if expectedSuccess {
			t.Errorf("with DockerConfig %+v, expected success %v, got error %v", config, expectedSuccess, err)
		}
		return
	}
	d := docker.New(client, api.AuthConfig{})
	err = d.CheckReachable()
	if (err == nil) != expectedSuccess {
		t.Errorf("with DockerConfig %+v, expected success %v, got error %v", config, expectedSuccess, err)
	}
}

func TestTCP(t *testing.T) {
	s := serveTCP(t)
	defer s.Close()

	dc := &api.DockerConfig{Endpoint: "tcp://" + tcpListener}

	for _, dc.UseTLS = range []bool{true, false} {
		for _, dc.TLSVerify = range []bool{true, false} {
			for _, dc.CAFile = range []string{"testdata/ca.crt", "bad", ""} {
				for _, dc.CertFile = range []string{"testdata/client.crt", "bad", ""} {
					for _, dc.KeyFile = range []string{"testdata/client.key", "bad", ""} {
						runTest(t, dc, !dc.UseTLS && !dc.TLSVerify)
					}
				}
			}
		}
	}
}

func TestUNIX(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}

	s := serveUNIX(t)
	defer s.Close()

	dc := &api.DockerConfig{Endpoint: "unix://" + unixListener}

	for _, dc.UseTLS = range []bool{true, false} {
		for _, dc.TLSVerify = range []bool{true, false} {
			for _, dc.CAFile = range []string{"testdata/ca.crt", "bad", ""} {
				for _, dc.CertFile = range []string{"testdata/client.crt", "bad", ""} {
					for _, dc.KeyFile = range []string{"testdata/client.key", "bad", ""} {
						runTest(t, dc, !dc.UseTLS && !dc.TLSVerify)
					}
				}
			}
		}
	}
}

func TestSSL(t *testing.T) {
	s := serveTLS(t, &tls.Config{})
	defer s.Close()

	dc := &api.DockerConfig{Endpoint: "tcp://" + tlsListener}

	for _, dc.UseTLS = range []bool{true, false} {
		for _, dc.TLSVerify = range []bool{true, false} {
			for _, dc.CAFile = range []string{"testdata/ca.crt", "bad", ""} {
				for _, dc.CertFile = range []string{"testdata/client.crt", "bad", ""} {
					for _, dc.KeyFile = range []string{"testdata/client.key", "bad", ""} {
						expected := dc.UseTLS && !dc.TLSVerify || dc.TLSVerify && dc.CAFile == "testdata/ca.crt"

						if (dc.CertFile == "testdata/client.crt") != (dc.KeyFile == "testdata/client.key") {
							expected = false
						}

						runTest(t, dc, expected)
					}
				}
			}
		}
	}
}

func TestSSLClientCert(t *testing.T) {
	s := serveTLS(t, &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  caPool,
	})
	defer s.Close()

	dc := &api.DockerConfig{Endpoint: "tcp://" + tlsListener}

	for _, dc.UseTLS = range []bool{true, false} {
		for _, dc.TLSVerify = range []bool{true, false} {
			for _, dc.CAFile = range []string{"testdata/ca.crt", "bad", ""} {
				for _, dc.CertFile = range []string{"testdata/client.crt", "bad", ""} {
					for _, dc.KeyFile = range []string{"testdata/client.key", "bad", ""} {
						expected := dc.UseTLS && !dc.TLSVerify || dc.TLSVerify && dc.CAFile == "testdata/ca.crt"

						if dc.CertFile != "testdata/client.crt" || dc.KeyFile != "testdata/client.key" {
							expected = false
						}

						runTest(t, dc, expected)
					}
				}
			}
		}
	}
}
