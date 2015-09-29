// +build integration,etcd

package integration

import (
	"crypto/tls"
	"net/http"
	"testing"

	testserver "github.com/openshift/origin/test/util/server"
)

func TestRootRedirect(t *testing.T) {
	masterConfig, _, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	req, err := http.NewRequest("GET", masterConfig.AssetConfig.MasterPublicURL, nil)
	req.Header.Set("Accept", "*/*")
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected %s, got %s", "application/json", resp.Header.Get("Content-Type"))
	}

	req, err = http.NewRequest("GET", masterConfig.AssetConfig.MasterPublicURL, nil)
	req.Header.Set("Accept", "text/html")
	resp, err = transport.RoundTrip(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Errorf("Expected %d, got %d", http.StatusFound, resp.StatusCode)
	}
	if resp.Header.Get("Location") != masterConfig.AssetConfig.PublicURL {
		t.Errorf("Expected %s, got %s", masterConfig.AssetConfig.PublicURL, resp.Header.Get("Location"))
	}

	// TODO add a test for when asset config is nil, the redirect should not occur in this case even when
	// accept header contains text/html
}
