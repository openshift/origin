package httpproxy

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/third_party/golang/netutil"

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
	reverseProxy := httputil.NewSingleHostReverseProxy(backendAddr)
	reverseProxy.FlushInterval = 200 * time.Millisecond
	p := &UpgradeAwareSingleHostReverseProxy{
		clientConfig: clientConfig,
		backendAddr:  backendAddr,
		transport:    transport,
		reverseProxy: reverseProxy,
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

	removeCORSHeaders(resp)
	removeChallengeHeaders(resp)
	if resp.StatusCode == http.StatusUnauthorized {
		util.HandleError(fmt.Errorf("got unauthorized error from backend for: %s %s", req.Method, req.URL))
		// Internal error, backend didn't recognize proxy identity
		// Surface as a server error to the client
		// TODO do we need to do more than this?
		resp = &http.Response{
			StatusCode:    http.StatusInternalServerError,
			Status:        http.StatusText(http.StatusInternalServerError),
			Body:          ioutil.NopCloser(strings.NewReader("Internal Server Error")),
			ContentLength: -1,
		}
	}

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
	backendURL := *p.backendAddr
	// if backendAddr is http://host/base and req is for /foo, the resulting path
	// for backendURL should be /base/foo
	backendURL.Opaque = singleJoiningSlash(backendURL.Path, req.RequestURI)

	// Method used by the httputil.ReverseProxy for requests
	// maps are still shallow copy, but ReverseProxy copies the headers correctly
	newReq := new(http.Request)
	*newReq = *req
	newReq.URL = &backendURL

	// TODO do we need to exclude any other headers?
	removeAuthHeaders(newReq)

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
	dialAddr := netutil.CanonicalAddr(req.URL)

	switch p.backendAddr.Scheme {
	case "http":
		return net.Dial("tcp", dialAddr)
	case "https":
		tlsConfig, err := kclient.TLSConfigFor(p.clientConfig)
		if err != nil {
			return nil, err
		}
		tlsConn, err := tls.Dial("tcp", dialAddr, tlsConfig)
		if err != nil {
			return nil, err
		}
		hostToVerify, _, err := net.SplitHostPort(dialAddr)
		if err != nil {
			return nil, err
		}
		err = tlsConn.VerifyHostname(hostToVerify)
		return tlsConn, err
	default:
		return nil, fmt.Errorf("unknown scheme: %s", p.backendAddr.Scheme)
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

	addAuthHeaders(req, p.clientConfig)

	err = req.Write(backendConn)
	if err != nil {
		glog.Errorf("Error writing request to backend: %s", err)
		return
	}

	resp, err := http.ReadResponse(bufio.NewReader(backendConn), req)
	if err != nil {
		glog.Errorf("Error reading response from backend: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
		return
	}

	if resp.StatusCode == http.StatusUnauthorized {
		glog.Errorf("Got unauthorized error from backend for: %s %s", req.Method, req.URL)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
		return
	}

	requestHijackedConn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		glog.Errorf("Error hijacking request connection: %s", err)
		return
	}
	defer requestHijackedConn.Close()

	// NOTE: from this point forward, we own the connection and we can't use
	// w.Header(), w.Write(), or w.WriteHeader any more

	removeCORSHeaders(resp)
	removeChallengeHeaders(resp)
	err = resp.Write(requestHijackedConn)
	if err != nil {
		glog.Errorf("Error writing backend response to client: %s", err)
		return
	}

	done := make(chan struct{}, 2)

	go func() {
		_, err := io.Copy(backendConn, requestHijackedConn)
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			util.HandleError(fmt.Errorf("error proxying data from client to backend: %v", err))
		}
		done <- struct{}{}
	}()

	go func() {
		_, err := io.Copy(requestHijackedConn, backendConn)
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			util.HandleError(fmt.Errorf("error proxying data from backend to client: %v", err))
		}
		done <- struct{}{}
	}()

	<-done
}

// removeAuthHeaders strips authorization headers from an incoming client
// This should be called on all requests before proxying
func removeAuthHeaders(req *http.Request) {
	req.Header.Del("Authorization")
}

// removeChallengeHeaders strips WWW-Authenticate headers from backend responses
// This should be called on all responses before returning
func removeChallengeHeaders(resp *http.Response) {
	resp.Header.Del("WWW-Authenticate")
}

// removeCORSHeaders strip CORS headers sent from the backend
// This should be called on all responses before returning
func removeCORSHeaders(resp *http.Response) {
	resp.Header.Del("Access-Control-Allow-Credentials")
	resp.Header.Del("Access-Control-Allow-Headers")
	resp.Header.Del("Access-Control-Allow-Methods")
	resp.Header.Del("Access-Control-Allow-Origin")
}

// addAuthHeaders adds basic/bearer auth from the given config (if specified)
// This should be run on any requests not handled by the transport returned from TransportFor(config)
func addAuthHeaders(req *http.Request, clientConfig *kclient.Config) {
	if clientConfig.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+clientConfig.BearerToken)
	} else if clientConfig.Username != "" || clientConfig.Password != "" {
		req.SetBasicAuth(clientConfig.Username, clientConfig.Password)
	}
}
