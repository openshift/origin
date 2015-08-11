// +build integration,!no-etcd

package integration

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testutil "github.com/openshift/origin/test/util"
)

var (
	stableWebConsoleEndpoints = map[string]int{
		"healthz": http.StatusOK,
		"login":   http.StatusOK,
	}
	switchableWebConsoleEndpoints = map[string]int{
		"console":      http.StatusMovedPermanently,
		"console/":     http.StatusOK,
		"console/java": http.StatusOK,
	}
)

func tryAccessURL(t *testing.T, url string, expectedStatus int, expectedRedirectLocation string) *http.Response {
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
		return nil
	}
	if resp.StatusCode != expectedStatus {
		t.Errorf("Expected status %d for %s, got %d", expectedStatus, url, resp.StatusCode)
	} else {
		if expectedRedirectLocation != "" {
			if resp.Header.Get("Location") != expectedRedirectLocation {
				t.Errorf("Expected %s for %s, got %s", expectedRedirectLocation, url, resp.Header.Get("Location"))
			}
		}
	}
	return resp
}

func TestAccessOriginWebConsole(t *testing.T) {
	masterOptions, err := testutil.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err = testutil.StartConfiguredMaster(masterOptions); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tryAccessURL(t, masterOptions.AssetConfig.MasterPublicURL+"/", http.StatusFound, masterOptions.AssetConfig.PublicURL)

	accessEndpoints := func(endpoints map[string]int) {
		for endpoint, expectedStatus := range endpoints {
			url := masterOptions.AssetConfig.MasterPublicURL + "/" + endpoint
			tryAccessURL(t, url, expectedStatus, "")
		}
	}

	accessEndpoints(stableWebConsoleEndpoints)
	accessEndpoints(switchableWebConsoleEndpoints)
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

	resp := tryAccessURL(t, masterOptions.AssetConfig.MasterPublicURL+"/", http.StatusOK, "")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("failed to read reposponse's body: %v", err)
	} else {
		var value interface{}
		if err = json.Unmarshal(body, &value); err != nil {
			t.Errorf("expected json body which couldn't be parsed: %v, got: %s", err, body)
		}
	}

	for endpoint, expectedStatus := range stableWebConsoleEndpoints {
		url := masterOptions.AssetConfig.MasterPublicURL + "/" + endpoint
		tryAccessURL(t, url, expectedStatus, "")
	}

	for endpoint := range switchableWebConsoleEndpoints {
		url := masterOptions.AssetConfig.MasterPublicURL + "/" + endpoint
		tryAccessURL(t, url, http.StatusForbidden, "")
	}
}
