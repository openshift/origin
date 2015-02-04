package httpproxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/golang/glog"
)

// UpgradeAwareSingleHostReverseProxy is capable of proxying both regular HTTP
// connections and those that require upgrading (e.g. web sockets). It implements
// the http.RoundTripper and http.Handler interfaces.
type UpgradeAwareSingleHostReverseProxy struct {
	clientConfig *kclient.Config
	backendAddr  *url.URL
	transport    http.RoundTripper
	reverseProxy *httputil.ReverseProxy
}

// NewUpgradeAwareSingleHostReverseProxy creates a new UpgradeAwareSingleHostReverseProxy.
func NewUpgradeAwareSingleHostReverseProxy(clientConfig *kclient.Config, backendAddr *url.URL) (*UpgradeAwareSingleHostReverseProxy, error) {
	transport, err := kclient.TransportFor(clientConfig)
	if err != nil {
		return nil, err
	}
	p := &UpgradeAwareSingleHostReverseProxy{
		clientConfig: clientConfig,
		backendAddr:  backendAddr,
		transport:    transport,
		reverseProxy: httputil.NewSingleHostReverseProxy(backendAddr),
	}
	p.reverseProxy.Transport = p
	return p, nil
}

// RoundTrip sends the request to the backend and strips off the CORS headers
// before returning the response.
func (p *UpgradeAwareSingleHostReverseProxy) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := p.transport.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	// strip off CORS headers sent from the backend
	resp.Header.Del("Access-Control-Allow-Credentials")
	resp.Header.Del("Access-Control-Allow-Headers")
	resp.Header.Del("Access-Control-Allow-Methods")
	resp.Header.Del("Access-Control-Allow-Origin")

	// TODO do we need to strip off anything else?

	return resp, err
}

// borrowed from net/http/httputil/reverseproxy.go
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func (p *UpgradeAwareSingleHostReverseProxy) newProxyRequest(req *http.Request) (*http.Request, error) {
	// TODO is this the best way to clone the original request and create
	// the new request for the backend? Do we need to copy anything else?
	//
	backendURL := *p.backendAddr
	// if backendAddr is http://host/base and req is for /foo, the resulting path
	// for backendURL should be /base/foo
	backendURL.Path = singleJoiningSlash(backendURL.Path, req.URL.Path)
	backendURL.RawQuery = req.URL.RawQuery

	newReq, err := http.NewRequest(req.Method, backendURL.String(), req.Body)
	if err != nil {
		return nil, err
	}
	// TODO is this the right way to copy headers?
	newReq.Header = req.Header
	// TODO do we need to exclude any headers?
	return newReq, nil
}

func (p *UpgradeAwareSingleHostReverseProxy) isUpgradeRequest(req *http.Request) bool {
	for _, h := range req.Header[http.CanonicalHeaderKey("Connection")] {
		if strings.Contains(strings.ToLower(h), "upgrade") {
			return true
		}
	}
	return false
}

// ServeHTTP inspects the request and either proxies an upgraded connection directly,
// or uses httputil.ReverseProxy to proxy the normal request.
func (p *UpgradeAwareSingleHostReverseProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	newReq, err := p.newProxyRequest(req)
	if err != nil {
		glog.Errorf("Error creating backend request: %s", err)
		// TODO do we need to do more than this?
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !p.isUpgradeRequest(req) {
		p.reverseProxy.ServeHTTP(w, newReq)
		return
	}

	p.serveUpgrade(w, newReq)
}

func (p *UpgradeAwareSingleHostReverseProxy) dialBackend(req *http.Request) (net.Conn, error) {
	switch p.backendAddr.Scheme {
	case "http":
		return net.Dial("tcp", req.URL.Host)
	case "https":
		tlsConfig, err := kclient.TLSConfigFor(p.clientConfig)
		if err != nil {
			return nil, err
		}
		tlsConn, err := tls.Dial("tcp", req.URL.Host, tlsConfig)
		if err != nil {
			return nil, err
		}
		hostToVerify := req.URL.Host
		if index := strings.Index(hostToVerify, ":"); index > -1 {
			hostToVerify = hostToVerify[0:index]
		}
		err = tlsConn.VerifyHostname(hostToVerify)
		return tlsConn, err
	default:
		return nil, fmt.Errorf("Unknown scheme: %s", p.backendAddr.Scheme)
	}
}

func (p *UpgradeAwareSingleHostReverseProxy) serveUpgrade(w http.ResponseWriter, req *http.Request) {
	backendConn, err := p.dialBackend(req)
	if err != nil {
		glog.Errorf("Error connecting to backend: %s", err)
		// TODO do we need to do more than this?
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	defer backendConn.Close()

	requestHijackedConn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		glog.Errorf("Error hijacking request connection: %s", err)
		return
	}
	defer requestHijackedConn.Close()

	// NOTE: from this point forward, we own the connection and we can't use
	// w.Header(), w.Write(), or w.WriteHeader any more

	err = req.Write(backendConn)
	if err != nil {
		glog.Errorf("Error writing request to backend: %s", err)
	}

	done := make(chan struct{}, 2)

	go func() {
		_, err := io.Copy(backendConn, requestHijackedConn)
		if err != nil {
			// TODO I see this printed at least whenever the client goes away from the page.
			// Should we check for different types of errors and only log certain ones?
			// Should we make this V(2+) ?
			glog.Errorf("Error proxying data from client to backend: %v", err)
		}
		done <- struct{}{}
	}()

	go func() {
		_, err := io.Copy(requestHijackedConn, backendConn)
		if err != nil {
			glog.Errorf("Error proxying data from backend to client: %v", err)
		}
		done <- struct{}{}
	}()

	<-done
}
