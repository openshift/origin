package login

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"

	"k8s.io/kubernetes/pkg/client/restclient"
)

func TestNormalizeServerURL(t *testing.T) {
	testCases := []struct {
		originalServerURL   string
		normalizedServerURL string
	}{
		{
			originalServerURL:   "localhost",
			normalizedServerURL: "https://localhost:443",
		},
		{
			originalServerURL:   "https://localhost",
			normalizedServerURL: "https://localhost:443",
		},
		{
			originalServerURL:   "localhost:443",
			normalizedServerURL: "https://localhost:443",
		},
		{
			originalServerURL:   "https://localhost:443",
			normalizedServerURL: "https://localhost:443",
		},
		{
			originalServerURL:   "http://localhost",
			normalizedServerURL: "http://localhost:80",
		},
		{
			originalServerURL:   "localhost:8443",
			normalizedServerURL: "https://localhost:8443",
		},
	}

	for _, test := range testCases {
		t.Logf("evaluating test: normalize %s -> %s", test.originalServerURL, test.normalizedServerURL)
		normalized, err := config.NormalizeServerURL(test.originalServerURL)
		if err != nil {
			t.Errorf("unexpected error normalizing %s: %s", test.originalServerURL, err)
		}
		if normalized != test.normalizedServerURL {
			t.Errorf("unexpected server URL normalization result for %s: expected %s, got %s", test.originalServerURL, test.normalizedServerURL, normalized)
		}
	}
}

func TestDialToHTTPServer(t *testing.T) {
	invoked := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		invoked <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	testCases := map[string]struct {
		serverURL       string
		evalExpectedErr func(error) bool
	}{
		"succeed dialing": {
			serverURL: server.URL,
		},
		"try using HTTPS against HTTP server": {
			serverURL:       "https:" + strings.TrimPrefix(server.URL, "http:"),
			evalExpectedErr: clientcmd.IsTLSOversizedRecord,
		},
	}

	for name, test := range testCases {
		t.Logf("evaluating test: %s", name)
		clientConfig := &restclient.Config{
			Host: test.serverURL,
		}
		if err := dialToServer(*clientConfig); err != nil {
			if test.evalExpectedErr == nil || !test.evalExpectedErr(err) {
				t.Errorf("%s: unexpected error: %v", name, err)
			}
		} else {
			if test.evalExpectedErr != nil {
				t.Errorf("%s: expected error but got nothing", name)
			}
		}
	}
}
