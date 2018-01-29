package imageprune

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/util/diff"
)

type requestStats struct {
	lock     sync.Mutex
	requests []string
}

func (rs *requestStats) addRequest(r *http.Request) {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	rs.requests = append(rs.requests, r.URL.String())
}
func (rs *requestStats) clear() {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	rs.requests = rs.requests[:0]
}
func (rs *requestStats) getRequests() []string {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	res := make([]string, 0, len(rs.requests))
	for _, r := range rs.requests {
		res = append(res, r)
	}
	return res
}

func TestDefaultImagePinger(t *testing.T) {
	rs := requestStats{requests: []string{}}

	type statusForPath map[string]int

	rt := knet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	})
	insecureClient := http.Client{Transport: rt}
	secureClient := http.Client{}

	for _, tc := range []struct {
		name                   string
		schemePrefix           string
		securedRegistry        bool
		insecure               bool
		statusForPath          statusForPath
		expectedErrorSubstring string
		expectedRequests       []string
	}{
		{
			name:             "tls secured registry with insecure fallback",
			securedRegistry:  true,
			insecure:         true,
			statusForPath:    statusForPath{"/": http.StatusOK},
			expectedRequests: []string{"/"},
		},

		{
			name:             "tls secured registry prefixed by scheme with insecure fallback",
			schemePrefix:     "https://",
			securedRegistry:  true,
			insecure:         true,
			statusForPath:    statusForPath{"/": http.StatusOK},
			expectedRequests: []string{"/"},
		},

		{
			name:                   "tls secured registry prefixed by http scheme with insecure fallback",
			schemePrefix:           "http://",
			securedRegistry:        true,
			insecure:               true,
			statusForPath:          statusForPath{"/": http.StatusOK},
			expectedErrorSubstring: "malformed HTTP response",
		},

		{
			name:                   "tls secured registry with no fallback",
			securedRegistry:        true,
			insecure:               false,
			statusForPath:          statusForPath{"/": http.StatusOK, "/healthz": http.StatusOK},
			expectedErrorSubstring: "x509: certificate signed by unknown authority",
		},

		{
			name:             "tls secured registry with old healthz endpoint",
			securedRegistry:  true,
			insecure:         true,
			statusForPath:    statusForPath{"/healthz": http.StatusOK},
			expectedRequests: []string{"/", "/healthz"},
		},

		{
			name:             "insecure registry with insecure fallback",
			securedRegistry:  false,
			insecure:         true,
			statusForPath:    statusForPath{"/": http.StatusOK},
			expectedRequests: []string{"/"},
		},

		{
			name:             "insecure registry prefixed by scheme with insecure fallback",
			schemePrefix:     "http://",
			securedRegistry:  false,
			insecure:         true,
			statusForPath:    statusForPath{"/": http.StatusOK},
			expectedRequests: []string{"/"},
		},

		{
			name:                   "insecure registry prefixed by https scheme with insecure fallback",
			schemePrefix:           "https://",
			securedRegistry:        false,
			insecure:               true,
			statusForPath:          statusForPath{"/": http.StatusOK},
			expectedErrorSubstring: "server gave HTTP response to HTTPS client",
		},

		{
			name:                   "insecure registry with no fallback",
			securedRegistry:        false,
			statusForPath:          statusForPath{"/": http.StatusOK, "/healthz": http.StatusOK},
			expectedErrorSubstring: "server gave HTTP response to HTTPS client",
		},

		{
			name:             "insecure registry with old healthz endpoint",
			securedRegistry:  false,
			insecure:         true,
			statusForPath:    statusForPath{"/healthz": http.StatusOK},
			expectedRequests: []string{"/", "/healthz"},
		},

		{
			name:                   "initializing insecure registry",
			securedRegistry:        false,
			insecure:               true,
			statusForPath:          statusForPath{},
			expectedErrorSubstring: "server gave HTTP response to HTTPS client, unexpected status: 404 Not Found",
			expectedRequests:       []string{"/", "/healthz"},
		},
	} {
		func() {
			defer rs.clear()

			handler := func(w http.ResponseWriter, r *http.Request) {
				rs.addRequest(r)
				if s, ok := tc.statusForPath[r.URL.Path]; ok {
					w.WriteHeader(s)
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}

			var server *httptest.Server
			if tc.securedRegistry {
				server = httptest.NewTLSServer(http.HandlerFunc(handler))
			} else {
				server = httptest.NewServer(http.HandlerFunc(handler))
			}
			defer server.Close()
			serverHost := strings.TrimLeft(strings.TrimLeft(server.URL, "http://"), "https://")

			client := &secureClient
			if tc.insecure {
				client = &insecureClient
			}

			pinger := DefaultRegistryPinger{
				Client:   client,
				Insecure: tc.insecure,
			}

			registryURL, err := pinger.Ping(tc.schemePrefix + serverHost)
			if err != nil {
				if len(tc.expectedErrorSubstring) == 0 {
					t.Errorf("[%s] got unexpected ping error of type %T: %v", tc.name, err, err)
				} else if !strings.Contains(err.Error(), tc.expectedErrorSubstring) {
					t.Errorf("[%s] expected substring %q not found in error message: %s", tc.name, tc.expectedErrorSubstring, err.Error())
				}
			} else if len(tc.expectedErrorSubstring) > 0 {
				t.Errorf("[%s] unexpected non-error", tc.name)
			}

			e := server.URL
			if len(tc.expectedErrorSubstring) > 0 {
				// the pinger should return unchanged input in case of error
				e = ""
			}
			a := ""
			if registryURL != nil {
				a = registryURL.String()
			}
			if a != e {
				t.Errorf("[%s] unexpected registry url: %q != %q", tc.name, a, e)
			}

			ers := tc.expectedRequests
			if ers == nil {
				ers = []string{}
			}
			if a := rs.getRequests(); !reflect.DeepEqual(a, ers) {
				t.Errorf("[%s] got unexpected requests: %s", tc.name, diff.ObjectDiff(a, ers))
			}
		}()
	}
}
