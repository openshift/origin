package httpproxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	// "strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	kclient "k8s.io/kubernetes/pkg/client"
)

// Test against two bugs in the original implementation: duplication of base-path on the backendaddr
// and handling of %2F
func TestNewProxyRequest(t *testing.T) {

	baseAddr := "/base"

	// Create request endpoint server
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// nur := r.RequestURI[len(base):] // Remove base from the start
		w.Header().Add("X-RequestURI", r.RequestURI[len(baseAddr):])
	}))
	defer s.Close()

	u, err := url.Parse(s.URL + baseAddr)
	assert.NoError(t, err)

	p, err := NewUpgradeAwareSingleHostReverseProxy(&kclient.Config{}, u)
	assert.NoError(t, err)

	// Create the proxy server that uses UpgradeAwareSingleHostReverseProxy
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Proxy-RequestURI", r.RequestURI)
		p.ServeHTTP(w, r)
	}))
	defer proxy.Close()

	proxyU, err := url.Parse(proxy.URL)
	assert.NoError(t, err)

	// Create requests towards the proxy and wait for the return value
	req := &http.Request{
		Method:     "GET",
		URL:        proxyU,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Host:       u.Host,
	}

	c := &http.Client{}

	// Use Opaque to override any modifications to the actual request path
	q := "q=a"

	req.URL.Path = fmt.Sprintf("%s/%s", req.URL.Path, "service/proxy")
	req.URL.RawQuery = q
	resp, err := c.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, req.URL.Path+"?"+q, resp.Header.Get("X-RequestURI"))

	req.URL.Opaque = fmt.Sprintf("%s/%s", req.URL.Opaque, "service/proxy/target%2Fcpu/data")
	resp, err = c.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, req.URL.Opaque+"?"+q, resp.Header.Get("X-RequestURI"))
}
