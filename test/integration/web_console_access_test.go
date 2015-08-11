// +build integration,!no-etcd

package integration

import (
	"crypto/tls"
	"net/http"
	"testing"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testutil "github.com/openshift/origin/test/util"
)

var (
	stableWebConsoleEndpoints     = []string{"healthz", "login"}
	switchableWebConsoleEndpoints = []string{"console", "console/", "console/java"}
)

func tryAccessURL(t *testing.T, url string, expectedStatus int) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "text/html")
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Errorf("Unexpected error while accessing %s: %v", url, err)
	}
	if resp.StatusCode != expectedStatus {
		t.Errorf("Expected status %d for %s, got %d", expectedStatus, url, resp.StatusCode)
	}
}

func TestAccessOriginWebConsole(t *testing.T) {
	masterOptions, err := testutil.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err = testutil.StartConfiguredMaster(masterOptions); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allEndpoints := append(stableWebConsoleEndpoints, switchableWebConsoleEndpoints...)
	for _, endpoint := range allEndpoints {
		url := masterOptions.AssetConfig.MasterPublicURL + "/" + endpoint
		expectedStatus := http.StatusOK
		if endpoint == "console" {
			expectedStatus = http.StatusMovedPermanently
		}
		tryAccessURL(t, url, expectedStatus)
	}
}

func TestAccessDisabledWebConsole(t *testing.T) {
	masterOptions, err := testutil.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	masterOptions.DisabledFeatures.Add(configapi.FeatureWebConsole)
	if _, err := testutil.StartConfiguredMaster(masterOptions); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, endpoint := range stableWebConsoleEndpoints {
		url := masterOptions.AssetConfig.MasterPublicURL + "/" + endpoint
		tryAccessURL(t, url, http.StatusOK)
	}

	for _, endpoint := range switchableWebConsoleEndpoints {
		url := masterOptions.AssetConfig.MasterPublicURL + "/" + endpoint
		tryAccessURL(t, url, http.StatusForbidden)
	}
}
