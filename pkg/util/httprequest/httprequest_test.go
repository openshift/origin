package httprequest

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"testing"
)

func TestSchemeHost(t *testing.T) {

	testcases := map[string]struct {
		req            *http.Request
		expectedScheme string
		expectedHost   string
	}{
		"X-Forwarded-Host and X-Forwarded-Port combined": {
			req: &http.Request{
				URL:  &url.URL{Path: "/"},
				Host: "127.0.0.1",
				Header: http.Header{
					"X-Forwarded-Host":  []string{"example.com"},
					"X-Forwarded-Port":  []string{"443"},
					"X-Forwarded-Proto": []string{"https"},
				},
			},
			expectedScheme: "https",
			expectedHost:   "example.com:443",
		},
		"X-Forwarded-Port overwrites X-Forwarded-Host port": {
			req: &http.Request{
				URL:  &url.URL{Path: "/"},
				Host: "127.0.0.1",
				Header: http.Header{
					"X-Forwarded-Host":  []string{"example.com:1234"},
					"X-Forwarded-Port":  []string{"443"},
					"X-Forwarded-Proto": []string{"https"},
				},
			},
			expectedScheme: "https",
			expectedHost:   "example.com:443",
		},
		"X-Forwarded-* multiple attrs": {
			req: &http.Request{
				URL:  &url.URL{Host: "urlhost", Path: "/"},
				Host: "reqhost",
				Header: http.Header{
					"X-Forwarded-Host":  []string{"example.com,foo.com"},
					"X-Forwarded-Port":  []string{"443,123"},
					"X-Forwarded-Proto": []string{"https,http"},
				},
			},
			expectedScheme: "https",
			expectedHost:   "example.com:443",
		},

		"req host": {
			req: &http.Request{
				URL:  &url.URL{Host: "urlhost", Path: "/"},
				Host: "example.com",
			},
			expectedScheme: "http",
			expectedHost:   "example.com",
		},
		"req host with port": {
			req: &http.Request{
				URL:  &url.URL{Host: "urlhost", Path: "/"},
				Host: "example.com:80",
			},
			expectedScheme: "http",
			expectedHost:   "example.com:80",
		},
		"req host with tls port": {
			req: &http.Request{
				URL:  &url.URL{Host: "urlhost", Path: "/"},
				Host: "example.com:443",
			},
			expectedScheme: "https",
			expectedHost:   "example.com:443",
		},

		"req tls": {
			req: &http.Request{
				URL:  &url.URL{Path: "/"},
				Host: "example.com",
				TLS:  &tls.ConnectionState{},
			},
			expectedScheme: "https",
			expectedHost:   "example.com",
		},

		"req url": {
			req: &http.Request{
				URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/"},
			},
			expectedScheme: "https",
			expectedHost:   "example.com",
		},
		"req url with port": {
			req: &http.Request{
				URL: &url.URL{Scheme: "https", Host: "example.com:123", Path: "/"},
			},
			expectedScheme: "https",
			expectedHost:   "example.com:123",
		},

		// The following scenarios are captured from actual direct requests to pods
		"non-tls pod": {
			req: &http.Request{
				URL:  &url.URL{Path: "/"},
				Host: "172.17.0.2:9080",
			},
			expectedScheme: "http",
			expectedHost:   "172.17.0.2:9080",
		},
		"tls pod": {
			req: &http.Request{
				URL:  &url.URL{Path: "/"},
				Host: "172.17.0.2:9443",
				TLS:  &tls.ConnectionState{ /* request has non-nil TLS connection state */ },
			},
			expectedScheme: "https",
			expectedHost:   "172.17.0.2:9443",
		},

		// The following scenarios are captured from actual requests to pods via services
		"svc -> non-tls pod": {
			req: &http.Request{
				URL:  &url.URL{Path: "/"},
				Host: "service.default.svc.cluster.local:10080",
			},
			expectedScheme: "http",
			expectedHost:   "service.default.svc.cluster.local:10080",
		},
		"svc -> tls pod": {
			req: &http.Request{
				URL:  &url.URL{Path: "/"},
				Host: "service.default.svc.cluster.local:10443",
				TLS:  &tls.ConnectionState{ /* request has non-nil TLS connection state */ },
			},
			expectedScheme: "https",
			expectedHost:   "service.default.svc.cluster.local:10443",
		},

		// The following scenarios are captured from actual requests to pods via services via routes serviced by haproxy
		"haproxy non-tls route -> svc -> non-tls pod": {
			req: &http.Request{
				URL:  &url.URL{Path: "/"},
				Host: "route-namespace.router.default.svc.cluster.local",
				Header: http.Header{
					"X-Forwarded-Host":  []string{"route-namespace.router.default.svc.cluster.local"},
					"X-Forwarded-Port":  []string{"80"},
					"X-Forwarded-Proto": []string{"http"},
					"Forwarded":         []string{"for=172.18.2.57;host=route-namespace.router.default.svc.cluster.local;proto=http"},
					"X-Forwarded-For":   []string{"172.18.2.57"},
				},
			},
			expectedScheme: "http",
			expectedHost:   "route-namespace.router.default.svc.cluster.local:80",
		},
		"haproxy edge terminated route -> svc -> non-tls pod": {
			req: &http.Request{
				URL:  &url.URL{Path: "/"},
				Host: "route-namespace.router.default.svc.cluster.local",
				Header: http.Header{
					"X-Forwarded-Host":  []string{"route-namespace.router.default.svc.cluster.local"},
					"X-Forwarded-Port":  []string{"443"},
					"X-Forwarded-Proto": []string{"https"},
					"Forwarded":         []string{"for=172.18.2.57;host=route-namespace.router.default.svc.cluster.local;proto=https"},
					"X-Forwarded-For":   []string{"172.18.2.57"},
				},
			},
			expectedScheme: "https",
			expectedHost:   "route-namespace.router.default.svc.cluster.local:443",
		},
		"haproxy passthrough route -> svc -> tls pod": {
			req: &http.Request{
				URL:  &url.URL{Path: "/"},
				Host: "route-namespace.router.default.svc.cluster.local",
				TLS:  &tls.ConnectionState{ /* request has non-nil TLS connection state */ },
			},
			expectedScheme: "https",
			expectedHost:   "route-namespace.router.default.svc.cluster.local",
		},
	}

	for k, tc := range testcases {
		scheme, host := SchemeHost(tc.req)
		if scheme != tc.expectedScheme {
			t.Errorf("%s: expected scheme %q, got %q", k, tc.expectedScheme, scheme)
		}
		if host != tc.expectedHost {
			t.Errorf("%s: expected host %q, got %q", k, tc.expectedHost, host)
		}
	}
}
